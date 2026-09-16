package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestPayloadCannotSelectSiblingOrEscape(t *testing.T) {
	for _, name := range []string{"missing.epub", "", "../foreign.epub", "/foreign.epub"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "foreign.epub"), []byte("unrelated"), 0600); err != nil {
				t.Fatal(err)
			}
			if path, _, err := locateDownloadSource(acquisition.DownloadStatus{SavePath: dir, Name: name}); err == nil {
				t.Fatalf("selected %s", path)
			}
		})
	}
}

func TestPayloadRejectsChapterSetAndSymlink(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		dir := t.TempDir()
		payload := filepath.Join(dir, "Book")
		if err := os.Mkdir(payload, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(payload, "01.mp3"), []byte("chapter one"), 0600); err != nil {
			t.Fatal(err)
		}
		if symlink {
			if err := os.Symlink(filepath.Join(payload, "01.mp3"), filepath.Join(payload, "02.mp3")); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(filepath.Join(payload, "02.mp3"), []byte("chapter two"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := locateDownloadSource(acquisition.DownloadStatus{SavePath: dir, Name: "Book"}); err == nil {
			t.Fatal("unsafe payload accepted")
		}
	}
}

func TestScanPreservesImportedIdentity(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	root := t.TempDir()
	path := filepath.Join(root, "guessed title.epub")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	original, err := store.UpsertFile(ctx, FileRecord{Path: path, SourcePath: "/downloads/original.epub", MediaFormat: "ebook", Title: "Manual title", AuthorName: "Manual author", ImportStatus: "imported", Metadata: map[string]any{"wantedId": "wanted-1", "downloadId": "download-1", "calibreId": 42, "manualOverride": true}})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, Config{}, nil, nil)
	for i := 0; i < 2; i++ {
		outcome, err := service.Scan(ctx, ScanRequest{Root: root})
		if err != nil || len(outcome.Errors) > 0 || len(outcome.Files) != 1 {
			t.Fatalf("scan: %+v %v", outcome, err)
		}
		file := outcome.Files[0]
		if file.ID != original.ID || file.Title != original.Title || file.AuthorName != original.AuthorName || file.SourcePath != original.SourcePath || file.ImportStatus != "imported" {
			t.Fatalf("identity lost: %+v", file)
		}
		if file.Metadata["wantedId"] != "wanted-1" || file.Metadata["downloadId"] != "download-1" || file.Metadata["calibreId"] != float64(42) || file.Metadata["manualOverride"] != true {
			t.Fatalf("provenance lost: %+v", file.Metadata)
		}
	}
}

func TestCleanupReverifiesSingleFileReceipt(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "downloads")
	destinationRoot := filepath.Join(root, "library")
	for _, dir := range []string{sourceRoot, destinationRoot} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(sourceRoot, "Book.epub")
	destination := filepath.Join(destinationRoot, "Book.epub")
	content := []byte("complete book")
	for _, path := range []string{source, destination} {
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := contentHash(source)
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.UpsertFile(ctx, FileRecord{Path: destination, SourcePath: source, MediaFormat: "ebook", ImportStatus: "imported", Metadata: map[string]any{"verifiedDownload": map[string]any{"client": "qBittorrent", "id": "fixture", "sha256": hash}}})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, Config{}, nil, nil)
	download := acquisition.DownloadStatus{Client: "qBittorrent", ID: "fixture", Name: "Book.epub", SavePath: sourceRoot, ImportedFileID: record.ID}
	inventory := []acquisition.DownloadFile{{Name: "Book.epub", SizeBytes: int64(len(content)), Progress: 1}}
	if err := service.VerifyCompletedDownload(ctx, download, inventory); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []acquisition.DownloadStatus{
		{Client: "Transmission", ID: "fixture", Name: "Book.epub", SavePath: sourceRoot, ImportedFileID: record.ID},
		{Client: "qBittorrent", ID: "different", Name: "Book.epub", SavePath: sourceRoot, ImportedFileID: record.ID},
	} {
		if err := service.VerifyCompletedDownload(ctx, invalid, inventory); err == nil {
			t.Fatal("foreign download accepted")
		}
	}
	if err := service.VerifyCompletedDownload(ctx, download, nil); err == nil {
		t.Fatal("missing inventory accepted")
	}
	if err := os.WriteFile(destination, []byte("damaged file!"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyCompletedDownload(ctx, download, inventory); err == nil {
		t.Fatal("corrupt destination accepted")
	}
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyCompletedDownload(ctx, download, inventory); err == nil {
		t.Fatal("missing destination accepted")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("verification touched source")
	}
}

func TestFailedReplacementKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "old.epub")
	if err := os.WriteFile(destination, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := importFile(filepath.Join(dir, "missing.epub"), destination, "copy", true); err == nil {
		t.Fatal("missing source accepted")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "original" {
		t.Fatalf("original lost: %q %v", data, err)
	}
}

func TestCopyCannotTruncateExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "new.epub")
	destination := filepath.Join(dir, "old.epub")
	for path, body := range map[string]string{source: "new", destination: "original"} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyFile(source, destination); err == nil {
		t.Fatal("overwrote an existing file")
	}
	data, _ := os.ReadFile(destination)
	if string(data) != "original" {
		t.Fatal("destination truncated")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("staging files leaked")
	}
}
