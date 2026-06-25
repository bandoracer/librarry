package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
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
	return []acquisition.DownloadStatus{{ID: "abc123", Name: "Book", State: "stoppedDL"}}, nil
}

func (fakeAcquire) DownloadAction(_ context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error) {
	return acquisition.DownloadActionResult{
		Action:  acquisition.DownloadActionStop,
		IDs:     request.IDs,
		Applied: true,
	}, nil
}

type fakeWanted struct{}

func (fakeWanted) Create(context.Context, wanted.CreateRequest) (wanted.WantedItem, error) {
	return wanted.WantedItem{ID: "wanted-1", Title: "Project Hail Mary", Format: "ebook", Status: "wanted"}, nil
}

func (fakeWanted) List(context.Context, string) ([]wanted.WantedItem, error) {
	return []wanted.WantedItem{{ID: "wanted-1", Title: "Project Hail Mary", Format: "ebook", Status: "wanted"}}, nil
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
