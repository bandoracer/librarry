package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
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

func TestAPIKeyAuthIsOptionalWhenUnset(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected unset API key to allow request, got %d: %s", res.Code, res.Body.String())
	}
}

func TestAPIKeyAuthRequiresConfiguredKeyForAPIPaths(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*", APIKey: "secret"},
		Metadata: metadata.NewService(nil),
	})

	for _, tt := range []struct {
		name       string
		path       string
		header     string
		authHeader string
		want       int
	}{
		{name: "missing", path: "/api/v1/system/status", want: http.StatusUnauthorized},
		{name: "wrong header", path: "/api/v1/system/status", header: "wrong", want: http.StatusUnauthorized},
		{name: "x api key header", path: "/api/v1/system/status", header: "secret", want: http.StatusOK},
		{name: "lowercase query", path: "/api/v1/system/status?apikey=secret", want: http.StatusOK},
		{name: "camel query", path: "/api/v1/system/status?apiKey=secret", want: http.StatusOK},
		{name: "bearer header", path: "/api/v1/system/status", authHeader: "Bearer secret", want: http.StatusOK},
		{name: "apikey auth header", path: "/api/v1/system/status", authHeader: "ApiKey secret", want: http.StatusOK},
		{name: "health exempt", path: "/healthz", want: http.StatusOK},
		{name: "ping exempt", path: "/ping", want: http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.header != "" {
				req.Header.Set("X-Api-Key", tt.header)
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			res := httptest.NewRecorder()

			router.ServeHTTP(res, req)

			if res.Code != tt.want {
				t.Fatalf("expected %d, got %d: %s", tt.want, res.Code, res.Body.String())
			}
		})
	}
}

func TestAPIKeyAuthCORSAllowsArrHeader(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*", APIKey: "secret"},
		Metadata: metadata.NewService(nil),
	})
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/system/status", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected CORS preflight to bypass auth, got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "X-Api-Key") {
		t.Fatalf("expected X-Api-Key in CORS headers, got %q", got)
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

func TestCompatRootFolderPersistenceEndpoints(t *testing.T) {
	compatResources := &fakeCompatResources{
		roots: []compatdata.RootFolder{{
			ID:          "root-1",
			Name:        "Comics",
			Path:        "/srv/books/comics",
			MediaFormat: "ebook",
			Metadata:    map[string]any{"source": "test"},
		}},
	}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*", EbookLibraryRoot: "/tmp/ebooks", AudiobookLibraryRoot: "/tmp/audiobooks"},
		Metadata: metadata.NewService(nil),
		Compat:   compatResources,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rootfolder", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"/srv/books/comics"`) || !strings.Contains(res.Body.String(), `"librarryId":"root-1"`) {
		t.Fatalf("expected persisted root folder, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/rootfolder/"+strconv.Itoa(stableInt("root-1")), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"/srv/books/comics"`) {
		t.Fatalf("expected persisted root folder lookup, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/rootfolder", strings.NewReader(`{"name":"Audiobooks Extra","path":"/srv/books/audio-extra","mediaFormat":"audiobook"}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"/srv/books/audio-extra"`) || !strings.Contains(res.Body.String(), `"mediaFormat":"audiobook"`) {
		t.Fatalf("expected created persisted root folder, got %s", res.Body.String())
	}
	if len(compatResources.roots) != 2 {
		t.Fatalf("expected root folder to be persisted, got %d roots", len(compatResources.roots))
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/rootfolder/"+strconv.Itoa(stableInt("root-1")), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", res.Code, res.Body.String())
	}
	if len(compatResources.deleted) != 1 || compatResources.deleted[0] != "root-1" {
		t.Fatalf("expected delete by stored id, got %#v", compatResources.deleted)
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

func TestCompatBlocklistEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeBlocklistAcquire{},
		Wanted:   fakeBlocklistWanted{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocklist?page=1&pageSize=10", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"totalRecords":2`) || !strings.Contains(res.Body.String(), `"librarrySource":"download"`) || !strings.Contains(res.Body.String(), `"librarrySource":"history"`) {
		t.Fatalf("expected failed download and failed history blocklist records, got %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"sourceTitle":"Project Hail Mary failed.epub"`) || !strings.Contains(res.Body.String(), `"message":"missing files"`) {
		t.Fatalf("expected failed download details, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/blacklist", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"totalRecords":2`) {
		t.Fatalf("expected legacy blacklist alias, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/blocklist/"+strconv.Itoa(stableInt("download:failed-1")), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/blacklist/bulk", strings.NewReader(`{"ids":[1]}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", res.Code, res.Body.String())
	}
}

func TestCompatReleaseSearchAndGrabEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*", EbookCategory: "books-ebook", BookTorrentRoot: "/downloads/books"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
		Wanted:   fakeWanted{},
	})

	bookID := stableInt("wanted-1")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/release?bookId="+strconv.Itoa(bookID), nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"guid":"release-1"`) || !strings.Contains(res.Body.String(), `"downloadUrl":"magnet:?xt=urn:btih:projecthailmary"`) {
		t.Fatalf("expected Readarr-style release payload, got %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"bookId":`) || !strings.Contains(res.Body.String(), `"authorName":"Andy Weir"`) {
		t.Fatalf("expected release linked to wanted book, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/release", strings.NewReader(`{"title":"Project Hail Mary EPUB","downloadUrl":"magnet:?xt=urn:btih:projecthailmary","protocol":"torrent","bookId":`+strconv.Itoa(bookID)+`}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"grabbed":true`) || !strings.Contains(res.Body.String(), `"downloadId":"download-1"`) {
		t.Fatalf("expected grabbed release payload, got %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"librarryWantedId":"wanted-1"`) {
		t.Fatalf("expected wanted item linkage, got %s", res.Body.String())
	}
}

func TestCompatReleaseGrabResolvesStoredDecisionID(t *testing.T) {
	wantedClient := &fakeReleaseWanted{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*", EbookCategory: "books-ebook", BookTorrentRoot: "/downloads/books"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
		Wanted:   wantedClient,
	})

	bookID := stableInt("wanted-1")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/release?bookId="+strconv.Itoa(bookID), nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var records []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0]["librarryReleaseId"] != "release-1" || records[0]["guid"] != "release-1" {
		t.Fatalf("expected persisted release decision payload, got %+v", records)
	}
	returnedID := int(records[0]["id"].(float64))

	req = httptest.NewRequest(http.MethodPost, "/api/v1/release", strings.NewReader(`{"id":`+strconv.Itoa(returnedID)+`,"bookId":`+strconv.Itoa(bookID)+`}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if len(wantedClient.grabs) != 1 || wantedClient.grabs[0].ReleaseID != "release-1" {
		t.Fatalf("expected resolved wanted grab for release-1, got %+v", wantedClient.grabs)
	}
	if !strings.Contains(res.Body.String(), `"grabbed":true`) || !strings.Contains(res.Body.String(), `"librarryWantedId":"wanted-1"`) {
		t.Fatalf("expected Readarr grabbed release payload, got %s", res.Body.String())
	}
}

func TestCompatReleaseGrabDoesNotBypassRejectedStoredDecision(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*", EbookCategory: "books-ebook", BookTorrentRoot: "/downloads/books"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
		Wanted:   fakeRejectedReleaseWanted{},
	})

	bookID := stableInt("wanted-1")
	releaseID := stableInt("release-1")
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/release",
		strings.NewReader(`{"id":`+strconv.Itoa(releaseID)+`,"bookId":`+strconv.Itoa(bookID)+`,"downloadUrl":"magnet:?xt=urn:btih:projecthailmary"}`),
	)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("expected rejected stored decision to block direct fallback, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "release is rejected") {
		t.Fatalf("expected rejected release error, got %s", res.Body.String())
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

func TestCompatCatalogResourceEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
	})

	checks := []struct {
		path string
		want string
	}{
		{"/api/v1/qualitydefinition", `"name":"EPUB"`},
		{"/api/v1/languageprofile", `"name":"English"`},
		{"/api/v1/metadataprofile", `"name":"Standard"`},
		{"/api/v1/tag", `"label":"librarry"`},
		{"/api/v1/customformat", `[]`},
		{"/api/v1/restriction", `[]`},
		{"/api/v1/notification", `[]`},
		{"/api/v1/importlist", `[]`},
		{"/api/v1/remotepathmapping", `[]`},
	}
	for _, check := range checks {
		req := httptest.NewRequest(http.MethodGet, check.path, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d: %s", check.path, res.Code, res.Body.String())
		}
		if !strings.Contains(res.Body.String(), check.want) {
			t.Fatalf("%s expected %s in response, got %s", check.path, check.want, res.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/qualitydefinition/2", strings.NewReader(`{"title":"EPUB","weight":110,"maxSize":900}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"maxSize":900`) || !strings.Contains(res.Body.String(), `"weight":110`) {
		t.Fatalf("expected quality definition update echo, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/tag", strings.NewReader(`{"label":"retail"}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"label":"retail"`) {
		t.Fatalf("expected created tag, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/notification", strings.NewReader(`{"name":"Webhook","implementation":"Webhook","enable":true}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"enable":true`) || !strings.Contains(res.Body.String(), `"implementation":"Webhook"`) {
		t.Fatalf("expected created notification, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/remotepathmapping", strings.NewReader(`{"host":"qbittorrent","remotePath":"/downloads","localPath":"/data/downloads"}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"remotePath":"/downloads"`) || !strings.Contains(res.Body.String(), `"localPath":"/data/downloads"`) {
		t.Fatalf("expected created remote path mapping, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/customformat/1", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", res.Code, res.Body.String())
	}
}

func TestCompatCatalogResourcePersistenceEndpoints(t *testing.T) {
	compatResources := &fakeCompatResources{
		resources: []compatdata.Resource{{
			ResourceType: "remote-path-mapping",
			CompatID:     55,
			Name:         "qbittorrent",
			Payload: map[string]any{
				"host":       "qbittorrent",
				"remotePath": "/downloads",
				"localPath":  "/data/downloads",
			},
		}},
	}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Compat:   compatResources,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tag", strings.NewReader(`{"label":"retail"}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"id":1`) || !strings.Contains(res.Body.String(), `"label":"retail"`) || !strings.Contains(res.Body.String(), `"librarryPersisted":true`) {
		t.Fatalf("expected persisted created tag, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/tag", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"label":"librarry"`) || !strings.Contains(res.Body.String(), `"label":"retail"`) {
		t.Fatalf("expected default and persisted tags, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/tag/1", strings.NewReader(`{"label":"priority"}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"label":"priority"`) {
		t.Fatalf("expected persisted tag update, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/tag/1", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"label":"priority"`) {
		t.Fatalf("expected persisted tag lookup, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/remotepathmapping/55", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"remotePath":"/downloads"`) || !strings.Contains(res.Body.String(), `"localPath":"/data/downloads"`) {
		t.Fatalf("expected persisted remote path mapping, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/tag/1", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", res.Code, res.Body.String())
	}
	if len(compatResources.deletedResources) != 1 || compatResources.deletedResources[0] != "tag:1" {
		t.Fatalf("expected persisted delete, got %#v", compatResources.deletedResources)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/tag/1", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected deleted resource to be missing, got %d: %s", res.Code, res.Body.String())
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
			TransmissionURL:    "http://transmission.local",
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
	if !strings.Contains(res.Body.String(), `"name":"qBittorrent"`) || !strings.Contains(res.Body.String(), `"name":"Transmission"`) || !strings.Contains(res.Body.String(), `"name":"SABnzbd"`) {
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

func TestCompatArrResourceUtilityEndpointsPersist(t *testing.T) {
	compatResources := &fakeCompatResources{}
	router := NewRouter(Dependencies{
		Logger: slog.Default(),
		Config: config.Config{
			WebOrigin:       "*",
			ProwlarrURL:     "http://prowlarr.local",
			ProwlarrAPIKey:  "secret",
			QBittorrentURL:  "http://qbittorrent.local",
			EbookCategory:   "books-ebook",
			BookTorrentRoot: "/downloads/books",
		},
		Metadata: metadata.NewService(nil),
		Compat:   compatResources,
	})

	checks := []struct {
		method string
		path   string
		body   string
		status int
		want   []string
	}{
		{http.MethodGet, "/api/v1/downloadclient/schema", "", http.StatusOK, []string{`"implementation":"qBittorrent"`, `"implementation":"SABnzbd"`}},
		{http.MethodPost, "/api/v1/downloadclient/test", `{"name":"qBittorrent","implementation":"qBittorrent"}`, http.StatusOK, []string{`"testPassed":true`, `"resourceType":"download-client"`}},
		{http.MethodPost, "/api/v1/downloadclient/action/getCategories", `{"id":1}`, http.StatusOK, []string{`"action":"getCategories"`, `"status":"completed"`}},
		{http.MethodGet, "/api/v1/indexer/schema", "", http.StatusOK, []string{`"implementation":"Torznab"`, `"implementation":"Newznab"`}},
		{http.MethodPost, "/api/v1/indexer/test", `{"name":"Prowlarr","implementation":"Torznab"}`, http.StatusOK, []string{`"testPassed":true`, `"resourceType":"indexer"`}},
		{http.MethodGet, "/api/v1/notification/schema", "", http.StatusOK, []string{`"implementation":"Webhook"`, `"configContract":"WebhookSettings"`}},
		{http.MethodPost, "/api/v1/notification/test", `{"name":"Webhook","implementation":"Webhook","enable":true}`, http.StatusOK, []string{`"testPassed":true`, `"resourceType":"notification"`}},
		{http.MethodGet, "/api/v1/importlist/schema", "", http.StatusOK, []string{`"implementation":"ReadarrImportList"`, `"rootFolderPath"`}},
		{http.MethodPost, "/api/v1/importlist/testall", `{}`, http.StatusOK, []string{`[]`}},
	}
	for _, check := range checks {
		req := httptest.NewRequest(check.method, check.path, strings.NewReader(check.body))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != check.status {
			t.Fatalf("%s %s expected %d, got %d: %s", check.method, check.path, check.status, res.Code, res.Body.String())
		}
		for _, want := range check.want {
			if !strings.Contains(res.Body.String(), want) {
				t.Fatalf("%s %s expected %s, got %s", check.method, check.path, want, res.Body.String())
			}
		}
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/downloadclient/50", strings.NewReader(`{"name":"Deluge","implementation":"Deluge","protocol":"torrent","enable":true}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"name":"Deluge"`) || !strings.Contains(res.Body.String(), `"librarryPersisted":true`) {
		t.Fatalf("expected persisted download client update, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/downloadclient/50", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"name":"Deluge"`) {
		t.Fatalf("expected persisted download client lookup, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/indexer/44", strings.NewReader(`{"name":"Books Torznab","implementation":"Torznab","protocol":"torrent","enableRss":true}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"name":"Books Torznab"`) {
		t.Fatalf("expected persisted indexer update, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/indexer/bulk", strings.NewReader(`{"ids":[44],"enableRss":false,"priority":12}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted || !strings.Contains(res.Body.String(), `"enableRss":false`) || !strings.Contains(res.Body.String(), `"priority":12`) {
		t.Fatalf("expected persisted indexer bulk update, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/indexer/bulk", strings.NewReader(`{"ids":[44]}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected indexer bulk delete 204, got %d: %s", res.Code, res.Body.String())
	}
	if len(compatResources.deletedResources) != 1 || compatResources.deletedResources[0] != "indexer:44" {
		t.Fatalf("expected persisted indexer bulk delete, got %#v", compatResources.deletedResources)
	}
}

func TestCompatOperationalSupportEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tempDir, "authors"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "book.epub"), []byte("epub"), 0o644); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*", EbookLibraryRoot: tempDir},
		Metadata: metadata.NewService(nil),
	})

	checks := []struct {
		path string
		want string
	}{
		{"/api/v1/language", `"name":"English"`},
		{"/api/v1/localization", `"book":"Book"`},
		{"/api/v1/localization/options", `"id":"en"`},
		{"/api/v1/system/backup", `[]`},
		{"/api/v1/update", `[]`},
		{"/api/v1/log", `"totalRecords":1`},
		{"/api/v1/log/file", `"filename":"librarry.txt"`},
		{"/api/v1/filesystem?path=" + tempDir + "&includeFiles=true", `"name":"book.epub"`},
		{"/api/v1/filesystem?path=" + tempDir + "&includeFiles=false", `"name":"authors"`},
	}
	for _, check := range checks {
		req := httptest.NewRequest(http.MethodGet, check.path, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d: %s", check.path, res.Code, res.Body.String())
		}
		if !strings.Contains(res.Body.String(), check.want) {
			t.Fatalf("%s expected %s, got %s", check.path, check.want, res.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem?path="+tempDir+"&includeFiles=false", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if strings.Contains(res.Body.String(), `"name":"book.epub"`) {
		t.Fatalf("expected includeFiles=false to omit files, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/log/file/librarry.txt", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "Librarry compatibility log") {
		t.Fatalf("expected log file body, got %d: %s", res.Code, res.Body.String())
	}
}

func TestCompatMetadataAndImportListExclusionEndpointsPersist(t *testing.T) {
	compatResources := &fakeCompatResources{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Compat:   compatResources,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metadata/schema", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"implementation":"Calibre"`) {
		t.Fatalf("expected metadata schema, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/metadata", strings.NewReader(`{"name":"Calibre","implementation":"Calibre","enable":true}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"implementation":"Calibre"`) || !strings.Contains(res.Body.String(), `"librarryPersisted":true`) {
		t.Fatalf("expected persisted metadata consumer, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/metadata/testall", strings.NewReader(`{}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"resourceType":"metadata-consumer"`) {
		t.Fatalf("expected metadata testall result, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/importlistexclusion", strings.NewReader(`{"authorName":"Andy Weir","bookTitle":"Project Hail Mary","foreignId":"openlibrary:OL1W"}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"bookTitle":"Project Hail Mary"`) || !strings.Contains(res.Body.String(), `"foreignId":"openlibrary:OL1W"`) {
		t.Fatalf("expected persisted import list exclusion, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/importlistexclusion", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"authorName":"Andy Weir"`) {
		t.Fatalf("expected import list exclusions, got %d: %s", res.Code, res.Body.String())
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

func TestCompatAuthorAndBookUpdateDeleteEndpointsPersist(t *testing.T) {
	wantedClient := &fakeMutableWanted{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   wantedClient,
	})

	authorID := stableInt("author-sub-1")
	req := httptest.NewRequest(http.MethodPut, "/api/v1/author/"+strconv.Itoa(authorID), strings.NewReader(`{"authorName":"Andy Weir","monitored":false,"qualityProfile":"retail","monitorNewItems":false}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected author update 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"monitored":false`) || !strings.Contains(res.Body.String(), `"librarryQualityName":"retail"`) {
		t.Fatalf("expected unmonitored author update response, got %s", res.Body.String())
	}
	if len(wantedClient.authorUpdates) != 1 || wantedClient.authorUpdates[0].id != "author-sub-1" || wantedClient.authorUpdates[0].request.Monitored == nil || *wantedClient.authorUpdates[0].request.Monitored {
		t.Fatalf("expected persisted author unmonitor update, got %+v", wantedClient.authorUpdates)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/author/"+strconv.Itoa(authorID), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected author delete 204, got %d: %s", res.Code, res.Body.String())
	}
	if len(wantedClient.authorDeletes) != 1 || wantedClient.authorDeletes[0] != "author-sub-1" {
		t.Fatalf("expected persisted author delete, got %+v", wantedClient.authorDeletes)
	}

	bookID := stableInt("wanted-1")
	req = httptest.NewRequest(http.MethodPut, "/api/v1/book/"+strconv.Itoa(bookID), strings.NewReader(`{"title":"Project Hail Mary","authorTitle":"Andy Weir","monitored":false,"qualityProfile":"retail"}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected book update 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"monitored":false`) || !strings.Contains(res.Body.String(), `"qualityProfile":"retail"`) {
		t.Fatalf("expected unmonitored book update response, got %s", res.Body.String())
	}
	if len(wantedClient.wantedUpdates) != 1 || wantedClient.wantedUpdates[0].id != "wanted-1" || wantedClient.wantedUpdates[0].request.Monitored == nil || *wantedClient.wantedUpdates[0].request.Monitored {
		t.Fatalf("expected persisted book unmonitor update, got %+v", wantedClient.wantedUpdates)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/book/monitor", strings.NewReader(fmt.Sprintf(`{"bookIds":[%d],"monitored":true}`, bookID)))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("expected book monitor 202, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"monitored":true`) {
		t.Fatalf("expected monitored book response, got %s", res.Body.String())
	}
	if len(wantedClient.wantedUpdates) != 2 || wantedClient.wantedUpdates[1].id != "wanted-1" || wantedClient.wantedUpdates[1].request.Monitored == nil || !*wantedClient.wantedUpdates[1].request.Monitored {
		t.Fatalf("expected persisted book monitor update, got %+v", wantedClient.wantedUpdates)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/book/"+strconv.Itoa(bookID), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected book delete 204, got %d: %s", res.Code, res.Body.String())
	}
	if len(wantedClient.wantedDeletes) != 1 || wantedClient.wantedDeletes[0] != "wanted-1" {
		t.Fatalf("expected persisted book delete, got %+v", wantedClient.wantedDeletes)
	}
}

func TestCompatBookFileEndpoints(t *testing.T) {
	deleteLibrary := &fakeDeleteLibrary{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
		Library:  deleteLibrary,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bookfile", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var list []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0]["librarryId"] != "file-1" || list[0]["path"] != "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub" {
		t.Fatalf("unexpected bookfile list: %+v", list)
	}
	if int(list[0]["bookId"].(float64)) != stableInt("wanted-1") {
		t.Fatalf("expected wanted book id, got %+v", list[0])
	}

	bookID := stableInt("wanted-1")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/bookfile?bookId="+strconv.Itoa(bookID), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	list = nil
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected bookId-filtered file, got %+v", list)
	}

	fileID := stableInt("file-1")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/bookfile/"+strconv.Itoa(fileID), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"relativePath":"Project Hail Mary.epub"`) || !strings.Contains(res.Body.String(), `"librarryImportStatus":"imported"`) {
		t.Fatalf("expected single bookfile payload, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/bookfile/"+strconv.Itoa(fileID), strings.NewReader(`{"quality":{"quality":{"name":"Audiobook"}}}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"librarryCompatibilityNote"`) {
		t.Fatalf("expected update compatibility echo, got %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/bookfile/"+strconv.Itoa(fileID), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", res.Code, res.Body.String())
	}
	if len(deleteLibrary.requests) != 1 || deleteLibrary.requests[0].IDs[0] != "file-1" || deleteLibrary.requests[0].DeleteFiles {
		t.Fatalf("expected single DB-only delete request, got %+v", deleteLibrary.requests)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/bookfile/bulk", strings.NewReader(`{"bookFileIds":[`+strconv.Itoa(fileID)+`],"deleteFiles":true}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", res.Code, res.Body.String())
	}
	if len(deleteLibrary.requests) != 2 || deleteLibrary.requests[1].IDs[0] != "file-1" || !deleteLibrary.requests[1].DeleteFiles {
		t.Fatalf("expected bulk physical delete request, got %+v", deleteLibrary.requests)
	}
}

func TestCompatRenamePreviewEndpoint(t *testing.T) {
	renameLibrary := &fakeRenameLibrary{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
		Library:  renameLibrary,
	})

	bookID := stableInt("wanted-1")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rename?bookId="+strconv.Itoa(bookID), nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var records []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0]["existingPath"] == "" || records[0]["newPath"] == "" {
		t.Fatalf("expected rename preview record, got %+v", records)
	}
	if len(renameLibrary.previewRequests) != 1 || renameLibrary.previewRequests[0].IDs[0] != "file-1" {
		t.Fatalf("expected preview request for file-1, got %+v", renameLibrary.previewRequests)
	}
}

func TestCompatRenameCommand(t *testing.T) {
	renameLibrary := &fakeRenameLibrary{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
		Library:  renameLibrary,
	})

	fileID := stableInt("file-1")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/command", strings.NewReader(`{"name":"RenameFiles","files":[`+strconv.Itoa(fileID)+`]}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"commandName":"RenameFiles"`) || !strings.Contains(res.Body.String(), `"renamed":1`) {
		t.Fatalf("expected rename command outcome, got %s", res.Body.String())
	}
	if len(renameLibrary.renameRequests) != 1 || renameLibrary.renameRequests[0].IDs[0] != "file-1" {
		t.Fatalf("expected rename request for file-1, got %+v", renameLibrary.renameRequests)
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

func TestCompatHostUIIndexerDownloadConfigAndTasks(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger: slog.Default(),
		Config: config.Config{
			WebOrigin:              "*",
			ListenAddr:             ":9090",
			ProwlarrURL:            "http://prowlarr.local",
			BookTorrentRoot:        "/downloads/books",
			EbookCategory:          "books-ebook",
			AudiobookCategory:      "books-audiobook",
			FeedSyncEnabled:        true,
			FeedSyncInterval:       15 * time.Minute,
			MonitorEnabled:         true,
			MonitorInterval:        30 * time.Minute,
			AuthorMonitorEnabled:   true,
			AuthorMonitorInterval:  6 * time.Hour,
			FailedDownloadEnabled:  true,
			FailedDownloadInterval: 30 * time.Minute,
			UpgradeSearchEnabled:   true,
			UpgradeSearchInterval:  12 * time.Hour,
		},
		Metadata: metadata.NewService(nil),
	})

	checks := []struct {
		path string
		want string
	}{
		{"/api/v1/config/host", `"port":9090`},
		{"/api/v1/config/ui", `"theme":"auto"`},
		{"/api/v1/config/downloadclient", `"librarryTorrentRoot":"/downloads/books"`},
		{"/api/v1/config/indexer", `"rssSyncInterval":15`},
		{"/api/v1/delayprofile", `"preferredProtocol":"torrent"`},
		{"/api/v1/system/task", `"taskName":"RssSync"`},
	}
	for _, check := range checks {
		req := httptest.NewRequest(http.MethodGet, check.path, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d: %s", check.path, res.Code, res.Body.String())
		}
		if !strings.Contains(res.Body.String(), check.want) {
			t.Fatalf("%s expected %s in response, got %s", check.path, check.want, res.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/host/1", strings.NewReader(`{"port":8081,"instanceName":"Test Librarry"}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"port":8081`) || !strings.Contains(res.Body.String(), `"instanceName":"Test Librarry"`) {
		t.Fatalf("expected host config update echo, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/config/ui/1", strings.NewReader(`{"theme":"dark","showRelativeDates":false}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"theme":"dark"`) || !strings.Contains(res.Body.String(), `"showRelativeDates":false`) {
		t.Fatalf("expected UI config update echo, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/delayprofile", strings.NewReader(`{"preferredProtocol":"usenet","usenetDelay":10,"torrentDelay":20}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"preferredProtocol":"usenet"`) || !strings.Contains(res.Body.String(), `"torrentDelay":20`) {
		t.Fatalf("expected created delay profile, got %d: %s", res.Code, res.Body.String())
	}

	taskID := stableInt("RssSync")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/task/"+strconv.Itoa(taskID), nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"taskName":"RssSync"`) {
		t.Fatalf("expected single system task, got %d: %s", res.Code, res.Body.String())
	}
}

func TestCompatConfigPersistenceEndpoints(t *testing.T) {
	compatResources := &fakeCompatResources{}
	router := NewRouter(Dependencies{
		Logger: slog.Default(),
		Config: config.Config{
			WebOrigin:            "*",
			ListenAddr:           ":9090",
			BookTorrentRoot:      "/downloads/books",
			EbookCategory:        "books-ebook",
			AudiobookCategory:    "books-audiobook",
			EbookLibraryRoot:     "/media/books/ebooks",
			AudiobookLibraryRoot: "/media/books/audiobooks",
		},
		Metadata: metadata.NewService(nil),
		Compat:   compatResources,
	})

	checks := []struct {
		path    string
		payload string
		want    []string
	}{
		{"/api/v1/config/naming/1", `{"standardBookFormat":"{Author}/{Title}{Ext}","replaceSpaces":false}`, []string{`"standardBookFormat":"{Author}/{Title}{Ext}"`, `"replaceSpaces":false`}},
		{"/api/v1/config/mediamanagement/1", `{"recycleBin":"/trash","copyUsingHardlinks":true}`, []string{`"recycleBin":"/trash"`, `"copyUsingHardlinks":true`}},
		{"/api/v1/config/host/1", `{"port":8081,"instanceName":"Public Librarry"}`, []string{`"port":8081`, `"instanceName":"Public Librarry"`}},
		{"/api/v1/config/ui/1", `{"theme":"dark","showRelativeDates":false}`, []string{`"theme":"dark"`, `"showRelativeDates":false`}},
		{"/api/v1/config/downloadclient/1", `{"removeCompletedDownloads":true,"autoRedownloadFailed":true}`, []string{`"removeCompletedDownloads":true`, `"autoRedownloadFailed":true`}},
		{"/api/v1/config/indexer/1", `{"rssSyncInterval":30,"minimumAge":5}`, []string{`"rssSyncInterval":30`, `"minimumAge":5`}},
	}

	for _, check := range checks {
		req := httptest.NewRequest(http.MethodPut, check.path, strings.NewReader(check.payload))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s expected update 200, got %d: %s", check.path, res.Code, res.Body.String())
		}
		if !strings.Contains(res.Body.String(), `"librarryPersistedViaNative":true`) {
			t.Fatalf("%s expected persisted update response, got %s", check.path, res.Body.String())
		}
		for _, want := range check.want {
			if !strings.Contains(res.Body.String(), want) {
				t.Fatalf("%s expected update response to contain %s, got %s", check.path, want, res.Body.String())
			}
		}

		getPath := strings.TrimSuffix(check.path, "/1")
		req = httptest.NewRequest(http.MethodGet, getPath, nil)
		res = httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s expected persisted get 200, got %d: %s", getPath, res.Code, res.Body.String())
		}
		if !strings.Contains(res.Body.String(), `"librarryPersistedViaNative":true`) {
			t.Fatalf("%s expected persisted get response, got %s", getPath, res.Body.String())
		}
		for _, want := range check.want {
			if !strings.Contains(res.Body.String(), want) {
				t.Fatalf("%s expected get response to contain %s, got %s", getPath, want, res.Body.String())
			}
		}
	}

	if len(compatResources.resources) != len(checks) {
		t.Fatalf("expected %d persisted config resources, got %+v", len(checks), compatResources.resources)
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
	acquire := &fakeActionAcquire{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  acquire,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/downloads/actions", strings.NewReader(`{"action":"setDownloadLimit","ids":["abc123"],"downloadLimit":1048576}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload acquisition.DownloadActionResult
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Action != acquisition.DownloadActionSetDownloadLimit || !payload.Applied || payload.IDs[0] != "abc123" {
		t.Fatalf("unexpected action payload: %+v", payload)
	}
	if len(acquire.requests) != 1 || acquire.requests[0].DownloadLimit != 1_048_576 {
		t.Fatalf("expected download limit request, got %+v", acquire.requests)
	}
}

func TestDownloadDetailsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/downloads/abc123?client=qBittorrent", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload acquisition.DownloadDetails
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status.ID != "abc123" || len(payload.Files) != 1 || payload.Files[0].Name != "Project Hail Mary.epub" {
		t.Fatalf("unexpected details payload: %+v", payload)
	}
	if len(payload.Trackers) != 1 || payload.Trackers[0].Status != "working" {
		t.Fatalf("unexpected tracker payload: %+v", payload.Trackers)
	}
	if len(payload.Peers) != 1 || payload.Peers[0].IP != "203.0.113.10" {
		t.Fatalf("unexpected peer payload: %+v", payload.Peers)
	}
}

func TestDownloadFileActionEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/downloads/abc123/files/actions", strings.NewReader(`{"action":"skip","ids":[0]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload acquisition.DownloadFileActionResult
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Applied || payload.DownloadID != "abc123" || payload.Action != acquisition.DownloadFileActionSkip || payload.Priority != 0 {
		t.Fatalf("unexpected file action payload: %+v", payload)
	}
}

func TestDownloadTrackerActionEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/downloads/abc123/trackers/actions", strings.NewReader(`{"action":"add","urls":["https://tracker.example/announce"]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload acquisition.DownloadTrackerActionResult
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Applied || payload.DownloadID != "abc123" || payload.Action != acquisition.DownloadTrackerActionAdd || payload.URLs[0] != "https://tracker.example/announce" {
		t.Fatalf("unexpected tracker action payload: %+v", payload)
	}
	if payload.Download == nil || len(payload.Download.Trackers) != 1 {
		t.Fatalf("expected refreshed download details, got %+v", payload.Download)
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

func TestDeleteLibraryFileEndpoint(t *testing.T) {
	deleteLibrary := &fakeDeleteLibrary{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  deleteLibrary,
	})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/library/files/file-1?deleteFiles=true", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if len(deleteLibrary.requests) != 1 || deleteLibrary.requests[0].IDs[0] != "file-1" || !deleteLibrary.requests[0].DeleteFiles {
		t.Fatalf("expected native delete request, got %+v", deleteLibrary.requests)
	}
	if !strings.Contains(res.Body.String(), `"deleted":1`) {
		t.Fatalf("expected delete outcome, got %s", res.Body.String())
	}
}

func TestRenameLibraryFileEndpoints(t *testing.T) {
	renameLibrary := &fakeRenameLibrary{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  renameLibrary,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/files/rename/preview", strings.NewReader(`{"ids":["file-1"]}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"destinationPath"`) || len(renameLibrary.previewRequests) != 1 || renameLibrary.previewRequests[0].IDs[0] != "file-1" {
		t.Fatalf("expected native preview request/outcome, got body=%s requests=%+v", res.Body.String(), renameLibrary.previewRequests)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/library/files/rename", strings.NewReader(`{"ids":["file-1"]}`))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"renamed":1`) || len(renameLibrary.renameRequests) != 1 || renameLibrary.renameRequests[0].IDs[0] != "file-1" {
		t.Fatalf("expected native rename request/outcome, got body=%s requests=%+v", res.Body.String(), renameLibrary.renameRequests)
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
	return []acquisition.Release{{
		ID:          "release-1",
		InfoHash:    "projecthailmary",
		Indexer:     "Prowlarr",
		Title:       "Andy Weir - Project Hail Mary EPUB",
		SizeBytes:   7340032,
		Seeders:     12,
		Leechers:    1,
		DownloadURL: "magnet:?xt=urn:btih:projecthailmary",
		InfoURL:     "https://indexer.example/releases/release-1",
		Protocol:    "torrent",
		Categories:  []string{"ebook", "epub"},
		PublishedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
	}}, nil
}

func (fakeAcquire) Grab(_ context.Context, request acquisition.DownloadRequest) (acquisition.DownloadStatus, error) {
	state := "downloading"
	if request.Paused {
		state = "stoppedDL"
	}
	now := time.Now().UTC()
	return acquisition.DownloadStatus{
		Client:    "qBittorrent",
		ID:        "download-1",
		Name:      defaultString(request.Title, "Project Hail Mary EPUB"),
		State:     state,
		Progress:  0,
		SavePath:  request.SavePath,
		Category:  request.Category,
		Tags:      request.Tags,
		SizeBytes: 7340032,
		AddedAt:   &now,
	}, nil
}

func (fakeAcquire) Downloads(context.Context, acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error) {
	now := time.Now().UTC()
	return []acquisition.DownloadStatus{{
		Client:      "qBittorrent",
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

func (fakeAcquire) DownloadDetails(context.Context, string, string) (acquisition.DownloadDetails, error) {
	now := time.Now().UTC()
	return acquisition.DownloadDetails{
		Status: acquisition.DownloadStatus{
			Client:       "qBittorrent",
			ID:           "abc123",
			Name:         "Project Hail Mary.epub",
			State:        "downloading",
			Progress:     0.5,
			SavePath:     "/downloads",
			Category:     "books-ebook",
			Tags:         []string{"librarry", "wanted:wanted-1"},
			SizeBytes:    1000,
			AddedAt:      &now,
			LastSeenAt:   &now,
			DownloadRate: 12,
			UploadRate:   3,
		},
		Properties: acquisition.DownloadProperties{
			SavePath:          "/downloads",
			TotalSizeBytes:    1000,
			TotalDownloaded:   500,
			TotalUploaded:     25,
			PieceSizeBytes:    16,
			PiecesHave:        4,
			PiecesTotal:       8,
			Connections:       6,
			ConnectionsLimit:  50,
			DownloadSpeed:     12,
			UploadSpeed:       3,
			ETASeconds:        42,
			ReannounceSeconds: 1200,
		},
		Files: []acquisition.DownloadFile{{
			ID:           0,
			Name:         "Project Hail Mary.epub",
			SizeBytes:    1000,
			Progress:     0.5,
			Priority:     1,
			Availability: 3.5,
		}},
		Trackers: []acquisition.DownloadTracker{{
			URL:        "https://tracker.example/announce",
			StatusCode: 2,
			Status:     "working",
			Peers:      10,
			Seeds:      8,
			Leeches:    2,
		}},
		Peers: []acquisition.DownloadPeer{{
			ID:           "203.0.113.10:51413",
			IP:           "203.0.113.10",
			Port:         51413,
			Client:       "Transmission 4.0",
			Progress:     0.5,
			DownloadRate: 12,
			UploadRate:   3,
		}},
	}, nil
}

func (fakeAcquire) DownloadAction(_ context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error) {
	return acquisition.DownloadActionResult{
		Action:  request.Action,
		IDs:     request.IDs,
		Applied: true,
	}, nil
}

type fakeActionAcquire struct {
	fakeAcquire
	requests []acquisition.DownloadActionRequest
}

func (f *fakeActionAcquire) DownloadAction(_ context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error) {
	f.requests = append(f.requests, request)
	return fakeAcquire{}.DownloadAction(context.Background(), request)
}

func (fakeAcquire) DownloadFileAction(_ context.Context, id string, request acquisition.DownloadFileActionRequest) (acquisition.DownloadFileActionResult, error) {
	priority := 1
	if request.Action == acquisition.DownloadFileActionSkip {
		priority = 0
	}
	return acquisition.DownloadFileActionResult{
		Action:     request.Action,
		DownloadID: id,
		IDs:        request.IDs,
		Priority:   priority,
		Applied:    true,
	}, nil
}

func (fakeAcquire) DownloadTrackerAction(_ context.Context, id string, request acquisition.DownloadTrackerActionRequest) (acquisition.DownloadTrackerActionResult, error) {
	details, _ := fakeAcquire{}.DownloadDetails(context.Background(), id, "qBittorrent")
	return acquisition.DownloadTrackerActionResult{
		Action:     request.Action,
		DownloadID: id,
		URLs:       request.URLs,
		Applied:    true,
		Download:   &details,
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

type fakeBlocklistAcquire struct {
	fakeAcquire
}

func (fakeBlocklistAcquire) Downloads(context.Context, acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error) {
	failedAt := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	return []acquisition.DownloadStatus{{
		Client:        "qBittorrent",
		ID:            "failed-1",
		Name:          "Project Hail Mary failed.epub",
		State:         "error",
		SavePath:      "/downloads",
		Category:      "books-ebook",
		Tags:          []string{"librarry", "wanted:wanted-1"},
		SizeBytes:     7340032,
		FailureReason: "missing files",
		FailedAt:      &failedAt,
	}}, nil
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
		Monitored:      true,
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
		Monitored:      true,
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
	return fakeWantedSearchOutcome(), nil
}

func (fakeWanted) ListReleases(context.Context, string) (wanted.SearchOutcome, error) {
	return fakeWantedSearchOutcome(), nil
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

func (fakeWanted) UpdateWanted(_ context.Context, _ string, request wanted.WantedUpdateRequest) (wanted.WantedItem, error) {
	monitored := true
	if request.Monitored != nil {
		monitored = *request.Monitored
	}
	return wanted.WantedItem{
		ID:             "wanted-1",
		WorkID:         "openlibrary:OL1W",
		Title:          defaultString(request.Title, "Project Hail Mary"),
		AuthorName:     defaultString(request.AuthorName, "Andy Weir"),
		Format:         "ebook",
		QualityProfile: defaultString(request.QualityProfile, "standard"),
		Status:         defaultString(request.Status, "wanted"),
		Monitored:      monitored,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}, nil
}

func (fakeWanted) DeleteWanted(context.Context, string) error {
	return nil
}

func (fakeWanted) UpdateAuthorSubscription(_ context.Context, _ string, request wanted.AuthorUpdateRequest) (wanted.AuthorSubscription, error) {
	status := defaultString(request.Status, "monitored")
	if request.Monitored != nil && !*request.Monitored {
		status = "unmonitored"
	}
	return wanted.AuthorSubscription{
		ID:              "author-sub-1",
		Provider:        "Open Library",
		ProviderKey:     "openlibrary:OL123A",
		AuthorName:      defaultString(request.AuthorName, "Andy Weir"),
		Format:          "ebook",
		QualityProfile:  defaultString(request.QualityProfile, "standard"),
		Status:          status,
		MonitorNewItems: request.MonitorNewItems == nil || *request.MonitorNewItems,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}, nil
}

func (fakeWanted) DeleteAuthorSubscription(context.Context, string) error {
	return nil
}

type fakeMutableWanted struct {
	fakeWanted
	authorUpdates []struct {
		id      string
		request wanted.AuthorUpdateRequest
	}
	authorDeletes []string
	wantedUpdates []struct {
		id      string
		request wanted.WantedUpdateRequest
	}
	wantedDeletes []string
}

func (f *fakeMutableWanted) UpdateAuthorSubscription(_ context.Context, id string, request wanted.AuthorUpdateRequest) (wanted.AuthorSubscription, error) {
	f.authorUpdates = append(f.authorUpdates, struct {
		id      string
		request wanted.AuthorUpdateRequest
	}{id: id, request: request})
	return fakeWanted{}.UpdateAuthorSubscription(context.Background(), id, request)
}

func (f *fakeMutableWanted) DeleteAuthorSubscription(_ context.Context, id string) error {
	f.authorDeletes = append(f.authorDeletes, id)
	return nil
}

func (f *fakeMutableWanted) UpdateWanted(_ context.Context, id string, request wanted.WantedUpdateRequest) (wanted.WantedItem, error) {
	f.wantedUpdates = append(f.wantedUpdates, struct {
		id      string
		request wanted.WantedUpdateRequest
	}{id: id, request: request})
	return fakeWanted{}.UpdateWanted(context.Background(), id, request)
}

func (f *fakeMutableWanted) DeleteWanted(_ context.Context, id string) error {
	f.wantedDeletes = append(f.wantedDeletes, id)
	return nil
}

type fakeReleaseWanted struct {
	fakeWanted
	grabs []wanted.GrabRequest
}

func (f *fakeReleaseWanted) Grab(_ context.Context, _ string, request wanted.GrabRequest) (acquisition.DownloadStatus, error) {
	f.grabs = append(f.grabs, request)
	return acquisition.DownloadStatus{
		Client:   "qBittorrent",
		ID:       "download-1",
		Name:     "Project Hail Mary EPUB",
		State:    "stoppedDL",
		SavePath: "/downloads/books",
		Category: "books-ebook",
		Tags:     []string{"librarry", "wanted:wanted-1"},
	}, nil
}

type fakeRejectedReleaseWanted struct {
	fakeWanted
}

func (fakeRejectedReleaseWanted) SearchReleases(context.Context, string, wanted.SearchReleasesRequest) (wanted.SearchOutcome, error) {
	outcome := fakeWantedSearchOutcome()
	outcome.Releases[0].Approved = false
	outcome.Releases[0].RejectedReason = "release is rejected"
	return outcome, nil
}

func (fakeRejectedReleaseWanted) ListReleases(context.Context, string) (wanted.SearchOutcome, error) {
	outcome := fakeWantedSearchOutcome()
	outcome.Releases[0].Approved = false
	outcome.Releases[0].RejectedReason = "release is rejected"
	return outcome, nil
}

func (fakeRejectedReleaseWanted) Grab(context.Context, string, wanted.GrabRequest) (acquisition.DownloadStatus, error) {
	return acquisition.DownloadStatus{}, errors.New("release is rejected: release is rejected")
}

func fakeWantedSearchOutcome() wanted.SearchOutcome {
	now := time.Now().UTC()
	item := wanted.WantedItem{
		ID:             "wanted-1",
		WorkID:         "openlibrary:OL1W",
		Title:          "Project Hail Mary",
		AuthorName:     "Andy Weir",
		Format:         "ebook",
		QualityProfile: "standard",
		Status:         "wanted",
		Monitored:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return wanted.SearchOutcome{
		WantedItem: item,
		Releases: []wanted.ReleaseDecision{{
			ID:           "release-1",
			WantedItemID: item.ID,
			SourceID:     "prowlarr:release-1",
			InfoHash:     "projecthailmary",
			Indexer:      "Prowlarr",
			Title:        "Andy Weir - Project Hail Mary EPUB",
			Protocol:     "torrent",
			DownloadURL:  "magnet:?xt=urn:btih:projecthailmary",
			InfoURL:      "https://indexer.example/releases/release-1",
			SizeBytes:    7340032,
			Seeders:      12,
			Leechers:     1,
			Categories:   []string{"ebook", "epub"},
			Score:        92,
			Approved:     true,
			PublishedAt:  time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
			SearchedAt:   now,
			CreatedAt:    now,
		}},
	}
}

type fakeCompatResources struct {
	roots            []compatdata.RootFolder
	deleted          []string
	resources        []compatdata.Resource
	deletedResources []string
}

func (f *fakeCompatResources) ListRootFolders(context.Context) ([]compatdata.RootFolder, error) {
	return append([]compatdata.RootFolder(nil), f.roots...), nil
}

func (f *fakeCompatResources) CreateRootFolder(_ context.Context, root compatdata.RootFolder) (compatdata.RootFolder, error) {
	if root.ID == "" {
		root.ID = "root-" + strconv.Itoa(len(f.roots)+1)
	}
	if root.MediaFormat == "" {
		root.MediaFormat = "mixed"
	}
	if root.Metadata == nil {
		root.Metadata = map[string]any{}
	}
	f.roots = append(f.roots, root)
	return root, nil
}

func (f *fakeCompatResources) DeleteRootFolder(_ context.Context, id string) (bool, error) {
	for index, root := range f.roots {
		if root.ID != id {
			continue
		}
		f.deleted = append(f.deleted, id)
		f.roots = append(f.roots[:index], f.roots[index+1:]...)
		return true, nil
	}
	return false, nil
}

func (f *fakeCompatResources) ListResources(_ context.Context, resourceType string) ([]compatdata.Resource, error) {
	var resources []compatdata.Resource
	for _, resource := range f.resources {
		if resource.ResourceType == resourceType {
			resources = append(resources, cloneFakeCompatResource(resource))
		}
	}
	return resources, nil
}

func (f *fakeCompatResources) GetResource(_ context.Context, resourceType string, compatID int) (compatdata.Resource, bool, error) {
	for _, resource := range f.resources {
		if resource.ResourceType == resourceType && resource.CompatID == compatID {
			return cloneFakeCompatResource(resource), true, nil
		}
	}
	return compatdata.Resource{}, false, nil
}

func (f *fakeCompatResources) UpsertResource(_ context.Context, resource compatdata.Resource) (compatdata.Resource, error) {
	if resource.CompatID <= 0 {
		for _, existing := range f.resources {
			if existing.ResourceType == resource.ResourceType && existing.CompatID > resource.CompatID {
				resource.CompatID = existing.CompatID
			}
		}
		resource.CompatID++
	}
	if resource.ID == "" {
		resource.ID = "compat-resource-" + strconv.Itoa(resource.CompatID)
	}
	if resource.Payload == nil {
		resource.Payload = map[string]any{}
	}
	for index, existing := range f.resources {
		if existing.ResourceType == resource.ResourceType && existing.CompatID == resource.CompatID {
			f.resources[index] = cloneFakeCompatResource(resource)
			return resource, nil
		}
	}
	f.resources = append(f.resources, cloneFakeCompatResource(resource))
	return resource, nil
}

func (f *fakeCompatResources) DeleteResource(_ context.Context, resourceType string, compatID int) (bool, error) {
	for index, resource := range f.resources {
		if resource.ResourceType != resourceType || resource.CompatID != compatID {
			continue
		}
		f.deletedResources = append(f.deletedResources, resourceType+":"+strconv.Itoa(compatID))
		f.resources = append(f.resources[:index], f.resources[index+1:]...)
		return true, nil
	}
	return false, nil
}

func cloneFakeCompatResource(resource compatdata.Resource) compatdata.Resource {
	cloned := resource
	cloned.Payload = map[string]any{}
	for key, value := range resource.Payload {
		cloned.Payload[key] = value
	}
	return cloned
}

type fakeBlocklistWanted struct {
	fakeWanted
}

func (fakeBlocklistWanted) History(context.Context, wanted.HistoryQuery) ([]wanted.HistoryEvent, error) {
	return []wanted.HistoryEvent{{
		ID:         "event-failed",
		EventType:  "download_failed",
		EntityType: "wanted_item",
		EntityID:   "wanted-1",
		Severity:   "error",
		Message:    "Download failed for Project Hail Mary",
		Data: map[string]any{
			"wantedId":     "wanted-1",
			"downloadId":   "failed-1",
			"releaseTitle": "Andy Weir - Project Hail Mary EPUB",
			"reason":       "missing files",
			"protocol":     "torrent",
		},
		CreatedAt: time.Date(2026, 6, 25, 12, 5, 0, 0, time.UTC),
	}}, nil
}

type fakeLibrary struct{}

func (fakeLibrary) ListFiles(context.Context, library.FileListQuery) ([]library.FileRecord, error) {
	return []library.FileRecord{fakeLibraryFile()}, nil
}

func (fakeLibrary) DeleteFiles(context.Context, library.DeleteFilesRequest) (library.DeleteFilesOutcome, error) {
	file := fakeLibraryFile()
	return library.DeleteFilesOutcome{
		Requested: 1,
		Deleted:   1,
		Files:     []library.FileRecord{file},
		Results:   []library.DeleteFileResult{{File: file, Status: "deleted"}},
	}, nil
}

func (fakeLibrary) PreviewRenameFiles(_ context.Context, request library.RenameFilesRequest) (library.RenameFilesOutcome, error) {
	return fakeRenameOutcome(request, false), nil
}

func (fakeLibrary) RenameFiles(_ context.Context, request library.RenameFilesRequest) (library.RenameFilesOutcome, error) {
	return fakeRenameOutcome(request, true), nil
}

type fakeDeleteLibrary struct {
	fakeLibrary
	requests []library.DeleteFilesRequest
}

func (f *fakeDeleteLibrary) DeleteFiles(_ context.Context, request library.DeleteFilesRequest) (library.DeleteFilesOutcome, error) {
	f.requests = append(f.requests, request)
	return fakeLibrary{}.DeleteFiles(context.Background(), request)
}

type fakeRenameLibrary struct {
	fakeLibrary
	previewRequests []library.RenameFilesRequest
	renameRequests  []library.RenameFilesRequest
}

func (f *fakeRenameLibrary) PreviewRenameFiles(_ context.Context, request library.RenameFilesRequest) (library.RenameFilesOutcome, error) {
	f.previewRequests = append(f.previewRequests, request)
	return fakeRenameOutcome(request, false), nil
}

func (f *fakeRenameLibrary) RenameFiles(_ context.Context, request library.RenameFilesRequest) (library.RenameFilesOutcome, error) {
	f.renameRequests = append(f.renameRequests, request)
	return fakeRenameOutcome(request, true), nil
}

func fakeLibraryFile() library.FileRecord {
	return library.FileRecord{
		ID:           "file-1",
		EditionID:    "openlibrary:OL1M",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		Extension:    ".epub",
		SizeBytes:    7340032,
		ImportStatus: "imported",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
}

func fakeRenameOutcome(request library.RenameFilesRequest, apply bool) library.RenameFilesOutcome {
	file := fakeLibraryFile()
	destination := "/library/ebooks/Andy Weir/Project Hail Mary/Andy Weir - Project Hail Mary.epub"
	preview := library.RenameFilePreview{
		File:            file,
		SourcePath:      file.Path,
		DestinationPath: destination,
		RelativePath:    "Andy Weir/Project Hail Mary/Andy Weir - Project Hail Mary.epub",
	}
	requested := len(request.IDs) + len(request.Paths)
	if requested == 0 {
		requested = 1
	}
	outcome := library.RenameFilesOutcome{
		Requested: requested,
		Previews:  []library.RenameFilePreview{preview},
	}
	if !apply {
		return outcome
	}
	renamed := file
	renamed.Path = destination
	outcome.Renamed = 1
	outcome.Results = []library.RenameFileResult{{Preview: preview, File: &renamed, Status: "renamed"}}
	return outcome
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
