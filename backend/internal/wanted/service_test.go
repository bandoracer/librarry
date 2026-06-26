package wanted

import (
	"context"
	"strings"
	"testing"
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
