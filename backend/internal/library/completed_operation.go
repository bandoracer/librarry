package library

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
		outcome, err := s.committedOperationOutcome(ctx, op)
		if err != nil {
			return outcome, err
		}
		return s.finishManualOperation(ctx, op, outcome)
	}
	token, err := s.store.claimOperation(ctx, op.ID)
	if err != nil {
		return outcome, err
	}
	// Read the file journal after claiming. A prior worker may have recorded its
	// stage between the caller's snapshot and this successful claim.
	op, err = s.store.getOperation(ctx, op.ID)
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
	if err := s.verifyOperationInventory(runCtx, op, false); err != nil {
		return outcome, err
	}
	records := make([]FileRecord, 0, len(op.Files))
	for _, f := range op.Files {
		if err := runCtx.Err(); err != nil {
			return outcome, err
		}
		if !pathWithinRoot(f.SourcePath, op.SourceRoot) || !pathWithinRoot(f.DestinationPath, op.DestinationRoot) || (op.SourceKind != "manual" && pathWithinRoot(f.DestinationPath, op.SourceRoot)) {
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
		if err := s.reclaimImportStage(runCtx, op, f); err != nil {
			return outcome, err
		}
		published := verifyManifestPath(f.DestinationPath, f) == nil
		if f.PreviousPath != "" && verifyPreviousImportFile(f) != nil {
			published = false
		}
		if !published {
			if f.PreviousPath == "" {
				if _, err := os.Lstat(f.DestinationPath); err == nil {
					return outcome, errors.New("destination differs from the saved import plan")
				} else if !errors.Is(err, os.ErrNotExist) {
					return outcome, err
				}
			}
			if err := s.transferOperationFile(runCtx, op, f); err != nil {
				return outcome, err
			}
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
		record.Metadata["wantedId"], record.Metadata["downloadId"], record.Metadata["downloadClient"] = f.WantedID, op.DownloadID, op.Client
		if f.PreviousPath != "" {
			record.Metadata["previousPath"] = f.PreviousPath
			record.Metadata["replacedExisting"] = true
		}
		record.Metadata["requestedImportMode"] = op.Mode
		record.Metadata["move"] = false
		sourceInfo, statErr := os.Stat(f.SourcePath)
		if statErr != nil {
			return outcome, statErr
		}
		record.Metadata["hardlinked"] = os.SameFile(sourceInfo, info)
		if value, ok := op.Metadata["conflictAction"]; ok {
			record.Metadata["conflictAction"] = value
		}
		if value, ok := op.Metadata["conflictPath"]; ok {
			record.Metadata["conflictPath"] = value
		}
		record.Metadata["importOperationId"] = op.ID
		record.Metadata["importMode"] = op.Mode
		record.Metadata["verifiedDownload"] = nil
		if op.SourceKind != "manual" {
			record.Metadata["verifiedDownload"] = map[string]any{"client": op.Client, "id": op.DownloadID, "sha256": f.SHA256}
		}
		record.Metadata["importedAt"] = time.Now().UTC().Format(time.RFC3339)
		records = append(records, record)
	}
	if err := s.verifyOperationInventory(runCtx, op, false); err != nil {
		return outcome, err
	}
	// Re-check the whole set at the visibility boundary, including sources that a
	// still-running download client may have modified during transfer.
	for _, f := range op.Files {
		if f.PreviousPath != "" {
			if err := verifyPreviousImportFile(f); err != nil {
				return outcome, err
			}
		}
		for _, path := range []string{f.SourcePath, f.DestinationPath} {
			if err := verifyManifestPath(path, f); err != nil {
				return outcome, err
			}
		}
	}
	if err := s.store.renewOperation(runCtx, op.ID, token); err != nil {
		return outcome, err
	}
	for i := range records {
		id, _ := records[i].Metadata["wantedId"].(string)
		if id == "" && op.SourceKind == "manual" {
			continue
		}
		current, err := s.lookupWanted(runCtx, id)
		if err != nil {
			return outcome, err
		}
		if current.Format != "" && current.Format != "any" && normalizeFormat(current.Format) != records[i].MediaFormat {
			return outcome, errors.New("wanted format changed; import needs review")
		}
		records[i].Title = firstNonEmpty(current.Title, records[i].Title)
		records[i].AuthorName = firstNonEmpty(current.AuthorName, records[i].AuthorName)
	}
	records, err = s.store.commitOperation(runCtx, op, records)
	if err != nil {
		return outcome, err
	}
	outcome = ImportOutcome{File: records[0], Files: records, OperationID: op.ID, DestinationPath: records[0].Path, Imported: true, ImportMode: op.Mode}
	if op.SourceKind == "manual" {
		cancel()
		<-done
		op, err = s.store.getOperation(ctx, op.ID)
		if err != nil {
			return outcome, err
		}
		return s.finishManualOperation(ctx, op, outcome)
	}
	return outcome, nil
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
	byID := make(map[string]FileRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	for i, id := range ids {
		records[i] = byID[id]
	}
	return ImportOutcome{File: records[0], Files: records, OperationID: op.ID, DestinationPath: records[0].Path, Imported: true, Skipped: true, ImportMode: op.Mode, Message: "verified import already committed"}, nil
}
