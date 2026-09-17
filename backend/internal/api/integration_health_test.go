package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/metadata"
)

func TestIntegrationStatusReadsDoNotProbeAndExplicitCheckIsProtected(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"appName":"Prowlarr","version":"2.3.4-private-build"}`)
	}))
	defer server.Close()
	acquire := acquisition.NewService(acquisition.IntegrationConfig{ProwlarrURL: server.URL, ProwlarrAPIKey: "fixture"})
	deps := Dependencies{Acquire: acquire, Config: config.Config{APIKey: "fixture-key", EbookLibraryRoot: t.TempDir(), AudiobookLibraryRoot: t.TempDir()}, Metadata: metadata.NewService(nil)}
	deps.Health = NewHealthEvaluator(deps)
	router := NewRouter(deps)
	call := func(method, path string, authorized bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		if authorized {
			req.Header.Set("X-Api-Key", "fixture-key")
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	for _, path := range []string{"/api/v1/integrations/health", "/api/v1/readiness", "/api/v1/health", "/api/v1/system/health", "/api/v1/system/support"} {
		if res := call("GET", path, true); res.Code != 200 {
			t.Fatal(path, res.Code, res.Body.String())
		}
	}
	if calls.Load() != 0 {
		t.Fatal("status read performed a remote check")
	}
	if res := call("POST", "/api/v1/integrations/Prowlarr/check", false); res.Code != 401 {
		t.Fatal(res.Code)
	}
	if res := call("POST", "/api/v1/integrations/unknown/check", true); res.Code != 404 {
		t.Fatal(res.Code)
	}
	res := call("POST", "/api/v1/integrations/Prowlarr/check", true)
	var observed acquisition.IntegrationHealth
	if err := json.Unmarshal(res.Body.Bytes(), &observed); err != nil || res.Code != 200 || observed.Status != "ready" || observed.LastCheckedAt == nil {
		t.Fatal(res.Code, res.Body.String(), err)
	}
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
	for range 3 {
		support := call("GET", "/api/v1/system/support", true)
		var report supportReport
		if err := json.Unmarshal(support.Body.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		integration := report.Integrations[0]
		if integration.Status != "ready" || integration.Version != "2.3.4" || integration.LastCheckedAt == nil || !integration.LastCheckedAt.Equal(*observed.LastCheckedAt) {
			t.Fatal(integration)
		}
		if strings.Contains(support.Body.String(), server.URL) || strings.Contains(support.Body.String(), "private-build") {
			t.Fatal("support leaked private version or endpoint")
		}
		call("GET", "/api/v1/system/health", true)
	}
	if calls.Load() != 1 {
		t.Fatal("status read changed request evidence")
	}
}

func TestHealthWorkerPerformsChecksWhileUnknownEvidenceIsNotAnOutage(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, "v5.0.4") }))
	defer server.Close()
	acquire := acquisition.NewService(acquisition.IntegrationConfig{QBittorrentURL: server.URL})
	deps := Dependencies{Acquire: acquire, Config: config.Config{EbookLibraryRoot: t.TempDir(), AudiobookLibraryRoot: t.TempDir()}}
	h := handler{deps: deps}
	before := downloadClientHealthCheck(h.healthInputs(context.Background()))
	if before.Severity != healthSeverityWarning || strings.Contains(before.Message, "unreachable") {
		t.Fatal(before)
	}
	evaluator := NewHealthEvaluator(deps)
	checks := evaluator.Evaluate(context.Background())
	if calls.Load() != 1 || checkSeverities(checks)["download-client"] != healthSeverityOK {
		t.Fatal(calls.Load(), checks)
	}
}
