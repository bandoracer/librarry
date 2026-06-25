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

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestHealthEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"service":"librarry"`) {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
}

func TestSettingsValidateEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/validate", strings.NewReader(`{"prowlarrUrl":"http://localhost"}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Prowlarr URL and API key") {
		t.Fatalf("expected validation error, got %s", res.Body.String())
	}
}

func TestDownloadActionEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/downloads/actions", strings.NewReader(`{"action":"stop","ids":["abc123"]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload acquisition.DownloadActionResult
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Action != acquisition.DownloadActionStop || !payload.Applied || payload.IDs[0] != "abc123" {
		t.Fatalf("unexpected action payload: %+v", payload)
	}
}

func TestListWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wanted", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Project Hail Mary") {
		t.Fatalf("expected wanted item in response, got %s", res.Body.String())
	}
}

func TestGrabWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted/wanted-1/grab", strings.NewReader(`{"releaseId":"release-1","paused":true}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "download-1") {
		t.Fatalf("expected download status in response, got %s", res.Body.String())
	}
}

func TestMonitorWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted/monitor", strings.NewReader(`{"force":true,"limit":5}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"wantedChecked":1`) {
		t.Fatalf("expected monitor run in response, got %s", res.Body.String())
	}
}

func TestAuthorSubscriptionsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/authors?status=monitored", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Andy Weir") {
		t.Fatalf("expected author subscription in response, got %s", res.Body.String())
	}
}

func TestSubscribeAuthorEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/authors", strings.NewReader(`{"authorName":"Andy Weir","provider":"Open Library","providerKey":"openlibrary:OL123A","format":"ebook","qualityProfile":"standard","monitorNewItems":true}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"status":"monitored"`) {
		t.Fatalf("expected saved author subscription in response, got %s", res.Body.String())
	}
}

func TestMonitorAuthorsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/authors/monitor", strings.NewReader(`{"force":true,"limit":5}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"authorsChecked":1`) {
		t.Fatalf("expected author monitor run in response, got %s", res.Body.String())
	}
}

func TestFeedSyncWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted/feed-sync", strings.NewReader(`{"limit":10}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"releasesSeen":2`) {
		t.Fatalf("expected feed sync run in response, got %s", res.Body.String())
	}
}

func TestRecoverFailedDownloadsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/downloads/recover-failed", strings.NewReader(`{"autoGrab":true,"downloadIds":["abc123"]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"failedCount":1`) {
		t.Fatalf("expected failed download run in response, got %s", res.Body.String())
	}
}

func TestUpgradeWantedEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wanted/upgrades", strings.NewReader(`{"autoGrab":true,"force":true}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"upgradeCount":1`) {
		t.Fatalf("expected upgrade run in response, got %s", res.Body.String())
	}
}

func TestHistoryEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/history?limit=5", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "wanted_searched") {
		t.Fatalf("expected history event in response, got %s", res.Body.String())
	}
}

func TestQualityProfilesEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/quality-profiles", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"minSeeders":2`) {
		t.Fatalf("expected profile payload in response, got %s", res.Body.String())
	}
}

func TestSaveQualityProfileEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quality-profiles", strings.NewReader(`{"name":"retail","mediaFormat":"ebook","minScore":70,"cutoffScore":92,"minSeeders":3,"rejectedTerms":["summary"]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"name":"retail"`) {
		t.Fatalf("expected saved profile in response, got %s", res.Body.String())
	}
}

func TestLibraryFilesEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/library/files?format=ebook", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Project Hail Mary") {
		t.Fatalf("expected file in response, got %s", res.Body.String())
	}
}

func TestScanLibraryEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/scan", strings.NewReader(`{"format":"ebook"}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"upserted":1`) {
		t.Fatalf("expected scan outcome in response, got %s", res.Body.String())
	}
}

func TestImportLibraryFileEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/import", strings.NewReader(`{"sourcePath":"/downloads/book.epub","wantedId":"wanted-1"}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "/library/ebooks") {
		t.Fatalf("expected import outcome in response, got %s", res.Body.String())
	}
}

func TestImportCompletedDownloadsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Acquire:  fakeAcquire{},
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/import-completed", strings.NewReader(`{"downloadIds":["abc123"]}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"imported":1`) {
		t.Fatalf("expected completed import outcome in response, got %s", res.Body.String())
	}
}

func TestImportReviewsEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/library/import-reviews?status=pending", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"reason":"download is not linked to a wanted item"`) {
		t.Fatalf("expected review list in response, got %s", res.Body.String())
	}
}

func TestResolveImportReviewEndpoint(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  fakeLibrary{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/import-reviews/review-1/resolve", strings.NewReader(`{"action":"import","wantedId":"wanted-1"}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"status":"imported"`) {
		t.Fatalf("expected review resolution in response, got %s", res.Body.String())
	}
}

type fakeAcquire struct{}

func (fakeAcquire) Health(context.Context) []acquisition.IntegrationHealth {
	return nil
}

func (fakeAcquire) Bootstrap(context.Context) (acquisition.BootstrapResult, error) {
	return acquisition.BootstrapResult{}, nil
}

func (fakeAcquire) Search(context.Context, acquisition.ReleaseSearchQuery) ([]acquisition.Release, error) {
	return nil, nil
}

func (fakeAcquire) Grab(context.Context, acquisition.DownloadRequest) (acquisition.DownloadStatus, error) {
	return acquisition.DownloadStatus{}, nil
}

func (fakeAcquire) Downloads(context.Context, acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error) {
	now := time.Now().UTC()
	return []acquisition.DownloadStatus{{
		ID:          "abc123",
		Name:        "Project Hail Mary.epub",
		State:       "uploading",
		Progress:    1,
		SavePath:    "/downloads",
		Category:    "books-ebook",
		Tags:        []string{"librarry", "wanted:wanted-1"},
		CompletedAt: &now,
	}}, nil
}

func (fakeAcquire) DownloadAction(_ context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error) {
	return acquisition.DownloadActionResult{
		Action:  acquisition.DownloadActionStop,
		IDs:     request.IDs,
		Applied: true,
	}, nil
}

type fakeWanted struct{}

func (fakeWanted) Create(context.Context, wanted.CreateRequest) (wanted.WantedItem, error) {
	return wanted.WantedItem{ID: "wanted-1", Title: "Project Hail Mary", Format: "ebook", Status: "wanted"}, nil
}

func (fakeWanted) List(context.Context, string) ([]wanted.WantedItem, error) {
	return []wanted.WantedItem{{ID: "wanted-1", Title: "Project Hail Mary", Format: "ebook", Status: "wanted"}}, nil
}

func (fakeWanted) ListQualityProfiles(context.Context) ([]wanted.QualityProfile, error) {
	return []wanted.QualityProfile{{
		ID:             "profile-1",
		Name:           "standard",
		MediaFormat:    "ebook",
		MinScore:       60,
		CutoffScore:    85,
		MinSeeders:     2,
		MaxSizeBytes:   786432000,
		PreferredTerms: []string{"epub"},
		RejectedTerms:  []string{"summary", "review"},
		PreferredScore: 8,
		UpgradeAllowed: true,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}}, nil
}

func (fakeWanted) SaveQualityProfile(_ context.Context, profile wanted.QualityProfile) (wanted.QualityProfile, error) {
	profile.ID = "profile-2"
	profile.CreatedAt = time.Now().UTC()
	profile.UpdatedAt = profile.CreatedAt
	return profile, nil
}

func (fakeWanted) SubscribeAuthor(_ context.Context, request wanted.AuthorSubscribeRequest) (wanted.AuthorSubscription, error) {
	return wanted.AuthorSubscription{
		ID:              "author-sub-1",
		Provider:        request.Provider,
		ProviderKey:     request.ProviderKey,
		AuthorName:      request.AuthorName,
		Format:          "ebook",
		QualityProfile:  "standard",
		Status:          "monitored",
		MonitorNewItems: true,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}, nil
}

func (fakeWanted) ListAuthorSubscriptions(context.Context, string) ([]wanted.AuthorSubscription, error) {
	return []wanted.AuthorSubscription{{
		ID:              "author-sub-1",
		Provider:        "Open Library",
		ProviderKey:     "openlibrary:OL123A",
		AuthorName:      "Andy Weir",
		Format:          "ebook",
		QualityProfile:  "standard",
		Status:          "monitored",
		MonitorNewItems: true,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}}, nil
}

func (fakeWanted) MonitorAuthors(context.Context, wanted.AuthorMonitorRequest) (wanted.AuthorMonitorRun, error) {
	return wanted.AuthorMonitorRun{
		ID:             "author-run-1",
		Trigger:        "manual",
		Status:         "completed",
		AuthorsChecked: 1,
		ItemsFound:     2,
		WantedCreated:  2,
		StartedAt:      time.Now().UTC(),
	}, nil
}

func (fakeWanted) SearchReleases(context.Context, string, wanted.SearchReleasesRequest) (wanted.SearchOutcome, error) {
	return wanted.SearchOutcome{}, nil
}

func (fakeWanted) ListReleases(context.Context, string) (wanted.SearchOutcome, error) {
	return wanted.SearchOutcome{}, nil
}

func (fakeWanted) Grab(context.Context, string, wanted.GrabRequest) (acquisition.DownloadStatus, error) {
	return acquisition.DownloadStatus{ID: "download-1", Name: "Book", State: "stoppedDL"}, nil
}

func (fakeWanted) Monitor(context.Context, wanted.MonitorRequest) (wanted.MonitorRun, error) {
	return wanted.MonitorRun{
		ID:            "run-1",
		Trigger:       "manual",
		Status:        "completed",
		WantedChecked: 1,
		StartedAt:     time.Now().UTC(),
	}, nil
}

func (fakeWanted) FeedSync(context.Context, wanted.FeedSyncRequest) (wanted.FeedSyncRun, error) {
	return wanted.FeedSyncRun{
		ID:            "feed-1",
		Trigger:       "manual",
		Status:        "completed",
		ReleasesSeen:  2,
		MatchedCount:  1,
		ApprovedCount: 1,
		StartedAt:     time.Now().UTC(),
	}, nil
}

func (fakeWanted) RecoverFailedDownloads(context.Context, wanted.FailedDownloadRequest) (wanted.FailedDownloadRun, error) {
	return wanted.FailedDownloadRun{
		ID:                "failed-1",
		Trigger:           "manual",
		Status:            "completed",
		DownloadsChecked:  1,
		FailedCount:       1,
		ReplacementsFound: 1,
		StartedAt:         time.Now().UTC(),
	}, nil
}

func (fakeWanted) SearchUpgrades(context.Context, wanted.UpgradeRequest) (wanted.UpgradeRun, error) {
	return wanted.UpgradeRun{
		ID:            "upgrade-1",
		Trigger:       "manual",
		Status:        "completed",
		WantedChecked: 1,
		ReleasesFound: 2,
		UpgradeCount:  1,
		StartedAt:     time.Now().UTC(),
	}, nil
}

func (fakeWanted) History(context.Context, wanted.HistoryQuery) ([]wanted.HistoryEvent, error) {
	return []wanted.HistoryEvent{{
		ID:         "event-1",
		EventType:  "wanted_searched",
		EntityType: "wanted_item",
		EntityID:   "wanted-1",
		Severity:   "info",
		Message:    "Searched wanted releases",
		CreatedAt:  time.Now().UTC(),
	}}, nil
}

type fakeLibrary struct{}

func (fakeLibrary) ListFiles(context.Context, library.FileListQuery) ([]library.FileRecord, error) {
	return []library.FileRecord{{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "imported",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}}, nil
}

func (fakeLibrary) ListImportReviews(context.Context, library.ReviewListQuery) ([]library.ImportReview, error) {
	return []library.ImportReview{{
		ID:          "review-1",
		SourcePath:  "/downloads/Project Hail Mary.epub",
		DownloadID:  "abc123",
		MediaFormat: "ebook",
		Title:       "Project Hail Mary",
		Reason:      "download is not linked to a wanted item",
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}}, nil
}

func (fakeLibrary) Scan(context.Context, library.ScanRequest) (library.ScanOutcome, error) {
	file := library.FileRecord{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "available",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	return library.ScanOutcome{
		Roots:    []string{"/library/ebooks"},
		Scanned:  1,
		Upserted: 1,
		Files:    []library.FileRecord{file},
	}, nil
}

func (fakeLibrary) ResolveImportReview(context.Context, string, library.ReviewDecisionRequest) (library.ReviewDecisionOutcome, error) {
	file := library.FileRecord{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "imported",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	return library.ReviewDecisionOutcome{
		Review: library.ImportReview{
			ID:              "review-1",
			SourcePath:      "/downloads/Project Hail Mary.epub",
			MediaFormat:     "ebook",
			Title:           "Project Hail Mary",
			Reason:          "download is not linked to a wanted item",
			Status:          "imported",
			Decision:        "import",
			DestinationPath: file.Path,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		},
		Import: &library.ImportOutcome{
			File:            file,
			DestinationPath: file.Path,
		},
	}, nil
}

func (fakeLibrary) Import(context.Context, library.ImportRequest) (library.ImportOutcome, error) {
	file := library.FileRecord{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "imported",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	return library.ImportOutcome{
		File:            file,
		DestinationPath: file.Path,
		Moved:           false,
	}, nil
}

func (fakeLibrary) ImportCompletedDownloads(_ context.Context, downloads []acquisition.DownloadStatus, _ library.CompletedImportRequest) (library.CompletedImportOutcome, error) {
	file := library.FileRecord{
		ID:           "file-1",
		MediaFormat:  "ebook",
		Path:         "/library/ebooks/Andy Weir/Project Hail Mary/Project Hail Mary.epub",
		Title:        "Project Hail Mary",
		AuthorName:   "Andy Weir",
		ImportStatus: "imported",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	return library.CompletedImportOutcome{
		Checked:  1,
		Imported: 1,
		Results: []library.DownloadImportResult{{
			Download: downloads[0],
			Status:   "imported",
			Import: &library.ImportOutcome{
				File:            file,
				DestinationPath: file.Path,
			},
		}},
	}, nil
}
