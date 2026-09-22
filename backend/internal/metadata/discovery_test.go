package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Replay real provider payloads through the shipped adapter, cache, merge,
// language filter and sorter. Labels are assertions, never ranking inputs.
func TestDiscoveryCapturedBenchmark(t *testing.T) {
	root := filepath.Join("..", "..", "..", "research", "metadata-ranking")
	var cases []struct{ ID, Query, Type string }
	var labels map[string]struct{ Targets []string }
	read := func(path string, target any) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	read("cases.json", &cases)
	read("labels.json", &labels)
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			suffix := "-q"
			if c.ID == "isbn" || c.ID == "riordan" {
				suffix = "-direct"
			}
			var snapshot struct {
				Data struct {
					Docs []json.RawMessage `json:"docs"`
				} `json:"data"`
			}
			read(filepath.Join("snapshots", c.ID+suffix+".json"), &snapshot)
			calls := 0
			p := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if c.Type == "book" {
					if req.URL.Query().Get("title") != "" {
						t.Fatal("title-only retrieval returned")
					}
					if req.URL.Query().Get("lang") != "en" {
						t.Fatal("missing English edition preference")
					}
					if req.URL.Query().Get("fields") != openLibrarySearchFields {
						t.Fatal("unexpected fields")
					}
					key := "q"
					want := literalOpenLibraryQuery(c.Query)
					if c.ID == "isbn" {
						key = "isbn"
						want = c.Query
					}
					if req.URL.Query().Get(key) != want {
						t.Fatalf("wrong query %s", req.URL.RawQuery)
					}
				}
				limit, err := strconv.Atoi(req.URL.Query().Get("limit"))
				if err != nil || limit > 25 {
					t.Fatal("unbounded retrieval")
				}
				docs := snapshot.Data.Docs
				if len(docs) > limit {
					docs = docs[:limit]
				}
				body, _ := json.Marshal(map[string]any{"docs": docs})
				return jsonResponse(string(body)), nil
			})})
			service := NewService([]Provider{p})
			query := Query{Query: c.Query, Type: SearchType(c.Type), PreferredLanguage: "English", Format: FormatAny, Limit: 10}
			for attempt := 0; attempt < 2; attempt++ {
				outcome := service.SearchDetailed(context.Background(), query)
				if len(outcome.ProviderErrors) > 0 || len(outcome.Results) == 0 {
					t.Fatalf("empty/error result: %+v", outcome)
				}
				first := strings.TrimPrefix(outcome.Results[0].Work.ID, "openlibrary:")
				if !hasString(labels[c.ID].Targets, first) {
					t.Fatalf("top result %s (%s) is not a judged target", first, outcome.Results[0].Work.Title)
				}
				if len(outcome.Results) > 10 {
					t.Fatal("display bound exceeded")
				}
				if c.ID == "isbn" && !hasString(outcome.Results[0].Edition.ISBNs, c.Query) {
					t.Fatal("ISBN must belong to selected edition")
				}
				t.Logf("attempt %d: %s [%s], edition %s", attempt, outcome.Results[0].Work.Title, first, outcome.Results[0].Edition.ID)
			}
			if calls != 1 {
				t.Fatalf("expected one request plus cached replay, got %d", calls)
			}
		})
	}
}

func searchOLFixture(t *testing.T, body string, query Query) SearchOutcome {
	t.Helper()
	p := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(body), nil })})
	return NewService([]Provider{p}).SearchDetailed(context.Background(), query)
}

func TestDiscoveryCoherentEditionAndProviderOrder(t *testing.T) {
	outcome := searchOLFixture(t, `{"docs":[
 {"key":"/works/OL1W","title":"Real Novel","author_name":["Writer One","Writer Two"],"author_key":["OL1A","OL2A"],"language":["fin","eng"],"isbn":["9780142437247"],"edition_key":["OL999M"],"cover_i":9,"editions":{"docs":[{"key":"/books/OL2M","title":"English Novel","language":["eng"],"isbn":["9781423103349"],"publisher":["Selected Press"],"publish_date":["2007"],"cover_i":2}]}},
 {"key":"/works/OL3W","title":"Novel","language":["eng"],"edition_key":["OL888M"],"isbn":["9780142437247"]}
 ]}`, Query{Query: "Novel", Type: SearchTypeBook, PreferredLanguage: "English", Format: FormatAudiobook, Limit: 10})
	if len(outcome.Results) != 2 {
		t.Fatalf("%+v", outcome)
	}
	r := outcome.Results[0]
	if r.Work.ID != "openlibrary:OL1W" {
		t.Fatal("lexical score overrode provider relevance")
	}
	if r.Edition.ID != "openlibrary:OL2M" || r.Edition.Language != "eng" || len(r.Edition.ISBNs) != 1 || r.Edition.ISBNs[0] != "9781423103349" || r.Edition.Publisher != "Selected Press" || r.Edition.PublishedDate != "2007" || r.Edition.CoverURL != openLibraryCoverURL(2) {
		t.Fatalf("mixed edition fields: %+v", r.Edition)
	}
	if r.Edition.Format != FormatAny || len(r.Work.Authors) != 2 || len(r.Work.Languages) != 2 {
		t.Fatalf("fabricated format or lost work metadata: %+v", r)
	}
	unknown := outcome.Results[1].Edition
	if unknown.ID != "" || unknown.Language != "" || len(unknown.ISBNs) != 0 {
		t.Fatalf("work fields leaked: %+v", unknown)
	}
}

func TestDiscoveryExactISBNRequiresEditionAndExposesConflict(t *testing.T) {
	outcome := searchOLFixture(t, `{"docs":[
 {"key":"/works/OL1W","title":"Wrong edition","isbn":["9781423103349"],"editions":{"docs":[{"key":"/books/OL1M","isbn":["9780142437247"]}]}},
 {"key":"/works/OL2W","title":"Exact edition","editions":{"docs":[{"key":"/books/OL2M","language":["fra"],"isbn":["1423103343"]}]}}
 ]}`, Query{Query: "9781423103349", Type: SearchTypeBook, PreferredLanguage: "English", Limit: 10})
	if len(outcome.Results) != 1 || outcome.Results[0].Work.ID != "openlibrary:OL2W" {
		t.Fatalf("%+v", outcome)
	}
	r := outcome.Results[0]
	if len(r.Conflicts) != 1 || r.Confidence != "review" || !hasString(r.Evidence, "Exact ISBN") {
		t.Fatalf("missing conflict evidence: %+v", r)
	}
}

func TestDiscoveryExplicitAuthorAndDistinctIdentities(t *testing.T) {
	rows := []SearchResult{
		{Provider: "Open Library", Kind: SearchTypeBook, Work: Work{ID: "openlibrary:OL1W", Title: "The Book", Authors: []Author{{Name: "Wrong Writer"}}}},
		{Provider: "Open Library", Kind: SearchTypeBook, Work: Work{ID: "openlibrary:OL2W", Title: "The Book", Authors: []Author{{Name: "Right Writer"}}}},
		{Provider: "Open Library", Kind: SearchTypeBook, Work: Work{ID: "openlibrary:OL3W", Title: "The Book", Authors: []Author{{Name: "Right Writer"}}}},
	}
	outcome := NewService([]Provider{staticMetadataProvider{name: "Open Library", results: rows}}).SearchDetailed(context.Background(), Query{Query: "The Book by Right Writer", Type: SearchTypeBook})
	if len(outcome.Results) != 3 || outcome.Results[0].Work.ID != "openlibrary:OL2W" || len(outcome.Results[2].Conflicts) != 1 {
		t.Fatalf("%+v", outcome)
	}
	if normalize("Élan 世界") != "élan 世界" {
		t.Fatal("Unicode title lost")
	}
}

func TestDiscoveryRejectsWrongEntityKind(t *testing.T) {
	outcome := searchOLFixture(t, `{"docs":[{"key":"/works/OL1M","title":"Edition disguised as work"}]}`, Query{Query: "test", Type: SearchTypeBook})
	if len(outcome.Results) != 0 {
		t.Fatal("edition accepted as work")
	}
}

// Opt-in read-only smoke test: no library writes, acquisition, or credentials.
func TestLiveOpenLibraryDiscovery(t *testing.T) {
	if os.Getenv("LIBRARRY_LIVE_DISCOVERY_TEST") != "1" {
		t.Skip("set LIBRARRY_LIVE_DISCOVERY_TEST=1 for read-only upstream checks")
	}
	service := NewService([]Provider{NewOpenLibraryProvider(&http.Client{Timeout: 15 * time.Second})})
	for _, tc := range []struct{ query, work string }{{"percy jackson", "openlibrary:OL492658W"}, {"project hail mary", "openlibrary:OL21745884W"}, {"9781423103349", "openlibrary:OL492646W"}, {"Devotions Mary Oliver", "openlibrary:OL19720501W"}, {"Parable of the Sower by Octavia E Butler", "openlibrary:OL35623W"}} {
		t.Run(tc.query, func(t *testing.T) {
			outcome := service.SearchDetailed(context.Background(), Query{Query: tc.query, Type: SearchTypeBook, PreferredLanguage: "English", Limit: 10})
			if len(outcome.ProviderErrors) > 0 || len(outcome.Results) == 0 || outcome.Results[0].Work.ID != tc.work {
				t.Fatalf("live discovery mismatch: %+v", outcome)
			}
			r := outcome.Results[0]
			t.Logf("%s: %s, work=%s, edition=%s, language=%s, format=%s", tc.query, r.Work.Title, r.Work.ID, r.Edition.ID, r.Edition.Language, r.Edition.Format)
		})
		time.Sleep(1100 * time.Millisecond)
	}
}

func TestMalformedISBNNeverContactsProviders(t *testing.T) {
	p := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Open Library"}}
	for _, raw := range []string{"isbn:bad", "9781423103340", "ISBN-13: 9781423103340"} {
		outcome := NewService([]Provider{p}).SearchDetailed(context.Background(), Query{Query: raw, Type: SearchTypeBook})
		if len(outcome.ProviderErrors) != 1 || len(outcome.Results) != 0 || p.calls != 0 {
			t.Fatalf("invalid ISBN queried a provider: %+v", outcome)
		}
	}
	if ValidateSearchQuery(Query{Query: "ISBN: 1423103343", Type: SearchTypeBook}) != nil {
		t.Fatal("rejected valid ISBN-10")
	}
	if stableID("author", "Élan") != stableID("author", "lan") {
		t.Fatal("legacy synthetic ID changed")
	}
	if normalize("Élan") == normalize("lan") {
		t.Fatal("search still loses Unicode")
	}
}

func TestExactCoverlessEditionBeatsIllustratedWrongBook(t *testing.T) {
	rows := []SearchResult{
		{Provider: "Open Library", Kind: SearchTypeBook, Work: Work{ID: "openlibrary:OL1W", Title: "Wrong", CoverURL: "https://example.test/cover.jpg"}, Edition: Edition{ID: "openlibrary:OL1M", ISBNs: []string{"9780142437247"}}, Score: 0.99},
		{Provider: "Open Library", Kind: SearchTypeBook, Work: Work{ID: "openlibrary:OL2W", Title: "Correct"}, Edition: Edition{ID: "openlibrary:OL2M", ISBNs: []string{"1423103343"}}, Score: 0.1},
	}
	outcome := NewService([]Provider{staticMetadataProvider{name: "Open Library", results: rows}}).SearchDetailed(context.Background(), Query{Query: "9781423103349", Type: SearchTypeBook})
	if len(outcome.Results) != 2 || outcome.Results[0].Work.ID != "openlibrary:OL2W" {
		t.Fatalf("cover or score overrode edition identity: %+v", outcome)
	}
}
