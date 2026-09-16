package notify

import "fmt"

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
