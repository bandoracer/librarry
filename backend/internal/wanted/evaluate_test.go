package wanted

import (
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func TestEvaluateReleaseApprovesStrongEbookMatch(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateRelease(item, release)
	if !decision.Approved {
		t.Fatalf("expected approved release, got rejection %q score %0.2f", decision.RejectedReason, decision.Score)
	}
	// Migration-shaped default ebook ladder: azw3 > epub > mobi > pdf >
	// unknownText, so epub scores (5-1)*1000 = 4000 with no preferred words.
	if decision.Score != 4000 {
		t.Fatalf("expected epub composite score 4000, got %0.2f", decision.Score)
	}
}

func TestEvaluateReleaseIgnoredTermFromReleaseProfile(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Summary and Review Project Hail Mary EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateReleaseWithOptions(item, release, releaseEvaluationOptions{
		profile:     defaultQualityProfile("standard", "ebook"),
		definitions: DefaultQualityDefinitions(),
		releaseProfiles: []ReleaseProfile{{
			Name:    "Migrated terms",
			Enabled: true,
			Ignored: []string{"summary", "review"},
		}},
	})
	if decision.Approved {
		t.Fatalf("expected summary release to be rejected")
	}
	if !strings.Contains(decision.RejectedReason, "ignored term: summary") {
		t.Fatalf("expected ignored-term rejection, got %q", decision.RejectedReason)
	}
	if !strings.Contains(decision.RejectedReason, "ignored term: review") {
		t.Fatalf("expected both ignored terms in reason, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseDisabledReleaseProfileIsIgnored(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB summary",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateReleaseWithOptions(item, release, releaseEvaluationOptions{
		profile:     defaultQualityProfile("standard", "ebook"),
		definitions: DefaultQualityDefinitions(),
		releaseProfiles: []ReleaseProfile{{
			Name:    "disabled",
			Enabled: false,
			Ignored: []string{"summary"},
		}},
	})
	if !decision.Approved {
		t.Fatalf("expected disabled release profile to be skipped, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseRejectsWrongFormat(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "audiobook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateRelease(item, release)
	if decision.Approved {
		t.Fatalf("expected wrong format to be rejected")
	}
	if !strings.Contains(decision.RejectedReason, "audiobook") {
		t.Fatalf("expected audiobook rejection, got %q", decision.RejectedReason)
	}
	if !strings.Contains(decision.RejectedReason, "quality not allowed: epub") {
		t.Fatalf("expected epub to be outside the audiobook ladder, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseApprovesExactISBNMatchWithWeakTitle(t *testing.T) {
	item := WantedItem{
		Title:      "Project Hail Mary",
		AuthorName: "Andy Weir",
		Format:     "ebook",
		ManualOverrides: []ManualOverride{
			{FieldName: "isbn", Value: "9780593135204"},
		},
	}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "978-0-593-13520-4 EPUB retail",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateRelease(item, release)
	if !decision.Approved {
		t.Fatalf("expected exact ISBN release to be approved, got rejection %q score %0.2f", decision.RejectedReason, decision.Score)
	}
	if strings.Contains(decision.RejectedReason, "weak title match") {
		t.Fatalf("expected ISBN match to avoid weak-title rejection, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseEnforcesProtectedNonEnglishLanguage(t *testing.T) {
	item := WantedItem{
		Title:      "Project Hail Mary",
		AuthorName: "Andy Weir",
		Format:     "ebook",
		ManualOverrides: []ManualOverride{
			{FieldName: "language", Value: "German"},
		},
	}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateRelease(item, release)
	if decision.Approved {
		t.Fatal("expected release without protected German language marker to be rejected")
	}
	if !strings.Contains(decision.RejectedReason, "missing requested language") {
		t.Fatalf("expected protected language rejection, got %q", decision.RejectedReason)
	}

	release.Title = "Project Hail Mary by Andy Weir German EPUB"
	decision = evaluateRelease(item, release)
	if !decision.Approved {
		t.Fatalf("expected explicit German release to be approved, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseQualityNotAllowed(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir PDF",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	profile := defaultQualityProfile("standard", "ebook")
	for i := range profile.Qualities {
		if profile.Qualities[i].ID == QualityPDF {
			profile.Qualities[i].Allowed = false
		}
	}
	decision := evaluateReleaseWithProfile(item, release, profile)
	if decision.Approved {
		t.Fatal("expected disallowed quality to reject release")
	}
	if !strings.Contains(decision.RejectedReason, "quality not allowed: pdf") {
		t.Fatalf("expected quality-not-allowed reason, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseRanksByLadderThenPreferredScore(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	base := acquisition.Release{
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	options := releaseEvaluationOptions{
		profile:     defaultQualityProfile("standard", "ebook"),
		definitions: DefaultQualityDefinitions(),
		releaseProfiles: []ReleaseProfile{{
			Enabled:   true,
			Preferred: []PreferredTerm{{Term: "retail", Score: 20}},
		}},
	}
	azw3 := base
	azw3.ID, azw3.Title = "azw3", "Project Hail Mary by Andy Weir AZW3"
	epubRetail := base
	epubRetail.ID, epubRetail.Title = "epub-retail", "Project Hail Mary by Andy Weir EPUB retail"
	epub := base
	epub.ID, epub.Title = "epub", "Project Hail Mary by Andy Weir EPUB"
	mobi := base
	mobi.ID, mobi.Title = "mobi", "Project Hail Mary by Andy Weir MOBI"

	azw3Decision := evaluateReleaseWithOptions(item, azw3, options)
	epubRetailDecision := evaluateReleaseWithOptions(item, epubRetail, options)
	epubDecision := evaluateReleaseWithOptions(item, epub, options)
	mobiDecision := evaluateReleaseWithOptions(item, mobi, options)

	if azw3Decision.Score != 5000 || epubRetailDecision.Score != 4020 || epubDecision.Score != 4000 || mobiDecision.Score != 3000 {
		t.Fatalf("unexpected composite scores: azw3=%0.1f epub+retail=%0.1f epub=%0.1f mobi=%0.1f",
			azw3Decision.Score, epubRetailDecision.Score, epubDecision.Score, mobiDecision.Score)
	}
	// Ladder position dominates the preferred-word component.
	if !(azw3Decision.Score > epubRetailDecision.Score && epubRetailDecision.Score > epubDecision.Score && epubDecision.Score > mobiDecision.Score) {
		t.Fatal("expected ladder-then-preferred ranking order")
	}
}

func TestEvaluateReleaseSizeGatesFromQualityDefinitions(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   700 * 1024 * 1024,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateRelease(item, release)
	if decision.Approved {
		t.Fatal("expected 700MB epub to exceed the 500MB definition")
	}
	if !strings.Contains(decision.RejectedReason, "above maximum size for epub") {
		t.Fatalf("expected above-maximum-size reason, got %q", decision.RejectedReason)
	}

	release.SizeBytes = 512 * 1024
	decision = evaluateReleaseWithOptions(item, release, releaseEvaluationOptions{
		profile: defaultQualityProfile("standard", "ebook"),
		definitions: []QualityDefinition{
			{Quality: QualityEPUB, Title: "EPUB", MinSizeMB: 1, MaxSizeMB: 500},
		},
	})
	if decision.Approved {
		t.Fatal("expected sub-minimum epub to be rejected")
	}
	if !strings.Contains(decision.RejectedReason, "below minimum size for epub") {
		t.Fatalf("expected below-minimum-size reason, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseRequiredTermFromReleaseProfile(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateReleaseWithOptions(item, release, releaseEvaluationOptions{
		profile:     defaultQualityProfile("standard", "ebook"),
		definitions: DefaultQualityDefinitions(),
		releaseProfiles: []ReleaseProfile{{
			Enabled:  true,
			Required: []string{"retail"},
		}},
	})
	if decision.Approved {
		t.Fatal("expected missing required term to reject release")
	}
	if !strings.Contains(decision.RejectedReason, "missing required term: retail") {
		t.Fatalf("expected missing-required-term reason, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseEnforcesProfileMinimumSeeders(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "retail"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB retail",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     2,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	profile := defaultQualityProfile("retail", "ebook")
	profile.MinSeeders = 3
	decision := evaluateReleaseWithProfile(item, release, profile)
	if decision.Approved {
		t.Fatal("expected low-seeder torrent to be rejected")
	}
	if !strings.Contains(decision.RejectedReason, "minimum seeders") {
		t.Fatalf("expected minimum seeders rejection, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseAppliesGlobalRestrictions(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB retail screener",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateReleaseWithPolicy(item, release, defaultQualityProfile("standard", "ebook"), []ReleaseRestriction{
		NewReleaseRestriction("1", "retail", "screener", "", nil),
	})
	if decision.Approved {
		t.Fatal("expected ignored restriction term to reject release")
	}
	if !strings.Contains(decision.RejectedReason, "ignored term: screener") {
		t.Fatalf("expected restriction ignored-term reason, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseRequiresGlobalRestrictionTerm(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateReleaseWithPolicy(item, release, defaultQualityProfile("standard", "ebook"), []ReleaseRestriction{
		NewReleaseRestriction("1", "retail", "", "", nil),
	})
	if decision.Approved {
		t.Fatal("expected missing required restriction term to reject release")
	}
	if !strings.Contains(decision.RejectedReason, "missing required term: retail") {
		t.Fatalf("expected missing required restriction reason, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseIgnoresTaggedRestrictionsUntilItemTagsExist(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB screener",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateReleaseWithPolicy(item, release, defaultQualityProfile("standard", "ebook"), []ReleaseRestriction{
		NewReleaseRestriction("1", "", "screener", "", []int{7}),
	})
	if !decision.Approved {
		t.Fatalf("expected tagged restriction to be ignored without item tags, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseAppliesTaggedRestrictionsForMatchingWantedTags(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard", Tags: []string{"7"}}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB screener",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateReleaseWithPolicy(item, release, defaultQualityProfile("standard", "ebook"), []ReleaseRestriction{
		NewReleaseRestriction("1", "", "screener", "", []int{7}),
		NewReleaseRestriction("2", "retail", "", "", []int{9}),
	})
	if decision.Approved {
		t.Fatal("expected matching tagged restriction to reject release")
	}
	if !strings.Contains(decision.RejectedReason, "ignored term: screener") {
		t.Fatalf("expected matching tagged restriction reason, got %q", decision.RejectedReason)
	}
	if strings.Contains(decision.RejectedReason, "retail") {
		t.Fatalf("expected non-matching tagged restriction to be ignored, got %q", decision.RejectedReason)
	}
}

func TestParseReleaseRestrictionTermsDeduplicatesSeparators(t *testing.T) {
	terms := ParseReleaseRestrictionTerms("Retail, retail; WEB\nproper|")
	if strings.Join(terms, "|") != "Retail|WEB|proper" {
		t.Fatalf("unexpected terms: %#v", terms)
	}
}
