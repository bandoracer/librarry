package api

import (
	"context"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderCheckUsesNamedAdapterAndDoesNotInventMissingCredentialHealth(t *testing.T) {
	service := metadata.NewService([]metadata.Provider{metadata.NewHardcoverProvider(nil, "")})
	router := NewRouter(Dependencies{Config: config.Config{WebOrigin: "*"}, Metadata: service})
	for path, want := range map[string]int{"/api/v1/providers/Hardcover/check": http.StatusOK, "/api/v1/providers/not-a-provider/check": http.StatusNotFound} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
		if response.Code != want {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	h := service.Health(context.Background())[0]
	if h.Status != "missing_credentials" || h.LastCheckedAt != nil {
		t.Fatal(h)
	}
}
func TestLocalMetadataDoesNotProveRemoteReadiness(t *testing.T) {
	h := &handler{deps: Dependencies{Metadata: metadata.NewService([]metadata.Provider{metadata.NewLocalOPFProvider()})}}
	if step := h.metadataReadinessStep(context.Background()); step.Status != "blocked" {
		t.Fatal(step)
	}
}
