package library

import (
	"regexp"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

// Permit a conventional surname-first spelling, not arbitrary token overlap or
// reordered coauthor lists. Different given names remain different authors.
func importAuthorMatches(actual, expected string) bool {
	normalize := func(value string) string {
		parts := strings.Split(value, ",")
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" && !strings.ContainsAny(value, ";&") {
			value = parts[1] + " " + parts[0]
		}
		return normalizeImportReviewMatchText(value)
	}
	return normalize(actual) == normalize(expected)
}

var importSeriesPrefix = regexp.MustCompile(`(?i)^(.*?)(?:\s+(?:book\s*|#)?(\d+(?:\.\d+)?))?$`)
var importTitleSeparator = regexp.MustCompile(`\s*[:–—]\s*|\s+-\s+`)

// A series prefix is decoration only when the remainder is the exact wanted
// title and the prefix identifies its recorded series. ISBN/format checks remain
// independent, so this cannot approve an adaptation, summary or different volume.
func importTitleMatches(actual string, item wanted.WantedItem) bool {
	expected := normalizeImportReviewMatchText(item.Title)
	if normalizeImportReviewMatchText(actual) == expected {
		return true
	}
	if expected == "" || strings.TrimSpace(item.Series) == "" {
		return false
	}
	parts := importTitleSeparator.Split(actual, -1)
	if len(parts) != 2 || normalizeImportReviewMatchText(parts[1]) != expected {
		return false
	}
	prefix := importSeriesPrefix.FindStringSubmatch(strings.TrimSpace(parts[0]))
	if len(prefix) != 3 {
		return false
	}
	series := normalizeImportReviewMatchText(item.Series)
	name := normalizeImportReviewMatchText(prefix[1])
	// Short series names such as "Percy Jackson" must contain at least two words
	// and be a whole-word prefix of the recorded series, not a substring.
	if name != series && !(len(strings.Fields(name)) >= 2 && len(name) >= 10 && strings.HasPrefix(series, name+" ")) {
		return false
	}
	if prefix[2] != "" && (item.SeriesPosition == "" || strings.TrimSpace(item.SeriesPosition) != prefix[2]) {
		return false
	}
	return true
}
