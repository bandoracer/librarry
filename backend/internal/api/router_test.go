package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestHealthEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"service":"librarry"`) {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
}

func TestCompatSystemStatusAndPingEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*", DatabaseURL: "postgres://librarry:librarry@localhost/librarry"},
		Metadata: metadata.NewService(nil),
	})

	pingReq := httptest.NewRequest(http.MethodGet, "/ping", nil)
	pingRes := httptest.NewRecorder()
	router.ServeHTTP(pingRes, pingReq)
	if pingRes.Code != http.StatusOK || strings.TrimSpace(pingRes.Body.String()) != "pong" {
		t.Fatalf("unexpected ping response: %d %q", pingRes.Code, pingRes.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"appName":"Librarry"`) || !strings.Contains(res.Body.String(), `"databaseType":"postgres"`) {
		t.Fatalf("expected Readarr-style status payload, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/routes", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `/api/v1/queue`) {
		t.Fatalf("expected route list payload, got %s", res.Body.String())
	}
}

func TestCompatRootFolderAndDiskspaceEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger: slog.Default(),
		Config: config.Config{
			WebOrigin:            "*",
			EbookLibraryRoot:     "/tmp",
			AudiobookLibraryRoot: "/tmp",
		},
		Metadata: metadata.NewService(nil),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rootfolder", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"path":"/tmp"`) || !strings.Contains(res.Body.String(), `"freeSpace"`) {
		t.Fatalf("expected root folder records, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/diskspace", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"totalSpace"`) {
		t.Fatalf("expected diskspace records, got %s", res.Body.String())
	}
}

func TestCompatQueueEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/queue?page=1&pageSize=1", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"totalRecords":1`) || !strings.Contains(res.Body.String(), `"downloadClient":"qBittorrent"`) {
		t.Fatalf("expected paged queue payload, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/queue/status", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"totalCount":1`) {
		t.Fatalf("expected queue status payload, got %s", res.Body.String())
	}
}

func TestCompatWantedMissingAndQualityProfiles(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wanted/missing", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"totalRecords":1`) || !strings.Contains(res.Body.String(), `"librarryId":"wanted-1"`) {
		t.Fatalf("expected missing book payload, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/qualityprofile", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"upgradeAllowed":true`) || !strings.Contains(res.Body.String(), `"librarry"`) {
		t.Fatalf("expected quality profile payload, got %s", res.Body.String())
	}
}

func TestCompatDownloadClientIndexerAndCommandEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger: slog.Default(),
		Config: config.Config{
			WebOrigin:          "*",
			ProwlarrURL:        "http://prowlarr.local",
			ProwlarrAPIKey:     "secret",
			QBittorrentURL:     "http://qbittorrent.local",
			SABnzbdURL:         "http://sabnzbd.local",
			EbookCategory:      "books-ebook",
			AudiobookCategory:  "books-audiobook",
			EbookLibraryRoot:   "/tmp",
			BookTorrentRoot:    "/downloads/books",
			GoogleBooksAPIKey:  "google",
			HardcoverToken:     "hardcover",
			MigrationsDir:      "backend/migrations",
			NamingAuthorFolder: "{Author}",
		},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
		Wanted:   fakeWanted{},
		Library:  fakeLibrary{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/downloadclient", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"name":"qBittorrent"`) || !strings.Contains(res.Body.String(), `"name":"SABnzbd"`) {
		t.Fatalf("expected download clients, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/indexer", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"name":"Prowlarr"`) || strings.Contains(res.Body.String(), "secret") {
		t.Fatalf("expected masked Prowlarr indexer, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/command", strings.NewReader(`{"name":"RssSync"}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"commandName":"RssSync"`) || !strings.Contains(res.Body.String(), `"status":"completed"`) {
		t.Fatalf("expected command result, got %s", res.Body.String())
	}
}

func TestCompatAuthorAndBookEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/author", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"authorName":"Andy Weir"`) || !strings.Contains(res.Body.String(), `"books"`) {
		t.Fatalf("expected author payload, got %s", res.Body.String())
	}

	authorID := stableInt("author-sub-1")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/author/"+strconv.Itoa(authorID), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"foreignAuthorId":"openlibrary:OL123A"`) {
		t.Fatalf("expected single author payload, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/book", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"title":"Project Hail Mary"`) || !strings.Contains(res.Body.String(), `"authorTitle":"Andy Weir"`) {
		t.Fatalf("expected book payload, got %s", res.Body.String())
	}

	bookID := stableInt("wanted-1")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/book/"+strconv.Itoa(bookID), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"librarryId":"wanted-1"`) {
		t.Fatalf("expected single book payload, got %s", res.Body.String())
	}
}

func TestCompatAuthorAndBookCreateEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/author", strings.NewReader(`{"authorName":"Andy Weir","foreignAuthorId":"ol:andy","monitored":true}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"authorName":"Andy Weir"`) {
		t.Fatalf("expected created author payload, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/book", strings.NewReader(`{"title":"Project Hail Mary","authorTitle":"Andy Weir","foreignBookId":"ol:work"}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"title":"Project Hail Mary"`) || !strings.Contains(res.Body.String(), `"authorTitle":"Andy Weir"`) {
		t.Fatalf("expected created book payload, got %s", res.Body.String())
	}
}

func TestCompatAuthorAndBookLookupEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService([]metadata.Provider{fakeMetadataProvider{}}),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/author/lookup?term=Andy%20Weir", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"authorName":"Andy Weir"`) || !strings.Contains(res.Body.String(), `"foreignAuthorId":"openlibrary:OL123A"`) {
		t.Fatalf("expected author lookup payload, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/book/lookup?term=Project%20Hail%20Mary", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"title":"Project Hail Mary"`) || !strings.Contains(res.Body.String(), `"foreignBookId":"openlibrary:OL1W"`) {
		t.Fatalf("expected book lookup payload, got %s", res.Body.String())
	}
}

func TestCompatManualImportEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
		Library:  fakeLibrary{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manualimport?downloadId=abc123", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"librarryReviewId":"review-1"`) || !strings.Contains(res.Body.String(), `"downloadId":"abc123"`) {
		t.Fatalf("expected pending import review, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/manualimport?folder=/downloads/books", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"librarrySource":"folderScan"`) || !strings.Contains(res.Body.String(), `"path":"`) {
		t.Fatalf("expected scanned import candidate, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/manualimport", strings.NewReader(`[{"path":"/downloads/Project Hail Mary.epub","wantedId":"wanted-1","downloadId":"abc123","importMode":"copy","mediaFormat":"ebook"}]`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"imported":true`) || !strings.Contains(res.Body.String(), `"destinationPath":"`) {
		t.Fatalf("expected imported manual import payload, got %s", res.Body.String())
	}
}

func TestCompatConfigEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger: slog.Default(),
		Config: config.Config{
			WebOrigin:              "*",
			EbookLibraryRoot:       "/media/books/ebooks",
			AudiobookLibraryRoot:   "/media/books/audiobooks",
			NamingAuthorFolder:     "{Author}",
			NamingBookFolder:       "{Title} ({Format})",
			NamingFileName:         "{Author} - {Title}{Ext}",
			NamingSpaceReplacement: "_",
		},
		Metadata: metadata.NewService(nil),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/naming", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"standardBookFormat":"{Author} - {Title}{Ext}"`) || !strings.Contains(res.Body.String(), `"replaceSpacesWith":"_"`) {
		t.Fatalf("expected naming config, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/config/naming/examples", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Andy_Weir") || !strings.Contains(res.Body.String(), "Project_Hail_Mary") {
		t.Fatalf("expected naming example, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/config/naming/1", strings.NewReader(`{"standardBookFormat":"{Title}{Ext}","replaceSpaces":false}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"standardBookFormat":"{Title}{Ext}"`) || !strings.Contains(res.Body.String(), `"replaceSpaces":false`) {
		t.Fatalf("expected echoed naming update, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/config/mediamanagement", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"/media/books/ebooks"`) || !strings.Contains(res.Body.String(), `"/media/books/audiobooks"`) {
		t.Fatalf("expected media management roots, got %s", res.Body.String())
	}
}

func TestSettingsValidateEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/validate", strings.NewReader(`{"prowlarrUrl":"http://localhost"}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Prowlarr URL and API key") {
		t.Fatalf("expected validation error, got %s", res.Body.String())
	}
}

func TestDownloadActionEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/downloads/actions", strings.NewReader(`{"action":"stop","ids":["abc123"]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload acquisition.DownloadActionResult
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Action != acquisition.DownloadActionStop || !payload.Applied || payload.IDs[0] != "abc123" {
		t.Fatalf("unexpected action payload: %+v", payload)
	}
}

func TestDownloadRebalanceEndpointStartsPausedDownloads(t *testing.T) {
	now := time.Now().UTC()
	acquire := &fakeRebalanceAcquire{downloads: []acquisition.DownloadStatus{
		{ID: "active-1", Name: "Active", State: "downloading", Progress: 0.4, AddedAt: &now},
		{ID: "paused-low", Name: "Paused Low", State: "pausedDL", Progress: 0.1, AddedAt: &now},
		{ID: "paused-high", Name: "Paused High", State: "stoppedDL", Progress: 0.8, AddedAt: &now},
		{ID: "done-1", Name: "Done", State: "uploading", Progress: 1, AddedAt: &now},
	}}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  acquire,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/downloads/rebalance", strings.NewReader(`{"maxActive":2,"dryRun":false}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload downloadRebalancePlan
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Applied || payload.StartIDs[0] != "paused-high" || len(payload.StopIDs) != 0 {
		t.Fatalf("unexpected rebalance payload: %+v", payload)
	}
	if len(acquire.actions) != 1 || acquire.actions[0].Action != acquisition.DownloadActionStart || acquire.actions[0].IDs[0] != "paused-high" {
		t.Fatalf("expected start action for paused-high, got %+v", acquire.actions)
	}
}

func TestDownloadRebalanceEndpointDryRunStopsOverflow(t *testing.T) {
	now := time.Now().UTC()
	acquire := &fakeRebalanceAcquire{downloads: []acquisition.DownloadStatus{
		{ID: "active-high", Name: "Active High", State: "downloading", Progress: 0.9, AddedAt: &now},
		{ID: "active-low", Name: "Active Low", State: "stalledDL", Progress: 0.2, AddedAt: &now},
	}}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  acquire,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/downloads/rebalance", strings.NewReader(`{"maxActive":1,"stopOverflow":true,"dryRun":true}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload downloadRebalancePlan
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Applied || payload.StopIDs[0] != "active-low" || len(acquire.actions) != 0 {
		t.Fatalf("unexpected dry-run payload/actions: %+v actions=%+v", payload, acquire.actions)
	}
}

func TestListWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wanted", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Project Hail Mary") {
		t.Fatalf("expected wanted item in response, got %s", res.Body.String())
	}
}

func TestGrabWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted/wanted-1/grab", strings.NewReader(`{"releaseId":"release-1","paused":true}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "download-1") {
		t.Fatalf("expected download status in response, got %s", res.Body.String())
	}
}

func TestMonitorWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted/monitor", strings.NewReader(`{"force":true,"limit":5}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"wantedChecked":1`) {
		t.Fatalf("expected monitor run in response, got %s", res.Body.String())
	}
}

func TestAuthorSubscriptionsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/authors?status=monitored", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Andy Weir") {
		t.Fatalf("expected author subscription in response, got %s", res.Body.String())
	}
}

func TestSubscribeAuthorEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/authors", strings.NewReader(`{"authorName":"Andy Weir","provider":"Open Library","providerKey":"openlibrary:OL123A","format":"ebook","qualityProfile":"standard","monitorNewItems":true}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"status":"monitored"`) {
		t.Fatalf("expected saved author subscription in response, got %s", res.Body.String())
	}
}

func TestMonitorAuthorsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/authors/monitor", strings.NewReader(`{"force":true,"limit":5}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"authorsChecked":1`) {
		t.Fatalf("expected author monitor run in response, got %s", res.Body.String())
	}
}

func TestFeedSyncWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted/feed-sync", strings.NewReader(`{"limit":10}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"releasesSeen":2`) {
		t.Fatalf("expected feed sync run in response, got %s", res.Body.String())
	}
}

func TestRecoverFailedDownloadsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/downloads/recover-failed", strings.NewReader(`{"autoGrab":true,"downloadIds":["abc123"]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"failedCount":1`) {
		t.Fatalf("expected failed download run in response, got %s", res.Body.String())
	}
}

func TestUpgradeWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted/upgrades", strings.NewReader(`{"autoGrab":true,"force":true}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"upgradeCount":1`) {
		t.Fatalf("expected upgrade run in response, got %s", res.Body.String())
	}
}

func TestCompatHistoryCalendarAndParseEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/history?limit=5", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"records"`) || !strings.Contains(res.Body.String(), `"librarryEventType":"wanted_searched"`) {
		t.Fatalf("expected Readarr history records in response, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/history/since?date=2000-01-01", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"eventType":"bookSearch"`) {
		t.Fatalf("expected Readarr history since records in response, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/calendar", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"title":"Project Hail Mary"`) || !strings.Contains(res.Body.String(), `"airDate"`) {
		t.Fatalf("expected calendar records in response, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/parse?title=Andy%20Weir%20-%20Project%20Hail%20Mary", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"parsedTitle":"Project Hail Mary"`) || !strings.Contains(res.Body.String(), `"authorTitle":"Andy Weir"`) {
		t.Fatalf("expected parse payload in response, got %s", res.Body.String())
	}
}

func TestNativeHistoryEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/librarry/history?limit=5", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"events"`) || !strings.Contains(res.Body.String(), "wanted_searched") {
		t.Fatalf("expected native history event in response, got %s", res.Body.String())
	}
}

func TestQualityProfilesEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/quality-profiles", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"minSeeders":2`) {
		t.Fatalf("expected profile payload in response, got %s", res.Body.String())
	}
}

func TestSaveQualityProfileEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quality-profiles", strings.NewReader(`{"name":"retail","mediaFormat":"ebook","minScore":70,"cutoffScore":92,"minSeeders":3,"rejectedTerms":["summary"]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"name":"retail"`) {
		t.Fatalf("expected saved profile in response, got %s", res.Body.String())
	}
}

func TestLibraryFilesEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/library/files?format=ebook", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Project Hail Mary") {
		t.Fatalf("expected file in response, got %s", res.Body.String())
	}
}

func TestScanLibraryEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/scan", strings.NewReader(`{"format":"ebook"}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"upserted":1`) {
		t.Fatalf("expected scan outcome in response, got %s", res.Body.String())
	}
}

func TestImportLibraryFileEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/import", strings.NewReader(`{"sourcePath":"/downloads/book.epub","wantedId":"wanted-1"}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "/library/ebooks") {
		t.Fatalf("expected import outcome in response, got %s", res.Body.String())
	}
}

func TestImportCompletedDownloadsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/import-completed", strings.NewReader(`{"downloadIds":["abc123"]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"imported":1`) {
		t.Fatalf("expected completed import outcome in response, got %s", res.Body.String())
	}
}

func TestImportReviewsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/library/import-reviews?status=pending", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"reason":"download is not linked to a wanted item"`) {
		t.Fatalf("expected review list in response, got %s", res.Body.String())
	}
}

func TestResolveImportReviewEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/import-reviews/review-1/resolve", strings.NewReader(`{"action":"import","wantedId":"wanted-1"}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"status":"imported"`) {
		t.Fatalf("expected review resolution in response, got %s", res.Body.String())
	}
}

type fakeAcquire struct{}

func (fakeAcquire) Health(context.Context) []acquisition.IntegrationHealth {
	return nil
}

func (fakeAcquire) Bootstrap(context.Context) (acquisition.BootstrapResult, error) {
	return acquisition.BootstrapResult{}, nil
}

func (fakeAcquire) Search(context.Context, acquisition.ReleaseSearchQuery) ([]acquisition.Release, error) {
	return nil, nil
}

func (fakeAcquire) Grab(context.Context, acquisition.DownloadRequest) (acquisition.DownloadStatus, error) {
	return acquisition.DownloadStatus{}, nil
}

func (fakeAcquire) Downloads(context.Context, acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error) {
	now := time.Now().UTC()
	return []acquisition.DownloadStatus{{
		ID:          "abc123",
		Name:        "Project Hail Mary.epub",
		State:       "uploading",
		Progress:    1,
		SavePath:    "/downloads",
		Category:    "books-ebook",
		Tags:        []string{"librarry", "wanted:wanted-1"},
		CompletedAt: &now,
	}}, nil
}

func (fakeAcquire) DownloadAction(_ context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error) {
	return acquisition.DownloadActionResult{
		Action:  acquisition.DownloadActionStop,
		IDs:     request.IDs,
		Applied: true,
	}, nil
}

type fakeRebalanceAcquire struct {
	fakeAcquire
	downloads []acquisition.DownloadStatus
	actions   []acquisition.DownloadActionRequest
}

func (f *fakeRebalanceAcquire) Downloads(context.Context, acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error) {
	return f.downloads, nil
}

func (f *fakeRebalanceAcquire) DownloadAction(_ context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error) {
	f.actions = append(f.actions, request)
	return acquisition.DownloadActionResult{
		Action:  request.Action,
		IDs:     request.IDs,
		Applied: true,
	}, nil
}

type fakeWanted struct{}

func (fakeWanted) Create(context.Context, wanted.CreateRequest) (wanted.WantedItem, error) {
	return wanted.WantedItem{
		ID:             "wanted-1",
		WorkID:         "openlibrary:OL1W",
		Title:          "Project Hail Mary",
		AuthorName:     "Andy Weir",
		Format:         "ebook",
		QualityProfile: "standard",
		Status:         "wanted",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}, nil
}

func (fakeWanted) List(context.Context, string) ([]wanted.WantedItem, error) {
	return []wanted.WantedItem{{
		ID:             "wanted-1",
		WorkID:         "openlibrary:OL1W",
		Title:          "Project Hail Mary",
		AuthorName:     "Andy Weir",
		Format:         "ebook",
		QualityProfile: "standard",
		Status:         "wanted",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}}, nil
}

func (fakeWanted) ListQualityProfiles(context.Context) ([]wanted.QualityProfile, error) {
	return []wanted.QualityProfile{{
		ID:             "profile-1",
		Name:           "standard",
		MediaFormat:    "ebook",
		MinScore:       60,
		CutoffScore:    85,
		MinSeeders:     2,
		MaxSizeBytes:   786432000,
		PreferredTerms: []string{"epub"},
		RejectedTerms:  []string{"summary", "review"},
		PreferredScore: 8,
		UpgradeAllowed: true,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}}, nil
}

func (fakeWanted) SaveQualityProfile(_ context.Context, profile wanted.QualityProfile) (wanted.QualityProfile, error) {
	profile.ID = "profile-2"
	profile.CreatedAt = time.Now().UTC()
	profile.UpdatedAt = profile.CreatedAt
	return profile, nil
}

func (fakeWanted) SubscribeAuthor(_ context.Context, request wanted.AuthorSubscribeRequest) (wanted.AuthorSubscription, error) {
	return wanted.AuthorSubscription{
		ID:              "author-sub-1",
		Provider:        request.Provider,
		ProviderKey:     request.ProviderKey,
		AuthorName:      request.AuthorName,
		Format:          "ebook",
		QualityProfile:  "standard",
		Status:          "monitored",
		MonitorNewItems: true,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}, nil
}

func (fakeWanted) ListAuthorSubscriptions(context.Context, string) ([]wanted.AuthorSubscription, error) {
	return []wanted.AuthorSubscription{{
		ID:              "author-sub-1",
		Provider:        "Open Library",
		ProviderKey:     "openlibrary:OL123A",
		AuthorName:      "Andy Weir",
		Format:          "ebook",
		QualityProfile:  "standard",
		Status:          "monitored",
		MonitorNewItems: true,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}}, nil
}

func (fakeWanted) MonitorAuthors(context.Context, wanted.AuthorMonitorRequest) (wanted.AuthorMonitorRun, error) {
	return wanted.AuthorMonitorRun{
		ID:             "author-run-1",
		Trigger:        "manual",
		Status:         "completed",
		AuthorsChecked: 1,
		ItemsFound:     2,
		WantedCreated:  2,
		StartedAt:      time.Now().UTC(),
	}, nil
}

func (fakeWanted) SearchReleases(context.Context, string, wanted.SearchReleasesRequest) (wanted.SearchOutcome, error) {
	return wanted.SearchOutcome{}, nil
}

func (fakeWanted) ListReleases(context.Context, string) (wanted.SearchOutcome, error) {
	return wanted.SearchOutcome{}, nil
}

func (fakeWanted) Grab(context.Context, string, wanted.GrabRequest) (acquisition.DownloadStatus, error) {
	return acquisition.DownloadStatus{ID: "download-1", Name: "Book", State: "stoppedDL"}, nil
}

func (fakeWanted) Monitor(context.Context, wanted.MonitorRequest) (wanted.MonitorRun, error) {
	return wanted.MonitorRun{
		ID:            "run-1",
		Trigger:       "manual",
		Status:        "completed",
		WantedChecked: 1,
		StartedAt:     time.Now().UTC(),
	}, nil
}

func (fakeWanted) FeedSync(context.Context, wanted.FeedSyncRequest) (wanted.FeedSyncRun, error) {
	return wanted.FeedSyncRun{
		ID:            "feed-1",
		Trigger:       "manual",
		Status:        "completed",
		ReleasesSeen:  2,
		MatchedCount:  1,
		ApprovedCount: 1,
		StartedAt:     time.Now().UTC(),
	}, nil
}

func (fakeWanted) RecoverFailedDownloads(context.Context, wanted.FailedDownloadRequest) (wanted.FailedDownloadRun, error) {
	return wanted.FailedDownloadRun{
		ID:                "failed-1",
		Trigger:           "manual",
		Status:            "completed",
		DownloadsChecked:  1,
		FailedCount:       1,
		ReplacementsFound: 1,
		StartedAt:         time.Now().UTC(),
	}, nil
}

func (fakeWanted) SearchUpgrades(context.Context, wanted.UpgradeRequest) (wanted.UpgradeRun, error) {
	return wanted.UpgradeRun{
		ID:            "upgrade-1",
		Trigger:       "manual",
		Status:        "completed",
		WantedChecked: 1,
		ReleasesFound: 2,
		UpgradeCount:  1,
		StartedAt:     time.Now().UTC(),
	}, nil
}

func (fakeWanted) History(context.Context, wanted.HistoryQuery) ([]wanted.HistoryEvent, error) {
	return []wanted.HistoryEvent{{
		ID:         "event-1",
		EventType:  "wanted_searched",
		EntityType: "wanted_item",
		EntityID:   "wanted-1",
		Severity:   "info",
		Message:    "Searched wanted releases",
		CreatedAt:  time.Now().UTC(),
	}}, nil
}

type fakeLibrary struct{}

func (fakeLibrary) ListFiles(context.Context, library.FileListQuery) ([]library.FileRecord, error) {
	return []library.FileRecord{{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "imported",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}}, nil
}

func (fakeLibrary) ListImportReviews(context.Context, library.ReviewListQuery) ([]library.ImportReview, error) {
	return []library.ImportReview{{
		ID:          "review-1",
		SourcePath:  "/downloads/Project Hail Mary.epub",
		DownloadID:  "abc123",
		MediaFormat: "ebook",
		Title:       "Project Hail Mary",
		Reason:      "download is not linked to a wanted item",
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}}, nil
}

func (fakeLibrary) Scan(context.Context, library.ScanRequest) (library.ScanOutcome, error) {
	file := library.FileRecord{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "available",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	return library.ScanOutcome{
		Roots:    []string{"/library/ebooks"},
		Scanned:  1,
		Upserted: 1,
		Files:    []library.FileRecord{file},
	}, nil
}

func (fakeLibrary) ResolveImportReview(context.Context, string, library.ReviewDecisionRequest) (library.ReviewDecisionOutcome, error) {
	file := library.FileRecord{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "imported",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	return library.ReviewDecisionOutcome{
		Review: library.ImportReview{
			ID:              "review-1",
			SourcePath:      "/downloads/Project Hail Mary.epub",
			MediaFormat:     "ebook",
			Title:           "Project Hail Mary",
			Reason:          "download is not linked to a wanted item",
			Status:          "imported",
			Decision:        "import",
			DestinationPath: file.Path,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		},
		Import: &library.ImportOutcome{
			File:            file,
			DestinationPath: file.Path,
		},
	}, nil
}

func (fakeLibrary) Import(context.Context, library.ImportRequest) (library.ImportOutcome, error) {
	file := library.FileRecord{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "imported",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	return library.ImportOutcome{
		File:            file,
		DestinationPath: file.Path,
		Moved:           false,
	}, nil
}

func (fakeLibrary) ImportCompletedDownloads(_ context.Context, downloads []acquisition.DownloadStatus, _ library.CompletedImportRequest) (library.CompletedImportOutcome, error) {
	file := library.FileRecord{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "imported",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	return library.CompletedImportOutcome{
		Checked:  1,
		Imported: 1,
		Results: []library.DownloadImportResult{{
			Download: downloads[0],
			Status:   "imported",
			Import: &library.ImportOutcome{
				File:            file,
				DestinationPath: file.Path,
			},
		}},
	}, nil
}

type fakeMetadataProvider struct{}

func (fakeMetadataProvider) Name() string {
	return "Fake Metadata"
}

func (fakeMetadataProvider) Health(metadata.Context) metadata.ProviderHealth {
	return metadata.ProviderHealth{Name: "Fake Metadata", Status: "ready", Configured: true, Message: "Ready", CheckedAt: time.Now().UTC()}
}

func (fakeMetadataProvider) Diagnostics(metadata.Context) metadata.Diagnostic {
	return metadata.Diagnostic{Name: "Fake Metadata", Configured: true}
}

func (fakeMetadataProvider) Search(_ metadata.Context, query metadata.Query) ([]metadata.SearchResult, error) {
	return []metadata.SearchResult{{
		Provider:   "Fake Metadata",
		Kind:       query.Type,
		Score:      99,
		Confidence: "high",
		MatchedOn:  []string{"title_author"},
		Work: metadata.Work{
			ID:               "openlibrary:OL1W",
			Title:            "Project Hail Mary",
			FirstPublishYear: 2021,
			CoverURL:         "https://covers.example/project-hail-mary.jpg",
			Authors: []metadata.Author{{
				ID:   "openlibrary:OL123A",
				Name: "Andy Weir",
			}},
		},
		Edition: metadata.Edition{
			ID:     "openlibrary:OL1M",
			Title:  "Project Hail Mary",
			Format: metadata.FormatEbook,
			ISBNs:  []string{"9780593135204"},
		},
	}}, nil
}
