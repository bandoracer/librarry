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
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/notify"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

const (
	notificationEventGrab           = "Grab"
	notificationEventReleaseImport  = "ReleaseImport"
	notificationEventDownloadFailed = "DownloadFailure"
	notificationEventUpgrade        = "Upgrade"
	notificationEventTest           = "Test"
)

var notificationHTTPClient = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

type notificationEvent struct {
	ID         string
	OccurredAt time.Time
	Files      []library.FileRecord
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

func buildNotificationWebhook(ctx context.Context, target notificationTarget, payload map[string]any, eventType string) (*http.Request, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("webhook request settings are invalid")
	}
	method := target.Method
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, target.URL, bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("webhook request settings are invalid")
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
	req.GetBody = nil
	return req, nil
}

func sendNotificationWebhook(ctx context.Context, target notificationTarget, payload map[string]any, eventType string) error {
	req, err := buildNotificationWebhook(ctx, target, payload, eventType)
	if err != nil {
		return err
	}
	resp, err := notificationHTTPClient.Do(req)
	if err != nil {
		return errors.New("webhook acceptance could not be verified")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func notificationPayload(event notificationEvent) map[string]any {
	payload := map[string]any{
		"eventType":    event.EventType,
		"application":  "Librarry",
		"instanceName": "Librarry",
		"source":       event.Source,
		"timestamp":    notificationTimestamp(event),
		"eventId":      event.ID,
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
	if len(event.Files) > 0 {
		files := make([]map[string]any, 0, len(event.Files))
		for _, file := range event.Files {
			files = append(files, notificationBookFileRecord(file))
		}
		payload["bookFiles"] = files
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

func notificationTimestamp(event notificationEvent) string {
	if event.OccurredAt.IsZero() {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return event.OccurredAt.UTC().Format(time.RFC3339)
}

func buildCompatOutboxRequest(ctx context.Context, settings json.RawMessage, event notify.Event, snapshot json.RawMessage) (*http.Request, error) {
	var payload map[string]any
	if err := json.Unmarshal(settings, &payload); err != nil {
		return nil, err
	}
	target, err := notificationTargetFromPayload(payload)
	if err != nil {
		return nil, err
	}
	var saved struct {
		Extra          map[string]any              `json:"extra"`
		WantedItem     *wanted.WantedItem          `json:"wantedItem"`
		Download       *acquisition.DownloadStatus `json:"download"`
		Release        *wanted.ReleaseDecision     `json:"release"`
		Files          []library.FileRecord        `json:"files"`
		OperationID    string                      `json:"operationId"`
		ConflictAction string                      `json:"conflictAction"`
	}
	if err = json.Unmarshal(snapshot, &saved); err != nil {
		return nil, err
	}
	types := map[notify.EventType]string{notify.EventGrab: notificationEventGrab, notify.EventImport: notificationEventReleaseImport, notify.EventUpgrade: notificationEventUpgrade, notify.EventDownloadFailure: notificationEventDownloadFailed, notify.EventHealthIssue: "HealthIssue"}
	kind, ok := types[event.Type]
	if !ok {
		return nil, errors.New("unsupported compatibility notification event")
	}
	legacy := notificationEvent{ID: event.ID, OccurredAt: event.OccurredAt, EventType: kind, Source: firstNonEmptyString(event.Fields["source"], "notification-outbox"), Message: event.Message, WantedID: event.Fields["wantedId"], WantedItem: saved.WantedItem, Download: saved.Download, Release: saved.Release, Files: saved.Files, Extra: saved.Extra}
	if len(saved.Files) > 0 {
		legacy.File = &saved.Files[0]
		legacy.Import = &library.ImportOutcome{OperationID: saved.OperationID, File: saved.Files[0], Files: saved.Files, DestinationPath: saved.Files[0].Path, Imported: true, Replaced: saved.ConflictAction == "replace", ConflictAction: saved.ConflictAction}
	}

	return buildNotificationWebhook(ctx, target, notificationPayload(legacy), kind)
}
