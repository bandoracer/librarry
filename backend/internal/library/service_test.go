package library

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/calibre"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestParseBookFilename(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		title  string
		author string
	}{
		{name: "author dash title", path: "Andy Weir - Project Hail Mary.epub", title: "Project Hail Mary", author: "Andy Weir"},
		{name: "title by author", path: "Project Hail Mary by Andy Weir.m4b", title: "Project Hail Mary", author: "Andy Weir"},
		{name: "underscores", path: "Dungeon_Crawler_Carl.epub", title: "Dungeon Crawler Carl", author: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBookFilename(tt.path)
			if got.Title != tt.title || got.AuthorName != tt.author {
				t.Fatalf("expected %q/%q, got %q/%q", tt.title, tt.author, got.Title, got.AuthorName)
			}
		})
	}
}

func TestNoDatabaseReadPathsReturnEmptyStartupData(t *testing.T) {
	service := NewService(nil, Config{}, nil, nil)
	ctx := context.Background()

	files, err := service.ListFiles(ctx, FileListQuery{Limit: 25})
	if err != nil || len(files) != 0 {
		t.Fatalf("expected empty files without database, got %d files and error %v", len(files), err)
	}

	reviews, err := service.ListImportReviews(ctx, ReviewListQuery{Status: "pending", Limit: 25})
	if err != nil || len(reviews) != 0 {
		t.Fatalf("expected empty import reviews without database, got %d reviews and error %v", len(reviews), err)
	}
}

func TestParsedBookForPathPrefersEPUBMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bad_File_Name.epub")
	writeTestEPUB(t, path, `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
  <metadata>
    <dc:title>Project Hail Mary</dc:title>
    <dc:creator>Andy Weir</dc:creator>
    <dc:identifier opf:scheme="ISBN">978-0-593-13520-4</dc:identifier>
    <dc:language>en</dc:language>
    <dc:publisher>Ballantine Books</dc:publisher>
    <meta name="calibre:series" content="Hail Mary" />
    <meta name="calibre:series_index" content="1" />
  </metadata>
</package>`)

	parsed := parsedBookForPath(path)
	if parsed.Title != "Project Hail Mary" || parsed.AuthorName != "Andy Weir" {
		t.Fatalf("expected EPUB metadata title/author, got %+v", parsed)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	record := fileRecordFromPath(path, "ebook", info, "available")
	if record.Title != "Project Hail Mary" || record.AuthorName != "Andy Weir" {
		t.Fatalf("expected file record to use EPUB metadata, got %+v", record)
	}
	if record.Metadata["isbn13"] != "9780593135204" || record.Metadata["metadataSource"] != "epub-opf" {
		t.Fatalf("expected EPUB identifiers in metadata, got %#v", record.Metadata)
	}
	local, ok := record.Metadata["localMetadata"].(map[string]any)
	if !ok || local["series"] != "Hail Mary" || local["language"] != "en" || local["publisher"] != "Ballantine Books" {
		t.Fatalf("expected local metadata payload, got %#v", record.Metadata["localMetadata"])
	}
}

func TestParsedBookForPathUsesSidecarOPF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "filename-only.epub")
	if err := os.WriteFile(path, []byte("not a zip but sidecar exists"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "filename-only.opf"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns:dc="http://purl.org/dc/elements/1.1/">
  <metadata>
    <dc:title>Dungeon Crawler Carl</dc:title>
    <dc:creator>Matt Dinniman</dc:creator>
    <dc:identifier>9798986133815</dc:identifier>
  </metadata>
</package>`), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed := parsedBookForPath(path)
	if parsed.Title != "Dungeon Crawler Carl" || parsed.AuthorName != "Matt Dinniman" {
		t.Fatalf("expected sidecar OPF title/author, got %+v", parsed)
	}
	metadata := localBookMetadataForPath(path)
	if metadata.Source != "sidecar-opf" || metadata.Identifiers["isbn13"] != "9798986133815" {
		t.Fatalf("expected sidecar identifiers, got %+v", metadata)
	}
}

func TestImportReviewMetadataBuildsEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-name.epub")
	if err := os.WriteFile(path, []byte("not a zip but sidecar exists"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad-name.opf"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns:dc="http://purl.org/dc/elements/1.1/">
  <metadata>
    <dc:title>Dungeon Crawler Carl</dc:title>
    <dc:creator>Matt Dinniman</dc:creator>
    <dc:identifier>9798986133815</dc:identifier>
  </metadata>
</package>`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	filenameParsed := parseBookFilename(path)
	local := localBookMetadataForPath(path)
	parsed := parsedBook{Title: firstNonEmpty(local.Title, filenameParsed.Title), AuthorName: firstNonEmpty(local.AuthorName, filenameParsed.AuthorName)}

	metadata := importReviewMetadata(path, info, "ebook", acquisition.DownloadStatus{
		Client:   "qBittorrent",
		ID:       "abc123",
		Name:     "Dungeon Crawler Carl EPUB",
		Category: "books-ebook",
		State:    "completed",
		Tags:     []string{"librarry"},
	}, parsed, filenameParsed, local, "download is not linked to a wanted item")

	if metadata["matchConfidence"] != "high" || metadata["isbn13"] != "9798986133815" {
		t.Fatalf("expected high-confidence identifier evidence, got %#v", metadata)
	}
	if source, ok := metadata["source"].(map[string]any); !ok || source["fileName"] != "bad-name.epub" || source["mediaFormat"] != "ebook" {
		t.Fatalf("expected source evidence, got %#v", metadata["source"])
	}
	if parsedPayload, ok := metadata["parsed"].(map[string]any); !ok || parsedPayload["title"] != "Dungeon Crawler Carl" || parsedPayload["authorName"] != "Matt Dinniman" {
		t.Fatalf("expected parsed title/author evidence, got %#v", metadata["parsed"])
	}
	evidence, ok := metadata["reviewEvidence"].([]map[string]any)
	if !ok || len(evidence) < 4 {
		t.Fatalf("expected review evidence list, got %#v", metadata["reviewEvidence"])
	}
	if evidence[len(evidence)-1]["source"] != "policy" {
		t.Fatalf("expected policy review reason evidence, got %#v", evidence)
	}
}

func TestImportReviewWantedCandidatesSuggestsUniqueStrongMatch(t *testing.T) {
	service := &Service{wanted: fakeImportWantedStore{items: []wanted.WantedItem{{
		ID:         "wanted-1",
		Title:      "Dungeon Crawler Carl",
		AuthorName: "Matt Dinniman",
		Format:     "ebook",
		Status:     "grabbed",
	}, {
		ID:         "wanted-2",
		Title:      "Carl's Doomsday Scenario",
		AuthorName: "Matt Dinniman",
		Format:     "ebook",
		Status:     "wanted",
	}}}}

	candidates, suggestedID := service.importReviewWantedCandidates(context.Background(), parsedBook{
		Title:      "Dungeon Crawler Carl",
		AuthorName: "Matt Dinniman",
	}, "ebook")

	if suggestedID != "wanted-1" {
		t.Fatalf("expected wanted-1 suggestion, got %q with candidates %#v", suggestedID, candidates)
	}
	if len(candidates) == 0 || candidates[0].WantedID != "wanted-1" || candidates[0].Score < 0.9 {
		t.Fatalf("expected high-confidence first candidate, got %#v", candidates)
	}
}

func TestImportReviewWantedCandidatesDoNotSuggestAmbiguousMatch(t *testing.T) {
	service := &Service{wanted: fakeImportWantedStore{items: []wanted.WantedItem{{
		ID:         "ebook",
		Title:      "Project Hail Mary",
		AuthorName: "Andy Weir",
		Format:     "ebook",
		Status:     "wanted",
	}, {
		ID:         "audio",
		Title:      "Project Hail Mary",
		AuthorName: "Andy Weir",
		Format:     "audiobook",
		Status:     "wanted",
	}}}}

	candidates, suggestedID := service.importReviewWantedCandidates(context.Background(), parsedBook{
		Title:      "Project Hail Mary",
		AuthorName: "Andy Weir",
	}, "unknown")

	if suggestedID != "" {
		t.Fatalf("expected no suggestion for ambiguous format, got %q", suggestedID)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected both format candidates, got %#v", candidates)
	}
}

func TestParsedBookForPathReadsID3AudioTags(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-name.mp3")
	writeTestMP3WithID3(t, path, map[string]string{
		"TIT2": "The Hobbit",
		"TPE1": "J. R. R. Tolkien",
		"TALB": "Middle-earth",
		"TCON": "Audiobook",
		"TDRC": "1937",
		"TRCK": "1",
	})

	parsed := parsedBookForPath(path)
	if parsed.Title != "The Hobbit" || parsed.AuthorName != "J. R. R. Tolkien" {
		t.Fatalf("expected ID3 title/author, got %+v", parsed)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	record := fileRecordFromPath(path, "audiobook", info, "available")
	if record.Title != "The Hobbit" || record.AuthorName != "J. R. R. Tolkien" || record.Metadata["metadataSource"] != "id3v2" {
		t.Fatalf("expected ID3 metadata record, got %+v metadata=%#v", record, record.Metadata)
	}
	local, ok := record.Metadata["localMetadata"].(map[string]any)
	if !ok || local["album"] != "Middle-earth" || local["year"] != "1937" || local["track"] != "1" {
		t.Fatalf("expected ID3 local metadata, got %#v", record.Metadata["localMetadata"])
	}
}

type fakeImportWantedStore struct {
	items []wanted.WantedItem
}

func (f fakeImportWantedStore) GetWanted(_ context.Context, id string) (wanted.WantedItem, error) {
	for _, item := range f.items {
		if item.ID == id {
			return item, nil
		}
	}
	return wanted.WantedItem{}, os.ErrNotExist
}

func (f fakeImportWantedStore) ListWanted(context.Context, string) ([]wanted.WantedItem, error) {
	return f.items, nil
}

func (f fakeImportWantedStore) MarkWantedStatus(context.Context, string, string) error {
	return nil
}

func TestParsedBookForPathReadsM4BMetadataAtoms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-name.m4b")
	writeTestM4B(t, path, map[string]string{
		"\xa9nam": "Project Hail Mary",
		"\xa9ART": "Andy Weir",
		"\xa9alb": "Project Hail Mary",
		"\xa9day": "2021",
		"\xa9gen": "Audiobook",
	})

	metadata := localBookMetadataForPath(path)
	if metadata.Source != "mp4-tags" || metadata.Title != "Project Hail Mary" || metadata.AuthorName != "Andy Weir" {
		t.Fatalf("expected M4B metadata, got %+v", metadata)
	}
	parsed := parsedBookForPath(path)
	if parsed.Title != "Project Hail Mary" || parsed.AuthorName != "Andy Weir" {
		t.Fatalf("expected M4B title/author, got %+v", parsed)
	}
}

func TestClassifyFile(t *testing.T) {
	tests := map[string]string{
		"book.epub": "ebook",
		"book.azw3": "ebook",
		"book.m4b":  "audiobook",
		"book.mp3":  "audiobook",
	}
	for path, expected := range tests {
		got, ok := classifyFile(path)
		if !ok || got != expected {
			t.Fatalf("expected %s to classify as %s, got %s ok=%v", path, expected, got, ok)
		}
	}
	if _, ok := classifyFile("cover.jpg"); ok {
		t.Fatal("expected unsupported extension to be rejected")
	}
}

func TestDestinationPathSanitizesSegments(t *testing.T) {
	service := NewService(nil, Config{EbookRoot: "/library/ebooks"}, nil, nil)
	got := service.destinationPath("ebook", parsedBook{AuthorName: `A/B:C`, Title: `Bad*Book?`}, ".EPUB")
	want := filepath.Join("/library/ebooks", "A-B-C", "Bad-Book-", "Bad-Book-.epub")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestDestinationPathUsesNamingPolicy(t *testing.T) {
	service := NewService(nil, Config{
		AudiobookRoot:              "/library/audio",
		NamingAuthorFolderTemplate: "{Author}",
		NamingBookFolderTemplate:   "{Format}/{Title}",
		NamingFileNameTemplate:     "{Author} - {Title}",
		NamingSpaceReplacement:     "_",
	}, nil, nil)
	got := service.destinationPath("audiobook", parsedBook{AuthorName: "Andy Weir", Title: "Project Hail Mary"}, ".m4b")
	want := filepath.Join("/library/audio", "Andy_Weir", "audiobook", "Project_Hail_Mary", "Andy_Weir_-_Project_Hail_Mary.m4b")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestRenamePreviewUsesNamingPolicy(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "old.epub")
	if err := os.WriteFile(source, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, Config{
		EbookRoot:                  dir,
		NamingAuthorFolderTemplate: "{Author}",
		NamingBookFolderTemplate:   "{Title}",
		NamingFileNameTemplate:     "{Author} - {Title}{Ext}",
		NamingSpaceReplacement:     "_",
	}, nil, nil)

	preview, err := service.renamePreviewForFile(FileRecord{
		ID:          "file-1",
		MediaFormat: "ebook",
		Path:        source,
		Title:       "Project Hail Mary",
		AuthorName:  "Andy Weir",
		Extension:   ".epub",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "Andy_Weir", "Project_Hail_Mary", "Andy_Weir_-_Project_Hail_Mary.epub")
	if preview.SourcePath != source || preview.DestinationPath != want || preview.Noop {
		t.Fatalf("unexpected preview: %+v want=%s", preview, want)
	}
}

func TestAvailableDestinationAvoidsOverwrite(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "Book.epub")
	if err := os.WriteFile(first, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := availableDestination(first)
	want := filepath.Join(dir, "Book (2).epub")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestPlanImportDestinationConflictActions(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "incoming.epub")
	destination := filepath.Join(dir, "Book.epub")
	if err := os.WriteFile(source, []byte("incoming"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	renamePlan, err := planImportDestination(source, destination, "rename")
	if err != nil {
		t.Fatal(err)
	}
	if renamePlan.DestinationPath != filepath.Join(dir, "Book (2).epub") || renamePlan.ConflictPath != destination || renamePlan.Replaced || renamePlan.Skipped {
		t.Fatalf("unexpected rename plan: %+v", renamePlan)
	}

	replacePlan, err := planImportDestination(source, destination, "replace")
	if err != nil {
		t.Fatal(err)
	}
	if replacePlan.DestinationPath != destination || !replacePlan.Replaced || replacePlan.Skipped {
		t.Fatalf("unexpected replace plan: %+v", replacePlan)
	}

	skipPlan, err := planImportDestination(source, destination, "skip")
	if err != nil {
		t.Fatal(err)
	}
	if !skipPlan.Skipped || skipPlan.Message == "" {
		t.Fatalf("unexpected skip plan: %+v", skipPlan)
	}

	if _, err := planImportDestination(source, destination, "fail"); err == nil {
		t.Fatal("expected fail conflict action to reject existing destination")
	}
}

func TestImportFileModes(t *testing.T) {
	t.Run("copy", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source.epub")
		destination := filepath.Join(dir, "destination.epub")
		if err := os.WriteFile(source, []byte("copy"), 0o644); err != nil {
			t.Fatal(err)
		}
		operation, err := importFile(source, destination, "copy", false)
		if err != nil {
			t.Fatal(err)
		}
		if operation.Mode != "copy" || operation.Moved || operation.Hardlinked {
			t.Fatalf("unexpected copy operation: %+v", operation)
		}
		if _, err := os.Stat(source); err != nil {
			t.Fatalf("expected source to remain, got %v", err)
		}
		if data, err := os.ReadFile(destination); err != nil || string(data) != "copy" {
			t.Fatalf("expected copied destination, data=%q err=%v", data, err)
		}
	})

	t.Run("move", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source.epub")
		destination := filepath.Join(dir, "destination.epub")
		if err := os.WriteFile(source, []byte("move"), 0o644); err != nil {
			t.Fatal(err)
		}
		operation, err := importFile(source, destination, "move", false)
		if err != nil {
			t.Fatal(err)
		}
		if operation.Mode != "move" || !operation.Moved {
			t.Fatalf("unexpected move operation: %+v", operation)
		}
		if _, err := os.Stat(source); !os.IsNotExist(err) {
			t.Fatalf("expected source to be moved away, stat err=%v", err)
		}
		if data, err := os.ReadFile(destination); err != nil || string(data) != "move" {
			t.Fatalf("expected moved destination, data=%q err=%v", data, err)
		}
	})

	t.Run("hardlink replace", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source.epub")
		destination := filepath.Join(dir, "destination.epub")
		if err := os.WriteFile(source, []byte("link"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		operation, err := importFile(source, destination, "hardlink", true)
		if err != nil {
			t.Fatal(err)
		}
		if operation.Mode != "hardlink" || !operation.Hardlinked || operation.Moved {
			t.Fatalf("unexpected hardlink operation: %+v", operation)
		}
		sourceInfo, err := os.Stat(source)
		if err != nil {
			t.Fatal(err)
		}
		destinationInfo, err := os.Stat(destination)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(sourceInfo, destinationInfo) {
			t.Fatal("expected source and destination to be hardlinks to the same file")
		}
	})
}

func TestRemoveLibraryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Book.epub")
	if err := os.WriteFile(path, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeLibraryFile(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed, stat err=%v", err)
	}
	if err := removeLibraryFile(path); err != nil {
		t.Fatalf("expected missing file cleanup to succeed, got %v", err)
	}
}

func TestCalibreSettingsForDestinationUsesBestMatchingRoot(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "books")
	child := filepath.Join(parent, "ebooks")
	service := NewService(nil, Config{}, nil, nil).WithCalibre(&fakeCalibreImporter{}, fakeRootFolders{roots: []compatdata.RootFolder{
		{
			Path: parent,
			Metadata: map[string]any{
				"isCalibreLibrary": true,
				"host":             "parent-calibre",
				"port":             8080,
			},
		},
		{
			Path: child,
			Metadata: map[string]any{
				"isCalibreLibrary": true,
				"host":             "child-calibre",
				"port":             float64(8081),
				"urlBase":          "/calibre",
				"username":         "reader",
				"password":         "secret",
				"library":          "Main",
				"outputFormat":     "EPUB,AZW3",
				"outputProfile":    "kindle",
				"useSsl":           "true",
			},
		},
	}})

	settings, ok, err := service.calibreSettingsForDestination(context.Background(), filepath.Join(child, "Andy Weir", "Project Hail Mary.epub"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected matching Calibre root")
	}
	if settings.Host != "child-calibre" || settings.Port != 8081 || settings.URLBase != "/calibre" || settings.Username != "reader" ||
		settings.Password != "secret" || settings.Library != "Main" || settings.OutputFormat != "EPUB,AZW3" ||
		settings.OutputProfile != "kindle" || !settings.UseSSL {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestApplyCalibreImportAddsCalibreMetadata(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ebooks")
	destination := filepath.Join(root, "Andy Weir", "Project Hail Mary.epub")
	importer := &fakeCalibreImporter{
		id: 77,
		convertResult: calibre.ConvertResult{
			Jobs:    []calibre.ConvertJob{{OutputFormat: "AZW3", JobID: 901}},
			Skipped: []string{"EPUB"},
		},
		conversionStatuses: []calibre.ConversionStatus{{
			OutputFormat: "AZW3",
			JobID:        901,
			Running:      true,
			OK:           false,
			Log:          "working",
		}},
	}
	service := NewService(nil, Config{}, nil, nil).WithCalibre(importer, fakeRootFolders{roots: []compatdata.RootFolder{{
		Path: root,
		Metadata: map[string]any{
			"isCalibreLibrary": true,
			"host":             "calibre.local",
			"library":          "Main",
			"outputFormat":     "EPUB,AZW3",
			"outputProfile":    "kindle",
		},
	}}})
	record := FileRecord{
		Path:       destination,
		Title:      "Project Hail Mary",
		AuthorName: "Andy Weir",
		Extension:  ".epub",
		Metadata:   map[string]any{"isbn13": "9780593135204"},
	}

	if err := service.applyCalibreImport(context.Background(), destination, &record); err != nil {
		t.Fatal(err)
	}
	if importer.request.Path != destination || importer.request.Settings.Host != "calibre.local" {
		t.Fatalf("unexpected importer request: %+v", importer.request)
	}
	if record.Metadata["calibreId"] != 77 || record.Metadata["calibreLibrary"] != "Main" ||
		record.Metadata["calibreOutputFormat"] != "EPUB,AZW3" || record.Metadata["calibreOutputProfile"] != "kindle" ||
		record.Metadata["calibreMetadataSyncedAt"] == "" {
		t.Fatalf("expected Calibre metadata, got %#v", record.Metadata)
	}
	if len(importer.setFieldsRequests) != 1 || importer.setFieldsRequests[0].ID != 77 ||
		importer.setFieldsRequests[0].Metadata.Title != "Project Hail Mary" ||
		len(importer.setFieldsRequests[0].Metadata.Authors) != 1 ||
		importer.setFieldsRequests[0].Metadata.Authors[0] != "Andy Weir" ||
		importer.setFieldsRequests[0].Metadata.Identifiers["isbn"] != "9780593135204" {
		t.Fatalf("unexpected Calibre set-fields request: %+v", importer.setFieldsRequests)
	}
	if len(importer.convertRequests) != 1 || importer.convertRequests[0].ID != 77 ||
		importer.convertRequests[0].InputFormat != ".epub" ||
		importer.convertRequests[0].Settings.OutputFormat != "EPUB,AZW3" {
		t.Fatalf("unexpected Calibre conversion request: %+v", importer.convertRequests)
	}
	jobs, _ := record.Metadata["calibreConversionJobs"].([]map[string]any)
	if len(jobs) != 1 || jobs[0]["outputFormat"] != "AZW3" || jobs[0]["jobId"] != int64(901) ||
		record.Metadata["calibreConversionStartedAt"] == "" {
		t.Fatalf("expected conversion metadata, got %#v", record.Metadata)
	}
	if len(importer.pollRequests) != 1 || importer.pollRequests[0].MaxAttempts != 1 ||
		len(importer.pollRequests[0].Jobs) != 1 || importer.pollRequests[0].Jobs[0].JobID != 901 {
		t.Fatalf("unexpected Calibre poll request: %+v", importer.pollRequests)
	}
	statuses, _ := record.Metadata["calibreConversionStatuses"].([]map[string]any)
	if len(statuses) != 1 || statuses[0]["jobId"] != int64(901) || statuses[0]["running"] != true ||
		statuses[0]["ok"] != false || statuses[0]["log"] != "working" ||
		record.Metadata["calibreConversionPolledAt"] == "" {
		t.Fatalf("expected conversion status metadata, got %#v", record.Metadata)
	}
}

func TestApplyCalibreDeleteUsesStoredCalibreID(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ebooks")
	destination := filepath.Join(root, "Andy Weir", "Project Hail Mary.epub")
	importer := &fakeCalibreImporter{}
	service := NewService(nil, Config{}, nil, nil).WithCalibre(importer, fakeRootFolders{roots: []compatdata.RootFolder{{
		Path: root,
		Metadata: map[string]any{
			"isCalibreLibrary": true,
			"host":             "calibre.local",
			"library":          "Main",
		},
	}}})

	err := service.applyCalibreDelete(context.Background(), FileRecord{
		Path:     destination,
		Metadata: map[string]any{"calibreId": float64(88)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(importer.deleteRequests) != 1 || importer.deleteRequests[0].IDs[0] != 88 ||
		importer.deleteRequests[0].Settings.Host != "calibre.local" ||
		importer.deleteRequests[0].Settings.Library != "Main" {
		t.Fatalf("unexpected Calibre delete request: %+v", importer.deleteRequests)
	}
}

func TestApplyCalibreDeleteSkipsFilesWithoutCalibreID(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ebooks")
	importer := &fakeCalibreImporter{}
	service := NewService(nil, Config{}, nil, nil).WithCalibre(importer, fakeRootFolders{roots: []compatdata.RootFolder{{
		Path:     root,
		Metadata: map[string]any{"isCalibreLibrary": true, "host": "calibre.local"},
	}}})

	if err := service.applyCalibreDelete(context.Background(), FileRecord{
		Path:     filepath.Join(root, "Book.epub"),
		Metadata: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}
	if len(importer.deleteRequests) != 0 {
		t.Fatalf("expected no Calibre delete request, got %+v", importer.deleteRequests)
	}
}

func TestCalibreConversionMetadataParsesJSONShapes(t *testing.T) {
	metadata := map[string]any{
		"calibreConversionJobs": []any{
			map[string]any{"outputFormat": "AZW3", "jobId": float64(901)},
			map[string]any{"outputFormat": "MOBI", "jobId": "902"},
			map[string]any{"outputFormat": "ignored", "jobId": float64(0)},
		},
		"calibreConversionStatuses": []any{
			map[string]any{"outputFormat": "AZW3", "jobId": float64(901), "running": true, "ok": false, "wasAborted": false, "log": "working"},
			map[string]any{"outputFormat": "MOBI", "jobId": "902", "running": "false", "ok": "true", "wasAborted": "false"},
		},
	}
	jobs := calibreConversionJobsFromMetadata(metadata)
	if len(jobs) != 2 || jobs[0].OutputFormat != "AZW3" || jobs[0].JobID != 901 ||
		jobs[1].OutputFormat != "MOBI" || jobs[1].JobID != 902 {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
	statuses := calibreConversionStatusesFromMetadata(metadata)
	if len(statuses) != 2 || !statuses[0].Running || statuses[0].Log != "working" ||
		statuses[1].Running || !statuses[1].OK {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
}

func TestCalibreConversionNeedsRefreshUntilAllJobsTerminal(t *testing.T) {
	jobs := []calibre.ConvertJob{{OutputFormat: "AZW3", JobID: 901}, {OutputFormat: "MOBI", JobID: 902}}
	if !calibreConversionNeedsRefresh(jobs, nil) {
		t.Fatal("expected refresh when statuses are missing")
	}
	if !calibreConversionNeedsRefresh(jobs, []calibre.ConversionStatus{{JobID: 901, Running: false, OK: true}}) {
		t.Fatal("expected refresh when a job is missing a status")
	}
	if !calibreConversionNeedsRefresh(jobs, []calibre.ConversionStatus{{JobID: 901, Running: true}, {JobID: 902, Running: false, OK: true}}) {
		t.Fatal("expected refresh while any job is running")
	}
	if calibreConversionNeedsRefresh(jobs, []calibre.ConversionStatus{{JobID: 901, Running: false, OK: true}, {JobID: 902, Running: false, OK: false}}) {
		t.Fatal("expected terminal success/failure statuses to stop refresh")
	}
	if !calibreConversionAnyFailed([]calibre.ConversionStatus{{JobID: 902, Running: false, OK: false}}) {
		t.Fatal("expected failed terminal status to be detected")
	}
}

func TestIsCompletedDownload(t *testing.T) {
	completedAt := time.Now().UTC()
	if !isCompletedDownload(acquisition.DownloadStatus{Progress: 1}) {
		t.Fatal("expected progress 1 download to be complete")
	}
	if !isCompletedDownload(acquisition.DownloadStatus{State: "uploading"}) {
		t.Fatal("expected uploading download to be complete")
	}
	if !isCompletedDownload(acquisition.DownloadStatus{CompletedAt: &completedAt}) {
		t.Fatal("expected completed_at download to be complete")
	}
	if isCompletedDownload(acquisition.DownloadStatus{State: "downloading", Progress: 0.5}) {
		t.Fatal("expected partial download to be incomplete")
	}
}

type fakeRootFolders struct {
	roots []compatdata.RootFolder
}

func (f fakeRootFolders) ListRootFolders(context.Context) ([]compatdata.RootFolder, error) {
	return append([]compatdata.RootFolder(nil), f.roots...), nil
}

type fakeCalibreImporter struct {
	id                 int
	request            calibre.AddBookRequest
	deleteRequests     []calibre.DeleteBooksRequest
	setFieldsRequests  []calibre.SetFieldsRequest
	convertRequests    []calibre.ConvertRequest
	convertResult      calibre.ConvertResult
	pollRequests       []calibre.PollConversionsRequest
	conversionStatuses []calibre.ConversionStatus
}

func (f *fakeCalibreImporter) AddBook(_ context.Context, request calibre.AddBookRequest) (calibre.AddBookResult, error) {
	f.request = request
	id := f.id
	if id <= 0 {
		id = 1
	}
	return calibre.AddBookResult{ID: id}, nil
}

func (f *fakeCalibreImporter) DeleteBooks(_ context.Context, request calibre.DeleteBooksRequest) error {
	f.deleteRequests = append(f.deleteRequests, request)
	return nil
}

func (f *fakeCalibreImporter) SetFields(_ context.Context, request calibre.SetFieldsRequest) error {
	f.setFieldsRequests = append(f.setFieldsRequests, request)
	return nil
}

func (f *fakeCalibreImporter) Convert(_ context.Context, request calibre.ConvertRequest) (calibre.ConvertResult, error) {
	f.convertRequests = append(f.convertRequests, request)
	return f.convertResult, nil
}

func (f *fakeCalibreImporter) PollConversions(_ context.Context, request calibre.PollConversionsRequest) ([]calibre.ConversionStatus, error) {
	f.pollRequests = append(f.pollRequests, request)
	return f.conversionStatuses, nil
}

func writeTestEPUB(t *testing.T, path string, opf string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	writeZipEntry(t, writer, "META-INF/container.xml", `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`)
	writeZipEntry(t, writer, "OEBPS/content.opf", opf)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeZipEntry(t *testing.T, writer *zip.Writer, name string, body string) {
	t.Helper()
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
}

func writeTestMP3WithID3(t *testing.T, path string, frames map[string]string) {
	t.Helper()
	var payload []byte
	for id, value := range frames {
		framePayload := append([]byte{3}, []byte(value)...)
		header := make([]byte, 10)
		copy(header[:4], []byte(id))
		binary.BigEndian.PutUint32(header[4:8], uint32(len(framePayload)))
		payload = append(payload, header...)
		payload = append(payload, framePayload...)
	}
	header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}
	putSyncsafe(header[6:10], len(payload))
	body := append(header, payload...)
	body = append(body, []byte("audio")...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func putSyncsafe(target []byte, value int) {
	target[0] = byte((value >> 21) & 0x7f)
	target[1] = byte((value >> 14) & 0x7f)
	target[2] = byte((value >> 7) & 0x7f)
	target[3] = byte(value & 0x7f)
}

func writeTestM4B(t *testing.T, path string, items map[string]string) {
	t.Helper()
	var ilst []byte
	for atomType, value := range items {
		ilst = append(ilst, mp4Atom([]byte(atomType), mp4DataAtom(value))...)
	}
	metaPayload := append([]byte{0, 0, 0, 0}, mp4Atom([]byte("ilst"), ilst)...)
	moov := mp4Atom([]byte("moov"), mp4Atom([]byte("udta"), mp4Atom([]byte("meta"), metaPayload)))
	ftyp := mp4Atom([]byte("ftyp"), []byte("M4B \x00\x00\x00\x00M4B "))
	if err := os.WriteFile(path, append(ftyp, moov...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mp4DataAtom(value string) []byte {
	payload := append([]byte{0, 0, 0, 1, 0, 0, 0, 0}, []byte(value)...)
	return mp4Atom([]byte("data"), payload)
}

func mp4Atom(atomType []byte, payload []byte) []byte {
	if len(atomType) != 4 {
		panic("mp4 atom type must be four bytes")
	}
	body := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(body[:4], uint32(len(body)))
	copy(body[4:8], atomType)
	copy(body[8:], payload)
	return body
}

func TestLocateDownloadSourceFindsNamedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Andy Weir - Project Hail Mary.epub")
	if err := os.WriteFile(path, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, format, err := locateDownloadSource(acquisition.DownloadStatus{
		Name:     filepath.Base(path),
		SavePath: dir,
		Category: "books-ebook",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != path || format != "ebook" {
		t.Fatalf("expected %s ebook, got %s %s", path, got, format)
	}
}

func TestLocateDownloadSourceFindsBestFileInFolder(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "Book Folder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	small := filepath.Join(folder, "sample.epub")
	large := filepath.Join(folder, "full.epub")
	if err := os.WriteFile(small, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(large, []byte("full book"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, format, err := locateDownloadSource(acquisition.DownloadStatus{
		Name:     "Book Folder",
		SavePath: dir,
		Category: "books-ebook",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != large || format != "ebook" {
		t.Fatalf("expected %s ebook, got %s %s", large, got, format)
	}
}
