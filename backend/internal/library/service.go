package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/calibre"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type WantedStore interface {
	GetWanted(ctx context.Context, id string) (wanted.WantedItem, error)
	ListWanted(ctx context.Context, status string) ([]wanted.WantedItem, error)
	MarkWantedStatus(ctx context.Context, wantedID string, status string) error
}

type DownloadStore interface {
	MarkDownloadImported(ctx context.Context, id string, fileID string) error
	MarkDownloadImportError(ctx context.Context, id string, message string) error
}

type RootFolderProvider interface {
	ListRootFolders(ctx context.Context) ([]compatdata.RootFolder, error)
}

type importFileOperation struct {
	Mode       string
	Moved      bool
	Hardlinked bool
}

type importDestinationPlan struct {
	DestinationPath string
	ConflictPath    string
	ConflictAction  string
	Replaced        bool
	Skipped         bool
	Message         string
}

type importReviewWantedCandidate struct {
	WantedID      string   `json:"wantedId"`
	Title         string   `json:"title,omitempty"`
	AuthorName    string   `json:"authorName,omitempty"`
	Format        string   `json:"format,omitempty"`
	Status        string   `json:"status,omitempty"`
	Score         float64  `json:"score"`
	MatchedFields []string `json:"matchedFields,omitempty"`
	Reason        string   `json:"reason,omitempty"`
}

type Service struct {
	store       *Store
	config      Config
	wanted      WantedStore
	downloads   DownloadStore
	calibre     calibre.Manager
	rootFolders RootFolderProvider
}

func NewService(store *Store, config Config, wanted WantedStore, downloads DownloadStore) *Service {
	return &Service{store: store, config: config, wanted: wanted, downloads: downloads}
}

func (s *Service) WithCalibre(manager calibre.Manager, rootFolders RootFolderProvider) *Service {
	if s == nil {
		return s
	}
	s.calibre = manager
	s.rootFolders = rootFolders
	return s
}

func (s *Service) Available() bool {
	return s != nil && s.store != nil && s.store.Configured()
}

func (s *Service) ListFiles(ctx context.Context, query FileListQuery) ([]FileRecord, error) {
	if !s.Available() {
		return nil, errors.New("library service requires database persistence")
	}
	return s.store.ListFiles(ctx, query)
}

func (s *Service) UpdateFile(ctx context.Context, file FileRecord) (FileRecord, error) {
	if !s.Available() {
		return FileRecord{}, errors.New("library service requires database persistence")
	}
	return s.store.UpdateFile(ctx, file)
}

func (s *Service) DeleteFiles(ctx context.Context, request DeleteFilesRequest) (DeleteFilesOutcome, error) {
	if !s.Available() {
		return DeleteFilesOutcome{}, errors.New("library service requires database persistence")
	}
	ids := compactStrings(request.IDs)
	paths := compactStrings(request.Paths)
	if len(ids) == 0 && len(paths) == 0 {
		return DeleteFilesOutcome{}, errors.New("at least one file id or path is required")
	}
	outcome := DeleteFilesOutcome{Requested: len(ids) + len(paths)}
	candidates, err := s.store.FindFiles(ctx, ids, paths)
	if err != nil {
		return DeleteFilesOutcome{}, err
	}
	if len(candidates) == 0 {
		outcome.Skipped = outcome.Requested
		return outcome, nil
	}

	deleteIDs := make([]string, 0, len(candidates))
	for _, file := range candidates {
		if request.DeleteFiles {
			if err := s.applyCalibreDelete(ctx, file); err != nil {
				outcome.Errored++
				outcome.Results = append(outcome.Results, DeleteFileResult{File: file, Status: "error", Message: err.Error()})
				continue
			}
			if err := removeLibraryFile(file.Path); err != nil {
				outcome.Errored++
				outcome.Results = append(outcome.Results, DeleteFileResult{File: file, Status: "error", Message: err.Error()})
				continue
			}
		}
		deleteIDs = append(deleteIDs, file.ID)
	}
	if len(deleteIDs) == 0 {
		return outcome, nil
	}
	deleted, err := s.store.DeleteFiles(ctx, deleteIDs, nil)
	if err != nil {
		return outcome, err
	}
	outcome.Deleted = len(deleted)
	outcome.Files = deleted
	for _, file := range deleted {
		outcome.Results = append(outcome.Results, DeleteFileResult{File: file, Status: "deleted"})
	}
	if unmatched := outcome.Requested - outcome.Deleted - outcome.Errored; unmatched > 0 {
		outcome.Skipped = unmatched
	}
	return outcome, nil
}

func (s *Service) PreviewRenameFiles(ctx context.Context, request RenameFilesRequest) (RenameFilesOutcome, error) {
	return s.renameFiles(ctx, request, false)
}

func (s *Service) RenameFiles(ctx context.Context, request RenameFilesRequest) (RenameFilesOutcome, error) {
	return s.renameFiles(ctx, request, true)
}

func (s *Service) RefreshCalibreConversions(ctx context.Context, request CalibreConversionRefreshRequest) (CalibreConversionRefreshOutcome, error) {
	if !s.Available() {
		return CalibreConversionRefreshOutcome{}, errors.New("library service requires database persistence")
	}
	if s.calibre == nil || s.rootFolders == nil {
		return CalibreConversionRefreshOutcome{}, errors.New("calibre integration is unavailable")
	}
	ids := compactStrings(request.IDs)
	paths := compactStrings(request.Paths)
	limit := request.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var files []FileRecord
	var err error
	if len(ids) > 0 || len(paths) > 0 {
		files, err = s.store.FindFiles(ctx, ids, paths)
	} else {
		files, err = s.store.ListFiles(ctx, FileListQuery{Limit: limit})
	}
	if err != nil {
		return CalibreConversionRefreshOutcome{}, err
	}
	outcome := CalibreConversionRefreshOutcome{}
	for _, file := range files {
		if outcome.Checked >= limit {
			break
		}
		outcome.Checked++
		result := CalibreConversionRefreshResult{File: file}
		jobs := calibreConversionJobsFromMetadata(file.Metadata)
		if len(jobs) == 0 {
			result.Status = "skipped"
			result.Message = "file has no Calibre conversion jobs"
			outcome.Skipped++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		currentStatuses := calibreConversionStatusesFromMetadata(file.Metadata)
		if !request.Force && !calibreConversionNeedsRefresh(jobs, currentStatuses) {
			result.Status = "skipped"
			result.Message = "Calibre conversion jobs already terminal"
			result.Statuses = calibreConversionStatusMetadata(currentStatuses)
			outcome.Skipped++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		settings, ok, err := s.calibreSettingsForDestination(ctx, file.Path)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			outcome.Errored++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		if !ok {
			result.Status = "skipped"
			result.Message = "file is not under a Calibre-enabled root"
			outcome.Skipped++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		statuses, err := s.calibre.PollConversions(ctx, calibre.PollConversionsRequest{
			Settings:    settings,
			Jobs:        jobs,
			MaxAttempts: request.MaxAttempts,
			Interval:    time.Duration(request.IntervalMillis) * time.Millisecond,
		})
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			outcome.Errored++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		if file.Metadata == nil {
			file.Metadata = map[string]any{}
		}
		statusRecords := calibreConversionStatusMetadata(statuses)
		file.Metadata["calibreConversionStatuses"] = statusRecords
		file.Metadata["calibreConversionPolledAt"] = time.Now().UTC().Format(time.RFC3339)
		if len(statuses) > 0 && !calibreConversionNeedsRefresh(jobs, statuses) {
			if calibreConversionAnyFailed(statuses) {
				file.Metadata["calibreConversionFailedAt"] = time.Now().UTC().Format(time.RFC3339)
			} else {
				file.Metadata["calibreConversionCompletedAt"] = time.Now().UTC().Format(time.RFC3339)
			}
		}
		updated, err := s.store.UpdateFile(ctx, file)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			outcome.Errored++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		result.File = updated
		result.Status = "refreshed"
		result.Statuses = statusRecords
		outcome.Refreshed++
		outcome.Results = append(outcome.Results, result)
	}
	return outcome, nil
}

func (s *Service) renameFiles(ctx context.Context, request RenameFilesRequest, apply bool) (RenameFilesOutcome, error) {
	if !s.Available() {
		return RenameFilesOutcome{}, errors.New("library service requires database persistence")
	}
	ids := compactStrings(request.IDs)
	paths := compactStrings(request.Paths)
	if len(ids) == 0 && len(paths) == 0 {
		return RenameFilesOutcome{}, errors.New("at least one file id or path is required")
	}
	outcome := RenameFilesOutcome{Requested: len(ids) + len(paths)}
	files, err := s.store.FindFiles(ctx, ids, paths)
	if err != nil {
		return RenameFilesOutcome{}, err
	}
	if len(files) == 0 {
		outcome.Skipped = outcome.Requested
		return outcome, nil
	}
	for _, file := range files {
		preview, err := s.renamePreviewForFile(file, request.Overwrite)
		if err != nil {
			outcome.Errored++
			outcome.Results = append(outcome.Results, RenameFileResult{Preview: RenameFilePreview{File: file, SourcePath: file.Path}, Status: "error", Message: err.Error()})
			continue
		}
		outcome.Previews = append(outcome.Previews, preview)
		if !apply {
			if preview.Noop {
				outcome.Skipped++
			}
			continue
		}
		if preview.Noop {
			outcome.Skipped++
			outcome.Results = append(outcome.Results, RenameFileResult{Preview: preview, Status: "skipped", Message: "file already matches naming template"})
			continue
		}
		renamed, err := s.applyRename(ctx, preview)
		if err != nil {
			outcome.Errored++
			outcome.Results = append(outcome.Results, RenameFileResult{Preview: preview, Status: "error", Message: err.Error()})
			continue
		}
		outcome.Renamed++
		outcome.Results = append(outcome.Results, RenameFileResult{Preview: preview, File: &renamed, Status: "renamed"})
	}
	if unmatched := outcome.Requested - len(files); unmatched > 0 {
		outcome.Skipped += unmatched
	}
	return outcome, nil
}

func (s *Service) ListImportReviews(ctx context.Context, query ReviewListQuery) ([]ImportReview, error) {
	if !s.Available() {
		return nil, errors.New("library service requires database persistence")
	}
	return s.store.ListImportReviews(ctx, query)
}

func (s *Service) Scan(ctx context.Context, request ScanRequest) (ScanOutcome, error) {
	if !s.Available() {
		return ScanOutcome{}, errors.New("library service requires database persistence")
	}
	roots := s.scanRoots(request)
	outcome := ScanOutcome{Roots: roots}
	limit := request.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				outcome.Errors = append(outcome.Errors, err.Error())
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if outcome.Scanned >= limit {
				return filepath.SkipAll
			}
			format, ok := classifyFile(path)
			if !ok || !formatAllowed(request.Format, format) {
				outcome.Skipped++
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				outcome.Errors = append(outcome.Errors, err.Error())
				return nil
			}
			record := fileRecordFromPath(path, format, info, "available")
			stored, err := s.store.UpsertFile(ctx, record)
			if err != nil {
				outcome.Errors = append(outcome.Errors, err.Error())
				return nil
			}
			outcome.Scanned++
			outcome.Upserted++
			outcome.Files = append(outcome.Files, stored)
			return nil
		})
		if walkErr != nil {
			outcome.Errors = append(outcome.Errors, walkErr.Error())
		}
	}
	return outcome, nil
}

func (s *Service) Import(ctx context.Context, request ImportRequest) (ImportOutcome, error) {
	if !s.Available() {
		return ImportOutcome{}, errors.New("library service requires database persistence")
	}
	source := filepath.Clean(strings.TrimSpace(request.SourcePath))
	if source == "." || source == "" {
		return ImportOutcome{}, errors.New("source path is required")
	}
	info, err := os.Stat(source)
	if err != nil {
		return ImportOutcome{}, err
	}
	if info.IsDir() {
		return ImportOutcome{}, errors.New("source path must be a file")
	}
	format, ok := classifyFile(source)
	if !ok {
		return ImportOutcome{}, errors.New("source file extension is not supported")
	}
	if strings.TrimSpace(request.Format) != "" && strings.TrimSpace(request.Format) != "any" {
		format = normalizeFormat(request.Format)
	}

	parsed := parsedBookForPath(source)
	if strings.TrimSpace(request.WantedID) != "" {
		item, err := s.lookupWanted(ctx, request.WantedID)
		if err != nil {
			return ImportOutcome{}, err
		}
		if strings.TrimSpace(item.Format) != "" {
			format = normalizeFormat(item.Format)
		}
		parsed.Title = firstNonEmpty(item.Title, parsed.Title)
		parsed.AuthorName = firstNonEmpty(item.AuthorName, parsed.AuthorName)
	}

	destination := s.destinationPath(format, parsed, filepath.Ext(source))
	if destination == "" {
		return ImportOutcome{}, errors.New("library root is not configured")
	}
	mode := normalizeImportMode(request.ImportMode, request.Move)
	conflictAction := normalizeConflictAction(request.ConflictAction, request.Overwrite)
	plan, err := planImportDestination(source, destination, conflictAction)
	if err != nil {
		return ImportOutcome{}, err
	}
	if plan.Skipped {
		return ImportOutcome{
			DestinationPath: plan.DestinationPath,
			Skipped:         true,
			ImportMode:      mode,
			ConflictAction:  plan.ConflictAction,
			ConflictPath:    plan.ConflictPath,
			Message:         plan.Message,
		}, nil
	}
	destination = plan.DestinationPath
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return ImportOutcome{}, err
	}
	operation, err := importFile(source, destination, mode, plan.Replaced)
	if err != nil {
		return ImportOutcome{}, err
	}
	destinationInfo, err := os.Stat(destination)
	if err != nil {
		return ImportOutcome{}, err
	}
	record := fileRecordFromPath(destination, format, destinationInfo, "imported")
	record.SourcePath = source
	record.Title = parsed.Title
	record.AuthorName = parsed.AuthorName
	record.Metadata["importedAt"] = time.Now().UTC().Format(time.RFC3339)
	record.Metadata["move"] = operation.Moved
	record.Metadata["importMode"] = operation.Mode
	record.Metadata["conflictAction"] = plan.ConflictAction
	if plan.ConflictPath != "" {
		record.Metadata["conflictPath"] = plan.ConflictPath
	}
	if plan.Replaced {
		record.Metadata["replacedExisting"] = true
	}
	if operation.Hardlinked {
		record.Metadata["hardlinked"] = true
	}
	if strings.TrimSpace(request.WantedID) != "" {
		record.Metadata["wantedId"] = strings.TrimSpace(request.WantedID)
	}
	if strings.TrimSpace(request.DownloadID) != "" {
		record.Metadata["downloadId"] = strings.TrimSpace(request.DownloadID)
	}
	if err := s.applyCalibreImport(ctx, destination, &record); err != nil {
		return ImportOutcome{}, err
	}
	stored, err := s.store.UpsertFile(ctx, record)
	if err != nil {
		return ImportOutcome{}, err
	}
	if strings.TrimSpace(request.WantedID) != "" && s.wanted != nil {
		_ = s.wanted.MarkWantedStatus(ctx, request.WantedID, "imported")
	}
	return ImportOutcome{
		File:            stored,
		DestinationPath: destination,
		Moved:           operation.Moved,
		Imported:        true,
		Replaced:        plan.Replaced,
		Hardlinked:      operation.Hardlinked,
		ImportMode:      operation.Mode,
		ConflictAction:  plan.ConflictAction,
		ConflictPath:    plan.ConflictPath,
	}, nil
}

func (s *Service) ImportCompletedDownloads(ctx context.Context, downloads []acquisition.DownloadStatus, request CompletedImportRequest) (CompletedImportOutcome, error) {
	if !s.Available() {
		return CompletedImportOutcome{}, errors.New("library service requires database persistence")
	}
	limit := request.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	allowedIDs := stringSet(request.DownloadIDs)
	outcome := CompletedImportOutcome{}
	for _, download := range downloads {
		if outcome.Checked >= limit {
			break
		}
		if len(allowedIDs) > 0 && !allowedIDs[download.ID] {
			continue
		}
		outcome.Checked++
		result := DownloadImportResult{
			Download: download,
			WantedID: wantedIDFromTags(download.Tags),
		}
		if !isCompletedDownload(download) {
			result.Status = "skipped"
			result.Message = "download is not complete"
			outcome.Skipped++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		if strings.TrimSpace(download.ImportStatus) == "imported" {
			result.Status = "skipped"
			result.Message = "download is already imported"
			outcome.Skipped++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		sourcePath, format, err := locateDownloadSource(download)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			outcome.Errored++
			if s.downloads != nil {
				_ = s.downloads.MarkDownloadImportError(ctx, download.ID, err.Error())
			}
			outcome.Results = append(outcome.Results, result)
			continue
		}
		result.SourcePath = sourcePath
		format = firstNonEmpty(formatFromDownload(download), format)
		if strings.TrimSpace(result.WantedID) == "" {
			review, err := s.queueImportReview(ctx, download, sourcePath, format, "download is not linked to a wanted item")
			if err != nil {
				result.Status = "error"
				result.Message = err.Error()
				outcome.Errored++
				if s.downloads != nil {
					_ = s.downloads.MarkDownloadImportError(ctx, download.ID, err.Error())
				}
				outcome.Results = append(outcome.Results, result)
				continue
			}
			result.Status = "review"
			result.Message = review.Reason
			result.Review = &review
			outcome.ReviewQueued++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		imported, err := s.Import(ctx, ImportRequest{
			SourcePath:     sourcePath,
			WantedID:       result.WantedID,
			DownloadID:     download.ID,
			Format:         format,
			Move:           request.Move,
			ImportMode:     request.ImportMode,
			ConflictAction: request.ConflictAction,
			Overwrite:      request.Overwrite,
		})
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			outcome.Errored++
			if s.downloads != nil {
				_ = s.downloads.MarkDownloadImportError(ctx, download.ID, err.Error())
			}
			outcome.Results = append(outcome.Results, result)
			continue
		}
		if imported.Skipped {
			result.Status = "skipped"
			result.Message = imported.Message
			result.Import = &imported
			outcome.Skipped++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		result.Status = "imported"
		result.Import = &imported
		outcome.Imported++
		if s.downloads != nil {
			_ = s.downloads.MarkDownloadImported(ctx, download.ID, imported.File.ID)
		}
		outcome.Results = append(outcome.Results, result)
	}
	return outcome, nil
}

func (s *Service) ResolveImportReview(ctx context.Context, id string, request ReviewDecisionRequest) (ReviewDecisionOutcome, error) {
	if !s.Available() {
		return ReviewDecisionOutcome{}, errors.New("library service requires database persistence")
	}
	review, err := s.store.GetImportReview(ctx, id)
	if err != nil {
		return ReviewDecisionOutcome{}, err
	}
	if review.Status != "pending" {
		return ReviewDecisionOutcome{}, errors.New("import review is already resolved")
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action == "" {
		action = "import"
	}
	switch action {
	case "import":
		wantedID := firstNonEmpty(request.WantedID, review.WantedID)
		format := firstNonEmpty(request.Format, review.MediaFormat)
		if normalized := normalizeFormat(format); normalized != "ebook" && normalized != "audiobook" {
			format = ""
		}
		imported, err := s.Import(ctx, ImportRequest{
			SourcePath:     review.SourcePath,
			WantedID:       wantedID,
			DownloadID:     review.DownloadID,
			Format:         format,
			Move:           request.Move,
			ImportMode:     request.ImportMode,
			ConflictAction: request.ConflictAction,
			Overwrite:      request.Overwrite,
		})
		if err != nil {
			return ReviewDecisionOutcome{}, err
		}
		if imported.Skipped {
			resolved, err := s.store.ResolveImportReview(ctx, review.ID, "skipped", action, imported.DestinationPath, wantedID)
			if err != nil {
				return ReviewDecisionOutcome{}, err
			}
			return ReviewDecisionOutcome{Review: resolved, Import: &imported}, nil
		}
		resolved, err := s.store.ResolveImportReview(ctx, review.ID, "imported", action, imported.DestinationPath, wantedID)
		if err != nil {
			return ReviewDecisionOutcome{}, err
		}
		if strings.TrimSpace(review.DownloadID) != "" && s.downloads != nil {
			_ = s.downloads.MarkDownloadImported(ctx, review.DownloadID, imported.File.ID)
		}
		return ReviewDecisionOutcome{Review: resolved, Import: &imported}, nil
	case "skip":
		resolved, err := s.store.ResolveImportReview(ctx, review.ID, "skipped", action, "", firstNonEmpty(request.WantedID, review.WantedID))
		if err != nil {
			return ReviewDecisionOutcome{}, err
		}
		return ReviewDecisionOutcome{Review: resolved}, nil
	case "reject":
		resolved, err := s.store.ResolveImportReview(ctx, review.ID, "rejected", action, "", firstNonEmpty(request.WantedID, review.WantedID))
		if err != nil {
			return ReviewDecisionOutcome{}, err
		}
		return ReviewDecisionOutcome{Review: resolved}, nil
	default:
		return ReviewDecisionOutcome{}, errors.New("review action must be import, skip, or reject")
	}
}

func (s *Service) scanRoots(request ScanRequest) []string {
	if strings.TrimSpace(request.Root) != "" {
		return []string{filepath.Clean(request.Root)}
	}
	switch normalizeFormat(request.Format) {
	case "ebook":
		return []string{s.ebookRoot()}
	case "audiobook":
		return []string{s.audiobookRoot()}
	default:
		return []string{s.ebookRoot(), s.audiobookRoot()}
	}
}

func (s *Service) destinationPath(format string, parsed parsedBook, ext string) string {
	root := s.ebookRoot()
	if normalizeFormat(format) == "audiobook" {
		root = s.audiobookRoot()
	}
	if strings.TrimSpace(root) == "" {
		return ""
	}
	policy := s.namingPolicy()
	values := map[string]string{
		"Author": firstNonEmpty(parsed.AuthorName, "Unknown Author"),
		"Title":  firstNonEmpty(parsed.Title, "Unknown Title"),
		"Format": normalizeFormat(format),
		"Ext":    strings.ToLower(ext),
	}
	parts := []string{root}
	parts = append(parts, renderPathSegments(policy.AuthorFolderTemplate, values, policy.SpaceReplacement)...)
	parts = append(parts, renderPathSegments(policy.BookFolderTemplate, values, policy.SpaceReplacement)...)
	fileName := renderFileName(policy.FileNameTemplate, values, policy.SpaceReplacement, ext)
	return filepath.Join(append(parts, fileName)...)
}

func (s *Service) renamePreviewForFile(file FileRecord, overwrite bool) (RenameFilePreview, error) {
	source := filepath.Clean(strings.TrimSpace(file.Path))
	if source == "" || source == "." {
		return RenameFilePreview{}, errors.New("file path is required")
	}
	parsedFromPath := parseBookFilename(source)
	parsed := parsedBook{
		Title:      firstNonEmpty(file.Title, parsedFromPath.Title),
		AuthorName: firstNonEmpty(file.AuthorName, parsedFromPath.AuthorName),
	}
	ext := firstNonEmpty(file.Extension, filepath.Ext(source))
	destination := s.destinationPath(file.MediaFormat, parsed, ext)
	if destination == "" {
		return RenameFilePreview{}, errors.New("library root is not configured")
	}
	destination = filepath.Clean(destination)
	if !overwrite && source != destination {
		destination = availableDestination(destination)
	}
	relativePath := filepath.Base(destination)
	root := s.ebookRoot()
	if normalizeFormat(file.MediaFormat) == "audiobook" {
		root = s.audiobookRoot()
	}
	if root != "" {
		if rel, err := filepath.Rel(root, destination); err == nil && !strings.HasPrefix(rel, "..") {
			relativePath = rel
		}
	}
	exists := false
	if source != destination {
		if _, err := os.Stat(destination); err == nil {
			exists = true
		}
	}
	return RenameFilePreview{
		File:            file,
		SourcePath:      source,
		DestinationPath: destination,
		RelativePath:    relativePath,
		Exists:          exists,
		Noop:            source == destination,
	}, nil
}

func (s *Service) applyRename(ctx context.Context, preview RenameFilePreview) (FileRecord, error) {
	if err := os.MkdirAll(filepath.Dir(preview.DestinationPath), 0o755); err != nil {
		return FileRecord{}, err
	}
	if err := copyOrMoveFile(preview.SourcePath, preview.DestinationPath, true); err != nil {
		return FileRecord{}, err
	}
	info, err := os.Stat(preview.DestinationPath)
	if err != nil {
		return FileRecord{}, err
	}
	file := preview.File
	file.Path = preview.DestinationPath
	file.Extension = strings.ToLower(filepath.Ext(preview.DestinationPath))
	file.SizeBytes = info.Size()
	modified := info.ModTime().UTC()
	file.ModifiedAt = &modified
	if strings.TrimSpace(file.ImportStatus) == "" {
		file.ImportStatus = "imported"
	}
	if file.Metadata == nil {
		file.Metadata = map[string]any{}
	}
	file.Metadata["renamedAt"] = time.Now().UTC().Format(time.RFC3339)
	file.Metadata["previousPath"] = preview.SourcePath
	return s.store.UpdateFile(ctx, file)
}

type namingPolicy struct {
	AuthorFolderTemplate string
	BookFolderTemplate   string
	FileNameTemplate     string
	SpaceReplacement     string
}

func (s *Service) namingPolicy() namingPolicy {
	return namingPolicy{
		AuthorFolderTemplate: firstNonEmpty(s.config.NamingAuthorFolderTemplate, "{Author}"),
		BookFolderTemplate:   firstNonEmpty(s.config.NamingBookFolderTemplate, "{Title}"),
		FileNameTemplate:     firstNonEmpty(s.config.NamingFileNameTemplate, "{Title}{Ext}"),
		SpaceReplacement:     safeSpaceReplacement(s.config.NamingSpaceReplacement),
	}
}

func (s *Service) ebookRoot() string {
	if strings.TrimSpace(s.config.EbookRoot) != "" {
		return strings.TrimSpace(s.config.EbookRoot)
	}
	return "/data/media/books/ebooks"
}

func (s *Service) audiobookRoot() string {
	if strings.TrimSpace(s.config.AudiobookRoot) != "" {
		return strings.TrimSpace(s.config.AudiobookRoot)
	}
	return "/data/media/books/audiobooks"
}

func (s *Service) lookupWanted(ctx context.Context, id string) (wanted.WantedItem, error) {
	if s.wanted == nil {
		return wanted.WantedItem{}, errors.New("wanted store is unavailable")
	}
	return s.wanted.GetWanted(ctx, id)
}

func (s *Service) applyCalibreImport(ctx context.Context, destination string, record *FileRecord) error {
	if s == nil || s.calibre == nil || s.rootFolders == nil || record == nil {
		return nil
	}
	settings, ok, err := s.calibreSettingsForDestination(ctx, destination)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	result, err := s.calibre.AddBook(ctx, calibre.AddBookRequest{
		Settings: settings,
		Path:     destination,
	})
	if err != nil {
		return err
	}
	if record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	record.Metadata["calibreId"] = result.ID
	record.Metadata["calibreImportedAt"] = time.Now().UTC().Format(time.RFC3339)
	record.Metadata["calibreHost"] = settings.Host
	record.Metadata["calibreLibrary"] = settings.Library
	record.Metadata["calibreOutputFormat"] = settings.OutputFormat
	record.Metadata["calibreOutputProfile"] = settings.OutputProfile
	metadata := calibreMetadataFromRecord(*record)
	if calibreMetadataHasChanges(metadata) {
		if err := s.calibre.SetFields(ctx, calibre.SetFieldsRequest{
			Settings: settings,
			ID:       result.ID,
			Metadata: metadata,
		}); err != nil {
			return err
		}
		record.Metadata["calibreMetadataSyncedAt"] = time.Now().UTC().Format(time.RFC3339)
	}
	conversion, err := s.calibre.Convert(ctx, calibre.ConvertRequest{
		Settings:    settings,
		ID:          result.ID,
		InputFormat: firstNonEmpty(record.Extension, filepath.Ext(record.Path)),
	})
	if err != nil {
		return err
	}
	if len(conversion.Jobs) > 0 || len(conversion.Skipped) > 0 {
		record.Metadata["calibreConversionJobs"] = calibreConversionJobMetadata(conversion.Jobs)
		record.Metadata["calibreConversionSkipped"] = conversion.Skipped
		record.Metadata["calibreConversionStartedAt"] = time.Now().UTC().Format(time.RFC3339)
	}
	if len(conversion.Jobs) > 0 {
		statuses, err := s.calibre.PollConversions(ctx, calibre.PollConversionsRequest{
			Settings:    settings,
			Jobs:        conversion.Jobs,
			MaxAttempts: 1,
		})
		if err != nil {
			return err
		}
		if len(statuses) > 0 {
			record.Metadata["calibreConversionStatuses"] = calibreConversionStatusMetadata(statuses)
			record.Metadata["calibreConversionPolledAt"] = time.Now().UTC().Format(time.RFC3339)
		}
	}
	return nil
}

func (s *Service) applyCalibreDelete(ctx context.Context, file FileRecord) error {
	if s == nil || s.calibre == nil || s.rootFolders == nil {
		return nil
	}
	calibreID := metadataInt(file.Metadata, "calibreId", 0)
	if calibreID <= 0 {
		return nil
	}
	settings, ok, err := s.calibreSettingsForDestination(ctx, file.Path)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return s.calibre.DeleteBooks(ctx, calibre.DeleteBooksRequest{
		Settings: settings,
		IDs:      []int{calibreID},
	})
}

func (s *Service) calibreSettingsForDestination(ctx context.Context, destination string) (calibre.Settings, bool, error) {
	if s == nil || s.rootFolders == nil {
		return calibre.Settings{}, false, nil
	}
	roots, err := s.rootFolders.ListRootFolders(ctx)
	if err != nil {
		return calibre.Settings{}, false, err
	}
	destination = filepath.Clean(strings.TrimSpace(destination))
	var best compatdata.RootFolder
	bestLength := -1
	for _, root := range roots {
		if !metadataBool(root.Metadata, "isCalibreLibrary", false) || strings.TrimSpace(root.Path) == "" {
			continue
		}
		if !pathWithinRoot(destination, root.Path) {
			continue
		}
		length := len(filepath.Clean(root.Path))
		if length > bestLength {
			best = root
			bestLength = length
		}
	}
	if bestLength < 0 {
		return calibre.Settings{}, false, nil
	}
	return calibreSettingsFromMetadata(best.Metadata), true, nil
}

func calibreSettingsFromMetadata(metadata map[string]any) calibre.Settings {
	return calibre.Settings{
		Host:          metadataString(metadata, "host"),
		Port:          metadataInt(metadata, "port", 8080),
		URLBase:       metadataString(metadata, "urlBase"),
		Username:      metadataString(metadata, "username"),
		Password:      metadataString(metadata, "password"),
		Library:       metadataString(metadata, "library"),
		OutputFormat:  metadataString(metadata, "outputFormat"),
		OutputProfile: metadataString(metadata, "outputProfile"),
		UseSSL:        metadataBool(metadata, "useSsl", false),
	}
}

func calibreMetadataFromRecord(record FileRecord) calibre.Metadata {
	return calibre.Metadata{
		Title:       record.Title,
		Authors:     compactStrings([]string{record.AuthorName}),
		Identifiers: calibreIdentifiers(record),
	}
}

func calibreMetadataHasChanges(metadata calibre.Metadata) bool {
	return strings.TrimSpace(metadata.Title) != "" || len(compactStrings(metadata.Authors)) > 0 ||
		strings.TrimSpace(metadata.Publisher) != "" || strings.TrimSpace(metadata.Languages) != "" ||
		len(compactStrings(metadata.Tags)) > 0 || strings.TrimSpace(metadata.Comments) != "" ||
		len(metadata.Identifiers) > 0 || strings.TrimSpace(metadata.Series) != "" || metadata.SeriesIndex != nil
}

func calibreIdentifiers(record FileRecord) map[string]string {
	pairs := map[string]string{
		"isbn":        firstNonEmpty(metadataString(record.Metadata, "isbn"), metadataString(record.Metadata, "isbn13"), metadataString(record.Metadata, "isbn10")),
		"asin":        metadataString(record.Metadata, "asin"),
		"goodreads":   metadataString(record.Metadata, "goodreads"),
		"openlibrary": firstNonEmpty(metadataString(record.Metadata, "openlibrary"), metadataString(record.Metadata, "openLibraryId")),
	}
	result := map[string]string{}
	for key, value := range pairs {
		value = strings.TrimSpace(value)
		if value != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func calibreConversionJobMetadata(jobs []calibre.ConvertJob) []map[string]any {
	result := make([]map[string]any, 0, len(jobs))
	for _, job := range jobs {
		if strings.TrimSpace(job.OutputFormat) == "" || job.JobID <= 0 {
			continue
		}
		result = append(result, map[string]any{
			"outputFormat": job.OutputFormat,
			"jobId":        job.JobID,
		})
	}
	return result
}

func calibreConversionStatusMetadata(statuses []calibre.ConversionStatus) []map[string]any {
	result := make([]map[string]any, 0, len(statuses))
	for _, status := range statuses {
		if status.JobID <= 0 {
			continue
		}
		record := map[string]any{
			"jobId":        status.JobID,
			"outputFormat": status.OutputFormat,
			"running":      status.Running,
			"ok":           status.OK,
			"wasAborted":   status.WasAborted,
		}
		if strings.TrimSpace(status.Traceback) != "" {
			record["traceback"] = strings.TrimSpace(status.Traceback)
		}
		if strings.TrimSpace(status.Log) != "" {
			record["log"] = strings.TrimSpace(status.Log)
		}
		result = append(result, record)
	}
	return result
}

func calibreConversionJobsFromMetadata(metadata map[string]any) []calibre.ConvertJob {
	raw, ok := metadata["calibreConversionJobs"]
	if !ok || raw == nil {
		return nil
	}
	var result []calibre.ConvertJob
	appendJob := func(record map[string]any) {
		jobID := metadataMapInt64(record, "jobId")
		if jobID <= 0 {
			return
		}
		result = append(result, calibre.ConvertJob{
			OutputFormat: metadataMapString(record, "outputFormat"),
			JobID:        jobID,
		})
	}
	switch typed := raw.(type) {
	case []map[string]any:
		for _, record := range typed {
			appendJob(record)
		}
	case []any:
		for _, item := range typed {
			if record, ok := item.(map[string]any); ok {
				appendJob(record)
			}
		}
	}
	return result
}

func calibreConversionStatusesFromMetadata(metadata map[string]any) []calibre.ConversionStatus {
	raw, ok := metadata["calibreConversionStatuses"]
	if !ok || raw == nil {
		return nil
	}
	var result []calibre.ConversionStatus
	appendStatus := func(record map[string]any) {
		jobID := metadataMapInt64(record, "jobId")
		if jobID <= 0 {
			return
		}
		result = append(result, calibre.ConversionStatus{
			OutputFormat: metadataMapString(record, "outputFormat"),
			JobID:        jobID,
			Running:      metadataMapBool(record, "running", false),
			OK:           metadataMapBool(record, "ok", false),
			WasAborted:   metadataMapBool(record, "wasAborted", false),
			Traceback:    metadataMapString(record, "traceback"),
			Log:          metadataMapString(record, "log"),
		})
	}
	switch typed := raw.(type) {
	case []map[string]any:
		for _, record := range typed {
			appendStatus(record)
		}
	case []any:
		for _, item := range typed {
			if record, ok := item.(map[string]any); ok {
				appendStatus(record)
			}
		}
	}
	return result
}

func calibreConversionNeedsRefresh(jobs []calibre.ConvertJob, statuses []calibre.ConversionStatus) bool {
	if len(jobs) == 0 {
		return false
	}
	statusByJob := map[int64]calibre.ConversionStatus{}
	for _, status := range statuses {
		statusByJob[status.JobID] = status
	}
	for _, job := range jobs {
		status, ok := statusByJob[job.JobID]
		if !ok || status.Running {
			return true
		}
	}
	return false
}

func calibreConversionAnyFailed(statuses []calibre.ConversionStatus) bool {
	for _, status := range statuses {
		if status.WasAborted || (!status.Running && !status.OK) {
			return true
		}
	}
	return false
}

func metadataMapString(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func metadataMapInt64(metadata map[string]any, key string) int64 {
	value, ok := metadata[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func metadataMapBool(metadata map[string]any, key string, fallback bool) bool {
	value, ok := metadata[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func pathWithinRoot(candidate string, root string) bool {
	candidate = filepath.Clean(strings.TrimSpace(candidate))
	root = filepath.Clean(strings.TrimSpace(root))
	if candidate == "" || candidate == "." || root == "" || root == "." {
		return false
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func metadataString(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func metadataInt(metadata map[string]any, key string, fallback int) int {
	value, ok := metadata[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			return parsed
		}
	}
	return fallback
}

func metadataBool(metadata map[string]any, key string, fallback bool) bool {
	value, ok := metadata[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		if parsed, err := strconv.ParseBool(strings.TrimSpace(typed)); err == nil {
			return parsed
		}
	}
	return fallback
}

func (s *Service) queueImportReview(ctx context.Context, download acquisition.DownloadStatus, sourcePath string, format string, reason string) (ImportReview, error) {
	source := filepath.Clean(strings.TrimSpace(sourcePath))
	if source == "." || source == "" {
		return ImportReview{}, errors.New("review source path is required")
	}
	info, err := os.Stat(source)
	if err != nil {
		return ImportReview{}, err
	}
	filenameParsed := parseBookFilename(source)
	localMetadata := localBookMetadataForPath(source)
	parsed := parsedBook{
		Title:      firstNonEmpty(localMetadata.Title, filenameParsed.Title),
		AuthorName: firstNonEmpty(localMetadata.AuthorName, filenameParsed.AuthorName),
	}
	metadata := importReviewMetadata(source, info, format, download, parsed, filenameParsed, localMetadata, reason)
	reviewWantedID := wantedIDFromTags(download.Tags)
	if strings.TrimSpace(reviewWantedID) == "" {
		candidates, suggestedID := s.importReviewWantedCandidates(ctx, parsed, format)
		addImportReviewWantedCandidateMetadata(metadata, candidates, suggestedID)
		reviewWantedID = suggestedID
	}
	return s.store.CreateImportReview(ctx, ImportReview{
		SourcePath:  source,
		DownloadID:  download.ID,
		WantedID:    reviewWantedID,
		MediaFormat: firstNonEmpty(normalizeFormat(format), "unknown"),
		Title:       parsed.Title,
		AuthorName:  parsed.AuthorName,
		SizeBytes:   info.Size(),
		Reason:      reason,
		Status:      "pending",
		Metadata:    metadata,
	})
}

func importReviewMetadata(source string, info fs.FileInfo, format string, download acquisition.DownloadStatus, parsed parsedBook, filenameParsed parsedBook, localMetadata localBookMetadata, reason string) map[string]any {
	source = filepath.Clean(strings.TrimSpace(source))
	metadata := map[string]any{
		"downloadName":       download.Name,
		"downloadClient":     download.Client,
		"downloadCategory":   download.Category,
		"downloadState":      download.State,
		"downloadSavePath":   download.SavePath,
		"tags":               download.Tags,
		"manualReview":       true,
		"manualReviewReason": strings.TrimSpace(reason),
		"matchConfidence":    importReviewConfidence(parsed, filenameParsed, localMetadata),
		"source": map[string]any{
			"path":        source,
			"fileName":    filepath.Base(source),
			"extension":   strings.ToLower(filepath.Ext(source)),
			"mediaFormat": firstNonEmpty(normalizeFormat(format), "unknown"),
			"sizeBytes":   info.Size(),
		},
		"download": map[string]any{
			"id":       download.ID,
			"client":   download.Client,
			"name":     download.Name,
			"category": download.Category,
			"state":    download.State,
			"savePath": download.SavePath,
			"tags":     download.Tags,
		},
		"parsed": map[string]any{
			"title":      parsed.Title,
			"authorName": parsed.AuthorName,
		},
		"filenameParsed": map[string]any{
			"title":      filenameParsed.Title,
			"authorName": filenameParsed.AuthorName,
		},
	}
	applyLocalMetadataToMap(metadata, localMetadata)
	if local, ok := metadata["localMetadata"].(map[string]any); ok {
		metadata["localMetadataEvidence"] = local
	}
	metadata["reviewEvidence"] = importReviewEvidence(source, info, format, download, parsed, filenameParsed, localMetadata, reason)
	return metadata
}

func (s *Service) importReviewWantedCandidates(ctx context.Context, parsed parsedBook, format string) ([]importReviewWantedCandidate, string) {
	if s == nil || s.wanted == nil || strings.TrimSpace(parsed.Title) == "" {
		return nil, ""
	}
	items, err := s.wanted.ListWanted(ctx, "")
	if err != nil {
		return nil, ""
	}
	candidates := make([]importReviewWantedCandidate, 0, len(items))
	for _, item := range items {
		if !importReviewWantedStatusEligible(item.Status) {
			continue
		}
		score, fields := importReviewWantedScore(parsed, format, item)
		if score < 0.55 {
			continue
		}
		candidates = append(candidates, importReviewWantedCandidate{
			WantedID:      strings.TrimSpace(item.ID),
			Title:         strings.TrimSpace(item.Title),
			AuthorName:    strings.TrimSpace(item.AuthorName),
			Format:        normalizeFormat(item.Format),
			Status:        strings.TrimSpace(item.Status),
			Score:         score,
			MatchedFields: fields,
			Reason:        strings.Join(fields, ", "),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		if candidates[i].Title != candidates[j].Title {
			return candidates[i].Title < candidates[j].Title
		}
		return candidates[i].WantedID < candidates[j].WantedID
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	strong := make([]importReviewWantedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Score >= 0.9 {
			strong = append(strong, candidate)
		}
	}
	if len(strong) == 1 {
		return candidates, strong[0].WantedID
	}
	return candidates, ""
}

func importReviewWantedStatusEligible(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "wanted", "grabbed", "missing":
		return true
	default:
		return false
	}
}

func importReviewWantedScore(parsed parsedBook, format string, item wanted.WantedItem) (float64, []string) {
	titleMatch := importReviewTextMatchScore(parsed.Title, item.Title)
	var score float64
	switch {
	case titleMatch >= 0.95:
		score = 0.58
	case titleMatch >= 0.75:
		score = 0.42
	case titleMatch >= 0.55:
		score = 0.30
	default:
		return 0, nil
	}
	fields := []string{"title"}
	if authorScore := importReviewTextMatchScore(parsed.AuthorName, item.AuthorName); authorScore >= 0.95 {
		score += 0.30
		fields = append(fields, "author")
	} else if authorScore >= 0.75 {
		score += 0.22
		fields = append(fields, "author")
	} else if authorScore >= 0.55 {
		score += 0.16
		fields = append(fields, "author")
	}
	requestedFormat := normalizeFormat(format)
	itemFormat := normalizeFormat(item.Format)
	if requestedFormat == "" || requestedFormat == "unknown" {
		score += 0.04
	} else if itemFormat == requestedFormat {
		score += 0.12
		fields = append(fields, "format")
	} else if itemFormat != "" {
		score -= 0.20
	}
	if score > 1 {
		score = 1
	}
	if score < 0 {
		return 0, nil
	}
	return score, fields
}

func importReviewTextMatchScore(left string, right string) float64 {
	left = normalizeImportReviewMatchText(left)
	right = normalizeImportReviewMatchText(right)
	if left == "" || right == "" {
		return 0
	}
	if left == right {
		return 1
	}
	if len(left) >= 6 && len(right) >= 6 && (strings.Contains(left, right) || strings.Contains(right, left)) {
		return 0.75
	}
	overlap := importReviewTokenOverlap(left, right)
	switch {
	case overlap >= 0.85:
		return 0.68
	case overlap >= 0.65:
		return 0.55
	default:
		return 0
	}
}

func normalizeImportReviewMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, value)
	fields := strings.Fields(value)
	if len(fields) > 1 {
		switch fields[0] {
		case "a", "an", "the":
			fields = fields[1:]
		}
	}
	return strings.Join(fields, " ")
}

func importReviewTokenOverlap(left string, right string) float64 {
	leftTokens := strings.Fields(left)
	rightTokens := strings.Fields(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	rightSet := map[string]bool{}
	for _, token := range rightTokens {
		rightSet[token] = true
	}
	matches := 0
	for _, token := range leftTokens {
		if rightSet[token] {
			matches++
		}
	}
	shorter := len(leftTokens)
	if len(rightTokens) < shorter {
		shorter = len(rightTokens)
	}
	if shorter == 0 {
		return 0
	}
	return float64(matches) / float64(shorter)
}

func addImportReviewWantedCandidateMetadata(metadata map[string]any, candidates []importReviewWantedCandidate, suggestedID string) {
	if len(candidates) == 0 {
		return
	}
	metadata["wantedCandidates"] = candidates
	metadata["wantedCandidateCount"] = len(candidates)
	if suggestedID != "" {
		metadata["suggestedWantedId"] = suggestedID
		metadata["autoLinkedWantedCandidate"] = true
		metadata["wantedMatchStrategy"] = "unique high-confidence title, author, and format match"
	}
	best := candidates[0]
	for _, candidate := range candidates {
		if candidate.WantedID == suggestedID {
			best = candidate
			metadata["suggestedWantedTitle"] = importReviewCandidateValue(candidate)
			break
		}
	}
	evidence := map[string]any{
		"source": "wanted",
		"label":  "Wanted match",
		"value":  importReviewCandidateValue(best),
		"weight": map[bool]string{true: "high", false: "medium"}[suggestedID != ""],
		"details": map[string]any{
			"wantedId": best.WantedID,
			"score":    best.Score,
			"status":   best.Status,
			"format":   best.Format,
		},
	}
	if existing, ok := metadata["reviewEvidence"].([]map[string]any); ok {
		metadata["reviewEvidence"] = append(existing, evidence)
		return
	}
	metadata["reviewEvidence"] = []map[string]any{evidence}
}

func importReviewCandidateValue(candidate importReviewWantedCandidate) string {
	value := importReviewTitleAuthorValue(parsedBook{Title: candidate.Title, AuthorName: candidate.AuthorName})
	if value == "" {
		value = candidate.WantedID
	}
	if candidate.Score > 0 {
		value = fmt.Sprintf("%s (%.2f)", value, candidate.Score)
	}
	return value
}

func importReviewEvidence(source string, info fs.FileInfo, format string, download acquisition.DownloadStatus, parsed parsedBook, filenameParsed parsedBook, localMetadata localBookMetadata, reason string) []map[string]any {
	evidence := []map[string]any{{
		"source": "file",
		"label":  "Source file",
		"value":  filepath.Base(source),
		"weight": "medium",
		"details": map[string]any{
			"extension":   strings.ToLower(filepath.Ext(source)),
			"mediaFormat": firstNonEmpty(normalizeFormat(format), "unknown"),
			"sizeBytes":   info.Size(),
		},
	}}
	if filenameParsed.Title != "" || filenameParsed.AuthorName != "" {
		evidence = append(evidence, map[string]any{
			"source": "filename",
			"label":  "Filename parse",
			"value":  importReviewTitleAuthorValue(filenameParsed),
			"weight": map[bool]string{true: "medium", false: "low"}[filenameParsed.Title != "" && filenameParsed.AuthorName != ""],
		})
	}
	if localMetadata.Source != "" || localMetadata.ExtractError != "" {
		value := importReviewTitleAuthorValue(parsed)
		if value == "" {
			value = localMetadata.ExtractError
		}
		evidence = append(evidence, map[string]any{
			"source": firstNonEmpty(localMetadata.Source, "local-metadata"),
			"label":  "Embedded metadata",
			"value":  value,
			"weight": map[bool]string{true: "high", false: "low"}[localMetadata.Title != "" && (localMetadata.AuthorName != "" || len(localMetadata.Authors) > 0)],
		})
	}
	if download.Name != "" || download.Category != "" {
		evidence = append(evidence, map[string]any{
			"source": "download",
			"label":  "Download context",
			"value":  firstNonEmpty(download.Name, download.Category, download.ID),
			"weight": "low",
			"details": map[string]any{
				"client":   download.Client,
				"category": download.Category,
				"state":    download.State,
			},
		})
	}
	if strings.TrimSpace(reason) != "" {
		evidence = append(evidence, map[string]any{
			"source": "policy",
			"label":  "Manual review reason",
			"value":  strings.TrimSpace(reason),
			"weight": "high",
		})
	}
	return evidence
}

func importReviewConfidence(parsed parsedBook, filenameParsed parsedBook, localMetadata localBookMetadata) string {
	if localMetadata.Title != "" && (localMetadata.AuthorName != "" || len(localMetadata.Authors) > 0) {
		return "high"
	}
	if localMetadata.Title != "" || localMetadata.AuthorName != "" || (filenameParsed.Title != "" && filenameParsed.AuthorName != "") {
		return "medium"
	}
	if parsed.Title != "" || filenameParsed.Title != "" {
		return "low"
	}
	return "unknown"
}

func importReviewTitleAuthorValue(parsed parsedBook) string {
	switch {
	case strings.TrimSpace(parsed.Title) != "" && strings.TrimSpace(parsed.AuthorName) != "":
		return strings.TrimSpace(parsed.Title) + " by " + strings.TrimSpace(parsed.AuthorName)
	case strings.TrimSpace(parsed.Title) != "":
		return strings.TrimSpace(parsed.Title)
	default:
		return strings.TrimSpace(parsed.AuthorName)
	}
}

func fileRecordFromPath(path string, format string, info fs.FileInfo, status string) FileRecord {
	parsed := parsedBookForPath(path)
	modified := info.ModTime().UTC()
	metadata := map[string]any{
		"fingerprint": fingerprint(path, info),
		"modifiedAt":  modified.Format(time.RFC3339),
		"scannedAt":   time.Now().UTC().Format(time.RFC3339),
	}
	applyLocalMetadataToMap(metadata, localBookMetadataForPath(path))
	return FileRecord{
		MediaFormat:  normalizeFormat(format),
		Path:         filepath.Clean(path),
		Title:        parsed.Title,
		AuthorName:   parsed.AuthorName,
		Extension:    strings.ToLower(filepath.Ext(path)),
		SizeBytes:    info.Size(),
		ImportStatus: status,
		Metadata:     metadata,
		ModifiedAt:   &modified,
	}
}

type parsedBook struct {
	Title      string
	AuthorName string
}

func parseBookFilename(path string) parsedBook {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
	for _, separator := range []string{" - ", " -- ", " by "} {
		if parts := strings.SplitN(name, separator, 2); len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			if left != "" && right != "" {
				if separator == " by " {
					return parsedBook{Title: left, AuthorName: right}
				}
				return parsedBook{AuthorName: left, Title: right}
			}
		}
	}
	return parsedBook{Title: name}
}

func classifyFile(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub", ".mobi", ".azw", ".azw3", ".pdf", ".cbz", ".cbr":
		return "ebook", true
	case ".m4b", ".mp3", ".m4a", ".flac", ".ogg", ".opus", ".aac":
		return "audiobook", true
	default:
		return "", false
	}
}

func formatAllowed(requested string, actual string) bool {
	requested = normalizeFormat(requested)
	return requested == "" || requested == "any" || requested == actual
}

func normalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "audio", "audiobook":
		return "audiobook"
	case "ebook", "book":
		return "ebook"
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

func safePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Unknown"
	}
	value = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		default:
			return r
		}
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 120 {
		value = strings.TrimSpace(string(runes[:120]))
	}
	return firstNonEmpty(value, "Unknown")
}

func renderPathSegments(template string, values map[string]string, spaceReplacement string) []string {
	parts := strings.FieldsFunc(template, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		segment := applySpaceReplacement(safePathSegment(renderTemplate(part, values)), spaceReplacement)
		if segment != "" && segment != "." && segment != ".." {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return []string{"Unknown"}
	}
	return segments
}

func renderFileName(template string, values map[string]string, spaceReplacement string, ext string) string {
	fileName := applySpaceReplacement(safePathSegment(renderTemplate(template, values)), spaceReplacement)
	if fileName == "" {
		fileName = "Unknown"
	}
	ext = strings.ToLower(ext)
	if ext != "" && !strings.EqualFold(filepath.Ext(fileName), ext) && filepath.Ext(fileName) == "" {
		fileName += ext
	}
	return fileName
}

func renderTemplate(template string, values map[string]string) string {
	rendered := template
	for key, value := range values {
		rendered = strings.ReplaceAll(rendered, "{"+key+"}", value)
		rendered = strings.ReplaceAll(rendered, "{"+strings.ToLower(key)+"}", value)
	}
	return rendered
}

func applySpaceReplacement(value string, replacement string) string {
	if replacement == "" {
		return value
	}
	return strings.ReplaceAll(value, " ", replacement)
}

func safeSpaceReplacement(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, `/\:*?"<>|`) {
		return "_"
	}
	runes := []rune(value)
	if len(runes) > 8 {
		value = string(runes[:8])
	}
	return value
}

func planImportDestination(source string, destination string, conflictAction string) (importDestinationPlan, error) {
	source = filepath.Clean(strings.TrimSpace(source))
	destination = filepath.Clean(strings.TrimSpace(destination))
	conflictAction = normalizeConflictAction(conflictAction, false)
	plan := importDestinationPlan{
		DestinationPath: destination,
		ConflictAction:  conflictAction,
	}
	if source == destination {
		return plan, nil
	}
	if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
		return plan, nil
	} else if err != nil {
		return plan, err
	}
	plan.ConflictPath = destination
	switch conflictAction {
	case "rename":
		plan.DestinationPath = availableDestination(destination)
		return plan, nil
	case "replace":
		plan.Replaced = true
		return plan, nil
	case "skip":
		plan.Skipped = true
		plan.Message = "destination already exists"
		return plan, nil
	default:
		return plan, fmt.Errorf("destination already exists: %s", destination)
	}
}

func normalizeImportMode(mode string, move bool) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	switch normalized {
	case "move":
		return "move"
	case "hardlink", "link":
		return "hardlink"
	case "hardlinkorcopy", "linkorcopy", "copyorhardlink", "auto":
		return "hardlinkOrCopy"
	case "copy":
		return "copy"
	default:
		if move {
			return "move"
		}
		return "copy"
	}
}

func normalizeConflictAction(action string, overwrite bool) string {
	normalized := strings.ToLower(strings.TrimSpace(action))
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	switch normalized {
	case "replace", "overwrite":
		return "replace"
	case "skip", "ignore":
		return "skip"
	case "fail", "error", "reject":
		return "fail"
	case "rename", "keepboth", "keep", "newname", "new":
		return "rename"
	default:
		if overwrite {
			return "replace"
		}
		return "rename"
	}
}

func copyOrMoveFile(source string, destination string, move bool) error {
	_, err := importFile(source, destination, normalizeImportMode("", move), false)
	return err
}

func importFile(source string, destination string, mode string, replace bool) (importFileOperation, error) {
	source = filepath.Clean(strings.TrimSpace(source))
	destination = filepath.Clean(strings.TrimSpace(destination))
	mode = normalizeImportMode(mode, false)
	operation := importFileOperation{Mode: mode, Moved: mode == "move"}
	if source == destination {
		operation.Moved = false
		return operation, nil
	}
	if replace {
		if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
			return operation, err
		}
	}
	switch mode {
	case "move":
		if err := os.Rename(source, destination); err == nil {
			return operation, nil
		}
		if err := copyFile(source, destination); err != nil {
			return operation, err
		}
		return operation, os.Remove(source)
	case "hardlink":
		if err := os.Link(source, destination); err != nil {
			return operation, err
		}
		operation.Hardlinked = true
		return operation, nil
	case "hardlinkOrCopy":
		if err := os.Link(source, destination); err == nil {
			operation.Hardlinked = true
			return operation, nil
		}
		operation.Mode = "copy"
		return operation, copyFile(source, destination)
	default:
		operation.Mode = "copy"
		return operation, copyFile(source, destination)
	}
}

func removeLibraryFile(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return errors.New("file path is required")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func copyFile(source string, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Sync()
}

func availableDestination(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; i < 1000; i++ {
		candidate := base + " (" + strconv.Itoa(i) + ")" + ext
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return base + " (" + strconv.FormatInt(time.Now().UnixNano(), 10) + ")" + ext
}

func locateDownloadSource(download acquisition.DownloadStatus) (string, string, error) {
	savePath := filepath.Clean(strings.TrimSpace(download.SavePath))
	if savePath == "." || savePath == "" {
		return "", "", errors.New("download save path is empty")
	}
	name := strings.TrimSpace(download.Name)
	var candidates []string
	if name != "" {
		candidates = append(candidates, filepath.Join(savePath, name))
	}
	candidates = append(candidates, savePath)
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if format, ok := classifyFile(candidate); ok {
				return candidate, format, nil
			}
			continue
		}
		if path, format, ok := bestBookFile(candidate, formatFromDownload(download)); ok {
			return path, format, nil
		}
	}
	return "", "", errors.New("no supported ebook or audiobook file found in completed download")
}

func bestBookFile(root string, preferredFormat string) (string, string, bool) {
	type candidate struct {
		path   string
		format string
		size   int64
	}
	var best candidate
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		format, ok := classifyFile(path)
		if !ok {
			return nil
		}
		if preferredFormat != "" && preferredFormat != "any" && format != preferredFormat {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if best.path == "" || info.Size() > best.size {
			best = candidate{path: path, format: format, size: info.Size()}
		}
		return nil
	})
	return best.path, best.format, best.path != ""
}

func isCompletedDownload(download acquisition.DownloadStatus) bool {
	state := strings.ToLower(strings.TrimSpace(download.State))
	if state == "removed" || strings.Contains(state, "error") {
		return false
	}
	if download.CompletedAt != nil || download.Progress >= 1 {
		return true
	}
	return strings.Contains(state, "upload") || strings.Contains(state, "seed")
}

func formatFromDownload(download acquisition.DownloadStatus) string {
	category := strings.ToLower(strings.TrimSpace(download.Category))
	switch {
	case strings.Contains(category, "audio"):
		return "audiobook"
	case strings.Contains(category, "ebook"), strings.Contains(category, "book"):
		return "ebook"
	default:
		return ""
	}
}

func wantedIDFromTags(tags []string) string {
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "wanted:") {
			return strings.TrimSpace(strings.TrimPrefix(tag, "wanted:"))
		}
	}
	return ""
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func fingerprint(path string, info fs.FileInfo) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path) + "\x00" + info.ModTime().UTC().Format(time.RFC3339Nano) + "\x00" + strconv.FormatInt(info.Size(), 10)))
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
