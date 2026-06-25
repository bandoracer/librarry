package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type WantedStore interface {
	GetWanted(ctx context.Context, id string) (wanted.WantedItem, error)
	MarkWantedStatus(ctx context.Context, wantedID string, status string) error
}

type Service struct {
	store  *Store
	config Config
	wanted WantedStore
}

func NewService(store *Store, config Config, wanted WantedStore) *Service {
	return &Service{store: store, config: config, wanted: wanted}
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
	stored, err := s.store.UpsertFile(ctx, record)
	if err != nil {
		return ImportOutcome{}, err
	}
	if strings.TrimSpace(request.WantedID) != "" && s.wanted != nil {
		_ = s.wanted.MarkWantedStatus(ctx, request.WantedID, "imported")
	}
	return ImportOutcome{File: stored, DestinationPath: destination, Moved: request.Move}, nil
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
	author := safePathSegment(firstNonEmpty(parsed.AuthorName, "Unknown Author"))
	title := safePathSegment(firstNonEmpty(parsed.Title, "Unknown Title"))
	fileName := title + strings.ToLower(ext)
	return filepath.Join(root, author, title, fileName)
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
