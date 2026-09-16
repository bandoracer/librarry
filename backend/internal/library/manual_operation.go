package library

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func manualRequestScope(request ImportRequest) string {
	if request.originalScope != "" {
		return request.originalScope
	}
	value := struct{ Source, Wanted, Download, Format, Mode, Conflict string }{request.SourcePath, strings.TrimSpace(request.WantedID), strings.TrimSpace(request.DownloadID), strings.TrimSpace(request.Format), normalizeImportMode(request.ImportMode, request.Move), normalizeConflictAction(request.ConflictAction, request.Overwrite)}
	raw, _ := json.Marshal(value)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func (s *Store) manualOperationForKey(ctx context.Context, key string) (ImportOperation, error) {
	var id string
	if err := s.db.QueryRowContext(ctx, `select id::text from import_operations where source_kind='manual' and request_key=$1`, key).Scan(&id); err != nil {
		return ImportOperation{}, err
	}
	return s.getOperation(ctx, id)
}

// An unfinished request resumes its original destinations even if settings have
// changed. A committed move can be retried after its source has been removed.
func (s *Service) resumeManualRequest(ctx context.Context, request ImportRequest) (ImportOutcome, bool, error) {
	var id string
	missing := false
	if _, err := os.Lstat(request.SourcePath); errors.Is(err, os.ErrNotExist) {
		missing = true
	}
	err := s.store.db.QueryRowContext(ctx, `select id::text from import_operations where source_kind='manual' and metadata->>'requestScope'=$1 and (state<>'committed' or $2) order by (state<>'committed') desc,created_at desc,id limit 1`, manualRequestScope(request), missing).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ImportOutcome{}, false, nil
	}
	if err != nil {
		return ImportOutcome{}, true, err
	}
	op, err := s.store.getOperation(ctx, id)
	if err != nil {
		return ImportOutcome{}, true, err
	}
	outcome, err := s.runImportOperation(ctx, op)
	return outcome, true, err
}

func (s *Service) importNativeManual(ctx context.Context, request ImportRequest, source, format string, parsed parsedBook, root string) (ImportOutcome, error) {
	root, err := canonicalPlannedPath(root)
	if err != nil {
		return ImportOutcome{}, err
	}
	sourceRoot := filepath.Dir(source)
	sources := []string{source}
	extras, err := siblingExtraFiles(source, importExtraExtensions(s.Config().ImportExtraFiles))
	if err != nil {
		return ImportOutcome{}, err
	}
	sources = append(sources, extras...)
	op := ImportOperation{SourceKind: "manual", WantedID: strings.TrimSpace(request.WantedID), SourceRoot: sourceRoot, DestinationRoot: root, Format: format, Mode: normalizeImportMode(request.ImportMode, request.Move), Metadata: map[string]any{"title": parsed.Title, "author": parsed.AuthorName, "requestScope": manualRequestScope(request), "request": request}, Files: []ImportOperationFile{}}
	for i, path := range sources {
		mediaFormat := format
		if i > 0 {
			mediaFormat = "sidecar"
		}
		file, err := newManifestFile(path, "", sourceRoot, mediaFormat, i)
		if err != nil {
			return ImportOutcome{}, err
		}
		if i == 0 {
			file.WantedID = op.WantedID
		}
		op.Files = append(op.Files, file)
	}
	raw, err := json.Marshal(struct {
		Scope string
		Files []ImportOperationFile
	}{manualRequestScope(request), op.Files})
	if err != nil {
		return ImportOutcome{}, err
	}
	hash := sha256.Sum256(raw)
	op.RequestKey = hex.EncodeToString(hash[:])
	if existing, err := s.store.manualOperationForKey(ctx, op.RequestKey); err == nil {
		return s.runImportOperation(ctx, existing)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ImportOutcome{}, err
	}
	destination := s.importDestinationPath(root, format, parsed, source)
	for _, extra := range extras {
		switch strings.ToLower(filepath.Ext(extra)) {
		case ".cue", ".m3u", ".m3u8":
			destination = filepath.Join(filepath.Dir(destination), filepath.Base(source))
		}
	}
	if !pathWithinRoot(destination, root) {
		return ImportOutcome{}, errors.New("manual destination is outside the configured library")
	}
	action := normalizeConflictAction(request.ConflictAction, request.Overwrite)
	plan, err := planImportDestination(source, destination, action)
	if err != nil {
		return ImportOutcome{}, err
	}
	if plan.Skipped {
		return ImportOutcome{Skipped: true, DestinationPath: plan.DestinationPath, ImportMode: op.Mode, ConflictAction: action, ConflictPath: plan.ConflictPath, Message: plan.Message}, nil
	}
	destination = plan.DestinationPath
	// Replacing/adopting a tracked file without a new book assignment preserves
	// its existing canonical identity and manual title/author corrections.
	if request.WantedID == "" {
		existing, err := s.store.FindFiles(ctx, nil, []string{destination})
		if err != nil {
			return ImportOutcome{}, err
		}
		if len(existing) == 1 {
			op.Metadata["title"] = firstNonEmpty(existing[0].Title, parsed.Title)
			op.Metadata["author"] = firstNonEmpty(existing[0].AuthorName, parsed.AuthorName)
			var id string
			var count int
			err := s.store.db.QueryRowContext(ctx, `select count(*),coalesce(min(wanted_item_id::text),'') from file_wanted_links where file_id=$1`, existing[0].ID).Scan(&count, &id)
			if err != nil {
				return ImportOutcome{}, err
			}
			if count > 1 {
				return ImportOutcome{}, errors.New("existing file belongs to multiple books; select a book explicitly")
			}
			if id != "" {
				item, err := s.lookupWanted(ctx, id)
				if err != nil {
					return ImportOutcome{}, err
				}
				if item.Status == "removed" || item.Status == "ignored" || (item.Format != "" && item.Format != "any" && normalizeFormat(item.Format) != format) {
					return ImportOutcome{}, errors.New("existing file belongs to an ineligible book; select a book explicitly")
				}
				op.WantedID = id
				op.Files[0].WantedID = id
			}
		}
	}
	op.Metadata["conflictAction"], op.Metadata["conflictPath"] = action, plan.ConflictPath
	for i := range op.Files {
		target := destination
		if i > 0 {
			target = strings.TrimSuffix(destination, filepath.Ext(destination)) + strings.ToLower(filepath.Ext(op.Files[i].SourcePath))
		}
		file := &op.Files[i]
		file.DestinationPath = target
		if file.SourcePath == target {
			continue
		}
		if _, err := os.Lstat(target); err == nil {
			if action != "replace" {
				return ImportOutcome{}, fmt.Errorf("sidecar or destination already exists: %s; choose replace or another destination", target)
			}
			info, err := os.Lstat(target)
			if err != nil || !info.Mode().IsRegular() {
				return ImportOutcome{}, errors.New("replacement must name an existing regular file")
			}
			resolved, err := filepath.EvalSymlinks(target)
			if err != nil || resolved != target {
				return ImportOutcome{}, errors.New("replacement destination crosses a symlink")
			}
			previousHash, err := contentHash(target)
			if err != nil {
				return ImportOutcome{}, err
			}
			file.PreviousPath = filepath.Join(filepath.Dir(target), ".librarry-previous-"+rand.Text())
			file.PreviousSHA256, file.PreviousSizeBytes = previousHash, info.Size()
		} else if !errors.Is(err, os.ErrNotExist) {
			return ImportOutcome{}, err
		}
	}
	op, err = s.store.planOperation(ctx, op)
	if err != nil {
		return ImportOutcome{}, err
	}
	return s.runImportOperation(ctx, op)
}
