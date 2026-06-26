package metadata

import (
	"context"
	"crypto/sha1"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)
var yearPattern = regexp.MustCompile(`\b(\d{4})\b`)

func scoreResult(query Query, title string, author string, isbns []string) float64 {
	needle := normalize(query.Query)
	titleNorm := normalize(title)
	authorNorm := normalize(author)
	isbnNeedle := normalizeISBN(query.Query)

	if isbnNeedle != "" {
		for _, isbn := range isbns {
			if normalizeISBN(isbn) == isbnNeedle {
				return 0.99
			}
		}
	}

	score := 0.20
	if titleNorm == needle {
		score += 0.55
	} else if strings.Contains(titleNorm, needle) || strings.Contains(needle, titleNorm) {
		score += 0.35
	} else {
		score += jaccard(titleNorm, needle) * 0.35
	}

	if authorNorm != "" && strings.Contains(needle, authorNorm) {
		score += 0.18
	}
	if query.Format == FormatEbook || query.Format == FormatAudiobook {
		score += 0.03
	}
	if score > 0.98 {
		score = 0.98
	}
	return score
}

func confidence(score float64) string {
	switch {
	case score >= 0.90:
		return "high"
	case score >= 0.70:
		return "medium"
	default:
		return "review"
	}
}

func matchedOn(query Query, title string, author string, isbns []string) []string {
	var matches []string
	isbnNeedle := normalizeISBN(query.Query)
	if isbnNeedle != "" {
		for _, isbn := range isbns {
			if normalizeISBN(isbn) == isbnNeedle {
				matches = append(matches, "isbn")
				break
			}
		}
	}
	if normalize(title) != "" && (strings.Contains(normalize(query.Query), normalize(title)) || strings.Contains(normalize(title), normalize(query.Query))) {
		matches = append(matches, "title")
	}
	if author != "" && strings.Contains(normalize(query.Query), normalize(author)) {
		matches = append(matches, "author")
	}
	if len(matches) == 0 {
		matches = append(matches, "fuzzy")
	}
	return matches
}

func normalize(value string) string {
	return strings.TrimSpace(nonWord.ReplaceAllString(strings.ToLower(value), " "))
}

func normalizeISBN(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
		if r == 'x' || r == 'X' {
			builder.WriteRune('X')
		}
	}
	normalized := builder.String()
	if len(normalized) == 10 || len(normalized) == 13 {
		return normalized
	}
	return ""
}

func jaccard(a string, b string) float64 {
	aParts := strings.Fields(a)
	bParts := strings.Fields(b)
	if len(aParts) == 0 || len(bParts) == 0 {
		return 0
	}
	seen := map[string]bool{}
	for _, part := range aParts {
		seen[part] = true
	}
	intersection := 0
	union := len(seen)
	for _, part := range bParts {
		if seen[part] {
			intersection++
			continue
		}
		union++
	}
	return float64(intersection) / float64(union)
}

func inferFormat(requested MediaFormat, isbns []string) MediaFormat {
	if requested == FormatEbook || requested == FormatAudiobook {
		return requested
	}
	return FormatAny
}

func openLibraryCoverURL(coverID int) string {
	if coverID <= 0 {
		return ""
	}
	return fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", coverID)
}

func openLibraryAuthorCoverURL(authorKey string) string {
	authorKey = strings.TrimSpace(strings.TrimPrefix(authorKey, "/authors/"))
	if authorKey == "" {
		return ""
	}
	return fmt.Sprintf("https://covers.openlibrary.org/a/olid/%s-M.jpg", authorKey)
}

func openLibraryAuthorKey(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "openlibrary:")
	value = strings.TrimPrefix(value, "/authors/")
	value = strings.TrimPrefix(value, "authors/")
	if strings.HasPrefix(value, "OL") && strings.HasSuffix(value, "A") {
		return value
	}
	return ""
}

func authorIdentityScore(query Query, name string) float64 {
	queryName := normalize(query.Query)
	authorName := normalize(name)
	switch {
	case queryName != "" && queryName == authorName:
		return 0.96
	case queryName != "" && authorName != "" && (strings.Contains(authorName, queryName) || strings.Contains(queryName, authorName)):
		return 0.86
	default:
		return scoreResult(query, name, "", nil)
	}
}

func authorWorkScore(query Query) float64 {
	if query.Format == FormatEbook || query.Format == FormatAudiobook {
		return 0.95
	}
	return 0.92
}

func authorMatchedOn(query Query, name string) []string {
	if normalize(query.Query) == normalize(name) {
		return []string{"author"}
	}
	return []string{"author_fuzzy"}
}

func yearFromOpenLibraryDate(value string) int {
	matches := yearPattern.FindStringSubmatch(value)
	if len(matches) < 2 {
		return 0
	}
	year, _ := strconv.Atoi(matches[1])
	return year
}

func firstInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func first(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func compactStrings(values []string) []string {
	var compacted []string
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		compacted = append(compacted, value)
	}
	return compacted
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 25 {
		return 25
	}
	return limit
}

func stableID(prefix string, value string) string {
	sum := sha1.Sum([]byte(normalize(value)))
	return fmt.Sprintf("%s:%x", prefix, sum[:8])
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if asString, ok := value.(string); ok {
		return asString
	}
	return fmt.Sprintf("%v", value)
}

func asContext(ctx Context) context.Context {
	if concrete, ok := ctx.(context.Context); ok {
		return concrete
	}
	return context.Background()
}
