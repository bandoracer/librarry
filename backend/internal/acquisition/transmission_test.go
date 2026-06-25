package acquisition

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransmissionAddRetriesSessionAndLabelsTorrent(t *testing.T) {
	var methods []string
	var sessionHeaders []string
	var addArgs map[string]any
	var labelArgs map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != transmissionRPCPath {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if len(methods) == 0 && r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "session-1")
			w.WriteHeader(http.StatusConflict)
			return
		}
		sessionHeaders = append(sessionHeaders, r.Header.Get("X-Transmission-Session-Id"))
		var payload struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, payload.Method)
		switch payload.Method {
		case "torrent-add":
			addArgs = payload.Arguments
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": "success",
				"arguments": map[string]any{
					"torrent-added": map[string]any{"id": 42, "hashString": "abc123", "name": "Book.epub"},
				},
			})
		case "torrent-set":
			labelArgs = payload.Arguments
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
		default:
			t.Fatalf("unexpected method %s", payload.Method)
		}
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "user", "pass", server.Client())
	status, err := client.Add(context.Background(), DownloadRequest{
		ReleaseURL: "magnet:?xt=urn:btih:abc123",
		Title:      "Book",
		Category:   CategoryBooksEbook,
		SavePath:   "/downloads/books",
		Paused:     true,
		Tags:       []string{"librarry", "wanted:1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, ",") != "torrent-add,torrent-set" {
		t.Fatalf("unexpected methods: %v", methods)
	}
	if len(sessionHeaders) != 2 || sessionHeaders[0] != "session-1" || sessionHeaders[1] != "session-1" {
		t.Fatalf("expected session id retry and reuse, got %#v", sessionHeaders)
	}
	if addArgs["filename"] != "magnet:?xt=urn:btih:abc123" || addArgs["download-dir"] != "/downloads/books" || addArgs["paused"] != true {
		t.Fatalf("unexpected add args: %#v", addArgs)
	}
	labels, ok := labelArgs["labels"].([]any)
	if !ok || len(labels) != 3 || labels[0] != CategoryBooksEbook || labels[1] != "librarry" || labels[2] != "wanted:1" {
		t.Fatalf("unexpected labels: %#v", labelArgs["labels"])
	}
	if status.Client != "Transmission" || status.ID != "abc123" || status.Name != "Book.epub" || status.State != "queued" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestTransmissionListMapsDownloadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Method != "torrent-get" {
			t.Fatalf("unexpected method %s", payload.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"arguments": map[string]any{
				"torrents": []map[string]any{{
					"id":                 42,
					"hashString":         "abc123",
					"name":               "Book.epub",
					"status":             4,
					"percentDone":        0.5,
					"downloadDir":        "/downloads/books",
					"totalSize":          1000,
					"downloadedEver":     500,
					"uploadedEver":       25,
					"rateDownload":       12,
					"rateUpload":         3,
					"eta":                42,
					"uploadRatio":        0.25,
					"peersConnected":     4,
					"peersGettingFromUs": 2,
					"peersSendingToUs":   1,
					"addedDate":          1_700_000_000,
					"activityDate":       1_700_000_120,
					"labels":             []string{CategoryBooksEbook, "librarry"},
				}, {
					"id":          43,
					"hashString":  "def456",
					"name":        "Other.epub",
					"status":      0,
					"percentDone": 1,
					"labels":      []string{"other"},
				}},
			},
		})
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "", "", server.Client())
	statuses, err := client.List(context.Background(), DownloadListQuery{Tag: "librarry"})
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected one status, got %d", len(statuses))
	}
	status := statuses[0]
	if status.ID != "abc123" || status.Client != "Transmission" || status.State != "downloading" || status.Progress != 0.5 {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.Category != CategoryBooksEbook || status.SizeBytes != 1000 || status.DownloadedBytes != 500 || status.DownloadRate != 12 {
		t.Fatalf("unexpected mapped fields: %+v", status)
	}
	if status.AddedAt == nil || status.LastActivityAt == nil || status.LastSeenAt == nil {
		t.Fatalf("expected timestamps: %+v", status)
	}
}

func TestTransmissionActions(t *testing.T) {
	tests := []struct {
		name     string
		request  DownloadActionRequest
		method   string
		wantArgs map[string]any
	}{
		{
			name:    "start",
			request: DownloadActionRequest{Action: DownloadActionStart, IDs: []string{"abc123"}},
			method:  "torrent-start",
		},
		{
			name:    "stop",
			request: DownloadActionRequest{Action: DownloadActionStop, IDs: []string{"abc123"}},
			method:  "torrent-stop",
		},
		{
			name:     "delete",
			request:  DownloadActionRequest{Action: DownloadActionDelete, IDs: []string{"abc123"}, DeleteFiles: true},
			method:   "torrent-remove",
			wantArgs: map[string]any{"delete-local-data": true},
		},
		{
			name:    "recheck",
			request: DownloadActionRequest{Action: DownloadActionRecheck, IDs: []string{"abc123"}},
			method:  "torrent-verify",
		},
		{
			name:     "location",
			request:  DownloadActionRequest{Action: DownloadActionSetLocation, IDs: []string{"abc123"}, SavePath: "/downloads/new"},
			method:   "torrent-set-location",
			wantArgs: map[string]any{"location": "/downloads/new", "move": true},
		},
		{
			name:     "download limit",
			request:  DownloadActionRequest{Action: DownloadActionSetDownloadLimit, IDs: []string{"abc123"}, DownloadLimit: 1_048_576},
			method:   "torrent-set",
			wantArgs: map[string]any{"downloadLimited": true, "downloadLimit": float64(1024)},
		},
		{
			name:     "upload limit",
			request:  DownloadActionRequest{Action: DownloadActionSetUploadLimit, IDs: []string{"abc123"}, UploadLimit: 262_144},
			method:   "torrent-set",
			wantArgs: map[string]any{"uploadLimited": true, "uploadLimit": float64(256)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var method string
			var args map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Method    string         `json:"method"`
					Arguments map[string]any `json:"arguments"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				method = payload.Method
				args = payload.Arguments
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
			}))
			defer server.Close()

			client := NewTransmissionClient(server.URL, "", "", server.Client())
			result, err := client.Action(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}
			if method != test.method {
				t.Fatalf("expected method %s, got %s", test.method, method)
			}
			for key, want := range test.wantArgs {
				if args[key] != want {
					t.Fatalf("expected arg %s=%#v, got %#v in %#v", key, want, args[key], args)
				}
			}
			if !result.Applied || result.Action != normalizeAction(test.request.Action) {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestServiceRoutesTorrentGrabsToTransmissionWhenQBittorrentMissing(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, payload.Method)
		switch payload.Method {
		case "torrent-add":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": "success",
				"arguments": map[string]any{
					"torrent-added": map[string]any{"id": 42, "hashString": "abc123", "name": "Book.epub"},
				},
			})
		case "torrent-set":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
		default:
			t.Fatalf("unexpected method %s", payload.Method)
		}
	}))
	defer server.Close()

	service := NewService(IntegrationConfig{TransmissionURL: server.URL})
	status, err := service.Grab(context.Background(), DownloadRequest{
		Protocol:   "torrent",
		ReleaseURL: "magnet:?xt=urn:btih:abc123",
		Title:      "Book",
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.Client != "Transmission" || status.ID != "abc123" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if strings.Join(methods, ",") != "torrent-add,torrent-set" {
		t.Fatalf("unexpected methods: %v", methods)
	}
}

func TestServiceRoutesExplicitTransmissionAction(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, payload.Method)
		if payload.Method == "torrent-get" {
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{"torrents": []any{}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
	}))
	defer server.Close()

	service := NewService(IntegrationConfig{
		QBittorrentURL:  "http://qbittorrent.invalid",
		TransmissionURL: server.URL,
	})
	result, err := service.DownloadAction(context.Background(), DownloadActionRequest{
		Client: "Transmission",
		Action: DownloadActionStop,
		IDs:    []string{"abc123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) == 0 || methods[0] != "torrent-stop" {
		t.Fatalf("expected Transmission stop method first, got %v", methods)
	}
	if !result.Applied || strings.Join(result.IDs, ",") != "abc123" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
