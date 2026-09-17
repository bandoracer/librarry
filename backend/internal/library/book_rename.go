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
	"strings"
)

type BookRenamePreview struct {
	Generation        string                `json:"-"`
	WantedID          string                `json:"wantedId"`
	Title             string                `json:"title"`
	Revision          string                `json:"revision"`
	SourceFolder      string                `json:"sourceFolder"`
	DestinationFolder string                `json:"destinationFolder"`
	MediaFiles        int                   `json:"mediaFiles"`
	CompanionFiles    int                   `json:"companionFiles"`
	Noop              bool                  `json:"noop"`
	OperationID       string                `json:"operationId,omitempty"`
	Files             []ImportOperationFile `json:"files"`
}

// The entire recorded book set is the unit of authorization. File basenames and
// relative disc paths stay unchanged, including relative playlist references.
func (s *Service) PreviewBookRename(ctx context.Context, wantedID string) (BookRenamePreview, error) {
	preview, _, err := s.planBookRename(ctx, wantedID)
	return preview, err
}
func (s *Service) RenameBook(ctx context.Context, wantedID, revision string) (ImportOutcome, error) {
	preview, op, err := s.planBookRename(ctx, wantedID)
	if err != nil {
		return ImportOutcome{}, err
	}
	if revision == "" || revision != preview.Revision {
		return ImportOutcome{}, errors.New("book rename preview changed; refresh before applying")
	}
	if preview.Noop {
		return ImportOutcome{Skipped: true, Message: "book folder already matches naming templates"}, nil
	}
	if op.ID == "" {
		op, err = s.store.planOperation(ctx, op)
		if err != nil {
			return ImportOutcome{}, err
		}
	}
	return s.runImportOperation(ctx, op)
}
func (s *Service) planBookRename(ctx context.Context, wantedID string) (BookRenamePreview, ImportOperation, error) {
	wantedID = strings.ToLower(strings.TrimSpace(wantedID))
	preview := BookRenamePreview{WantedID: wantedID, Files: []ImportOperationFile{}}
	op := ImportOperation{}
	fail := func(err error) (BookRenamePreview, ImportOperation, error) { return preview, op, err }
	if !s.Available() || !repairUUID.MatchString(wantedID) {
		return fail(errors.New("a persisted book is required"))
	}
	item, err := s.lookupWanted(ctx, wantedID)
	if err != nil {
		return fail(err)
	}
	preview.Title = item.Title
	preview.Generation = item.UpdatedAt.String()
	// Resume the exact set already authorized, even if current naming changed.
	var saved string
	err = s.store.db.QueryRowContext(ctx, `select id::text from import_operations where source_kind='manual' and metadata ? 'renameFileId' and metadata->>'renameWantedId'=$1 and (state<>'committed' or cleanup_state<>'cleaned') order by created_at,id limit 1`, wantedID).Scan(&saved)
	if err == nil {
		op, err = s.store.getOperation(ctx, saved)
		if err != nil {
			return fail(err)
		}
		preview.SourceFolder = op.SourceRoot
		preview.DestinationFolder = metadataString(op.Metadata, "renameDestinationFolder")
		preview.OperationID = op.ID
		preview.Files = op.Files
		completeBookRenamePreview(&preview)
		return preview, op, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fail(err)
	}
	// A current complete manifest must account for every linked media row. Multiple
	// editions, corrected associations, stale receipts and incomplete sets require
	// review instead of silently moving a subset of the book.
	var originID string
	err = s.store.db.QueryRowContext(ctx, `select o.id::text from import_operations o
 where o.state='committed' and not(o.metadata ? 'renameFileId')
 and exists(select 1 from import_operation_files m where m.operation_id=o.id and coalesce(m.wanted_item_id,o.wanted_item_id)=$1 and m.media_format<>'sidecar')
 and not exists(select 1 from file_wanted_links l where l.wanted_item_id=$1 and not exists(select 1 from import_operation_files m where m.operation_id=o.id and m.file_id=l.file_id and coalesce(m.wanted_item_id,o.wanted_item_id)=$1))
 and not exists(select 1 from import_operation_files m left join files f on f.id=m.file_id where m.operation_id=o.id and coalesce(m.wanted_item_id,o.wanted_item_id)=$1 and m.media_format<>'sidecar' and (f.id is null or f.checksum<>m.sha256 or f.size_bytes<>m.size_bytes or not exists(select 1 from file_wanted_links l where l.file_id=f.id and l.wanted_item_id=$1)))
 order by o.created_at desc,o.id desc limit 1`, wantedID).Scan(&originID)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(errors.New("no complete recorded import covers this book's current files; review its file associations before renaming the folder"))
	}
	if err != nil {
		return fail(err)
	}
	origin, err := s.store.getOperation(ctx, originID)
	if err != nil {
		return fail(err)
	}
	if (origin.SourceKind == "manual" && origin.CleanupState != "cleaned") || origin.ReplacementCleanupState == "pending" {
		return fail(errors.New("finish the original import cleanup before renaming its folder"))
	}
	members := []ImportOperationFile{}
	groups := map[string]bool{}
	mediaCount := 0
	for _, f := range origin.Files {
		group := firstNonEmpty(f.WantedID, origin.WantedID)
		if f.Format != "sidecar" {
			groups[group] = true
		}
		if group == wantedID {
			members = append(members, f)
			if f.Format != "sidecar" {
				mediaCount++
			}
		}
	}
	if mediaCount == 0 {
		return fail(errors.New("book manifest has no media files"))
	}
	folders, err := s.store.ListRootFolders(ctx)
	if err != nil {
		return fail(err)
	}
	if item.RootFolderID != "" {
		if _, ok := rootFolderByID(folders, item.RootFolderID); !ok {
			return fail(errors.New("selected library root is no longer configured"))
		}
	}
	for i := range folders {
		path, err := canonicalPlannedPath(folders[i].Path)
		if err != nil {
			return fail(err)
		}
		folders[i].Path = path
	}
	libraryRoot := s.legacyRootForFormat(item.Format)
	if folder, ok := resolveImportRootFolder(folders, item.Format, item.RootFolderID); ok {
		if folder.Calibre.Enabled {
			return fail(errors.New("Calibre manages this book's destination layout"))
		}
		libraryRoot = folder.Path
	}
	if strings.TrimSpace(libraryRoot) == "" {
		return fail(errors.New("library root is not configured"))
	}
	libraryRoot, err = canonicalPlannedPath(libraryRoot)
	if err != nil {
		return fail(err)
	}
	parsed := parsedBook{Title: item.Title, AuthorName: item.AuthorName, Series: firstNonEmpty(wantedOverrideValue(item, "series"), item.Series), SeriesPosition: firstNonEmpty(wantedOverrideValue(item, "series_position"), item.SeriesPosition)}
	if item.FirstPublishYear > 0 {
		parsed.Year = fmt.Sprint(item.FirstPublishYear)
	}
	target := filepath.Dir(s.destinationPathIn(libraryRoot, item.Format, parsed, filepath.Ext(members[0].DestinationPath)))
	if !pathWithinRoot(target, libraryRoot) || target == libraryRoot {
		return fail(errors.New("naming must put the complete book in its own folder inside the library root"))
	}
	if origin.SourceKind != "manual" && pathWithinRoot(target, origin.SourceRoot) {
		return fail(errors.New("book folder must stay outside its recorded download storage"))
	}
	oldAnchor, err := originalBookAnchor(origin, members, len(groups))
	if err != nil {
		return fail(err)
	}
	currentAnchor := oldAnchor
	var movedAnchor string
	err = s.store.db.QueryRowContext(ctx, `select metadata->>'renameDestinationFolder' from import_operations where source_kind='manual' and metadata ? 'renameFileId' and metadata->>'renameOriginOperationId'=$1 and metadata->>'renameWantedId'=$2 and state='committed' order by committed_at desc,id desc limit 1`, origin.ID, wantedID).Scan(&movedAnchor)
	if err == nil {
		currentAnchor = movedAnchor
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fail(err)
	}
	op = ImportOperation{SourceKind: "manual", Mode: "move", Format: item.Format, DestinationRoot: libraryRoot, Files: []ImportOperationFile{}, Metadata: map[string]any{"renameWantedId": wantedID, "renameOriginOperationId": origin.ID, "renameDestinationFolder": target, "title": item.Title, "author": item.AuthorName}}
	for _, old := range members {
		current, err := s.currentManifestDestination(ctx, origin, old)
		if err != nil {
			return fail(err)
		}
		if _, managed := calibreRootForPath(folders, current); managed {
			return fail(errors.New("Calibre manages a file in this book"))
		}
		anchor := currentAnchor
		if len(members) == 1 {
			anchor = filepath.Dir(current)
		}
		if !pathWithinRoot(current, anchor) {
			return fail(errors.New("book files no longer share a proven book folder; review the complete set before renaming"))
		}
		relative, err := filepath.Rel(anchor, current)
		if err != nil {
			return fail(err)
		}

		if op.SourceRoot == "" {
			op.SourceRoot = anchor
		} else if op.SourceRoot != anchor {
			return fail(errors.New("book files are spread across folders; review the complete set before renaming"))
		}
		f, err := newManifestFile(current, filepath.Join(target, relative), anchor, old.Format, len(op.Files))
		if err != nil {
			return fail(err)
		}
		if f.SizeBytes != old.SizeBytes || f.SHA256 != old.SHA256 {
			return fail(errors.New("book content changed since its verified import; review before renaming"))
		}
		f.FileID = old.FileID
		f.RenameOriginFileID = firstNonEmpty(old.RenameOriginFileID, old.ID)
		if f.Format != "sidecar" {
			if metadataString(op.Metadata, "renameFileId") == "" {
				op.Metadata["renameFileId"] = f.FileID
			}
			var owned bool
			if err = s.store.db.QueryRowContext(ctx, `select exists(select 1 from files where id=$1 and media_format=$3 and not(metadata ? 'calibreId')) and exists(select 1 from file_wanted_links where file_id=$1 and wanted_item_id=$2) and not exists(select 1 from file_wanted_links where file_id=$1 and wanted_item_id<>$2)`, f.FileID, wantedID, item.Format).Scan(&owned); err != nil {
				return fail(err)
			}
			if !owned {
				return fail(errors.New("a book file is shared, reassigned or managed by Calibre; retain it for review"))
			}
			var generation string
			if err = s.store.db.QueryRowContext(ctx, `select updated_at::text from files where id=$1`, f.FileID).Scan(&generation); err != nil {
				return fail(err)
			}
			preview.Generation += "|" + f.FileID + ":" + generation

		}
		if f.Format == "sidecar" {
			var tracked bool
			if err = s.store.db.QueryRowContext(ctx, `select exists(select 1 from files where path=$1)`, f.SourcePath).Scan(&tracked); err != nil {
				return fail(err)
			}
			if tracked {
				return fail(errors.New("a companion is also a tracked file; review its ownership before renaming"))
			}
		}
		op.Files = append(op.Files, f)
	}
	if op.SourceRoot != target && (pathWithinRoot(target, op.SourceRoot) || pathWithinRoot(op.SourceRoot, target)) {
		return fail(errors.New("source and destination book folders must not overlap"))
	}
	if err = verifyBookRenameInventory(op); err != nil {
		return fail(err)
	}
	if op.SourceRoot != target {
		for _, f := range op.Files {
			if _, err := os.Lstat(f.DestinationPath); err == nil {
				return fail(errors.New("destination book folder already contains a planned filename; retain it for review"))
			} else if !errors.Is(err, os.ErrNotExist) {
				return fail(err)
			}
		}
	}
	preview.SourceFolder = op.SourceRoot
	preview.DestinationFolder = target
	preview.Noop = op.SourceRoot == target
	preview.Files = op.Files
	completeBookRenamePreview(&preview)
	op.RequestKey = "book-rename:" + preview.Revision
	op.Metadata["previewRevision"] = preview.Revision
	return preview, op, nil
}

func completeBookRenamePreview(preview *BookRenamePreview) {
	preview.Revision = ""
	for _, f := range preview.Files {
		if f.Format == "sidecar" {
			preview.CompanionFiles++
		} else {
			preview.MediaFiles++
		}
	}
	raw, _ := json.Marshal(struct {
		Preview    *BookRenamePreview
		Generation string
	}{preview, preview.Generation})
	hash := sha256.Sum256(raw)
	preview.Revision = hex.EncodeToString(hash[:])
}

func originalBookAnchor(op ImportOperation, files []ImportOperationFile, groups int) (string, error) {
	if op.SourceKind == "manual" || len(files) == 1 {
		anchor := filepath.Dir(files[0].DestinationPath)
		for _, f := range files {
			anchor = commonPathRoot(anchor, filepath.Dir(f.DestinationPath))
		}
		return anchor, nil
	}
	var payload DownloadPayload
	raw, _ := json.Marshal(op.Metadata["payload"])
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Root == "" {
		return "", errors.New("original chapter folder evidence is unavailable")
	}
	groupRoot := payload.Root
	if groups > 1 {
		groupRoot = ""
		for _, f := range files {
			if f.Format != "sidecar" {
				if groupRoot == "" {
					groupRoot = filepath.Dir(f.SourcePath)
				} else {
					groupRoot = commonPathRoot(groupRoot, filepath.Dir(f.SourcePath))
				}
			}
		}
	}
	anchor := ""
	for _, f := range files {
		relative, err := filepath.Rel(groupRoot, f.SourcePath)
		if err != nil {
			return "", err
		}
		if !pathWithinRoot(f.SourcePath, groupRoot) {
			relative = filepath.Base(f.SourcePath)
		}
		candidate := f.DestinationPath
		for range strings.Split(relative, string(filepath.Separator)) {
			candidate = filepath.Dir(candidate)
		}
		if filepath.Join(candidate, relative) != f.DestinationPath {
			return "", errors.New("original book folder cannot be proven from its manifest")
		}
		if anchor == "" {
			anchor = candidate
		} else if anchor != candidate {
			return "", errors.New("original manifest spans multiple book folders")
		}
	}
	return anchor, nil
}

func verifyBookRenameInventory(op ImportOperation) error {
	if err := verifyBookRenameSourceLayout(op, true); err != nil {
		return err
	}
	return verifyBookRenameTargetLayout(op)
}
func verifyBookRenameSourceLayout(op ImportOperation, requireAll bool) error {
	if metadataString(op.Metadata, "renameWantedId") == "" {
		return nil
	}
	expected := map[string]ImportOperationFile{}
	for _, f := range op.Files {
		expected[f.SourcePath] = f
	}
	seen := 0
	err := filepath.WalkDir(op.SourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("book folder contains a symlink; retain the set for review")
		}
		if entry.IsDir() {
			return nil
		}
		f, ok := expected[path]
		if !ok {
			return fmt.Errorf("book folder contains an unrecorded file: %s", path)
		}
		seen++
		if f.Format == "sidecar" {
			return verifyBookRenameReferences(path, op.SourceRoot, expected)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if requireAll && seen != len(expected) {
		return errors.New("a recorded book file is missing")
	}
	return nil
}

func verifyBookRenameTargetLayout(op ImportOperation) error {
	if metadataString(op.Metadata, "renameWantedId") == "" {
		return nil
	}
	target := metadataString(op.Metadata, "renameDestinationFolder")
	if target == op.SourceRoot {
		return nil
	}
	allowed := map[string]bool{}
	for _, f := range op.Files {
		allowed[f.DestinationPath] = true
		if f.StagePath != "" {
			allowed[f.StagePath] = true
		}
	}
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(target, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("destination book folder contains a symlink")
		}
		if entry.IsDir() {
			return nil
		}
		if !allowed[path] {
			return fmt.Errorf("destination book folder contains an unplanned file: %s", path)
		}
		return nil
	})
}
