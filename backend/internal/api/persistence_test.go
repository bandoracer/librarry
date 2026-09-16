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
	"github.com/bandoracer/librarry/backend/internal/auth"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestFreshPostgresListAndAuthenticationContracts(t *testing.T) {
	db := testdb.Open(t)
	acquire := acquisition.NewService(acquisition.IntegrationConfig{DownloadStore: acquisition.NewSQLDownloadStore(db)})
	wantedStore := wanted.NewStore(db)
	service := auth.NewService(auth.NewStore(db), slog.Default())
	deps := Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*"}, Metadata: metadata.NewService(nil), Acquire: acquire, Wanted: wanted.NewService(wantedStore, acquire), Library: library.NewService(library.NewStore(db), library.Config{}, wantedStore, nil), Compat: compatdata.NewStore(db), Auth: service}
	router := NewRouter(deps)
	for path, key := range map[string]string{"/api/v1/wanted?view=library": "wanted", "/api/v1/authors": "authors", "/api/v1/library/files": "files"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		var payload map[string]json.RawMessage
		if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &payload) != nil || string(payload[key]) != "[]" {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPut, "/api/v1/auth/config", strings.NewReader(`{"method":"forms","username":"fixture","password":"fixture-only-password"}`)))
	if res.Code != 200 || service.Method() != auth.MethodForms {
		t.Fatalf("save auth: %d %s", res.Code, res.Body.String())
	}
	saved, ok, err := deps.Compat.GetResource(context.Background(), "auth-config", 1)
	if err != nil || !ok || saved.Payload["method"] != "forms" {
		t.Fatalf("auth was not persisted: %+v %v", saved, err)
	}
	if got := requestStatus(t, router, httptest.NewRequest(http.MethodGet, "/api/v1/wanted", nil)); got != 401 {
		t.Fatalf("auth not enforced: %d", got)
	}
}

func TestStatusReportsStableBuildAndActiveAuthentication(t *testing.T) {
	service := auth.NewService(nil, slog.Default())
	service.SetMethod(auth.MethodForms)
	h := &handler{deps: Dependencies{SchemaMigration: "0029_calibre_roots.sql", Auth: service}}
	var previous string
	for i := 0; i < 2; i++ {
		res := httptest.NewRecorder()
		h.compatSystemStatus(res, httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil))
		var status map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status["authentication"] != "forms" || status["migrationVersion"] != float64(29) {
			t.Fatalf("false status: %+v", status)
		}
		timestamp := status["buildTime"].(string)
		if i > 0 && previous != timestamp {
			t.Fatal("build timestamp changes between requests")
		}
		previous = timestamp
	}
}
