package acquisition

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransmissionAddUploadsTorrentMetainfo(t *testing.T) {
	var addArgs map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != transmissionRPCPath {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var payload struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Method != "torrent-add" {
			t.Fatalf("unexpected method %s", payload.Method)
		}
		addArgs = payload.Arguments
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"arguments": map[string]any{
				"torrent-added": map[string]any{"id": 42, "hashString": "abc123", "name": "Book upload"},
			},
		})
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "", "", server.Client())
	status, err := client.Add(context.Background(), DownloadRequest{
		Title:      "Book upload",
		SavePath:   "/downloads/books",
		Paused:     true,
		UploadName: "book.torrent",
		UploadData: []byte("torrent-bytes"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if addArgs["filename"] != nil {
		t.Fatalf("expected upload to avoid filename arg, got %#v", addArgs)
	}
	if addArgs["metainfo"] != base64.StdEncoding.EncodeToString([]byte("torrent-bytes")) || addArgs["paused"] != true {
		t.Fatalf("unexpected add args: %#v", addArgs)
	}
	if status.Client != "Transmission" || status.ID != "abc123" || status.Name != "Book upload" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

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

func TestTransmissionResourcesDeriveCategoriesAndTagsFromLabels(t *testing.T) {
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
		if _, hasIDs := payload.Arguments["ids"]; hasIDs {
			t.Fatalf("resource lookup should not request specific ids: %#v", payload.Arguments)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"arguments": map[string]any{
				"torrents": []map[string]any{{
					"id":          42,
					"hashString":  "abc123",
					"downloadDir": "/downloads/books",
					"labels":      []string{CategoryBooksEbook, "librarry"},
				}, {
					"id":          43,
					"hashString":  "def456",
					"downloadDir": "/downloads/audio",
					"labels":      []string{CategoryBooksAudiobook, "manual", "librarry"},
				}, {
					"id":          44,
					"hashString":  "ghi789",
					"downloadDir": "/downloads/other",
					"labels":      []string{"other"},
				}},
			},
		})
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "", "", server.Client())
	resources, err := client.Resources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resources.Client != "Transmission" {
		t.Fatalf("unexpected resources client: %+v", resources)
	}
	if len(resources.Categories) != 3 ||
		resources.Categories[0].Name != CategoryBooksAudiobook ||
		resources.Categories[1].Name != CategoryBooksEbook ||
		resources.Categories[1].SavePath != "/downloads/books" {
		t.Fatalf("unexpected categories: %+v", resources.Categories)
	}
	if strings.Join(resources.Tags, ",") != "books-audiobook,books-ebook,librarry,manual,other" {
		t.Fatalf("unexpected tags: %#v", resources.Tags)
	}
}

func TestTransmissionPreferencesReadAndUpdate(t *testing.T) {
	var sessionGets int
	var setArgs map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		switch payload.Method {
		case "session-get":
			sessionGets++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": "success",
				"arguments": map[string]any{
					"download-dir":             "/downloads/books",
					"incomplete-dir-enabled":   true,
					"incomplete-dir":           "/downloads/incomplete",
					"start-added-torrents":     false,
					"speed-limit-down":         1024,
					"speed-limit-down-enabled": true,
					"speed-limit-up":           512,
					"speed-limit-up-enabled":   false,
					"alt-speed-down":           256,
					"alt-speed-up":             128,
					"alt-speed-time-enabled":   true,
					"download-queue-enabled":   true,
					"download-queue-size":      3,
					"seed-queue-enabled":       true,
					"seed-queue-size":          2,
				},
			})
		case "session-set":
			setArgs = payload.Arguments
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
		default:
			t.Fatalf("unexpected method %s", payload.Method)
		}
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "", "", server.Client())
	preferences, err := client.Preferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if preferences.Client != "Transmission" || preferences.SavePath != "/downloads/books" || !preferences.StartPaused || preferences.DownloadLimit != 1_048_576 || preferences.UploadLimit != 0 || preferences.MaxActiveTorrents != 5 {
		t.Fatalf("unexpected preferences: %+v", preferences)
	}

	savePath := "/downloads/books/audio"
	startPaused := false
	downloadLimit := int64(2_097_152)
	uploadLimit := int64(0)
	queueing := true
	maxDownloads := 4
	maxUploads := 1
	updated, err := client.UpdatePreferences(context.Background(), DownloadPreferencesUpdate{
		SavePath:           &savePath,
		StartPaused:        &startPaused,
		DownloadLimit:      &downloadLimit,
		UploadLimit:        &uploadLimit,
		QueueingEnabled:    &queueing,
		MaxActiveDownloads: &maxDownloads,
		MaxActiveUploads:   &maxUploads,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sessionGets != 2 {
		t.Fatalf("expected initial and refreshed session-get calls, got %d", sessionGets)
	}
	if setArgs["download-dir"] != savePath ||
		setArgs["start-added-torrents"] != true ||
		setArgs["speed-limit-down-enabled"] != true ||
		int64(setArgs["speed-limit-down"].(float64)) != 2048 ||
		setArgs["speed-limit-up-enabled"] != false ||
		setArgs["download-queue-enabled"] != true ||
		int(setArgs["download-queue-size"].(float64)) != maxDownloads ||
		int(setArgs["seed-queue-size"].(float64)) != maxUploads {
		t.Fatalf("unexpected session-set args: %+v", setArgs)
	}
	if updated.LibrarryPreferenceWriteScope != "transmission-session-preferences" {
		t.Fatalf("expected Transmission preference write scope, got %+v", updated)
	}
}

func TestTransmissionPreferencePatchQueueToggleWins(t *testing.T) {
	queueing := false
	maxDownloads := 4
	maxUploads := 2
	patch, err := transmissionPreferencePatch(DownloadPreferencesUpdate{
		QueueingEnabled:    &queueing,
		MaxActiveDownloads: &maxDownloads,
		MaxActiveUploads:   &maxUploads,
	})
	if err != nil {
		t.Fatal(err)
	}
	if patch["download-queue-enabled"] != false || patch["seed-queue-enabled"] != false {
		t.Fatalf("expected explicit queueing=false to win, got %+v", patch)
	}
	if patch["download-queue-size"] != maxDownloads || patch["seed-queue-size"] != maxUploads {
		t.Fatalf("expected queue sizes to be preserved in patch, got %+v", patch)
	}
}

func TestTransmissionCategoryActionRenamesLabelAcrossTorrents(t *testing.T) {
	var methods []string
	var setArgs map[string]any
	getCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, payload.Method)
		switch payload.Method {
		case "torrent-get":
			getCount++
			labels := []string{CategoryBooksEbook, "librarry"}
			if getCount > 1 {
				labels = []string{"books-archive", "librarry"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": "success",
				"arguments": map[string]any{
					"torrents": []map[string]any{{
						"id":          42,
						"hashString":  "abc123",
						"downloadDir": "/downloads/books",
						"labels":      labels,
					}, {
						"id":          43,
						"hashString":  "def456",
						"downloadDir": "/downloads/audio",
						"labels":      []string{CategoryBooksAudiobook, "librarry"},
					}},
				},
			})
		case "torrent-set":
			setArgs = payload.Arguments
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
		default:
			t.Fatalf("unexpected method %s", payload.Method)
		}
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "", "", server.Client())
	result, err := client.CategoryAction(context.Background(), DownloadCategoryActionRequest{
		Action:  "rename",
		Name:    CategoryBooksEbook,
		NewName: "books-archive",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, ",") != "torrent-get,torrent-set,torrent-get" {
		t.Fatalf("unexpected methods: %v", methods)
	}
	if !jsonStringListMatches(setArgs["ids"], []string{"abc123"}) || !jsonStringListMatches(setArgs["labels"], []string{"books-archive", "librarry"}) {
		t.Fatalf("unexpected set args: %#v", setArgs)
	}
	if !result.Applied || result.Resources == nil || result.Message != "updated 1 Transmission torrent" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTransmissionTagActionDeletesLabelsAcrossTorrents(t *testing.T) {
	var setArgs []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		switch payload.Method {
		case "torrent-get":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": "success",
				"arguments": map[string]any{
					"torrents": []map[string]any{{
						"id":         42,
						"hashString": "abc123",
						"labels":     []string{CategoryBooksEbook, "manual", "librarry"},
					}, {
						"id":         43,
						"hashString": "def456",
						"labels":     []string{CategoryBooksAudiobook, "manual"},
					}},
				},
			})
		case "torrent-set":
			setArgs = append(setArgs, payload.Arguments)
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
		default:
			t.Fatalf("unexpected method %s", payload.Method)
		}
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "", "", server.Client())
	result, err := client.TagAction(context.Background(), DownloadTagActionRequest{
		Action: "delete",
		Names:  []string{"manual"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(setArgs) != 2 {
		t.Fatalf("expected two label updates, got %#v", setArgs)
	}
	if !jsonStringListMatches(setArgs[0]["labels"], []string{CategoryBooksEbook, "librarry"}) ||
		!jsonStringListMatches(setArgs[1]["labels"], []string{CategoryBooksAudiobook}) {
		t.Fatalf("unexpected label updates: %#v", setArgs)
	}
	if !result.Applied || result.Message != "updated 2 Transmission torrents" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTransmissionDetailsMapsFilesTrackersAndPeers(t *testing.T) {
	var requestedFields []any
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
		requestedFields, _ = payload.Arguments["fields"].([]any)
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
					"sizeWhenDone":       1000,
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
					"dateCreated":        1_690_000_000,
					"activityDate":       1_700_000_120,
					"labels":             []string{CategoryBooksEbook, "librarry"},
					"creator":            "mktorrent",
					"comment":            "metadata",
					"secondsDownloading": 60,
					"downloadLimit":      2048,
					"downloadLimited":    true,
					"uploadLimit":        512,
					"uploadLimited":      true,
					"pieceSize":          250,
					"pieceCount":         4,
					"files": []map[string]any{
						{"name": "Book.epub", "length": 800, "bytesCompleted": 400},
						{"name": "sample.txt", "length": 200, "bytesCompleted": 0},
					},
					"fileStats": []map[string]any{
						{"bytesCompleted": 400, "wanted": true, "priority": 1},
						{"bytesCompleted": 0, "wanted": false, "priority": 0},
					},
					"trackerStats": []map[string]any{{
						"announce":              "https://tracker.example/announce",
						"announceState":         2,
						"lastAnnounceSucceeded": true,
						"lastAnnouncePeerCount": 12,
						"seederCount":           10,
						"leecherCount":          2,
						"downloadCount":         5,
						"tier":                  0,
						"host":                  "tracker.example",
					}},
					"peers": []map[string]any{{
						"address":           "1.2.3.4",
						"port":              51413,
						"clientName":        "transmission-peer",
						"flagStr":           "D",
						"progress":          0.75,
						"rateToClient":      100,
						"rateToPeer":        40,
						"isEncrypted":       true,
						"isUTP":             true,
						"isDownloadingFrom": true,
					}},
				}},
			},
		})
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "", "", server.Client())
	details, err := client.Details(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !jsonFieldRequested(requestedFields, "files") || !jsonFieldRequested(requestedFields, "trackerStats") || !jsonFieldRequested(requestedFields, "peers") {
		t.Fatalf("expected detail fields, got %#v", requestedFields)
	}
	if details.Status.Client != "Transmission" || details.Status.ID != "abc123" || details.Status.State != "downloading" {
		t.Fatalf("unexpected status: %+v", details.Status)
	}
	if details.Properties.DownloadLimit != 2_097_152 || details.Properties.UploadLimit != 524_288 || details.Properties.PiecesHave != 2 || details.Properties.PiecesTotal != 4 {
		t.Fatalf("unexpected properties: %+v", details.Properties)
	}
	if len(details.Files) != 2 || details.Files[0].Priority != 6 || details.Files[0].Progress != 0.5 || details.Files[1].Priority != 0 {
		t.Fatalf("unexpected files: %+v", details.Files)
	}
	if len(details.Trackers) != 1 || details.Trackers[0].Status != "working" || details.Trackers[0].Seeds != 10 {
		t.Fatalf("unexpected trackers: %+v", details.Trackers)
	}
	if len(details.Peers) != 1 || details.Peers[0].ID != "1.2.3.4:51413" || details.Peers[0].DownloadRate != 100 || !strings.Contains(details.Peers[0].Connection, "encrypted") {
		t.Fatalf("unexpected peers: %+v", details.Peers)
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
			name:    "force start",
			request: DownloadActionRequest{Action: DownloadActionForceStart, IDs: []string{"abc123"}},
			method:  "torrent-start-now",
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
			name:    "queue up",
			request: DownloadActionRequest{Action: DownloadActionIncreasePriority, IDs: []string{"abc123"}},
			method:  "queue-move-up",
		},
		{
			name:    "queue down",
			request: DownloadActionRequest{Action: DownloadActionDecreasePriority, IDs: []string{"abc123"}},
			method:  "queue-move-down",
		},
		{
			name:    "queue top",
			request: DownloadActionRequest{Action: DownloadActionTopPriority, IDs: []string{"abc123"}},
			method:  "queue-move-top",
		},
		{
			name:    "queue bottom",
			request: DownloadActionRequest{Action: DownloadActionBottomPriority, IDs: []string{"abc123"}},
			method:  "queue-move-bottom",
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

func TestTransmissionLabelActionsMutateTorrentLabels(t *testing.T) {
	tests := []struct {
		name       string
		initial    []string
		request    DownloadActionRequest
		wantLabels []string
	}{
		{
			name:       "set category replaces book category label",
			initial:    []string{CategoryBooksEbook, "librarry", "keep"},
			request:    DownloadActionRequest{Action: DownloadActionSetCategory, IDs: []string{"abc123"}, Category: CategoryBooksAudiobook},
			wantLabels: []string{"librarry", "keep", CategoryBooksAudiobook},
		},
		{
			name:       "add tags appends unique labels",
			initial:    []string{CategoryBooksEbook, "librarry"},
			request:    DownloadActionRequest{Action: DownloadActionAddTags, IDs: []string{"abc123"}, Tags: []string{"wanted:1", "librarry"}},
			wantLabels: []string{CategoryBooksEbook, "librarry", "wanted:1"},
		},
		{
			name:       "remove tags removes matching labels",
			initial:    []string{CategoryBooksEbook, "librarry", "wanted:1"},
			request:    DownloadActionRequest{Action: DownloadActionRemoveTags, IDs: []string{"abc123"}, Tags: []string{"wanted:1"}},
			wantLabels: []string{CategoryBooksEbook, "librarry"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var methods []string
			var setArgs map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Method    string         `json:"method"`
					Arguments map[string]any `json:"arguments"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				methods = append(methods, payload.Method)
				switch payload.Method {
				case "torrent-get":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"result": "success",
						"arguments": map[string]any{
							"torrents": []map[string]any{{
								"id":         42,
								"hashString": "abc123",
								"labels":     test.initial,
							}},
						},
					})
				case "torrent-set":
					setArgs = payload.Arguments
					_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
				default:
					t.Fatalf("unexpected method %s", payload.Method)
				}
			}))
			defer server.Close()

			client := NewTransmissionClient(server.URL, "", "", server.Client())
			result, err := client.Action(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(methods, ",") != "torrent-get,torrent-set" {
				t.Fatalf("unexpected methods: %v", methods)
			}
			if !jsonStringListMatches(setArgs["labels"], test.wantLabels) {
				t.Fatalf("expected labels %#v, got %#v", test.wantLabels, setArgs["labels"])
			}
			if !result.Applied || result.Action != normalizeAction(test.request.Action) {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestTransmissionTrackerActionMutatesTrackerList(t *testing.T) {
	tests := []struct {
		name      string
		request   DownloadTrackerActionRequest
		wantList  string
		wantURLs  []string
		wantError string
	}{
		{
			name:     "add tracker to last tier",
			request:  DownloadTrackerActionRequest{Action: DownloadTrackerActionAdd, URLs: []string{"https://tracker.three/announce"}},
			wantList: "https://tracker.one/announce\n\nhttps://tracker.two/announce\nhttps://tracker.three/announce",
			wantURLs: []string{"https://tracker.three/announce"},
		},
		{
			name:     "edit tracker url",
			request:  DownloadTrackerActionRequest{Action: DownloadTrackerActionEdit, OriginalURL: "https://tracker.two/announce", NewURL: "https://tracker.three/announce"},
			wantList: "https://tracker.one/announce\n\nhttps://tracker.three/announce",
			wantURLs: []string{"https://tracker.two/announce", "https://tracker.three/announce"},
		},
		{
			name:     "remove tracker url",
			request:  DownloadTrackerActionRequest{Action: DownloadTrackerActionRemove, URLs: []string{"https://tracker.two/announce"}},
			wantList: "https://tracker.one/announce",
			wantURLs: []string{"https://tracker.two/announce"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var methods []string
			var setArgs map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Method    string         `json:"method"`
					Arguments map[string]any `json:"arguments"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				methods = append(methods, payload.Method)
				switch payload.Method {
				case "torrent-get":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"result": "success",
						"arguments": map[string]any{
							"torrents": []map[string]any{{
								"id":          42,
								"hashString":  "abc123",
								"trackerList": "https://tracker.one/announce\n\nhttps://tracker.two/announce",
							}},
						},
					})
				case "torrent-set":
					setArgs = payload.Arguments
					_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
				default:
					t.Fatalf("unexpected method %s", payload.Method)
				}
			}))
			defer server.Close()

			client := NewTransmissionClient(server.URL, "", "", server.Client())
			result, err := client.TrackerAction(context.Background(), "abc123", test.request)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("expected error containing %q, got %v", test.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(methods, ",") != "torrent-get,torrent-set" {
				t.Fatalf("unexpected methods: %v", methods)
			}
			if setArgs["trackerList"] != test.wantList {
				t.Fatalf("expected trackerList %q, got %#v", test.wantList, setArgs["trackerList"])
			}
			if !stringSlicesEqual(result.URLs, test.wantURLs) {
				t.Fatalf("expected urls %#v, got %#v", test.wantURLs, result.URLs)
			}
			if !result.Applied || result.DownloadID != "abc123" || result.Action != normalizeTrackerAction(test.request.Action) {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestTransmissionFileActionSetsWantedAndPriority(t *testing.T) {
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
	result, err := client.FileAction(context.Background(), DownloadFileActionRequest{
		DownloadID: "abc123",
		Action:     DownloadFileActionHigh,
		IDs:        []int{0, 2, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != "torrent-set" {
		t.Fatalf("expected torrent-set, got %s", method)
	}
	if !jsonNumberListMatches(args["files-wanted"], []int{0, 2}) || !jsonNumberListMatches(args["priority-high"], []int{0, 2}) {
		t.Fatalf("unexpected file priority args: %#v", args)
	}
	if !result.Applied || result.Action != DownloadFileActionHigh || result.Priority != 6 {
		t.Fatalf("unexpected file action result: %+v", result)
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

func TestServiceRoutesExplicitTransmissionDetailsAndFileAction(t *testing.T) {
	var methods []string
	var fileActionArgs map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, payload.Method)
		switch payload.Method {
		case "torrent-get":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": "success",
				"arguments": map[string]any{
					"torrents": []map[string]any{{
						"id":           42,
						"hashString":   "abc123",
						"name":         "Book.epub",
						"status":       4,
						"percentDone":  0.5,
						"downloadDir":  "/downloads/books",
						"totalSize":    1000,
						"sizeWhenDone": 1000,
						"files":        []map[string]any{{"name": "Book.epub", "length": 1000, "bytesCompleted": 500}},
						"fileStats":    []map[string]any{{"bytesCompleted": 500, "wanted": true, "priority": 0}},
					}},
				},
			})
		case "torrent-set":
			fileActionArgs = payload.Arguments
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
		default:
			t.Fatalf("unexpected method %s", payload.Method)
		}
	}))
	defer server.Close()

	service := NewService(IntegrationConfig{
		QBittorrentURL:  "http://qbittorrent.invalid",
		TransmissionURL: server.URL,
	})
	details, err := service.DownloadDetails(context.Background(), "abc123", "Transmission")
	if err != nil {
		t.Fatal(err)
	}
	if details.Status.Client != "Transmission" || len(details.Files) != 1 {
		t.Fatalf("unexpected details: %+v", details)
	}
	result, err := service.DownloadFileAction(context.Background(), "abc123", DownloadFileActionRequest{
		Client: "Transmission",
		Action: DownloadFileActionSkip,
		IDs:    []int{0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, ",") != "torrent-get,torrent-set,torrent-get" {
		t.Fatalf("unexpected service routing methods: %v", methods)
	}
	if !jsonNumberListMatches(fileActionArgs["files-unwanted"], []int{0}) {
		t.Fatalf("expected files-unwanted action, got %#v", fileActionArgs)
	}
	if !result.Applied || result.Download == nil || result.Download.Status.Client != "Transmission" {
		t.Fatalf("unexpected file action result: %+v", result)
	}
}

func TestServiceRoutesExplicitTransmissionTrackerAction(t *testing.T) {
	var methods []string
	var trackerSetArgs map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, payload.Method)
		switch payload.Method {
		case "torrent-get":
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
						"sizeWhenDone":       1000,
						"trackerList":        "https://tracker.one/announce",
						"trackers":           []map[string]any{{"announce": "https://tracker.one/announce", "tier": 0}},
						"trackerStats":       []map[string]any{{"announce": "https://tracker.one/announce", "tier": 0, "lastAnnounceSucceeded": true}},
						"peersConnected":     1,
						"peersGettingFromUs": 1,
					}},
				},
			})
		case "torrent-set":
			trackerSetArgs = payload.Arguments
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": map[string]any{}})
		default:
			t.Fatalf("unexpected method %s", payload.Method)
		}
	}))
	defer server.Close()

	service := NewService(IntegrationConfig{
		QBittorrentURL:  "http://qbittorrent.invalid",
		TransmissionURL: server.URL,
	})
	result, err := service.DownloadTrackerAction(context.Background(), "abc123", DownloadTrackerActionRequest{
		Client: "Transmission",
		Action: DownloadTrackerActionAdd,
		URLs:   []string{"https://tracker.two/announce"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, ",") != "torrent-get,torrent-set,torrent-get" {
		t.Fatalf("unexpected service routing methods: %v", methods)
	}
	if trackerSetArgs["trackerList"] != "https://tracker.one/announce\nhttps://tracker.two/announce" {
		t.Fatalf("unexpected trackerList: %#v", trackerSetArgs["trackerList"])
	}
	if !result.Applied || result.Download == nil || result.Download.Status.Client != "Transmission" {
		t.Fatalf("unexpected tracker action result: %+v", result)
	}
}

func jsonFieldRequested(fields []any, field string) bool {
	for _, value := range fields {
		if text, ok := value.(string); ok && text == field {
			return true
		}
	}
	return false
}

func jsonNumberListMatches(value any, want []int) bool {
	values, ok := value.([]any)
	if !ok || len(values) != len(want) {
		return false
	}
	for i, item := range values {
		if int(item.(float64)) != want[i] {
			return false
		}
	}
	return true
}

func jsonStringListMatches(value any, want []string) bool {
	values, ok := value.([]any)
	if !ok || len(values) != len(want) {
		return false
	}
	for i, item := range values {
		if item != want[i] {
			return false
		}
	}
	return true
}

func stringSlicesEqual(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
