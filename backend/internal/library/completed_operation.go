package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

// importCompletedFile persists the immutable source/destination/hash plan before
// transferring bytes. A retry uses that plan even if naming settings change.
func (s *Service) importCompletedFile(ctx context.Context, download acquisition.DownloadStatus, request ImportRequest) (ImportOutcome, error) {
	op, err := s.store.operationForDownload(ctx, download.Client, download.ID)
	if err == nil {
		return s.runImportOperation(ctx, op)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ImportOutcome{}, err
	}
	item, err := s.lookupWanted(ctx, request.WantedID)
	if err != nil {
		return ImportOutcome{}, err
	}
	if item.Status == "removed" || item.Status == "ignored" {
		return ImportOutcome{}, errors.New("book is no longer eligible for import")
	}
	source := filepath.Clean(request.SourcePath)
	format, ok := classifyFile(source)
	if !ok {
		return ImportOutcome{}, errors.New("unsupported source format")
	}
	if item.Format != "" && item.Format != "any" && normalizeFormat(item.Format) != format {
		return ImportOutcome{}, errors.New("source format conflicts with the wanted book")
	}
	// Remote Calibre handoff retains its existing operator flow. It cannot earn
	// local filesystem verification or cleanup eligibility from a remote add alone.
	if folder, ok := resolveImportRootFolder(s.nativeRootFolders(ctx), format, item.RootFolderID); ok && folder.Calibre.Enabled {
		imported, err := s.Import(ctx, request)
		if err == nil && imported.Imported && s.downloads != nil {
			err = s.downloads.MarkDownloadImported(ctx, download.ID, imported.File.ID)
		}
		return imported, err
	}
	parsed := parsedBookForPath(source)
	parsed.Title = firstNonEmpty(item.Title, parsed.Title)
	parsed.AuthorName = firstNonEmpty(item.AuthorName, parsed.AuthorName)
	parsed.Series = firstNonEmpty(wantedOverrideValue(item, "series"), item.Series, parsed.Series)
	parsed.SeriesPosition = firstNonEmpty(wantedOverrideValue(item, "series_position"), item.SeriesPosition, parsed.SeriesPosition)
	if item.FirstPublishYear > 0 {
		parsed.Year = strconv.Itoa(item.FirstPublishYear)
	}
	root := s.importRootPath(ctx, format, item.RootFolderID)
	if root == "" || !filepath.IsAbs(root) {
		return ImportOutcome{}, errors.New("an absolute library root is required")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return ImportOutcome{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return ImportOutcome{}, err
	}
	destination := s.importDestinationPath(root, format, parsed, source)
	plan, err := planImportDestination(source, destination, normalizeConflictAction(request.ConflictAction, request.Overwrite))
	if err != nil {
		return ImportOutcome{}, err
	}
	if plan.Skipped {
		return ImportOutcome{Skipped: true, DestinationPath: plan.DestinationPath, Message: plan.Message}, nil
	}
	if plan.Replaced {
		return ImportOutcome{}, errors.New("completed import replacement requires review; use keep both to preserve the existing book")
	}
	sourceRoot, err := filepath.EvalSymlinks(download.SavePath)
	if err != nil {
		return ImportOutcome{}, err
	}
	if !pathWithinRoot(source, sourceRoot) || pathWithinRoot(plan.DestinationPath, sourceRoot) {
		return ImportOutcome{}, errors.New("import destination must be outside the download deletion tree")
	}
	entry, err := newManifestFile(source, plan.DestinationPath, sourceRoot, format, 0)
	if err != nil {
		return ImportOutcome{}, err
	}
	op = ImportOperation{Client: download.Client, DownloadID: download.ID, WantedID: request.WantedID,
		SourceRoot: sourceRoot, DestinationRoot: root, Format: format, Mode: normalizeImportMode(request.ImportMode, false),
		Metadata: map[string]any{"title": parsed.Title, "author": parsed.AuthorName, "conflictAction": plan.ConflictAction}, Files: []ImportOperationFile{entry}}
	// Same-basename configured sidecars are part of this operation only when the
	// download has its own payload directory. Never consume siblings of a loose file.
	payload := filepath.Join(sourceRoot, filepath.Base(download.Name))
	if info, err := os.Lstat(payload); err == nil && info.IsDir() && pathWithinRoot(source, payload) {
		extras, err := siblingExtraFiles(source, importExtraExtensions(s.Config().ImportExtraFiles))
		if err != nil {
			return ImportOutcome{}, err
		}
		for _, extra := range extras {
			target := strings.TrimSuffix(plan.DestinationPath, filepath.Ext(plan.DestinationPath)) + strings.ToLower(filepath.Ext(extra))
			f, err := newManifestFile(extra, target, sourceRoot, "sidecar", len(op.Files))
			if err != nil {
				return ImportOutcome{}, err
			}
			op.Files = append(op.Files, f)
		}
	}
	op, err = s.store.planOperation(ctx, op)
	if err != nil {
		return ImportOutcome{}, err
	}
	return s.runImportOperation(ctx, op)
}

func newManifestFile(source, destination, root, format string, order int) (ImportOperationFile, error) {
	var f ImportOperationFile
	info, err := os.Lstat(source)
	if err != nil {
		return f, err
	}
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil || resolved != source || !info.Mode().IsRegular() || info.Size() <= 0 {
		return f, errors.New("import source must be a nonempty regular file without symlinks")
	}
	relative, err := filepath.Rel(root, source)
	if err != nil || !pathWithinRoot(source, root) {
		return f, errors.New("source is outside download root")
	}
	hash, err := contentHash(source)
	if err != nil {
		return f, err
	}
	return ImportOperationFile{Order: order, RelativePath: relative, SourcePath: source, DestinationPath: destination,
		SizeBytes: info.Size(), SHA256: hash, Format: format, Required: true}, nil
}

func verifyManifestPath(path string, file ImportOperationFile) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path || !info.Mode().IsRegular() || info.Size() != file.SizeBytes {
		return fmt.Errorf("manifest path or size changed: %s", path)
	}
	hash, err := contentHash(path)
	if err != nil {
		return err
	}
	if hash != file.SHA256 {
		return fmt.Errorf("manifest checksum changed: %s", path)
	}
	return nil
}

func (s *Service) runImportOperation(ctx context.Context, op ImportOperation) (outcome ImportOutcome, resultErr error) {
	if op.State == "committed" {
		return s.committedOperationOutcome(ctx, op)
	}
	token, err := s.store.claimOperation(ctx, op.ID)
	if err != nil {
		return outcome, err
	}
	op.LeaseToken = token
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := s.store.renewOperation(runCtx, op.ID, token); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		<-done
		if resultErr != nil {
			persistCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer stop()
			s.store.failOperation(persistCtx, op.ID, token, resultErr.Error())
		}
	}()
	records := make([]FileRecord, 0, len(op.Files))
	for _, f := range op.Files {
		if err := runCtx.Err(); err != nil {
			return outcome, err
		}
		if !pathWithinRoot(f.SourcePath, op.SourceRoot) || !pathWithinRoot(f.DestinationPath, op.DestinationRoot) || pathWithinRoot(f.DestinationPath, op.SourceRoot) {
			return outcome, errors.New("manifest paths cross the import boundary")
		}
		if err := verifyManifestPath(f.SourcePath, f); err != nil {
			return outcome, err
		}
		if err := os.MkdirAll(filepath.Dir(f.DestinationPath), 0755); err != nil {
			return outcome, err
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(f.DestinationPath))
		if err != nil || parent != filepath.Dir(f.DestinationPath) {
			return outcome, errors.New("destination directory contains a symlink")
		}
		if _, err := os.Lstat(f.DestinationPath); errors.Is(err, os.ErrNotExist) {
			// Exclusive publication never overwrites another file. If the process dies
			// after publication, the next attempt verifies and reuses these exact bytes.
			if err := s.store.renewOperation(runCtx, op.ID, token); err != nil {
				return outcome, err
			}
			if _, err := importFile(f.SourcePath, f.DestinationPath, op.Mode, false); err != nil {
				return outcome, err
			}
			dir, err := os.Open(filepath.Dir(f.DestinationPath))
			if err != nil {
				return outcome, err
			}
			syncErr := dir.Sync()
			closeErr := dir.Close()
			if syncErr != nil {
				return outcome, syncErr
			}
			if closeErr != nil {
				return outcome, closeErr
			}
		} else if err != nil {
			return outcome, err
		}
		if err := verifyManifestPath(f.DestinationPath, f); err != nil {
			return outcome, err
		}
		durable, err := os.Open(f.DestinationPath)
		if err != nil {
			return outcome, err
		}
		syncErr := durable.Sync()
		closeErr := durable.Close()
		if syncErr != nil {
			return outcome, syncErr
		}
		if closeErr != nil {
			return outcome, closeErr
		}
		if err := s.store.markOperationFile(runCtx, op, f.ID); err != nil {
			return outcome, err
		}
		if f.Format == "sidecar" {
			continue
		}
		info, err := os.Stat(f.DestinationPath)
		if err != nil {
			return outcome, err
		}
		record := fileRecordFromPath(f.DestinationPath, f.Format, info, "imported")
		record.SourcePath, record.Checksum = f.SourcePath, f.SHA256
		record.Title, _ = op.Metadata["title"].(string)
		record.AuthorName, _ = op.Metadata["author"].(string)
		record.Metadata["wantedId"], record.Metadata["downloadId"], record.Metadata["downloadClient"] = op.WantedID, op.DownloadID, op.Client
		record.Metadata["importOperationId"] = op.ID
		record.Metadata["importMode"] = op.Mode
		record.Metadata["verifiedDownload"] = map[string]any{"client": op.Client, "id": op.DownloadID, "sha256": f.SHA256}
		record.Metadata["importedAt"] = time.Now().UTC().Format(time.RFC3339)
		records = append(records, record)
	}
	// Re-check the whole set at the visibility boundary, including sources that a
	// still-running download client may have modified during transfer.
	for _, f := range op.Files {
		for _, path := range []string{f.SourcePath, f.DestinationPath} {
			if err := verifyManifestPath(path, f); err != nil {
				return outcome, err
			}
		}
	}
	if err := s.store.renewOperation(runCtx, op.ID, token); err != nil {
		return outcome, err
	}
	current, err := s.lookupWanted(runCtx, op.WantedID)
	if err != nil {
		return outcome, err
	}
	if current.Format != "" && current.Format != "any" && normalizeFormat(current.Format) != op.Format {
		return outcome, errors.New("wanted format changed; import needs review")
	}
	for i := range records {
		records[i].Title = firstNonEmpty(current.Title, records[i].Title)
		records[i].AuthorName = firstNonEmpty(current.AuthorName, records[i].AuthorName)
	}
	records, err = s.store.commitOperation(runCtx, op, records)
	if err != nil {
		return outcome, err
	}
	return ImportOutcome{File: records[0], Files: records, OperationID: op.ID, DestinationPath: records[0].Path, Imported: true, ImportMode: op.Mode}, nil
}

func (s *Service) committedOperationOutcome(ctx context.Context, op ImportOperation) (ImportOutcome, error) {
	ids := []string{}
	for _, f := range op.Files {
		if err := verifyManifestPath(f.DestinationPath, f); err != nil {
			return ImportOutcome{}, err
		}
		if f.Format != "sidecar" {
			if f.FileID == "" {
				return ImportOutcome{}, errors.New("committed import lost its library record")
			}
			ids = append(ids, f.FileID)
		}
	}
	records, err := s.store.FindFiles(ctx, ids, nil)
	if err != nil {
		return ImportOutcome{}, err
	}
	if len(records) == 0 || len(records) != len(ids) {
		return ImportOutcome{}, errors.New("committed import files are missing")
	}
	return ImportOutcome{File: records[0], Files: records, OperationID: op.ID, DestinationPath: records[0].Path, Imported: true, Skipped: true, ImportMode: op.Mode, Message: "verified import already committed"}, nil
}
