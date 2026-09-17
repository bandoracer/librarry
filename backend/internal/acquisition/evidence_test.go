package acquisition

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestLiveDownloadEvidenceDoesNotResurrectSavedDownloads(t *testing.T) {
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "fixture", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()
	service := NewService(IntegrationConfig{QBittorrentURL: server.URL, DownloadStore: fakeDownloadStore{downloads: []DownloadStatus{{ID: "stale", State: "downloading"}}}})
	for _, tc := range []struct {
		fail   bool
		status string
	}{{false, "fresh"}, {true, "unavailable"}} {
		fail.Store(tc.fail)
		got := service.LiveDownloadEvidence(context.Background(), DownloadListQuery{})
		if got.Status != tc.status || len(got.Downloads) != 0 {
			t.Fatal(got)
		}
	}
	got := NewService(IntegrationConfig{}).LiveDownloadEvidence(context.Background(), DownloadListQuery{})
	if got.Status != "notConfigured" {
		t.Fatal(got)
	}
}

func TestLiveDownloadEvidenceRetainsPartialSuccess(t *testing.T) {
	qbit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"hash":"live","state":"downloading","tags":"librarry, wanted:book"}]`))
	}))
	defer qbit.Close()
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "fixture", 503) }))
	defer failed.Close()
	service := NewService(IntegrationConfig{QBittorrentURL: qbit.URL, TransmissionURL: failed.URL})
	got := service.LiveDownloadEvidence(context.Background(), DownloadListQuery{Tag: "librarry"})
	if got.Status != "partial" || len(got.Downloads) != 1 || got.Downloads[0].ID != "live" {
		t.Fatal(got)
	}
}
