package wanted

import (
	"testing"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

func filterCandidate(language string, isbns []string, pages int, title string) metadata.SearchResult {
	return metadata.SearchResult{
		Provider: "Hardcover",
		Work:     metadata.Work{ID: "hardcover:1", Title: title},
		Edition: metadata.Edition{
			Title:    title,
			Format:   metadata.FormatEbook,
			Language: language,
			ISBNs:    isbns,
			Pages:    pages,
		},
	}
}

func TestAuthorResultFilterReasons(t *testing.T) {
	base := AuthorSubscription{
		AllowedLanguages: []string{"English"},
		MustNotContain:   []string{"summary", "workbook"},
		SkipMissingISBN:  true,
		MinPages:         100,
	}

	cases := []struct {
		name      string
		candidate metadata.SearchResult
		want      string
	}{
		{
			name:      "passes all filters",
			candidate: filterCandidate("English", []string{"9780593135204"}, 300, "Project Hail Mary"),
			want:      "",
		},
		{
			name:      "language filtered",
			candidate: filterCandidate("German", []string{"9780593135204"}, 300, "Der Astronaut"),
			want:      "language-filtered",
		},
		{
			name:      "unknown language passes",
			candidate: filterCandidate("", []string{"9780593135204"}, 300, "Project Hail Mary"),
			want:      "",
		},
		{
			name:      "term filtered",
			candidate: filterCandidate("English", []string{"9780593135204"}, 300, "Project Hail Mary Summary & Analysis"),
			want:      "term-filtered",
		},
		{
			name:      "missing isbn",
			candidate: filterCandidate("English", nil, 300, "Project Hail Mary"),
			want:      "missing-isbn",
		},
		{
			name:      "below min pages",
			candidate: filterCandidate("English", []string{"9780593135204"}, 40, "Project Hail Mary"),
			want:      "below-min-pages",
		},
		{
			name:      "unknown page count passes",
			candidate: filterCandidate("English", []string{"9780593135204"}, 0, "Project Hail Mary"),
			want:      "",
		},
	}
	for _, testCase := range cases {
		if got := authorResultFilterReason(base, testCase.candidate); got != testCase.want {
			t.Fatalf("%s: expected %q, got %q", testCase.name, testCase.want, got)
		}
	}
}

func TestAuthorResultFilterLanguageVariants(t *testing.T) {
	subscription := AuthorSubscription{AllowedLanguages: []string{"en", "French"}}
	if got := authorResultFilterReason(subscription, filterCandidate("English", nil, 0, "Book")); got != "" {
		t.Fatalf("expected en prefix to match English, got %q", got)
	}
	if got := authorResultFilterReason(subscription, filterCandidate("french", nil, 0, "Livre")); got != "" {
		t.Fatalf("expected case-insensitive match, got %q", got)
	}
	if got := authorResultFilterReason(subscription, filterCandidate("Spanish", nil, 0, "Libro")); got != "language-filtered" {
		t.Fatalf("expected spanish to be filtered, got %q", got)
	}
}

func TestAuthorResultFilterDisabledByDefault(t *testing.T) {
	// A subscription without filters never rejects.
	subscription := AuthorSubscription{}
	if got := authorResultFilterReason(subscription, filterCandidate("Klingon", nil, 1, "Anything Summary")); got != "" {
		t.Fatalf("expected no filtering by default, got %q", got)
	}
}

func TestFilterTermHelpers(t *testing.T) {
	if got := joinFilterTerms([]string{" English ", "", "english", "French"}); got != "English,French" {
		t.Fatalf("unexpected join: %q", got)
	}
	terms := splitFilterTerms("summary; workbook | study guide")
	if len(terms) != 3 || terms[0] != "summary" || terms[1] != "workbook" || terms[2] != "study guide" {
		t.Fatalf("unexpected split: %#v", terms)
	}
}

func TestTagLabelHelpers(t *testing.T) {
	if got := tagLabelsString([]string{" Favorites ", "sci-fi", "FAVORITES"}); got != "favorites,sci-fi" {
		t.Fatalf("unexpected label string: %q", got)
	}
	labels := splitTagLabels("favorites, sci-fi,,signed")
	if len(labels) != 3 || labels[0] != "favorites" || labels[2] != "signed" {
		t.Fatalf("unexpected labels: %#v", labels)
	}
	// Restrictions with integer tags match items whose label hash (or legacy
	// numeric label) equals the restriction tag.
	restriction := ReleaseRestriction{Tags: []int{TagLabelHash("favorites")}}
	if !releaseRestrictionAppliesToLabels(restriction, []string{"favorites"}) {
		t.Fatal("expected hash match to apply restriction")
	}
	if releaseRestrictionAppliesToLabels(restriction, []string{"other"}) {
		t.Fatal("expected non-matching labels to skip restriction")
	}
	legacy := ReleaseRestriction{Tags: []int{42}}
	if !releaseRestrictionAppliesToLabels(legacy, []string{"42"}) {
		t.Fatal("expected legacy numeric label to match")
	}
	open := ReleaseRestriction{}
	if !releaseRestrictionAppliesToLabels(open, nil) {
		t.Fatal("expected untagged restriction to apply to everything")
	}
}

func TestWantedReleaseDateRequiresFullPrecision(t *testing.T) {
	full := metadata.SearchResult{Edition: metadata.Edition{PublishedDate: "2026-07-04"}}
	if got := wantedReleaseDate(full); got != "2026-07-04" {
		t.Fatalf("expected full date to persist, got %v", got)
	}
	for _, value := range []string{"2026", "2026-07", "", "not-a-date"} {
		partial := metadata.SearchResult{Edition: metadata.Edition{PublishedDate: value}}
		if got := wantedReleaseDate(partial); got != nil {
			t.Fatalf("expected %q to stay off the calendar, got %v", value, got)
		}
	}
}
