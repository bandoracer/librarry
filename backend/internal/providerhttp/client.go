// Package providerhttp coordinates the application's metadata request budget.
package providerhttp

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NotSentError means the transport refused before making a network request.
// Its text is deliberately independent of URLs, headers and provider bodies.
type NotSentError struct {
	RetryAt *time.Time
	Cause   error
}

func (e *NotSentError) Error() string {
	if e.RetryAt != nil {
		return "Provider quota is exhausted; retry after the displayed time."
	}
	return "Provider request was canceled while waiting for its request budget."
}
func (e *NotSentError) Unwrap() error { return e.Cause }

type budget struct {
	gate  chan struct{}
	next  time.Time
	retry time.Time
}
type Transport struct {
	Base     http.RoundTripper
	mu       sync.Mutex
	budgets  map[string]*budget
	interval time.Duration
	now      func() time.Time
	wait     func(context.Context, time.Duration) error
}

func NewClient(timeout time.Duration) *http.Client {
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 12 * time.Second
	}
	return &http.Client{Timeout: timeout, Transport: NewTransport(http.DefaultTransport)}
}
func NewTransport(base http.RoundTripper) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &Transport{Base: base, budgets: map[string]*budget{}, interval: time.Second, now: time.Now, wait: waitFor}
}
func waitFor(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := strings.ToLower(req.URL.Hostname())
	switch host {
	case "openlibrary.org", "api.hardcover.app", "www.googleapis.com":
	default:
		return t.Base.RoundTrip(req)
	}
	if err := req.Context().Err(); err != nil {
		return nil, &NotSentError{Cause: err}
	}
	t.mu.Lock()
	b := t.budgets[host]
	if b == nil {
		b = &budget{gate: make(chan struct{}, 1)}
		t.budgets[host] = b
	}
	t.mu.Unlock()
	select {
	case b.gate <- struct{}{}:
	case <-req.Context().Done():
		return nil, &NotSentError{Cause: req.Context().Err()}
	}
	defer func() { <-b.gate }()
	t.mu.Lock()
	now := t.now()
	retry := b.retry
	delay := b.next.Sub(now)
	t.mu.Unlock()
	if retry.After(now) {
		return nil, &NotSentError{RetryAt: &retry}
	}
	if delay > 0 {
		if err := t.wait(req.Context(), delay); err != nil {
			return nil, &NotSentError{Cause: err}
		}
	}
	if err := req.Context().Err(); err != nil {
		return nil, &NotSentError{Cause: err}
	}
	t.mu.Lock()
	b.next = t.now().Add(t.interval)
	t.mu.Unlock()
	response, err := t.Base.RoundTrip(req)
	if err != nil {
		return response, err
	}
	t.mu.Lock()
	until := quotaBackoff(response, t.now())
	if until.After(b.retry) {
		b.retry = until
	}
	t.mu.Unlock()
	return response, nil
}

func RetryAfter(client *http.Client, host string) *time.Time {
	if client == nil {
		return nil
	}
	t, ok := client.Transport.(*Transport)
	if !ok {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	b := t.budgets[host]
	if b == nil || !b.retry.After(t.now()) {
		return nil
	}
	at := b.retry
	return &at
}
func boundedRetry(at, now time.Time) time.Time {
	if !at.After(now) {
		return time.Time{}
	}
	if at.After(now.Add(24 * time.Hour)) {
		return now.Add(24 * time.Hour)
	}
	return at
}
func quotaBackoff(resp *http.Response, now time.Time) time.Time {
	var until time.Time
	take := func(at time.Time) {
		at = boundedRetry(at, now)
		if at.After(until) {
			until = at
		}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		retry := now.Add(time.Minute)
		if seconds, err := strconv.ParseInt(strings.TrimSpace(resp.Header.Get("Retry-After")), 10, 64); err == nil && seconds >= 0 {
			retry = now.Add(time.Duration(min(seconds, 86400)) * time.Second)
		} else if at, err := http.ParseTime(resp.Header.Get("Retry-After")); err == nil {
			retry = at
		}
		take(retry)
	}
	// Hardcover documents named structured buckets: "Free";r=8;t=42,
	// "daily";r=4231;t=51234. Do not expose names or raw header text.
	structured := false
	for _, entry := range strings.Split(strings.Join(resp.Header.Values("RateLimit"), ","), ",") {
		var remaining, seconds int64
		haveR, haveT := false, false
		for _, param := range strings.Split(entry, ";")[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
			if !ok {
				continue
			}
			n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil || n < 0 {
				continue
			}
			switch key {
			case "r":
				remaining = n
				haveR = true
			case "t":
				seconds = n
				haveT = true
			}
		}
		if haveR && haveT {
			structured = true
			if remaining == 0 {
				take(now.Add(time.Duration(min(seconds, 86400)) * time.Second))
			}
		}
	}
	if !structured {
		for _, prefix := range []string{"X-RateLimit-", "X-RateLimit-Daily-"} {
			remaining, err := strconv.ParseInt(strings.TrimSpace(resp.Header.Get(prefix+"Remaining")), 10, 64)
			if err != nil || remaining != 0 {
				continue
			}
			unix, err := strconv.ParseInt(strings.TrimSpace(resp.Header.Get(prefix+"Reset")), 10, 64)
			if err == nil {
				take(time.Unix(min(unix, now.Unix()+86400), 0))
			}
		}
	}
	return until
}
