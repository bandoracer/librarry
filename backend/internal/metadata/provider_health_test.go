package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func expireCheck(p *providerObservation) {
	p.mu.Lock()
	defer p.mu.Unlock()
	at := time.Now().Add(-time.Minute)
	p.latest.LastCheckedAt = &at
}
func TestProviderHealthDistinguishesConfigurationFromObservedAuthentication(t *testing.T) {
	var calls atomic.Int32
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		if req.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("bearer token normalization failed")
		}
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || !strings.Contains(body.Query, "me { id }") {
			t.Error(body, err)
		}
		return jsonResponse(`{"data":{"me":[{"id":123}]}}`), nil
	})}, "Bearer fixture-token")
	for range 3 {
		h := p.Health(context.Background())
		if h.Status != "configured" || h.Authenticated != nil || h.LastCheckedAt != nil || h.LastSuccessAt != nil {
			t.Fatal(h)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("health snapshot spent quota")
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h := p.Check(context.Background())
			if h.Status != "ready" || h.Authenticated == nil || !*h.Authenticated || h.LastSuccessAt == nil || h.LastCheckedAt == nil {
				t.Error(h)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("concurrent checks were not coalesced", calls.Load())
	}
	before := p.Health(context.Background()).LastCheckedAt
	h := p.Health(context.Background())
	if !h.LastCheckedAt.Equal(*before) {
		t.Fatal("snapshot advanced observation")
	}
}
func TestProviderHealthRetainsSuccessButInvalidatesBadCredentials(t *testing.T) {
	response := `{"data":{"me":[{"id":123}]}}`
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(response), nil })}, "fixture")
	first := p.Check(context.Background())
	expireCheck(p.observation)
	response = `{"errors":[{"message":"private server detail","extensions":{"code":"invalid-jwt"}}]}`
	h := p.Check(context.Background())
	if h.Status != "invalid_credentials" || h.Authenticated == nil || *h.Authenticated || h.Reachable == nil || !*h.Reachable || h.LastSuccessAt == nil || !h.LastSuccessAt.Equal(*first.LastSuccessAt) {
		t.Fatal(h)
	}
	if strings.Contains(h.Message, "private") {
		t.Fatal("provider error body exposed", h)
	}
}
func TestProviderFailuresAreBoundedAndDoNotLeakRequestCredentials(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		want   string
	}{{"unauthorized", 401, "invalid_credentials"}, {"forbidden", 403, "forbidden"}, {"throttle", 429, "rate_limited"}, {"outage", 503, "unavailable"}} {
		t.Run(tc.name, func(t *testing.T) {
			var calls int
			p := NewGoogleBooksProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				r := jsonResponse(`{"error":"private fixture-secret"}`)
				r.StatusCode = tc.status
				r.Header.Set("Retry-After", "120")
				return r, nil
			})}, "fixture-secret")
			_, err := p.Search(context.Background(), Query{Query: "isbn:9780142437247"})
			if err == nil || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatal(err)
			}
			h := p.Health(context.Background())
			if h.Status != tc.want || h.LastCheckedAt == nil || h.LastSuccessAt != nil {
				t.Fatal(h)
			}
			if tc.status == 429 {
				if h.RetryAfter == nil {
					t.Fatal(h)
				}
				_, _ = p.Search(context.Background(), Query{Query: "another"})
				if calls != 1 {
					t.Fatal("backoff bypassed", calls)
				}
			}
		})
	}
	p := NewGoogleBooksProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: fmt.Errorf("upstream fixture-secret")}
	})}, "fixture-secret")
	_, err := p.Search(context.Background(), Query{Query: "Walden"})
	if err == nil || strings.Contains(err.Error(), "fixture-secret") || strings.Contains(err.Error(), "googleapis") {
		t.Fatal("request URL exposed", err)
	}
	if h := p.Health(context.Background()); h.Status != "unavailable" || h.Reachable == nil || *h.Reachable {
		t.Fatal(h)
	}
}
func TestProviderMalformedResponsesAreNotHealthyEmptyResults(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"totalItems":1}`, `{"error":{"message":"bad"}}`} {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(body), nil })}
		providers := []Provider{NewHardcoverProvider(client, "fixture"), NewGoogleBooksProvider(client, "fixture"), NewOpenLibraryProvider(client)}
		for _, p := range providers {
			if _, err := p.Search(context.Background(), Query{Query: "Fixture"}); err == nil {
				t.Errorf("%s accepted %s", p.Name(), body)
			}
			if h := p.Health(context.Background()); h.Status != "degraded" {
				t.Fatal(h)
			}
		}
	}
	for _, body := range []string{`{"data":{"me":[]}}`, `{"data":{"me":[{"id":0}]}}`, `{"data":{"me":[{"id":""}]}}`, `{"data":{"me":[{"id":null}]}}`} {
		p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(body), nil })}, "fixture")
		if h := p.Check(context.Background()); h.Status != "degraded" || h.Authenticated != nil {
			t.Fatal(h)
		}
	}
}
func TestProviderCancellationAndUnsupportedSearchDoNotInventObservations(t *testing.T) {
	p := NewHardcoverProvider(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}, "fixture")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Search(ctx, Query{Query: "Book"})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	h := p.Health(context.Background())
	if h.Status != "configured" || h.LastCheckedAt != nil {
		t.Fatal(h)
	}
	missing := NewHardcoverProvider(nil, "")
	h = missing.Check(context.Background())
	if h.Status != "missing_credentials" || h.LastCheckedAt != nil {
		t.Fatal(h)
	}
}
func TestOpenLibrarySuccessfulEmptyLookupIsReachableWithoutAuthentication(t *testing.T) {
	p := NewOpenLibraryProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(`{"docs":[]}`), nil })})
	h := p.Check(context.Background())
	if h.Status != "ready" || h.Authenticated != nil || h.Reachable == nil || !*h.Reachable {
		t.Fatal(h)
	}
}
