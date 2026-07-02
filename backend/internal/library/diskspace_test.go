package library

import (
	"testing"
)

func TestDiskSpacesDedupesByFilesystem(t *testing.T) {
	probes := map[string]diskProbe{
		"/data/media/books/ebooks":     {free: 100, total: 500, device: 7, ok: true},
		"/data/media/books/audiobooks": {free: 100, total: 500, device: 7, ok: true},
		"/data/torrents/books":         {free: 100, total: 500, device: 7, ok: true},
		"/mnt/other":                   {free: 20, total: 80, device: 9, ok: true},
	}
	probe := func(path string) diskProbe { return probes[path] }

	disks := diskSpacesWithProbe([]DiskPath{
		{Path: "/data/media/books/ebooks", Label: "Ebooks"},
		{Path: "/data/media/books/audiobooks", Label: "Audiobooks"},
		{Path: "/data/torrents/books", Label: "Book torrents"},
		{Path: "/mnt/other", Label: "Other"},
	}, probe)

	if len(disks) != 2 {
		t.Fatalf("expected 2 deduped disks, got %d: %+v", len(disks), disks)
	}
	if disks[0].Path != "/data/media/books/ebooks" || disks[0].Label != "Ebooks" {
		t.Fatalf("expected first path on the shared filesystem to win, got %+v", disks[0])
	}
	if disks[0].FreeBytes != 100 || disks[0].TotalBytes != 500 {
		t.Fatalf("unexpected usage: %+v", disks[0])
	}
	if disks[1].Path != "/mnt/other" || disks[1].TotalBytes != 80 {
		t.Fatalf("unexpected second disk: %+v", disks[1])
	}
}

func TestDiskSpacesSkipsUnprobeablePaths(t *testing.T) {
	probe := func(path string) diskProbe {
		if path == "/exists" {
			return diskProbe{free: 1, total: 2, device: 1, ok: true}
		}
		return diskProbe{}
	}
	disks := diskSpacesWithProbe([]DiskPath{
		{Path: "", Label: "empty"},
		{Path: "/missing", Label: "missing"},
		{Path: "/exists", Label: "exists"},
	}, probe)
	if len(disks) != 1 || disks[0].Path != "/exists" {
		t.Fatalf("expected only the probeable path, got %+v", disks)
	}
}

func TestProbeDiskRealFilesystem(t *testing.T) {
	dir := t.TempDir()
	result := probeDisk(dir)
	if !result.ok {
		t.Skip("statfs unsupported on this platform")
	}
	if result.total <= 0 || result.free < 0 || result.free > result.total {
		t.Fatalf("implausible disk probe: %+v", result)
	}
}
