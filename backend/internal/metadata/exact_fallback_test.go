package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestExactLookupValidatesAndCanonicalizesISBNs(t *testing.T) {
	for _, value := range []string{"0142437247", "9780142437247", "978-0-14-243724-7", "ISBN: 0142437247", "isbn-13: 9780142437247", "ＩＳＢＮ：９７８０１４２４３７２４７"} {
		lookup, ok := exactBookLookup(Query{Query: value})
		if !ok || lookup.isbn != "9780142437247" {
			t.Fatal(value, lookup, ok)
		}
	}
	for _, value := range []string{"9780142437248", "0142437248", "isbn:bad", "isbn:9780142437247 garbage", "97801424372470", ""} {
		if lookup, ok := exactBookLookup(Query{Query: value}); ok {
			t.Fatal("accepted invalid ISBN/query", value, lookup)
		}
	}
	lookup, ok := exactBookLookup(Query{Query: "1984"})
	if !ok || lookup.title != "1984" || lookup.isbn != "" {
		t.Fatal("numeric title lost", lookup, ok)
	}
	for _, kind := range []SearchType{SearchTypeAuthor, SearchTypeAuthorWorks, SearchTypeSeries} {
		if _, ok := exactBookLookup(Query{Query: "Moby Dick", Type: kind}); ok {
			t.Fatal(kind)
		}
	}
	if got := canonicalISBN("0-8044-2957-X"); got != "9780804429573" {
		t.Fatal("ISBN X check digit", got)
	}
}

func googleFixtureProvider(t *testing.T, body string, inspect func(*http.Request)) *GoogleBooksProvider {
	t.Helper()
	return NewGoogleBooksProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if inspect != nil {
			inspect(req)
		}
		return jsonResponse(body), nil
	})}, "fixture-key")
}
func TestGoogleISBNFallbackChecksReturnedIdentifiersAndPreservesEvidence(t *testing.T) {
	body := `{"totalItems":4,"items":[
 {"id":"exact","volumeInfo":{"title":"Moby-Dick","authors":["Herman Melville","Contributor"],"language":"en","industryIdentifiers":[{"type":"ISBN_10","identifier":"0142437247"}],"publisher":"Fixture press"},"saleInfo":{"isEbook":true}},
 {"id":"foreign","volumeInfo":{"title":"Moby-Dick","industryIdentifiers":[{"type":"ISBN_13","identifier":"9780593135204"}]}},
 {"id":"unproven","volumeInfo":{"title":"Moby-Dick"}},
 {"id":"invalid","volumeInfo":{"title":"Moby-Dick","industryIdentifiers":[{"type":"ISBN_13","identifier":"9780142437248"}]}}]}`
	p := googleFixtureProvider(t, body, func(req *http.Request) {
		values := req.URL.Query()
		if values.Get("q") != "isbn:9780142437247" || values.Get("projection") != "full" || !strings.Contains(values.Get("fields"), "industryIdentifiers") || values.Get("printType") != "books" {
			t.Fatal(values)
		}
	})
	results, err := p.Search(context.Background(), Query{Query: "ISBN: 9780142437247", Type: SearchTypeBook, Format: FormatEbook, PreferredLanguage: "English"})
	if err != nil || len(results) != 1 {
		t.Fatal(results, err)
	}
	r := results[0]
	if r.Work.ID != "googlebooks:exact" || r.RawSourceKey != "exact" || r.Edition.ProviderIDs[0] != "googlebooks:exact:edition" || len(r.Work.Authors) != 2 || r.Edition.Language != "en" || r.Edition.Format != FormatEbook || r.Score != 0.99 || !hasString(r.MatchedOn, "exact ISBN fallback") {
		t.Fatal(r)
	}
}
func TestGoogleTitleFallbackIsLiteralAndDoesNotInventAudioFormat(t *testing.T) {
	body := `{"totalItems":5,"items":[
 {"id":"exact","volumeInfo":{"title":"Dune","authors":["Frank Herbert"],"language":"en"}},
 {"id":"related","volumeInfo":{"title":"Dune Messiah","authors":["Frank Herbert"]}},
 {"id":"guide","volumeInfo":{"title":"Dune","subtitle":"A study guide"}},
 {"id":"translation","volumeInfo":{"title":"Dune","language":"fr"}},
 {"id":"ebook","volumeInfo":{"title":"Dune","language":"en"},"saleInfo":{"isEbook":true}}]}`
	p := googleFixtureProvider(t, body, func(req *http.Request) {
		if req.URL.Query().Get("q") != `intitle:"Dune"` {
			t.Fatal(req.URL.Query().Get("q"))
		}
	})
	results, err := p.Search(context.Background(), Query{Query: "Dune", Format: FormatAudiobook, PreferredLanguage: "English"})
	if err != nil || len(results) != 1 || results[0].RawSourceKey != "exact" || results[0].Edition.Format != FormatAny {
		t.Fatal(results, err)
	}
	if !hasString(results[0].MatchedOn, "exact title fallback") {
		t.Fatal(results)
	}
}
func TestGoogleFallbackSkipsAuthorSeriesAndInvalidISBNWithoutIO(t *testing.T) {
	calls := 0
	p := googleFixtureProvider(t, `{"totalItems":0}`, func(*http.Request) { calls++ })
	for _, q := range []Query{{Query: "Author", Type: SearchTypeAuthor}, {Query: "Author", Type: SearchTypeAuthorWorks}, {Query: "Series", Type: SearchTypeSeries}, {Query: "isbn:9780142437248"}, {Query: ""}} {
		rows, err := p.Search(context.Background(), q)
		if err != nil || len(rows) != 0 {
			t.Fatal(q, rows, err)
		}
	}
	if calls != 0 || p.Health(context.Background()).LastCheckedAt != nil {
		t.Fatal("ineligible fallback spent quota", calls)
	}
}
func TestGoogleExactTitlePreservesUnicodeAndSubtitle(t *testing.T) {
	p := googleFixtureProvider(t, `{"totalItems":2,"items":[{"id":"one","volumeInfo":{"title":"Élan","subtitle":"The Return","language":"en"}},{"id":"two","volumeInfo":{"title":"Elan","subtitle":"The Return"}}]}`, nil)
	rows, err := p.Search(context.Background(), Query{Query: "E\u0301lan: The Return"})
	if err != nil || len(rows) != 1 || rows[0].Work.Title != "Élan: The Return" {
		t.Fatal(rows, err)
	}
}

type fallbackTestProvider struct {
	staticMetadataProvider
	calls    int
	order    *[]string
	err      error
	observed Query
}

func (p *fallbackTestProvider) Search(_ Context, q Query) ([]SearchResult, error) {
	p.calls++
	p.observed = q
	if p.order != nil {
		*p.order = append(*p.order, p.name)
	}
	return p.results, p.err
}
func bookFixture(provider, title, format, language, isbn string) SearchResult {
	return SearchResult{Provider: provider, Kind: SearchTypeBook, Work: Work{ID: provider + ":fixture", Title: title, Authors: []Author{{Name: "Fixture Author"}}}, Edition: Edition{Title: title, Format: MediaFormat(format), Language: language, ISBNs: []string{isbn}}, Score: 0.8}
}

func TestFallbackRunsAfterPrimaryOnlyWhenExactSuitableMatchIsMissing(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		primary     SearchResult
		err         error
		want        int
	}{
		{"exact title", "Dune", bookFixture("Open Library", "Dune", "ebook", "en", ""), nil, 0},
		{"exact ISBN-10", "9780142437247", bookFixture("Open Library", "Moby-Dick", "ebook", "en", "0142437247"), nil, 0},
		{"fuzzy title", "Dune", bookFixture("Open Library", "Dune Messiah", "ebook", "en", ""), nil, 1},
		{"wrong language", "Dune", bookFixture("Open Library", "Dune", "ebook", "fr", ""), nil, 1},
		{"wrong format", "Dune", bookFixture("Open Library", "Dune", "audiobook", "en", ""), nil, 1},
		{"primary unavailable", "Dune", SearchResult{}, errors.New("primary fixture unavailable"), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			order := []string{}
			primary := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Open Library", results: []SearchResult{tc.primary}}, order: &order, err: tc.err}
			fallbackResult := bookFixture("Google Books", tc.query, "ebook", "en", tc.query)
			google := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Google Books", results: []SearchResult{fallbackResult}}, order: &order}
			result := NewService([]Provider{google, primary}).SearchDetailed(context.Background(), Query{Query: tc.query, Format: FormatEbook, PreferredLanguage: "English"})
			if google.calls != tc.want || len(order) == 0 || order[0] != "Open Library" {
				t.Fatal(order, google.calls, tc.want)
			}
			if tc.err != nil && (len(result.ProviderErrors) != 1 || len(result.Results) != 1) {
				t.Fatal(result)
			}
			if tc.name == "fuzzy title" && result.Results[0].Provider != "Google Books" {
				t.Fatal("exact fallback was outranked by fuzzy primary", result)
			}
		})
	}
}
func TestServiceRejectsInexactFallbackAndKeepsEmptyLists(t *testing.T) {
	google := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Google Books", results: []SearchResult{bookFixture("Google Books", "Dune Messiah", "any", "en", "")}}}
	service := NewService([]Provider{google})
	result := service.SearchDetailed(context.Background(), Query{Query: "Dune"})
	if len(result.Results) != 0 || result.Results == nil {
		t.Fatal(result)
	}
	raw, err := json.Marshal(result)
	if err != nil || !strings.Contains(string(raw), `"results":[]`) {
		t.Fatal(string(raw), err)
	}
	for _, kind := range []SearchType{SearchTypeAuthor, SearchTypeAuthorWorks, SearchTypeSeries} {
		result := service.SearchDetailed(context.Background(), Query{Query: "Dune", Type: kind})
		if len(result.Results) != 0 {
			t.Fatal(result)
		}
	}
	if google.calls != 1 {
		t.Fatal("service called fallback for bibliography/series", google.calls)
	}
}
func TestCanonicalISBNQueryReachesPrimaryAdapters(t *testing.T) {
	primary := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Open Library"}}
	query := "ISBN-10: 0142437247"
	result := NewService([]Provider{primary}).SearchDetailed(context.Background(), Query{Query: query})
	if primary.observed.Query != "9780142437247" || result.Query.Query != query {
		t.Fatal(primary.observed, result.Query)
	}
}

func TestFallbackRespectsUnfilteredLanguage(t *testing.T) {
	for _, preference := range []string{"", "Any", "all", "no preference"} {
		primary := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Open Library", results: []SearchResult{bookFixture("Open Library", "Dune", "ebook", "fr", "")}}}
		google := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Google Books"}}
		result := NewService([]Provider{primary, google}).SearchDetailed(context.Background(), Query{Query: "Dune", PreferredLanguage: preference})
		if len(result.Results) != 1 || google.calls != 0 {
			t.Fatal(preference, result, google.calls)
		}
		p := googleFixtureProvider(t, `{"totalItems":1,"items":[{"id":"fr","volumeInfo":{"title":"Dune","language":"fr"}}]}`, nil)
		rows, err := p.Search(context.Background(), Query{Query: "Dune", PreferredLanguage: preference})
		if err != nil || len(rows) != 1 {
			t.Fatal(preference, rows, err)
		}
	}
}

func TestFallbackFailureRetainsPrimaryResultsAndProviderErrors(t *testing.T) {
	primary := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Open Library", results: []SearchResult{bookFixture("Open Library", "Dune Messiah", "ebook", "en", "")}}}
	failed := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Hardcover"}, err: errors.New("fixture primary failure")}
	google := &fallbackTestProvider{staticMetadataProvider: staticMetadataProvider{name: "Google Books"}, err: errors.New("fixture fallback failure")}
	result := NewService([]Provider{google, failed, primary}).SearchDetailed(context.Background(), Query{Query: "Dune"})
	if len(result.Results) != 1 || result.Results[0].Work.Title != "Dune Messiah" || len(result.ProviderErrors) != 2 || result.ProviderErrors[1].Provider != "Google Books" {
		t.Fatal(result)
	}
}
