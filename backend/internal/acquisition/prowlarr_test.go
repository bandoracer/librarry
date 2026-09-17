package acquisition

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeTorznabFeed(t *testing.T) {
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed">
  <channel>
    <item>
      <title>Project Hail Mary Andy Weir EPUB</title>
      <guid>prowlarr-guid-1</guid>
      <link>https://prowlarr.example/11/download/1</link>
      <comments>https://indexer.example/details/1</comments>
      <pubDate>Thu, 25 Jun 2026 12:30:00 -0700</pubDate>
      <enclosure url="https://prowlarr.example/11/download/1" length="12345" type="application/x-bittorrent" />
      <torznab:attr name="seeders" value="42" />
      <torznab:attr name="peers" value="45" />
      <torznab:attr name="infohash" value="ABCDEF123456" />
      <torznab:attr name="category" value="7000" />
    </item>
  </channel>
</rss>`

	releases, err := decodeTorznabFeed(strings.NewReader(feed), "Books", "torrent", 10)
	if err != nil {
		t.Fatalf("decodeTorznabFeed returned error: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}
	release := releases[0]
	if release.Title != "Project Hail Mary Andy Weir EPUB" {
		t.Fatalf("unexpected title: %q", release.Title)
	}
	if release.Indexer != "Books" {
		t.Fatalf("unexpected indexer: %q", release.Indexer)
	}
	if release.Protocol != "torrent" {
		t.Fatalf("unexpected protocol: %q", release.Protocol)
	}
	if release.DownloadURL != "https://prowlarr.example/11/download/1" {
		t.Fatalf("unexpected download url: %q", release.DownloadURL)
	}
	if release.InfoHash != "ABCDEF123456" {
		t.Fatalf("unexpected info hash: %q", release.InfoHash)
	}
	if release.SizeBytes != 12345 {
		t.Fatalf("unexpected size: %d", release.SizeBytes)
	}
	if release.Seeders != 42 || release.Leechers != 3 {
		t.Fatalf("unexpected swarm counts: seeders=%d leechers=%d", release.Seeders, release.Leechers)
	}
	if release.PublishedAt.IsZero() {
		t.Fatal("expected published date to parse")
	}
	if len(release.Categories) != 1 || release.Categories[0] != "7000" {
		t.Fatalf("unexpected categories: %#v", release.Categories)
	}
	if release.ID == "" {
		t.Fatal("expected stable release id")
	}
}

func TestProwlarrFeedUsesIndexerRSS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Fatalf("expected api key header, got %q", r.Header.Get("X-Api-Key"))
		}
		switch r.URL.Path {
		case "/api/v1/indexer":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"id":11,"name":"Books","protocol":"torrent","enableRss":true},
				{"id":12,"name":"Disabled","protocol":"torrent","enableRss":false}
			]`))
		case "/11/api":
			if r.URL.Query().Get("t") != "search" {
				t.Fatalf("expected torznab search feed, got %q", r.URL.Query().Get("t"))
			}
			if r.URL.Query().Get("cat") != "7000" {
				t.Fatalf("expected ebook category, got %q", r.URL.Query().Get("cat"))
			}
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed"><channel><item><title>Dungeon Crawler Carl EPUB</title><guid>guid-1</guid><link>https://prowlarr.example/download/1</link><torznab:attr name="seeders" value="7" /></item></channel></rss>`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewProwlarrClient(server.URL, "test-key", server.Client())
	releases, err := client.Feed(context.Background(), ReleaseFeedQuery{Format: "ebook", Limit: 10})
	if err != nil {
		t.Fatalf("Feed returned error: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}
	if releases[0].Title != "Dungeon Crawler Carl EPUB" {
		t.Fatalf("unexpected title: %q", releases[0].Title)
	}
	if releases[0].Indexer != "Books" {
		t.Fatalf("unexpected indexer: %q", releases[0].Indexer)
	}
	if releases[0].Seeders != 7 {
		t.Fatalf("unexpected seeders: %d", releases[0].Seeders)
	}
}

func TestProwlarrFeedPollsAllIndexersBeforeLimit(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		switch r.URL.Path {
		case "/api/v1/indexer":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"id":11,"name":"Old Books","protocol":"torrent","enableRss":true},
				{"id":12,"name":"New Books","protocol":"torrent","enableRss":true}
			]`))
		case "/11/api":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<rss version="2.0"><channel><item><title>Older Release EPUB</title><guid>old</guid><link>https://prowlarr.example/download/old</link><pubDate>Thu, 25 Jun 2026 12:00:00 -0700</pubDate></item></channel></rss>`))
		case "/12/api":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<rss version="2.0"><channel><item><title>Newer Release EPUB</title><guid>new</guid><link>https://prowlarr.example/download/new</link><pubDate>Thu, 25 Jun 2026 13:00:00 -0700</pubDate></item></channel></rss>`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewProwlarrClient(server.URL, "test-key", server.Client())
	releases, err := client.Feed(context.Background(), ReleaseFeedQuery{Format: "ebook", Limit: 1})
	if err != nil {
		t.Fatalf("Feed returned error: %v", err)
	}
	if calls["/11/api"] != 1 || calls["/12/api"] != 1 {
		t.Fatalf("expected both indexer feeds to be polled, got calls=%#v", calls)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release after limit, got %d", len(releases))
	}
	if releases[0].Title != "Newer Release EPUB" {
		t.Fatalf("expected newest release to win limit, got %q", releases[0].Title)
	}
}

func TestProwlarrSearchTriesISBNBeforeTitleAndDeduplicates(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Fatalf("expected api key header, got %q", r.Header.Get("X-Api-Key"))
		}
		queries = append(queries, r.URL.Query().Get("query"))
		if got := r.URL.Query()["categories"]; strings.Join(got, "|") != "7000" {
			t.Fatalf("expected ebook category, got %#v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("query") {
		case "9780593135204":
			_, _ = w.Write([]byte(`[
				{"guid":"same-guid","indexer":"Books","title":"Project Hail Mary Andy Weir EPUB","size":1024,"seeders":5,"downloadUrl":"https://prowlarr.example/download/same","protocol":"torrent"}
			]`))
		case "Project Hail Mary Andy Weir":
			_, _ = w.Write([]byte(`[
				{"guid":"same-guid","indexer":"Books","title":"Project Hail Mary Andy Weir EPUB","size":1024,"seeders":5,"downloadUrl":"https://prowlarr.example/download/same","protocol":"torrent"},
				{"guid":"other-guid","indexer":"Books","title":"Project Hail Mary Andy Weir AZW3","size":2048,"seeders":3,"downloadUrl":"https://prowlarr.example/download/other","protocol":"torrent"}
			]`))
		default:
			t.Fatalf("unexpected query: %q", r.URL.Query().Get("query"))
		}
	}))
	defer server.Close()

	client := NewProwlarrClient(server.URL, "test-key", server.Client())
	releases, err := client.Search(context.Background(), ReleaseSearchQuery{
		Query:  "Project Hail Mary Andy Weir",
		ISBN:   "9780593135204",
		Format: "ebook",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if strings.Join(queries, "|") != "9780593135204|Project Hail Mary Andy Weir" {
		t.Fatalf("expected ISBN-first queries, got %#v", queries)
	}
	if len(releases) != 2 {
		t.Fatalf("expected duplicate releases to collapse, got %d: %#v", len(releases), releases)
	}
	if releases[0].Title != "Project Hail Mary Andy Weir EPUB" || releases[1].Title != "Project Hail Mary Andy Weir AZW3" {
		t.Fatalf("unexpected release order: %#v", releases)
	}
}

func TestCategoriesForAnyFormatIncludeAudiobooks(t *testing.T) {
	if got := categoriesForFormat("any"); got != "7000,3030" {
		t.Fatalf("expected any format to include book and audiobook categories, got %q", got)
	}
	if got := categoriesForFormat(""); got != "7000,3030" {
		t.Fatalf("expected empty format to include book and audiobook categories, got %q", got)
	}
}

func TestProwlarrSearchSendsRepeatedCategoryParamsForAudiobooks(t *testing.T) {
	var categories []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prowlarr binds categories as an array of int; a comma-joined value
		// fails model binding with 400 Bad Request, so each category must be
		// sent as its own query parameter.
		if raw := r.URL.RawQuery; strings.Contains(raw, "categories=7000%2C3030") {
			t.Fatalf("categories were comma-joined into one parameter: %s", raw)
		}
		categories = r.URL.Query()["categories"]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewProwlarrClient(server.URL, "test-key", server.Client())
	if _, err := client.Search(context.Background(), ReleaseSearchQuery{
		Query:  "The Hobbit",
		Format: "audiobook",
		Limit:  10,
	}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if strings.Join(categories, "|") != "7000|3030" {
		t.Fatalf("expected separate audiobook categories, got %#v", categories)
	}
}
