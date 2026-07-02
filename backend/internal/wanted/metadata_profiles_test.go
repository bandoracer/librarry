package wanted

import (
	"context"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

func TestNormalizeMetadataProfileValidation(t *testing.T) {
	if _, err := normalizeMetadataProfile(MetadataProfile{Name: "   "}); err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected name-required error, got %v", err)
	}

	profile, err := normalizeMetadataProfile(MetadataProfile{
		Name:             "  English only  ",
		AllowedLanguages: []string{" en ", "en", "", "fr"},
		MustNotContain:   []string{" boxed set ", "Boxed Set", ""},
		MinPages:         -10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "English only" {
		t.Fatalf("expected trimmed name, got %q", profile.Name)
	}
	if len(profile.AllowedLanguages) != 2 || profile.AllowedLanguages[0] != "en" || profile.AllowedLanguages[1] != "fr" {
		t.Fatalf("expected deduped languages, got %#v", profile.AllowedLanguages)
	}
	if len(profile.MustNotContain) != 1 || profile.MustNotContain[0] != "boxed set" {
		t.Fatalf("expected deduped terms, got %#v", profile.MustNotContain)
	}
	if profile.MinPages != 0 {
		t.Fatalf("expected negative min pages clamped to 0, got %d", profile.MinPages)
	}
}

func TestApplyMetadataProfileFiltersProfileWinsOverOverrides(t *testing.T) {
	subscription := AuthorSubscription{
		ID:                "author-1",
		MetadataProfileID: "profile-1",
		// Per-author overrides that must lose to the profile.
		AllowedLanguages: []string{"de"},
		MustNotContain:   []string{"omnibus"},
		SkipMissingISBN:  true,
		MinPages:         500,
	}
	profile := MetadataProfile{
		ID:               "profile-1",
		Name:             "English only",
		AllowedLanguages: []string{"en"},
		MustNotContain:   nil,
		SkipMissingISBN:  false,
		MinPages:         0,
	}

	effective := applyMetadataProfileFilters(subscription, profile)
	if len(effective.AllowedLanguages) != 1 || effective.AllowedLanguages[0] != "en" {
		t.Fatalf("expected profile languages to win, got %#v", effective.AllowedLanguages)
	}
	if len(effective.MustNotContain) != 0 {
		t.Fatalf("expected profile (empty) must-not-contain to win, got %#v", effective.MustNotContain)
	}
	if effective.SkipMissingISBN {
		t.Fatal("expected profile skip-missing-isbn=false to win over per-author true")
	}
	if effective.MinPages != 0 {
		t.Fatalf("expected profile min pages to win, got %d", effective.MinPages)
	}

	// Filter evaluation follows the resolved subscription: an ISBN-less German
	// candidate is rejected by the overrides but accepted under the profile.
	candidate := metadata.SearchResult{}
	candidate.Work.Title = "Der Marsianer"
	candidate.Edition.Title = "Der Marsianer"
	candidate.Edition.Language = "de"

	if reason := authorResultFilterReason(subscription, candidate); reason != filterReasonMissingISBN {
		t.Fatalf("expected per-author overrides to reject on missing ISBN, got %q", reason)
	}
	if reason := authorResultFilterReason(effective, candidate); reason != filterReasonLanguage {
		t.Fatalf("expected profile language filter to reject, got %q", reason)
	}

	candidate.Edition.Language = "en"
	if reason := authorResultFilterReason(effective, candidate); reason != "" {
		t.Fatalf("expected profile-resolved filters to accept, got %q", reason)
	}
}

func TestApplyMetadataProfileFiltersKeepsOverridesWithoutProfileReference(t *testing.T) {
	subscription := AuthorSubscription{
		AllowedLanguages: []string{"de"},
		SkipMissingISBN:  true,
		MinPages:         100,
	}
	profile := MetadataProfile{AllowedLanguages: []string{"en"}}

	effective := applyMetadataProfileFilters(subscription, profile)
	if len(effective.AllowedLanguages) != 1 || effective.AllowedLanguages[0] != "de" ||
		!effective.SkipMissingISBN || effective.MinPages != 100 {
		t.Fatalf("expected per-author filters untouched without metadataProfileId, got %#v", effective)
	}
}

func TestMetadataProfilesRequirePersistenceForWrites(t *testing.T) {
	service := NewService(nil, nil)
	ctx := context.Background()

	profiles, err := service.ListMetadataProfiles(ctx)
	if err != nil || len(profiles) != 0 {
		t.Fatalf("expected empty profiles without database, got %d and error %v", len(profiles), err)
	}
	if _, err := service.CreateMetadataProfile(ctx, MetadataProfile{Name: "X"}); err == nil || !strings.Contains(err.Error(), "database persistence") {
		t.Fatalf("expected persistence error on create, got %v", err)
	}
	if _, err := service.UpdateMetadataProfile(ctx, "id", MetadataProfile{Name: "X"}); err == nil || !strings.Contains(err.Error(), "database persistence") {
		t.Fatalf("expected persistence error on update, got %v", err)
	}
	if err := service.DeleteMetadataProfile(ctx, "id"); err == nil || !strings.Contains(err.Error(), "database persistence") {
		t.Fatalf("expected persistence error on delete, got %v", err)
	}
}

func TestAuthorSubscriptionFromRequestCarriesMetadataProfileID(t *testing.T) {
	subscription := authorSubscriptionFromRequest(AuthorSubscribeRequest{
		AuthorName:        "Andy Weir",
		MetadataProfileID: "  profile-1  ",
	})
	if subscription.MetadataProfileID != "profile-1" {
		t.Fatalf("expected trimmed metadata profile id, got %q", subscription.MetadataProfileID)
	}
}
