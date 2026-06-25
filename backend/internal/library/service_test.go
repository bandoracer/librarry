package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
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
