package notify

import (
	"fmt"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

// The EventsFrom* helpers convert worker run outcomes into notification
// events so scheduler-run workers and API handlers share the same shapes.

// GrabEvent describes a release handed to a download client.
func GrabEvent(source string, item *wanted.WantedItem, release *wanted.ReleaseDecision, download *acquisition.DownloadStatus) Event {
	event := Event{
		Type:   EventGrab,
		Title:  "Book grabbed",
		Fields: map[string]string{"source": source},
	}
	applyGrabDetails(&event, item, release, download)
	return event
}

// UpgradeEvent describes an upgrade grab replacing an existing file.
func UpgradeEvent(source string, item *wanted.WantedItem, release *wanted.ReleaseDecision, download *acquisition.DownloadStatus) Event {
	event := Event{
		Type:   EventUpgrade,
		Title:  "Book upgrade grabbed",
		Fields: map[string]string{"source": source},
	}
	applyGrabDetails(&event, item, release, download)
	return event
}

func applyGrabDetails(event *Event, item *wanted.WantedItem, release *wanted.ReleaseDecision, download *acquisition.DownloadStatus) {
	subject := ""
	if item != nil {
		subject = bookLabel(item.Title, item.AuthorName)
		event.Fields["wantedId"] = item.ID
	}
	releaseTitle := ""
	if release != nil {
		releaseTitle = release.Title
		if release.Indexer != "" {
			event.Fields["indexer"] = release.Indexer
		}
	}
	client := ""
	if download != nil {
		if releaseTitle == "" {
			releaseTitle = download.Name
		}
		client = download.Client
	}
	if subject == "" {
		subject = releaseTitle
	}
	if subject != "" {
		event.Title += ": " + subject
	}
	message := releaseTitle
	if client != "" {
		message = strings.TrimSpace(message + " sent to " + client)
	}
	event.Message = message
}

// ImportEvent describes a file landing in the library.
func ImportEvent(source string, outcome library.ImportOutcome) Event {
	file := outcome.File
	event := Event{
		Type:    EventImport,
		Title:   "Book imported",
		Message: outcome.DestinationPath,
		Fields:  map[string]string{"source": source},
	}
	if subject := bookLabel(file.Title, file.AuthorName); subject != "" {
		event.Title += ": " + subject
	}
	if file.MediaFormat != "" {
		event.Fields["format"] = file.MediaFormat
	}
	return event
}

// DownloadFailureEvent describes a download the recovery worker marked
// failed.
func DownloadFailureEvent(source string, item *wanted.WantedItem, download acquisition.DownloadStatus, reason string) Event {
	event := Event{
		Type:    EventDownloadFailure,
		Title:   "Download failed",
		Message: reason,
		Fields:  map[string]string{"source": source},
	}
	subject := download.Name
	if item != nil {
		if label := bookLabel(item.Title, item.AuthorName); label != "" {
			subject = label
		}
		if item.ID != "" {
			event.Fields["wantedId"] = item.ID
		}
	}
	if subject != "" {
		event.Title += ": " + subject
	}
	if download.Client != "" {
		event.Fields["client"] = download.Client
	}
	return event
}

// HealthIssueEvent describes a health check transitioning to warning/error.
func HealthIssueEvent(name string, severity string, message string) Event {
	return Event{
		Type:    EventHealthIssue,
		Title:   fmt.Sprintf("Health %s: %s", severity, name),
		Message: message,
		Fields:  map[string]string{"severity": severity},
	}
}

// TestEvent is delivered by the notification test endpoint.
func TestEvent() Event {
	return Event{
		Type:    EventTest,
		Title:   "Librarry test notification",
		Message: "The notification target is reachable.",
	}
}

// EventsFromMonitorRun collects grab events from a wanted-monitor run.
func EventsFromMonitorRun(source string, run wanted.MonitorRun) []Event {
	var events []Event
	for _, item := range run.Items {
		if item.GrabbedDownload == nil {
			continue
		}
		wantedItem := item.WantedItem
		events = append(events, GrabEvent(source, &wantedItem, nil, item.GrabbedDownload))
	}
	return events
}

// EventsFromFeedSyncRun collects grab events from a feed-sync run.
func EventsFromFeedSyncRun(source string, run wanted.FeedSyncRun) []Event {
	var events []Event
	for _, match := range run.Matches {
		if match.GrabbedDownload == nil {
			continue
		}
		wantedItem := match.WantedItem
		release := match.Release
		events = append(events, GrabEvent(source, &wantedItem, &release, match.GrabbedDownload))
	}
	return events
}

// EventsFromUpgradeRun collects upgrade events from an upgrade-search run.
func EventsFromUpgradeRun(source string, run wanted.UpgradeRun) []Event {
	var events []Event
	for _, item := range run.Items {
		if item.GrabbedDownload == nil {
			continue
		}
		wantedItem := item.WantedItem
		events = append(events, UpgradeEvent(source, &wantedItem, item.UpgradeRelease, item.GrabbedDownload))
	}
	return events
}

// EventsFromFailedDownloadRun collects failure (and replacement grab) events
// from a failed-download recovery run.
func EventsFromFailedDownloadRun(source string, run wanted.FailedDownloadRun) []Event {
	var events []Event
	for _, item := range run.Items {
		wantedItem := item.WantedItem
		events = append(events, DownloadFailureEvent(source, &wantedItem, item.Download, item.FailureReason))
		if item.ReplacementDownload != nil {
			events = append(events, GrabEvent(source+":replacement", &wantedItem, item.ReplacementRelease, item.ReplacementDownload))
		}
	}
	return events
}

// EventsFromCompletedImports collects import events from a completed-download
// import pass.
func EventsFromCompletedImports(source string, outcome library.CompletedImportOutcome) []Event {
	var events []Event
	for _, result := range outcome.Results {
		if result.Import == nil || !result.Import.Imported {
			continue
		}
		events = append(events, ImportEvent(source, *result.Import))
	}
	return events
}

func bookLabel(title string, author string) string {
	title = strings.TrimSpace(title)
	author = strings.TrimSpace(author)
	switch {
	case title != "" && author != "":
		return title + " by " + author
	case title != "":
		return title
	default:
		return author
	}
}
