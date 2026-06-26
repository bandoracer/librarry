package wanted

import (
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

func TestBestUpgradeReleaseRequiresCurrentBelowCutoff(t *testing.T) {
	_, ok := bestUpgradeRelease([]ReleaseDecision{
		{ID: "better", Approved: true, Score: 96, Seeders: 30},
	}, 90, 85, 5)
	if ok {
		t.Fatal("expected no upgrade when current score already meets cutoff")
	}
}

func TestBestUpgradeReleaseRequiresScoreDelta(t *testing.T) {
	_, ok := bestUpgradeRelease([]ReleaseDecision{
		{ID: "small-bump", Approved: true, Score: 82, Seeders: 30},
	}, 80, 85, 5)
	if ok {
		t.Fatal("expected no upgrade below score delta")
	}
}

func TestBestUpgradeReleasePicksHighestApprovedCandidate(t *testing.T) {
	release, ok := bestUpgradeRelease([]ReleaseDecision{
		{ID: "rejected", Approved: false, Score: 99, Seeders: 50},
		{ID: "good", Approved: true, Score: 86, Seeders: 15},
		{ID: "best", Approved: true, Score: 92, Seeders: 4},
	}, 70, 85, 5)
	if !ok {
		t.Fatal("expected upgrade candidate")
	}
	if release.ID != "best" {
		t.Fatalf("expected highest approved candidate, got %+v", release)
	}
}

func TestUpgradeCutoffForProfiles(t *testing.T) {
	if got := upgradeCutoffFor(WantedItem{QualityProfile: "standard"}); got != 85 {
		t.Fatalf("expected standard cutoff 85, got %0.1f", got)
	}
	if got := upgradeCutoffFor(WantedItem{QualityProfile: "large"}); got != 90 {
		t.Fatalf("expected large cutoff 90, got %0.1f", got)
	}
}

func TestAuthorSubscriptionFromSearchResult(t *testing.T) {
	subscription := authorSubscriptionFromRequest(AuthorSubscribeRequest{
		Result: metadata.SearchResult{
			Provider: "Open Library",
			Work: metadata.Work{
				Authors: []metadata.Author{{ID: "openlibrary:OL123A", Name: "Andy Weir"}},
			},
			Edition: metadata.Edition{Format: metadata.FormatAudiobook},
		},
		QualityProfile: "large",
	})
	if subscription.AuthorName != "Andy Weir" || subscription.ProviderKey != "openlibrary:OL123A" {
		t.Fatalf("unexpected author identity: %+v", subscription)
	}
	if subscription.Format != "audiobook" || subscription.QualityProfile != "large" || !subscription.MonitorNewItems {
		t.Fatalf("unexpected subscription policy: %+v", subscription)
	}
	if subscription.MissingBookPolicy != "all" {
		t.Fatalf("expected default all-books missing policy, got %q", subscription.MissingBookPolicy)
	}
}

func TestAuthorSubscriptionPolicyFromRequest(t *testing.T) {
	subscription := authorSubscriptionFromRequest(AuthorSubscribeRequest{
		AuthorName:        "Terry Pratchett",
		Provider:          "Open Library",
		ProviderKey:       "openlibrary:OL25712A",
		MissingBookPolicy: "futureBooks",
	})
	if subscription.MissingBookPolicy != "future" || !subscription.MonitorNewItems {
		t.Fatalf("expected future policy to monitor new books, got %+v", subscription)
	}

	monitor := true
	subscription = authorSubscriptionFromRequest(AuthorSubscribeRequest{
		AuthorName:        "Terry Pratchett",
		MissingBookPolicy: "none",
		MonitorNewItems:   &monitor,
	})
	if subscription.MissingBookPolicy != "none" || subscription.MonitorNewItems {
		t.Fatalf("expected none policy to disable new-book monitoring, got %+v", subscription)
	}
}

func TestAuthorResultMatchesSubscription(t *testing.T) {
	subscription := AuthorSubscription{
		ProviderKey: "openlibrary:OL123A",
		AuthorName:  "Andy Weir",
	}
	match := metadata.SearchResult{Work: metadata.Work{Authors: []metadata.Author{{ID: "openlibrary:OL123A", Name: "Andrew Weir"}}}}
	if !authorResultMatchesSubscription(subscription, match) {
		t.Fatal("expected provider key author match")
	}
	nameMatch := metadata.SearchResult{Work: metadata.Work{Authors: []metadata.Author{{ID: "other", Name: "Andy Weir"}}}}
	if !authorResultMatchesSubscription(subscription, nameMatch) {
		t.Fatal("expected normalized author name match")
	}
	miss := metadata.SearchResult{Work: metadata.Work{Authors: []metadata.Author{{ID: "other", Name: "Terry Pratchett"}}}}
	if authorResultMatchesSubscription(subscription, miss) {
		t.Fatal("expected unrelated author to miss")
	}
}

func TestAuthorMissingBookPolicyAllowsFutureOnly(t *testing.T) {
	subscription := AuthorSubscription{
		AuthorName:        "Andy Weir",
		MonitorNewItems:   true,
		MissingBookPolicy: "future",
		CreatedAt:         time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
	}
	newBook := metadata.SearchResult{
		Work:    metadata.Work{FirstPublishYear: 2027},
		Edition: metadata.Edition{PublishedDate: "2027-03-01"},
	}
	if allowed, reason := authorResultAllowedByMissingPolicy(subscription, newBook, time.Now()); !allowed || reason != "" {
		t.Fatal("expected future-dated result to be allowed")
	}

	oldBook := metadata.SearchResult{
		Work:    metadata.Work{FirstPublishYear: 2021},
		Edition: metadata.Edition{PublishedDate: "2021-05-04"},
	}
	if allowed, reason := authorResultAllowedByMissingPolicy(subscription, oldBook, time.Now()); allowed || reason == "" {
		t.Fatal("expected existing bibliography result to be skipped")
	}

	undatedBook := metadata.SearchResult{Work: metadata.Work{Title: "Untitled"}}
	if allowed, reason := authorResultAllowedByMissingPolicy(subscription, undatedBook, time.Now()); allowed || reason == "" {
		t.Fatal("expected undated future-only result to require review instead of auto-wanted")
	}
}

func TestAuthorSkippedItemCarriesMetadataResult(t *testing.T) {
	result := metadata.SearchResult{
		Provider: "Open Library",
		Kind:     metadata.SearchTypeAuthor,
		Work: metadata.Work{
			ID:               "openlibrary:OL1W",
			Title:            "Project Hail Mary",
			FirstPublishYear: 2021,
			Authors:          []metadata.Author{{ID: "openlibrary:OL123A", Name: "Andy Weir"}},
		},
		Edition: metadata.Edition{ID: "openlibrary:OL1M", Format: metadata.FormatEbook, PublishedDate: "2021-05-04"},
		Score:   94,
	}
	item := AuthorSkippedItem{Result: result, Policy: "future", Reason: "published before the author subscription cutoff"}
	if item.Result.Work.Title != "Project Hail Mary" || item.Result.Edition.PublishedDate != "2021-05-04" {
		t.Fatalf("expected skipped item to preserve metadata result, got %+v", item)
	}
	if item.Policy != "future" || item.Reason == "" {
		t.Fatalf("expected skipped item policy and reason, got %+v", item)
	}
}
