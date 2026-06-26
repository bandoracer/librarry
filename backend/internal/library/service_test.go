package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/calibre"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
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
