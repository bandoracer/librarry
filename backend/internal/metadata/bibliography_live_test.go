package metadata

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/providerhttp"
)

// Explicit read-only qualification, never part of the default credential-free
// suite. Use the same pacing transport as the application.
func TestLiveOpenLibraryBibliography(t *testing.T) {
	if os.Getenv("LIBRARRY_TEST_LIVE_OPEN_LIBRARY") != "1" {
		t.Skip("opt-in live Open Library qualification")
	}
	calls := 0
	starts := []time.Time{}
	transport := providerhttp.NewTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		starts = append(starts, time.Now())
		return http.DefaultTransport.RoundTrip(req)
	}))
	p := NewOpenLibraryProvider(&http.Client{Transport: transport, Timeout: 12 * time.Second})
	rows, err := NewService([]Provider{p}).AuthorBibliography(context.Background(), Query{ProviderKey: "openlibrary:OL23919A", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) <= 100 || calls < 3 {
		t.Fatal("fixture author no longer qualifies multi-page traversal", len(rows), calls)
	}
	for i := 1; i < len(starts); i++ {
		if starts[i].Sub(starts[i-1]) < 990*time.Millisecond {
			t.Fatal("production pacing burst", starts[i].Sub(starts[i-1]))
		}
	}
	t.Logf("Read-only Open Library qualification: %d works across %d requests; identities/counts verified, no local catalog mutations", len(rows), calls)
}
