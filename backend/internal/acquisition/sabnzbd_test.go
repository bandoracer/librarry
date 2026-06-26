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

func TestSABnzbdDetailsMapsQueueFiles(t *testing.T) {
	var modes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		modes = append(modes, query.Get("mode"))
		switch query.Get("mode") {
		case "queue":
			if query.Get("nzo_ids") != "SABnzbd_nzo_downloading" {
				t.Fatalf("expected filtered queue lookup, got %#v", query)
			}
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
						"time_added": 1_700_000_000,
						"priority":   "Normal",
						"script":     "Default",
					}},
				},
			})
		case "get_files":
			if query.Get("value") != "SABnzbd_nzo_downloading" {
				t.Fatalf("expected get_files value, got %#v", query)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"files": []map[string]any{{
					"status":   "finished",
					"mbleft":   "0.00",
					"mb":       "10.00",
					"bytes":    "10485760",
					"filename": "Dungeon Crawler Carl.m4b",
					"nzf_id":   "SABnzbd_nzf_audio",
				}, {
					"status":   "queued",
					"mbleft":   "3.13",
					"mb":       "3.13",
					"bytes":    "3282042",
					"filename": "repair.par2",
					"nzf_id":   "SABnzbd_nzf_par2",
				}},
			})
		default:
			t.Fatalf("unexpected mode %q", query.Get("mode"))
		}
	}))
	defer server.Close()

	client := NewSABnzbdClient(server.URL, "api-key", "", "", server.Client())
	details, err := client.Details(context.Background(), "SABnzbd_nzo_downloading")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(modes, ",") != "queue,get_files" {
		t.Fatalf("unexpected API modes: %v", modes)
	}
	if details.Status.Client != "SABnzbd" || details.Status.ID != "SABnzbd_nzo_downloading" || details.Status.State != "downloading" {
		t.Fatalf("unexpected status: %+v", details.Status)
	}
	if details.Properties.TotalSizeBytes != 1_073_741_824 || details.Properties.TotalDownloaded != 268_435_456 || details.Properties.ETASeconds != 605 || details.Properties.AdditionDate == nil {
		t.Fatalf("unexpected properties: %+v", details.Properties)
	}
	if len(details.Files) != 2 || details.Files[0].ExternalID != "SABnzbd_nzf_audio" || details.Files[0].Progress != 1 || details.Files[1].Priority != -1 {
		t.Fatalf("unexpected files: %+v", details.Files)
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

func TestSABnzbdJobManagementActions(t *testing.T) {
	var commands []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		commands = append(commands, query.Get("mode")+":"+query.Get("name")+":"+query.Get("value")+":"+query.Get("value2"))
		_ = json.NewEncoder(w).Encode(map[string]any{"status": true})
	}))
	defer server.Close()

	client := NewSABnzbdClient(server.URL, "api-key", "", "", server.Client())
	tests := []DownloadActionRequest{
		{Action: DownloadActionSetCategory, IDs: []string{"SABnzbd_nzo_abc123"}, Category: CategoryBooksAudiobook},
		{Action: DownloadActionRename, IDs: []string{"SABnzbd_nzo_abc123"}, Name: "Dungeon Crawler Carl"},
		{Action: DownloadActionIncreasePriority, IDs: []string{"SABnzbd_nzo_abc123"}},
		{Action: DownloadActionBottomPriority, IDs: []string{"SABnzbd_nzo_abc123"}},
	}
	for _, request := range tests {
		if _, err := client.Action(context.Background(), request); err != nil {
			t.Fatalf("%s failed: %v", request.Action, err)
		}
	}
	want := strings.Join([]string{
		"change_cat::SABnzbd_nzo_abc123:" + CategoryBooksAudiobook,
		"queue:rename:SABnzbd_nzo_abc123:Dungeon Crawler Carl",
		"queue:priority:SABnzbd_nzo_abc123:1",
		"queue:priority:SABnzbd_nzo_abc123:-1",
	}, "|")
	if strings.Join(commands, "|") != want {
		t.Fatalf("unexpected commands:\nwant %s\n got %s", want, strings.Join(commands, "|"))
	}
}

func TestSABnzbdResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("mode") {
		case "get_cats":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"categories": []string{CategoryBooksEbook, CategoryBooksAudiobook, CategoryBooksEbook, "*"},
			})
		case "get_config":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"categories": map[string]any{
					"*":                       map[string]any{"dir": ""},
					CategoryBooksAudiobook:    map[string]any{"dir": "/downloads/audiobooks"},
					CategoryBooksEbook:        map[string]any{"dir": "/downloads/ebooks"},
					"ignored-without-dir-key": map[string]any{"name": "ignored-without-dir-key"},
				},
			})
		default:
			t.Fatalf("unexpected mode %q", r.URL.Query().Get("mode"))
		}
	}))
	defer server.Close()

	client := NewSABnzbdClient(server.URL, "api-key", "", "", server.Client())
	resources, err := client.Resources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resources.Client != "SABnzbd" || len(resources.Categories) != 3 || len(resources.Tags) != 0 {
		t.Fatalf("unexpected resources: %+v", resources)
	}
	if resources.Categories[0].Name != "*" || resources.Categories[1].Name != CategoryBooksAudiobook || resources.Categories[2].Name != CategoryBooksEbook {
		t.Fatalf("unexpected category order: %+v", resources.Categories)
	}
	if resources.Categories[1].SavePath != "/downloads/audiobooks" || resources.Categories[2].SavePath != "/downloads/ebooks" {
		t.Fatalf("expected category save paths, got %+v", resources.Categories)
	}
}

func TestSABnzbdCategoryActions(t *testing.T) {
	var commands []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch query.Get("mode") {
		case "set_config":
			commands = append(commands, "set_config:"+query.Get("name")+":"+query.Get("dir"))
			_, _ = w.Write([]byte("true"))
		case "del_config":
			commands = append(commands, "del_config:"+query.Get("keyword"))
			_, _ = w.Write([]byte("true"))
		case "get_cats":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"categories": []string{"*", CategoryBooksAudiobook, CategoryBooksEbook},
			})
		case "get_config":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"categories": map[string]any{
					CategoryBooksAudiobook: map[string]any{"dir": "/downloads/audiobooks"},
					CategoryBooksEbook:     map[string]any{"dir": "/downloads/ebooks"},
				},
			})
		default:
			t.Fatalf("unexpected mode %q", query.Get("mode"))
		}
	}))
	defer server.Close()

	client := NewSABnzbdClient(server.URL, "api-key", "", "", server.Client())
	result, err := client.CategoryAction(context.Background(), DownloadCategoryActionRequest{
		Action:   "create",
		Name:     CategoryBooksEbook,
		SavePath: "/downloads/ebooks",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Client != "SABnzbd" || result.Resources == nil {
		t.Fatalf("unexpected create result: %+v", result)
	}
	if _, err := client.CategoryAction(context.Background(), DownloadCategoryActionRequest{
		Action:   "edit",
		Name:     CategoryBooksEbook,
		NewName:  CategoryBooksAudiobook,
		SavePath: "/downloads/audiobooks",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CategoryAction(context.Background(), DownloadCategoryActionRequest{
		Action: "delete",
		Name:   "old-books",
	}); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"set_config:" + CategoryBooksEbook + ":/downloads/ebooks",
		"set_config:" + CategoryBooksAudiobook + ":/downloads/audiobooks",
		"del_config:" + CategoryBooksEbook,
		"del_config:old-books",
	}, "|")
	if strings.Join(commands, "|") != want {
		t.Fatalf("unexpected category commands:\nwant %s\n got %s", want, strings.Join(commands, "|"))
	}
}

func TestServiceRoutesSABnzbdResources(t *testing.T) {
	var commands []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch query.Get("mode") {
		case "get_cats":
			_ = json.NewEncoder(w).Encode(map[string]any{"categories": []string{"*", CategoryBooksEbook}})
		case "get_config":
			_ = json.NewEncoder(w).Encode(map[string]any{"categories": map[string]any{CategoryBooksEbook: map[string]any{"dir": "/downloads/ebooks"}}})
		case "set_config":
			commands = append(commands, query.Get("section")+":"+query.Get("name")+":"+query.Get("dir"))
			_ = json.NewEncoder(w).Encode(map[string]any{"status": true})
		default:
			t.Fatalf("unexpected mode %q", query.Get("mode"))
		}
	}))
	defer server.Close()

	service := NewService(IntegrationConfig{
		SABnzbdURL:    server.URL,
		SABnzbdAPIKey: "api-key",
	})
	resources, err := service.DownloadResources(context.Background(), "SABnzbd")
	if err != nil {
		t.Fatal(err)
	}
	if resources.Client != "SABnzbd" || len(resources.Categories) != 2 {
		t.Fatalf("unexpected service resources: %+v", resources)
	}
	result, err := service.DownloadCategoryAction(context.Background(), DownloadCategoryActionRequest{
		Client:   "SABnzbd",
		Action:   "create",
		Name:     CategoryBooksEbook,
		SavePath: "/downloads/ebooks",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Resources == nil {
		t.Fatalf("unexpected service action result: %+v", result)
	}
	if strings.Join(commands, "|") != "categories:"+CategoryBooksEbook+":/downloads/ebooks" {
		t.Fatalf("unexpected service category commands: %v", commands)
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

func TestServiceRejectsTorrentUploadToSABnzbd(t *testing.T) {
	service := NewService(IntegrationConfig{SABnzbdURL: "http://sabnzbd.example", SABnzbdAPIKey: "api-key"})
	_, err := service.Grab(context.Background(), DownloadRequest{
		Client:     "SABnzbd",
		UploadName: "book.torrent",
		UploadData: []byte("torrent-bytes"),
	})
	if err == nil || !strings.Contains(err.Error(), "torrent uploads require") {
		t.Fatalf("expected torrent upload rejection, got %v", err)
	}
}

func TestServiceRoutesExplicitSABnzbdDetailsAndAction(t *testing.T) {
	var commands []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		commandValue := firstNonEmpty(query.Get("value"), query.Get("nzo_ids"))
		commands = append(commands, query.Get("mode")+":"+query.Get("name")+":"+commandValue+":"+query.Get("value2"))
		switch query.Get("mode") {
		case "queue":
			if query.Get("name") == "rename" {
				_ = json.NewEncoder(w).Encode(map[string]any{"status": true})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"queue": map[string]any{
					"slots": []map[string]any{{
						"nzo_id":     "SABnzbd_nzo_downloading",
						"filename":   "Project Hail Mary.epub",
						"status":     "Downloading",
						"cat":        CategoryBooksEbook,
						"percentage": "50",
						"size":       "100 MB",
						"mbleft":     "50",
					}},
				},
			})
		case "get_files":
			_ = json.NewEncoder(w).Encode(map[string]any{"files": []any{}})
		case "history":
			_ = json.NewEncoder(w).Encode(map[string]any{"history": map[string]any{"slots": []any{}}})
		default:
			t.Fatalf("unexpected mode %q", query.Get("mode"))
		}
	}))
	defer server.Close()

	service := NewService(IntegrationConfig{
		SABnzbdURL:    server.URL,
		SABnzbdAPIKey: "api-key",
	})
	details, err := service.DownloadDetails(context.Background(), "SABnzbd_nzo_downloading", "SABnzbd")
	if err != nil {
		t.Fatal(err)
	}
	if details.Status.Client != "SABnzbd" || details.Status.Progress != 0.5 {
		t.Fatalf("unexpected details: %+v", details)
	}
	result, err := service.DownloadAction(context.Background(), DownloadActionRequest{
		Client: "SABnzbd",
		Action: DownloadActionRename,
		IDs:    []string{"SABnzbd_nzo_downloading"},
		Name:   "Project Hail Mary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || strings.Join(result.IDs, ",") != "SABnzbd_nzo_downloading" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if strings.Join(commands, "|") != "queue::SABnzbd_nzo_downloading:|get_files::SABnzbd_nzo_downloading:|queue:rename:SABnzbd_nzo_downloading:Project Hail Mary|queue::SABnzbd_nzo_downloading:|history::SABnzbd_nzo_downloading:" {
		t.Fatalf("unexpected service commands: %v", commands)
	}
}

func queryValues(r *http.Request) map[string]string {
	values := map[string]string{}
	for key, list := range r.URL.Query() {
		values[key] = firstString(list)
	}
	return values
}
