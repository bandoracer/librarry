package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestCalendarWindowDefaultsAndParsing(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	start, end := calendarWindow("", "", now)
	wantStart := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC) // month start - 7d
	wantEnd := now.AddDate(0, 0, 60)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("unexpected default window: %s .. %s", start, end)
	}

	start, end = calendarWindow("2026-01-01", "2026-02-01", now)
	if start.Format("2006-01-02") != "2026-01-01" || end.Format("2006-01-02") != "2026-02-01" {
		t.Fatalf("unexpected yyyy-mm-dd window: %s .. %s", start, end)
	}

	start, end = calendarWindow("2026-03-01T00:00:00Z", "2026-01-01T00:00:00Z", now)
	if !start.Before(end) {
		t.Fatalf("expected inverted bounds to be swapped: %s .. %s", start, end)
	}
}

type calendarWanted struct {
	fakeWanted
	gotStart       time.Time
	gotEnd         time.Time
	gotUnmonitored bool
}

func (c *calendarWanted) ListCalendar(_ context.Context, start time.Time, end time.Time, includeUnmonitored bool) ([]wanted.WantedItem, error) {
	c.gotStart = start
	c.gotEnd = end
	c.gotUnmonitored = includeUnmonitored
	return []wanted.WantedItem{{
		ID:          "wanted-9",
		Title:       "Project Hail Mary",
		AuthorName:  "Andy Weir",
		Status:      "wanted",
		Monitored:   true,
		ReleaseDate: "2026-07-04",
		CoverURL:    "https://covers.example/phm.jpg",
	}}, nil
}

func TestLibrarryCalendarEndpointShape(t *testing.T) {
	wantedClient := &calendarWanted{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   wantedClient,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/librarry/calendar?start=2026-07-01&end=2026-07-31&unmonitored=true", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !wantedClient.gotUnmonitored {
		t.Fatal("expected unmonitored=true to propagate")
	}
	if wantedClient.gotStart.Format("2006-01-02") != "2026-07-01" || wantedClient.gotEnd.Format("2006-01-02") != "2026-07-31" {
		t.Fatalf("unexpected window: %s .. %s", wantedClient.gotStart, wantedClient.gotEnd)
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one item, got %+v", payload.Items)
	}
	item := payload.Items[0]
	if item["wantedId"] != "wanted-9" || item["title"] != "Project Hail Mary" ||
		item["authorName"] != "Andy Weir" || item["releaseDate"] != "2026-07-04" ||
		item["status"] != "wanted" || item["monitored"] != true ||
		item["coverUrl"] != "https://covers.example/phm.jpg" {
		t.Fatalf("unexpected calendar item shape: %+v", item)
	}
}

func TestRenderCalendarICS(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 30, 0, 0, time.UTC)
	items := []wanted.WantedItem{
		{ID: "wanted-1", Title: "Hail; Mary, v2", AuthorName: "Andy Weir", Format: "ebook", ReleaseDate: "2026-07-04"},
		{ID: "wanted-2", Title: "No Date Book", AuthorName: "Someone"},
	}
	ics := renderCalendarICS(items, now)

	if !strings.HasPrefix(ics, "BEGIN:VCALENDAR\r\n") || !strings.HasSuffix(ics, "END:VCALENDAR\r\n") {
		t.Fatalf("expected VCALENDAR wrapper, got %q", ics)
	}
	if !strings.Contains(ics, "UID:wanted-1\r\n") {
		t.Fatal("expected UID to be the wanted id")
	}
	if !strings.Contains(ics, "DTSTART;VALUE=DATE:20260704\r\n") || !strings.Contains(ics, "DTEND;VALUE=DATE:20260705\r\n") {
		t.Fatal("expected all-day DTSTART/DTEND")
	}
	if !strings.Contains(ics, `SUMMARY:Andy Weir - Hail\; Mary\, v2`) {
		t.Fatalf("expected escaped summary, got %q", ics)
	}
	if strings.Contains(ics, "wanted-2") {
		t.Fatal("expected items without a release date to be skipped")
	}
	if strings.Count(ics, "BEGIN:VEVENT") != 1 {
		t.Fatalf("expected exactly one VEVENT, got %q", ics)
	}
}

func TestCalendarFeedEndpointServesICS(t *testing.T) {
	wantedClient := &calendarWanted{}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   wantedClient,
	})
	req := httptest.NewRequest(http.MethodGet, "/feed/v1/calendar.ics?pastDays=3&futureDays=14", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.HasPrefix(res.Header().Get("Content-Type"), "text/calendar") {
		t.Fatalf("expected text/calendar, got %q", res.Header().Get("Content-Type"))
	}
	if !strings.Contains(res.Body.String(), "UID:wanted-9") {
		t.Fatalf("expected VEVENT for wanted-9, got %s", res.Body.String())
	}
	if window := wantedClient.gotEnd.Sub(wantedClient.gotStart); window < 16*24*time.Hour || window > 18*24*time.Hour {
		t.Fatalf("expected ~17d window from pastDays/futureDays, got %s", window)
	}
	if !wantedClient.gotUnmonitored {
		t.Fatal("expected the feed to include unmonitored items")
	}
}
