package providerhttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(code int, headers http.Header) *http.Response {
	return &http.Response{StatusCode: code, Header: headers, Body: io.NopCloser(strings.NewReader(`{}`))}
}
func request(host string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "https://"+host+"/fixture", nil)
	return r
}
func TestQuotaHeaders(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		code    int
		headers http.Header
		wait    time.Duration
	}{
		{"modern daily exhaustion", 200, http.Header{"Ratelimit": {`"Free";r=8;t=42, "daily";r=0;t=200`}}, 200 * time.Second},
		{"burst exhaustion", 200, http.Header{"Ratelimit": {`"Free";r=0;t=3, "daily";r=500;t=200`}}, 3 * time.Second},
		{"multiple fields", 200, http.Header{"Ratelimit": {`"Free";r=0;t=3`, `"daily";r=0;t=200`}}, 200 * time.Second},
		{"modern positive overrides stale legacy", 200, http.Header{"Ratelimit": {`"Free";r=8;t=42`}, "X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1789561000"}}, 0},
		{"legacy daily", 200, http.Header{"X-Ratelimit-Daily-Remaining": {"0"}, "X-Ratelimit-Daily-Reset": {"1789560100"}}, 100 * time.Second},
		{"429 missing delay", 429, http.Header{}, time.Minute},
		{"429 seconds", 429, http.Header{"Retry-After": {"12"}}, 12 * time.Second},
		{"429 date", 429, http.Header{"Retry-After": {now.Add(20 * time.Second).Format(http.TimeFormat)}}, 20 * time.Second},
		{"429 long", 429, http.Header{"Retry-After": {"9999999999999"}}, 24 * time.Hour},
		{"429 negative", 429, http.Header{"Retry-After": {"-3"}}, time.Minute},
		{"daily and 429", 429, http.Header{"Retry-After": {"10"}, "Ratelimit": {`"daily";r=0;t=500`}}, 500 * time.Second},
		{"invalid", 200, http.Header{"Ratelimit": {`"Free";r=oops;t=42`}}, 0},
		{"huge", 200, http.Header{"Ratelimit": {`"daily";r=0;t=9223372036854775807`}}, 24 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := quotaBackoff(response(tc.code, tc.headers), now)
			if tc.wait == 0 {
				if !got.IsZero() {
					t.Fatal(got)
				}
			} else if !got.Equal(now.Add(tc.wait)) {
				t.Fatal(got, now.Add(tc.wait))
			}
		})
	}
}
func TestPacingAndBackoffShareOnlyTheProviderHost(t *testing.T) {
	now := time.Now()
	starts := []time.Time{}
	calls := 0
	quota := false
	transport := NewTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		starts = append(starts, now)
		headers := http.Header{}
		if quota && r.URL.Hostname() == "api.hardcover.app" {
			headers.Set("RateLimit", `"daily";r=0;t=60`)
		}
		return response(200, headers), nil
	}))
	transport.now = func() time.Time { return now }
	transport.wait = func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil }
	client := &http.Client{Transport: transport}
	for range 2 {
		r, err := client.Do(request("api.hardcover.app"))
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
	}
	if starts[1].Sub(starts[0]) != time.Second {
		t.Fatal(starts)
	}
	quota = true
	r, err := client.Do(request("api.hardcover.app"))
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	_, err = client.Do(request("api.hardcover.app"))
	var blocked *NotSentError
	if !errors.As(err, &blocked) || blocked.RetryAt == nil || calls != 3 {
		t.Fatal(err, calls)
	}
	if RetryAfter(client, "api.hardcover.app") == nil {
		t.Fatal("quota not exposed")
	}
	r, err = client.Do(request("openlibrary.org"))
	if err != nil {
		t.Fatal("other provider blocked", err)
	}
	r.Body.Close()
	now = now.Add(time.Minute)
	quota = false
	r, err = client.Do(request("api.hardcover.app"))
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if RetryAfter(client, "api.hardcover.app") != nil {
		t.Fatal("expired quota retained")
	}
}
func TestCanceledPacingWaitDoesNotSendOrConsumeNextSlot(t *testing.T) {
	calls := 0
	transport := NewTransport(roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return response(200, http.Header{}), nil }))
	r, err := transport.RoundTrip(request("openlibrary.org"))
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	before := transport.budgets["openlibrary.org"].next
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = transport.RoundTrip(request("openlibrary.org").WithContext(ctx))
	var notSent *NotSentError
	if !errors.As(err, &notSent) || !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal(err, calls)
	}
	if !transport.budgets["openlibrary.org"].next.Equal(before) {
		t.Fatal("canceled waiter consumed quota")
	}
}
func TestConcurrentClientsSharePacing(t *testing.T) {
	starts := []time.Time{}
	var mu sync.Mutex
	transport := NewTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		return response(200, http.Header{}), nil
	}))
	transport.interval = 15 * time.Millisecond
	clients := []*http.Client{{Transport: transport}, {Transport: transport}}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := clients[i%2].Do(request("api.hardcover.app"))
			if err != nil {
				t.Error(err)
				return
			}
			r.Body.Close()
		}()
	}
	wg.Wait()
	if len(starts) != 8 {
		t.Fatal(len(starts))
	}
	for i := 1; i < len(starts); i++ {
		if starts[i].Sub(starts[i-1]) < 14*time.Millisecond {
			t.Fatal("shared clients burst requests", starts)
		}
	}
}
