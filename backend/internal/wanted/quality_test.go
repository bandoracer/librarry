package wanted

import (
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func TestDetectReleaseQuality(t *testing.T) {
	ebookProfile := defaultQualityProfile("standard", "ebook")
	audiobookProfile := defaultQualityProfile("standard", "audiobook")
	tests := []struct {
		name     string
		title    string
		category string
		profile  QualityProfile
		format   string
		want     string
	}{
		{name: "epub token", title: "Project Hail Mary EPUB retail", profile: ebookProfile, format: "ebook", want: QualityEPUB},
		{name: "case insensitive", title: "Project Hail Mary ePuB", profile: ebookProfile, format: "ebook", want: QualityEPUB},
		{name: "file extension", title: "Project Hail Mary.azw3", profile: ebookProfile, format: "ebook", want: QualityAZW3},
		{name: "mobi word bounded", title: "Project Hail Mary mobile edition", profile: ebookProfile, format: "ebook", want: QualityUnknownText},
		{name: "mobi token", title: "Project Hail Mary MOBI", profile: ebookProfile, format: "ebook", want: QualityMOBI},
		{name: "pdf token", title: "Project Hail Mary [PDF]", profile: ebookProfile, format: "ebook", want: QualityPDF},
		{name: "category token", title: "Project Hail Mary", category: "Books/EPUB", profile: ebookProfile, format: "ebook", want: QualityEPUB},
		{name: "bundle picks most preferred", title: "Project Hail Mary EPUB MOBI PDF", profile: ebookProfile, format: "ebook", want: QualityEPUB},
		{name: "unknown text fallback", title: "Project Hail Mary retail", profile: ebookProfile, format: "ebook", want: QualityUnknownText},
		{name: "m4b token", title: "Project Hail Mary M4B unabridged", profile: audiobookProfile, format: "audiobook", want: QualityM4B},
		{name: "mp3 token", title: "Project Hail Mary MP3 64k", profile: audiobookProfile, format: "audiobook", want: QualityMP3},
		{name: "flac token", title: "Project Hail Mary FLAC", profile: audiobookProfile, format: "audiobook", want: QualityFLAC},
		{name: "audio bundle picks most preferred", title: "Project Hail Mary MP3 FLAC", profile: audiobookProfile, format: "audiobook", want: QualityFLAC},
		{name: "unknown audio fallback", title: "Project Hail Mary audiobook", profile: audiobookProfile, format: "audiobook", want: QualityUnknownAudio},
		{name: "cross-format token detected", title: "Project Hail Mary MP3", profile: ebookProfile, format: "ebook", want: QualityMP3},
	}
	for _, test := range tests {
		release := acquisition.Release{Title: test.title}
		if test.category != "" {
			release.Categories = []string{test.category}
		}
		if got := detectReleaseQuality(release, test.profile, test.format); got != test.want {
			t.Fatalf("%s: expected %q, got %q", test.name, test.want, got)
		}
	}
}

func TestCompositeReleaseScoreFormula(t *testing.T) {
	profile := defaultQualityProfile("standard", "ebook")
	// (ladderLen - ladderIndex) * 1000 + clamp(preferred, 0, 999)
	if got := compositeReleaseScore(profile, QualityAZW3, 0); got != 5000 {
		t.Fatalf("expected azw3 composite 5000, got %0.1f", got)
	}
	if got := compositeReleaseScore(profile, QualityEPUB, 42); got != 4042 {
		t.Fatalf("expected epub composite 4042, got %0.1f", got)
	}
	if got := compositeReleaseScore(profile, QualityUnknownText, 0); got != 1000 {
		t.Fatalf("expected unknownText composite 1000, got %0.1f", got)
	}
	if got := compositeReleaseScore(profile, QualityFLAC, 10); got != 10 {
		t.Fatalf("expected off-ladder quality to score preferred component only, got %0.1f", got)
	}
	if got := compositeReleaseScore(profile, QualityEPUB, 5000); got != 4999 {
		t.Fatalf("expected preferred component clamped below the next slot, got %0.1f", got)
	}
	if got := compositeReleaseScore(profile, QualityEPUB, -25); got != 4000 {
		t.Fatalf("expected negative preferred sum clamped to 0, got %0.1f", got)
	}
}

func TestNormalizeQualityLadderAndCutoff(t *testing.T) {
	ladder := normalizeQualityLadder([]QualityLevel{
		{ID: "EPUB", Allowed: true},
		{ID: "epub", Allowed: false},
		{ID: "unknowntext", Allowed: true},
		{ID: "not-a-quality", Allowed: true},
	}, "ebook")
	if len(ladder) != 2 || ladder[0].ID != QualityEPUB || ladder[1].ID != QualityUnknownText {
		t.Fatalf("unexpected normalized ladder: %+v", ladder)
	}
	if !ladder[0].Allowed {
		t.Fatal("expected first occurrence of a duplicate to win")
	}

	if got := normalizeCutoffQuality("EPUB", ladder, "ebook"); got != QualityEPUB {
		t.Fatalf("expected canonical cutoff epub, got %q", got)
	}
	if got := normalizeCutoffQuality("m4b", ladder, "ebook"); got != QualityEPUB {
		t.Fatalf("expected off-ladder cutoff to fall back to a ladder member, got %q", got)
	}

	empty := normalizeQualityLadder(nil, "audiobook")
	if len(empty) != 4 || empty[0].ID != QualityFLAC || empty[3].ID != QualityUnknownAudio {
		t.Fatalf("expected default audiobook ladder, got %+v", empty)
	}
	if got := normalizeCutoffQuality("", empty, "audiobook"); got != QualityM4B {
		t.Fatalf("expected default audiobook cutoff m4b, got %q", got)
	}
}

func TestDefaultQualityProfileIsMigrationShaped(t *testing.T) {
	ebook := defaultQualityProfile("standard", "ebook")
	wantEbook := []string{QualityAZW3, QualityEPUB, QualityMOBI, QualityPDF, QualityUnknownText}
	if len(ebook.Qualities) != len(wantEbook) {
		t.Fatalf("unexpected ebook ladder: %+v", ebook.Qualities)
	}
	for i, id := range wantEbook {
		if ebook.Qualities[i].ID != id || !ebook.Qualities[i].Allowed {
			t.Fatalf("unexpected ebook ladder entry %d: %+v", i, ebook.Qualities[i])
		}
	}
	if ebook.Cutoff != QualityEPUB || !ebook.UpgradeAllowed {
		t.Fatalf("unexpected ebook cutoff/upgrade: %+v", ebook)
	}

	audiobook := defaultQualityProfile("standard", "audiobook")
	wantAudio := []string{QualityFLAC, QualityM4B, QualityMP3, QualityUnknownAudio}
	for i, id := range wantAudio {
		if audiobook.Qualities[i].ID != id || !audiobook.Qualities[i].Allowed {
			t.Fatalf("unexpected audiobook ladder entry %d: %+v", i, audiobook.Qualities[i])
		}
	}
	if audiobook.Cutoff != QualityM4B {
		t.Fatalf("unexpected audiobook cutoff: %q", audiobook.Cutoff)
	}
}

func TestNormalizeQualityDefinitionsRejectsUnknownQuality(t *testing.T) {
	if _, unknown := normalizeQualityDefinitions([]QualityDefinition{{Quality: "cassette"}}); unknown != "cassette" {
		t.Fatalf("expected unknown quality to be reported, got %q", unknown)
	}
	normalized, unknown := normalizeQualityDefinitions([]QualityDefinition{
		{Quality: "EPUB", Title: " EPUB ", MinSizeMB: -5, MaxSizeMB: 400},
		{Quality: "epub", MaxSizeMB: 999},
	})
	if unknown != "" {
		t.Fatalf("unexpected unknown quality %q", unknown)
	}
	if len(normalized) != 1 || normalized[0].Quality != QualityEPUB || normalized[0].MinSizeMB != 0 || normalized[0].MaxSizeMB != 400 || normalized[0].Title != "EPUB" {
		t.Fatalf("unexpected normalized definitions: %+v", normalized)
	}
}

func TestNormalizeReleaseProfileDedupesTerms(t *testing.T) {
	profile := normalizeReleaseProfile(ReleaseProfile{
		Name:     "  terms ",
		Required: []string{"Retail", "retail", ""},
		Ignored:  []string{"screener", "Screener"},
		Preferred: []PreferredTerm{
			{Term: "epub", Score: 10},
			{Term: "EPUB", Score: 5},
			{Term: " ", Score: 3},
		},
	})
	if profile.Name != "terms" {
		t.Fatalf("expected trimmed name, got %q", profile.Name)
	}
	if len(profile.Required) != 1 || profile.Required[0] != "Retail" {
		t.Fatalf("unexpected required terms: %+v", profile.Required)
	}
	if len(profile.Ignored) != 1 {
		t.Fatalf("unexpected ignored terms: %+v", profile.Ignored)
	}
	if len(profile.Preferred) != 1 || profile.Preferred[0].Score != 10 {
		t.Fatalf("unexpected preferred terms: %+v", profile.Preferred)
	}
}
