package api

import (
	"context"
	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/wanted"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAPINotificationsSkipReplayedOperations(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count.Add(1); w.WriteHeader(202) }))
	defer server.Close()
	h := &handler{deps: Dependencies{Compat: fakeNotificationCompat(server.URL, map[string]any{"onUpgrade": true, "onDownloadFailure": true})}}
	ctx := context.Background()
	h.notifyReleaseImport(ctx, "fixture", library.ImportOutcome{Imported: true, Skipped: true})
	h.notifyReleaseImport(ctx, "fixture", library.ImportOutcome{})
	replay := &acquisition.DownloadStatus{Deduplicated: true}
	h.notifyDownloadGrab(ctx, "fixture", *replay, "")
	h.notifyMonitorGrabs(ctx, "fixture", wanted.MonitorRun{Items: []wanted.MonitorItemResult{{GrabbedDownload: replay}}})
	h.notifyFeedGrabs(ctx, "fixture", wanted.FeedSyncRun{Matches: []wanted.FeedSyncMatch{{GrabbedDownload: replay}}})
	h.notifyUpgradeGrabs(ctx, "fixture", wanted.UpgradeRun{Items: []wanted.UpgradeItemResult{{GrabbedDownload: replay}}})
	if count.Load() != 0 {
		t.Fatal("replayed operation sent webhook", count.Load())
	}
	h.notifyReleaseImport(ctx, "fixture", library.ImportOutcome{Imported: true})
	if count.Load() != 1 {
		t.Fatal("fresh import was suppressed", count.Load())
	}
}
