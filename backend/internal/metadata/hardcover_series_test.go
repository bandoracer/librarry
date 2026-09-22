package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func seriesFixtureProvider(t *testing.T, body string, query string) (*HardcoverProvider, *int) {
	t.Helper()
	calls := 0
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		var payload struct {
			Query     string
			Variables map[string]json.RawMessage
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Query == hardcoverSeriesDiscoveryQuery {
			var got string
			_ = json.Unmarshal(payload.Variables["query"], &got)
			if got != query {
				t.Fatalf("wrong series search %q", got)
			}
			return jsonResponse(`{"data":{"search":{"results":{"hits":[{"document":{"id":"3","name":"` + query + `"}},{"document":{"id":"9","name":"` + query + ` extended"}}]}}}}`), nil
		}
		if payload.Query != hardcoverSeriesQuery || string(payload.Variables["ids"]) != "[3,9]" || strings.Contains(payload.Query, "_ilike") {
			t.Fatalf("wrong series request: %+v", payload)
		}
		return jsonResponse(body), nil
	})}, "fixture")
	return p, &calls
}
func seriesEntry(id int, position string) string {
	return fmt.Sprintf(`{"book_id":%d,"position":%s,"featured":false,"compilation":false,"book":{"id":%d,"title":"Book %d","release_year":%d}}`, id, position, id, id, 2030-id)
}
func TestHardcoverSeriesOrderAndCache(t *testing.T) {
	// Publication years deliberately oppose numbered order. Zero and fractions
	// are real positions; null stays unknown. Overlapping series stay separate.
	body := `{"data":{"series":[{"id":9,"name":"Test extended","book_series":[` + seriesEntry(1, "2") + `]},{"id":3,"name":"Test","book_series":[` + strings.Join([]string{seriesEntry(3, "null"), seriesEntry(2, "1.5"), seriesEntry(1, "0"), seriesEntry(4, "1")}, ",") + `]}]}}`
	p, calls := seriesFixtureProvider(t, body, "Test")
	s := NewService([]Provider{p, NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("series must not use title search fallback")
		return nil, nil
	})})})
	q := Query{Query: "Test", Type: SearchTypeSeries, Limit: 50}
	for attempt := 0; attempt < 2; attempt++ {
		outcome := s.SearchDetailed(context.Background(), q)
		if len(outcome.ProviderErrors) != 0 || len(outcome.Results) != 5 {
			t.Fatal(outcome)
		}
		for i, want := range []string{"hardcover:1", "hardcover:4", "hardcover:2", "hardcover:3", "hardcover:1"} {
			if outcome.Results[i].Work.ID != want {
				t.Fatal(outcome.Results)
			}
		}
		if outcome.Results[0].Work.SeriesID != "hardcover-series:3" || outcome.Results[0].Work.SeriesPosition != "0" || outcome.Results[4].Work.SeriesID != "hardcover-series:9" || !hasString(outcome.Results[2].Evidence, "Series position 1.5") || !hasString(outcome.Results[3].Evidence, "Series position unknown") {
			t.Fatal(outcome)
		}
	}
	if *calls != 2 {
		t.Fatal("series cache missed", *calls)
	}
}
func TestHardcoverSeriesRejectsInvalidMembership(t *testing.T) {
	valid := `{"data":{"series":[{"id":3,"name":"Test","book_series":[` + seriesEntry(1, "1") + `]}]}}`
	for _, body := range []string{
		`{"data":{}}`,
		strings.Replace(valid, `"book_id":1`, `"book_id":2`, 1),
		strings.Replace(valid, `"compilation":false`, `"compilation":true`, 1),
		strings.Replace(valid, `"name":"Test"`, `"name":"Unrelated"`, 1),
		strings.Replace(valid, `"id":3`, `"id":0`, 1),
		strings.Replace(valid, `"book_series":[`+seriesEntry(1, "1")+`]`, `"book_series":[`+seriesEntry(1, "1")+`,`+seriesEntry(1, "2")+`]`, 1),
	} {
		p, _ := seriesFixtureProvider(t, body, "Test")
		outcome := NewService([]Provider{p}).SearchDetailed(context.Background(), Query{Query: "Test", Type: SearchTypeSeries})
		if len(outcome.Results) != 0 || len(outcome.ProviderErrors) != 1 {
			t.Fatalf("accepted invalid membership %s: %+v", body, outcome)
		}
	}
}
func TestHardcoverSeriesEmptyAndLiteralPattern(t *testing.T) {
	p, _ := seriesFixtureProvider(t, `{"data":{"series":[]}}`, `100%_Test`)
	out := NewService([]Provider{p}).SearchDetailed(context.Background(), Query{Query: "100%_Test", Type: SearchTypeSeries})
	if len(out.Results) != 0 || len(out.ProviderErrors) != 0 {
		t.Fatal(out)
	}
	missing := NewService([]Provider{NewHardcoverProvider(nil, "")}).SearchDetailed(context.Background(), Query{Query: "Test", Type: SearchTypeSeries})
	if len(missing.Results) != 0 || len(missing.ProviderErrors) != 1 || !strings.Contains(missing.ProviderErrors[0].Message, "token") {
		t.Fatal(missing)
	}
}
func TestSeriesMergeDoesNotMixNamesAndPositions(t *testing.T) {
	got := mergeWork(Work{Series: "Other", SeriesID: "hardcover-series:1"}, Work{Series: "Test", SeriesID: "hardcover-series:2", SeriesPosition: "5"})
	if got.Series != "Other" || got.SeriesPosition != "" || got.SeriesID != "hardcover-series:1" {
		t.Fatal(got)
	}
}

func TestHardcoverSeriesDiscoveryBoundsAndIdentity(t *testing.T) {
	for _, hits := range []string{
		`[{"document":{"id":"0","name":"Test"}}]`,
		`[{"document":{"id":"3","name":"Test"}},{"document":{"id":"3","name":"Test"}}]`,
		`[{"document":{"id":"3"}}]`,
		`[{"document":{"id":"1","name":"Test"}},{"document":{"id":"2","name":"Test"}},{"document":{"id":"3","name":"Test"}},{"document":{"id":"4","name":"Test"}}]`,
	} {
		p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(`{"data":{"search":{"results":{"hits":` + hits + `}}}}`), nil
		})}, "fixture")
		_, err := p.Search(context.Background(), Query{Query: "Test", Type: SearchTypeSeries})
		if err == nil {
			t.Fatalf("accepted invalid discovery: %s", hits)
		}
	}
}

func TestHardcoverSeriesUsableEditionsPrecedeSparseDuplicates(t *testing.T) {
	sparse := seriesEntry(1, "1")
	verified := strings.Replace(seriesEntry(2, "2"), `"release_year":2028`, `"release_year":2028,"default_ebook_edition":{"id":22,"book_id":2,"title":"Book 2","reading_format_id":4,"language":{"language":"English"}}`, 1)
	body := `{"data":{"series":[{"id":3,"name":"Test","book_series":[` + sparse + `,` + verified + `]}]}}`
	p, _ := seriesFixtureProvider(t, body, "Test")
	got, err := p.Search(context.Background(), Query{Query: "Test", Type: SearchTypeSeries})
	if err != nil || len(got) != 2 || got[0].Work.ID != "hardcover:2" || got[1].Work.ID != "hardcover:1" {
		t.Fatalf("usable edition buried: %+v %v", got, err)
	}
}
