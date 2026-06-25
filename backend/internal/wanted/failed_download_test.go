package wanted

import (
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func TestFailedDownloadReasonDetectsMissingFiles(t *testing.T) {
	reason := failedDownloadReason(acquisition.DownloadStatus{
		State:    "missingFiles",
		Progress: 0.4,
	}, time.Hour, time.Now().UTC())
	if !strings.Contains(reason, "missing files") {
		t.Fatalf("expected missing files reason, got %q", reason)
	}
}

func TestFailedDownloadReasonDetectsOldStalledDownload(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	lastActivity := now.Add(-26 * time.Hour)
	reason := failedDownloadReason(acquisition.DownloadStatus{
		State:          "stalledDL",
		Progress:       0.25,
		Seeders:        0,
		DownloadRate:   0,
		LastActivityAt: &lastActivity,
	}, 24*time.Hour, now)
	if !strings.Contains(reason, "stalled with no seeders") {
		t.Fatalf("expected stalled reason, got %q", reason)
	}
}

func TestFailedDownloadReasonIgnoresActiveStalledDownload(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	lastActivity := now.Add(-26 * time.Hour)
	reason := failedDownloadReason(acquisition.DownloadStatus{
		State:          "stalledDL",
		Progress:       0.25,
		Seeders:        4,
		DownloadRate:   0,
		LastActivityAt: &lastActivity,
	}, 24*time.Hour, now)
	if reason != "" {
		t.Fatalf("expected active stalled download to be ignored, got %q", reason)
	}
}

func TestFirstApprovedReplacementSkipsFailedHash(t *testing.T) {
	failed := acquisition.DownloadStatus{ID: "failed-hash"}
	release, ok := firstApprovedReplacement([]ReleaseDecision{
		{ID: "same", InfoHash: "failed-hash", Approved: true, Title: "same"},
		{ID: "next", InfoHash: "new-hash", Approved: true, Title: "next"},
	}, failed)
	if !ok {
		t.Fatal("expected replacement")
	}
	if release.ID != "next" {
		t.Fatalf("expected new release, got %+v", release)
	}
}
