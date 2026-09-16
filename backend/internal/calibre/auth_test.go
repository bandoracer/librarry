package calibre

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthenticationProbeNeverUploadsOrForwardsCredentials(t *testing.T) {
	for _, scenario := range []string{"basic", "probe redirect", "write redirect", "unsupported", "no challenge"} {
		t.Run(scenario, func(t *testing.T) {
			forwarded := 0
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded++; w.WriteHeader(200) }))
			defer target.Close()
			probes, writes := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/ajax/library-info" {
					probes++
					if r.Method != "GET" || r.Header.Get("Authorization") != "" || r.ContentLength > 0 {
						t.Error("authentication probe carried credentials or a body")
					}
					switch scenario {
					case "probe redirect":
						http.Redirect(w, r, target.URL, 307)
					case "unsupported":
						w.Header().Set("WWW-Authenticate", "Bearer fixture")
						w.WriteHeader(401)
					case "no challenge":
						w.WriteHeader(200)
					default:
						w.Header().Set("WWW-Authenticate", `Basic realm="fixture"`)
						w.WriteHeader(401)
					}
					return
				}
				writes++
				u, p, ok := r.BasicAuth()
				if !ok || u != "fixture" || p != "password" {
					t.Error("missing authenticated write")
				}
				if scenario == "write redirect" {
					http.Redirect(w, r, target.URL, 307)
					return
				}
				_, _ = w.Write([]byte(`{"id":"999","book_id":7}`))
			}))
			defer server.Close()
			book := filepath.Join(t.TempDir(), "book.epub")
			if err := os.WriteFile(book, []byte("fixture"), 0644); err != nil {
				t.Fatal(err)
			}
			result, err := NewClient(server.Client()).AddBook(context.Background(), AddBookRequest{Path: book, Settings: Settings{Host: server.URL, Username: "fixture", Password: "password"}})
			if scenario == "basic" {
				if err != nil || result.ID != 7 {
					t.Fatal(result, err)
				}
			} else if err == nil {
				t.Fatal("unsafe authentication response accepted")
			}
			expectedWrites := 0
			if scenario == "basic" || scenario == "write redirect" {
				expectedWrites = 1
			}
			if probes != 1 || writes != expectedWrites || forwarded != 0 {
				t.Fatal(probes, writes, forwarded)
			}
			if err != nil && (strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), target.URL)) {
				t.Fatal("error exposed credentials or redirect URL")
			}
		})
	}
}
