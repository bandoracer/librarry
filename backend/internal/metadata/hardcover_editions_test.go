package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const richBookFixture = `{"id":1,"title":"Fixture Book","release_year":1990,"release_date":"1990-01-02","description":"Work description","image":{"url":"https://example.test/work.jpg"},"contributions":[{"contribution":"Illustrator","author":{"id":9,"name":"Artist"}},{"contribution":"Author","author":{"id":7,"name":"Writer"}}],"default_ebook_edition":{"id":101,"book_id":1,"title":"Fixture Book","subtitle":"Ebook Edition","reading_format_id":4,"isbn_10":"0142437247","isbn_13":"9780142437247","language":{"language":"English"},"publisher":{"name":"Ebook Publisher"},"pages":301,"release_date":"2020-02-03","image":{"url":"https://example.test/ebook.jpg"}},"default_audio_edition":{"id":102,"book_id":1,"title":"Fixture Book","reading_format_id":2,"asin":"fixture-asin","audio_seconds":36001,"release_date":"2024-03-04","language":{"language":"English"},"contributions":[{"contribution":"Narrator","author":{"id":8,"name":"Reader"}}]}}`

func editionFixtureProvider(t *testing.T, book string) (*HardcoverProvider, *int) {
	t.Helper()
	calls := 0
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		var body struct {
			Query     string
			Variables map[string]json.RawMessage
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(body.Query, "SearchBookDetails") {
			if string(body.Variables["ids"]) != "[1]" || !strings.Contains(body.Query, "default_ebook_edition") || !strings.Contains(body.Query, "default_audio_edition") {
				t.Fatal(body)
			}
			return jsonResponse(`{"data":{"books":[` + book + `]}}`), nil
		}
		return jsonResponse(`{"data":{"search":{"results":[{"id":1,"title":"Fixture Book","author_names":["Unverified name"]}]}}}`), nil
	})}, "fixture")
	return p, &calls
}
func TestHardcoverRichSearchKeepsWorkAndEditionEvidenceSeparate(t *testing.T) {
	p, calls := editionFixtureProvider(t, richBookFixture)
	service := NewService([]Provider{p})
	result := service.SearchDetailed(context.Background(), Query{Query: "Fixture Book", Format: FormatAny, PreferredLanguage: "English"})
	if *calls != 2 || len(result.ProviderErrors) != 0 || len(result.Results) != 2 {
		t.Fatal(*calls, result)
	}
	ebook, audio := result.Results[0], result.Results[1]
	if ebook.Edition.ID != "hardcover-edition:101" || ebook.Edition.Format != FormatEbook || ebook.Edition.Pages != 301 || ebook.Edition.Language != "English" || ebook.Edition.Publisher != "Ebook Publisher" || ebook.Edition.CoverURL == "" || ebook.Edition.PublishedDate != "2020-02-03" || len(ebook.Edition.ISBNs) != 2 {
		t.Fatal(ebook)
	}
	if audio.Edition.ID != "hardcover-edition:102" || audio.Edition.Format != FormatAudiobook || audio.Edition.AudioSeconds != 36001 || audio.Edition.ASIN != "fixture-asin" || len(audio.Edition.Contributors) != 1 || audio.Edition.Contributors[0].Role != "Narrator" || len(audio.Edition.ISBNs) != 0 {
		t.Fatal(audio)
	}
	for _, r := range result.Results {
		if r.Work.ID != "hardcover:1" || r.Work.FirstPublishDate != "1990-01-02" || r.Work.FirstPublishYear != 1990 || r.Work.Authors[0].ID != "hardcover-author:7" || r.Work.Authors[0].Name != "Writer" || r.Work.Authors[0].Role != "Author" || len(r.Work.Authors) != 2 {
			t.Fatal(r)
		}
	}
	// Cached snapshots retain new nested contributors without sharing mutable slices.
	result.Results[1].Edition.Contributors[0].Name = "Mutated"
	cached := service.SearchDetailed(context.Background(), Query{Query: "Fixture Book", Format: FormatAny, PreferredLanguage: "English"})
	if *calls != 2 || cached.Results[1].Edition.Contributors[0].Name != "Reader" {
		t.Fatal("cache shared mutable edition evidence", cached, *calls)
	}
}
func TestHardcoverDefaultEditionSelectionDoesNotInventFormat(t *testing.T) {
	for _, test := range []struct {
		name, book string
		format     MediaFormat
		want       string
	}{
		{"ebook", richBookFixture, FormatEbook, "hardcover-edition:101"},
		{"audio", richBookFixture, FormatAudiobook, "hardcover-edition:102"},
		{"unknown", `{"id":1,"title":"Fixture Book"}`, FormatAudiobook, ""},
		{"physical", `{"id":1,"title":"Fixture Book","default_ebook_edition":{"id":101,"book_id":1,"reading_format_id":1}}`, FormatEbook, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, _ := editionFixtureProvider(t, test.book)
			rows, err := p.Search(context.Background(), Query{Query: "Fixture Book", Format: test.format})
			if err != nil || len(rows) != 1 || rows[0].Edition.ID != test.want {
				t.Fatal(rows, err)
			}
			if test.want == "" && rows[0].Edition.Format != FormatAny {
				t.Fatal(rows)
			}
		})
	}
}
func TestHardcoverEditionValidationRejectsUnrelatedOrConflictingDetails(t *testing.T) {
	for _, book := range []string{
		strings.Replace(richBookFixture, `"book_id":1`, `"book_id":2`, 1),
		strings.Replace(richBookFixture, `"reading_format_id":4`, `"reading_format_id":2`, 1),
		strings.Replace(richBookFixture, `"isbn_13":"9780142437247"`, `"isbn_13":"9780593135204"`, 1),
		strings.Replace(richBookFixture, `"id":101`, `"id":0`, 1),
		strings.Replace(richBookFixture, `"id":102`, `"id":101`, 1),
		strings.Replace(richBookFixture, `"id":7`, `"id":0`, 1),
		strings.Replace(richBookFixture, `"id":1`, `"id":2`, 1),
	} {
		p, _ := editionFixtureProvider(t, book)
		rows, err := p.Search(context.Background(), Query{Query: "Fixture Book"})
		if err == nil || rows != nil || p.Health(context.Background()).Status != "degraded" {
			t.Fatal(rows, err)
		}
	}
}
func TestHardcoverExactISBNSearchUsesEditionIdentityAndEquivalentISBN10(t *testing.T) {
	for _, isbn := range []string{"9780142437247", "0142437247", "isbn:9780142437247"} {
		calls := 0
		p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			var body struct {
				Query     string
				Variables map[string]json.RawMessage
			}
			json.NewDecoder(req.Body).Decode(&body)
			if !strings.Contains(body.Query, "ExactBookEdition") || string(body.Variables["isbn13"]) != `"9780142437247"` || string(body.Variables["isbn10s"]) != `["0142437247"]` {
				t.Fatal(body)
			}
			return jsonResponse(`{"data":{"editions":[{"id":201,"book_id":1,"reading_format_id":4,"isbn_10":"0142437247","release_date":"2001-01-01","book":{"id":1,"title":"Fixture Book"}}]}}`), nil
		})}, "fixture")
		rows, err := p.Search(context.Background(), Query{Query: isbn, Format: FormatEbook})
		if err != nil || calls != 1 || len(rows) != 1 || rows[0].Edition.ID != "hardcover-edition:201" || rows[0].Score != 0.99 {
			t.Fatal(rows, calls, err)
		}
	}
}
func TestHardcoverISBN979NeverMatchesEmptyISBN10(t *testing.T) {
	isbn := "979123456789"
	isbn += string(rune('0' + isbn13CheckDigit(isbn)))
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			Query     string
			Variables map[string]json.RawMessage
		}
		json.NewDecoder(req.Body).Decode(&body)
		if string(body.Variables["isbn10s"]) != "[]" || !strings.Contains(body.Query, "isbn_10:{_in:$isbn10s}") {
			t.Fatal(body)
		}
		return jsonResponse(`{"data":{"editions":[]}}`), nil
	})}, "fixture")
	if rows, err := p.Search(context.Background(), Query{Query: isbn}); err != nil || rows == nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}
func TestRichEditionMergeRejectsFormatLanguageAndIdentityConflicts(t *testing.T) {
	base := SearchResult{Provider: "Hardcover", Kind: SearchTypeBook, Work: Work{Title: "Book", Authors: []Author{{Name: "Writer"}}}, Edition: Edition{ID: "hardcover-edition:1", Format: FormatEbook, Language: "English", ISBNs: []string{"9780142437247"}}}
	for _, mutate := range []func(*SearchResult){
		func(r *SearchResult) { r.Edition.Format = FormatAudiobook },
		func(r *SearchResult) { r.Edition.Language = "French" },
		func(r *SearchResult) { r.Edition.ID = "hardcover-edition:2" },
		func(r *SearchResult) { r.Provider = "Open Library"; r.Edition.ISBNs = []string{"9780593135204"} },
	} {
		other := base
		mutate(&other)
		if resultsCanMerge(Query{Format: FormatEbook}, base, other) {
			t.Fatal("merged distinct edition", other)
		}
	}
	other := base
	other.Provider = "Open Library"
	other.Edition.ID = "openlibrary:OL1M"
	other.Edition.ISBNs = []string{"0142437247"}
	if !resultsCanMerge(Query{}, base, other) {
		t.Fatal("equivalent ISBN-10 did not corroborate")
	}
	unknown := base
	unknown.Provider = "Local"
	unknown.Edition = Edition{}
	audio := base
	audio.Provider = "Open Library"
	audio.Edition = Edition{ID: "openlibrary:OL2M", Format: FormatAudiobook}
	if rows := mergeEquivalentResults(Query{}, []SearchResult{unknown, base, audio}); len(rows) != 2 {
		t.Fatal("unknown work bridged incompatible editions", rows)
	}
}

func TestHardcoverBibliographyUsesRequestedDefaultEdition(t *testing.T) {
	calls := 0
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		var body struct {
			Query     string
			Variables map[string]int
		}
		json.NewDecoder(req.Body).Decode(&body)
		if strings.Contains(body.Query, "AuthorIdentity") {
			return jsonResponse(`{"data":{"authors":[{"id":7,"name":"Writer"}]}}`), nil
		}
		if !strings.Contains(body.Query, "default_audio_edition") {
			t.Fatal("missing edition selection", body.Query)
		}
		if body.Variables["after"] > 0 {
			return jsonResponse(`{"data":{"books":[]}}`), nil
		}
		return jsonResponse(`{"data":{"books":[` + richBookFixture + `]}}`), nil
	})}, "fixture")
	rows, err := p.Bibliography(context.Background(), Query{ProviderKey: "hardcover-author:7", Format: FormatAudiobook})
	if err != nil || len(rows) != 1 || calls != 3 || rows[0].Edition.ID != "hardcover-edition:102" || rows[0].Work.FirstPublishDate != "1990-01-02" {
		t.Fatal(rows, err, calls)
	}
}
func TestHardcoverExactEditionRejectsWrongIdentityAndAcceptsKnownEmpty(t *testing.T) {
	for _, body := range []string{
		`{"data":{"editions":null}}`,
		`{"data":{"editions":[{"id":1,"book_id":1,"isbn_13":"9780142437247"}]}}`,
		`{"data":{"editions":[{"id":1,"book_id":1,"isbn_13":"9780593135204","book":{"id":1,"title":"Wrong ISBN"}}]}}`,
	} {
		p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(body), nil })}, "fixture")
		rows, err := p.Search(context.Background(), Query{Query: "9780142437247"})
		if err == nil || rows != nil || p.Health(context.Background()).Status != "degraded" {
			t.Fatal(rows, err)
		}
	}
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(`{"data":{"editions":[]}}`), nil })}, "fixture")
	if rows, err := p.Search(context.Background(), Query{Query: "9780142437247"}); err != nil || rows == nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}
func TestHardcoverFailedEnrichmentReturnsNoUnverifiedSearchDetails(t *testing.T) {
	calls := 0
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return jsonResponse(`{"data":{"search":{"results":[{"id":1,"title":"Fixture"}]}}}`), nil
		}
		response := jsonResponse(`{"error":"fixture-secret"}`)
		response.StatusCode = 429
		response.Header.Set("Retry-After", "60")
		return response, nil
	})}, "fixture")
	rows, err := p.Search(context.Background(), Query{Query: "Fixture"})
	if err == nil || rows != nil || strings.Contains(err.Error(), "fixture-secret") || p.Health(context.Background()).Status != "rate_limited" {
		t.Fatal(rows, err)
	}
}

func TestEditionContributorMergesPreserveRolesAndSameNameIdentities(t *testing.T) {
	base := Edition{Contributors: []Author{{ID: "hardcover-author:7", Name: "Alex", Role: "Author"}}}
	merged := mergeEdition(base, Edition{Contributors: []Author{{ID: "hardcover-author:7", Name: "Alex", Role: "Narrator"}, {ID: "hardcover-author:8", Name: "Alex", Role: "Author"}}})
	if len(merged.Contributors) != 3 {
		t.Fatal(merged)
	}
}
