package metadata

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// Explicit read-only qualification, never part of the default credential-free
// suite. Pace this probe conservatively; it does not qualify production pacing.
func TestLiveOpenLibraryBibliography(t *testing.T) {
	if os.Getenv("LIBRARRY_TEST_LIVE_OPEN_LIBRARY") != "1" {
		t.Skip("opt-in live Open Library qualification")
	}
	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		calls++
		return http.DefaultTransport.RoundTrip(req)
	})
	p := NewOpenLibraryProvider(&http.Client{Transport: transport, Timeout: 12 * time.Second})
	rows, err := NewService([]Provider{p}).AuthorBibliography(context.Background(), Query{ProviderKey: "openlibrary:OL23919A", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) <= 100 || calls < 3 {
		t.Fatal("fixture author no longer qualifies multi-page traversal", len(rows), calls)
	}
	t.Logf("Read-only Open Library qualification: %d works across %d requests; identities/counts verified, no local catalog mutations", len(rows), calls)
}
