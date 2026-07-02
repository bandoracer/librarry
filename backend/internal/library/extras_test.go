package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportExtraExtensions(t *testing.T) {
	got := importExtraExtensions(" .CUE, nfo,, .cue ")
	if len(got) != 2 || got[0] != ".cue" || got[1] != ".nfo" {
		t.Fatalf("expected normalized deduped extensions, got %v", got)
	}
	if got := importExtraExtensions(""); len(got) != 0 {
		t.Fatalf("expected no extensions for empty config, got %v", got)
	}
}

func TestSiblingExtraFilesMatchesBasenameAndExtension(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "Album Book.m4b")
	for _, name := range []string{
		"Album Book.m4b",  // the source itself
		"Album Book.cue",  // match
		"album book.CUE",  // case-insensitive match on darwin/linux alike
		"Album Book.nfo",  // extension not configured
		"Other Book.cue",  // different basename
		"Album Book.epub", // not an extra extension
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	extras, err := siblingExtraFiles(source, []string{".cue"})
	if err != nil {
		t.Fatal(err)
	}
	if len(extras) == 0 {
		t.Fatal("expected at least one matching extra")
	}
	for _, extra := range extras {
		if !strings.EqualFold(filepath.Base(extra), "Album Book.cue") {
			t.Fatalf("unexpected extra matched: %s", extra)
		}
	}
}

func TestCopyImportExtrasRenamesToDestinationBasename(t *testing.T) {
	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	source := filepath.Join(sourceDir, "download-name.m4b")
	destination := filepath.Join(destinationDir, "Project Hail Mary.m4b")
	if err := os.WriteFile(source, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "download-name.cue"), []byte("cue sheet"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := NewService(nil, Config{ImportExtraFiles: ".cue"}, nil, nil)
	service.copyImportExtras(source, destination)

	data, err := os.ReadFile(filepath.Join(destinationDir, "Project Hail Mary.cue"))
	if err != nil || string(data) != "cue sheet" {
		t.Fatalf("expected extra copied alongside import, err=%v data=%q", err, data)
	}
}

func TestCopyImportExtrasSkipsExistingTargets(t *testing.T) {
	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	source := filepath.Join(sourceDir, "book.m4b")
	destination := filepath.Join(destinationDir, "Book.m4b")
	if err := os.WriteFile(source, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "book.cue"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(destinationDir, "Book.cue")
	if err := os.WriteFile(existing, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := NewService(nil, Config{ImportExtraFiles: ".cue"}, nil, nil)
	service.copyImportExtras(source, destination)

	data, err := os.ReadFile(existing)
	if err != nil || string(data) != "existing" {
		t.Fatalf("expected existing extra to be preserved, err=%v data=%q", err, data)
	}
}
