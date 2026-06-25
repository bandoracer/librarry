package wanted

import (
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func TestEvaluateReleaseApprovesStrongEbookMatch(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateRelease(item, release)
	if !decision.Approved {
		t.Fatalf("expected approved release, got rejection %q score %0.2f", decision.RejectedReason, decision.Score)
	}
	if decision.Score < 60 {
		t.Fatalf("expected useful score, got %0.2f", decision.Score)
	}
}

func TestEvaluateReleaseRejectsSummary(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Summary and Review Project Hail Mary EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateRelease(item, release)
	if decision.Approved {
		t.Fatalf("expected summary release to be rejected")
	}
	if !strings.Contains(decision.RejectedReason, "summary") {
		t.Fatalf("expected summary rejection, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseRejectsWrongFormat(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "audiobook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateRelease(item, release)
	if decision.Approved {
		t.Fatalf("expected wrong format to be rejected")
	}
	if !strings.Contains(decision.RejectedReason, "audiobook") {
		t.Fatalf("expected audiobook rejection, got %q", decision.RejectedReason)
	}
}
