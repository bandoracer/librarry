package library

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

const renameSetReason = "Keep this file with its chapter set or companion files. Use Rename book folder on the book page to preview a complete recorded set."

// Per-file selection cannot authorize moving unselected chapters or rewriting
// playlist references. Preserve these layouts until a complete-set rename can
// capture and preview every member in one operation.
func (s *Service) renameSetRestriction(ctx context.Context, file FileRecord) (string, error) {
	if s.Available() {
		var grouped bool
		err := s.store.db.QueryRowContext(ctx, `select exists(
   select 1 from import_operation_files own join import_operations o on o.id=own.operation_id
   join import_operation_files other on other.operation_id=own.operation_id and other.id<>own.id
   where own.file_id=$1 and o.state='committed' and not(o.metadata ? 'renameFileId')
    and (other.media_format='sidecar' or (own.wanted_item_id is not distinct from other.wanted_item_id and own.media_format='audiobook'))
  ) or exists(select 1 from file_wanted_links a join file_wanted_links b on b.wanted_item_id=a.wanted_item_id and b.file_id<>a.file_id join files f on f.id=b.file_id where a.file_id=$1 and $2='audiobook' and f.media_format='audiobook')`, file.ID, file.MediaFormat).Scan(&grouped)
		if err != nil {
			return "", err
		}
		if grouped {
			return renameSetReason, nil
		}
	}
	if !s.Available() {
		return "", nil
	}
	if file.MediaFormat == "audiobook" && (discFolder.MatchString(filepath.Base(filepath.Dir(file.Path))) || chapterFilename.MatchString(strings.TrimSuffix(filepath.Base(file.Path), filepath.Ext(file.Path)))) {
		return renameSetReason, nil
	}
	entries, err := os.ReadDir(filepath.Dir(file.Path))
	if err != nil {
		return "", err
	}
	extras := map[string]bool{".cue": true, ".m3u": true, ".m3u8": true, ".opf": true, ".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".nfo": true}
	for _, ext := range importExtraExtensions(s.Config().ImportExtraFiles) {
		extras[ext] = true
	}
	for _, entry := range entries {
		path := filepath.Join(filepath.Dir(file.Path), entry.Name())
		if path == file.Path {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !entry.IsDir() && extras[ext] {
			return renameSetReason, nil
		}
		if file.MediaFormat == "audiobook" && (entry.IsDir() || payloadFileFormat(path, "") == "audiobook") {
			return renameSetReason, nil
		}
	}
	return "", nil
}
