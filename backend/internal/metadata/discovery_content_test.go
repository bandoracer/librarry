package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDiscoveryContentEvidence(t *testing.T) {
	original := SearchResult{Provider: "Hardcover", Kind: SearchTypeBook, Work: Work{ID: "hardcover:1", Title: "The Hobbit", Authors: []Author{{Name: "J.R.R. Tolkien"}}}, Edition: Edition{ID: "hardcover-edition:1", Title: "The Hobbit", Format: FormatEbook, Language: "English"}}
	for _, tc := range []struct {
		name, query    string
		result         SearchResult
		section, label string
	}{
		{"explicit category", "The Hobbit", SearchResult{Provider: "Hardcover", Work: Work{ID: "hardcover:2", Title: "The Hobbit", ContentType: "graphic_novel", Authors: original.Work.Authors}}, "related", "Graphic novel"},
		{"explicit comic intent", "The Hobbit comics", SearchResult{Provider: "Hardcover", Work: Work{ID: "hardcover:2", Title: "The Hobbit: A Graphic Novel", ContentType: "graphic_novel", Authors: original.Work.Authors}}, "", "Graphic novel"},
		{"mixed original subjects", "The Hobbit", SearchResult{Provider: "Open Library", Work: Work{ID: "openlibrary:OL27482W", Title: "The Hobbit", Subjects: []string{"Graphic novels", "Comic books, strips"}, Authors: original.Work.Authors}}, "", ""},
		{"corroborated comic subjects", "The Hobbit", SearchResult{Provider: "Open Library", Work: Work{ID: "openlibrary:OL219602W", Title: "The Hobbit", Subjects: []string{"Comic books, strips"}, Authors: []Author{{Name: "Charles Dixon"}, {Name: "J.R.R. Tolkien"}}}}, "related", "Possible graphic adaptation"},
		{"extra writer alone insufficient", "The Hobbit", SearchResult{Provider: "Open Library", Work: Work{ID: "openlibrary:OL2W", Title: "The Hobbit", Authors: []Author{{Name: "Other Writer"}, {Name: "J.R.R. Tolkien"}}}}, "", ""},
		{"edition abridgment", "The Hobbit", SearchResult{Provider: "Hardcover", Work: original.Work, Edition: Edition{ID: "hardcover-edition:3", Title: "The Hobbit", EditionInformation: "Abridged edition"}}, "related", "Abridged"},
		{"unabridged", "The Hobbit", SearchResult{Provider: "Hardcover", Work: original.Work, Edition: Edition{ID: "hardcover-edition:3", Title: "The Hobbit", EditionInformation: "Unabridged edition"}}, "", ""},
		{"explicit abridgment", "The Hobbit abridged", SearchResult{Provider: "Hardcover", Work: original.Work, Edition: Edition{ID: "hardcover-edition:3", Title: "The Hobbit", EditionInformation: "Abridged edition"}}, "", "Abridged"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []SearchResult{original, tc.result}
			assignDiscoverySections(Query{Query: tc.query, Type: SearchTypeBook, PreferredLanguage: "English"}, rows)
			if rows[1].DiscoverySection != tc.section || rows[1].ContentLabel != tc.label {
				t.Fatalf("got %q / %q", rows[1].DiscoverySection, rows[1].ContentLabel)
			}
			if rows[1].Work.ID != tc.result.Work.ID || rows[1].Edition.ID != tc.result.Edition.ID {
				t.Fatal("changed identity")
			}
		})
	}
	hcComic := original
	hcComic.Work.Subjects = []string{"Graphic Novels"}
	hcComic.Work.Authors = append([]Author{{Name: "Adapter"}}, original.Work.Authors...)
	hcComic.Work.ID = "hardcover:2"
	pair := []SearchResult{original, hcComic}
	assignDiscoverySections(Query{Query: "The Hobbit", Type: SearchTypeBook}, pair)
	if pair[1].ContentLabel != "Possible graphic adaptation" || pair[1].DiscoverySection != "related" {
		t.Fatal("ignored corroborated Hardcover genre")
	}
	hcComic.Work.ID = original.Work.ID
	pair = []SearchResult{original, hcComic}
	assignDiscoverySections(Query{Query: "The Hobbit", Type: SearchTypeBook}, pair)
	if pair[1].ContentLabel != "" || pair[1].DiscoverySection != "" {
		t.Fatal("same work with extra edition contributors mislabeled")
	}
	hcComic.Work.ID = "openlibrary:OL9W"
	hcComic.Work.ProviderIDs = []string{original.Work.ID}
	pair = []SearchResult{original, hcComic}
	assignDiscoverySections(Query{Query: "The Hobbit", Type: SearchTypeBook}, pair)
	if pair[1].ContentLabel != "" {
		t.Fatal("verified same-work alias mislabeled")
	}
	graphic := original
	graphic.Work.ContentType = "graphic_novel"
	rows := []SearchResult{graphic}
	assignDiscoverySections(Query{Query: "The Hobbit", Type: SearchTypeBook}, rows)
	if rows[0].DiscoverySection != "" {
		t.Fatal("standalone graphic work hidden without original counterpart")
	}
	graphic.Edition.ISBNs = []string{"9780593135204"}
	rows = []SearchResult{original, graphic}
	assignDiscoverySections(Query{Query: "9780593135204", Type: SearchTypeBook}, rows)
	if rows[1].DiscoverySection != "" || rows[1].ContentLabel != "Graphic novel" {
		t.Fatal("exact edition lookup hidden or unlabeled")
	}
}

func TestContentEvidenceSurvivesProviderNormalizationAndCache(t *testing.T) {
	book := strings.Replace(richBookFixture, `"release_year":1990`, `"genres":[{"tag":"Comics"}],"book_category_id":4,"release_year":1990`, 1)
	book = strings.Replace(book, `"subtitle":"Ebook Edition"`, `"subtitle":"Ebook Edition","edition_information":"Abridged"`, 1)
	p, _ := editionFixtureProvider(t, book)
	rows, err := p.Search(context.Background(), Query{Query: "Fixture Book", Format: FormatAny})
	if err != nil || len(rows) != 2 || rows[0].Work.ContentType != "graphic_novel" || len(rows[0].Work.Subjects) != 1 || rows[0].Work.Subjects[0] != "Comics" || rows[0].Edition.EditionInformation != "Abridged" || rows[1].Edition.EditionInformation != "" {
		t.Fatal(rows, err)
	}
	ol := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Query().Get("fields"), ",subject,") {
			t.Fatal("subject evidence not requested")
		}
		return jsonResponse(`{"docs":[{"key":"/works/OL219602W","title":"The Hobbit","author_name":["Charles Dixon","J.R.R. Tolkien"],"subject":["Comic books, strips"]}]}`), nil
	})})
	svc := NewService([]Provider{ol})
	q := Query{Query: "The Hobbit", Type: SearchTypeBook}
	first := svc.SearchDetailed(context.Background(), q)
	if len(first.Results) != 1 || len(first.Results[0].Work.Subjects) != 1 {
		t.Fatal(first)
	}
	first.Results[0].Work.Subjects[0] = "changed"
	second := svc.SearchDetailed(context.Background(), q)
	if second.Results[0].Work.Subjects[0] != "Comic books, strips" {
		t.Fatal("mutable cache evidence")
	}
	// API JSON preserves the evidence used by classification.
	raw, _ := json.Marshal(second.Results[0])
	if !strings.Contains(string(raw), "Comic books, strips") {
		t.Fatal(string(raw))
	}
}

func TestExplicitGraphicSearchPrefersRequestedTitle(t *testing.T) {
	rows := []SearchResult{
		{Provider: "Hardcover", Kind: SearchTypeBook, Work: Work{ID: "hardcover:1", Title: "Ultimate Comics: Divided We Fall, United We Stand", Authors: []Author{{Name: "Marvel Writer"}}}, Edition: Edition{ID: "hardcover-edition:1", Format: FormatEbook, Language: "English"}},
		{Provider: "Open Library", Kind: SearchTypeBook, Work: Work{ID: "openlibrary:OL16619489W", Title: "The stand", Subjects: []string{"Graphic novels"}, Authors: []Author{{Name: "Roberto Aguirre-Sacasa"}}}},
	}
	svc := NewService([]Provider{staticMetadataProvider{name: "Hardcover", results: rows[:1]}, staticMetadataProvider{name: "Open Library", results: rows[1:]}})
	result := svc.SearchDetailed(context.Background(), Query{Query: "The Stand comics", Type: SearchTypeBook, PreferredLanguage: "English", Limit: 10})
	if len(result.Results) != 2 || result.Results[0].Work.ID != "openlibrary:OL16619489W" || result.Results[0].DiscoverySection != "" {
		t.Fatal(result)
	}
}
