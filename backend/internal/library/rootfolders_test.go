package library

import (
	"context"
	"errors"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestDefaultRootFolderPathPrefersFlaggedDefault(t *testing.T) {
	folders := []RootFolder{
		{ID: "1", Path: "/library/ebooks-old", MediaFormat: "ebook"},
		{ID: "2", Path: "/library/ebooks", MediaFormat: "ebook", IsDefault: true},
		{ID: "3", Path: "/library/audio", MediaFormat: "audiobook"},
	}
	if got := defaultRootFolderPath(folders, "ebook"); got != "/library/ebooks" {
		t.Fatalf("expected flagged default, got %s", got)
	}
	if got := defaultRootFolderPath(folders, "audiobook"); got != "/library/audio" {
		t.Fatalf("expected first format root when no default flagged, got %s", got)
	}
	if got := defaultRootFolderPath(nil, "ebook"); got != "" {
		t.Fatalf("expected empty path without roots, got %s", got)
	}
}

func TestRootFolderPathByID(t *testing.T) {
	folders := []RootFolder{
		{ID: "abc", Path: "/library/special", MediaFormat: "ebook"},
	}
	if got := rootFolderPathByID(folders, "abc"); got != "/library/special" {
		t.Fatalf("expected explicit root path, got %s", got)
	}
	if got := rootFolderPathByID(folders, "missing"); got != "" {
		t.Fatalf("expected empty path for unknown id, got %s", got)
	}
}

func TestImportRootPathFallsBackToLegacyConfig(t *testing.T) {
	service := NewService(nil, Config{
		EbookRoot:     "/legacy/ebooks",
		AudiobookRoot: "/legacy/audio",
	}, nil, nil)
	if got := service.importRootPath(context.Background(), "ebook", ""); got != "/legacy/ebooks" {
		t.Fatalf("expected legacy ebook root, got %s", got)
	}
	if got := service.importRootPath(context.Background(), "audiobook", "any-id"); got != "/legacy/audio" {
		t.Fatalf("expected legacy audiobook root, got %s", got)
	}
}

func TestScanRootsWithoutNativeRootsUsesLegacyConfig(t *testing.T) {
	service := NewService(nil, Config{
		EbookRoot:     "/legacy/ebooks",
		AudiobookRoot: "/legacy/audio",
	}, nil, nil)
	roots := service.scanRoots(context.Background(), ScanRequest{})
	if len(roots) != 2 || roots[0] != "/legacy/ebooks" || roots[1] != "/legacy/audio" {
		t.Fatalf("expected both legacy roots, got %v", roots)
	}
	roots = service.scanRoots(context.Background(), ScanRequest{Format: "audiobook"})
	if len(roots) != 1 || roots[0] != "/legacy/audio" {
		t.Fatalf("expected audiobook root only, got %v", roots)
	}
	roots = service.scanRoots(context.Background(), ScanRequest{Root: "/explicit"})
	if len(roots) != 1 || roots[0] != "/explicit" {
		t.Fatalf("expected explicit root override, got %v", roots)
	}
}

func TestNormalizeRootFolderInput(t *testing.T) {
	folder, err := normalizeRootFolderInput(RootFolder{Path: " /library/books/ ", MediaFormat: "Ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if folder.Path != "/library/books" || folder.MediaFormat != "ebook" || folder.Name != "books" {
		t.Fatalf("unexpected normalized folder: %+v", folder)
	}

	if _, err := normalizeRootFolderInput(RootFolder{MediaFormat: "ebook"}); err == nil {
		t.Fatal("expected missing path to error")
	}
	if _, err := normalizeRootFolderInput(RootFolder{Path: "/x", MediaFormat: "vinyl"}); err == nil {
		t.Fatal("expected invalid media format to error")
	}
	// Empty format defaults to ebook.
	folder, err = normalizeRootFolderInput(RootFolder{Path: "/x"})
	if err != nil || folder.MediaFormat != "ebook" {
		t.Fatalf("expected ebook default format, got %+v err=%v", folder, err)
	}
}

func TestConflictErrorMatchesWithErrorsAs(t *testing.T) {
	var conflict *ConflictError
	err := error(&ConflictError{Reason: "cannot delete"})
	if !errors.As(err, &conflict) || conflict.Reason != "cannot delete" {
		t.Fatalf("expected errors.As to match ConflictError, got %v", err)
	}
}

func TestWantedOverrideValueWinsOverProviderData(t *testing.T) {
	item := wanted.WantedItem{
		Series: "Provider Series",
		ManualOverrides: []wanted.ManualOverride{
			{FieldName: "series", Value: "Corrected Series"},
		},
	}
	if got := wantedOverrideValue(item, "series"); got != "Corrected Series" {
		t.Fatalf("expected manual override value, got %q", got)
	}
	if got := wantedOverrideValue(item, "series_position"); got != "" {
		t.Fatalf("expected empty value for missing override, got %q", got)
	}
}
