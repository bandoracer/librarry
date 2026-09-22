package library

import (
	"github.com/bandoracer/librarry/backend/internal/wanted"
	"testing"
)

func TestImportSeriesAndAuthorDecoration(t *testing.T) {
	item := wanted.WantedItem{Title: "The Sea of Monsters", AuthorName: "Rick Riordan", Series: "Percy Jackson and the Olympians", SeriesPosition: "2"}
	for _, tc := range []struct {
		title string
		match bool
	}{
		{"The Sea of Monsters", true}, {"Percy Jackson 2 - The Sea of Monsters", true},
		{"Percy Jackson and the Olympians: The Sea of Monsters", true},
		{"Percy Jackson 3 - The Sea of Monsters", false}, {"Summary: The Sea of Monsters", false},
		{"Percy Jackson 2 - The Sea of Monsters Graphic Novel", false},
		{"Other Series 2 - The Sea of Monsters", false}, {"The Sea of Monsters Study Guide", false},
	} {
		if got := importTitleMatches(tc.title, item); got != tc.match {
			t.Errorf("%q: %v", tc.title, got)
		}
	}
	if !importAuthorMatches("Riordan, Rick", "Rick Riordan") {
		t.Fatal("surname-first should match")
	}
	if importAuthorMatches("Riordan, Richard", "Rick Riordan") {
		t.Fatal("different given name must not match")
	}
	if importAuthorMatches("Rick Riordan; Other Writer", "Rick Riordan") {
		t.Fatal("coauthors must not disappear")
	}
	item.Series = ""
	if importTitleMatches("Percy Jackson 2 - The Sea of Monsters", item) {
		t.Fatal("unknown series cannot justify prefix")
	}
}
