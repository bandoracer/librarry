package acquisition

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQBittorrentActionStop(t *testing.T) {
	var endpoint string
	var hashes string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint = r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		hashes = r.Form.Get("hashes")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewQBittorrentClient(server.URL, "", "", server.Client())
	result, err := client.Action(context.Background(), DownloadActionRequest{
		Action: DownloadActionStop,
		IDs:    []string{"abc", "def"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "/api/v2/torrents/stop" {
		t.Fatalf("expected stop endpoint, got %s", endpoint)
	}
	if hashes != "abc|def" {
		t.Fatalf("expected joined hashes, got %s", hashes)
	}
	if !result.Applied || result.Action != DownloadActionStop {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestQBittorrentListMapsDownloadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/torrents/info" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("tag") != "librarry" {
			t.Fatalf("expected tag query, got %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"hash":          "abc123",
			"name":          "Book.epub",
			"state":         "downloading",
			"progress":      0.5,
			"save_path":     "/data/torrents/books",
			"category":      "books-ebook",
			"tags":          "librarry, ui",
			"size":          1000,
			"downloaded":    500,
			"uploaded":      25,
			"dlspeed":       12,
			"upspeed":       3,
			"eta":           42,
			"ratio":         0.25,
			"num_seeds":     8,
			"num_leechs":    2,
			"added_on":      1_700_000_000,
			"completion_on": 0,
		}})
	}))
	defer server.Close()

	client := NewQBittorrentClient(server.URL, "", "", server.Client())
	statuses, err := client.List(context.Background(), DownloadListQuery{Tag: "librarry"})
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected one status, got %d", len(statuses))
	}
	status := statuses[0]
	if status.ID != "abc123" || status.SizeBytes != 1000 || status.DownloadedBytes != 500 || status.DownloadRate != 12 {
		t.Fatalf("unexpected status: %+v", status)
	}
	if strings.Join(status.Tags, ",") != "librarry,ui" {
		t.Fatalf("unexpected tags: %+v", status.Tags)
	}
	if status.AddedAt == nil || status.LastSeenAt == nil {
		t.Fatalf("expected timestamps: %+v", status)
	}
}
