package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

const (
	notificationEventGrab           = "Grab"
	notificationEventReleaseImport  = "ReleaseImport"
	notificationEventDownloadFailed = "DownloadFailure"
	notificationEventUpgrade        = "Upgrade"
	notificationEventTest           = "Test"
)

var notificationHTTPClient = &http.Client{Timeout: 10 * time.Second}

type notificationEvent struct {
	EventType  string
	Source     string
	Message    string
	WantedID   string
	WantedItem *wanted.WantedItem
	Download   *acquisition.DownloadStatus
	Release    *wanted.ReleaseDecision
	Import     *library.ImportOutcome
	File       *library.FileRecord
	Extra      map[string]any
}

type notificationTarget struct {
	ID      int
	Name    string
	URL     string
	Method  string
	Headers map[string]string
	Payload map[string]any
}

func (h *handler) notifyDownloadGrab(ctx context.Context, source string, status acquisition.DownloadStatus, wantedID string) {
	h.dispatchNotifications(ctx, notificationEvent{
		EventType: notificationEventGrab,
		Source:    source,
		WantedID:  wantedID,
		Download:  &status,
	})
}

func (h *handler) notifyReleaseImport(ctx context.Context, source string, outcome library.ImportOutcome) {
	file := outcome.File
	h.dispatchNotifications(ctx, notificationEvent{
		EventType: notificationEventReleaseImport,
		Source:    source,
		Import:    &outcome,
		File:      &file,
		Message:   "Book file imported",
	})
}

func (h *handler) notifyCompletedImports(ctx context.Context, source string, outcome library.CompletedImportOutcome) {
	for _, result := range outcome.Results {
		if result.Import == nil {
			continue
		}
		h.notifyReleaseImport(ctx, source, *result.Import)
	}
}

func (h *handler) notifyReviewImport(ctx context.Context, source string, outcome library.ReviewDecisionOutcome) {
	if outcome.Import == nil {
		return
	}
	h.notifyReleaseImport(ctx, source, *outcome.Import)
}

func (h *handler) notifyMonitorGrabs(ctx context.Context, source string, run wanted.MonitorRun) {
	for _, item := range run.Items {
		if item.GrabbedDownload == nil {
			continue
		}
		wantedItem := item.WantedItem
		h.dispatchNotifications(ctx, notificationEvent{
			EventType:  notificationEventGrab,
			Source:     source,
			WantedID:   wantedItem.ID,
			WantedItem: &wantedItem,
			Download:   item.GrabbedDownload,
		})
	}
}

func (h *handler) notifyFeedGrabs(ctx context.Context, source string, run wanted.FeedSyncRun) {
	for _, match := range run.Matches {
		if match.GrabbedDownload == nil {
			continue
		}
		wantedItem := match.WantedItem
		release := match.Release
		h.dispatchNotifications(ctx, notificationEvent{
			EventType:  notificationEventGrab,
			Source:     source,
			WantedID:   wantedItem.ID,
			WantedItem: &wantedItem,
			Release:    &release,
			Download:   match.GrabbedDownload,
		})
	}
}

func (h *handler) notifyUpgradeGrabs(ctx context.Context, source string, run wanted.UpgradeRun) {
	for _, item := range run.Items {
		if item.GrabbedDownload == nil {
			continue
		}
		wantedItem := item.WantedItem
		h.dispatchNotifications(ctx, notificationEvent{
			EventType:  notificationEventUpgrade,
			Source:     source,
			WantedID:   wantedItem.ID,
			WantedItem: &wantedItem,
			Release:    item.UpgradeRelease,
			Download:   item.GrabbedDownload,
			Extra: map[string]any{
				"currentScore": item.CurrentScore,
				"cutoffScore":  item.CutoffScore,
			},
		})
	}
}

func (h *handler) notifyFailedDownloads(ctx context.Context, source string, run wanted.FailedDownloadRun) {
	for _, item := range run.Items {
		wantedItem := item.WantedItem
		event := notificationEvent{
			EventType:  notificationEventDownloadFailed,
			Source:     source,
			Message:    item.FailureReason,
			WantedID:   wantedItem.ID,
			WantedItem: &wantedItem,
			Download:   &item.Download,
			Release:    item.ReplacementRelease,
		}
		h.dispatchNotifications(ctx, event)
		if item.ReplacementDownload != nil {
			replacementEvent := notificationEvent{
				EventType:  notificationEventGrab,
				Source:     source + ":replacement",
				WantedID:   wantedItem.ID,
				WantedItem: &wantedItem,
				Release:    item.ReplacementRelease,
				Download:   item.ReplacementDownload,
			}
			h.dispatchNotifications(ctx, replacementEvent)
		}
	}
}

func (h *handler) dispatchNotifications(ctx context.Context, event notificationEvent) {
	if h == nil || h.deps.Compat == nil {
		return
	}
	targets, err := h.notificationTargets(ctx, event.EventType, false)
	if err != nil {
		h.logNotificationError("load notification targets", event, err)
		return
	}
	if len(targets) == 0 {
		return
	}
	payload := notificationPayload(event)
	for _, target := range targets {
		if err := sendNotificationWebhook(ctx, target, payload, event.EventType); err != nil {
			h.logNotificationError(target.Name, event, err)
		}
	}
}

func (h *handler) testNotification(ctx context.Context, record map[string]any) map[string]any {
	target, err := notificationTargetFromPayload(record)
	result := compatResourceTestResult("notification", record)
	if err == nil {
		err = sendNotificationWebhook(ctx, target, notificationPayload(notificationEvent{
			EventType: notificationEventTest,
			Source:    "notification-test",
			Message:   "Librarry notification test",
		}), notificationEventTest)
	}
	if err != nil {
		result["isValid"] = false
		result["valid"] = false
		result["testPassed"] = false
		result["failures"] = []map[string]any{{
			"propertyName": "url",
			"errorMessage": err.Error(),
		}}
		return result
	}
	result["message"] = "webhook delivered"
	return result
}

func (h *handler) notificationTargets(ctx context.Context, eventType string, includeDisabled bool) ([]notificationTarget, error) {
	if h == nil || h.deps.Compat == nil {
		return nil, nil
	}
	resources, err := h.deps.Compat.ListResources(ctx, "notification")
	if err != nil {
		return nil, err
	}
	targets := make([]notificationTarget, 0, len(resources))
	for _, resource := range resources {
		record := compatStoredResourceRecord(resource, compatNotificationRecord)
		raw := cloneCompatRecord(resource.Payload)
		merged := mergeCompatPayload(record, raw)
		if !includeDisabled && !payloadBoolDefault(merged, "enable", false) {
			continue
		}
		if !notificationSupportsEvent(merged, eventType) {
			continue
		}
		target, err := notificationTargetFromResource(resource, merged)
		if err != nil {
			if includeDisabled {
				targets = append(targets, notificationTarget{
					ID:      resource.CompatID,
					Name:    firstNonEmptyString(resource.Name, payloadString(merged, "name"), "Webhook"),
					Payload: merged,
				})
			}
			continue
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func notificationTargetFromResource(resource compatdata.Resource, payload map[string]any) (notificationTarget, error) {
	target, err := notificationTargetFromPayload(payload)
	target.ID = resource.CompatID
	if target.Name == "" {
		target.Name = firstNonEmptyString(resource.Name, payloadString(payload, "name"), "Webhook")
	}
	return target, err
}

func notificationTargetFromPayload(payload map[string]any) (notificationTarget, error) {
	implementation := strings.ToLower(firstNonEmptyString(payloadString(payload, "implementation"), payloadString(payload, "implementationName"), "Webhook"))
	if !strings.Contains(implementation, "webhook") {
		return notificationTarget{}, fmt.Errorf("notification implementation %q is not supported for native delivery", implementation)
	}
	url := notificationFieldValue(payload, "url", "webhookUrl", "webhookURL")
	if strings.TrimSpace(url) == "" {
		return notificationTarget{}, errors.New("webhook URL is required")
	}
	method := strings.ToUpper(firstNonEmptyString(notificationFieldValue(payload, "method"), "POST"))
	if method == "" {
		method = http.MethodPost
	}
	headers := map[string]string{}
	if token := notificationFieldValue(payload, "authorization", "Authorization"); token != "" {
		headers["Authorization"] = token
	}
	return notificationTarget{
		Name:    firstNonEmptyString(payloadString(payload, "name"), payloadString(payload, "implementation"), "Webhook"),
		URL:     url,
		Method:  method,
		Headers: headers,
		Payload: payload,
	}, nil
}

func sendNotificationWebhook(ctx context.Context, target notificationTarget, payload map[string]any, eventType string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	method := target.Method
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, target.URL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Librarry")
	req.Header.Set("X-Librarry-Event", eventType)
	req.Header.Set("X-Readarr-EventType", eventType)
	for key, value := range target.Headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	if username := notificationFieldValue(target.Payload, "username"); username != "" {
		req.SetBasicAuth(username, notificationFieldValue(target.Payload, "password"))
	}
	resp, err := notificationHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %s", resp.Status)
	}
	return nil
}

func notificationPayload(event notificationEvent) map[string]any {
	payload := map[string]any{
		"eventType":    event.EventType,
		"application":  "Librarry",
		"instanceName": "Librarry",
		"source":       event.Source,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(event.Message) != "" {
		payload["message"] = event.Message
	}
	if strings.TrimSpace(event.WantedID) != "" {
		payload["wantedId"] = event.WantedID
	}
	if event.WantedItem != nil {
		payload["wantedItem"] = event.WantedItem
		payload["book"] = notificationBookRecord(*event.WantedItem)
		payload["author"] = notificationAuthorRecord(*event.WantedItem)
		if strings.TrimSpace(event.WantedID) == "" {
			payload["wantedId"] = event.WantedItem.ID
		}
	}
	if event.Download != nil {
		payload["download"] = event.Download
		payload["downloadId"] = event.Download.ID
		payload["downloadClient"] = firstNonEmptyString(event.Download.Client, "qBittorrent")
		payload["releaseTitle"] = event.Download.Name
		payload["release"] = map[string]any{
			"title":          event.Download.Name,
			"downloadClient": firstNonEmptyString(event.Download.Client, "qBittorrent"),
			"size":           event.Download.SizeBytes,
			"category":       event.Download.Category,
			"state":          event.Download.State,
		}
	}
	if event.Release != nil {
		payload["releaseDecision"] = event.Release
		payload["releaseTitle"] = firstNonEmptyString(event.Release.Title, stringValue(payload["releaseTitle"]))
	}
	if event.File != nil {
		payload["file"] = event.File
		payload["bookFile"] = notificationBookFileRecord(*event.File)
		payload["bookFiles"] = []map[string]any{notificationBookFileRecord(*event.File)}
		if _, ok := payload["book"]; !ok {
			payload["book"] = notificationBookRecordFromFile(*event.File)
		}
		if _, ok := payload["author"]; !ok {
			payload["author"] = notificationAuthorRecordFromFile(*event.File)
		}
	}
	if event.Import != nil {
		payload["import"] = event.Import
		payload["destinationPath"] = event.Import.DestinationPath
		payload["isUpgrade"] = false
	}
	if event.EventType == notificationEventUpgrade {
		payload["isUpgrade"] = true
	}
	for key, value := range event.Extra {
		payload[key] = value
	}
	payload["librarry"] = map[string]any{
		"source":    event.Source,
		"eventType": event.EventType,
	}
	return payload
}

func notificationBookRecord(item wanted.WantedItem) map[string]any {
	return map[string]any{
		"id":               stableInt(item.ID),
		"librarryId":       item.ID,
		"title":            item.Title,
		"authorTitle":      item.AuthorName,
		"foreignBookId":    firstNonEmptyString(item.WorkID, item.SourceKey, item.ID),
		"qualityProfile":   item.QualityProfile,
		"monitored":        item.Monitored,
		"librarryFormat":   item.Format,
		"librarryStatus":   item.Status,
		"currentReleaseId": item.CurrentReleaseID,
	}
}

func notificationAuthorRecord(item wanted.WantedItem) map[string]any {
	return map[string]any{
		"id":              stableInt(item.AuthorName),
		"authorName":      item.AuthorName,
		"foreignAuthorId": firstNonEmptyString(item.SourceProvider, item.AuthorName),
		"titleSlug":       slug(item.AuthorName),
	}
}

func notificationBookRecordFromFile(file library.FileRecord) map[string]any {
	return map[string]any{
		"id":             stableInt(firstNonEmptyString(file.EditionID, file.ID, file.Path)),
		"librarryFileId": file.ID,
		"title":          file.Title,
		"authorTitle":    file.AuthorName,
		"foreignBookId":  firstNonEmptyString(file.EditionID, file.ID),
		"librarryFormat": file.MediaFormat,
	}
}

func notificationAuthorRecordFromFile(file library.FileRecord) map[string]any {
	return map[string]any{
		"id":         stableInt(file.AuthorName),
		"authorName": file.AuthorName,
		"titleSlug":  slug(file.AuthorName),
	}
}

func notificationBookFileRecord(file library.FileRecord) map[string]any {
	return map[string]any{
		"id":             stableInt(firstNonEmptyString(file.ID, file.Path)),
		"librarryId":     file.ID,
		"path":           file.Path,
		"size":           file.SizeBytes,
		"quality":        map[string]any{"quality": map[string]any{"name": file.MediaFormat}},
		"dateAdded":      file.CreatedAt,
		"editionId":      file.EditionID,
		"mediaFormat":    file.MediaFormat,
		"releaseGroup":   "",
		"sceneName":      "",
		"librarrySource": file.SourcePath,
	}
}

func notificationSupportsEvent(payload map[string]any, eventType string) bool {
	switch eventType {
	case notificationEventGrab:
		return payloadBoolDefault(payload, "onGrab", true)
	case notificationEventReleaseImport:
		return payloadBoolDefault(payload, "onReleaseImport", payloadBoolDefault(payload, "onDownload", true))
	case notificationEventUpgrade:
		return payloadBoolDefault(payload, "onUpgrade", true)
	case notificationEventDownloadFailed:
		return payloadBoolDefault(payload, "onDownloadFailure", true)
	case notificationEventTest:
		return true
	default:
		return true
	}
}

func notificationFieldValue(payload map[string]any, names ...string) string {
	for _, name := range names {
		if value := payloadString(payload, name); value != "" {
			return value
		}
	}
	nameSet := map[string]bool{}
	for _, name := range names {
		nameSet[strings.ToLower(strings.TrimSpace(name))] = true
	}
	for _, raw := range compatPayloadArray(payload, "fields") {
		field, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := strings.ToLower(firstNonEmptyString(payloadString(field, "name"), payloadString(field, "fieldName")))
		if !nameSet[name] {
			continue
		}
		if value := payloadString(field, "value"); value != "" {
			return value
		}
	}
	return ""
}

func (h *handler) logNotificationError(target string, event notificationEvent, err error) {
	if h == nil || h.deps.Logger == nil || err == nil {
		return
	}
	h.deps.Logger.Warn("notification webhook failed", "target", target, "event", event.EventType, "error", err)
}
