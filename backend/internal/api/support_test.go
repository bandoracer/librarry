package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/buildinfo"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/scheduler"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

type supportTransport func(*http.Request) (*http.Response, error)

func (f supportTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSupportExportOmitsPrivateDataAndMakesNoExternalRequests(t *testing.T) {
	const secret = "PRIVATE-SENTINEL-DO-NOT-EXPORT"
	cfg := config.Config{}
	value := reflect.ValueOf(&cfg).Elem()
	for i := 0; i < value.NumField(); i++ {
		if value.Field(i).Kind() == reflect.String {
			value.Field(i).SetString(secret)
		}
	}
	cfg.WebOrigin = "*"
	var calls atomic.Int32
	client := &http.Client{Transport: supportTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(secret))}, nil
	})}
	hc := metadata.NewHardcoverProvider(client, secret)
	// Establish a real failed-request observation, then prove export preserves its time.
	before := hc.Check(context.Background())
	if before.Status != "unavailable" {
		t.Fatal(before)
	}
	md := metadata.NewService([]metadata.Provider{hc, metadata.NewOpenLibraryProvider(client), metadata.NewGoogleBooksProvider(client, secret), metadata.NewLocalOPFProvider()})
	registry := scheduler.NewRegistry(nil)
	if err := registry.Register(scheduler.Task{ID: "backup", Name: secret, DisabledReason: secret, UnavailableReason: secret, Interval: time.Hour}); err != nil {
		t.Fatal(err)
	}
	// Endpoint configuration contains URLs, passwords and usernames; no Health call is allowed.
	acquire := acquisition.NewService(acquisition.IntegrationConfig{ProwlarrURL: "https://" + secret + "/", ProwlarrAPIKey: secret})
	h := Dependencies{Config: cfg, Metadata: md, Acquire: acquire, Scheduler: registry, SchemaMigration: "0051_notification_retention.sql"}
	router := NewRouter(h)
	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest("GET", "/api/v1/system/support", nil))
	if unauthorized.Code != 401 {
		t.Fatal(unauthorized.Code)
	}
	req := httptest.NewRequest("GET", "/api/v1/system/support", nil)
	req.Header.Set("X-Api-Key", secret)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 200 || strings.Contains(res.Body.String(), secret) {
		t.Fatal(res.Code, res.Body.String())
	}
	if res.Header().Get("Cache-Control") != "no-store" || !strings.Contains(res.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal(res.Header())
	}
	var report supportReport
	if err := json.Unmarshal(res.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.FormatVersion != 1 || report.Build["commit"] != buildinfo.Commit || report.Build["imageDigest"] != "unknown" {
		t.Fatal(report.Build)
	}
	if len(report.Providers) != 4 || len(report.Tasks) != 1 || report.Tasks[0].Enabled || report.Tasks[0].Available {
		t.Fatal(report)
	}
	if report.Providers[0].LastCheckedAt == nil || !report.Providers[0].LastCheckedAt.Equal(*before.LastCheckedAt) || report.Providers[0].Status != "unavailable" {
		t.Fatal(report.Providers)
	}
	if report.Providers[1].LastCheckedAt != nil || report.Providers[1].Status != "configured" {
		t.Fatal(report.Providers[1])
	}
	if report.Integrations[0].Status != "unknown" || !report.Integrations[0].EndpointConfigured {
		t.Fatal(report.Integrations)
	}
	if calls.Load() != 1 {
		t.Fatalf("support export made external requests: %d", calls.Load())
	}
}

func TestSupportRootsReportCurrentEvidenceAndRecoveryWithoutPaths(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing-private-name")
	file := filepath.Join(root, "private-file")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, want string }{{root, "directory_present"}, {missing, "missing"}, {file, "not_directory"}, {"", "not_configured"}} {
		got := observeSupportRoot(context.Background(), "library-1", "ebook", tc.path)
		raw, _ := json.Marshal(got)
		if got.Status != tc.want || got.CheckedAt.IsZero() || strings.Contains(string(raw), root) {
			t.Fatal(got, string(raw))
		}
	}
	if err := os.Mkdir(missing, 0700); err != nil {
		t.Fatal(err)
	}
	if got := observeSupportRoot(context.Background(), "library-1", "ebook", missing); got.Status != "directory_present" {
		t.Fatal(got)
	}
	// Occupy every slot to model filesystem calls stuck in the kernel. New checks
	// must be bounded without starting more filesystem goroutines.
	for i := 0; i < cap(supportStatSlots); i++ {
		supportStatSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(supportStatSlots); i++ {
			<-supportStatSlots
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	got := observeSupportRoot(ctx, "library-1", "ebook", root)
	if got.Status != "timed_out" || time.Since(started) > time.Second {
		t.Fatal(got)
	}
}

func TestReadinessRequiresLivePersistenceWhileHealthIsLiveness(t *testing.T) {
	db := testdb.Open(t)
	deps := Dependencies{Database: db, Config: config.Config{APIKey: "fixture-secret"}}
	router := NewRouter(deps)
	request := func(path string, want int) {
		t.Helper()
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest("GET", path, nil))
		if res.Code != want || strings.Contains(res.Body.String(), "fixture-secret") {
			t.Fatal(res.Code, res.Body.String())
		}
	}
	request("/readyz", 200)
	request("/healthz", 200)
	// Exhaust the pool: readiness must report unavailability, not configured=true.
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest("GET", "/readyz", nil).WithContext(ctx)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 503 {
		t.Fatal(res.Code, res.Body.String())
	}
	request("/healthz", 200)
	conn.Close()
	request("/readyz", 200)
	db.Close()
	request("/readyz", 503)
	request("/healthz", 200)
}

func TestSupportPartialFailuresAndTaskErrorsRemainRedacted(t *testing.T) {
	db := testdb.Open(t)
	registry := scheduler.NewRegistry(nil).WithDatabase(db)
	if err := registry.Register(scheduler.Task{ID: "backup", Name: "Private backup name", Interval: time.Hour, Run: func(context.Context, string) (string, error) { return "", errors.New("private credential") }}); err != nil {
		t.Fatal(err)
	}
	// Seed a saved failure containing text that must not be included in the report.
	_, err := db.Exec(`insert into worker_tasks(task_id) values ('backup')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`insert into worker_task_runs(id,task_id,trigger,state,started_at,heartbeat_at,finished_at,error,details,backend_pid)
 values ('11111111-1111-4111-8111-111111111111','backup','manual','failed',now()-interval '1 minute',now(),now(),'private credential','{"errors":2,"nextAction":"private credential"}',0)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`update worker_tasks set run_id='11111111-1111-4111-8111-111111111111' where task_id='backup'`)
	if err != nil {
		t.Fatal(err)
	}
	h := handler{deps: Dependencies{Database: db, Scheduler: registry}}
	report := h.supportReport(context.Background())
	raw, _ := json.Marshal(report)
	if strings.Contains(string(raw), "private credential") || len(report.Tasks) != 1 || !report.Tasks[0].HasError || report.Tasks[0].Errors != 2 || report.Tasks[0].RunState != "failed" {
		t.Fatal(string(raw))
	}
	db.Close()
	report = h.supportReport(context.Background())
	if report.Database.Status != "unavailable" || report.Sections["tasks"] != "unavailable" || len(report.Tasks) != 0 {
		t.Fatal(report)
	}
}

func TestSupportWithoutServicesUsesEmptyArraysAndReadinessIsUnavailable(t *testing.T) {
	router := NewRouter(Dependencies{})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/system/support", nil))
	var report supportReport
	if err := json.Unmarshal(res.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Providers == nil || report.Tasks == nil || report.Database.Status != "not_configured" {
		t.Fatal(report)
	}
	res = httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest("GET", "/readyz", nil))
	if res.Code != 503 {
		t.Fatal(res.Code)
	}
}

func TestSupportRootReadDoesNotSeedOrReconfigureLibrary(t *testing.T) {
	db := testdb.Open(t)
	original := t.TempDir()
	configured := t.TempDir()
	service := library.NewService(library.NewStore(db), library.Config{EbookRoot: original, AudiobookRoot: original}, nil, nil)
	h := handler{deps: Dependencies{Database: db, Library: service}}
	first := h.supportReport(context.Background())
	if first.Sections["library"] != "available" {
		t.Fatal(first.Sections)
	}
	var count int
	if err := db.QueryRow(`select count(*) from root_folders`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("export seeded roots: count=%d err=%v", count, err)
	}
	if _, err := db.Exec(`insert into root_folders(name,path,media_format,is_default) values ('private root name',$1,'ebook',true)`, configured); err != nil {
		t.Fatal(err)
	}
	next := h.supportReport(context.Background())
	var roots []supportRoot
	for _, root := range next.Roots {
		if strings.HasPrefix(root.ID, "library-") {
			roots = append(roots, root)
		}
	}
	if len(roots) != 1 || roots[0].Role != "ebook" || roots[0].Status != "directory_present" {
		t.Fatal(roots)
	}
	if service.Config().EbookRoot != original {
		t.Fatal("export changed effective library config")
	}
	raw, _ := json.Marshal(next)
	for _, secret := range []string{original, configured, "private root name"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("root data escaped redaction")
		}
	}
}
