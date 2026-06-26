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
	if decision.Score < 60 {
		t.Fatalf("expected useful score, got %0.2f", decision.Score)
	}
}

func TestEvaluateReleaseRejectsSummary(t *testing.T) {
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
	decision := evaluateRelease(item, release)
	if decision.Approved {
		t.Fatalf("expected summary release to be rejected")
	}
	if !strings.Contains(decision.RejectedReason, "summary") {
		t.Fatalf("expected summary rejection, got %q", decision.RejectedReason)
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
}

func TestEvaluateReleaseUsesProfileRequiredAndRejectedTerms(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "strict"}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB retail summary",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateReleaseWithProfile(item, release, QualityProfile{
		Name:          "strict",
		MediaFormat:   "ebook",
		MinScore:      70,
		CutoffScore:   95,
		MinSeeders:    5,
		RequiredTerms: []string{"retail"},
		RejectedTerms: []string{"summary"},
	})
	if decision.Approved {
		t.Fatal("expected rejected term to reject release")
	}
	if !strings.Contains(decision.RejectedReason, "rejected term: summary") {
		t.Fatalf("expected rejected-term reason, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseAppliesPreferredTermsAndMinimumSeeders(t *testing.T) {
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
	decision := evaluateReleaseWithProfile(item, release, QualityProfile{
		Name:           "retail",
		MediaFormat:    "ebook",
		MinScore:       70,
		CutoffScore:    90,
		MinSeeders:     3,
		PreferredTerms: []string{"retail", "epub"},
		RejectedTerms:  []string{"summary", "review"},
		PreferredScore: 10,
	})
	if decision.Approved {
		t.Fatal("expected low-seeder torrent to be rejected")
	}
	if !strings.Contains(decision.RejectedReason, "minimum seeders") {
		t.Fatalf("expected minimum seeders rejection, got %q", decision.RejectedReason)
	}
	if decision.Score < 69 {
		t.Fatalf("expected preferred terms to improve score before rejection cap, got %0.2f", decision.Score)
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
	decision := evaluateReleaseWithPolicy(item, release, QualityProfile{
		Name:          "standard",
		MediaFormat:   "ebook",
		MinScore:      60,
		CutoffScore:   85,
		MinSeeders:    1,
		RejectedTerms: []string{"summary", "review"},
	}, []ReleaseRestriction{
		NewReleaseRestriction("1", "retail", "screener", "", nil),
	})
	if decision.Approved {
		t.Fatal("expected ignored restriction term to reject release")
	}
	if !strings.Contains(decision.RejectedReason, "rejected term: screener") {
		t.Fatalf("expected restriction rejected-term reason, got %q", decision.RejectedReason)
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
	decision := evaluateReleaseWithPolicy(item, release, QualityProfile{
		Name:          "standard",
		MediaFormat:   "ebook",
		MinScore:      60,
		CutoffScore:   85,
		MinSeeders:    1,
		RejectedTerms: []string{"summary", "review"},
	}, []ReleaseRestriction{
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
	decision := evaluateReleaseWithPolicy(item, release, QualityProfile{
		Name:          "standard",
		MediaFormat:   "ebook",
		MinScore:      60,
		CutoffScore:   85,
		MinSeeders:    1,
		RejectedTerms: []string{"summary", "review"},
	}, []ReleaseRestriction{
		NewReleaseRestriction("1", "", "screener", "", []int{7}),
	})
	if !decision.Approved {
		t.Fatalf("expected tagged restriction to be ignored without item tags, got %q", decision.RejectedReason)
	}
}

func TestEvaluateReleaseAppliesTaggedRestrictionsForMatchingWantedTags(t *testing.T) {
	item := WantedItem{Title: "Project Hail Mary", AuthorName: "Andy Weir", Format: "ebook", QualityProfile: "standard", Tags: []int{7}}
	release := acquisition.Release{
		ID:          "r1",
		Title:       "Project Hail Mary by Andy Weir EPUB screener",
		DownloadURL: "magnet:?xt=urn:btih:abc",
		Protocol:    "torrent",
		Seeders:     20,
		SizeBytes:   2_000_000,
		Categories:  []string{"Books/EBook"},
	}
	decision := evaluateReleaseWithPolicy(item, release, QualityProfile{
		Name:          "standard",
		MediaFormat:   "ebook",
		MinScore:      60,
		CutoffScore:   85,
		MinSeeders:    1,
		RejectedTerms: []string{"summary", "review"},
	}, []ReleaseRestriction{
		NewReleaseRestriction("1", "", "screener", "", []int{7}),
		NewReleaseRestriction("2", "retail", "", "", []int{9}),
	})
	if decision.Approved {
		t.Fatal("expected matching tagged restriction to reject release")
	}
	if !strings.Contains(decision.RejectedReason, "rejected term: screener") {
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
