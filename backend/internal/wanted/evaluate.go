package wanted

import (
	"math"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

type releasePolicy struct {
	format  string
	profile QualityProfile
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
	policy := releasePolicy{
		format:  normalizeFormat(item.Format),
		profile: profileWithReleaseRestrictions(normalizeProfile(profile, item), restrictions, item.Tags),
	}
	score := 20.0
	var reasons []string

	haystackNorm := normalizeText(release.Title + " " + strings.Join(release.Categories, " "))
	titleNorm := normalizeText(release.Title)
	wantedTitle := normalizeText(item.Title)
	authorName := normalizeText(item.AuthorName)
	overrides := wantedManualOverrideValues(item)
	isbnMatched := wantedISBNMatchesRelease(overrides["isbn"], release)

	if blocklistMatchesRelease(blocklist, release) {
		reasons = append(reasons, blocklistRejectionReason)
	}
	if release.DownloadURL == "" {
		reasons = append(reasons, "missing download URL")
	}
	if isbnMatched {
		score += 45
	}
	if wantedTitle != "" {
		if strings.Contains(titleNorm, wantedTitle) {
			score += 35
		} else {
			overlap := tokenOverlap(wantedTitle, titleNorm)
			score += overlap * 30
			if overlap < 0.45 && !isbnMatched {
				reasons = append(reasons, "weak title match")
			}
		}
	}
	if authorName != "" && strings.Contains(titleNorm, authorName) {
		score += 10
	}
	if languageReason := releaseLanguageRejection(overrides["language"], haystackNorm); languageReason != "" {
		reasons = append(reasons, languageReason)
	} else if strings.TrimSpace(overrides["language"]) != "" {
		score += 8
	}
	if policy.format == "ebook" && !looksLikeEbook(release) {
		reasons = append(reasons, "does not look like an ebook release")
	}
	if policy.format == "audiobook" && !looksLikeAudiobook(release) {
		reasons = append(reasons, "does not look like an audiobook release")
	}
	if strings.EqualFold(release.Protocol, "torrent") && release.Seeders < policy.profile.MinSeeders {
		reasons = append(reasons, "below profile minimum seeders")
	} else if release.Seeders > 0 {
		score += math.Min(float64(release.Seeders), 50) * 0.5
	}

	maxSize := policy.profile.MaxSizeBytes
	if maxSize <= 0 {
		maxSize = maxSizeFor(policy)
	}
	if maxSize > 0 && release.SizeBytes > maxSize {
		reasons = append(reasons, "release exceeds profile size limit")
	} else if release.SizeBytes > 0 {
		score += 5
	}
	for _, term := range policy.profile.RequiredTerms {
		if !containsNormalizedTerm(haystackNorm, term) {
			reasons = append(reasons, "missing required term: "+term)
		}
	}
	for _, term := range policy.profile.RejectedTerms {
		if containsNormalizedTerm(haystackNorm, term) {
			reasons = append(reasons, "rejected term: "+term)
		}
	}
	preferredMatches := 0
	for _, term := range policy.profile.PreferredTerms {
		if containsNormalizedTerm(haystackNorm, term) {
			preferredMatches++
		}
	}
	if preferredMatches > 0 && policy.profile.PreferredScore > 0 {
		score += math.Min(float64(preferredMatches), 3) * policy.profile.PreferredScore
	}

	if len(reasons) > 0 && score >= policy.profile.MinScore {
		score = policy.profile.MinScore - 1
	}
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

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
		Approved:       len(reasons) == 0 && score >= policy.profile.MinScore,
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

func profileWithReleaseRestrictions(profile QualityProfile, restrictions []ReleaseRestriction, itemTags []int) QualityProfile {
	for _, restriction := range restrictions {
		if !releaseRestrictionApplies(restriction, itemTags) {
			continue
		}
		profile.RequiredTerms = appendUniqueTerms(profile.RequiredTerms, restriction.RequiredTerms...)
		profile.RejectedTerms = appendUniqueTerms(profile.RejectedTerms, restriction.IgnoredTerms...)
		profile.PreferredTerms = appendUniqueTerms(profile.PreferredTerms, restriction.PreferredTerms...)
	}
	return profile
}

func releaseRestrictionApplies(restriction ReleaseRestriction, itemTags []int) bool {
	if len(restriction.Tags) == 0 {
		return true
	}
	itemTags = compactRestrictionTags(itemTags)
	if len(itemTags) == 0 {
		return false
	}
	itemTagSet := make(map[int]bool, len(itemTags))
	for _, tag := range itemTags {
		itemTagSet[tag] = true
	}
	for _, tag := range restriction.Tags {
		if itemTagSet[tag] {
			return true
		}
	}
	return false
}

func appendUniqueTerms(base []string, additions ...string) []string {
	if len(additions) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base)+len(additions))
	for _, term := range base {
		if key := normalizeText(term); key != "" {
			seen[key] = true
		}
	}
	for _, term := range additions {
		term = strings.TrimSpace(term)
		key := normalizeText(term)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		base = append(base, term)
	}
	return base
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

func maxSizeFor(policy releasePolicy) int64 {
	if policy.format == "audiobook" {
		if normalizeQualityProfile(policy.profile.Name) == "large" {
			return 20 * 1024 * 1024 * 1024
		}
		return 8 * 1024 * 1024 * 1024
	}
	if normalizeQualityProfile(policy.profile.Name) == "large" {
		return 2 * 1024 * 1024 * 1024
	}
	return 750 * 1024 * 1024
}

func defaultQualityProfile(name string, format string) QualityProfile {
	name = normalizeQualityProfile(name)
	format = normalizeFormat(format)
	profile := QualityProfile{
		Name:           name,
		MediaFormat:    format,
		MinScore:       60,
		CutoffScore:    85,
		MinSeeders:     1,
		MaxSizeBytes:   750 * 1024 * 1024,
		PreferredTerms: []string{},
		RejectedTerms:  []string{"summary", "review"},
		PreferredScore: 8,
		UpgradeAllowed: true,
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
	if profile.MinScore <= 0 {
		profile.MinScore = 60
	}
	if profile.CutoffScore <= 0 {
		profile.CutoffScore = upgradeCutoffFor(item)
	}
	if profile.MinSeeders < 0 {
		profile.MinSeeders = 0
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
