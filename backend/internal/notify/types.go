// Package notify delivers Librarry events (grabs, imports, upgrades, download
// failures, health issues) to operator-configured notification targets:
// generic webhooks, ntfy, Discord, and Telegram. Workers and API handlers
// dispatch events through Service; delivery errors are logged, never fatal.
package notify

import (
	"strings"
	"time"
)

// EventType identifies which trigger a notification event maps to.
type EventType string

const (
	EventGrab            EventType = "grab"
	EventImport          EventType = "import"
	EventUpgrade         EventType = "upgrade"
	EventDownloadFailure EventType = "downloadFailure"
	EventHealthIssue     EventType = "healthIssue"
	EventTest            EventType = "test"
)

// Event is a provider-agnostic notification.
type Event struct {
	Type    EventType
	Title   string
	Message string
	Fields  map[string]string
}

// Triggers selects which event types a target receives.
type Triggers struct {
	OnGrab            bool `json:"onGrab"`
	OnImport          bool `json:"onImport"`
	OnUpgrade         bool `json:"onUpgrade"`
	OnDownloadFailure bool `json:"onDownloadFailure"`
	OnHealthIssue     bool `json:"onHealthIssue"`
}

// DefaultTriggers mirrors the schema defaults (health issues opt-in).
func DefaultTriggers() Triggers {
	return Triggers{
		OnGrab:            true,
		OnImport:          true,
		OnUpgrade:         true,
		OnDownloadFailure: true,
		OnHealthIssue:     false,
	}
}

// Matches reports whether the trigger set includes the event type. Test
// events always deliver.
func (t Triggers) Matches(eventType EventType) bool {
	switch eventType {
	case EventGrab:
		return t.OnGrab
	case EventImport:
		return t.OnImport
	case EventUpgrade:
		return t.OnUpgrade
	case EventDownloadFailure:
		return t.OnDownloadFailure
	case EventHealthIssue:
		return t.OnHealthIssue
	case EventTest:
		return true
	default:
		return false
	}
}

// Target is a persisted notification destination.
type Target struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Settings  map[string]string `json:"settings"`
	Triggers  Triggers          `json:"triggers"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

const (
	TargetTypeWebhook  = "webhook"
	TargetTypeNtfy     = "ntfy"
	TargetTypeDiscord  = "discord"
	TargetTypeTelegram = "telegram"
)

// Setting returns a trimmed settings value, accepting any of the given keys
// (first match wins, case-sensitive first then case-insensitive).
func (t Target) Setting(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(t.Settings[key]); value != "" {
			return value
		}
	}
	for _, key := range keys {
		for stored, value := range t.Settings {
			if strings.EqualFold(strings.TrimSpace(stored), key) {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}
