package importlists

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fixtureRoundTrip func(*http.Request) (*http.Response, error)

func (f fixtureRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestHardcoverListErrorsDoNotExposeProviderBodiesOrRequestURLs(t *testing.T) {
	for _, body := range []string{`{"errors":[{"message":"fixture-secret"}]}`, `{}`, `{"data":{"list_books":null}}`} {
		client := NewHardcoverClient(&http.Client{Transport: fixtureRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}, "fixture-token")
		if client.client.Timeout != 15*time.Second {
			t.Fatal(client.client.Timeout)
		}
		if _, err := client.FetchList(context.Background(), map[string]string{"listId": "1"}, 10); err == nil || strings.Contains(err.Error(), "fixture-secret") {
			t.Fatal(body, err)
		}
	}
	client := NewHardcoverClient(&http.Client{Transport: fixtureRoundTrip(func(req *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "post", URL: "https://example.test/fixture-secret", Err: errors.New("fixture-secret")}
	})}, "fixture-token")
	if _, err := client.FetchList(context.Background(), map[string]string{"listId": "1"}, 10); err == nil || strings.Contains(err.Error(), "fixture-secret") || strings.Contains(err.Error(), "https:") {
		t.Fatal(err)
	}
}
