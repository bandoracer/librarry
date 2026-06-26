package calibre

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddBookPostsCalibreContentServerPayload(t *testing.T) {
	dir := t.TempDir()
	bookPath := filepath.Join(dir, "Project Hail Mary.epub")
	if err := os.WriteFile(bookPath, []byte("book-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotPath string
	var gotBody string
	var gotAuthUser string
	var gotAuthPass string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotAuthUser, gotAuthPass, _ = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":321}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.jobID = func() int { return 12345 }

	result, err := client.AddBook(context.Background(), AddBookRequest{
		Path: bookPath,
		Settings: Settings{
			Host:     strings.TrimPrefix(server.URL, "http://"),
			URLBase:  "/calibre",
			Username: "reader",
			Password: "secret",
			Library:  "Main Library",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != 321 {
		t.Fatalf("expected calibre id 321, got %d", result.ID)
	}
	if gotPath != "/calibre/cdb/add-book/12345/1/$dummy.epub/Main%20Library" {
		t.Fatalf("unexpected add-book path: %s", gotPath)
	}
	if gotBody != "book-bytes" {
		t.Fatalf("unexpected body %q", gotBody)
	}
	if gotAuthUser != "reader" || gotAuthPass != "secret" {
		t.Fatalf("expected basic auth, got %q/%q", gotAuthUser, gotAuthPass)
	}
}

func TestAddBookRejectsZeroCalibreID(t *testing.T) {
	dir := t.TempDir()
	bookPath := filepath.Join(dir, "Book.epub")
	if err := os.WriteFile(bookPath, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":0}`))
	}))
	defer server.Close()

	_, err := NewClient(server.Client()).AddBook(context.Background(), AddBookRequest{
		Path:     bookPath,
		Settings: Settings{Host: strings.TrimPrefix(server.URL, "http://")},
	})
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("expected rejected duplicate error, got %v", err)
	}
}
