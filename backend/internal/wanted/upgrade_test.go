package wanted

import (
	"testing"

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
