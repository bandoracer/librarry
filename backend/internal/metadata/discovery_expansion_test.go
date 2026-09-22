package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoveryExpandedBenchmark(t *testing.T) {
	root := filepath.Join("..", "..", "..", "research", "metadata-ranking", "expansion")
	read := func(path string, target any) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	var cases []struct{ ID, Query, Language string }
	var labels map[string]struct {
		Targets      []string
		KnownTop1Gap string
	}
	read("cases.json", &cases)
	read("labels.json", &labels)
	report := []map[string]any{}
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			calls := 0
			p := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls > 2 {
					t.Fatal("search exceeded request budget")
				}
				name := c.ID
				if req.URL.Query().Get("author") != "" {
					name += "-rescue"
				}
				var s struct {
					Data   json.RawMessage `json:"data"`
					URL    string          `json:"url"`
					Status int             `json:"status"`
				}
				if name == "parable-rescue" {
					data, err := os.ReadFile(filepath.Join(root, "..", "edge-cases", "snapshots", "parable-author-rescue.json"))
					if err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(data, &s); err != nil {
						t.Fatal(err)
					}
				} else {
					read(filepath.Join("snapshots", name+".json"), &s)
				}
				captured, err := url.Parse(s.URL)
				if err != nil || s.Status != 200 {
					t.Fatal("invalid provider fixture")
				}
				if req.URL.Path != captured.Path {
					t.Fatal("wrong search endpoint")
				}
				for _, field := range []string{"q", "title", "author", "lang", "limit", "fields"} {
					if !strings.EqualFold(req.URL.Query().Get(field), captured.Query().Get(field)) {
						t.Fatalf("request %s does not match captured request", field)
					}
				}
				return jsonResponse(string(s.Data)), nil
			})})
			outcome := NewService([]Provider{p}).SearchDetailed(context.Background(), Query{Query: c.Query, Type: SearchTypeBook, PreferredLanguage: c.Language, Limit: 10})
			rank := 0
			first := ""
			id := ""
			if len(outcome.Results) > 0 {
				first = outcome.Results[0].Work.Title
				id = outcome.Results[0].Work.ID
			}
			for i, r := range outcome.Results {
				if hasString(labels[c.ID].Targets, strings.TrimPrefix(r.Work.ID, "openlibrary:")) {
					rank = i + 1
					break
				}
			}
			report = append(report, map[string]any{"id": c.ID, "query": c.Query, "firstTitle": first, "firstID": id, "targetRank": rank, "requests": calls, "errors": outcome.ProviderErrors})
			if rank != 1 && !(labels[c.ID].KnownTop1Gap != "" && rank > 0 && rank <= 6) && os.Getenv("LIBRARRY_REPORT_DISCOVERY") != "1" {
				t.Errorf("intended target rank %d, first %s %s", rank, first, id)
			}
			t.Logf("target rank=%d; first=%s [%s]; requests=%d", rank, first, id, calls)
		})
	}
	if path := os.Getenv("LIBRARRY_DISCOVERY_REPORT_PATH"); path != "" {
		data, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStructuredRescueFailurePreservesPrimaryAndDoesNotCachePartial(t *testing.T) {
	calls := 0
	p := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Query().Get("author") != "" {
			return jsonResponse(`{"broken":true}`), nil
		}
		return jsonResponse(`{"docs":[{"key":"/works/OL1W","title":"Related book","author_name":["Someone Else"],"author_key":["OL2A"]}]}`), nil
	})})
	service := NewService([]Provider{p})
	query := Query{Query: "The Book by Right Writer", Type: SearchTypeBook}
	for i := 0; i < 2; i++ {
		outcome := service.SearchDetailed(context.Background(), query)
		if len(outcome.Results) != 1 || len(outcome.ProviderErrors) != 1 {
			t.Fatalf("partial primary evidence lost or error hidden: %+v", outcome)
		}
	}
	if calls != 4 {
		t.Fatalf("partial response cached or request budget exceeded: %d", calls)
	}
}

func TestExplicitAuthorRescueUsesStructuredFields(t *testing.T) {
	calls := 0
	p := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		v := req.URL.Query()
		if calls == 1 {
			return jsonResponse(`{"docs":[]}`), nil
		}
		if v.Get("q") != "the book" || v.Get("author") != "right writer" || v.Get("title") != "" || v.Get("limit") != "25" {
			t.Fatalf("incorrect structured lookup: %s", req.URL.RawQuery)
		}
		return jsonResponse(`{"docs":[{"key":"/works/OL1W","title":"The Book","author_name":["Right Writer"],"author_key":["OL2A"]}]}`), nil
	})})
	outcome := NewService([]Provider{p}).SearchDetailed(context.Background(), Query{Query: "The Book by Right Writer", Type: SearchTypeBook})
	if len(outcome.Results) != 1 || calls != 2 || !hasString(outcome.Results[0].Evidence, "Title and author match") {
		t.Fatalf("%+v calls %d", outcome, calls)
	}
}
