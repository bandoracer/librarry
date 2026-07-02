package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/scheduler"
)

func TestSystemTasksEndpointListsRegisteredTasks(t *testing.T) {
	registry := scheduler.NewRegistry(slog.Default())
	if err := registry.Register(scheduler.Task{
		ID:       "wanted-monitor",
		Name:     "Wanted Monitor",
		Interval: 30 * time.Minute,
		Run:      func(context.Context, string) (string, error) { return "ok", nil },
	}); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{
		Logger:    slog.Default(),
		Config:    config.Config{WebOrigin: "*"},
		Scheduler: registry,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/tasks", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var payload struct {
		Tasks []scheduler.TaskStatus `json:"tasks"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Tasks) != 1 {
		t.Fatalf("expected one task, got %+v", payload.Tasks)
	}
	task := payload.Tasks[0]
	if task.ID != "wanted-monitor" || task.Name != "Wanted Monitor" || task.Interval != "30m" || task.Running {
		t.Fatalf("unexpected task record: %+v", task)
	}
}

func TestSystemTasksEndpointWithoutRegistryReturnsEmptyList(t *testing.T) {
	router := NewRouter(Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*"}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/tasks", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if body := res.Body.String(); !json.Valid([]byte(body)) || body == "" {
		t.Fatalf("unexpected body: %s", body)
	}
	var payload struct {
		Tasks []scheduler.TaskStatus `json:"tasks"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Tasks == nil || len(payload.Tasks) != 0 {
		t.Fatalf("expected empty task list, got %+v", payload.Tasks)
	}
}

func TestRunSystemTaskAcceptsBusyAndUnknownStates(t *testing.T) {
	registry := scheduler.NewRegistry(slog.Default())
	release := make(chan struct{})
	started := make(chan struct{})
	if err := registry.Register(scheduler.Task{
		ID:       "feed-sync",
		Name:     "Feed Sync",
		Interval: time.Hour,
		Run: func(context.Context, string) (string, error) {
			close(started)
			<-release
			return "done", nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{
		Logger:    slog.Default(),
		Config:    config.Config{WebOrigin: "*"},
		Scheduler: registry,
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/system/tasks/feed-sync/run", nil))
	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", res.Code, res.Body.String())
	}
	if body := res.Body.String(); !json.Valid([]byte(body)) {
		t.Fatalf("invalid body: %s", body)
	}
	var accepted map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &accepted)
	if accepted["started"] != true {
		t.Fatalf("expected started:true, got %v", accepted)
	}

	<-started
	res = httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/system/tasks/feed-sync/run", nil))
	if res.Code != http.StatusConflict {
		t.Fatalf("expected 409 while running, got %d", res.Code)
	}
	var conflict map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &conflict)
	if conflict["error"] != "task is running" {
		t.Fatalf("unexpected conflict body: %v", conflict)
	}
	close(release)

	res = httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/system/tasks/does-not-exist/run", nil))
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown task, got %d", res.Code)
	}
}

func TestSystemHealthEndpointReturnsAllChecks(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:  slog.Default(),
		Config:  config.Config{WebOrigin: "*", CompletedImportEnabled: true},
		Acquire: acquisition.NewService(acquisition.IntegrationConfig{}),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/health", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var payload struct {
		Checks []HealthCheck `json:"checks"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	bySeverity := map[string]string{}
	for _, check := range payload.Checks {
		bySeverity[check.ID] = check.Severity
	}
	if bySeverity["database"] != "warning" {
		t.Fatalf("expected database warning without persistence, got %+v", payload.Checks)
	}
	if bySeverity["indexer"] != "error" || bySeverity["download-client"] != "error" {
		t.Fatalf("expected unconfigured indexer/client errors, got %+v", payload.Checks)
	}
	if bySeverity["completed-import"] != "ok" {
		t.Fatalf("expected completed-import ok, got %+v", payload.Checks)
	}
}

func TestSystemDiskspaceEndpointReportsRoots(t *testing.T) {
	dir := t.TempDir()
	router := NewRouter(Dependencies{
		Logger: slog.Default(),
		Config: config.Config{WebOrigin: "*"},
		Library: library.NewService(nil, library.Config{
			EbookRoot:     dir,
			AudiobookRoot: dir,
		}, nil, nil),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/diskspace", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var payload struct {
		Disks []library.DiskSpace `json:"disks"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Disks) != 1 {
		t.Fatalf("expected the shared temp filesystem to dedupe to one disk, got %+v", payload.Disks)
	}
	disk := payload.Disks[0]
	if disk.Path != dir || disk.TotalBytes <= 0 {
		t.Fatalf("unexpected disk record: %+v", disk)
	}
}
