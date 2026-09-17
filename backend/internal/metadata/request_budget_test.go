package metadata_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/importlists"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/providerhttp"
)

type budgetRoundTrip func(*http.Request) (*http.Response, error)

func (f budgetRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func budgetResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Ratelimit": {`"daily";r=0;t=60`}}, Body: io.NopCloser(strings.NewReader(body))}
}
func TestMetadataAndListsShareQuotaWithoutInventingRequestEvidence(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: providerhttp.NewTransport(budgetRoundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Fatal("incorrect token normalization")
		}
		return budgetResponse(`{"data":{"search":{"results":[{"id":1,"name":"Dune"}]}}}`), nil
	}))}
	p := metadata.NewHardcoverProvider(client, "fixture-token")
	s := metadata.NewService([]metadata.Provider{p})
	q := metadata.Query{Query: "Dune", Type: metadata.SearchTypeAuthor}
	initial := s.SearchDetailed(context.Background(), q)
	if len(initial.Results) != 1 || len(initial.ProviderErrors) != 0 {
		t.Fatal(initial)
	}
	before := p.Health(context.Background())
	if before.Status != "rate_limited" || before.RetryAfter == nil || before.LastSuccessAt == nil || before.Authenticated == nil || !*before.Authenticated {
		t.Fatal(before)
	}
	lists := importlists.NewHardcoverClient(client, "Bearer fixture-token")
	_, err := lists.FetchList(context.Background(), map[string]string{"listId": "1"}, 10)
	var blocked *providerhttp.NotSentError
	if !errors.As(err, &blocked) || calls != 1 {
		t.Fatal(err, calls)
	}
	if _, err := s.CheckProvider(context.Background(), "Hardcover"); err != nil {
		t.Fatal(err)
	}
	cached := s.SearchDetailed(context.Background(), q)
	if len(cached.Results) != 1 || len(cached.ProviderErrors) != 0 || calls != 1 {
		t.Fatal("quota-aware check discarded usable cached search", cached, calls)
	}
	failed := s.SearchDetailed(context.Background(), metadata.Query{Query: "another"})
	if len(failed.ProviderErrors) != 1 || calls != 1 {
		t.Fatal(failed, calls)
	}
	if cached := s.SearchDetailed(context.Background(), q); len(cached.Results) != 1 || calls != 1 {
		t.Fatal("unsent quota wait discarded usable cache", cached, calls)
	}
	after := p.Health(context.Background())
	if !after.LastCheckedAt.Equal(*before.LastCheckedAt) || !after.LastSuccessAt.Equal(*before.LastSuccessAt) {
		t.Fatal("unsent request advanced health", before, after)
	}
	fresh := metadata.NewHardcoverProvider(client, "fixture-token")
	health := fresh.Check(context.Background())
	if health.LastCheckedAt != nil || health.LastSuccessAt != nil || health.Authenticated != nil || health.Reachable != nil || health.RetryAfter == nil || health.Status != "rate_limited" || calls != 1 {
		t.Fatal("shared quota invented a successful probe", health, calls)
	}
}
func TestListQuotaAlsoStopsMetadataBeforeFirstRequest(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: providerhttp.NewTransport(budgetRoundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Fatal("bearer prefix doubled")
		}
		return budgetResponse(`{"data":{"lists_by_pk":{"id":1,"books_count":0,"updated_at":"2026-09-16T00:00:00Z","list_books":[]}}}`), nil
	}))}
	lists := importlists.NewHardcoverClient(client, "Bearer fixture-token")
	if rows, err := lists.FetchList(context.Background(), map[string]string{"listId": "1"}, 10); err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	p := metadata.NewHardcoverProvider(client, "fixture-token")
	if _, err := p.Search(context.Background(), metadata.Query{Query: "Dune"}); err == nil {
		t.Fatal("metadata bypassed shared daily limit")
	}
	health := p.Health(context.Background())
	if calls != 1 || health.LastCheckedAt != nil || health.RetryAfter == nil {
		t.Fatal(calls, health)
	}
}
