package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type PayloadMapping struct {
	RelativePath string `json:"relativePath"`
	WantedID     string `json:"wantedId,omitempty"`
	Exclude      bool   `json:"exclude,omitempty"`
}

type PayloadReviewError struct {
	Payload DownloadPayload
	Reasons []string
}

func (e *PayloadReviewError) Error() string { return strings.Join(e.Reasons, "; ") }

func canonicalPlannedPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("an absolute library root is required")
	}
	suffix := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if path == filepath.Dir(path) {
			return "", err
		}
		suffix = append(suffix, filepath.Base(path))
		path = filepath.Dir(path)
	}
}

func (s *Service) planPayload(ctx context.Context, payload DownloadPayload, request ImportRequest, mappings []PayloadMapping, confirmed bool) (ImportOperation, ImportOutcome, error) {
	op := ImportOperation{Client: payload.Client, DownloadID: payload.DownloadID, SourceRoot: payload.SourceRoot, Mode: normalizeImportMode(request.ImportMode, request.Move), Files: []ImportOperationFile{}}
	if op.Mode == "move" {
		return op, ImportOutcome{}, errors.New("completed downloads must retain their sources")
	}
	assignments := map[string]PayloadMapping{}
	for _, mapping := range mappings {
		if _, ok := assignments[mapping.RelativePath]; ok {
			return op, ImportOutcome{}, errors.New("duplicate file assignment")
		}
		assignments[mapping.RelativePath] = mapping
	}
	books := map[string]wanted.WantedItem{}
	groups := map[string][]PayloadFile{}
	groupOrder := []string{}
	reasons := []string{}
	exclusions := []PayloadFile{}
	sidecars := []PayloadFile{}
	for _, f := range payload.Files {
		if f.Format == "excluded" {
			exclusions = append(exclusions, f)
			continue
		}
		mapping, mapped := assignments[f.RelativePath]
		delete(assignments, f.RelativePath)
		if mapped && mapping.Exclude {
			if !confirmed {
				reasons = append(reasons, "file exclusions require review confirmation")
			}
			f.Included = false
			f.Reason = "retained in downloads by operator"
			exclusions = append(exclusions, f)
			continue
		}
		if !f.Included {
			reasons = append(reasons, f.RelativePath+": "+f.Reason)
			continue
		}
		if f.Format == "sidecar" {
			sidecars = append(sidecars, f)
			continue
		}
		id := request.WantedID
		if mapped {
			id = mapping.WantedID
		}
		if id == "" {
			reasons = append(reasons, f.RelativePath+": select a wanted book")
			continue
		}
		item, ok := books[id]
		if !ok {
			var err error
			item, err = s.lookupWanted(ctx, id)
			if err != nil {
				return op, ImportOutcome{}, err
			}
			books[id] = item
			groupOrder = append(groupOrder, id)
		}
		if item.Status == "removed" || item.Status == "ignored" {
			reasons = append(reasons, item.Title+": book is not eligible for import")
		}
		if item.Format != "any" && item.Format != "" && normalizeFormat(item.Format) != f.Format {
			reasons = append(reasons, f.RelativePath+": wanted format does not match")
		}
		groups[id] = append(groups[id], f)
	}
	if len(assignments) > 0 {
		return op, ImportOutcome{}, errors.New("file assignment is not in the current client inventory")
	}
	if len(groups) == 0 {
		reasons = append(reasons, "select at least one complete book file")
	}
	if !confirmed {
		if len(groups) != 1 {
			reasons = append(reasons, "multiple books require explicit file mapping")
		}
		for _, id := range groupOrder {
			reasons = append(reasons, s.payloadMatchReasons(ctx, payload, books[id])...)
		}
	}
	if len(reasons) > 0 {
		return op, ImportOutcome{}, &PayloadReviewError{Payload: payload, Reasons: compactStrings(reasons)}
	}
	action := normalizeConflictAction(request.ConflictAction, request.Overwrite)
	if action == "replace" && !confirmed {
		return op, ImportOutcome{}, &PayloadReviewError{Payload: payload, Reasons: []string{"replacement requires a current destination preview and explicit book confirmation"}}
	}
	replacementDirs := []string{}
	destinations := map[string]string{}
	groupRoots := map[string]string{}
	reservedDirectories := map[string]bool{}
	for _, id := range groupOrder {
		item, files := books[id], groups[id]
		first := files[0]
		if folder, ok := resolveImportRootFolder(s.nativeRootFolders(ctx), first.Format, item.RootFolderID); ok && folder.Calibre.Enabled {
			return op, ImportOutcome{}, &PayloadReviewError{Payload: payload, Reasons: []string{"Calibre-managed roots require a separate handoff; select a native root for a complete payload import"}}
		}
		root, err := canonicalPlannedPath(s.importRootPath(ctx, first.Format, item.RootFolderID))
		if err != nil {
			return op, ImportOutcome{}, err
		}
		parsed := parsedBookForPath(first.SourcePath)
		parsed.Title, parsed.AuthorName = firstNonEmpty(item.Title, parsed.Title), firstNonEmpty(item.AuthorName, parsed.AuthorName)
		parsed.Series = firstNonEmpty(wantedOverrideValue(item, "series"), item.Series, parsed.Series)
		parsed.SeriesPosition = firstNonEmpty(wantedOverrideValue(item, "series_position"), item.SeriesPosition, parsed.SeriesPosition)
		if item.FirstPublishYear > 0 {
			parsed.Year = fmt.Sprint(item.FirstPublishYear)
		}
		destination := s.importDestinationPath(root, first.Format, parsed, first.SourcePath)
		if !pathWithinRoot(destination, root) || pathWithinRoot(destination, payload.SourceRoot) {
			return op, ImportOutcome{}, errors.New("destination must be inside the library and outside download storage")
		}
		if op.WantedID == "" {
			op.WantedID, op.Format, op.DestinationRoot = id, first.Format, root
		}
		if root != op.DestinationRoot { // All group paths are still explicit and individually verified.
			op.DestinationRoot = commonPathRoot(op.DestinationRoot, root)
		}
		groupRoot := payload.Root
		for _, f := range files {
			if !pathWithinRoot(f.SourcePath, groupRoot) {
				return op, ImportOutcome{}, errors.New("payload group crossed its root")
			}
		}
		// Use the media group's own common parent inside a reviewed multi-book pack.
		if len(groups) > 1 {
			groupRoot = filepath.Dir(files[0].SourcePath)
			for _, f := range files {
				groupRoot = commonPathRoot(groupRoot, filepath.Dir(f.SourcePath))
			}
		}
		groupRoots[id] = groupRoot
		preserveNames := len(files) > 1 || len(sidecars) > 0
		if preserveNames {
			bookDir := filepath.Dir(destination)
			if !s.Config().RenameBooksEnabled() {
				bookDir = filepath.Join(bookDir, safePathSegment(item.Title))
			}
			bookDir, err = planBookDirectory(bookDir, normalizeConflictAction(request.ConflictAction, request.Overwrite), reservedDirectories)
			if err != nil {
				return op, ImportOutcome{}, err
			}
			destinations[id] = bookDir
			if action == "replace" {
				replacementDirs = append(replacementDirs, bookDir)
			}
			for _, f := range files {
				relative, _ := filepath.Rel(groupRoot, f.SourcePath)
				if err := appendPlannedFile(&op, f, filepath.Join(bookDir, relative), id); err != nil {
					return op, ImportOutcome{}, err
				}
			}
		} else {
			plan, err := planImportDestination(first.SourcePath, destination, normalizeConflictAction(request.ConflictAction, request.Overwrite))
			if err != nil {
				return op, ImportOutcome{}, err
			}
			if plan.Skipped {
				return op, ImportOutcome{Skipped: true, DestinationPath: plan.DestinationPath, Message: plan.Message}, nil
			}
			destinations[id] = filepath.Dir(plan.DestinationPath)
			if err := appendPlannedFile(&op, first, plan.DestinationPath, id); err != nil {
				return op, ImportOutcome{}, err
			}
		}
	}
	for _, f := range sidecars {
		// Attach to the most specific group directory. Equal candidates must be
		// assigned explicitly rather than copying a shared OPF into the wrong book.
		id := ""
		best := -1
		for _, candidate := range groupOrder {
			root := groupRoots[candidate]
			if pathWithinRoot(f.SourcePath, root) {
				if len(root) > best {
					id, best = candidate, len(root)
				} else if len(root) == best {
					id = ""
				}
			}
		}
		for _, m := range mappings {
			if m.RelativePath == f.RelativePath && m.WantedID != "" {
				if _, ok := groups[m.WantedID]; !ok {
					return op, ImportOutcome{}, errors.New("sidecar assignment has no media files")
				}
				id = m.WantedID
			}
		}
		if id == "" {
			return op, ImportOutcome{}, &PayloadReviewError{Payload: payload, Reasons: []string{f.RelativePath + ": assign this shared sidecar to a book or retain it in downloads"}}
		}
		relative, err := filepath.Rel(groupRoots[id], f.SourcePath)
		if err != nil || strings.HasPrefix(relative, "..") {
			relative = filepath.Base(f.SourcePath)
		}
		if err := appendPlannedFile(&op, f, filepath.Join(destinations[id], relative), id); err != nil {
			return op, ImportOutcome{}, err
		}
	}
	if action == "replace" {
		if err := s.planCompletedReplacements(ctx, &op, replacementDirs); err != nil {
			return op, ImportOutcome{}, &PayloadReviewError{Payload: payload, Reasons: []string{err.Error()}}
		}
	}
	op.Metadata = map[string]any{"conflictAction": action, "replacementDirectories": replacementDirs, "title": books[op.WantedID].Title, "author": books[op.WantedID].AuthorName, "payload": payload, "manualMapping": confirmed, "exclusions": exclusions, "mapping": mappings}
	return op, ImportOutcome{}, nil
}

func appendPlannedFile(op *ImportOperation, f PayloadFile, destination, wantedID string) error {
	entry, err := newManifestFile(f.SourcePath, destination, op.SourceRoot, f.Format, len(op.Files))
	if err != nil {
		return err
	}
	entry.WantedID = wantedID
	op.Files = append(op.Files, entry)
	return nil
}
func commonPathRoot(a, b string) string {
	for !pathWithinRoot(b, a) {
		a = filepath.Dir(a)
	}
	return a
}
func planBookDirectory(path, action string, reserved map[string]bool) (string, error) {
	available := func(candidate string) (bool, error) {
		if reserved[candidate] {
			return false, nil
		}
		_, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	if free, err := available(path); free {
		reserved[path] = true
		return path, nil
	} else if err != nil {
		return "", err
	}
	if action == "replace" {
		if reserved[path] {
			return "", errors.New("multiple books cannot replace the same directory")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path || !info.IsDir() {
			return "", errors.New("replacement requires an existing directory without symlinks")
		}
		reserved[path] = true
		return path, nil
	}
	if action != "rename" {
		return "", errors.New("book directory already exists; keep both preserves complete file sets")
	}
	for n := 2; n < 10000; n++ {
		candidate := fmt.Sprintf("%s (%d)", path, n)
		if free, err := available(candidate); free {
			reserved[candidate] = true
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("no free book directory")
}

func (s *Service) importCompletedPayload(ctx context.Context, download acquisition.DownloadStatus, payload DownloadPayload, request ImportRequest, mappings []PayloadMapping, confirmed bool) (ImportOutcome, error) {
	var calibreID string
	calibreErr := s.store.db.QueryRowContext(ctx, `select h.id::text from calibre_handoffs h join downloads d on d.id=h.download_record_id where lower(d.client)=lower($1) and d.external_id=$2`, download.Client, download.ID).Scan(&calibreID)
	if calibreErr == nil {
		return s.RetryCalibreHandoff(ctx, calibreID)
	}
	if !errors.Is(calibreErr, sql.ErrNoRows) {
		return ImportOutcome{}, calibreErr
	}
	existing, err := s.store.operationForDownload(ctx, download.Client, download.ID)
	if err == nil {
		return s.runImportOperation(ctx, existing)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ImportOutcome{}, err
	}
	// Preserve the established single-book Calibre handoff. A remote add earns
	// no native manifest or source-cleanup receipt; chapter sets need a native root.
	media := []PayloadFile{}
	for _, f := range payload.Files {
		if f.Format == "ebook" || f.Format == "audiobook" {
			media = append(media, f)
		}
	}
	if len(media) == 1 && request.WantedID != "" && len(mappings) == 0 {
		item, lookupErr := s.lookupWanted(ctx, request.WantedID)
		if lookupErr != nil {
			return ImportOutcome{}, lookupErr
		}
		if folder, ok := resolveImportRootFolder(s.nativeRootFolders(ctx), media[0].Format, item.RootFolderID); ok && folder.Calibre.Enabled {
			if reasons := s.payloadMatchReasons(ctx, payload, item); len(reasons) > 0 {
				return ImportOutcome{}, &PayloadReviewError{Payload: payload, Reasons: reasons}
			}
			request.SourcePath = media[0].SourcePath
			imported, importErr := s.Import(ctx, request)
			return imported, importErr
		}
	}
	op, outcome, err := s.planPayload(ctx, payload, request, mappings, confirmed)
	if err != nil || outcome.Skipped {
		return outcome, err
	}
	op, err = s.store.planOperation(ctx, op)
	if err != nil {
		return ImportOutcome{}, err
	}
	return s.runImportOperation(ctx, op)
}
