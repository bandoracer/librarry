package calibre

import (
	"context"
	"encoding/json"
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

func TestDeleteBooksPostsCalibreDeleteEndpoint(t *testing.T) {
	var gotPath string
	var gotMethod string
	var gotAuthUser string
	var gotAuthPass string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotMethod = r.Method
		gotAuthUser, gotAuthPass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := NewClient(server.Client()).DeleteBooks(context.Background(), DeleteBooksRequest{
		IDs: []int{7, 0, 8, 7},
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
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/calibre/cdb/delete-books/7,8/Main%20Library" {
		t.Fatalf("unexpected delete-books path: %s", gotPath)
	}
	if gotAuthUser != "reader" || gotAuthPass != "secret" {
		t.Fatalf("expected basic auth, got %q/%q", gotAuthUser, gotAuthPass)
	}
}

func TestDeleteBooksNoopsWithoutPositiveIDs(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := NewClient(server.Client()).DeleteBooks(context.Background(), DeleteBooksRequest{
		IDs:      []int{0, -1},
		Settings: Settings{Host: strings.TrimPrefix(server.URL, "http://")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("expected no request without positive IDs")
	}
}

func TestSetFieldsPostsCalibreMetadataPayload(t *testing.T) {
	var gotPath string
	var gotMethod string
	var gotContentType string
	var gotAuthUser string
	var gotAuthPass string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotAuthUser, gotAuthPass, _ = r.BasicAuth()
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	seriesIndex := 1.5
	err := NewClient(server.Client()).SetFields(context.Background(), SetFieldsRequest{
		ID: 99,
		Settings: Settings{
			Host:     strings.TrimPrefix(server.URL, "http://"),
			URLBase:  "/calibre",
			Username: "reader",
			Password: "secret",
			Library:  "Main Library",
		},
		Metadata: Metadata{
			Title:       "Project Hail Mary",
			Authors:     []string{"Andy Weir", "Andy Weir", " "},
			Publisher:   "Ballantine",
			Languages:   "eng",
			Tags:        []string{"science-fiction", "science-fiction", "space"},
			Comments:    "A rescue mission.",
			Identifiers: map[string]string{"isbn": "9780593135204", "empty": ""},
			Series:      "Hail Mary",
			SeriesIndex: &seriesIndex,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/calibre/cdb/set-fields/99/Main%20Library" {
		t.Fatalf("unexpected set-fields path: %s", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("expected application/json, got %s", gotContentType)
	}
	if gotAuthUser != "reader" || gotAuthPass != "secret" {
		t.Fatalf("expected basic auth, got %q/%q", gotAuthUser, gotAuthPass)
	}
	if gotPayload["loaded_book_ids"].([]any)[0].(float64) != 99 {
		t.Fatalf("unexpected loaded ids: %#v", gotPayload)
	}
	changes := gotPayload["changes"].(map[string]any)
	if changes["title"] != "Project Hail Mary" || changes["publisher"] != "Ballantine" ||
		changes["languages"] != "eng" || changes["comments"] != "A rescue mission." ||
		changes["series"] != "Hail Mary" || changes["series_index"].(float64) != 1.5 {
		t.Fatalf("unexpected changes payload: %#v", changes)
	}
	authors := changes["authors"].([]any)
	if len(authors) != 1 || authors[0] != "Andy Weir" {
		t.Fatalf("unexpected authors: %#v", authors)
	}
	tags := changes["tags"].([]any)
	if len(tags) != 2 || tags[0] != "science-fiction" || tags[1] != "space" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	identifiers := changes["identifiers"].(map[string]any)
	if identifiers["isbn"] != "9780593135204" || identifiers["empty"] != nil {
		t.Fatalf("unexpected identifiers: %#v", identifiers)
	}
}

func TestSetFieldsNoopsWithoutMetadataChanges(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := NewClient(server.Client()).SetFields(context.Background(), SetFieldsRequest{
		ID:       99,
		Settings: Settings{Host: strings.TrimPrefix(server.URL, "http://")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("expected no request without metadata changes")
	}
}
