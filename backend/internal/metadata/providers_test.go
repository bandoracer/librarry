package metadata

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOpenLibraryAuthorLookupUsesAuthorSearch(t *testing.T) {
	provider := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/search/authors.json" {
			t.Fatalf("expected author search endpoint, got %s", req.URL.String())
		}
		if req.URL.Query().Get("q") != "Andy Weir" {
			t.Fatalf("expected author query, got %s", req.URL.RawQuery)
		}
		return jsonResponse(`{"docs":[{"key":"OL123A","name":"Andy Weir","top_work":"Project Hail Mary","work_count":5}]}`), nil
	})})

	results, err := provider.Search(context.Background(), Query{Query: "Andy Weir", Type: SearchTypeAuthor, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one author result, got %d", len(results))
	}
	result := results[0]
	if result.Kind != SearchTypeAuthor || result.Work.Title != "Andy Weir" || result.Work.Description != "Project Hail Mary" {
		t.Fatalf("expected normalized author identity, got %+v", result)
	}
	if len(result.Work.Authors) != 1 || result.Work.Authors[0].ID != "openlibrary:OL123A" || result.Work.Authors[0].Name != "Andy Weir" {
		t.Fatalf("expected Open Library author identity, got %+v", result.Work.Authors)
	}
	if result.RawSourceKey != "/authors/OL123A" || result.Confidence != "high" {
		t.Fatalf("expected high-confidence author source, got key=%q confidence=%q", result.RawSourceKey, result.Confidence)
	}
}

func TestOpenLibraryAuthorWorksUsesProviderKey(t *testing.T) {
	provider := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/authors/OL123A/works.json" {
			t.Fatalf("expected author works endpoint, got %s", req.URL.String())
		}
		if req.URL.Query().Get("limit") != "3" {
			t.Fatalf("expected requested limit, got %s", req.URL.RawQuery)
		}
		return jsonResponse(`{"entries":[{"key":"/works/OL1W","title":"Project Hail Mary","first_publish_date":"May 4, 2021","covers":[12345]}]}`), nil
	})})

	results, err := provider.Search(context.Background(), Query{
		Query:       "Andy Weir",
		Type:        SearchTypeAuthorWorks,
		Format:      FormatEbook,
		Limit:       3,
		ProviderKey: "openlibrary:OL123A",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one author work result, got %d", len(results))
	}
	result := results[0]
	if result.Kind != SearchTypeBook || result.Work.ID != "openlibrary:OL1W" || result.Work.Title != "Project Hail Mary" {
		t.Fatalf("expected normalized author work, got %+v", result)
	}
	if result.Work.FirstPublishYear != 2021 || result.Edition.PublishedDate != "May 4, 2021" || result.Edition.Format != FormatEbook {
		t.Fatalf("expected publication and format metadata, got work=%+v edition=%+v", result.Work, result.Edition)
	}
	if result.Work.Authors[0].ID != "openlibrary:OL123A" || result.Work.Authors[0].Name != "Andy Weir" {
		t.Fatalf("expected subscribed author identity, got %+v", result.Work.Authors)
	}
	if !strings.Contains(result.Work.CoverURL, "/b/id/12345-L.jpg") || result.Confidence != "high" {
		t.Fatalf("expected cover and high confidence, got cover=%q confidence=%q", result.Work.CoverURL, result.Confidence)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
