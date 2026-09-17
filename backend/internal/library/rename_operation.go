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
	"sort"
	"strings"
	"time"
)

func (op ImportOperation) isRename() bool {
	return op.SourceKind == "manual" && metadataString(op.Metadata, "renameFileId") != ""
}

func (s *Store) activeRename(ctx context.Context, fileID string) (ImportOperation, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `select operation_id::text from file_rename_claims where file_id=$1`, fileID).Scan(&id)
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
		p := RenameFilePreview{File: file, SourcePath: file.Path, DestinationPath: file.Path, RelativePath: filepath.Base(file.Path), Noop: true, Reason: "This file belongs to a saved complete-book rename. Resume its full plan in Imports."}
		p.Revision = renameRevision(p)
		return p, true, nil
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
	if len(op.Files) == 0 {
		return nil, errors.New("rename manifest is empty")
	}
	// Lock media identities before their associations, matching ordinary file
	// edits and avoiding a file/link lock inversion during owner corrections.
	ids := []string{}
	for _, f := range op.Files {
		if f.Format != "sidecar" {
			ids = append(ids, f.FileID)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		var locked string
		if err := tx.QueryRowContext(ctx, `select id::text from files where id=$1 for update`, id).Scan(&locked); err != nil {
			return nil, err
		}
	}
	if err := fenceBookRenameMembers(ctx, tx, op); err != nil {
		return nil, err
	}
	stored := []FileRecord{}
	for _, f := range op.Files {
		if f.Format == "sidecar" {
			continue
		}
		id := f.FileID
		var currentPath, format string
		if err := tx.QueryRowContext(ctx, `select path,media_format from files where id=$1 for update`, id).Scan(&currentPath, &format); err != nil {
			return nil, err
		}
		if currentPath != f.SourcePath || format != f.Format {
			return nil, errors.New("file location or format changed; saved rename requires review")
		}
		if bookID := metadataString(op.Metadata, "renameWantedId"); bookID != "" {
			var foreign bool
			if err := tx.QueryRowContext(ctx, `select exists(select 1 from file_wanted_links where file_id=$1 and wanted_item_id<>$2)`, id, bookID).Scan(&foreign); err != nil {
				return nil, err
			}
			if foreign {
				return nil, errors.New("a book file was assigned to another book; retain the saved rename for review")
			}
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
		stored = append(stored, file)
	}
	if len(stored) == 0 {
		return nil, errors.New("rename manifest has no recorded media files")
	}
	if _, err := tx.ExecContext(ctx, `update import_operation_files set state='committed',updated_at=now() where operation_id=$1`, op.ID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update import_operations set state='committed',committed_at=now(),lease_token=null,lease_expires_at=null,last_error='',updated_at=now() where id=$1`, op.ID); err != nil {
		return nil, err
	}
	return stored, nil
}

func renameDestinationFileID(op ImportOperation, f ImportOperationFile) string {
	if op.isRename() {
		return f.FileID
	}
	return ""
}

// Serialize planning by the original file ID, including requests using another
// destination. An unfinished plan wins; a retry never invents another target.
func reserveRenameFile(ctx context.Context, tx *sql.Tx, op ImportOperation) (string, error) {
	primary := metadataString(op.Metadata, "renameFileId")
	files := map[string]ImportOperationFile{}
	ids := []string{}
	for _, f := range op.Files {
		if f.Format == "sidecar" {
			if f.FileID != "" {
				return "", errors.New("sidecar cannot claim a media identity")
			}
			continue
		}
		if !repairUUID.MatchString(f.FileID) {
			return "", errors.New("invalid rename file identity")
		}
		if _, ok := files[f.FileID]; ok {
			return "", errors.New("duplicate rename file identity")
		}
		ids = append(ids, f.FileID)
		files[f.FileID] = f
	}
	if _, ok := files[primary]; !ok {
		return "", errors.New("rename primary file is missing")
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,2))`, "rename-file:"+id); err != nil {
			return "", err
		}
	}
	for _, id := range ids {
		var existing, key string
		err := tx.QueryRowContext(ctx, `select o.id::text,o.request_key from file_rename_claims c join import_operations o on o.id=c.operation_id where c.file_id=$1`, id).Scan(&existing, &key)
		if err == nil {
			if key == op.RequestKey {
				return existing, nil
			}
			return "", errors.New("a selected file belongs to another saved rename; recover it in Imports")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		var path, format string
		if err = tx.QueryRowContext(ctx, `select path,media_format from files where id=$1`, id).Scan(&path, &format); err != nil {
			return "", err
		}
		if path != files[id].SourcePath || format != files[id].Format {
			return "", errors.New("selected file changed before rename planning")
		}
		var pending bool
		if err = tx.QueryRowContext(ctx, `select exists(select 1 from import_operation_files f join import_operations o on o.id=f.operation_id where (f.destination_path=$1 or f.file_id=$2) and (o.state<>'committed' or (o.source_kind='manual' and o.cleanup_state<>'cleaned') or o.replacement_cleanup_state='pending'))`, path, id).Scan(&pending); err != nil {
			return "", err
		}
		if pending {
			return "", errors.New("file belongs to an unfinished import; recover it before renaming")
		}
	}
	return "", nil
}

func renameDestinationOriginID(op ImportOperation, f ImportOperationFile) string {
	if op.isRename() {
		return f.RenameOriginFileID
	}
	return ""
}

func fenceBookRenameMembers(ctx context.Context, tx *sql.Tx, op ImportOperation) error {
	bookID := metadataString(op.Metadata, "renameWantedId")
	if bookID == "" {
		return nil
	}
	var locked string
	if err := tx.QueryRowContext(ctx, `select id::text from wanted_items where id=$1 for update`, bookID).Scan(&locked); err != nil {
		return err
	}
	expected := map[string]bool{}
	for _, f := range op.Files {
		if f.Format != "sidecar" {
			expected[f.FileID] = true
		}
	}
	rows, err := tx.QueryContext(ctx, `select file_id::text from file_wanted_links where wanted_item_id=$1 order by file_id for update`, bookID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if !expected[id] {
			return errors.New("book membership changed; review the saved rename")
		}
		delete(expected, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(expected) > 0 {
		return errors.New("a selected file is no longer assigned to this book")
	}
	return nil
}
