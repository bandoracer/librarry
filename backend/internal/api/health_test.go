package api

import (
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/library"
)

func checkSeverities(checks []HealthCheck) map[string]string {
	severities := map[string]string{}
	for _, check := range checks {
		severities[check.ID] = check.Severity
	}
	return severities
}

func TestEvaluateHealthChecksHealthySystem(t *testing.T) {
	checks := evaluateHealthChecks(healthInputs{
		databaseConfigured:    true,
		integrationsAvailable: true,
		integrations: []acquisition.IntegrationHealth{
			{Name: "Prowlarr", Configured: true, Status: "ready", Message: "9 indexers"},
			{Name: "qBittorrent", Configured: true, Status: "ready"},
		},
		roots: []healthRoot{
			{Path: "/data/media/books/ebooks", Label: "Ebooks", Accessible: true},
		},
		disks: []library.DiskSpace{
			{Path: "/data/media/books/ebooks", FreeBytes: 100 << 30, TotalBytes: 500 << 30},
		},
		completedImportEnabled:   true,
		qualityProfilesAvailable: true,
		qualityProfileCount:      2,
	})
	for _, check := range checks {
		if check.Severity != healthSeverityOK {
			t.Fatalf("expected all ok, got %+v", check)
		}
	}
	if len(checks) != 7 {
		t.Fatalf("expected 7 checks, got %d: %+v", len(checks), checks)
	}
}

func TestEvaluateHealthChecksUnconfiguredSystem(t *testing.T) {
	severities := checkSeverities(evaluateHealthChecks(healthInputs{}))
	expectations := map[string]string{
		"database":         healthSeverityWarning,
		"indexer":          healthSeverityError,
		"download-client":  healthSeverityError,
		"root-folders":     healthSeverityError,
		"completed-import": healthSeverityWarning,
		"quality-profiles": healthSeverityWarning,
	}
	for id, expected := range expectations {
		if severities[id] != expected {
			t.Fatalf("check %s: expected %s, got %s", id, expected, severities[id])
		}
	}
}

func TestEvaluateHealthChecksUnreachableIntegrations(t *testing.T) {
	checks := evaluateHealthChecks(healthInputs{
		databaseConfigured:    true,
		integrationsAvailable: true,
		integrations: []acquisition.IntegrationHealth{
			{Name: "Prowlarr", Configured: true, Status: "error", Message: "connection refused"},
			{Name: "qBittorrent", Configured: true, Status: "error", Message: "login failed"},
		},
		completedImportEnabled:   true,
		qualityProfilesAvailable: true,
		qualityProfileCount:      1,
	})
	severities := checkSeverities(checks)
	if severities["indexer"] != healthSeverityError {
		t.Fatalf("expected unreachable indexer error, got %+v", checks)
	}
	if severities["download-client"] != healthSeverityError {
		t.Fatalf("expected unreachable client error, got %+v", checks)
	}
}

func TestEvaluateHealthChecksRootAndDiskStates(t *testing.T) {
	checks := evaluateHealthChecks(healthInputs{
		databaseConfigured: true,
		roots: []healthRoot{
			{Path: "/roots/good", Accessible: true},
			{Path: "/roots/missing", Accessible: false},
		},
		disks: []library.DiskSpace{
			{Path: "/roots/good", FreeBytes: 512 << 20, TotalBytes: 100 << 30}, // < 1 GiB
			{Path: "/roots/tight", FreeBytes: 3 << 30, TotalBytes: 100 << 30},  // < 5 GiB
			{Path: "/roots/roomy", FreeBytes: 50 << 30, TotalBytes: 100 << 30}, // fine
		},
		completedImportEnabled:   true,
		qualityProfilesAvailable: true,
		qualityProfileCount:      1,
	})
	severities := checkSeverities(checks)
	if severities["root-folder:/roots/good"] != healthSeverityOK {
		t.Fatalf("expected accessible root ok, got %+v", checks)
	}
	if severities["root-folder:/roots/missing"] != healthSeverityError {
		t.Fatalf("expected missing root error, got %+v", checks)
	}
	if severities["disk-space:/roots/good"] != healthSeverityError {
		t.Fatalf("expected sub-1GiB error, got %+v", checks)
	}
	if severities["disk-space:/roots/tight"] != healthSeverityWarning {
		t.Fatalf("expected sub-5GiB warning, got %+v", checks)
	}
	if severities["disk-space:/roots/roomy"] != healthSeverityOK {
		t.Fatalf("expected roomy disk ok, got %+v", checks)
	}
}

func TestHealthTransitionsDispatchOnlyOkToBad(t *testing.T) {
	checks := []HealthCheck{
		{ID: "indexer", Severity: healthSeverityError, Name: "Indexer", Message: "unreachable"},
		{ID: "database", Severity: healthSeverityOK, Name: "Database persistence"},
	}

	// First evaluation: unseen bad checks notify.
	state, events := healthTransitions(map[string]string{}, checks)
	if len(events) != 1 {
		t.Fatalf("expected one event for the new issue, got %d", len(events))
	}
	if events[0].Fields["severity"] != healthSeverityError {
		t.Fatalf("unexpected event: %+v", events[0])
	}
	if state["indexer"] != healthSeverityError || state["database"] != healthSeverityOK {
		t.Fatalf("unexpected state: %+v", state)
	}

	// Still bad: no re-notification.
	state, events = healthTransitions(state, checks)
	if len(events) != 0 {
		t.Fatalf("expected no repeat events, got %+v", events)
	}

	// Recovered then broken again: notifies once more.
	recovered := []HealthCheck{
		{ID: "indexer", Severity: healthSeverityOK, Name: "Indexer"},
		{ID: "database", Severity: healthSeverityOK, Name: "Database persistence"},
	}
	state, events = healthTransitions(state, recovered)
	if len(events) != 0 {
		t.Fatalf("recovery must not notify, got %+v", events)
	}
	_, events = healthTransitions(state, checks)
	if len(events) != 1 {
		t.Fatalf("expected re-break to notify, got %d", len(events))
	}
}
