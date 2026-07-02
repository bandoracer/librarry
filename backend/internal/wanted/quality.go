package wanted

import (
	"math"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

// Known quality IDs, Readarr-style. Ladders order these most-preferred first.
const (
	QualityAZW3         = "azw3"
	QualityEPUB         = "epub"
	QualityMOBI         = "mobi"
	QualityPDF          = "pdf"
	QualityUnknownText  = "unknownText"
	QualityFLAC         = "flac"
	QualityM4B          = "m4b"
	QualityMP3          = "mp3"
	QualityUnknownAudio = "unknownAudio"
)

// QualityLevel is one rung of a profile's quality ladder.
type QualityLevel struct {
	ID      string `json:"id"`
	Allowed bool   `json:"allowed"`
}

// ReleaseProfile carries required/ignored/preferred release terms, Readarr
// release-profile style. Required and ignored terms reject releases; matched
// preferred terms add their score to the composite release score.
type ReleaseProfile struct {
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name"`
	Enabled   bool            `json:"enabled"`
	Required  []string        `json:"required"`
	Ignored   []string        `json:"ignored"`
	Preferred []PreferredTerm `json:"preferred"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type PreferredTerm struct {
	Term  string `json:"term"`
	Score int    `json:"score"`
}

// QualityDefinition sets the acceptable size window for one quality.
// Zero means "no bound".
type QualityDefinition struct {
	Quality   string `json:"quality"`
	Title     string `json:"title"`
	MinSizeMB int    `json:"minSizeMB"`
	MaxSizeMB int    `json:"maxSizeMB"`
}

// defaultRestrictionPreferredScore is the score a matched preferred term from
// a legacy compat restriction contributes (mirrors the old profile
// preferred_score default of 8).
const defaultRestrictionPreferredScore = 8

// preferredScoreSlot is the per-ladder-rung width of the composite score.
const preferredScoreSlot = 1000

func knownEbookQualities() []string {
	return []string{QualityAZW3, QualityEPUB, QualityMOBI, QualityPDF, QualityUnknownText}
}

func knownAudiobookQualities() []string {
	return []string{QualityFLAC, QualityM4B, QualityMP3, QualityUnknownAudio}
}

// KnownQualities lists the valid quality IDs for a media format, ladder order.
func KnownQualities(format string) []string {
	if normalizeFormat(format) == "audiobook" {
		return knownAudiobookQualities()
	}
	return knownEbookQualities()
}

func defaultQualityLadder(format string) []QualityLevel {
	ids := KnownQualities(format)
	ladder := make([]QualityLevel, 0, len(ids))
	for _, id := range ids {
		ladder = append(ladder, QualityLevel{ID: id, Allowed: true})
	}
	return ladder
}

func defaultCutoffQuality(format string) string {
	if normalizeFormat(format) == "audiobook" {
		return QualityM4B
	}
	return QualityEPUB
}

// DefaultQualityDefinitions mirrors the migration-seeded size windows: text
// formats top out around 500MB, audio formats larger.
func DefaultQualityDefinitions() []QualityDefinition {
	return []QualityDefinition{
		{Quality: QualityAZW3, Title: "AZW3", MaxSizeMB: 500},
		{Quality: QualityEPUB, Title: "EPUB", MaxSizeMB: 500},
		{Quality: QualityMOBI, Title: "MOBI", MaxSizeMB: 500},
		{Quality: QualityPDF, Title: "PDF", MaxSizeMB: 500},
		{Quality: QualityUnknownText, Title: "Unknown Text", MaxSizeMB: 500},
		{Quality: QualityFLAC, Title: "FLAC", MaxSizeMB: 20480},
		{Quality: QualityM4B, Title: "M4B", MaxSizeMB: 10240},
		{Quality: QualityMP3, Title: "MP3", MaxSizeMB: 10240},
		{Quality: QualityUnknownAudio, Title: "Unknown Audio", MaxSizeMB: 20480},
	}
}

// canonicalQualityID maps case-insensitive input to a known quality ID
// ("EPUB" -> "epub", "unknowntext" -> "unknownText"). Unknown input returns
// ok=false.
func canonicalQualityID(id string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(id))
	for _, known := range append(knownEbookQualities(), knownAudiobookQualities()...) {
		if normalized == strings.ToLower(known) {
			return known, true
		}
	}
	return "", false
}

// normalizeQualityLadder canonicalizes and dedupes ladder entries, dropping
// unknown quality IDs; an empty result falls back to the format default.
func normalizeQualityLadder(levels []QualityLevel, format string) []QualityLevel {
	seen := map[string]bool{}
	ladder := make([]QualityLevel, 0, len(levels))
	for _, level := range levels {
		id, ok := canonicalQualityID(level.ID)
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		ladder = append(ladder, QualityLevel{ID: id, Allowed: level.Allowed})
	}
	if len(ladder) == 0 {
		return defaultQualityLadder(format)
	}
	return ladder
}

// normalizeCutoffQuality clamps the cutoff to a ladder member: an invalid or
// missing cutoff falls back to the format default when present, then to the
// first allowed rung, then to the top of the ladder.
func normalizeCutoffQuality(cutoff string, ladder []QualityLevel, format string) string {
	if id, ok := canonicalQualityID(cutoff); ok {
		for _, level := range ladder {
			if level.ID == id {
				return id
			}
		}
	}
	fallback := defaultCutoffQuality(format)
	for _, level := range ladder {
		if level.ID == fallback {
			return fallback
		}
	}
	for _, level := range ladder {
		if level.Allowed {
			return level.ID
		}
	}
	if len(ladder) > 0 {
		return ladder[0].ID
	}
	return fallback
}

func qualityLadderIndex(profile QualityProfile, quality string) (int, bool) {
	for index, level := range profile.Qualities {
		if level.ID == quality {
			return index, true
		}
	}
	return 0, false
}

func qualityAllowed(profile QualityProfile, quality string) bool {
	for _, level := range profile.Qualities {
		if level.ID == quality {
			return level.Allowed
		}
	}
	return false
}

// releaseQualityTokens are matched (word-bounded) against normalized release
// title + category text. normalizeText splits punctuation, so file-extension
// fragments like "book.epub" surface as the "epub" token.
var releaseQualityTokens = []string{
	QualityFLAC, QualityM4B, QualityMP3,
	QualityAZW3, QualityEPUB, QualityMOBI, QualityPDF,
}

// detectReleaseQuality identifies the release quality from title/category
// tokens. When a release names several formats (bundles), the profile's most
// preferred ladder entry among the matches wins; with no known token the
// format's unknown bucket is used.
func detectReleaseQuality(release acquisition.Release, profile QualityProfile, format string) string {
	haystack := " " + normalizeText(release.Title+" "+strings.Join(release.Categories, " ")) + " "
	var matched []string
	for _, token := range releaseQualityTokens {
		if strings.Contains(haystack, " "+token+" ") {
			matched = append(matched, token)
		}
	}
	if len(matched) > 0 {
		best := ""
		bestIndex := -1
		for _, quality := range matched {
			index, ok := qualityLadderIndex(profile, quality)
			if !ok {
				continue
			}
			if bestIndex == -1 || index < bestIndex {
				best, bestIndex = quality, index
			}
		}
		if best != "" {
			return best
		}
		return matched[0]
	}
	if normalizeFormat(format) == "audiobook" {
		return QualityUnknownAudio
	}
	return QualityUnknownText
}

// compositeReleaseScore encodes ladder rank and preferred-word score into one
// sortable number:
//
//	score = (ladderLen - ladderIndex) * 1000 + clamp(preferredScore, 0, 999)
//
// Each ladder rung owns a 1000-point slot (most-preferred first), so quality
// rank always dominates and the preferred-word sum only breaks ties inside a
// slot. Qualities absent from the ladder score rank 0 (preferred component
// only). wanted_items.current_release_score persists this same composite,
// which keeps history/sorting and the cutoff-unmet predicate ("quality below
// cutoff" <=> "score below CutoffCompositeScore") working as plain numeric
// comparisons.
func compositeReleaseScore(profile QualityProfile, quality string, preferredScore float64) float64 {
	rank := 0
	if index, ok := qualityLadderIndex(profile, quality); ok {
		rank = len(profile.Qualities) - index
	}
	return float64(rank)*preferredScoreSlot + clampPreferredScore(preferredScore)
}

func clampPreferredScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > preferredScoreSlot-1 {
		return preferredScoreSlot - 1
	}
	return score
}

// preferredScoreComponent extracts the sub-slot preferred-word component from
// a stored composite score.
func preferredScoreComponent(score float64) float64 {
	component := math.Mod(score, preferredScoreSlot)
	if component < 0 {
		return 0
	}
	return component
}

// CutoffCompositeScore is the composite score of a bare release at the
// profile's cutoff quality; stored scores at or above it mean the quality
// cutoff is met. Profiles without a ladder fall back to the format defaults.
func (p QualityProfile) CutoffCompositeScore() float64 {
	ladder := p.Qualities
	if len(ladder) == 0 {
		ladder = defaultQualityLadder(p.MediaFormat)
	}
	cutoff := p.Cutoff
	if _, ok := canonicalQualityID(cutoff); !ok {
		cutoff = defaultCutoffQuality(p.MediaFormat)
	}
	for index, level := range ladder {
		if level.ID == cutoff {
			return float64(len(ladder)-index) * preferredScoreSlot
		}
	}
	return float64(len(ladder)) * preferredScoreSlot
}

func definitionForQuality(definitions []QualityDefinition, quality string) (QualityDefinition, bool) {
	for _, definition := range definitions {
		if definition.Quality == quality {
			return definition, true
		}
	}
	return QualityDefinition{}, false
}

// normalizeReleaseProfile trims and dedupes a release profile's terms.
func normalizeReleaseProfile(profile ReleaseProfile) ReleaseProfile {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Required = ParseReleaseRestrictionTerms(strings.Join(profile.Required, ","))
	profile.Ignored = ParseReleaseRestrictionTerms(strings.Join(profile.Ignored, ","))
	seen := map[string]bool{}
	preferred := make([]PreferredTerm, 0, len(profile.Preferred))
	for _, term := range profile.Preferred {
		term.Term = strings.TrimSpace(term.Term)
		key := normalizeText(term.Term)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		preferred = append(preferred, term)
	}
	profile.Preferred = preferred
	return profile
}

// normalizeQualityDefinitions canonicalizes quality IDs and clamps sizes;
// unknown qualities are reported so PUT payloads fail loudly.
func normalizeQualityDefinitions(definitions []QualityDefinition) ([]QualityDefinition, string) {
	normalized := make([]QualityDefinition, 0, len(definitions))
	seen := map[string]bool{}
	for _, definition := range definitions {
		id, ok := canonicalQualityID(definition.Quality)
		if !ok {
			return nil, definition.Quality
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		definition.Quality = id
		definition.Title = strings.TrimSpace(definition.Title)
		if definition.MinSizeMB < 0 {
			definition.MinSizeMB = 0
		}
		if definition.MaxSizeMB < 0 {
			definition.MaxSizeMB = 0
		}
		normalized = append(normalized, definition)
	}
	return normalized, ""
}

// qualityDefinitionSortIndex orders definitions ebook ladder first, then
// audiobook ladder, matching the constant declarations.
func qualityDefinitionSortIndex(quality string) int {
	all := append(knownEbookQualities(), knownAudiobookQualities()...)
	for index, id := range all {
		if id == quality {
			return index
		}
	}
	return len(all)
}
