package notify

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testService() *Service {
	return NewService(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type capturedRequest struct {
	method  string
	path    string
	headers http.Header
	body    []byte
}

func captureServer(t *testing.T) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.headers = r.Header.Clone()
		captured.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func TestDeliverWebhookPayload(t *testing.T) {
	server, captured := captureServer(t)
	service := testService()
	target := Target{
		Name: "hook",
		Type: TargetTypeWebhook,
		Settings: map[string]string{
			"url":           server.URL + "/notify",
			"authorization": "Bearer secret",
		},
	}
	event := Event{
		Type:    EventGrab,
		Title:   "Book grabbed: Walden by Henry David Thoreau",
		Message: "Walden [epub] sent to qBittorrent",
		Fields:  map[string]string{"indexer": "Public Domain"},
	}
	if err := service.Deliver(context.Background(), target, event); err != nil {
		t.Fatal(err)
	}
	if captured.method != http.MethodPost || captured.path != "/notify" {
		t.Fatalf("unexpected request: %s %s", captured.method, captured.path)
	}
	if got := captured.headers.Get("Authorization"); got != "Bearer secret" {
		t.Fatalf("expected authorization header, got %q", got)
	}
	if got := captured.headers.Get("X-Librarry-Event"); got != "grab" {
		t.Fatalf("expected grab event header, got %q", got)
	}
	var payload map[string]any
	if err := json.Unmarshal(captured.body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["eventType"] != "Grab" || payload["application"] != "Librarry" {
		t.Fatalf("unexpected payload: %v", payload)
	}
	if payload["title"] != event.Title || payload["message"] != event.Message {
		t.Fatalf("expected title/message passthrough, got %v", payload)
	}
	fields, ok := payload["fields"].(map[string]any)
	if !ok || fields["indexer"] != "Public Domain" {
		t.Fatalf("expected fields in payload, got %v", payload["fields"])
	}
}

func TestDeliverNtfyTopicAndHeaders(t *testing.T) {
	server, captured := captureServer(t)
	service := testService()
	target := Target{
		Name: "phone",
		Type: TargetTypeNtfy,
		Settings: map[string]string{
			"url":   server.URL,
			"topic": "librarry",
			"token": "tk-123",
		},
	}
	event := Event{Type: EventImport, Title: "Book imported", Message: "/data/media/books/walden.epub"}
	if err := service.Deliver(context.Background(), target, event); err != nil {
		t.Fatal(err)
	}
	if captured.path != "/librarry" {
		t.Fatalf("expected topic path, got %q", captured.path)
	}
	if got := captured.headers.Get("X-Title"); got != "Book imported" {
		t.Fatalf("expected title header, got %q", got)
	}
	if got := captured.headers.Get("Authorization"); got != "Bearer tk-123" {
		t.Fatalf("expected bearer token, got %q", got)
	}
	if string(captured.body) != "/data/media/books/walden.epub" {
		t.Fatalf("expected message body, got %q", string(captured.body))
	}
}

func TestDeliverDiscordEmbeds(t *testing.T) {
	server, captured := captureServer(t)
	service := testService()
	target := Target{
		Name:     "ops",
		Type:     TargetTypeDiscord,
		Settings: map[string]string{"webhookUrl": server.URL + "/api/webhooks/1/abc"},
	}
	event := Event{
		Type:    EventDownloadFailure,
		Title:   "Download failed: Walden",
		Message: "stalled for 24h",
		Fields:  map[string]string{"client": "qBittorrent"},
	}
	if err := service.Deliver(context.Background(), target, event); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Username string `json:"username"`
		Embeds   []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Color       int    `json:"color"`
			Fields      []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"fields"`
		} `json:"embeds"`
	}
	if err := json.Unmarshal(captured.body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Username != "Librarry" || len(payload.Embeds) != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	embed := payload.Embeds[0]
	if embed.Title != event.Title || embed.Description != event.Message {
		t.Fatalf("unexpected embed: %+v", embed)
	}
	if embed.Color == 0 {
		t.Fatal("expected a severity color")
	}
	if len(embed.Fields) != 1 || embed.Fields[0].Name != "client" || embed.Fields[0].Value != "qBittorrent" {
		t.Fatalf("unexpected embed fields: %+v", embed.Fields)
	}
}

func TestDeliverTelegramSendMessage(t *testing.T) {
	server, captured := captureServer(t)
	service := testService()
	service.telegramAPIBase = server.URL
	target := Target{
		Name: "tg",
		Type: TargetTypeTelegram,
		Settings: map[string]string{
			"botToken": "12345:secret-token",
			"chatId":   "-100987",
		},
	}
	event := Event{Type: EventGrab, Title: "Book grabbed", Message: "Walden"}
	if err := service.Deliver(context.Background(), target, event); err != nil {
		t.Fatal(err)
	}
	if captured.path != "/bot12345:secret-token/sendMessage" {
		t.Fatalf("unexpected telegram path: %q", captured.path)
	}
	var payload map[string]any
	if err := json.Unmarshal(captured.body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["chat_id"] != "-100987" {
		t.Fatalf("unexpected chat id: %v", payload["chat_id"])
	}
	text, _ := payload["text"].(string)
	if !strings.Contains(text, "Book grabbed") || !strings.Contains(text, "Walden") {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestDeliverSurfacesHTTPErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)
	service := testService()
	target := Target{Name: "hook", Type: TargetTypeWebhook, Settings: map[string]string{"url": server.URL}}
	err := service.Deliver(context.Background(), target, TestEvent())
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected 403 error, got %v", err)
	}
}

func TestNormalizeTargetValidation(t *testing.T) {
	cases := []struct {
		name    string
		target  Target
		wantErr string
	}{
		{"missing name", Target{Type: TargetTypeWebhook, Settings: map[string]string{"url": "http://x"}}, "name is required"},
		{"unknown type", Target{Name: "x", Type: "pigeon"}, "not supported"},
		{"webhook without url", Target{Name: "x", Type: TargetTypeWebhook}, "settings.url"},
		{"ntfy without url or topic", Target{Name: "x", Type: TargetTypeNtfy}, "settings.url or settings.topic"},
		{"discord without webhookUrl", Target{Name: "x", Type: TargetTypeDiscord}, "settings.webhookUrl"},
		{"telegram without token", Target{Name: "x", Type: TargetTypeTelegram, Settings: map[string]string{"chatId": "1"}}, "botToken"},
		{"telegram without chat", Target{Name: "x", Type: TargetTypeTelegram, Settings: map[string]string{"botToken": "t"}}, "chatId"},
	}
	for _, testCase := range cases {
		_, err := NormalizeTarget(testCase.target)
		if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
			t.Fatalf("%s: expected error containing %q, got %v", testCase.name, testCase.wantErr, err)
		}
	}
	valid, err := NormalizeTarget(Target{
		Name:     " tg ",
		Type:     " Telegram ",
		Settings: map[string]string{"botToken": " token-1234 ", "chatId": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if valid.Name != "tg" || valid.Type != TargetTypeTelegram || valid.Settings["botToken"] != "token-1234" {
		t.Fatalf("unexpected normalized target: %+v", valid)
	}
}

func TestTriggersMatch(t *testing.T) {
	triggers := Triggers{OnGrab: true, OnHealthIssue: false}
	if !triggers.Matches(EventGrab) {
		t.Fatal("expected grab to match")
	}
	if triggers.Matches(EventHealthIssue) {
		t.Fatal("expected health issue to be filtered")
	}
	if !triggers.Matches(EventTest) {
		t.Fatal("test events always deliver")
	}
	if triggers.Matches(EventImport) {
		t.Fatal("expected import to be filtered")
	}
}

func TestRedactTargetMasksTelegramToken(t *testing.T) {
	target := Target{
		Name:     "tg",
		Type:     TargetTypeTelegram,
		Settings: map[string]string{"botToken": "12345:secret-9876", "chatId": "1"},
	}
	redacted := RedactTarget(target)
	if redacted.Settings["botToken"] != "****9876" {
		t.Fatalf("expected masked token, got %q", redacted.Settings["botToken"])
	}
	if redacted.Settings["chatId"] != "1" {
		t.Fatal("chatId must not be redacted")
	}
	if target.Settings["botToken"] != "12345:secret-9876" {
		t.Fatal("redaction must not mutate the source target")
	}
	webhook := Target{Name: "hook", Type: TargetTypeWebhook, Settings: map[string]string{"url": "http://x/secret"}}
	if RedactTarget(webhook).Settings["url"] != "http://x/secret" {
		t.Fatal("webhook URLs are operator-entered and returned as stored")
	}
}

func TestMergeSecretsKeepsStoredToken(t *testing.T) {
	stored := Target{
		Type:     TargetTypeTelegram,
		Settings: map[string]string{"botToken": "12345:secret-9876", "chatId": "1"},
	}
	blank := Target{Type: TargetTypeTelegram, Settings: map[string]string{"chatId": "2"}}
	merged := MergeSecrets(blank, stored)
	if merged.Settings["botToken"] != "12345:secret-9876" {
		t.Fatalf("expected blank token to keep stored value, got %q", merged.Settings["botToken"])
	}
	echoed := Target{Type: TargetTypeTelegram, Settings: map[string]string{"botToken": "****9876", "chatId": "2"}}
	merged = MergeSecrets(echoed, stored)
	if merged.Settings["botToken"] != "12345:secret-9876" {
		t.Fatalf("expected redacted echo to keep stored value, got %q", merged.Settings["botToken"])
	}
	replaced := Target{Type: TargetTypeTelegram, Settings: map[string]string{"botToken": "new-token", "chatId": "2"}}
	merged = MergeSecrets(replaced, stored)
	if merged.Settings["botToken"] != "new-token" {
		t.Fatalf("expected replacement token to win, got %q", merged.Settings["botToken"])
	}
}
