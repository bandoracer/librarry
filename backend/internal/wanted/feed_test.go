package wanted

import (
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func TestFeedReleaseMatchesWantedStrongTitle(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir"}
	release := acquisition.Release{Title: "Project Hail Mary by Andy Weir EPUB"}
	if !feedReleaseMatchesWanted(item, release) {
		t.Fatal("expected feed release to match wanted item")
	}
}

func TestFeedReleaseMatchesWantedRejectsUnrelatedTitle(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir"}
	release := acquisition.Release{Title: "The Martian by Andy Weir EPUB"}
	if feedReleaseMatchesWanted(item, release) {
		t.Fatal("expected unrelated feed release to be ignored")
	}
}

func TestFormatMatchesRequest(t *testing.T) {
	if !formatMatchesRequest("", "ebook") {
		t.Fatal("expected empty request format to match ebook")
	}
	if !formatMatchesRequest("any", "audiobook") {
		t.Fatal("expected any request format to match audiobook")
	}
	if formatMatchesRequest("audiobook", "ebook") {
		t.Fatal("expected audiobook request to reject ebook wanted item")
	}
}
