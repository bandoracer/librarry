package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/metadata"
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

