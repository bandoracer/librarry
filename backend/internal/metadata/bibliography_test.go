package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestHardcoverAuthorDiscoveryKeepsSameNameIdentitiesSeparate(t *testing.T) {
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			Query string `json:"query"`
		}
		json.NewDecoder(req.Body).Decode(&body)
		if !strings.Contains(body.Query, `query_type:"Author"`) {
			t.Fatal(body.Query)
		}
		return jsonResponse(`{"data":{"search":{"results":{"hits":[{"document":{"id":1,"name":"Alex Smith","image":{"url":"https://example.test/author.jpg"}}},{"document":{"id":2,"name":"Alex Smith"}}]}}}}`), nil
	})}, "fixture")
	ol := staticMetadataProvider{name: "Open Library", results: []SearchResult{{Kind: SearchTypeAuthor, Provider: "Open Library", Work: Work{Title: "Alex Smith", Authors: []Author{{ID: "openlibrary:OL3A", Name: "Alex Smith"}}}}}}
	out := NewService([]Provider{p, ol}).SearchDetailed(context.Background(), Query{Query: "Alex Smith", Type: SearchTypeAuthor})
	if len(out.Results) != 3 || len(out.ProviderErrors) != 0 {
		t.Fatal(out)
	}
	if out.Results[0].Work.Authors[0].ID != "hardcover-author:1" || out.Results[0].Work.CoverURL == "" {
		t.Fatal(out)
	}
}
func TestHardcoverAuthorMalformedIdentityIsDegraded(t *testing.T) {
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"data":{"search":{"results":[{"name":"Alex"}]}}}`), nil
	})}, "fixture")
	if _, err := p.Search(context.Background(), Query{Type: SearchTypeAuthor, Query: "Alex"}); err == nil {
		t.Fatal("accepted missing ID")
	}
	if p.Health(context.Background()).Status != "degraded" {
		t.Fatal(p.Health(context.Background()))
	}
}
func hardcoverBibliographyFixture(t *testing.T, failPage bool) (*HardcoverProvider, *[]int) {
	t.Helper()
	cursors := []int{}
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]int `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(body.Query, "AuthorIdentity") {
			return jsonResponse(`{"data":{"authors":[{"id":7,"name":"Fixture Author"}]}}`), nil
		}
		after := body.Variables["after"]
		cursors = append(cursors, after)
		if body.Variables["author"] != 7 || body.Variables["limit"] != 100 || !strings.Contains(body.Query, "order_by:{id:asc}") {
			t.Fatal(body)
		}
		if failPage && after > 0 {
			return jsonResponse(`{"errors":[{"message":"fixture page failed"}]}`), nil
		}
		books := []map[string]any{}
		for id := after + 1; id <= min(after+100, 205); id++ {
			books = append(books, map[string]any{"id": id, "title": fmt.Sprintf("Book %d", id), "release_date": "2020-01-01", "contributions": []any{map[string]any{"contribution": "Author", "author": map[string]any{"id": 7, "name": "Fixture Author"}}}})
		}
		raw, _ := json.Marshal(map[string]any{"data": map[string]any{"books": books}})
		return jsonResponse(string(raw)), nil
	})}, "fixture")
	return p, &cursors
}
func TestHardcoverBibliographyTraversesEveryPageBeyondSearchLimit(t *testing.T) {
	p, cursors := hardcoverBibliographyFixture(t, false)
	results, err := NewService([]Provider{p}).AuthorBibliography(context.Background(), Query{Query: "Fixture", ProviderKey: "hardcover-author:7", Limit: 3, Format: FormatAudiobook})
	if err != nil || len(results) != 205 || fmt.Sprint(*cursors) != "[0 100 200 205]" {
		t.Fatal(len(results), err, *cursors)
	}
	last := results[204]
	if last.Work.ID != "hardcover:205" || last.Edition.Format != FormatAny || last.Edition.PublishedDate != "" || last.Work.FirstPublishDate != "2020-01-01" || last.Work.Authors[0].Role != "Author" {
		t.Fatal(last)
	}
}
func TestHardcoverBibliographyPageFailureReturnsNoPartialResults(t *testing.T) {
	p, _ := hardcoverBibliographyFixture(t, true)
	results, err := NewService([]Provider{p}).AuthorBibliography(context.Background(), Query{ProviderKey: "hardcover-author:7"})
	if err == nil || len(results) != 0 || p.Health(context.Background()).Status != "degraded" {
		t.Fatal(results, err, p.Health(context.Background()))
	}
}
func TestHardcoverBibliographyRejectsBrokenIdentityAndPagination(t *testing.T) {
	for _, body := range []string{`{}`, `{"books":null}`, `{"books":[{"id":1,"title":"Wrong author","contributions":[{"author":{"id":8,"name":"Other"}}]}]}`, `{"books":[{"id":0,"title":"No ID"}]}`, `{"books":[{"id":1,"title":"Duplicate","contributions":[{"author":{"id":7,"name":"Fixture"}}]}]}`} {
		calls := 0
		p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return jsonResponse(`{"data":{"authors":[{"id":7,"name":"Fixture"}]}}`), nil
			}
			return jsonResponse(`{"data":` + body + `}`), nil
		})}, "fixture")
		rows, err := p.Bibliography(context.Background(), Query{ProviderKey: "hardcover-author:7"})
		if err == nil || len(rows) != 0 || calls > 3 {
			t.Fatal(body, rows, err, calls)
		}
	}
}
func openLibraryBibliographyFixture(t *testing.T, fault string) (*OpenLibraryProvider, *[]int) {
	t.Helper()
	offsets := []int{}
	p := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/authors/OL7A.json" {
			return jsonResponse(`{"key":"/authors/OL7A","name":"Actual Author"}`), nil
		}
		if req.URL.Path != "/authors/OL7A/works.json" || req.URL.Query().Get("limit") != "100" {
			t.Fatal(req.URL)
		}
		offset, _ := strconv.Atoi(req.URL.Query().Get("offset"))
		offsets = append(offsets, offset)
		size := 205
		if fault == "count" && offset > 0 {
			size = 206
		}
		if fault == "limit" {
			size = 10001
		}
		entries := []map[string]any{}
		for i := offset + 1; i <= min(offset+100, 205); i++ {
			id := i
			if fault == "duplicate" && offset > 0 {
				id = 1
			}
			entries = append(entries, map[string]any{"key": fmt.Sprintf("/works/OL%dW", id), "title": fmt.Sprintf("Book %d", i), "first_publish_date": "2020-01-01"})
		}
		if fault == "short" && offset > 0 {
			entries = []map[string]any{}
		}
		if fault == "malformed" && offset > 0 {
			return jsonResponse(`{}`), nil
		}
		raw, _ := json.Marshal(map[string]any{"size": size, "entries": entries})
		return jsonResponse(string(raw)), nil
	})})
	return p, &offsets
}
func TestOpenLibraryBibliographyVerifiesCountAndTraversesAllPages(t *testing.T) {
	p, offsets := openLibraryBibliographyFixture(t, "")
	rows, err := NewService([]Provider{p}).AuthorBibliography(context.Background(), Query{Query: "Untrusted name", ProviderKey: "/authors/OL7A", Limit: 1, Format: FormatEbook})
	if err != nil || len(rows) != 205 || fmt.Sprint(*offsets) != "[0 100 200]" {
		t.Fatal(len(rows), err, *offsets)
	}
	r := rows[204]
	if r.Work.Authors[0].Name != "Actual Author" || r.Work.FirstPublishDate != "2020-01-01" || r.Edition.PublishedDate != "" || r.Edition.Format != FormatAny {
		t.Fatal(r)
	}
}
func TestOpenLibraryBibliographyFailuresNeverApplyPartialLists(t *testing.T) {
	for _, fault := range []string{"count", "duplicate", "short", "malformed", "limit"} {
		t.Run(fault, func(t *testing.T) {
			p, _ := openLibraryBibliographyFixture(t, fault)
			rows, err := p.Bibliography(context.Background(), Query{ProviderKey: "openlibrary:OL7A"})
			if err == nil || len(rows) != 0 || p.Health(context.Background()).Status != "degraded" {
				t.Fatal(len(rows), err, p.Health(context.Background()))
			}
		})
	}
}
func TestBibliographyRoutesOnlyExactProviderAndRejectsNameHashes(t *testing.T) {
	p, _ := openLibraryBibliographyFixture(t, "")
	google := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Google Books"}}
	hc := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("queried wrong provider"); return nil, nil })}, "fixture")
	s := NewService([]Provider{hc, p, google})
	if _, err := s.AuthorBibliography(context.Background(), Query{ProviderKey: "openlibrary:OL7A"}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "Hardcover:author:alex", "hardcover-author:hash", "openlibrary:OL../7A", "OLxA"} {
		if _, err := s.AuthorBibliography(context.Background(), Query{Query: "Alex", ProviderKey: key}); err == nil {
			t.Fatal(key)
		}
	}
	if google.calls != 0 {
		t.Fatal(google.calls)
	}
}

func TestHardcoverLargeNumericIDsRemainStable(t *testing.T) {
	for _, value := range []any{float64(12345678), "12345678", json.Number("12345678")} {
		if got := hardcoverDocumentID(value); got != 12345678 {
			t.Fatal(value, got)
		}
	}
	for _, value := range []any{float64(1.5), float64(2147483648), "hash", nil, float64(-1)} {
		if got := hardcoverDocumentID(value); got != 0 {
			t.Fatal(value, got)
		}
	}
}
func TestEmptyBibliographyRequiresVerifiedAuthor(t *testing.T) {
	for _, valid := range []bool{false, true} {
		calls := 0
		hc := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				if valid {
					return jsonResponse(`{"data":{"authors":[{"id":7,"name":"Fixture"}]}}`), nil
				}
				return jsonResponse(`{"data":{"authors":[]}}`), nil
			}
			return jsonResponse(`{"data":{"books":[]}}`), nil
		})}, "fixture")
		ol := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/authors/OL7A.json" {
				if valid {
					return jsonResponse(`{"key":"/authors/OL7A","name":"Fixture"}`), nil
				}
				return jsonResponse(`{}`), nil
			}
			return jsonResponse(`{"size":0,"entries":[]}`), nil
		})})
		for _, tc := range []struct {
			p   Provider
			key string
		}{{hc, "hardcover-author:7"}, {ol, "openlibrary:OL7A"}} {
			rows, err := NewService([]Provider{tc.p}).AuthorBibliography(context.Background(), Query{ProviderKey: tc.key})
			if (err == nil) != valid || len(rows) != 0 || (valid && rows == nil) {
				t.Fatal(valid, tc.p.Name(), rows, err)
			}
		}
	}
}
