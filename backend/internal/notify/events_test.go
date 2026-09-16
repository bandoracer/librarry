package notify

import (
	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/wanted"
	"testing"
)

func TestWorkerEventsDoNotRepeatReplayedImportsOrGrabs(t *testing.T) {
	fresh := &acquisition.DownloadStatus{ID: "fresh"}
	replay := &acquisition.DownloadStatus{ID: "replay", Deduplicated: true}
	for name, events := range map[string][]Event{
		"monitor": EventsFromMonitorRun("fixture", wanted.MonitorRun{Items: []wanted.MonitorItemResult{{GrabbedDownload: fresh}, {GrabbedDownload: replay}}}),
		"feed":    EventsFromFeedSyncRun("fixture", wanted.FeedSyncRun{Matches: []wanted.FeedSyncMatch{{GrabbedDownload: fresh}, {GrabbedDownload: replay}}}),
		"upgrade": EventsFromUpgradeRun("fixture", wanted.UpgradeRun{Items: []wanted.UpgradeItemResult{{GrabbedDownload: fresh}, {GrabbedDownload: replay}}}),
		"import":  EventsFromCompletedImports("fixture", library.CompletedImportOutcome{Results: []library.DownloadImportResult{{Import: &library.ImportOutcome{Imported: true}}, {Import: &library.ImportOutcome{Imported: true, Skipped: true}}, {Import: &library.ImportOutcome{}}}}),
	} {
		if len(events) != 1 {
			t.Fatalf("%s: %+v", name, events)
		}
	}
	events := EventsFromFailedDownloadRun("fixture", wanted.FailedDownloadRun{Items: []wanted.FailedDownloadResult{{ReplacementDownload: replay}}})
	if len(events) != 1 || events[0].Type != EventDownloadFailure {
		t.Fatal(events)
	}
}
