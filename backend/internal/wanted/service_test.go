package wanted

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func TestNoDatabaseReadPathsReturnEmptyStartupData(t *testing.T) {
	service := NewService(nil, nil)
	ctx := context.Background()

	items, err := service.List(ctx, "")
	if err != nil || len(items) != 0 {
		t.Fatalf("expected empty wanted list without database, got %d items and error %v", len(items), err)
	}

	authors, err := service.ListAuthorSubscriptions(ctx, "monitored")
	if err != nil || len(authors) != 0 {
		t.Fatalf("expected empty author list without database, got %d items and error %v", len(authors), err)
	}

	reviews, err := service.ListAuthorMetadataReviews(ctx, AuthorMetadataReviewQuery{Status: "pending", Limit: 25})
	if err != nil || len(reviews) != 0 {
		t.Fatalf("expected empty author metadata reviews without database, got %d items and error %v", len(reviews), err)
	}

	queue, err := service.MetadataReviewQueue(ctx)
	if err != nil || len(queue.Items) != 0 || queue.GeneratedAt.IsZero() {
		t.Fatalf("expected empty metadata review queue without database, got %+v and error %v", queue, err)
	}

	history, err := service.History(ctx, HistoryQuery{Limit: 25})
	if err != nil || len(history) != 0 {
		t.Fatalf("expected empty history without database, got %d items and error %v", len(history), err)
	}
}

func TestNoDatabaseQualityProfilesReturnDefaults(t *testing.T) {
	profiles, err := NewService(nil, nil).ListQualityProfiles(context.Background())
	if err != nil {
		t.Fatalf("expected default profiles without database, got error %v", err)
	}
	if len(profiles) < 4 {
		t.Fatalf("expected ebook and audiobook default profiles, got %#v", profiles)
	}
	seen := map[string]QualityProfile{}
	for _, profile := range profiles {
		seen[profile.Name+":"+profile.MediaFormat] = profile
	}
	if seen["standard:ebook"].ID == "" || seen["standard:audiobook"].MaxSizeBytes <= seen["standard:ebook"].MaxSizeBytes {
		t.Fatalf("expected format-aware default profiles, got %#v", profiles)
	}
}

func TestNoDatabaseWritePathsStillRequirePersistence(t *testing.T) {
	_, err := NewService(nil, nil).Create(context.Background(), CreateRequest{})
	if err == nil || !strings.Contains(err.Error(), "database persistence") {
		t.Fatalf("expected persistence error, got %v", err)
	}
}

func TestAcquisitionQueueItemStatesReadarrWorkflow(t *testing.T) {
	now := time.Now().UTC()
	item := WantedItem{
		ID:             "wanted-1",
		Title:          "Project Hail Mary",
		AuthorName:     "Andy Weir",
		Format:         "ebook",
		QualityProfile: "standard",
		Status:         "wanted",
		Monitored:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	approved := ReleaseDecision{
		ID:           "release-1",
		WantedItemID: item.ID,
		Title:        "Project Hail Mary EPUB",
		DownloadURL:  "magnet:?xt=urn:btih:abc",
		Protocol:     "torrent",
		Score:        96,
		Approved:     true,
		SearchedAt:   now,
		CreatedAt:    now,
	}

	if row := acquisitionQueueItem(item, nil, nil); row.State != "needs_search" || row.NextAction != "Search releases" {
		t.Fatalf("expected needs-search row, got %+v", row)
	}

	row := acquisitionQueueItem(item, []ReleaseDecision{approved}, nil)
	if row.State != "ready_to_grab" || row.ApprovedCount != 1 || row.BestRelease == nil {
		t.Fatalf("expected ready-to-grab row, got %+v", row)
	}

	row = acquisitionQueueItem(item, []ReleaseDecision{approved}, []acquisition.DownloadStatus{{
		ID:       "download-1",
		Name:     approved.Title,
		State:    "stoppedDL",
		Progress: 0.42,
		Tags:     []string{"librarry", "wanted:" + item.ID},
	}})
	if row.State != "downloading" || row.NextAction != "Wait for external client" {
		t.Fatalf("expected external-client wait row, got %+v", row)
	}

	row = acquisitionQueueItem(item, []ReleaseDecision{approved}, []acquisition.DownloadStatus{{
		ID:           "download-1",
		Name:         approved.Title,
		State:        "uploading",
		Progress:     1,
		ImportStatus: "ready",
		Tags:         []string{"librarry", "wanted:" + item.ID},
	}})
	if row.State != "import_ready" || row.NextAction != "Import completed download" {
		t.Fatalf("expected import-ready row, got %+v", row)
	}

	row = acquisitionQueueItem(item, []ReleaseDecision{approved}, []acquisition.DownloadStatus{{
		ID:            "download-1",
		Name:          approved.Title,
		State:         "error",
		FailureReason: "missing files",
		Tags:          []string{"librarry", "wanted:" + item.ID},
	}})
	if row.State != "blocked" || row.NextAction != "Recover failed download" {
		t.Fatalf("expected blocked recovery row, got %+v", row)
	}
}

func TestReleaseSearchQueryForWantedUsesProtectedBibliographicMetadata(t *testing.T) {
	query := releaseSearchQueryForWanted(WantedItem{
		Title:      "Project Hail Mary",
		AuthorName: "Andy Weir",
		Format:     "ebook",
		ManualOverrides: []ManualOverride{
			{FieldName: "isbn", Value: "9780593135204; 9780593135211"},
			{FieldName: "language", Value: "German"},
		},
	}, 12)

	if query.Query != "Project Hail Mary Andy Weir" {
		t.Fatalf("expected title-author query, got %q", query.Query)
	}
	if query.Author != "Andy Weir" || query.Format != "ebook" || query.Limit != 12 {
		t.Fatalf("unexpected search query fields: %+v", query)
	}
	if query.ISBN != "9780593135204,9780593135211" {
		t.Fatalf("expected protected ISBNs in search query, got %q", query.ISBN)
	}
	if len(query.Languages) != 1 || query.Languages[0] != "German" {
		t.Fatalf("expected protected language in search query, got %#v", query.Languages)
	}
}

func TestWantedDefaultSearchLanguageAppliesWhenNoManualLanguageOverride(t *testing.T) {
	item := wantedItemWithDefaultSearchLanguage(WantedItem{
		Title:      "Project Hail Mary",
		AuthorName: "Andy Weir",
		Format:     "ebook",
	}, "English")
	query := releaseSearchQueryForWanted(item, 20)

	if len(query.Languages) != 1 || query.Languages[0] != "English" {
		t.Fatalf("expected default English language in release query, got %#v", query.Languages)
	}

	item = wantedItemWithDefaultSearchLanguage(WantedItem{
		Title:      "Project Hail Mary",
		AuthorName: "Andy Weir",
		Format:     "ebook",
		ManualOverrides: []ManualOverride{
			{FieldName: "language", Value: "German"},
		},
	}, "English")
	query = releaseSearchQueryForWanted(item, 20)
	if len(query.Languages) != 1 || query.Languages[0] != "German" {
		t.Fatalf("expected manual language override to win, got %#v", query.Languages)
	}
}
