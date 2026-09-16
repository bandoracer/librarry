package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type providerFailure struct {
	status    string
	message   string
	reachable bool
	retryAt   *time.Time
}

func (e *providerFailure) Error() string { return e.message }

// Provider observations describe actual requests. Merely rendering System must
// never perform network IO, manufacture authentication, or advance these times.
type providerObservation struct {
	mu              sync.Mutex
	slot            chan struct{}
	latest          ProviderHealth
	needsCredential bool
}

func newProviderObservation(name string, credentials bool) *providerObservation {
	return &providerObservation{slot: make(chan struct{}, 1), latest: ProviderHealth{Name: name}, needsCredential: credentials}
}
func (p *providerObservation) health(configured bool, message string) ProviderHealth {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := p.latest
	h.Configured = configured
	h.CheckedAt = time.Now().UTC() // legacy snapshot timestamp, not evidence of IO
	if !configured {
		h.Status = "missing_credentials"
		h.Message = message
	} else if h.Status == "" {
		h.Status = "configured"
		h.Message = message
	}
	return h
}

// begin bounds concurrency per provider; checks reuse observations for 15s and
// all requests respect a received Retry-After. Caller cancellation is not an
// outage observation. Returned errors contain no request URLs or provider bodies.
func (p *providerObservation) begin(ctx Context, check bool) (func(error) error, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	waitCtx, cancel := context.WithTimeout(asContext(ctx), 15*time.Second)
	defer cancel()
	select {
	case p.slot <- struct{}{}:
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	}
	p.mu.Lock()
	now := time.Now().UTC()
	if check && p.latest.LastCheckedAt != nil && now.Sub(*p.latest.LastCheckedAt) < 15*time.Second {
		p.mu.Unlock()
		<-p.slot
		return nil, nil
	}
	if p.latest.RetryAfter != nil && p.latest.RetryAfter.After(now) {
		retry := *p.latest.RetryAfter
		p.mu.Unlock()
		<-p.slot
		return nil, &providerFailure{status: "rate_limited", message: "Provider rate limit is active; retry after the displayed time.", reachable: true, retryAt: &retry}
	}
	p.mu.Unlock()
	return func(err error) error {
		defer func() { <-p.slot }()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		now := time.Now().UTC()
		p.latest.LastCheckedAt = &now
		p.latest.RetryAfter = nil
		reachable := true
		p.latest.Reachable = &reachable
		p.latest.Authenticated = nil
		if err == nil {
			p.latest.Status = "ready"
			p.latest.Message = "Last request succeeded."
			p.latest.LastSuccessAt = &now
			if p.needsCredential {
				authenticated := true
				p.latest.Authenticated = &authenticated
				p.latest.Message = "Credentials accepted by the last successful request."
			}
			return nil
		}
		failure := &providerFailure{status: "degraded", message: "Provider response could not be read or did not match the expected API shape.", reachable: true}
		var known *providerFailure
		if errors.As(err, &known) {
			failure = known
		}
		p.latest.Status = failure.status
		p.latest.Message = failure.message
		p.latest.Reachable = &failure.reachable
		p.latest.RetryAfter = failure.retryAt
		if failure.status == "invalid_credentials" {
			authenticated := false
			p.latest.Authenticated = &authenticated
		}
		return failure
	}, nil
}
func providerRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, &providerFailure{status: "unavailable", message: "Provider request failed. Check the connection and try again.", reachable: false}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	failure := &providerFailure{status: "degraded", message: fmt.Sprintf("Provider returned HTTP %d.", resp.StatusCode), reachable: true}
	switch {
	case resp.StatusCode == 401:
		failure.status = "invalid_credentials"
		failure.message = "Provider rejected the credentials. Check the configured token or key."
	case resp.StatusCode == 403:
		failure.status = "forbidden"
		failure.message = "Provider denied access. Check API permissions or restrictions."
	case resp.StatusCode == 429:
		failure.status = "rate_limited"
		failure.message = "Provider rate limit reached. Requests will wait until the retry time."
		now := time.Now().UTC()
		retry := now.Add(time.Minute)
		if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds >= 0 {
			retry = now.Add(time.Duration(min(seconds, 86400)) * time.Second)
		} else if value, err := http.ParseTime(resp.Header.Get("Retry-After")); err == nil && value.After(now) {
			retry = value
		}
		if retry.After(now.Add(24 * time.Hour)) {
			retry = now.Add(24 * time.Hour)
		}
		failure.retryAt = &retry
	case resp.StatusCode >= 500:
		failure.status = "unavailable"
		failure.message = "Provider is temporarily unavailable. Try again later."
	}
	return nil, failure
}
func boundedProviderClient(client *http.Client) *http.Client {
	if client == nil {
		return &http.Client{Timeout: 10 * time.Second}
	}
	clone := *client
	if clone.Timeout <= 0 || clone.Timeout > 30*time.Second {
		clone.Timeout = 10 * time.Second
	}
	return &clone
}

func (p *HardcoverProvider) Check(ctx Context) ProviderHealth {
	if p.token == "" {
		return p.Health(ctx)
	}
	finish, err := p.observation.begin(ctx, true)
	if err != nil || finish == nil {
		return p.Health(ctx)
	}
	payload := []byte(`{"query":"query LibrarryConnectionCheck { me { id } }"}`)
	req, err := http.NewRequestWithContext(asContext(ctx), http.MethodPost, "https://api.hardcover.app/v1/graphql", bytes.NewReader(payload))
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+p.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "librarry/0.4")
		var resp *http.Response
		resp, err = providerRequest(p.client, req)
		if err == nil {
			defer resp.Body.Close()
			var result struct {
				Data struct {
					Me []struct {
						ID json.RawMessage `json:"id"`
					} `json:"me"`
				} `json:"data"`
				Errors []hardcoverGraphQLError `json:"errors"`
			}
			err = json.NewDecoder(resp.Body).Decode(&result)
			if err == nil {
				err = hardcoverErrors(result.Errors)
			}
			if err == nil && (len(result.Data.Me) != 1 || !validHardcoverUserID(result.Data.Me[0].ID)) {
				err = errors.New("missing authenticated user")
			}
		}
	}
	_ = finish(err)
	return p.Health(ctx)
}
func (p *OpenLibraryProvider) Check(ctx Context) ProviderHealth {
	finish, err := p.observation.begin(ctx, true)
	if err != nil || finish == nil {
		return p.Health(ctx)
	}
	_, err = p.searchBooks(ctx, Query{Query: "9780142437247", Type: SearchTypeBook, Limit: 1})
	_ = finish(err)
	return p.Health(ctx)
}
func (p *GoogleBooksProvider) Check(ctx Context) ProviderHealth {
	if p.apiKey == "" {
		return p.Health(ctx)
	}
	finish, err := p.observation.begin(ctx, true)
	if err != nil || finish == nil {
		return p.Health(ctx)
	}
	_, err = p.searchBooks(ctx, Query{Query: "isbn:9780142437247", Type: SearchTypeBook, Limit: 1})
	_ = finish(err)
	return p.Health(ctx)
}

type hardcoverGraphQLError struct {
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

func hardcoverErrors(failures []hardcoverGraphQLError) error {
	if len(failures) == 0 {
		return nil
	}
	for _, failure := range failures {
		switch strings.ToLower(failure.Extensions.Code) {
		case "invalid-jwt", "jwt-invalid-claims", "jwt-missing-role-claims", "unauthenticated":
			return &providerFailure{status: "invalid_credentials", message: "Hardcover rejected the token. Check the configured token.", reachable: true}
		case "access-denied", "permission-denied":
			return &providerFailure{status: "forbidden", message: "Hardcover denied access to this query.", reachable: true}
		}
	}
	return &providerFailure{status: "degraded", message: "Hardcover returned GraphQL errors. Check API access and try again.", reachable: true}
}

// CheckProvider is explicit, read-only remote verification. Snapshot endpoints
// only call Health and never spend provider quota.
func (s *Service) CheckProvider(ctx context.Context, name string) (ProviderHealth, error) {
	for index, p := range s.providers {
		if !strings.EqualFold(strings.TrimSpace(name), p.Name()) {
			continue
		}
		if checker, ok := p.(interface{ Check(Context) ProviderHealth }); ok {
			health := checker.Check(ctx)
			if health.Status != "ready" && ctx.Err() == nil {
				s.searchCaches[index].invalidate()
			}
			return health, nil
		}
		return p.Health(ctx), nil
	}
	return ProviderHealth{}, errors.New("unknown metadata provider")
}

func validHardcoverUserID(raw json.RawMessage) bool {
	id, err := strconv.ParseInt(strings.Trim(string(raw), "\""), 10, 64)
	return err == nil && id > 0
}
