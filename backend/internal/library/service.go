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
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/calibre"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type WantedStore interface {
	GetWanted(ctx context.Context, id string) (wanted.WantedItem, error)
	MarkWantedStatus(ctx context.Context, wantedID string, status string) error
}

type DownloadStore interface {
	MarkDownloadImported(ctx context.Context, id string, fileID string) error
	MarkDownloadImportError(ctx context.Context, id string, message string) error
}

type RootFolderProvider interface {
	ListRootFolders(ctx context.Context) ([]compatdata.RootFolder, error)
}

type Service struct {
	store       *Store
	config      Config
	wanted      WantedStore
	downloads   DownloadStore
	calibre     calibre.Importer
	rootFolders RootFolderProvider
}

func NewService(store *Store, config Config, wanted WantedStore, downloads DownloadStore) *Service {
	return &Service{store: store, config: config, wanted: wanted, downloads: downloads}
}

func (s *Service) WithCalibre(importer calibre.Importer, rootFolders RootFolderProvider) *Service {
	if s == nil {
		return s
	}
	s.calibre = importer
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

	parsed := parseBookFilename(source)
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
	if filepath.Clean(source) != filepath.Clean(destination) {
		destination = availableDestination(destination)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return ImportOutcome{}, err
	}
	if err := copyOrMoveFile(source, destination, request.Move); err != nil {
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
	record.Metadata["move"] = request.Move
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
	return ImportOutcome{File: stored, DestinationPath: destination, Moved: request.Move}, nil
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
			SourcePath: sourcePath,
			WantedID:   result.WantedID,
			DownloadID: download.ID,
			Format:     format,
			Move:       request.Move,
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
			SourcePath: review.SourcePath,
			WantedID:   wantedID,
			DownloadID: review.DownloadID,
			Format:     format,
			Move:       request.Move,
		})
		if err != nil {
			return ReviewDecisionOutcome{}, err
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
	return nil
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
	parsed := parseBookFilename(source)
	metadata := map[string]any{
		"downloadName":     download.Name,
		"downloadCategory": download.Category,
		"downloadState":    download.State,
		"tags":             download.Tags,
	}
	return s.store.CreateImportReview(ctx, ImportReview{
		SourcePath:  source,
		DownloadID:  download.ID,
		WantedID:    wantedIDFromTags(download.Tags),
		MediaFormat: firstNonEmpty(normalizeFormat(format), "unknown"),
		Title:       parsed.Title,
		AuthorName:  parsed.AuthorName,
		SizeBytes:   info.Size(),
		Reason:      reason,
		Status:      "pending",
		Metadata:    metadata,
	})
}

func fileRecordFromPath(path string, format string, info fs.FileInfo, status string) FileRecord {
	parsed := parseBookFilename(path)
	modified := info.ModTime().UTC()
	metadata := map[string]any{
		"fingerprint": fingerprint(path, info),
		"modifiedAt":  modified.Format(time.RFC3339),
		"scannedAt":   time.Now().UTC().Format(time.RFC3339),
	}
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

func copyOrMoveFile(source string, destination string, move bool) error {
	if source == destination {
		return nil
	}
	if move {
		if err := os.Rename(source, destination); err == nil {
			return nil
		}
	}
	if err := copyFile(source, destination); err != nil {
		return err
	}
	if move {
		return os.Remove(source)
	}
	return nil
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
