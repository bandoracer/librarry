package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/scheduler"
	"github.com/bandoracer/librarry/backend/internal/testdb"
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

func TestTaskRunHistoryRouteDoesNotReturnTaskList(t *testing.T) {
	registry := scheduler.NewRegistry(slog.Default())
	if err := registry.Register(scheduler.Task{ID: "history", Interval: time.Hour, Run: func(context.Context, string) (string, error) { return "ok", nil }}); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*"}, Scheduler: registry})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/system/tasks/history/runs", nil))
	var payload map[string]json.RawMessage
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &payload) != nil || string(payload["runs"]) != "[]" || payload["tasks"] != nil {
		t.Fatalf("%d %s", res.Code, res.Body.String())
	}
	res = httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/system/tasks/missing/runs", nil))
	if res.Code != 404 {
		t.Fatal(res.Code)
	}
}

func TestTaskHistoryPaginationAndReviewAPI(t *testing.T) {
	db := testdb.Open(t)
	registry := scheduler.NewRegistry(slog.Default()).WithDatabase(db)
	if err := registry.Register(scheduler.Task{ID: "history", Interval: time.Hour, Run: func(context.Context, string) (string, error) { return "ok", nil }}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into worker_tasks(task_id) values('history')`); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := db.QueryRow(`insert into worker_task_runs(task_id,trigger,backend_pid,state,error) values('history','fixture',0,'failed','fixture failure') returning id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*"}, Scheduler: registry})
	call := func(method, path, body string, code int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != code {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	base := "/api/v1/system/tasks/history/runs"
	for _, query := range []string{"?view=invalid", "?limit=101", "?limit=bad", "?offset=-1", "?offset=bad"} {
		call("GET", base+query, "", 400)
	}
	w := call("GET", base+"?view=unreviewed&limit=1&offset=1", "", 200)
	var page scheduler.RunPage
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Total != 1 || page.Offset != 1 || page.Limit != 1 || page.Runs == nil || len(page.Runs) != 0 {
		t.Fatal(w.Body.String(), err)
	}
	review := base + "/" + id + "/review"
	call("POST", review, `{"reviewed":true}`, 400)
	call("POST", review, `{"reviewed":true,"expectedState":"completed"}`, 409)
	call("POST", review, `{"reviewed":true,"expectedState":"failed"}`, 200)
	call("POST", review, `{"reviewed":true,"expectedState":"failed"}`, 409)
	w = call("GET", base+"?view=unreviewed", "", 200)
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Total != 0 || len(page.Runs) != 0 {
		t.Fatal(w.Body.String(), err)
	}
	call("POST", base+"/not-a-run/review", `{"reviewed":true,"expectedState":"failed"}`, 404)
	call("POST", "/api/v1/system/tasks/missing/runs/"+id+"/review", `{"reviewed":true,"expectedState":"failed"}`, 404)
}

func TestTaskAvailabilityRefusesManualRunAndCompatDoesNotInventTimes(t *testing.T) {
	registry := scheduler.NewRegistry(slog.Default())
	for _, task := range []scheduler.Task{{ID: "feed-sync", Interval: time.Minute, DisabledReason: "Disabled by fixture."}, {ID: "wanted-monitor", Interval: time.Hour, UnavailableReason: "Database required."}} {
		if err := registry.Register(task); err != nil {
			t.Fatal(err)
		}
	}
	router := NewRouter(Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*", ImportListSyncInterval: 24 * time.Hour, ImportListSyncEnabled: true, FeedSyncInterval: 15 * time.Minute}, Scheduler: registry})
	for path, code := range map[string]int{"/api/v1/system/tasks/feed-sync/run": 409, "/api/v1/system/tasks/wanted-monitor/run": 503} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != code {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/system/task", nil))
	var rows []map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &rows) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, row := range rows {
		if row["lastExecution"] != compatUnknownTaskTime || row["lastStartTime"] != compatUnknownTaskTime || row["nextExecution"] != compatUnknownTaskTime || row["lastDuration"] != "00:00:00" || row["librarryLastExecutionKnown"] != false || row["librarryLastStartTimeKnown"] != false || row["librarryNextExecutionKnown"] != false || row["librarryLastDurationKnown"] != false || row["started"] != false {
			t.Fatal("invented run evidence", row)
		}
		if row["name"] == "ImportListSync" && (row["enabled"] != true || row["interval"] != float64(1440)) {
			t.Fatal("import list borrowed feed settings", row)
		}
		if row["name"] == "RssSync" && (row["enabled"] != false || row["librarryAvailable"] != true) {
			t.Fatal(row)
		}
		if row["name"] == "MissingBookSearch" && (row["enabled"] != true || row["librarryAvailable"] != false) {
			t.Fatal(row)
		}
	}
}

func TestCompatTaskReadsSavedTimesAndRefusesDatabaseOutage(t *testing.T) {
	db := testdb.Open(t)
	registry := scheduler.NewRegistry(slog.Default()).WithDatabase(db)
	if err := registry.Register(scheduler.Task{ID: "failed-download-recovery", Interval: time.Minute, DisabledReason: "Disabled in this process."}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into worker_tasks(task_id) values('failed-download-recovery');with run as(insert into worker_task_runs(task_id,trigger,backend_pid,state,started_at,finished_at) values('failed-download-recovery','fixture',0,'completed','2026-01-01T00:00:00Z','2026-01-01T00:00:01.125Z') returning id) update worker_tasks set run_id=(select id from run),last_success_run_id=(select id from run),last_success_at='2026-01-01T00:00:01.125Z',next_run_at=now()+interval '1 hour' where task_id='failed-download-recovery'`); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*"}, Scheduler: registry})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/system/task/4", nil))
	var row map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &row) != nil || row["lastDuration"] != "00:00:01.125" || row["nextExecution"] != compatUnknownTaskTime || row["librarryNextExecutionKnown"] != false || row["librarryLastExecutionKnown"] != true || row["librarryLastStartTimeKnown"] != true || row["librarryLastDurationKnown"] != true {
		t.Fatal(w.Code, w.Body.String())
	}
	for field, want := range map[string]string{"lastStartTime": "2026-01-01T00:00:00Z", "lastExecution": "2026-01-01T00:00:01.125Z"} {
		raw, ok := row[field].(string)
		if !ok {
			t.Fatal(row)
		}
		actual, err := time.Parse(time.RFC3339Nano, raw)
		expected, _ := time.Parse(time.RFC3339Nano, want)
		if err != nil || !actual.Equal(expected) {
			t.Fatal(field, raw, err)
		}
	}
	db.Close()
	for _, path := range []string{"/api/v1/system/tasks", "/api/v1/system/task", "/api/v1/system/task/4"} {
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 503 {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
}
