package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

// Native calendar (M6.1): wanted items with a confident release date inside
// the requested window, plus an iCal feed for external calendar apps.

const (
	calendarDefaultPastDays   = 7
	calendarDefaultFutureDays = 60
)

type calendarItem struct {
	WantedID    string `json:"wantedId"`
	Title       string `json:"title"`
	AuthorName  string `json:"authorName"`
	ReleaseDate string `json:"releaseDate"`
	Status      string `json:"status"`
	Monitored   bool   `json:"monitored"`
	CoverURL    string `json:"coverUrl,omitempty"`
}

// calendarWindow resolves start/end query values (RFC3339 or yyyy-mm-dd).
// Default: start of the current month minus 7 days through now plus 60 days.
func calendarWindow(startRaw string, endRaw string, now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	start := parseCalendarTime(startRaw)
	end := parseCalendarTime(endRaw)
	if start.IsZero() {
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		start = monthStart.AddDate(0, 0, -calendarDefaultPastDays)
	}
	if end.IsZero() {
		end = now.AddDate(0, 0, calendarDefaultFutureDays)
	}
	if end.Before(start) {
		start, end = end, start
	}
	return start, end
}

func parseCalendarTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func (h *handler) librarryCalendar(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	query := r.URL.Query()
	start, end := calendarWindow(query.Get("start"), query.Get("end"), time.Now())
	includeUnmonitored, _ := strconv.ParseBool(query.Get("unmonitored"))
	items, err := h.deps.Wanted.ListCalendar(r.Context(), start, end, includeUnmonitored)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	records := make([]calendarItem, 0, len(items))
	for _, item := range items {
		records = append(records, calendarItem{
			WantedID:    item.ID,
			Title:       item.Title,
			AuthorName:  item.AuthorName,
			ReleaseDate: item.ReleaseDate,
			Status:      item.Status,
			Monitored:   item.Monitored,
			CoverURL:    item.CoverURL,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": records})
}

// calendarFeed renders GET /feed/v1/calendar.ics. The withAuth middleware has
// already enforced the apikey query parameter when auth is configured.
func (h *handler) calendarFeed(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		http.Error(w, "wanted service is unavailable", http.StatusServiceUnavailable)
		return
	}
	query := r.URL.Query()
	pastDays := positiveIntQuery(query.Get("pastDays"), calendarDefaultPastDays)
	futureDays := positiveIntQuery(query.Get("futureDays"), calendarDefaultFutureDays)
	now := time.Now().UTC()
	items, err := h.deps.Wanted.ListCalendar(r.Context(), now.AddDate(0, 0, -pastDays), now.AddDate(0, 0, futureDays), true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="librarry-calendar.ics"`)
	_, _ = w.Write([]byte(renderCalendarICS(items, now)))
}

func positiveIntQuery(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

// renderCalendarICS renders all-day VEVENTs (UID = wantedId) for every item
// with a release date.
func renderCalendarICS(items []wanted.WantedItem, now time.Time) string {
	var builder strings.Builder
	writeICSLine := func(line string) {
		builder.WriteString(line)
		builder.WriteString("\r\n")
	}
	writeICSLine("BEGIN:VCALENDAR")
	writeICSLine("VERSION:2.0")
	writeICSLine("PRODID:-//librarry//calendar//EN")
	writeICSLine("CALSCALE:GREGORIAN")
	writeICSLine("METHOD:PUBLISH")
	writeICSLine("X-WR-CALNAME:Librarry Books")
	stamp := now.UTC().Format("20060102T150405Z")
	for _, item := range items {
		date, err := time.Parse("2006-01-02", item.ReleaseDate)
		if err != nil {
			continue
		}
		summary := item.Title
		if strings.TrimSpace(item.AuthorName) != "" {
			summary = item.AuthorName + " - " + item.Title
		}
		writeICSLine("BEGIN:VEVENT")
		writeICSLine("UID:" + escapeICSText(item.ID))
		writeICSLine("DTSTAMP:" + stamp)
		writeICSLine("DTSTART;VALUE=DATE:" + date.Format("20060102"))
		writeICSLine("DTEND;VALUE=DATE:" + date.AddDate(0, 0, 1).Format("20060102"))
		writeICSLine("SUMMARY:" + escapeICSText(summary))
		writeICSLine("STATUS:CONFIRMED")
		writeICSLine("CATEGORIES:" + escapeICSText(item.Format))
		writeICSLine("END:VEVENT")
	}
	writeICSLine("END:VCALENDAR")
	return builder.String()
}

// escapeICSText escapes per RFC 5545 (backslash, semicolon, comma, newline).
func escapeICSText(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\r\n", "\\n",
		"\n", "\\n",
	)
	return replacer.Replace(value)
}
