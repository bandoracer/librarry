package acquisition

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSABnzbdAddURL(t *testing.T) {
	var values map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		values = queryValues(r)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  true,
			"nzo_ids": []string{"SABnzbd_nzo_abc123"},
		})
	}))
	defer server.Close()

	client := NewSABnzbdClient(server.URL, "api-key", "", "", server.Client())
	status, err := client.Add(context.Background(), DownloadRequest{
		ReleaseURL: "https://indexer.example/get/abc.nzb",
		Title:      "Project Hail Mary EPUB",
		Category:   CategoryBooksEbook,
		Tags:       []string{"librarry"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if values["mode"] != "addurl" || values["apikey"] != "api-key" {
		t.Fatalf("unexpected query values: %#v", values)
	}
	if values["name"] != "https://indexer.example/get/abc.nzb" {
		t.Fatalf("unexpected name value: %q", values["name"])
	}
	if values["cat"] != CategoryBooksEbook || values["nzbname"] != "Project Hail Mary EPUB" {
		t.Fatalf("unexpected add metadata: %#v", values)
	}
	if status.Client != "SABnzbd" || status.ID != "SABnzbd_nzo_abc123" || status.State != "queued" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestSABnzbdListQueueAndHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("mode") {
		case "queue":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"queue": map[string]any{
					"slots": []map[string]any{{
						"nzo_id":     "SABnzbd_nzo_downloading",
						"filename":   "Dungeon Crawler Carl.m4b",
						"status":     "Downloading",
						"cat":        CategoryBooksAudiobook,
						"percentage": "25",
						"size":       "1.0 GB",
						"mbleft":     "768",
						"timeleft":   "0:10:05",
					}},
				},
			})
		case "history":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"history": map[string]any{
					"slots": []map[string]any{{
						"nzo_id":   "SABnzbd_nzo_complete",
						"name":     "Project Hail Mary.epub",
						"status":   "Completed",
						"category": CategoryBooksEbook,
						"size":     "512 MB",
					}, {
						"nzo_id":       "SABnzbd_nzo_failed",
						"name":         "Broken Book.epub",
						"status":       "Failed",
						"category":     CategoryBooksEbook,
						"size":         "10 MB",
						"fail_message": "missing articles",
					}},
				},
			})
		default:
			t.Fatalf("unexpected mode %q", r.URL.Query().Get("mode"))
		}
	}))
	defer server.Close()

	client := NewSABnzbdClient(server.URL, "api-key", "", "", server.Client())
	statuses, err := client.List(context.Background(), DownloadListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	byID := map[string]DownloadStatus{}
	for _, status := range statuses {
		byID[status.ID] = status
	}
	downloading := byID["SABnzbd_nzo_downloading"]
	if downloading.Client != "SABnzbd" || downloading.State != "downloading" || downloading.Progress != 0.25 {
		t.Fatalf("unexpected downloading status: %+v", downloading)
	}
	if downloading.SizeBytes != 1_073_741_824 || downloading.DownloadedBytes != 268_435_456 || downloading.ETASeconds != 605 {
		t.Fatalf("unexpected queue metrics: %+v", downloading)
	}
	complete := byID["SABnzbd_nzo_complete"]
	if complete.State != "completed" || complete.Progress != 1 || complete.DownloadedBytes != complete.SizeBytes {
		t.Fatalf("unexpected complete status: %+v", complete)
	}
	failed := byID["SABnzbd_nzo_failed"]
	if failed.State != "error" || failed.FailureReason != "missing articles" || failed.FailedAt == nil {
		t.Fatalf("unexpected failed status: %+v", failed)
	}
}

func TestSABnzbdActions(t *testing.T) {
	var commands []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		commands = append(commands, query.Get("mode")+":"+query.Get("name")+":"+query.Get("value")+":"+query.Get("del_files"))
		_ = json.NewEncoder(w).Encode(map[string]any{"status": true})
	}))
	defer server.Close()

	client := NewSABnzbdClient(server.URL, "api-key", "", "", server.Client())
	for _, action := range []string{DownloadActionStart, DownloadActionStop, DownloadActionDelete} {
		_, err := client.Action(context.Background(), DownloadActionRequest{
			Action:      action,
			IDs:         []string{"SABnzbd_nzo_abc123"},
			DeleteFiles: true,
		})
		if err != nil {
			t.Fatalf("%s action failed: %v", action, err)
		}
	}
	joined := strings.Join(commands, "|")
	want := "queue:resume:SABnzbd_nzo_abc123:|queue:pause:SABnzbd_nzo_abc123:|queue:delete:SABnzbd_nzo_abc123:true|history:delete:SABnzbd_nzo_abc123:"
	if joined != want {
		t.Fatalf("unexpected commands:\nwant %s\n got %s", want, joined)
	}
}

func TestServiceRoutesUsenetGrabsToSABnzbd(t *testing.T) {
	var mode string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode = r.URL.Query().Get("mode")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  true,
			"nzo_ids": []string{"SABnzbd_nzo_routed"},
		})
	}))
	defer server.Close()

	service := NewService(IntegrationConfig{
		SABnzbdURL:    server.URL,
		SABnzbdAPIKey: "api-key",
	})
	status, err := service.Grab(context.Background(), DownloadRequest{
		Protocol:   "usenet",
		ReleaseURL: "https://indexer.example/get/book.nzb",
		Title:      "Book",
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != "addurl" {
		t.Fatalf("expected SABnzbd addurl mode, got %q", mode)
	}
	if status.Client != "SABnzbd" || status.ID != "SABnzbd_nzo_routed" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func queryValues(r *http.Request) map[string]string {
	values := map[string]string{}
	for key, list := range r.URL.Query() {
		values[key] = firstString(list)
	}
	return values
}
