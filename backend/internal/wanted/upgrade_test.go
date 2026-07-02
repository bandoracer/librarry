package wanted

import (
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

// Composite scores below follow the default ebook ladder
// (azw3 > epub > mobi > pdf > unknownText, cutoff epub): azw3=5000, epub=4000,
// mobi=3000, plus the sub-1000 preferred-word component.

func TestBestUpgradeReleaseBlocksQualityAboveCutoff(t *testing.T) {
	// Current epub (+8 preferred) already meets the epub cutoff; a bare azw3
	// (higher quality, no preferred words) must not upgrade past the cutoff.
	_, ok := bestUpgradeRelease([]ReleaseDecision{
		{ID: "azw3", Approved: true, Score: 5000, Seeders: 30},
	}, 4008, 4000)
	if ok {
		t.Fatal("expected no quality upgrade once cutoff quality is met")
	}
}

func TestBestUpgradeReleasePreferredScoreAlwaysUpgrades(t *testing.T) {
	// arr rule: even at/above the cutoff quality, a release with a higher
	// preferred-word score is always an upgrade.
	release, ok := bestUpgradeRelease([]ReleaseDecision{
		{ID: "better-terms", Approved: true, Score: 4020, Seeders: 10},
	}, 4008, 4000)
	if !ok || release.ID != "better-terms" {
		t.Fatalf("expected preferred-score upgrade above cutoff, got ok=%v release=%+v", ok, release)
	}
}

func TestBestUpgradeReleaseRequiresStrictImprovement(t *testing.T) {
	_, ok := bestUpgradeRelease([]ReleaseDecision{
		{ID: "same", Approved: true, Score: 3005, Seeders: 30},
	}, 3005, 4000)
	if ok {
		t.Fatal("expected no upgrade without a strictly better composite score")
	}
}

func TestBestUpgradeReleasePicksHighestApprovedCandidate(t *testing.T) {
	release, ok := bestUpgradeRelease([]ReleaseDecision{
		{ID: "rejected", Approved: false, Score: 5000, Seeders: 50},
		{ID: "good", Approved: true, Score: 4000, Seeders: 15},
		{ID: "best", Approved: true, Score: 4020, Seeders: 4},
	}, 3005, 4000)
	if !ok {
		t.Fatal("expected upgrade candidate")
	}
	if release.ID != "best" {
		t.Fatalf("expected highest approved candidate, got %+v", release)
	}
}

func TestCutoffCompositeScoreDefaults(t *testing.T) {
	ebook := defaultQualityProfile("standard", "ebook")
	if got := ebook.CutoffCompositeScore(); got != 4000 {
		t.Fatalf("expected ebook epub cutoff composite 4000, got %0.1f", got)
	}
	audiobook := defaultQualityProfile("standard", "audiobook")
	if got := audiobook.CutoffCompositeScore(); got != 3000 {
		t.Fatalf("expected audiobook m4b cutoff composite 3000, got %0.1f", got)
	}
}

func TestCutoffUnmetUsesCompositeThreshold(t *testing.T) {
	profile := defaultQualityProfile("standard", "ebook")
	if !cutoffUnmet(profile, 3005) {
		t.Fatal("expected mobi-quality score below epub cutoff to be unmet")
	}
	if cutoffUnmet(profile, 4008) {
		t.Fatal("expected epub-quality score to meet the epub cutoff")
	}
	profile.UpgradeAllowed = false
	if cutoffUnmet(profile, 3005) {
		t.Fatal("expected upgrade-disabled profile to never report cutoff unmet")
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
	policyCtx := authorPolicyContext{now: time.Now().UTC()}
	newBook := metadata.SearchResult{
		Work:    metadata.Work{FirstPublishYear: 2027},
		Edition: metadata.Edition{PublishedDate: "2027-03-01"},
	}
	if allowed, reason := authorResultAllowedByPolicy(subscription, newBook, policyCtx); !allowed || reason != "" {
		t.Fatal("expected future-dated result to be allowed")
	}

	oldBook := metadata.SearchResult{
		Work:    metadata.Work{FirstPublishYear: 2021},
		Edition: metadata.Edition{PublishedDate: "2021-05-04"},
	}
	if allowed, reason := authorResultAllowedByPolicy(subscription, oldBook, policyCtx); allowed || reason == "" {
		t.Fatal("expected existing bibliography result to be skipped")
	}

	undatedBook := metadata.SearchResult{Work: metadata.Work{Title: "Untitled"}}
	if allowed, reason := authorResultAllowedByPolicy(subscription, undatedBook, policyCtx); allowed || reason == "" {
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
