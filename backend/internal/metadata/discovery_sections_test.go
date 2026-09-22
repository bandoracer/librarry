package metadata

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDiscoveryLiveReviewRegression(t *testing.T) {
	data, err := os.ReadFile("../../../research/metadata-ranking/live-review/baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID      string
		Query   Query
		Results []SearchResult
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			providers := []Provider{}
			for _, name := range []string{"Hardcover", "Open Library", "Google Books"} {
				rows := []SearchResult{}
				for _, r := range c.Results {
					if r.Provider == name {
						rows = append(rows, r)
					}
				}
				if len(rows) > 0 {
					providers = append(providers, staticMetadataProvider{name: name, results: rows})
				}
			}
			outcome := NewService(providers).SearchDetailed(context.Background(), c.Query)
			if len(outcome.Results) == 0 {
				t.Fatal("lost all results")
			}
			want := c.Results[0].Work.ID
			if c.ID == "data" {
				want = "hardcover:441376"
			}
			if outcome.Results[0].Work.ID != want {
				t.Fatalf("first %s, want %s", outcome.Results[0].Work.ID, want)
			}
			for _, r := range outcome.Results {
				if c.ID == "typo" && r.Work.Title == "King Lear" && r.DiscoverySection != "related" {
					t.Fatal("unrelated provider hit remains primary")
				}
				if r.Work.ID == "hardcover:2894000" && r.DiscoverySection != "incomplete" {
					t.Fatal("empty record remains primary")
				}
				if strings.Contains(strings.ToLower(r.Work.Title), "summary") && r.DiscoverySection != "related" {
					t.Fatal("summary remains primary")
				}
			}
			if c.ID == "series" {
				n := 0
				for _, r := range outcome.Results {
					if r.DiscoverySection == "" {
						n++
						if concreteFormat(r.Edition.Format) == FormatAny {
							t.Fatal("series stub remains primary")
						}
					}
				}
				if n == 0 {
					t.Fatal("lost series editions")
				}
			}
		})
	}
}

func TestDiscoverySecondarySectionsNeverMergeOrDiscardIdentity(t *testing.T) {
	author := []Author{{ID: "hardcover-author:1", Name: "Fixture Writer"}}
	rows := []SearchResult{
		{Provider: "Hardcover", Kind: SearchTypeBook, Work: Work{ID: "hardcover:1", Title: "Novel", Authors: author}, Edition: Edition{ID: "hardcover-edition:1", Format: FormatEbook, Language: "English"}},
		{Provider: "Open Library", Kind: SearchTypeBook, Work: Work{ID: "openlibrary:OL1W", Title: "Novel", Authors: author}},
		{Provider: "Hardcover", Kind: SearchTypeBook, Work: Work{ID: "hardcover:2", Title: "Novel Workbook", Authors: author}},
	}
	for _, query := range []string{"Novel", "Novel Workbook"} {
		results := append([]SearchResult{}, rows...)
		assignDiscoverySections(Query{Query: query, Type: SearchTypeBook}, results)
		if results[0].Work.ID == results[1].Work.ID {
			t.Fatal("invented shared identity")
		}
		expected := "related"
		if query == "Novel Workbook" {
			expected = ""
		}
		if results[2].DiscoverySection != expected {
			t.Fatal("explicit workbook request was demoted")
		}
	}
	isbn := SearchResult{Kind: SearchTypeBook, Work: Work{Title: "Summary of Novel"}, Edition: Edition{ISBNs: []string{"9780593135204"}}}
	exact := []SearchResult{isbn}
	assignDiscoverySections(Query{Query: "9780593135204", Type: SearchTypeBook}, exact)
	if exact[0].DiscoverySection != "" {
		t.Fatal("explicit ISBN hidden")
	}
	// Topic queries must not reject a provider's semantic matches without a
	// credible title anchor in the returned results.
	topic := []SearchResult{{Work: Work{Title: "Dune", Authors: author}}}
	assignDiscoverySections(Query{Query: "desert science fiction", Type: SearchTypeBook}, topic)
	if topic[0].DiscoverySection != "" {
		t.Fatal("topic discovery became title-only search")
	}
}

func TestDiscoveryTitleFocusDoesNotPromoteIncidentalKeywords(t *testing.T) {
	for _, tc := range []struct {
		query, title string
		want         bool
	}{
		{"Educated", "The Amazing Maurice and His Educated Rodents", false},
		{"Educated", "Educated", true},
		{"Dune", "Dune Messiah", false},
		{"Atomic Habits", "Atomic Habits: An Easy & Proven Way", true},
		{"The Hobbit", "The Hobbit, or There and Back Again", true},
	} {
		if got := discoveryTitleFocus(Query{Query: tc.query}, SearchResult{Work: Work{Title: tc.title}}); got != tc.want {
			t.Fatalf("%s / %s: %v", tc.query, tc.title, got)
		}
	}
}
