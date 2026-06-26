package metadata

import (
	"context"
	"testing"
	"time"
)

func TestSearchDetailedMergesDuplicateISBNResults(t *testing.T) {
	service := NewService([]Provider{
		staticMetadataProvider{name: "Open Library", results: []SearchResult{{
			Provider: "Open Library",
			Kind:     SearchTypeBook,
			Work: Work{
				ID:               "openlibrary:OL37444382W",
				Title:            "Project Hail Mary",
				FirstPublishYear: 2021,
				Authors:          []Author{{ID: "openlibrary:OL6811384A", Name: "Andy Weir"}},
				ProviderIDs:      []string{"openlibrary:OL37444382W"},
			},
			Edition: Edition{
				ID:       "openlibrary:OL32695809M",
				WorkID:   "openlibrary:OL37444382W",
				Title:    "Project Hail Mary",
				Format:   FormatEbook,
				Language: "eng",
				ISBNs:    []string{"9780593135204"},
			},
			Score:      0.93,
			Confidence: "high",
			MatchedOn:  []string{"isbn", "title", "author"},
		}}},
		staticMetadataProvider{name: "Google Books", results: []SearchResult{{
			Provider: "Google Books",
			Kind:     SearchTypeBook,
			Work: Work{
				ID:          "googlebooks:abc123",
				Title:       "Project Hail Mary",
				Authors:     []Author{{ID: "googlebooks-author:andy", Name: "Andy Weir"}},
				CoverURL:    "https://example.test/project-hail-mary.jpg",
				ProviderIDs: []string{"googlebooks:abc123"},
			},
			Edition: Edition{
				ID:            "googlebooks:abc123:edition",
				WorkID:        "googlebooks:abc123",
				Title:         "Project Hail Mary",
				Format:        FormatEbook,
				ISBNs:         []string{"978-0-593-13520-4", "9780593135211"},
				Publisher:     "Ballantine Books",
				PublishedDate: "2021-05-04",
			},
			Score:      0.91,
			Confidence: "high",
			MatchedOn:  []string{"isbn"},
		}}},
	})

	outcome := service.SearchDetailed(context.Background(), Query{Query: "9780593135204", Type: SearchTypeBook, Format: FormatEbook, Limit: 10})
	if len(outcome.Results) != 1 {
		t.Fatalf("expected duplicate ISBN records to merge, got %d results: %+v", len(outcome.Results), outcome.Results)
	}
	result := outcome.Results[0]
	if result.Provider != "Open Library" {
		t.Fatalf("expected Open Library to remain primary on close scores, got %q", result.Provider)
	}
	if result.Score <= 0.93 || result.Confidence != "high" {
		t.Fatalf("expected provider corroboration to preserve high confidence, got score=%0.2f confidence=%s", result.Score, result.Confidence)
	}
	if !hasString(result.Work.ProviderIDs, "openlibrary:OL37444382W") || !hasString(result.Work.ProviderIDs, "googlebooks:abc123") {
		t.Fatalf("expected merged provider IDs, got %+v", result.Work.ProviderIDs)
	}
	if !hasString(result.Edition.ISBNs, "9780593135211") || result.Edition.Publisher != "Ballantine Books" {
		t.Fatalf("expected edition enrichment from fallback provider, got %+v", result.Edition)
	}
	if !hasString(result.MatchedOn, "provider corroboration") {
		t.Fatalf("expected merged match evidence, got %+v", result.MatchedOn)
	}
}

func TestSearchDetailedMergesHardcoverWorkWithOpenLibraryEdition(t *testing.T) {
	service := NewService([]Provider{
		staticMetadataProvider{name: "Hardcover", results: []SearchResult{{
			Provider: "Hardcover",
			Kind:     SearchTypeBook,
			Work: Work{
				ID:          "hardcover:1",
				Title:       "Dungeon Crawler Carl",
				Authors:     []Author{{ID: "hardcover-author:matt-dinniman", Name: "Matt Dinniman"}},
				CoverURL:    "https://assets.example.test/dcc.jpg",
				ProviderIDs: []string{"hardcover:1"},
			},
			Score:      0.84,
			Confidence: "medium",
			MatchedOn:  []string{"hardcover search"},
		}}},
		staticMetadataProvider{name: "Open Library", results: []SearchResult{{
			Provider: "Open Library",
			Kind:     SearchTypeBook,
			Work: Work{
				ID:               "openlibrary:OL999W",
				Title:            "Dungeon Crawler Carl",
				FirstPublishYear: 2020,
				Authors:          []Author{{ID: "openlibrary:OL999A", Name: "Matt Dinniman"}},
				ProviderIDs:      []string{"openlibrary:OL999W"},
			},
			Edition: Edition{
				ID:       "openlibrary:OL999M",
				WorkID:   "openlibrary:OL999W",
				Title:    "Dungeon Crawler Carl",
				Format:   FormatAudiobook,
				Language: "eng",
				ISBNs:    []string{"9781705040873"},
			},
			Score:      0.83,
			Confidence: "medium",
			MatchedOn:  []string{"title", "author"},
		}}},
	})

	outcome := service.SearchDetailed(context.Background(), Query{Query: "Dungeon Crawler Carl Matt Dinniman", Type: SearchTypeBook, Format: FormatAudiobook, Limit: 10})
	if len(outcome.Results) != 1 {
		t.Fatalf("expected corroborated work result, got %d results", len(outcome.Results))
	}
	result := outcome.Results[0]
	if result.Provider != "Hardcover" {
		t.Fatalf("expected Hardcover to remain primary for close work matches, got %q", result.Provider)
	}
	if result.Work.FirstPublishYear != 2020 || result.Work.CoverURL == "" {
		t.Fatalf("expected rich work fields to merge, got %+v", result.Work)
	}
	if result.Edition.ID != "openlibrary:OL999M" || !hasString(result.Edition.ISBNs, "9781705040873") {
		t.Fatalf("expected edition evidence to merge into primary result, got %+v", result.Edition)
	}
}

func TestSearchDetailedDoesNotMergeSameTitleDifferentAuthors(t *testing.T) {
	service := NewService([]Provider{
		staticMetadataProvider{name: "Open Library", results: []SearchResult{{
			Provider: "Open Library",
			Kind:     SearchTypeBook,
			Work:     Work{ID: "openlibrary:one", Title: "The Long Way", Authors: []Author{{Name: "Alice Writer"}}},
			Score:    0.82,
		}}},
		staticMetadataProvider{name: "Google Books", results: []SearchResult{{
			Provider: "Google Books",
			Kind:     SearchTypeBook,
			Work:     Work{ID: "googlebooks:two", Title: "The Long Way", Authors: []Author{{Name: "Bob Writer"}}},
			Score:    0.81,
		}}},
	})

	outcome := service.SearchDetailed(context.Background(), Query{Query: "The Long Way", Type: SearchTypeBook, Limit: 10})
	if len(outcome.Results) != 2 {
		t.Fatalf("expected distinct authors to stay separate, got %d results", len(outcome.Results))
	}
}

func TestSearchDetailedDoesNotMergeConflictingConcreteFormatsForAnyQuery(t *testing.T) {
	service := NewService([]Provider{
		staticMetadataProvider{name: "Open Library", results: []SearchResult{{
			Provider: "Open Library",
			Kind:     SearchTypeBook,
			Work:     Work{ID: "openlibrary:ebook", Title: "Same Book", Authors: []Author{{Name: "Same Author"}}},
			Edition:  Edition{ID: "openlibrary:ebook:edition", Title: "Same Book", Format: FormatEbook},
			Score:    0.82,
		}}},
		staticMetadataProvider{name: "Google Books", results: []SearchResult{{
			Provider: "Google Books",
			Kind:     SearchTypeBook,
			Work:     Work{ID: "googlebooks:audio", Title: "Same Book", Authors: []Author{{Name: "Same Author"}}},
			Edition:  Edition{ID: "googlebooks:audio:edition", Title: "Same Book", Format: FormatAudiobook},
			Score:    0.81,
		}}},
	})

	outcome := service.SearchDetailed(context.Background(), Query{Query: "Same Book Same Author", Type: SearchTypeBook, Format: FormatAny, Limit: 10})
	if len(outcome.Results) != 2 {
		t.Fatalf("expected ebook and audiobook editions to stay separate for any-format search, got %d results", len(outcome.Results))
	}
}

type staticMetadataProvider struct {
	name    string
	results []SearchResult
	err     error
}

func (p staticMetadataProvider) Name() string { return p.name }

func (p staticMetadataProvider) Health(ctx Context) ProviderHealth {
	return ProviderHealth{Name: p.name, Status: "ready", Configured: true, CheckedAt: time.Now().UTC()}
}

func (p staticMetadataProvider) Diagnostics(ctx Context) Diagnostic {
	return Diagnostic{Name: p.name, Configured: true}
}

func (p staticMetadataProvider) Search(ctx Context, query Query) ([]SearchResult, error) {
	if p.err != nil {
		return nil, p.err
	}
	return append([]SearchResult(nil), p.results...), nil
}

func hasString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
