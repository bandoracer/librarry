package library

import (
	"os"
	"path/filepath"
	"testing"
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
	service := NewService(nil, Config{EbookRoot: "/library/ebooks"}, nil)
	got := service.destinationPath("ebook", parsedBook{AuthorName: `A/B:C`, Title: `Bad*Book?`}, ".EPUB")
	want := filepath.Join("/library/ebooks", "A-B-C", "Bad-Book-", "Bad-Book-.epub")
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
