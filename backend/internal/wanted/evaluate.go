package wanted

import (
	"math"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

type releasePolicy struct {
	format         string
	qualityProfile string
}

func evaluateRelease(item WantedItem, release acquisition.Release) ReleaseDecision {
	policy := releasePolicy{
		format:         normalizeFormat(item.Format),
		qualityProfile: normalizeQualityProfile(item.QualityProfile),
	}
	score := 20.0
	var reasons []string

	titleNorm := normalizeText(release.Title)
	wantedTitle := normalizeText(item.Title)
	authorName := normalizeText(item.AuthorName)

	if release.DownloadURL == "" {
		reasons = append(reasons, "missing download URL")
	}
	if wantedTitle != "" {
		if strings.Contains(titleNorm, wantedTitle) {
			score += 35
		} else {
			overlap := tokenOverlap(wantedTitle, titleNorm)
			score += overlap * 30
			if overlap < 0.45 {
				reasons = append(reasons, "weak title match")
			}
		}
	}
	if authorName != "" && strings.Contains(titleNorm, authorName) {
		score += 10
	}
	if policy.format == "ebook" && !looksLikeEbook(release) {
		reasons = append(reasons, "does not look like an ebook release")
	}
	if policy.format == "audiobook" && !looksLikeAudiobook(release) {
		reasons = append(reasons, "does not look like an audiobook release")
	}
	if strings.EqualFold(release.Protocol, "torrent") && release.Seeders <= 0 {
		reasons = append(reasons, "no seeders")
	} else if release.Seeders > 0 {
		score += math.Min(float64(release.Seeders), 50) * 0.5
	}

	maxSize := maxSizeFor(policy)
	if release.SizeBytes > maxSize {
		reasons = append(reasons, "release exceeds profile size limit")
	} else if release.SizeBytes > 0 {
		score += 5
	}
	if strings.Contains(titleNorm, "summary") || strings.Contains(titleNorm, "review") {
		reasons = append(reasons, "summary or review release")
	}

	if len(reasons) > 0 && score > 59 {
		score = 59
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
		Approved:       len(reasons) == 0 && score >= 60,
		RejectedReason: strings.Join(reasons, "; "),
		PublishedAt:    release.PublishedAt,
	}
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
		if policy.qualityProfile == "large" {
			return 20 * 1024 * 1024 * 1024
		}
		return 8 * 1024 * 1024 * 1024
	}
	if policy.qualityProfile == "large" {
		return 2 * 1024 * 1024 * 1024
	}
	return 750 * 1024 * 1024
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
