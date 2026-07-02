package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// buildWebhookRequest posts an arr-ish JSON payload to settings.url.
func buildWebhookRequest(ctx context.Context, target Target, event Event) (*http.Request, error) {
	endpoint := target.Setting("url")
	if endpoint == "" {
		return nil, fmt.Errorf("webhook target %q has no url", target.Name)
	}
	body, err := json.Marshal(webhookPayload(event))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Librarry")
	req.Header.Set("X-Librarry-Event", string(event.Type))
	if token := target.Setting("authorization", "Authorization"); token != "" {
		req.Header.Set("Authorization", token)
	}
	return req, nil
}

func webhookPayload(event Event) map[string]any {
	payload := map[string]any{
		"eventType":    webhookEventType(event.Type),
		"application":  "Librarry",
		"instanceName": "Librarry",
		"title":        event.Title,
		"message":      event.Message,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	if len(event.Fields) > 0 {
		fields := map[string]any{}
		for key, value := range event.Fields {
			fields[key] = value
		}
		payload["fields"] = fields
	}
	return payload
}

// webhookEventType maps native event types onto the Readarr-style names
// compat webhook consumers already understand.
func webhookEventType(eventType EventType) string {
	switch eventType {
	case EventGrab:
		return "Grab"
	case EventImport:
		return "ReleaseImport"
	case EventUpgrade:
		return "Upgrade"
	case EventDownloadFailure:
		return "DownloadFailure"
	case EventHealthIssue:
		return "HealthIssue"
	case EventTest:
		return "Test"
	default:
		return string(eventType)
	}
}

// buildNtfyRequest posts the message body to <url>/<topic> with ntfy title
// and tag headers. settings.url may already include the topic path.
func buildNtfyRequest(ctx context.Context, target Target, event Event) (*http.Request, error) {
	endpoint := target.Setting("url")
	topic := target.Setting("topic")
	if endpoint == "" {
		endpoint = "https://ntfy.sh"
	}
	if topic != "" {
		endpoint = strings.TrimRight(endpoint, "/") + "/" + url.PathEscape(topic)
	}
	message := event.Message
	if extras := formatFieldLines(event.Fields); extras != "" {
		if message != "" {
			message += "\n"
		}
		message += extras
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(message))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("User-Agent", "Librarry")
	req.Header.Set("X-Title", event.Title)
	req.Header.Set("X-Tags", ntfyTags(event.Type))
	if token := target.Setting("token", "accessToken"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if priority := target.Setting("priority"); priority != "" {
		req.Header.Set("X-Priority", priority)
	}
	return req, nil
}

func ntfyTags(eventType EventType) string {
	switch eventType {
	case EventGrab:
		return "books,arrow_down"
	case EventImport, EventUpgrade:
		return "books,white_check_mark"
	case EventDownloadFailure, EventHealthIssue:
		return "books,warning"
	default:
		return "books"
	}
}

// buildDiscordRequest posts an embeds payload to settings.webhookUrl.
func buildDiscordRequest(ctx context.Context, target Target, event Event) (*http.Request, error) {
	endpoint := target.Setting("webhookUrl", "url")
	if endpoint == "" {
		return nil, fmt.Errorf("discord target %q has no webhookUrl", target.Name)
	}
	body, err := json.Marshal(discordPayload(event))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Librarry")
	return req, nil
}

func discordPayload(event Event) map[string]any {
	embed := map[string]any{
		"title":       event.Title,
		"description": event.Message,
		"color":       discordColor(event.Type),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"footer":      map[string]any{"text": "Librarry"},
	}
	if len(event.Fields) > 0 {
		fields := make([]map[string]any, 0, len(event.Fields))
		for _, key := range sortedFieldKeys(event.Fields) {
			fields = append(fields, map[string]any{
				"name":   key,
				"value":  event.Fields[key],
				"inline": true,
			})
		}
		embed["fields"] = fields
	}
	return map[string]any{
		"username": "Librarry",
		"embeds":   []map[string]any{embed},
	}
}

func discordColor(eventType EventType) int {
	switch eventType {
	case EventGrab:
		return 0x3498db // blue
	case EventImport, EventUpgrade:
		return 0x2ecc71 // green
	case EventDownloadFailure:
		return 0xe74c3c // red
	case EventHealthIssue:
		return 0xf39c12 // orange
	default:
		return 0x95a5a6 // grey
	}
}

// buildTelegramRequest calls the Bot API sendMessage method with
// settings.botToken and settings.chatId.
func buildTelegramRequest(ctx context.Context, apiBase string, target Target, event Event) (*http.Request, error) {
	botToken := target.Setting("botToken")
	chatID := target.Setting("chatId", "chatID")
	if botToken == "" {
		return nil, fmt.Errorf("telegram target %q has no botToken", target.Name)
	}
	if chatID == "" {
		return nil, fmt.Errorf("telegram target %q has no chatId", target.Name)
	}
	if strings.TrimSpace(apiBase) == "" {
		apiBase = "https://api.telegram.org"
	}
	endpoint := strings.TrimRight(apiBase, "/") + "/bot" + botToken + "/sendMessage"
	text := event.Title
	if event.Message != "" {
		text += "\n" + event.Message
	}
	if extras := formatFieldLines(event.Fields); extras != "" {
		text += "\n" + extras
	}
	body, err := json.Marshal(map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Librarry")
	return req, nil
}

func formatFieldLines(fields map[string]string) string {
	if len(fields) == 0 {
		return ""
	}
	lines := make([]string, 0, len(fields))
	for _, key := range sortedFieldKeys(fields) {
		lines = append(lines, key+": "+fields[key])
	}
	return strings.Join(lines, "\n")
}

func sortedFieldKeys(fields map[string]string) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
