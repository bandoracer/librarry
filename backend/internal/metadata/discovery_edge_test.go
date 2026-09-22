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

func TestDiscoveryCapturedEdgeCases(t *testing.T) {
	root := filepath.Join("..", "..", "..", "research", "metadata-ranking", "edge-cases")
	var cases []struct {
		ID, Query, Target, Format string
		Empty                     bool
	}
	b, err := os.ReadFile(filepath.Join(root, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	report := []map[string]any{}
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			calls := 0
			p := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls > 2 {
					t.Fatal("exceeded request budget")
				}
				name := c.ID
				if req.URL.Query().Get("author") != "" {
					name += "-author-rescue"
				}
				b, err := os.ReadFile(filepath.Join(root, "snapshots", name+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var snapshot struct {
					URL    string
					Status int
					Data   json.RawMessage
				}
				if err = json.Unmarshal(b, &snapshot); err != nil {
					t.Fatal(err)
				}
				captured, err := url.Parse(snapshot.URL)
				if err != nil || snapshot.Status != 200 {
					t.Fatal("invalid fixture")
				}
				// Historical captures predate the optional subject projection.
				actual := req.URL.Query()
				actual.Set("fields", strings.ReplaceAll(actual.Get("fields"), ",subject,", ","))
				if captured.Path != req.URL.Path || captured.Query().Encode() != actual.Encode() {
					t.Fatalf("wrong fixture request %s", req.URL)
				}
				return jsonResponse(string(snapshot.Data)), nil
			})})
			service := NewService([]Provider{p})
			q := Query{Query: c.Query, Type: SearchTypeBook, Format: MediaFormat(c.Format), PreferredLanguage: "English", Limit: 10}
			out := service.SearchDetailed(context.Background(), q)
			if len(out.ProviderErrors) > 0 {
				t.Fatal(out.ProviderErrors)
			}
			rank := 0
			first := ""
			for i, r := range out.Results {
				if i == 0 {
					first = r.Work.ID
				}
				if strings.TrimPrefix(r.Work.ID, "openlibrary:") == c.Target && c.Target != "" {
					rank = i + 1
				}
				if c.Format == "audiobook" && (r.Edition.Format != FormatAny || !hasString(r.Evidence, "Edition format unknown")) {
					t.Fatal("Open Library invented audio format", r)
				}
			}
			// Typos are retained as known retrieval misses, not counted as successes.
			knownMiss := c.ID == "percy-typo" || c.ID == "hail-typo"
			if c.Empty || knownMiss {
				if len(out.Results) != 0 {
					t.Fatalf("review changed empty observation: %+v", out)
				}
			} else if rank != 1 {
				t.Fatalf("intended work rank %d: %+v", rank, out)
			}
			before := calls
			cached := service.SearchDetailed(context.Background(), q)
			if calls != before || len(cached.Results) != len(out.Results) {
				t.Fatal("cache replay differs")
			}
			report = append(report, map[string]any{"id": c.ID, "query": c.Query, "targetRank": rank, "firstID": first, "results": len(out.Results), "requests": calls, "knownRetrievalMiss": knownMiss})
			t.Logf("rank %d, results %d, requests %d, known retrieval miss %t", rank, len(out.Results), calls, knownMiss)
		})
	}
	if path := os.Getenv("LIBRARRY_EDGE_REPORT_PATH"); path != "" {
		b, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
