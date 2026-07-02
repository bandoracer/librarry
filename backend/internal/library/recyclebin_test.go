package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscardLibraryFileMovesIntoBin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "recycle")
	source := filepath.Join(dir, "Book.epub")
	if err := os.WriteFile(source, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	if err := discardLibraryFile(bin, source, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("expected source to be moved away, stat err=%v", err)
	}
	recycled := filepath.Join(bin, "2026-07-01", "Book.epub")
	data, err := os.ReadFile(recycled)
	if err != nil || string(data) != "book" {
		t.Fatalf("expected recycled file at %s, err=%v data=%q", recycled, err, data)
	}
}

func TestDiscardLibraryFileKeepsBothWhenNameCollides(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "recycle")
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	day := filepath.Join(bin, "2026-07-01")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(day, "Book.epub"), []byte("earlier"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "Book.epub")
	if err := os.WriteFile(source, []byte("later"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := discardLibraryFile(bin, source, now); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(day, "Book (2).epub"))
	if err != nil || string(data) != "later" {
		t.Fatalf("expected renamed recycled copy, err=%v data=%q", err, data)
	}
}

func TestDiscardLibraryFileFallsBackToRemoveWhenBinUnusable(t *testing.T) {
	dir := t.TempDir()
	// The "bin" is a regular file, so MkdirAll inside it fails.
	bin := filepath.Join(dir, "recycle")
	if err := os.WriteFile(bin, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "Book.epub")
	if err := os.WriteFile(source, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := discardLibraryFile(bin, source, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("expected fallback remove to delete the file, stat err=%v", err)
	}
}

func TestDiscardLibraryFileWithoutBinRemoves(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "Book.epub")
	if err := os.WriteFile(source, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := discardLibraryFile("", source, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("expected plain remove without a bin, stat err=%v", err)
	}
}

func TestCleanupRecycleBinRemovesExpiredDayFolders(t *testing.T) {
	bin := t.TempDir()
	for _, day := range []string{"2026-06-01", "2026-06-30", "not-a-day"} {
		if err := os.MkdirAll(filepath.Join(bin, day), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, day, "file.epub"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	removed, err := cleanupRecycleBin(bin, 168*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("expected exactly one expired day folder removed, got %d", removed)
	}
	if _, err := os.Stat(filepath.Join(bin, "2026-06-01")); !os.IsNotExist(err) {
		t.Fatalf("expected 2026-06-01 to be purged, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(bin, "2026-06-30")); err != nil {
		t.Fatalf("expected 2026-06-30 to remain within retention: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bin, "not-a-day")); err != nil {
		t.Fatalf("expected foreign folders to be left alone: %v", err)
	}
}

func TestCleanupRecycleBinDisabledOrMissing(t *testing.T) {
	if removed, err := cleanupRecycleBin("", time.Hour, time.Now()); err != nil || removed != 0 {
		t.Fatalf("expected disabled bin to no-op, removed=%d err=%v", removed, err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if removed, err := cleanupRecycleBin(missing, time.Hour, time.Now()); err != nil || removed != 0 {
		t.Fatalf("expected missing bin to no-op, removed=%d err=%v", removed, err)
	}
}

func TestServiceCleanupRecycleBinUsesConfig(t *testing.T) {
	bin := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bin, "2020-01-01"), 0o755); err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, Config{RecycleBin: bin}, nil, nil)
	removed, err := service.CleanupRecycleBin(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || removed != 1 {
		t.Fatalf("expected default retention (168h) cleanup through the service, removed=%d err=%v", removed, err)
	}
}
