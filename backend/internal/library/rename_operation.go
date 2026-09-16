package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (op ImportOperation) isRename() bool {
	return op.SourceKind == "manual" && metadataString(op.Metadata, "renameFileId") != ""
}

func (s *Store) activeRename(ctx context.Context, fileID string) (ImportOperation, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `select id::text from import_operations where source_kind='manual' and metadata ? 'renameFileId' and metadata->>'renameFileId'=$1 and (state<>'committed' or cleanup_state<>'cleaned') order by created_at,id limit 1`, fileID).Scan(&id)
	if err != nil {
		return ImportOperation{}, err
	}
	return s.getOperation(ctx, id)
}

func renameRevision(p RenameFilePreview) string {
	p.Revision = ""
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// An existing plan remains the target of retries even after naming settings
// change or its visibility commit succeeds but source cleanup is interrupted.
func (s *Service) resumeRenamePreview(ctx context.Context, file FileRecord) (RenameFilePreview, bool, error) {
	if !s.Available() {
		return RenameFilePreview{}, false, nil
	}
	op, err := s.store.activeRename(ctx, file.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return RenameFilePreview{}, false, nil
	}
	if err != nil {
		return RenameFilePreview{}, false, err
	}
	if len(op.Files) != 1 {
		return RenameFilePreview{}, true, errors.New("rename manifest needs review")
	}
	f := op.Files[0]
	p := RenameFilePreview{File: file, OperationID: op.ID, SourcePath: f.SourcePath, DestinationPath: f.DestinationPath, RelativePath: f.RelativePath, Reason: "Resume saved rename"}
	p.Revision = renameRevision(p)
	return p, true, nil
}

func (s *Service) applyRename(ctx context.Context, preview RenameFilePreview) (FileRecord, string, error) {
	var op ImportOperation
	var err error
	if preview.OperationID != "" {
		op, err = s.store.getOperation(ctx, preview.OperationID)
	} else {
		op, err = s.store.activeRename(ctx, preview.File.ID)
		if errors.Is(err, sql.ErrNoRows) {
			root, e := canonicalPlannedPath(s.renameRootForFile(ctx, preview.File))
			if e != nil {
				return FileRecord{}, "", e
			}
			if !pathWithinRoot(preview.DestinationPath, root) {
				return FileRecord{}, "", errors.New("rename destination is outside the library root")
			}
			file, e := newManifestFile(preview.SourcePath, preview.DestinationPath, filepath.Dir(preview.SourcePath), preview.File.MediaFormat, 0)
			if e != nil {
				return FileRecord{}, "", e
			}
			if preview.File.Checksum != "" && preview.File.Checksum != file.SHA256 {
				return FileRecord{}, "", errors.New("file bytes changed since the recorded import or scan; rescan before renaming")
			}
			file.FileID = preview.File.ID
			raw, _ := json.Marshal(struct{ ID, Source, Destination, Hash, Generation string }{file.FileID, file.SourcePath, file.DestinationPath, file.SHA256, preview.File.UpdatedAt.Format(time.RFC3339Nano)})
			sum := sha256.Sum256(raw)
			op = ImportOperation{SourceKind: "manual", RequestKey: "rename:" + hex.EncodeToString(sum[:]), SourceRoot: filepath.Dir(file.SourcePath), DestinationRoot: root, Format: file.Format, Mode: "move", Metadata: map[string]any{"renameFileId": file.FileID, "title": preview.File.Title, "author": preview.File.AuthorName, "previewRevision": preview.Revision}, Files: []ImportOperationFile{file}}
			op, err = s.store.planOperation(ctx, op)
		}
	}
	if err != nil {
		return FileRecord{}, "", err
	}
	if !op.isRename() || metadataString(op.Metadata, "renameFileId") != preview.File.ID {
		return FileRecord{}, op.ID, errors.New("rename plan does not match the selected file")
	}
	outcome, err := s.runImportOperation(ctx, op)
	if err != nil {
		return outcome.File, op.ID, fmt.Errorf("rename needs recovery in Imports: %w", err)
	}
	return outcome.File, op.ID, nil
}

// Called under the operation lock, in the same transaction as the visibility
// commit. Update only file location/evidence; never replay stale JSON links or
// overwrite current names, notes, source provenance, book status or monitoring.
func commitRenameFile(ctx context.Context, tx *sql.Tx, op ImportOperation) ([]FileRecord, error) {
	if len(op.Files) != 1 {
		return nil, errors.New("rename manifest must contain one recorded file")
	}
	f := op.Files[0]
	id := metadataString(op.Metadata, "renameFileId")
	var currentPath, format string
	if err := tx.QueryRowContext(ctx, `select path,media_format from files where id=$1 for update`, id).Scan(&currentPath, &format); err != nil {
		return nil, err
	}
	if currentPath != f.SourcePath || format != f.Format || f.FileID != id {
		return nil, errors.New("file location or format changed; saved rename requires review")
	}
	info, err := os.Stat(f.DestinationPath)
	if err != nil {
		return nil, err
	}
	file, err := scanFile(tx.QueryRowContext(ctx, `update files set path=$2,size_bytes=$3,checksum=$4,modified_at=$5,presence_state='present',extension=$6,updated_at=now() where id=$1 returning id,coalesce(edition_id::text,''),media_format,path,source_path,title,author_name,extension,coalesce(size_bytes,0),coalesce(checksum,''),import_status,metadata,modified_at,created_at,updated_at,presence_state`, id, f.DestinationPath, f.SizeBytes, f.SHA256, info.ModTime().UTC(), strings.ToLower(filepath.Ext(f.DestinationPath))))
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(map[string]any{"operationId": op.ID, "sourcePath": f.SourcePath, "destinationPath": f.DestinationPath, "sha256": f.SHA256})
	if _, err = tx.ExecContext(ctx, `insert into history_events(event_type,entity_type,entity_id,severity,message,data) values('file_renamed','file',$1,'info',$2,$3::jsonb)`, id, "Renamed "+filepath.Base(f.SourcePath), string(data)); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `update import_operation_files set state='committed',file_id=$2,updated_at=now() where operation_id=$1`, op.ID, id); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `update import_operations set state='committed',committed_at=now(),lease_token=null,lease_expires_at=null,last_error='',updated_at=now() where id=$1`, op.ID); err != nil {
		return nil, err
	}
	return []FileRecord{file}, nil
}

// Serialize planning by the original file ID, including requests using another
// destination. An unfinished plan wins; a retry never invents another target.
func reserveRenameFile(ctx context.Context, tx *sql.Tx, op ImportOperation) (string, error) {
	id := metadataString(op.Metadata, "renameFileId")
	if len(op.Files) != 1 || op.Files[0].FileID != id || !repairUUID.MatchString(id) {
		return "", errors.New("invalid rename file identity")
	}
	if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,2))`, "rename-file:"+id); err != nil {
		return "", err
	}
	var existing string
	err := tx.QueryRowContext(ctx, `select id::text from import_operations where source_kind='manual' and metadata ? 'renameFileId' and metadata->>'renameFileId'=$1 and (state<>'committed' or cleanup_state<>'cleaned') limit 1`, id).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	var path, format string
	if err = tx.QueryRowContext(ctx, `select path,media_format from files where id=$1`, id).Scan(&path, &format); err != nil {
		return "", err
	}
	if path != op.Files[0].SourcePath || format != op.Format {
		return "", errors.New("selected file changed before rename planning")
	}
	var pending bool
	if err = tx.QueryRowContext(ctx, `select exists(select 1 from import_operation_files f join import_operations o on o.id=f.operation_id where (f.destination_path=$1 or f.file_id=$2) and (o.state<>'committed' or (o.source_kind='manual' and o.cleanup_state<>'cleaned') or o.replacement_cleanup_state='pending'))`, path, id).Scan(&pending); err != nil {
		return "", err
	}
	if pending {
		return "", errors.New("file belongs to an unfinished import; recover it before renaming")
	}
	return "", nil
}
