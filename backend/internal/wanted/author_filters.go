package wanted

import (
	"strings"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

// Author add-filter reasons (M6.5). They are stable API: the review queue and
// history surface them verbatim.
const (
	filterReasonLanguage    = "language-filtered"
	filterReasonTerm        = "term-filtered"
	filterReasonMissingISBN = "missing-isbn"
	filterReasonMinPages    = "below-min-pages"
)

// authorResultFilterReason applies the subscription's add-filters to a
// candidate BEFORE the missing-book policy runs. An empty reason means the
// candidate passes. Filters only reject on evidence the provider actually
// supplies (an unknown language or page count never rejects), except
// skip-missing-isbn, whose whole point is rejecting ISBN-less candidates.
func authorResultFilterReason(subscription AuthorSubscription, result metadata.SearchResult) string {
	if key := metadata.CanonicalAuthorKey(subscription.ProviderKey); strings.HasPrefix(key, "hardcover-author:") {
		authored := false
		for _, author := range result.Work.Authors {
			if metadata.CanonicalAuthorKey(author.ID) == key && (strings.EqualFold(author.Role, "Author") || strings.EqualFold(author.Role, "Writer")) {
				authored = true
			}
		}
		if !authored {
			return "Selected person has no verified writing credit; review this contribution before adding the book."
		}
	}
	if reason := languageFilterReason(subscription.AllowedLanguages, result.Edition.Language); reason != "" {
		return reason
	}
	if reason := termFilterReason(subscription.MustNotContain, result); reason != "" {
		return reason
	}
	if subscription.SkipMissingISBN && len(result.Edition.ISBNs) == 0 {
		return filterReasonMissingISBN
	}
	if subscription.MinPages > 0 && result.Edition.Pages > 0 && result.Edition.Pages < subscription.MinPages {
		return filterReasonMinPages
	}
	return ""
}

func languageFilterReason(allowed []string, language string) string {
	allowed = normalizeFilterTerms(allowed)
	if len(allowed) == 0 {
		return ""
	}
	language = metadata.LanguageName(language)
	if language == "" {
		return ""
	}
	for _, candidate := range allowed {
		candidate = metadata.LanguageName(candidate)
		if candidate == language || candidate == "any" || candidate == "all" {
			return ""
		}
	}
	return filterReasonLanguage
}

func termFilterReason(terms []string, result metadata.SearchResult) string {
	terms = normalizeFilterTerms(terms)
	if len(terms) == 0 {
		return ""
	}
	haystack := normalizeText(strings.Join([]string{result.Work.Title, result.Edition.Title}, " "))
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(haystack, normalizeText(term)) {
			return filterReasonTerm
		}
	}
	return ""
}

// normalizeFilterTerms trims, dedupes, and drops empty comma-list terms.
func normalizeFilterTerms(terms []string) []string {
	if len(terms) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.TrimSpace(term)
		key := strings.ToLower(term)
		if term == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, term)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// joinFilterTerms renders the stored comma-list text for a filter column.
func joinFilterTerms(terms []string) string {
	return strings.Join(normalizeFilterTerms(terms), ",")
}

// splitFilterTerms parses a stored comma-list text column.
func splitFilterTerms(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '|'
	})
	return normalizeFilterTerms(parts)
}
