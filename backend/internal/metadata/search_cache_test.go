package metadata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type cacheFixtureProvider struct {
	staticMetadataProvider
	calls atomic.Int32
	fetch func(Context, Query) ([]SearchResult, error)
}

func (p *cacheFixtureProvider) Search(ctx Context, q Query) ([]SearchResult, error) {
	p.calls.Add(1)
	if p.fetch != nil {
		return p.fetch(ctx, q)
	}
	return p.results, nil
}
func cacheResult() SearchResult {
	r := bookFixture("Open Library", "Dune", "ebook", "en", "9780441172719")
	r.Work.ProviderIDs = []string{"openlibrary:work"}
	r.Work.Authors[0].ProviderIDs = []string{"openlibrary:author"}
	r.Edition.ProviderIDs = []string{"openlibrary:edition"}
	r.MatchedOn = []string{"exact identifier"}
	return r
}
func TestSearchCacheIsolatesNestedResultsAndAllQueryDimensions(t *testing.T) {
	p := &cacheFixtureProvider{staticMetadataProvider: staticMetadataProvider{name: "Open Library", results: []SearchResult{cacheResult()}}}
	s := NewService([]Provider{p})
	q := Query{Query: "Dune", Type: SearchTypeBook, Format: FormatEbook, PreferredLanguage: "English", Limit: 10}
	first := s.SearchDetailed(context.Background(), q)
	first.Results[0].Work.Title = "corrupted"
	first.Results[0].Work.Authors[0].Name = "corrupted"
	first.Results[0].Work.Authors[0].ProviderIDs[0] = "corrupted"
	first.Results[0].Work.ProviderIDs[0] = "corrupted"
	first.Results[0].Edition.ISBNs[0] = "corrupted"
	first.Results[0].Edition.ProviderIDs[0] = "corrupted"
	first.Results[0].MatchedOn[0] = "corrupted"
	for range 2 {
		cached := s.SearchDetailed(context.Background(), q)
		r := cached.Results[0]
		if r.Work.Title != "Dune" || r.Work.Authors[0].Name != "Fixture Author" || r.Work.Authors[0].ProviderIDs[0] != "openlibrary:author" || r.Work.ProviderIDs[0] != "openlibrary:work" || r.Edition.ISBNs[0] != "9780441172719" || r.Edition.ProviderIDs[0] != "openlibrary:edition" || r.MatchedOn[0] != "exact identifier" {
			t.Fatal(r)
		}
		r.Edition.ISBNs[0] = "corrupted cached response"
	}
	if p.calls.Load() != 1 {
		t.Fatal(p.calls.Load())
	}
	for _, change := range []func(*Query){
		func(q *Query) { q.Query = "Other" },
		func(q *Query) { q.Type = SearchTypeAuthor },
		func(q *Query) { q.Format = FormatAudiobook },
		func(q *Query) { q.PreferredLanguage = "French" },
		func(q *Query) { q.Limit = 20 },
		func(q *Query) { q.ProviderKey = "openlibrary:another-author" },
	} {
		other := q
		change(&other)
		s.SearchDetailed(context.Background(), other)
	}
	if p.calls.Load() != 7 {
		t.Fatal("cache key lost a query dimension", p.calls.Load())
	}
	otherService := NewService([]Provider{p})
	otherService.SearchDetailed(context.Background(), q)
	if p.calls.Load() != 8 {
		t.Fatal("cache escaped service configuration", p.calls.Load())
	}
}
func TestSearchCacheExpiresPositiveAndEmptyResultsWithoutSlidingTTL(t *testing.T) {
	for _, empty := range []bool{false, true} {
		c := newProviderSearchCache()
		now := time.Now()
		c.clock = func() time.Time { return now }
		rows := []SearchResult{cacheResult()}
		ttl := searchCacheTTL
		if empty {
			rows = []SearchResult{}
			ttl = emptySearchCacheTTL
		}
		p := &cacheFixtureProvider{staticMetadataProvider: staticMetadataProvider{results: rows}}
		q := Query{Query: "Dune"}
		c.search(context.Background(), q, p)
		now = now.Add(ttl - time.Second)
		c.search(context.Background(), q, p)
		if p.calls.Load() != 1 {
			t.Fatal(empty, p.calls.Load())
		}
		now = now.Add(time.Second)
		c.search(context.Background(), q, p)
		if p.calls.Load() != 2 {
			t.Fatal("TTL extended by read", empty, p.calls.Load())
		}
	}
}
func TestSearchCacheErrorsInvalidateOtherQueriesAndAreNotCached(t *testing.T) {
	c := newProviderSearchCache()
	fail := false
	p := &cacheFixtureProvider{fetch: func(Context, Query) ([]SearchResult, error) {
		if fail {
			return nil, errors.New("fixture failure")
		}
		return []SearchResult{cacheResult()}, nil
	}}
	q := Query{Query: "Dune"}
	c.search(context.Background(), q, p)
	fail = true
	for range 2 {
		if _, err := c.search(context.Background(), Query{Query: "Other"}, p); err == nil {
			t.Fatal("hidden error")
		}
	}
	if _, ok := c.get(q); ok {
		t.Fatal("previous success survived provider failure")
	}
	fail = false
	if _, err := c.search(context.Background(), q, p); err != nil {
		t.Fatal(err)
	}
	if p.calls.Load() != 4 {
		t.Fatal(p.calls.Load())
	}
}
func TestSearchCacheCapacityEvictsLeastRecentlyUsedAndOversizeIsNotStored(t *testing.T) {
	c := newProviderSearchCache()
	for i := range searchCacheMaxEntries {
		c.put(Query{Query: fmt.Sprint(i)}, []SearchResult{cacheResult()}, 0)
	}
	c.get(Query{Query: "0"})
	c.put(Query{Query: "new"}, []SearchResult{cacheResult()}, 0)
	if _, ok := c.get(Query{Query: "1"}); ok {
		t.Fatal("oldest entry retained")
	}
	if _, ok := c.get(Query{Query: "0"}); !ok {
		t.Fatal("recently used entry evicted")
	}
	if len(c.entries) != searchCacheMaxEntries {
		t.Fatal(len(c.entries))
	}
	large := cacheResult()
	large.Work.Description = strings.Repeat("x", 100<<10)
	for i := range 50 {
		c.put(Query{Query: fmt.Sprintf("large%d", i)}, []SearchResult{large}, 0)
	}
	if c.bytes > searchCacheMaxBytes || len(c.entries) >= 50 {
		t.Fatal("byte bound exceeded", c.bytes, len(c.entries))
	}
	sum := 0
	for _, e := range c.entries {
		sum += e.bytes
	}
	if sum != c.bytes {
		t.Fatal("incorrect byte accounting", sum, c.bytes)
	}
	large.Work.Description = strings.Repeat("x", searchCacheMaxEntryBytes)
	p := &cacheFixtureProvider{staticMetadataProvider: staticMetadataProvider{results: []SearchResult{large}}}
	for range 2 {
		rows, err := c.search(context.Background(), Query{Query: "oversize"}, p)
		if err != nil || len(rows) != 1 || len(rows[0].Work.Description) != searchCacheMaxEntryBytes {
			t.Fatal("oversize response lost", err)
		}
	}
	if p.calls.Load() != 2 {
		t.Fatal("oversize response cached", p.calls.Load())
	}
	c.put(Query{Query: strings.Repeat("q", searchCacheMaxKeyBytes)}, []SearchResult{cacheResult()}, 0)
	for key := range c.entries {
		if len(key.Query) >= searchCacheMaxKeyBytes {
			t.Fatal("oversize key cached")
		}
	}
}
func TestSearchCacheConcurrentRequestsShareFetchAndCanceledWaiterExits(t *testing.T) {
	c := newProviderSearchCache()
	started, release := make(chan struct{}), make(chan struct{})
	p := &cacheFixtureProvider{fetch: func(Context, Query) ([]SearchResult, error) {
		close(started)
		<-release
		return []SearchResult{cacheResult()}, nil
	}}
	q := Query{Query: "Dune"}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := c.search(context.Background(), q, p)
			if err != nil || len(rows) != 1 {
				t.Error(rows, err)
			}
		}()
	}
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	waitDone := make(chan error, 1)
	go func() { _, err := c.search(ctx, q, p); waitDone <- err }()
	cancel()
	if err := <-waitDone; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	wg.Wait()
	if p.calls.Load() != 1 {
		t.Fatal("parallel searches spent repeated quota", p.calls.Load())
	}
}
func TestSearchCacheCanceledFetchDoesNotClearOtherSuccess(t *testing.T) {
	c := newProviderSearchCache()
	q := Query{Query: "Dune"}
	c.put(q, []SearchResult{cacheResult()}, 0)
	started := make(chan struct{})
	p := &cacheFixtureProvider{fetch: func(ctx Context, _ Query) ([]SearchResult, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := c.search(ctx, Query{Query: "Other"}, p); done <- err }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, ok := c.get(q); !ok {
		t.Fatal("cancellation cleared valid cached results")
	}
	if _, err := c.search(ctx, q, p); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled caller received cached success", err)
	}
}
func TestSearchCacheInvalidationFencesInFlightSuccess(t *testing.T) {
	c := newProviderSearchCache()
	started, release := make(chan struct{}), make(chan struct{})
	p := &cacheFixtureProvider{fetch: func(Context, Query) ([]SearchResult, error) {
		close(started)
		<-release
		return []SearchResult{cacheResult()}, nil
	}}
	q := Query{Query: "Dune"}
	done := make(chan struct{})
	go func() { defer close(done); c.search(context.Background(), q, p) }()
	<-started
	c.invalidate()
	close(release)
	<-done
	if _, ok := c.get(q); ok {
		t.Fatal("stale fetch repopulated invalidated generation")
	}
}
func TestSearchCacheKeepsHealthTimesAndExplicitFailuresEvict(t *testing.T) {
	var calls atomic.Int32
	var unauthorized atomic.Bool
	p := NewGoogleBooksProvider(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		if unauthorized.Load() {
			r := jsonResponse(`{}`)
			r.StatusCode = http.StatusUnauthorized
			return r, nil
		}
		return jsonResponse(`{"totalItems":1,"items":[{"id":"fixture","volumeInfo":{"title":"Dune"}}]}`), nil
	})}, "fixture-key")
	s := NewService([]Provider{p})
	q := Query{Query: "Dune"}
	s.SearchDetailed(context.Background(), q)
	before := p.Health(context.Background())
	s.SearchDetailed(context.Background(), q)
	after := p.Health(context.Background())
	if calls.Load() != 1 || before.LastCheckedAt == nil || !before.LastCheckedAt.Equal(*after.LastCheckedAt) || !before.LastSuccessAt.Equal(*after.LastSuccessAt) {
		t.Fatal(calls.Load(), before, after)
	}
	unauthorized.Store(true)
	expireCheck(p.observation)
	h, err := s.CheckProvider(context.Background(), "Google Books")
	if err != nil || h.Status != "invalid_credentials" {
		t.Fatal(h, err)
	}
	outcome := s.SearchDetailed(context.Background(), q)
	if len(outcome.Results) != 0 || len(outcome.ProviderErrors) != 1 || calls.Load() != 3 {
		t.Fatal("health failure hidden by cache", outcome, calls.Load())
	}
}

func TestSearchCacheFailureIsScopedToProviderAndConfigurationIsImmutable(t *testing.T) {
	var fail atomic.Bool
	primary := &cacheFixtureProvider{staticMetadataProvider: staticMetadataProvider{name: "Hardcover"}, fetch: func(_ Context, q Query) ([]SearchResult, error) {
		if fail.Load() {
			return nil, errors.New("fixture outage")
		}
		r := cacheResult()
		r.Provider = "Hardcover"
		r.Work.Title = q.Query
		r.Edition.Title = q.Query
		return []SearchResult{r}, nil
	}}
	backbone := &cacheFixtureProvider{staticMetadataProvider: staticMetadataProvider{name: "Open Library"}, fetch: func(_ Context, q Query) ([]SearchResult, error) {
		r := cacheResult()
		r.Work.Title = q.Query
		r.Edition.Title = q.Query
		return []SearchResult{r}, nil
	}}
	providers := []Provider{primary, backbone}
	s := NewService(providers)
	providers[0] = backbone
	q := Query{Query: "Dune"}
	s.SearchDetailed(context.Background(), q)
	fail.Store(true)
	s.SearchDetailed(context.Background(), Query{Query: "Other"})
	result := s.SearchDetailed(context.Background(), q)
	if primary.calls.Load() != 3 || backbone.calls.Load() != 2 || len(result.ProviderErrors) != 1 || result.ProviderErrors[0].Provider != "Hardcover" || len(result.Results) != 1 || result.Results[0].Provider != "Open Library" {
		t.Fatal(primary.calls.Load(), backbone.calls.Load(), result)
	}
}
