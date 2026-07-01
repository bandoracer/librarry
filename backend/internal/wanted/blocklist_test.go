package wanted

import (
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func TestBlocklistMatchesReleaseByInfoHash(t *testing.T) {
	entries := []BlocklistEntry{{InfoHash: "aabbccddeeff00112233445566778899aabbccdd"}}
	release := acquisition.Release{InfoHash: "AABBCCDDEEFF00112233445566778899AABBCCDD"}
	if !blocklistMatchesRelease(entries, release) {
		t.Fatal("expected infohash match to be case-insensitive")
	}
	if blocklistMatchesRelease(entries, acquisition.Release{InfoHash: "0000000000000000000000000000000000000000"}) {
		t.Fatal("expected different infohash to miss")
	}
}

func TestBlocklistMatchesReleaseByDownloadURLHash(t *testing.T) {
	url := "https://indexer.example/dl/release.torrent?key=abc"
	entries := []BlocklistEntry{{DownloadURLHash: hashDownloadURL(url)}}
	if !blocklistMatchesRelease(entries, acquisition.Release{DownloadURL: url}) {
		t.Fatal("expected download URL hash match")
	}
	if blocklistMatchesRelease(entries, acquisition.Release{DownloadURL: url + "&other=1"}) {
		t.Fatal("expected different download URL to miss")
	}
}

func TestBlocklistMatchesReleaseByTitleAndIndexerFallback(t *testing.T) {
	entries := []BlocklistEntry{{Title: "Andy Weir - Project Hail Mary EPUB", Indexer: "Public Indexer"}}
	match := acquisition.Release{Title: "Andy Weir - Project Hail Mary [EPUB]", Indexer: "public indexer"}
	if !blocklistMatchesRelease(entries, match) {
		t.Fatal("expected normalized title+indexer match")
	}
	otherIndexer := acquisition.Release{Title: "Andy Weir - Project Hail Mary EPUB", Indexer: "Other Indexer"}
	if blocklistMatchesRelease(entries, otherIndexer) {
		t.Fatal("expected different indexer to miss")
	}
	titleOnly := []BlocklistEntry{{Title: "Andy Weir - Project Hail Mary EPUB"}}
	if blocklistMatchesRelease(titleOnly, match) {
		t.Fatal("expected title-only entry without indexer to skip the fallback")
	}
}

func TestEvaluateReleaseRejectsBlocklistedRelease(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		InfoHash:    "aabbccddeeff00112233445566778899aabbccdd",
		Title:       "Project Hail Mary by Andy Weir EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	profile := defaultQualityProfile("standard", "ebook")

	decision := evaluateReleaseWithBlocklist(item, release, profile, nil, nil)
	if !decision.Approved {
		t.Fatalf("expected release to be approved without blocklist, got %q", decision.RejectedReason)
	}

	blocklist := []BlocklistEntry{{InfoHash: release.InfoHash}}
	decision = evaluateReleaseWithBlocklist(item, release, profile, nil, blocklist)
	if decision.Approved {
		t.Fatal("expected blocklisted release to be rejected")
	}
	if decision.RejectedReason != blocklistRejectionReason {
		t.Fatalf("expected rejection reason %q, got %q", blocklistRejectionReason, decision.RejectedReason)
	}
}

func TestBlocklistEntryForDownloadDerivesInfohashFromDownloadID(t *testing.T) {
	item := WantedItem{ID: "wanted-1", Title: "Project Hail Mary"}
	download := acquisition.DownloadStatus{
		ID:   "AABBCCDDEEFF00112233445566778899AABBCCDD",
		Name: "Project Hail Mary EPUB",
	}
	entry := blocklistEntryForDownload(item, download, nil, "missing files", BlocklistSourceAutoFailed)
	if entry.InfoHash != "aabbccddeeff00112233445566778899aabbccdd" {
		t.Fatalf("expected lowered infohash from download id, got %q", entry.InfoHash)
	}
	if entry.WantedItemID != "wanted-1" || entry.Title != "Project Hail Mary EPUB" {
		t.Fatalf("unexpected identity: %+v", entry)
	}
	if entry.Reason != "missing files" || entry.Source != BlocklistSourceAutoFailed {
		t.Fatalf("unexpected reason/source: %+v", entry)
	}
}

func TestBlocklistEntryForDownloadUsesMatchingReleaseIdentity(t *testing.T) {
	item := WantedItem{ID: "wanted-1", Title: "Project Hail Mary", CurrentReleaseID: "release-2"}
	download := acquisition.DownloadStatus{ID: "aabbccddeeff00112233445566778899aabbccdd", Name: "phm.epub"}
	releases := []ReleaseDecision{
		{ID: "release-1", InfoHash: "0000000000000000000000000000000000000000", Indexer: "Other", Title: "Other Release"},
		{
			ID:          "release-2",
			InfoHash:    "AABBCCDDEEFF00112233445566778899AABBCCDD",
			Indexer:     "Public Indexer",
			Protocol:    "torrent",
			Title:       "Andy Weir - Project Hail Mary EPUB",
			DownloadURL: "https://indexer.example/dl/1.torrent",
		},
	}
	entry := blocklistEntryForDownload(item, download, releases, "stalled", BlocklistSourceAutoFailed)
	if entry.Indexer != "Public Indexer" || entry.Protocol != "torrent" {
		t.Fatalf("expected release identity, got %+v", entry)
	}
	if entry.Title != "Andy Weir - Project Hail Mary EPUB" {
		t.Fatalf("expected release title, got %q", entry.Title)
	}
	if entry.DownloadURLHash != hashDownloadURL("https://indexer.example/dl/1.torrent") {
		t.Fatalf("expected download URL hash, got %q", entry.DownloadURLHash)
	}
}

func TestBlocklistEntryForDownloadFallsBackToCurrentRelease(t *testing.T) {
	item := WantedItem{ID: "wanted-1", Title: "Project Hail Mary", CurrentReleaseID: "release-2"}
	// SABnzbd-style opaque ID: not an infohash and no release infohash match.
	download := acquisition.DownloadStatus{ID: "SABnzbd_nzo_abc123"}
	releases := []ReleaseDecision{
		{ID: "release-2", Indexer: "Usenet Indexer", Protocol: "usenet", Title: "Project Hail Mary EPUB", DownloadURL: "https://indexer.example/nzb/2"},
	}
	entry := blocklistEntryForDownload(item, download, releases, "failed", BlocklistSourceHistoryMarkFailed)
	if entry.InfoHash != "" {
		t.Fatalf("expected no infohash for opaque download id, got %q", entry.InfoHash)
	}
	if entry.Indexer != "Usenet Indexer" || entry.Protocol != "usenet" {
		t.Fatalf("expected current release identity, got %+v", entry)
	}
	if entry.Source != BlocklistSourceHistoryMarkFailed {
		t.Fatalf("expected history-mark-failed source, got %q", entry.Source)
	}
}

func TestLooksLikeInfoHash(t *testing.T) {
	if !looksLikeInfoHash("aabbccddeeff00112233445566778899aabbccdd") {
		t.Fatal("expected 40-char hex to look like an infohash")
	}
	if !looksLikeInfoHash(strings.Repeat("ab", 32)) {
		t.Fatal("expected 64-char hex to look like a v2 infohash")
	}
	if looksLikeInfoHash("SABnzbd_nzo_abc123") {
		t.Fatal("expected SABnzbd id to be rejected")
	}
	if looksLikeInfoHash("zzbbccddeeff00112233445566778899aabbccdd") {
		t.Fatal("expected non-hex characters to be rejected")
	}
}

func TestNormalizeBlocklistEntryDefaultsSource(t *testing.T) {
	entry := normalizeBlocklistEntry(BlocklistEntry{Title: "  Some Release  ", InfoHash: "ABCDEF"})
	if entry.Source != BlocklistSourceAutoFailed {
		t.Fatalf("expected default auto-failed source, got %q", entry.Source)
	}
	if entry.Title != "Some Release" || entry.InfoHash != "abcdef" {
		t.Fatalf("expected trimmed/lowered fields, got %+v", entry)
	}
}
