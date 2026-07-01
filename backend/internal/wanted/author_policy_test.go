package wanted

import (
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

func policyTestCandidate(editionID string, title string, published string) metadata.SearchResult {
	return metadata.SearchResult{
		Provider: "openlibrary",
		Work:     metadata.Work{ID: "openlibrary:" + title, Title: title},
		Edition:  metadata.Edition{ID: editionID, PublishedDate: published},
	}
}

func policyTestSubscription(policy string) AuthorSubscription {
	return AuthorSubscription{
		AuthorName:        "Andy Weir",
		Format:            "ebook",
		MonitorNewItems:   true,
		MissingBookPolicy: policy,
		CreatedAt:         time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestNormalizeAuthorMissingBookPolicyModes(t *testing.T) {
	cases := map[string]string{
		"all":      "all",
		"missing":  "missing",
		"existing": "existing",
		"first":    "first",
		"latest":   "latest",
		"future":   "future",
		"none":     "none",
		"Missing":  "missing",
		"":         "all",
	}
	for input, expected := range cases {
		if got := normalizeAuthorMissingBookPolicy(input, true); got != expected {
			t.Fatalf("normalizeAuthorMissingBookPolicy(%q) = %q, expected %q", input, got, expected)
		}
	}
	if got := normalizeAuthorMissingBookPolicy("", false); got != "none" {
		t.Fatalf("expected unmonitored fallback none, got %q", got)
	}
}

func TestAuthorPolicyMissingSkipsBooksWithFiles(t *testing.T) {
	subscription := policyTestSubscription("missing")
	withFile := policyTestCandidate("edition-1", "The Martian", "2014-02-11")
	withoutFile := policyTestCandidate("edition-2", "Project Hail Mary", "2021-05-04")
	fileKeys := map[string]bool{
		wantedSourceFileKey("openlibrary", "edition-1", "ebook"): true,
	}
	policyCtx := buildAuthorPolicyContext(subscription, []metadata.SearchResult{withFile, withoutFile}, fileKeys, time.Now().UTC())

	if allowed, reason := authorResultAllowedByPolicy(subscription, withFile, policyCtx); allowed || reason == "" {
		t.Fatalf("expected book with library file to be skipped, allowed=%v reason=%q", allowed, reason)
	}
	if allowed, _ := authorResultAllowedByPolicy(subscription, withoutFile, policyCtx); !allowed {
		t.Fatal("expected missing book to be allowed")
	}
}

func TestAuthorPolicyExistingRequiresFileOrFuture(t *testing.T) {
	subscription := policyTestSubscription("existing")
	withFile := policyTestCandidate("edition-1", "The Martian", "2014-02-11")
	oldWithoutFile := policyTestCandidate("edition-2", "Artemis", "2017-11-14")
	future := policyTestCandidate("edition-3", "Project Next", "2027-03-01")
	fileKeys := map[string]bool{
		wantedSourceFileKey("openlibrary", "edition-1", "ebook"): true,
	}
	candidates := []metadata.SearchResult{withFile, oldWithoutFile, future}
	policyCtx := buildAuthorPolicyContext(subscription, candidates, fileKeys, time.Now().UTC())

	if allowed, _ := authorResultAllowedByPolicy(subscription, withFile, policyCtx); !allowed {
		t.Fatal("expected book with library file to be allowed")
	}
	if allowed, reason := authorResultAllowedByPolicy(subscription, oldWithoutFile, policyCtx); allowed || reason == "" {
		t.Fatalf("expected old book without file to be skipped, allowed=%v reason=%q", allowed, reason)
	}
	if allowed, _ := authorResultAllowedByPolicy(subscription, future, policyCtx); !allowed {
		t.Fatal("expected future book to be allowed under existing policy")
	}
}

func TestAuthorPolicyFirstSelectsEarliestBook(t *testing.T) {
	subscription := policyTestSubscription("first")
	first := policyTestCandidate("edition-1", "The Martian", "2014-02-11")
	later := policyTestCandidate("edition-2", "Project Hail Mary", "2021-05-04")
	// Discovery order intentionally reversed: dates decide, not order.
	policyCtx := buildAuthorPolicyContext(subscription, []metadata.SearchResult{later, first}, nil, time.Now().UTC())

	if allowed, _ := authorResultAllowedByPolicy(subscription, first, policyCtx); !allowed {
		t.Fatal("expected earliest book to be allowed")
	}
	if allowed, reason := authorResultAllowedByPolicy(subscription, later, policyCtx); allowed || reason == "" {
		t.Fatalf("expected later book to be skipped, allowed=%v reason=%q", allowed, reason)
	}
}

func TestAuthorPolicyLatestSelectsMostRecentAndFuture(t *testing.T) {
	subscription := policyTestSubscription("latest")
	oldest := policyTestCandidate("edition-1", "The Martian", "2014-02-11")
	latest := policyTestCandidate("edition-2", "Project Hail Mary", "2021-05-04")
	future := policyTestCandidate("edition-3", "Project Next", "2027-03-01")
	policyCtx := buildAuthorPolicyContext(subscription, []metadata.SearchResult{oldest, latest, future}, nil, time.Now().UTC())

	// The future title carries the newest date, so it is both "latest" and future.
	if allowed, _ := authorResultAllowedByPolicy(subscription, future, policyCtx); !allowed {
		t.Fatal("expected future book to be allowed under latest policy")
	}
	if allowed, reason := authorResultAllowedByPolicy(subscription, oldest, policyCtx); allowed || reason == "" {
		t.Fatalf("expected oldest book to be skipped, allowed=%v reason=%q", allowed, reason)
	}

	// Without the future title, the newest published book wins.
	policyCtx = buildAuthorPolicyContext(subscription, []metadata.SearchResult{oldest, latest}, nil, time.Now().UTC())
	if allowed, _ := authorResultAllowedByPolicy(subscription, latest, policyCtx); !allowed {
		t.Fatal("expected most recent book to be allowed under latest policy")
	}
}

func TestBuildAuthorPolicyContextFallsBackToDiscoveryOrder(t *testing.T) {
	subscription := policyTestSubscription("first")
	undatedA := policyTestCandidate("edition-1", "Book A", "")
	undatedB := policyTestCandidate("edition-2", "Book B", "")
	policyCtx := buildAuthorPolicyContext(subscription, []metadata.SearchResult{undatedA, undatedB}, nil, time.Now().UTC())
	if policyCtx.firstKey != authorMetadataReviewCandidateKey(undatedA) {
		t.Fatalf("expected first discovered candidate as firstKey, got %q", policyCtx.firstKey)
	}
	if policyCtx.latestKey != authorMetadataReviewCandidateKey(undatedB) {
		t.Fatalf("expected last discovered candidate as latestKey, got %q", policyCtx.latestKey)
	}
}

func TestCutoffUnmetPredicate(t *testing.T) {
	profile := QualityProfile{CutoffScore: 85, UpgradeAllowed: true}
	if !cutoffUnmet(profile, 70) {
		t.Fatal("expected score below cutoff to be unmet")
	}
	if cutoffUnmet(profile, 85) {
		t.Fatal("expected score at cutoff to be met")
	}
	if cutoffUnmet(profile, 92) {
		t.Fatal("expected score above cutoff to be met")
	}
	locked := QualityProfile{CutoffScore: 85, UpgradeAllowed: false}
	if cutoffUnmet(locked, 10) {
		t.Fatal("expected upgrade-locked profile to never be cutoff unmet")
	}
}
