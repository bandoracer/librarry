package metadata

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bandoracer/librarry/backend/internal/providerhttp"
	"sync"
	"time"
)

const (
	searchCacheTTL           = 5 * time.Minute
	emptySearchCacheTTL      = 30 * time.Second
	searchCacheMaxEntries    = 128
	searchCacheMaxBytes      = 2 << 20
	searchCacheMaxEntryBytes = 256 << 10
	searchCacheMaxKeyBytes   = 4096
)

type cachedSearch struct {
	data    []byte
	expires time.Time
	used    uint64
	bytes   int
}

// Caches belong to one immutable service/provider configuration. Stored JSON
// prevents merge/caller mutations from contaminating a later response. Success
// TTLs are fixed from the fetch, not extended by reads; errors are never cached.
type providerSearchCache struct {
	mu         sync.Mutex
	gate       chan struct{}
	entries    map[Query]cachedSearch
	bytes      int
	clock      func() time.Time
	sequence   uint64
	generation uint64
}

func newProviderSearchCache() *providerSearchCache {
	return &providerSearchCache{gate: make(chan struct{}, 1), entries: make(map[Query]cachedSearch), clock: time.Now}
}

func (c *providerSearchCache) search(ctx context.Context, query Query, provider Provider) ([]SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if results, ok := c.get(query); ok {
		return results, nil
	}
	// Recheck after entering the provider slot: simultaneous identical requests
	// share the successful fetch without detached work or unbounded waits.
	waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if results, ok := c.get(query); ok {
		return results, nil
	}
	c.mu.Lock()
	generation := c.generation
	c.mu.Unlock()
	results, err := provider.Search(ctx, query)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		// A failed request invalidates other cached queries for this provider;
		// a known credential/outage failure cannot be hidden behind older success.
		// Quota backoff and unsent waits keep unexpired successful data usable.
		var notSent *providerhttp.NotSentError
		var failure *providerFailure
		quotaLimited := errors.As(err, &failure) && failure.status == "rate_limited"
		if !errors.As(err, &notSent) && !quotaLimited {
			c.invalidate()
		}
		return results, err
	}
	c.put(query, results, generation)
	return results, nil
}

func (c *providerSearchCache) get(query Query) ([]SearchResult, bool) {
	c.mu.Lock()
	entry, ok := c.entries[query]
	if !ok {
		c.mu.Unlock()
		return nil, false
	}
	if !c.clock().Before(entry.expires) {
		c.remove(query)
		c.mu.Unlock()
		return nil, false
	}
	c.sequence++
	entry.used = c.sequence
	c.entries[query] = entry
	c.mu.Unlock()
	var results []SearchResult
	if json.Unmarshal(entry.data, &results) != nil {
		return nil, false
	}
	return results, true
}

func (c *providerSearchCache) put(query Query, results []SearchResult, generation uint64) {
	keyBytes := len(query.Query) + len(query.Type) + len(query.Format) + len(query.PreferredLanguage) + len(query.ProviderKey) + 8
	if keyBytes > searchCacheMaxKeyBytes {
		return
	}
	data, err := json.Marshal(results)
	if err != nil || len(data) > searchCacheMaxEntryBytes {
		return
	}
	size := len(data) + keyBytes
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation != generation {
		return // an explicit health check invalidated this in-flight generation
	}
	now := c.clock()
	for key, entry := range c.entries {
		if !now.Before(entry.expires) {
			c.remove(key)
		}
	}
	c.remove(query)
	for len(c.entries) >= searchCacheMaxEntries || c.bytes+size > searchCacheMaxBytes {
		var oldest Query
		var age uint64 = ^uint64(0)
		for key, entry := range c.entries {
			if entry.used < age {
				oldest, age = key, entry.used
			}
		}
		c.remove(oldest)
	}
	ttl := searchCacheTTL
	if len(results) == 0 {
		ttl = emptySearchCacheTTL
	}
	c.sequence++
	c.entries[query] = cachedSearch{data: data, expires: now.Add(ttl), used: c.sequence, bytes: size}
	c.bytes += size
}

// Caller holds mu.
func (c *providerSearchCache) remove(key Query) {
	if entry, ok := c.entries[key]; ok {
		c.bytes -= entry.bytes
		delete(c.entries, key)
	}
}

func (c *providerSearchCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[Query]cachedSearch)
	c.bytes = 0
	c.generation++
}
