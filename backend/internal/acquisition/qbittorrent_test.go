package acquisition

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestQBittorrentActionSetCategoryAndLocation(t *testing.T) {
	tests := []struct {
		name       string
		request    DownloadActionRequest
		endpoint   string
		field      string
		wantValue  string
		wantHashes string
	}{
		{
			name: "category",
			request: DownloadActionRequest{
				Action:   DownloadActionSetCategory,
				IDs:      []string{"abc"},
				Category: "books-audiobook",
			},
			endpoint:   "/api/v2/torrents/setCategory",
			field:      "category",
			wantValue:  "books-audiobook",
			wantHashes: "abc",
		},
		{
			name: "location",
			request: DownloadActionRequest{
				Action:   DownloadActionSetLocation,
				IDs:      []string{"abc", "def"},
				SavePath: "/data/torrents/books/audio",
			},
			endpoint:   "/api/v2/torrents/setLocation",
			field:      "location",
			wantValue:  "/data/torrents/books/audio",
			wantHashes: "abc|def",
		},
		{
			name: "download limit",
			request: DownloadActionRequest{
				Action:        DownloadActionSetDownloadLimit,
				IDs:           []string{"abc"},
				DownloadLimit: 1_048_576,
			},
			endpoint:   "/api/v2/torrents/setDownloadLimit",
			field:      "limit",
			wantValue:  "1048576",
			wantHashes: "abc",
		},
		{
			name: "upload limit",
			request: DownloadActionRequest{
				Action:      DownloadActionSetUploadLimit,
				IDs:         []string{"abc", "def"},
				UploadLimit: 262_144,
			},
			endpoint:   "/api/v2/torrents/setUploadLimit",
			field:      "limit",
			wantValue:  "262144",
			wantHashes: "abc|def",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var endpoint string
			var hashes string
			var fieldValue string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				endpoint = r.URL.Path
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				hashes = r.Form.Get("hashes")
				fieldValue = r.Form.Get(test.field)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := NewQBittorrentClient(server.URL, "", "", server.Client())
			result, err := client.Action(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}
			if endpoint != test.endpoint {
				t.Fatalf("expected endpoint %s, got %s", test.endpoint, endpoint)
			}
			if hashes != test.wantHashes {
				t.Fatalf("expected hashes %s, got %s", test.wantHashes, hashes)
			}
			if fieldValue != test.wantValue {
				t.Fatalf("expected %s %s, got %s", test.field, test.wantValue, fieldValue)
			}
			if !result.Applied || result.Action != normalizeAction(test.request.Action) {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
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
			"last_activity": 1_700_000_120,
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
	if status.LastActivityAt == nil || status.LastActivityAt.Unix() != 1_700_000_120 {
		t.Fatalf("expected last activity timestamp, got %+v", status.LastActivityAt)
	}
}

func TestQBittorrentDetailsMapsPropertiesFilesAndTrackers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("hash") != "abc123" && r.URL.Query().Get("hashes") != "abc123" {
			t.Fatalf("expected hash query for %s, got %s", r.URL.Path, r.URL.RawQuery)
		}
		switch r.URL.Path {
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"hash":          "abc123",
				"name":          "Book.epub",
				"state":         "downloading",
				"progress":      0.5,
				"save_path":     "/data/torrents/books",
				"category":      "books-ebook",
				"tags":          "librarry",
				"size":          1000,
				"downloaded":    500,
				"dlspeed":       12,
				"num_seeds":     8,
				"num_leechs":    2,
				"added_on":      1_700_000_000,
				"last_activity": 1_700_000_120,
			}})
		case "/api/v2/torrents/properties":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"save_path":            "/data/torrents/books",
				"addition_date":        1_700_000_000,
				"completion_date":      0,
				"total_size":           1000,
				"total_downloaded":     500,
				"total_uploaded":       25,
				"dl_speed":             12,
				"up_speed":             3,
				"eta":                  42,
				"share_ratio":          0.25,
				"nb_connections":       6,
				"nb_connections_limit": 50,
				"piece_size":           16,
				"pieces_have":          4,
				"pieces_num":           8,
				"created_by":           "librarry-test",
			})
		case "/api/v2/torrents/files":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"name":         "Book.epub",
				"size":         1000,
				"progress":     0.5,
				"priority":     1,
				"availability": 3.5,
				"piece_range":  []int{0, 7},
			}})
		case "/api/v2/torrents/trackers":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"url":            "https://tracker.example/announce",
				"status":         2,
				"tier":           0,
				"msg":            "working",
				"num_peers":      10,
				"num_seeds":      8,
				"num_leeches":    2,
				"num_downloaded": 12,
			}})
		case "/api/v2/sync/torrentPeers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"full_update": true,
				"rid":         1,
				"peers": map[string]any{
					"203.0.113.10:51413": map[string]any{
						"client":       "Transmission 4.0",
						"connection":   "BT",
						"country":      "United States",
						"country_code": "US",
						"dl_speed":     12,
						"downloaded":   500,
						"files":        "Book.epub",
						"flags":        "I",
						"flags_desc":   "incoming",
						"ip":           "203.0.113.10",
						"port":         51413,
						"progress":     0.5,
						"relevance":    1,
						"up_speed":     3,
						"uploaded":     25,
					},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewQBittorrentClient(server.URL, "", "", server.Client())
	details, err := client.Details(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if details.Status.ID != "abc123" || details.Properties.TotalSizeBytes != 1000 || details.Properties.PiecesTotal != 8 {
		t.Fatalf("unexpected details: %+v", details)
	}
	if len(details.Files) != 1 || details.Files[0].ID != 0 || details.Files[0].FirstPiece != 0 || details.Files[0].LastPiece != 7 {
		t.Fatalf("unexpected files: %+v", details.Files)
	}
	if len(details.Trackers) != 1 || details.Trackers[0].Status != "working" || details.Trackers[0].Seeds != 8 {
		t.Fatalf("unexpected trackers: %+v", details.Trackers)
	}
	if len(details.Peers) != 1 || details.Peers[0].IP != "203.0.113.10" || details.Peers[0].DownloadRate != 12 {
		t.Fatalf("unexpected peers: %+v", details.Peers)
	}
}

func TestQBittorrentFileActionSetsPriority(t *testing.T) {
	var endpoint string
	var hash string
	var fileIDs string
	var priority string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint = r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		hash = r.Form.Get("hash")
		fileIDs = r.Form.Get("id")
		priority = r.Form.Get("priority")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewQBittorrentClient(server.URL, "", "", server.Client())
	result, err := client.FileAction(context.Background(), DownloadFileActionRequest{
		DownloadID: "abc123",
		Action:     DownloadFileActionHigh,
		IDs:        []int{0, 2, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "/api/v2/torrents/filePrio" || hash != "abc123" || fileIDs != "0|2" || priority != "6" {
		t.Fatalf("unexpected file action form endpoint=%s hash=%s ids=%s priority=%s", endpoint, hash, fileIDs, priority)
	}
	if !result.Applied || result.Action != DownloadFileActionHigh || result.Priority != 6 {
		t.Fatalf("unexpected file action result: %+v", result)
	}
}

func TestQBittorrentTrackerActions(t *testing.T) {
	tests := []struct {
		name       string
		request    DownloadTrackerActionRequest
		endpoint   string
		wantForm   map[string]string
		wantURLs   []string
		wantAction string
	}{
		{
			name:       "add",
			request:    DownloadTrackerActionRequest{Action: DownloadTrackerActionAdd, URLs: []string{"https://tracker.one/announce", "https://tracker.two/announce"}},
			endpoint:   "/api/v2/torrents/addTrackers",
			wantForm:   map[string]string{"hash": "abc123", "urls": "https://tracker.one/announce\nhttps://tracker.two/announce"},
			wantURLs:   []string{"https://tracker.one/announce", "https://tracker.two/announce"},
			wantAction: DownloadTrackerActionAdd,
		},
		{
			name:       "edit",
			request:    DownloadTrackerActionRequest{Action: DownloadTrackerActionEdit, OriginalURL: "https://old.example/announce", NewURL: "https://new.example/announce"},
			endpoint:   "/api/v2/torrents/editTracker",
			wantForm:   map[string]string{"hash": "abc123", "origUrl": "https://old.example/announce", "newUrl": "https://new.example/announce"},
			wantURLs:   []string{"https://old.example/announce", "https://new.example/announce"},
			wantAction: DownloadTrackerActionEdit,
		},
		{
			name:       "remove",
			request:    DownloadTrackerActionRequest{Action: DownloadTrackerActionRemove, URL: "https://tracker.one/announce"},
			endpoint:   "/api/v2/torrents/removeTrackers",
			wantForm:   map[string]string{"hash": "abc123", "urls": "https://tracker.one/announce"},
			wantURLs:   []string{"https://tracker.one/announce"},
			wantAction: DownloadTrackerActionRemove,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var endpoint string
			form := map[string]string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				endpoint = r.URL.Path
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				for key := range test.wantForm {
					form[key] = r.Form.Get(key)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := NewQBittorrentClient(server.URL, "", "", server.Client())
			result, err := client.TrackerAction(context.Background(), "abc123", test.request)
			if err != nil {
				t.Fatal(err)
			}
			if endpoint != test.endpoint {
				t.Fatalf("expected endpoint %s, got %s", test.endpoint, endpoint)
			}
			for key, want := range test.wantForm {
				if form[key] != want {
					t.Fatalf("expected form %s=%q, got %q", key, want, form[key])
				}
			}
			if result.Action != test.wantAction || result.DownloadID != "abc123" || !result.Applied {
				t.Fatalf("unexpected tracker result: %+v", result)
			}
			if strings.Join(result.URLs, "|") != strings.Join(test.wantURLs, "|") {
				t.Fatalf("expected urls %+v, got %+v", test.wantURLs, result.URLs)
			}
		})
	}
}

func TestMergeStoredDownloadStateIncludesFailureMetadata(t *testing.T) {
	failedAt := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	service := &Service{store: fakeDownloadStore{downloads: []DownloadStatus{{
		ID:            "abc123",
		ImportStatus:  "error",
		ImportError:   "no supported file",
		FailureReason: "qBittorrent reports missing files",
		FailedAt:      &failedAt,
		RetryCount:    2,
		ReplacementID: "def456",
	}}}}

	merged := service.mergeStoredDownloadState(context.Background(), []DownloadStatus{{ID: "abc123", Name: "Book"}}, DownloadListQuery{})
	if len(merged) != 1 {
		t.Fatalf("expected one download, got %d", len(merged))
	}
	if merged[0].FailureReason != "qBittorrent reports missing files" || merged[0].RetryCount != 2 || merged[0].ReplacementID != "def456" {
		t.Fatalf("expected failure metadata to merge, got %+v", merged[0])
	}
	if merged[0].ImportStatus != "error" || merged[0].ImportError != "no supported file" {
		t.Fatalf("expected import metadata to merge, got %+v", merged[0])
	}
	if merged[0].FailedAt == nil || !merged[0].FailedAt.Equal(failedAt) {
		t.Fatalf("expected failed timestamp to merge, got %+v", merged[0].FailedAt)
	}
}

type fakeDownloadStore struct {
	downloads []DownloadStatus
}

func (s fakeDownloadStore) UpsertDownloads(context.Context, []DownloadStatus) error {
	return nil
}

func (s fakeDownloadStore) ListDownloads(context.Context, DownloadListQuery) ([]DownloadStatus, error) {
	return s.downloads, nil
}

func (s fakeDownloadStore) MarkDownloadsDeleted(context.Context, []string) error {
	return nil
}

func (s fakeDownloadStore) MarkDownloadFailed(context.Context, string, string) error {
	return nil
}

func (s fakeDownloadStore) MarkDownloadReplacement(context.Context, string, string) error {
	return nil
}

func (s fakeDownloadStore) MarkDownloadImported(context.Context, string, string) error {
	return nil
}

func (s fakeDownloadStore) MarkDownloadImportError(context.Context, string, string) error {
	return nil
}
