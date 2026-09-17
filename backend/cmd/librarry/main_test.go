package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
)

func TestCalibreConversionRefreshScheduleDefaultsAndBounds(t *testing.T) {
	interval, request := calibreConversionRefreshSchedule(config.Config{})
	if interval != 15*time.Minute {
		t.Fatalf("expected default interval, got %s", interval)
	}
	if request.Limit != 200 || request.MaxAttempts != 1 || request.Force {
		t.Fatalf("unexpected default request: %+v", request)
	}

	interval, request = calibreConversionRefreshSchedule(config.Config{
		CalibreRefreshInterval:    2 * time.Hour,
		CalibreRefreshLimit:       1000,
		CalibreRefreshMaxAttempts: 99,
	})
	if interval != 2*time.Hour {
		t.Fatalf("expected configured interval, got %s", interval)
	}
	if request.Limit != 500 || request.MaxAttempts != 10 {
		t.Fatalf("expected bounded request, got %+v", request)
	}
}

func TestRunCalibreConversionRefreshOnceUsesConfiguredRequest(t *testing.T) {
	service := &fakeCalibreConversionRefreshService{
		outcome: library.CalibreConversionRefreshOutcome{Checked: 3, Refreshed: 2, Skipped: 1},
	}
	outcome, err := runCalibreConversionRefreshOnce(
		context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		service,
		config.Config{
			CalibreRefreshLimit:       17,
			CalibreRefreshMaxAttempts: 4,
		},
		"scheduled",
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Checked != 3 || outcome.Refreshed != 2 || outcome.Skipped != 1 {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if len(service.requests) != 1 {
		t.Fatalf("expected one refresh request, got %d", len(service.requests))
	}
	request := service.requests[0]
	if request.Limit != 17 || request.MaxAttempts != 4 || request.Force {
		t.Fatalf("unexpected refresh request: %+v", request)
	}
}

type fakeCalibreConversionRefreshService struct {
	requests []library.CalibreConversionRefreshRequest
	outcome  library.CalibreConversionRefreshOutcome
}

func (f *fakeCalibreConversionRefreshService) RefreshCalibreConversions(_ context.Context, request library.CalibreConversionRefreshRequest) (library.CalibreConversionRefreshOutcome, error) {
	f.requests = append(f.requests, request)
	return f.outcome, nil
}

func TestCompletedImportScheduleDefaults(t *testing.T) {
	if interval := completedImportInterval(config.Config{}); interval != time.Minute {
		t.Fatalf("expected one-minute default interval, got %s", interval)
	}
	if interval := completedImportInterval(config.Config{CompletedImportInterval: 5 * time.Minute}); interval != 5*time.Minute {
		t.Fatalf("expected configured interval, got %s", interval)
	}
	if limit := completedImportLimit(config.Config{}); limit != 50 {
		t.Fatalf("expected default limit 50, got %d", limit)
	}
	if limit := completedImportLimit(config.Config{CompletedImportLimit: 10}); limit != 10 {
		t.Fatalf("expected configured limit, got %d", limit)
	}
}

func TestRunCompletedDownloadImportOnceScopesToLibrarryTag(t *testing.T) {
	lister := &fakeCompletedDownloadLister{
		rows: []acquisition.DownloadStatus{{ID: "a", Tags: []string{"librarry", "wanted:w-1"}}},
	}
	service := &fakeCompletedImportService{
		outcome: library.CompletedImportOutcome{Checked: 1, Imported: 1},
	}
	outcome, err := runCompletedDownloadImportOnce(context.Background(), service, lister, config.Config{
		CompletedImportLimit: 25,
		CompletedImportMode:  "hardlinkOrCopy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Imported != 1 {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if len(lister.queries) != 1 || lister.queries[0].Tag != "librarry" {
		t.Fatalf("expected a librarry-tag scoped listing, got %+v", lister.queries)
	}
	if len(service.requests) != 1 || service.requests[0].Limit != 25 || service.requests[0].ImportMode != "hardlinkOrCopy" {
		t.Fatalf("expected limit and mode passed through, got %+v", service.requests)
	}
	if len(service.downloads) != 1 || len(service.downloads[0]) != 1 || service.downloads[0][0].ID != "a" {
		t.Fatalf("expected listed downloads forwarded, got %+v", service.downloads)
	}
}

type fakeCompletedDownloadLister struct {
	rows    []acquisition.DownloadStatus
	queries []acquisition.DownloadListQuery
	actions []acquisition.DownloadActionRequest
}

func (f *fakeCompletedDownloadLister) Downloads(_ context.Context, query acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error) {
	f.queries = append(f.queries, query)
	return f.rows, nil
}

func (f *fakeCompletedDownloadLister) DownloadAction(_ context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error) {
	f.actions = append(f.actions, request)
	return acquisition.DownloadActionResult{Action: request.Action, IDs: request.IDs, Applied: true}, nil
}

func TestCompletedDownloadRemovalEligible(t *testing.T) {
	cases := []struct {
		name     string
		download acquisition.DownloadStatus
		eligible bool
	}{
		{"qbit 5.x stoppedUP imported", acquisition.DownloadStatus{Client: "qBittorrent", State: "stoppedUP", ImportStatus: "imported", Progress: 1, SeedGoalMet: true}, true},
		{"qbit 4.x pausedUP imported", acquisition.DownloadStatus{Client: "qBittorrent", State: "pausedUP", ImportStatus: "imported", Progress: 1, SeedGoalMet: true}, true},
		{"qbit still uploading", acquisition.DownloadStatus{Client: "qBittorrent", State: "uploading", ImportStatus: "imported", Progress: 1, SeedGoalMet: true}, false},
		{"qbit stopped but not imported", acquisition.DownloadStatus{Client: "qBittorrent", State: "stoppedUP", ImportStatus: "ready", Progress: 1}, false},
		{"qbit stopped pending import", acquisition.DownloadStatus{Client: "qBittorrent", State: "stoppedUP", Progress: 1}, false},
		{"transmission stopped and done", acquisition.DownloadStatus{Client: "Transmission", State: "completed", ImportStatus: "imported", Progress: 1, SeedGoalMet: true}, true},
		{"transmission stopped mid-download", acquisition.DownloadStatus{Client: "Transmission", State: "stopped", ImportStatus: "imported", Progress: 0.4}, false},
		{"transmission still seeding", acquisition.DownloadStatus{Client: "Transmission", State: "seeding", ImportStatus: "imported", Progress: 1, SeedGoalMet: true}, false},
		{"sabnzbd completed import stays", acquisition.DownloadStatus{Client: "SABnzbd", State: "completed", ImportStatus: "imported", Progress: 1, SeedGoalMet: true}, false},
	}
	for _, testCase := range cases {
		if got := completedDownloadRemovalEligible(testCase.download); got != testCase.eligible {
			t.Fatalf("%s: expected eligible=%v, got %v", testCase.name, testCase.eligible, got)
		}
	}
}

func TestRunCompletedDownloadRemovalOnceDeletesEligibleDownloads(t *testing.T) {
	client := &fakeCompletedDownloadLister{
		rows: []acquisition.DownloadStatus{
			{Client: "qBittorrent", ID: "keep-seeding", State: "uploading", ImportStatus: "imported", Progress: 1, SeedGoalMet: true, Tags: []string{"librarry"}},
			{Client: "qBittorrent", ID: "done-1", State: "stoppedUP", ImportStatus: "imported", Progress: 1, SeedGoalMet: true, Tags: []string{"librarry"}},
			{Client: "Transmission", ID: "done-2", State: "completed", ImportStatus: "imported", Progress: 1, SeedGoalMet: true, Tags: []string{"librarry"}},
		},
	}
	removed, err := runCompletedDownloadRemovalOnce(context.Background(), client, fakeCompletedVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 removals, got %d", removed)
	}
	if len(client.actions) != 2 {
		t.Fatalf("expected 2 delete actions, got %+v", client.actions)
	}
	for _, action := range client.actions {
		if action.Action != acquisition.DownloadActionDelete || !action.DeleteFiles {
			t.Fatalf("expected delete-with-data action, got %+v", action)
		}
	}
	if client.actions[0].IDs[0] != "done-1" || client.actions[0].Client != "qBittorrent" {
		t.Fatalf("expected qBittorrent removal first, got %+v", client.actions[0])
	}
	if client.actions[1].IDs[0] != "done-2" || client.actions[1].Client != "Transmission" {
		t.Fatalf("expected Transmission removal second, got %+v", client.actions[1])
	}
}

type fakeCompletedImportService struct {
	requests  []library.CompletedImportRequest
	downloads [][]acquisition.DownloadStatus
	outcome   library.CompletedImportOutcome
}

func (f *fakeCompletedImportService) ImportCompletedDownloads(_ context.Context, downloads []acquisition.DownloadStatus, request library.CompletedImportRequest) (library.CompletedImportOutcome, error) {
	f.downloads = append(f.downloads, downloads)
	f.requests = append(f.requests, request)
	return f.outcome, nil
}

type fakeCompletedVerifier struct{}

func (fakeCompletedVerifier) VerifyCompletedDownload(context.Context, acquisition.DownloadStatus, []acquisition.DownloadFile) error {
	return nil
}
func (f *fakeCompletedDownloadLister) DownloadDetails(context.Context, string, string) (acquisition.DownloadDetails, error) {
	return acquisition.DownloadDetails{}, nil
}

func TestPausedDownloadDoesNotProveSeedGoal(t *testing.T) {
	for _, client := range []string{"qBittorrent", "Transmission"} {
		if completedDownloadRemovalEligible(acquisition.DownloadStatus{Client: client, State: "pausedUP", ImportStatus: "imported", Progress: 1}) {
			t.Fatal("pause authorized cleanup")
		}
	}
}

func TestBootAuthenticationRejectsUnknownMode(t *testing.T) {
	for _, mode := range []string{"formz", "false", "invalid"} {
		if _, err := bootAuthMethod(context.Background(), nil, config.Config{AuthMethod: mode}); err == nil {
			t.Fatalf("accepted %q", mode)
		}
	}
	for _, mode := range []string{"none", "forms", "basic"} {
		got, err := bootAuthMethod(context.Background(), nil, config.Config{AuthMethod: mode})
		if err != nil || got != mode {
			t.Fatalf("%s: %s %v", mode, got, err)
		}
	}
}

type cleanupFailureClient struct {
	fakeCompletedDownloadLister
	inventoryErr error
	apply        bool
}

func (f *cleanupFailureClient) DownloadDetails(context.Context, string, string) (acquisition.DownloadDetails, error) {
	return acquisition.DownloadDetails{}, f.inventoryErr
}
func (f *cleanupFailureClient) DownloadAction(_ context.Context, r acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error) {
	f.actions = append(f.actions, r)
	return acquisition.DownloadActionResult{Applied: f.apply}, nil
}

type cleanupRecordingVerifier struct {
	fakeCompletedVerifier
	errors []error
}

func (f *cleanupRecordingVerifier) RecordCompletedCleanup(_ context.Context, _ acquisition.DownloadStatus, err error) error {
	f.errors = append(f.errors, err)
	return nil
}
func TestCleanupFailuresArePersistedAndReported(t *testing.T) {
	for _, inventoryFailure := range []bool{false, true} {
		client := &cleanupFailureClient{fakeCompletedDownloadLister: fakeCompletedDownloadLister{rows: []acquisition.DownloadStatus{{Client: "qBittorrent", ID: "fixture", State: "stoppedUP", ImportStatus: "imported", Progress: 1, SeedGoalMet: true}}}}
		if inventoryFailure {
			client.inventoryErr = io.ErrUnexpectedEOF
		}
		service := &cleanupRecordingVerifier{}
		count, err := runCompletedDownloadRemovalOnce(context.Background(), client, service)
		if count != 0 || err == nil || len(service.errors) != 1 || service.errors[0] == nil {
			t.Fatalf("failure lost: %d %v %+v", count, err, service.errors)
		}
		if inventoryFailure && len(client.actions) != 0 {
			t.Fatal("inventory failure allowed deletion")
		}
	}
}
