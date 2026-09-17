package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Completed replacement is an explicit reviewed destination decision. A random
// backup name would invalidate preview fingerprints on every re-plan, so bind
// the saved recovery path to this exact client/release/content/destination pair.
func (s *Service) planCompletedReplacements(ctx context.Context, op *ImportOperation, bookDirs []string) error {
	planned := map[string]bool{}
	for i := range op.Files {
		f := &op.Files[i]
		planned[f.DestinationPath] = true
		if err := checkCompletedReplacementOwner(ctx, s.store.db, *f); err != nil {
			return err
		}
		info, err := os.Lstat(f.DestinationPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		resolved, err := filepath.EvalSymlinks(f.DestinationPath)
		if err != nil || resolved != f.DestinationPath || !info.Mode().IsRegular() {
			return errors.New("replacement requires a regular destination without symlinks")
		}
		hash, err := scanContentHash(ctx, f.DestinationPath)
		if err != nil {
			return err
		}
		after, err := os.Lstat(f.DestinationPath)
		if err != nil {
			return err
		}
		if !os.SameFile(info, after) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
			return errors.New("replacement destination changed while previewing")
		}
		sum := sha256.Sum256([]byte(op.Client + "\x00" + op.DownloadID + "\x00" + f.DestinationPath + "\x00" + f.SHA256 + "\x00" + hash))
		f.PreviousPath = filepath.Join(filepath.Dir(f.DestinationPath), ".librarry-previous-"+hex.EncodeToString(sum[:16]))
		f.PreviousSHA256, f.PreviousSizeBytes = hash, info.Size()
		if _, err := os.Lstat(f.PreviousPath); !errors.Is(err, os.ErrNotExist) {
			if err != nil {
				return err
			}
			return errors.New("replacement recovery path already exists; resume its saved operation or review it")
		}
		if err := checkCompletedReplacementOwner(ctx, s.store.db, *f); err != nil {
			return err
		}
	}
	return validateReplacementDirectories(bookDirs, planned)
}
func validateReplacementDirectories(dirs []string, allowed map[string]bool) error {
	for _, dir := range dirs {
		if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if errors.Is(err, os.ErrNotExist) && path == dir {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return errors.New("replacement book directory contains a symlink")
			}
			if entry.IsDir() {
				return nil
			}
			if !allowed[path] {
				return fmt.Errorf("existing book file is outside the new manifest: %s; review the old chapter set or keep both", path)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
func verifyCompletedReplacementLayout(op ImportOperation) error {
	if op.SourceKind == "manual" || op.Metadata["conflictAction"] != "replace" {
		return nil
	}
	dirs := []string{}
	switch paths := op.Metadata["replacementDirectories"].(type) {
	case []string:
		dirs = paths
	case []any:
		for _, path := range paths {
			value, ok := path.(string)
			if !ok {
				return errors.New("invalid saved replacement directory")
			}
			dirs = append(dirs, value)
		}
	}
	allowed := map[string]bool{}
	for _, f := range op.Files {
		allowed[f.DestinationPath] = true
		if f.PreviousPath != "" {
			allowed[f.PreviousPath] = true
		}
		if f.StagePath != "" {
			allowed[f.StagePath] = true
		}
	}
	return validateReplacementDirectories(dirs, allowed)
}

type replacementOwnerReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func checkCompletedReplacementOwner(ctx context.Context, db replacementOwnerReader, file ImportOperationFile) error {
	var conflicting bool
	err := db.QueryRowContext(ctx, `select
 exists(select 1 from files f join file_wanted_links l on l.file_id=f.id where f.path=$1 and l.wanted_item_id::text<>$2)
 or (not exists(select 1 from files f where f.path=$1) and coalesce((select coalesce(m.wanted_item_id,o.wanted_item_id)::text from import_operation_files m join import_operations o on o.id=m.operation_id where m.destination_path=$1 and o.state='committed' order by o.created_at desc,o.id desc limit 1),'') not in ('',$2))
 or exists(select 1 from files f where f.path=$1 and (f.metadata ? 'calibreId' or f.media_format<>$3))`, file.DestinationPath, file.WantedID, file.Format).Scan(&conflicting)
	if err != nil {
		return err
	}
	if conflicting {
		return errors.New("replacement destination belongs to a different book or Calibre; keep both or review its assignment")
	}
	return nil
}

func fenceCompletedReplacementOwner(ctx context.Context, tx *sql.Tx, file ImportOperationFile) error {
	var id string
	err := tx.QueryRowContext(ctx, `select id::text from files where path=$1 for update`, file.DestinationPath).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return checkCompletedReplacementOwner(ctx, tx, file)
}
func preserveCompletedReplacementMetadata(ctx context.Context, tx *sql.Tx, record FileRecord) (FileRecord, error) {
	var title, author string
	var raw []byte
	err := tx.QueryRowContext(ctx, `select title,author_name,metadata from files where path=$1`, record.Path).Scan(&title, &author, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return record, nil
	}
	if err != nil {
		return record, err
	}
	existing := map[string]any{}
	if err := json.Unmarshal(raw, &existing); err != nil {
		return record, err
	}
	operational := map[string]bool{}
	for _, key := range []string{"fingerprint", "modifiedAt", "scannedAt", "wantedId", "downloadId", "downloadClient", "previousPath", "replacedExisting", "requestedImportMode", "move", "hardlinked", "conflictAction", "conflictPath", "importOperationId", "importMode", "verifiedDownload", "importedAt"} {
		operational[key] = true
	}
	for key, value := range existing {
		if !operational[key] {
			record.Metadata[key] = value
		}
	}
	record.Title = firstNonEmpty(title, record.Title)
	record.AuthorName = firstNonEmpty(author, record.AuthorName)
	return record, nil
}
