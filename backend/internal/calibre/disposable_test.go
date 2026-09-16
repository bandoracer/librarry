package calibre

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This URL is supplied only by scripts/test-calibre.py for its new empty,
// authenticated container. Ordinary test runs never mutate a Calibre server.
func TestDisposableCalibre(t *testing.T) {
	endpoint := os.Getenv("LIBRARRY_CALIBRE_FIXTURE_URL")
	if endpoint == "" {
		t.Skip("run scripts/test-calibre.py for an isolated Calibre library")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.User != nil {
		t.Fatal("Calibre fixture must be the disposable loopback server")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := NewClient(&http.Client{Timeout: 30 * time.Second})
	client.jobID = func() int { return 424242 }
	settings := Settings{Host: endpoint, Username: "fixture", Password: "fixture-password", Library: "library", OutputFormat: "TXT"}
	source := filepath.Join("..", "..", "..", "docs", "fixtures", "e2e", "librarry-public-domain-e2e-book.epub")
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	added, err := client.AddBook(ctx, AddBookRequest{Settings: settings, Path: source})
	if err != nil {
		t.Fatal(err)
	}
	if added.ID != 1 {
		t.Fatalf("fresh library book should be 1, not echoed job 424242: %+v", added)
	}
	defer client.DeleteBooks(context.Background(), DeleteBooksRequest{Settings: settings, IDs: []int{added.ID}})
	readBook := func() map[string]any {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/ajax/book/%d?library_id=library", endpoint, added.ID), nil)
		if err != nil {
			t.Fatal(err)
		}
		request.SetBasicAuth(settings.Username, settings.Password)
		response, err := client.do(request, settings)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("book readback: %d %s", response.StatusCode, body)
		}
		var book map[string]any
		if err := json.NewDecoder(response.Body).Decode(&book); err != nil {
			t.Fatal(err)
		}
		return book
	}
	if readBook()["title"] != "Librarry Public Domain E2E Book" {
		t.Fatal("upload metadata missing")
	}
	if err := client.SetFields(ctx, SetFieldsRequest{Settings: settings, ID: added.ID, Metadata: Metadata{Title: "Calibre contract fixture", Authors: []string{"Librarry fixture"}}}); err != nil {
		t.Fatal(err)
	}
	if readBook()["title"] != "Calibre contract fixture" {
		t.Fatal("metadata targeted the wrong book")
	}
	converted, err := client.Convert(ctx, ConvertRequest{Settings: settings, ID: added.ID, InputFormat: "EPUB"})
	if err != nil {
		t.Fatal(err)
	}
	if len(converted.Jobs) != 1 {
		t.Fatal(converted)
	}
	statuses, err := client.PollConversions(ctx, PollConversionsRequest{Settings: settings, Jobs: converted.Jobs, MaxAttempts: 80, Interval: 250 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].Running || !statuses[0].OK {
		t.Fatal(statuses)
	}
	raw, _ := json.Marshal(readBook()["formats"])
	if !strings.Contains(strings.ToUpper(string(raw)), "TXT") {
		t.Fatalf("converted format absent: %s", raw)
	}
	if err := client.DeleteBooks(ctx, DeleteBooksRequest{Settings: settings, IDs: []int{added.ID}}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(source)
	if err != nil || string(after) != string(before) {
		t.Fatal("fixture source modified", err)
	}
}
