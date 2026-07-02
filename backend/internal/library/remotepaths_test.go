package library

import "testing"

func TestRewriteRemotePathLongestPrefixWins(t *testing.T) {
	mappings := []RemotePathMapping{
		{RemotePrefix: "/remote", LocalPrefix: "/mnt/remote"},
		{RemotePrefix: "/remote/torrents", LocalPrefix: "/data/torrents"},
	}
	got := rewriteRemotePath("/remote/torrents/books/file.epub", "qbittorrent", mappings)
	want := "/data/torrents/books/file.epub"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestRewriteRemotePathHostFiltering(t *testing.T) {
	mappings := []RemotePathMapping{
		{Host: "transmission", RemotePrefix: "/remote/torrents", LocalPrefix: "/transmission"},
		{Host: "", RemotePrefix: "/remote", LocalPrefix: "/any"},
	}
	if got := rewriteRemotePath("/remote/torrents/file.epub", "qBittorrent", mappings); got != "/any/torrents/file.epub" {
		t.Fatalf("expected empty-host mapping for other clients, got %s", got)
	}
	if got := rewriteRemotePath("/remote/torrents/file.epub", "Transmission", mappings); got != "/transmission/file.epub" {
		t.Fatalf("expected host-specific mapping to win by prefix length, got %s", got)
	}
}

func TestRewriteRemotePathRequiresBoundaryMatch(t *testing.T) {
	mappings := []RemotePathMapping{
		{RemotePrefix: "/data/tor", LocalPrefix: "/elsewhere"},
	}
	original := "/data/torrents/file.epub"
	if got := rewriteRemotePath(original, "", mappings); got != original {
		t.Fatalf("expected non-boundary prefix to be ignored, got %s", got)
	}
}

func TestRewriteRemotePathNoMappingsKeepsPath(t *testing.T) {
	original := "/data/torrents/file.epub"
	if got := rewriteRemotePath(original, "qbittorrent", nil); got != original {
		t.Fatalf("expected original path, got %s", got)
	}
	if got := rewriteRemotePath(original, "qbittorrent", []RemotePathMapping{{RemotePrefix: "/other", LocalPrefix: "/x"}}); got != original {
		t.Fatalf("expected original path without a matching prefix, got %s", got)
	}
}

func TestRewriteRemotePathTrailingSlashAndExactMatch(t *testing.T) {
	mappings := []RemotePathMapping{
		{RemotePrefix: "/remote/downloads/", LocalPrefix: "/local/downloads/"},
	}
	if got := rewriteRemotePath("/remote/downloads/book.epub", "", mappings); got != "/local/downloads/book.epub" {
		t.Fatalf("expected trailing-slash prefixes to normalize, got %s", got)
	}
	if got := rewriteRemotePath("/remote/downloads", "", mappings); got != "/local/downloads" {
		t.Fatalf("expected exact prefix match to rewrite, got %s", got)
	}
}

func TestRewriteRemotePathWindowsRemote(t *testing.T) {
	mappings := []RemotePathMapping{
		{RemotePrefix: `C:\Downloads`, LocalPrefix: "/data/downloads"},
	}
	if got := rewriteRemotePath(`C:\Downloads\books\file.epub`, "", mappings); got != "/data/downloads/books/file.epub" {
		t.Fatalf("expected windows remote prefix rewrite, got %s", got)
	}
}
