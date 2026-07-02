package wanted

import (
	"math"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

// releaseEvaluationOptions bundles everything a release decision depends on:
// the quality-ladder profile, release profiles (required/ignored/preferred
// terms), quality definitions (size windows), legacy compat restrictions, and
// the blocklist.
type releaseEvaluationOptions struct {
	profile         QualityProfile
	restrictions    []ReleaseRestriction
	releaseProfiles []ReleaseProfile
	definitions     []QualityDefinition
	blocklist       []BlocklistEntry
}

func evaluateRelease(item WantedItem, release acquisition.Release) ReleaseDecision {
	return evaluateReleaseWithProfile(item, release, defaultQualityProfile(item.QualityProfile, item.Format))
}

func evaluateReleaseWithProfile(item WantedItem, release acquisition.Release, profile QualityProfile) ReleaseDecision {
	return evaluateReleaseWithPolicy(item, release, profile, nil)
}

func evaluateReleaseWithPolicy(item WantedItem, release acquisition.Release, profile QualityProfile, restrictions []ReleaseRestriction) ReleaseDecision {
	return evaluateReleaseWithBlocklist(item, release, profile, restrictions, nil)
}

func evaluateReleaseWithBlocklist(item WantedItem, release acquisition.Release, profile QualityProfile, restrictions []ReleaseRestriction, blocklist []BlocklistEntry) ReleaseDecision {
	return evaluateReleaseWithOptions(item, release, releaseEvaluationOptions{
		profile:      profile,
		restrictions: restrictions,
		definitions:  DefaultQualityDefinitions(),
		blocklist:    blocklist,
	})
}

// evaluateReleaseWithOptions is the arr-model release decision: detect the
// release quality, reject anything outside the profile ladder, the quality
// size window, or the required/ignored terms, and score survivors by ladder
// rank first and summed preferred-word score second (see
// compositeReleaseScore). Every rejection carries an explainable reason.
func evaluateReleaseWithOptions(item WantedItem, release acquisition.Release, options releaseEvaluationOptions) ReleaseDecision {
	format := normalizeFormat(item.Format)
	profile := normalizeProfile(options.profile, item)
	var reasons []string

	haystackNorm := normalizeText(release.Title + " " + strings.Join(release.Categories, " "))
	titleNorm := normalizeText(release.Title)
	wantedTitle := normalizeText(item.Title)
	overrides := wantedManualOverrideValues(item)
	isbnMatched := wantedISBNMatchesRelease(overrides["isbn"], release)

	if blocklistMatchesRelease(options.blocklist, release) {
		reasons = append(reasons, blocklistRejectionReason)
	}
	if release.DownloadURL == "" {
		reasons = append(reasons, "missing download URL")
	}
	if wantedTitle != "" && !strings.Contains(titleNorm, wantedTitle) {
		if tokenOverlap(wantedTitle, titleNorm) < 0.45 && !isbnMatched {
			reasons = append(reasons, "weak title match")
		}
	}
	if languageReason := releaseLanguageRejection(overrides["language"], haystackNorm); languageReason != "" {
		reasons = append(reasons, languageReason)
	}
	if format == "ebook" && !looksLikeEbook(release) {
		reasons = append(reasons, "does not look like an ebook release")
	}
	if format == "audiobook" && !looksLikeAudiobook(release) {
		reasons = append(reasons, "does not look like an audiobook release")
	}
	if strings.EqualFold(release.Protocol, "torrent") && release.Seeders < profile.MinSeeders {
		reasons = append(reasons, "below profile minimum seeders")
	}

	quality := detectReleaseQuality(release, profile, format)
	if !qualityAllowed(profile, quality) {
		reasons = append(reasons, "quality not allowed: "+quality)
	}
	if definition, ok := definitionForQuality(options.definitions, quality); ok && release.SizeBytes > 0 {
		if definition.MinSizeMB > 0 && release.SizeBytes < int64(definition.MinSizeMB)*1024*1024 {
			reasons = append(reasons, "below minimum size for "+quality)
		}
		if definition.MaxSizeMB > 0 && release.SizeBytes > int64(definition.MaxSizeMB)*1024*1024 {
			reasons = append(reasons, "above maximum size for "+quality)
		}
	}

	preferredScore := 0.0
	seenReasons := map[string]bool{}
	appendTermReason := func(reason string) {
		if seenReasons[reason] {
			return
		}
		seenReasons[reason] = true
		reasons = append(reasons, reason)
	}
	for _, releaseProfile := range options.releaseProfiles {
		if !releaseProfile.Enabled {
			continue
		}
		for _, term := range releaseProfile.Required {
			if !containsNormalizedTerm(haystackNorm, term) {
				appendTermReason("missing required term: " + term)
			}
		}
		for _, term := range releaseProfile.Ignored {
			if containsNormalizedTerm(haystackNorm, term) {
				appendTermReason("ignored term: " + term)
			}
		}
		for _, preferred := range releaseProfile.Preferred {
			if containsNormalizedTerm(haystackNorm, preferred.Term) {
				preferredScore += float64(preferred.Score)
			}
		}
	}
	// Legacy compat restrictions keep working alongside release profiles.
	for _, restriction := range options.restrictions {
		if !releaseRestrictionAppliesToLabels(restriction, item.Tags) {
			continue
		}
		for _, term := range restriction.RequiredTerms {
			if !containsNormalizedTerm(haystackNorm, term) {
				appendTermReason("missing required term: " + term)
			}
		}
		for _, term := range restriction.IgnoredTerms {
			if containsNormalizedTerm(haystackNorm, term) {
				appendTermReason("ignored term: " + term)
			}
		}
		for _, term := range restriction.PreferredTerms {
			if containsNormalizedTerm(haystackNorm, term) {
				preferredScore += defaultRestrictionPreferredScore
			}
		}
	}

	score := compositeReleaseScore(profile, quality, preferredScore)

	return ReleaseDecision{
		SourceID:       release.ID,
		InfoHash:       release.InfoHash,
		Indexer:        release.Indexer,
		Title:          release.Title,
		Protocol:       release.Protocol,
		DownloadURL:    release.DownloadURL,
		InfoURL:        release.InfoURL,
		SizeBytes:      release.SizeBytes,
		Seeders:        release.Seeders,
		Leechers:       release.Leechers,
		Categories:     release.Categories,
		Score:          math.Round(score*1000) / 1000,
		Approved:       len(reasons) == 0,
		RejectedReason: strings.Join(reasons, "; "),
		PublishedAt:    release.PublishedAt,
	}
}

func wantedISBNMatchesRelease(value string, release acquisition.Release) bool {
	identifierText := compactIdentifier(release.Title + " " + strings.Join(release.Categories, " "))
	if identifierText == "" {
		return false
	}
	for _, isbn := range splitWantedISBNs(value) {
		normalized := compactIdentifier(isbn)
		if len(normalized) >= 10 && strings.Contains(identifierText, normalized) {
			return true
		}
	}
	return false
}

func compactIdentifier(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' || r == 'x' || r == 'X' {
			builder.WriteRune(r)
		}
	}
	return strings.ToUpper(builder.String())
}

func releaseLanguageRejection(language string, haystack string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return ""
	}
	canonical := canonicalLanguageName(language)
	if canonical == "" {
		return ""
	}
	targetAliases := languageAliases(canonical)
	if containsAnyLanguageTerm(haystack, targetAliases) {
		return ""
	}
	if canonical == "english" {
		if found := firstExplicitLanguage(haystack, canonical); found != "" {
			return "language mismatch: " + found
		}
		return ""
	}
	if found := firstExplicitLanguage(haystack, canonical); found != "" {
		return "language mismatch: " + found
	}
	return "missing requested language: " + language
}

func canonicalLanguageName(language string) string {
	normalized := normalizeText(language)
	for canonical, aliases := range releaseLanguageAliases() {
		if normalized == canonical || containsString(aliases, normalized) {
			return canonical
		}
	}
	if normalized == "" {
		return ""
	}
	return normalized
}

func languageAliases(canonical string) []string {
	aliases := releaseLanguageAliases()[canonical]
	if len(aliases) == 0 {
		return []string{canonical}
	}
	return append([]string{canonical}, aliases...)
}

func firstExplicitLanguage(haystack string, exceptCanonical string) string {
	for canonical, aliases := range releaseLanguageAliases() {
		if canonical == exceptCanonical {
			continue
		}
		if containsAnyLanguageTerm(haystack, append([]string{canonical}, aliases...)) {
			return canonical
		}
	}
	return ""
}

func releaseLanguageAliases() map[string][]string {
	return map[string][]string{
		"english":    {"eng"},
		"german":     {"deutsch", "ger"},
		"french":     {"francais", "fre"},
		"spanish":    {"espanol", "spa"},
		"italian":    {"ita"},
		"dutch":      {"nederlands"},
		"japanese":   {"jpn"},
		"chinese":    {"mandarin"},
		"korean":     {"kor"},
		"portuguese": {"por"},
		"russian":    {"rus"},
		"polish":     {"pol"},
		"swedish":    {"swe"},
		"danish":     {"dan"},
		"norwegian":  {"nor"},
	}
}

func containsAnyNormalizedTerm(haystack string, terms []string) bool {
	for _, term := range terms {
		if containsNormalizedTerm(haystack, term) {
			return true
		}
	}
	return false
}

func containsAnyLanguageTerm(haystack string, terms []string) bool {
	for _, term := range terms {
		term = normalizeText(term)
		if term == "" {
			continue
		}
		if haystack == term || strings.Contains(" "+haystack+" ", " "+term+" ") {
			return true
		}
	}
	return false
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func NewReleaseRestriction(id string, required string, ignored string, preferred string, tags []int) ReleaseRestriction {
	return ReleaseRestriction{
		ID:             strings.TrimSpace(id),
		RequiredTerms:  ParseReleaseRestrictionTerms(required),
		IgnoredTerms:   ParseReleaseRestrictionTerms(ignored),
		PreferredTerms: ParseReleaseRestrictionTerms(preferred),
		Tags:           compactRestrictionTags(tags),
	}
}

func ParseReleaseRestrictionTerms(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '|'
	})
	terms := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		term := strings.TrimSpace(part)
		if term == "" {
			continue
		}
		key := normalizeText(term)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		terms = append(terms, term)
	}
	return terms
}

func compactRestrictionTags(tags []int) []int {
	if len(tags) == 0 {
		return nil
	}
	compact := make([]int, 0, len(tags))
	seen := map[int]bool{}
	for _, tag := range tags {
		if tag <= 0 || seen[tag] {
			continue
		}
		seen[tag] = true
		compact = append(compact, tag)
	}
	return compact
}

func normalizeFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "audiobook", "audio":
		return "audiobook"
	default:
		return "ebook"
	}
}

func normalizeQualityProfile(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "standard"
	}
	return value
}

func looksLikeEbook(release acquisition.Release) bool {
	haystack := normalizeText(release.Title + " " + strings.Join(release.Categories, " "))
	for _, token := range []string{"ebook", "epub", "mobi", "azw3", "pdf", "book"} {
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

func looksLikeAudiobook(release acquisition.Release) bool {
	haystack := normalizeText(release.Title + " " + strings.Join(release.Categories, " "))
	for _, token := range []string{"audiobook", "audio book", "m4b", "mp3", "audible", "narrated"} {
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

// defaultQualityProfile is the migration-shaped default: the format's full
// quality ladder (all allowed), the format's default cutoff, and legacy
// score-model fields kept only for compat echo.
func defaultQualityProfile(name string, format string) QualityProfile {
	name = normalizeQualityProfile(name)
	format = normalizeFormat(format)
	profile := QualityProfile{
		Name:           name,
		MediaFormat:    format,
		Qualities:      defaultQualityLadder(format),
		Cutoff:         defaultCutoffQuality(format),
		UpgradeAllowed: true,
		MinSeeders:     1,
		MinScore:       60,
		CutoffScore:    85,
		MaxSizeBytes:   750 * 1024 * 1024,
		PreferredTerms: []string{},
		RejectedTerms:  []string{"summary", "review"},
		PreferredScore: 8,
	}
	if format == "audiobook" {
		profile.MaxSizeBytes = 8 * 1024 * 1024 * 1024
	}
	if name == "large" || name == "best" || name == "preferred" {
		profile.MinScore = 65
		profile.CutoffScore = 90
		if format == "audiobook" {
			profile.MaxSizeBytes = 20 * 1024 * 1024 * 1024
		} else {
			profile.MaxSizeBytes = 2 * 1024 * 1024 * 1024
		}
	}
	if name == "strict" {
		profile.MinScore = 75
		profile.CutoffScore = 95
	}
	if format == "audiobook" {
		profile.PreferredTerms = []string{"m4b", "mp3"}
	} else {
		profile.PreferredTerms = []string{"epub", "azw3"}
	}
	return profile
}

func DefaultQualityProfiles() []QualityProfile {
	profiles := []QualityProfile{
		defaultQualityProfile("standard", "ebook"),
		defaultQualityProfile("standard", "audiobook"),
		defaultQualityProfile("large", "ebook"),
		defaultQualityProfile("large", "audiobook"),
	}
	for i := range profiles {
		profiles[i].ID = profiles[i].Name + "-" + profiles[i].MediaFormat
	}
	return profiles
}

func normalizeProfile(profile QualityProfile, item WantedItem) QualityProfile {
	if strings.TrimSpace(profile.Name) == "" {
		profile = defaultQualityProfile(item.QualityProfile, item.Format)
	}
	profile.Name = normalizeQualityProfile(profile.Name)
	if strings.TrimSpace(profile.MediaFormat) == "" || profile.MediaFormat == "any" {
		profile.MediaFormat = normalizeFormat(item.Format)
	}
	profile.Qualities = normalizeQualityLadder(profile.Qualities, profile.MediaFormat)
	profile.Cutoff = normalizeCutoffQuality(profile.Cutoff, profile.Qualities, profile.MediaFormat)
	if profile.MinSeeders < 0 {
		profile.MinSeeders = 0
	}
	// Legacy score-model defaults are kept only so compat readers see sane
	// numbers; they no longer drive evaluation.
	if profile.MinScore <= 0 {
		profile.MinScore = 60
	}
	if profile.CutoffScore <= 0 {
		profile.CutoffScore = 85
	}
	if profile.PreferredScore <= 0 {
		profile.PreferredScore = 8
	}
	if len(profile.RejectedTerms) == 0 {
		profile.RejectedTerms = []string{"summary", "review"}
	}
	return profile
}

func containsNormalizedTerm(haystack string, term string) bool {
	term = normalizeText(term)
	if term == "" {
		return false
	}
	return strings.Contains(haystack, term)
}

func normalizeText(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	lastSpace := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func tokenOverlap(a string, b string) float64 {
	aParts := strings.Fields(a)
	if len(aParts) == 0 {
		return 0
	}
	bSet := map[string]bool{}
	for _, part := range strings.Fields(b) {
		bSet[part] = true
	}
	matches := 0
	for _, part := range aParts {
		if bSet[part] {
			matches++
		}
	}
	return float64(matches) / float64(len(aParts))
}
