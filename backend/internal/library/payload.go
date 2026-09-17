package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

// PayloadFile retains both included content and exclusions for operator review.
// RelativePath is relative to the client's normalized save root, not a guessed
// shared-directory search. Selected=nil means the adapter did not prove selection.
type PayloadFile struct {
	RelativePath string            `json:"relativePath"`
	SourcePath   string            `json:"sourcePath"`
	Format       string            `json:"format"`
	SizeBytes    int64             `json:"sizeBytes"`
	Progress     float64           `json:"progress"`
	Selected     *bool             `json:"selected"`
	Included     bool              `json:"included"`
	Reason       string            `json:"reason,omitempty"`
	Title        string            `json:"title,omitempty"`
	Author       string            `json:"author,omitempty"`
	Album        string            `json:"album,omitempty"`
	Track        string            `json:"track,omitempty"`
	Identifiers  map[string]string `json:"identifiers,omitempty"`
}

type DownloadPayload struct {
	Client          string        `json:"client"`
	DownloadID      string        `json:"downloadId"`
	DownloadName    string        `json:"downloadName"`
	SourceRoot      string        `json:"sourceRoot"`
	Root            string        `json:"root"`
	InventorySource string        `json:"inventorySource"`
	Files           []PayloadFile `json:"files"`
}

func cleanPayloadRelative(name string) (string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), `\`, "/")
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsRune(name, 0) {
		return "", errors.New("unsafe client file path")
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || strings.Contains(part, ":") {
			return "", errors.New("unsafe client file path")
		}
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) {
		return "", errors.New("unsafe client file path")
	}
	return clean, nil
}

func payloadFileFormat(path string, extras string) string {
	if format, ok := classifyFile(path); ok {
		return format
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, sidecar := range append([]string{".opf", ".cue", ".jpg", ".jpeg", ".png", ".webp", ".m3u", ".m3u8", ".nfo"}, importExtraExtensions(extras)...) {
		if ext == sidecar {
			return "sidecar"
		}
	}
	return "excluded"
}

func (s *Service) inspectDownloadPayload(ctx context.Context, download acquisition.DownloadStatus) (DownloadPayload, error) {
	if s.inspector == nil {
		return DownloadPayload{}, errors.New("download client inventory is required for import")
	}
	details, err := s.inspector.DownloadDetails(ctx, download.ID, download.Client)
	if err != nil {
		return DownloadPayload{}, fmt.Errorf("read download inventory: %w", err)
	}
	if details.Status.ID != download.ID || !strings.EqualFold(details.Status.Client, download.Client) {
		return DownloadPayload{}, errors.New("client inventory identity differs from requested download")
	}
	if !isCompletedDownload(details.Status) {
		return DownloadPayload{}, errors.New("client no longer reports a completed download")
	}
	mapped := remapDownloadSavePath(details.Status, s.remotePathMappings(ctx))
	payload := DownloadPayload{Client: download.Client, DownloadID: download.ID, DownloadName: details.Status.Name, InventorySource: details.InventorySource, Files: []PayloadFile{}}
	root := filepath.Clean(mapped.SavePath)
	if details.InventorySource == "completed-directory" {
		// Only SABnzbd's completed history record can attest a finalized extraction
		// directory. Queue/archive inventories cannot prove the extracted book set.
		if !strings.EqualFold(download.Client, "SABnzbd") || details.Status.State != "completed" || details.PayloadRoot == "" {
			return payload, errors.New("unproven extraction directory")
		}
		root = rewriteRemotePath(details.PayloadRoot, download.Client, s.remotePathMappings(ctx))
	}
	if !filepath.IsAbs(root) || root == string(filepath.Separator) {
		return payload, errors.New("client inventory requires a specific absolute save path")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return payload, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return payload, err
	}
	inventory := details.Files
	if details.InventorySource == "completed-directory" {
		if !info.IsDir() {
			return payload, errors.New("extraction output is not a directory")
		}
		inventory = []acquisition.DownloadFile{}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return errors.New("non-regular file in extraction output")
			}
			if len(inventory) >= 10000 {
				return errors.New("payload exceeds 10000-file review limit")
			}
			stat, err := entry.Info()
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			inventory = append(inventory, acquisition.DownloadFile{Name: relative, SizeBytes: stat.Size(), Progress: 1, Selected: new(true)})
			return nil
		})
		if err != nil {
			return payload, err
		}
		payload.Root = root
	} else {
		if details.InventorySource != "client-files" {
			return payload, errors.New("adapter did not provide a payload inventory")
		}
		if !info.IsDir() {
			root = filepath.Dir(root)
		}
	}
	if len(inventory) == 0 || len(inventory) > 10000 {
		return payload, errors.New("client payload inventory is empty or exceeds 10000 files")
	}
	payload.SourceRoot = root
	seen := map[string]bool{}
	resolvedPaths := map[string]bool{}
	for _, entry := range inventory {
		relative, err := cleanPayloadRelative(entry.Name)
		if err != nil {
			return payload, err
		}
		if seen[relative] {
			return payload, errors.New("duplicate path in client inventory")
		}
		seen[relative] = true
		path := filepath.Join(root, relative)
		f := PayloadFile{RelativePath: relative, SourcePath: path, Format: payloadFileFormat(path, s.Config().ImportExtraFiles), SizeBytes: entry.SizeBytes, Progress: entry.Progress, Selected: entry.Selected}
		// Resolve a client's save path that already names its payload directory.
		// The fallback strips only that exact leading component, never a filename search.
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) && strings.HasPrefix(relative, filepath.Base(root)+string(filepath.Separator)) {
			path = filepath.Join(root, strings.TrimPrefix(relative, filepath.Base(root)+string(filepath.Separator)))
			f.SourcePath = path
			f.RelativePath, _ = filepath.Rel(root, path)
		}
		if resolvedPaths[path] {
			return payload, errors.New("client inventory paths resolve to the same file")
		}
		resolvedPaths[path] = true
		// Even excluded client entries must not cross symlinks. Cleanup operates
		// on the complete payload, not just files selected for import.
		if stat, statErr := os.Lstat(path); statErr == nil {
			resolved, resolveErr := filepath.EvalSymlinks(path)
			if !stat.Mode().IsRegular() || resolveErr != nil || resolved != path || !pathWithinRoot(resolved, root) {
				return payload, errors.New("client payload contains a non-regular file or crosses a symlink")
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return payload, statErr
		}
		if f.Format == "excluded" {
			f.Reason = "not a supported book or sidecar"
		} else if entry.Selected == nil {
			f.Reason = "client selection is unknown"
		} else if !*entry.Selected {
			f.Reason = "not selected in download client"
		} else if entry.Progress < 1 || entry.SizeBytes <= 0 {
			f.Reason = "file is incomplete or empty"
		} else {
			stat, err := os.Lstat(path)
			if err != nil {
				f.Reason = "client file is unavailable: " + err.Error()
			} else if !stat.Mode().IsRegular() {
				return payload, errors.New("symlink or non-regular file in client payload")
			} else {
				resolved, err := filepath.EvalSymlinks(path)
				if err != nil || resolved != path || !pathWithinRoot(resolved, root) {
					return payload, errors.New("client file crosses a symlink or source boundary")
				}
				if stat.Size() != entry.SizeBytes {
					f.Reason = "client size does not match local file"
				} else {
					f.Included = true
				}
			}
		}
		if f.Included && f.Format != "sidecar" {
			metadata := localBookMetadataForPath(path)
			f.Title, f.Author, f.Album, f.Track, f.Identifiers = metadata.Title, metadata.AuthorName, metadata.Album, metadata.Track, metadata.Identifiers
			if f.Format == "audiobook" && (metadata.Source == "sidecar-opf" || metadata.Source == "opf") {
				f.Album = metadata.Title
			}
		}
		payload.Files = append(payload.Files, f)
	}
	sort.Slice(payload.Files, func(i, j int) bool {
		return naturalPathLess(payload.Files[i].RelativePath, payload.Files[j].RelativePath)
	})
	if payload.Root == "" {
		payload.Root = filepath.Dir(payload.Files[0].SourcePath)
		for _, f := range payload.Files {
			for !pathWithinRoot(f.SourcePath, payload.Root) {
				payload.Root = filepath.Dir(payload.Root)
			}
		}
	}
	// Only inspect an inventory-owned subdirectory. Other downloads in a shared
	// save root are outside this payload and must not influence selection.
	if payload.Root != root {
		known := map[string]bool{}
		for _, f := range payload.Files {
			known[f.SourcePath] = true
		}
		err = filepath.WalkDir(payload.Root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return errors.New("non-regular file inside payload directory")
			}
			if !known[path] {
				stat, err := entry.Info()
				if err != nil {
					return err
				}
				relative, _ := filepath.Rel(root, path)
				payload.Files = append(payload.Files, PayloadFile{RelativePath: relative, SourcePath: path, Format: payloadFileFormat(path, s.Config().ImportExtraFiles), SizeBytes: stat.Size(), Reason: "not present in client inventory"})
			}
			if len(payload.Files) > 10000 {
				return errors.New("payload exceeds 10000-file review limit")
			}
			return nil
		})
		if err != nil {
			return payload, err
		}
	}
	sort.Slice(payload.Files, func(i, j int) bool {
		return naturalPathLess(payload.Files[i].RelativePath, payload.Files[j].RelativePath)
	})
	return payload, nil
}

var pathNumbers = regexp.MustCompile(`\d+|\D+`)

func naturalPathLess(a, b string) bool {
	x, y := pathNumbers.FindAllString(strings.ToLower(a), -1), pathNumbers.FindAllString(strings.ToLower(b), -1)
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] == y[i] {
			continue
		}
		xi, xe := strconv.ParseUint(x[i], 10, 64)
		yi, ye := strconv.ParseUint(y[i], 10, 64)
		if xe == nil && ye == nil && xi != yi {
			return xi < yi
		}
		return x[i] < y[i]
	}
	if len(x) != len(y) {
		return len(x) < len(y)
	}
	return a < b
}

var chapterFilename = regexp.MustCompile(`(?i)^(?:(?:chapter|track|part)[ _.-]*)?[0-9]{1,4}$`)
var discFolder = regexp.MustCompile(`(?i)^(?:cd|disc|disk)[ _.-]*[0-9]{1,3}$`)

func (s *Service) payloadMatchReasons(ctx context.Context, payload DownloadPayload, item wanted.WantedItem) []string {
	reasons := []string{}
	media := []PayloadFile{}
	for _, f := range payload.Files {
		if f.Format == "ebook" || f.Format == "audiobook" {
			media = append(media, f)
			if !f.Included {
				reasons = append(reasons, f.RelativePath+": "+f.Reason)
			}
			if item.Format != "" && item.Format != "any" && normalizeFormat(item.Format) != f.Format {
				reasons = append(reasons, f.RelativePath+": format differs from wanted book")
			}
		} else if f.Format == "sidecar" && !f.Included {
			reasons = append(reasons, f.RelativePath+": "+f.Reason)
		}
	}
	if len(media) == 0 {
		return append(reasons, "payload has no supported book files")
	}
	providerISBNs := s.providerISBNsForWanted(ctx, []wanted.WantedItem{item})
	wantedISBNs := wantedItemISBNs(item, providerISBNs[item.ID]...)
	album := ""
	editionISBNs := []string{}
	extension := ""
	tracks := map[string]bool{}
	for _, f := range media {
		fileISBNs := importReviewIdentifierISBNs(f.Identifiers)
		if len(media) > 1 {
			if extension != "" && extension != strings.ToLower(filepath.Ext(f.SourcePath)) {
				reasons = append(reasons, "different audio encodings require explicit edition mapping")
			}
			extension = strings.ToLower(filepath.Ext(f.SourcePath))
			if len(fileISBNs) > 0 {
				if len(editionISBNs) > 0 && !importReviewISBNsOverlap(editionISBNs, fileISBNs) {
					reasons = append(reasons, "files identify different book editions")
				}
				editionISBNs = fileISBNs
			}
			if f.Track != "" {
				key := filepath.Dir(f.SourcePath) + ":" + strings.Split(f.Track, "/")[0]
				if tracks[key] {
					reasons = append(reasons, "duplicate chapter number requires review")
				}
				tracks[key] = true
			}
		}
		if len(fileISBNs) > 0 && len(wantedISBNs) > 0 && !importReviewISBNsOverlap(fileISBNs, wantedISBNs) {
			reasons = append(reasons, f.RelativePath+": ISBN differs from wanted book")
		}
		title := f.Title
		if f.Format == "audiobook" && (len(media) > 1 || f.Album != "") {
			title = f.Album
		}
		if title != "" && normalizeImportReviewMatchText(title) != normalizeImportReviewMatchText(item.Title) {
			reasons = append(reasons, f.RelativePath+": embedded book title differs from wanted book")
		}
		if f.Author != "" && item.AuthorName != "" && normalizeImportReviewMatchText(f.Author) != normalizeImportReviewMatchText(item.AuthorName) {
			reasons = append(reasons, f.RelativePath+": embedded author differs from wanted author")
		}
		if len(media) > 1 {
			if f.Format != "audiobook" {
				reasons = append(reasons, "multiple ebook editions or mixed formats require file mapping")
				break
			}
			if f.Album != "" {
				if album != "" && normalizeImportReviewMatchText(album) != normalizeImportReviewMatchText(f.Album) {
					reasons = append(reasons, "audio files identify different books")
				}
				album = f.Album
			}
		}
	}
	if len(media) > 1 {
		// Full album evidence or a strict numbered chapter/disc layout identifies a
		// set. A folder of unrelated audio books cannot pass on a wanted tag alone.
		strongAlbum := album != ""
		for _, f := range media {
			if f.Album == "" {
				strongAlbum = false
			}
		}
		if !strongAlbum {
			if normalizeImportReviewMatchText(payload.DownloadName) != normalizeImportReviewMatchText(item.Title) {
				reasons = append(reasons, "chapter layout lacks matching book-level metadata")
			}
			for _, f := range media {
				relative, _ := filepath.Rel(payload.Root, f.SourcePath)
				parts := strings.Split(relative, string(filepath.Separator))
				base := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(relative))
				if !chapterFilename.MatchString(base) {
					reasons = append(reasons, f.RelativePath+": chapter grouping is uncertain")
				}
				for _, dir := range parts[:len(parts)-1] {
					if !discFolder.MatchString(dir) {
						reasons = append(reasons, f.RelativePath+": directory may contain a separate book")
					}
				}
			}
		}
	}
	return compactStrings(reasons)
}
