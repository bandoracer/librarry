package metadata

import (
	"errors"
	"strings"
)

// Discovery ordering is not an acquisition score. Do not promote an empty
// exact-title record above a provider's relevant novels or series matches.
func discoveryTier(query Query, result SearchResult) int {
	if lookup, ok := exactBookLookup(query); ok && lookup.matches(result) {
		if lookup.isbn != "" {
			return 3
		}
		// Google is admitted only after an exact suitable primary lookup failed.
		if result.Provider == "Google Books" {
			return 2
		}
	}
	if discoveryAuthorAgreement(query, result) {
		return 2
	}
	if _, _, explicit := explicitTitleAuthor(query.Query); explicit && query.Type == SearchTypeBook {
		return 0
	}

	return 1
}

func explicitTitleAuthor(value string) (string, string, bool) {
	value = strings.ToLower(value)
	index := strings.LastIndex(value, " by ")
	if index <= 0 {
		return "", "", false
	}
	title, author := strings.TrimSpace(value[:index]), strings.TrimSpace(value[index+4:])
	// Avoid interpreting e.g. "Death by Chocolate" as a constrained author.
	return title, author, len(strings.Fields(author)) >= 2
}

func discoveryEligible(query Query, result SearchResult) bool {
	lookup, ok := exactBookLookup(query)
	return (ok && lookup.isbn != "" && lookup.matches(result)) || resultFitsQuery(query, result)
}

func explainDiscovery(query Query, result SearchResult) SearchResult {
	result.Evidence = nil
	result.Conflicts = nil
	lookup, ok := exactBookLookup(query)
	if ok && lookup.isbn != "" && lookup.matches(result) {
		result.Evidence = append(result.Evidence, "Exact ISBN")
	} else if discoveryAuthorAgreement(query, result) {
		result.Evidence = append(result.Evidence, "Title and author match")
	} else if result.Provider == "Google Books" && ok && lookup.matches(result) {
		result.Evidence = append(result.Evidence, "Exact title fallback")
	} else if result.Kind == SearchTypeAuthor && normalize(query.Query) == normalize(firstAuthorName(result)) {
		result.Evidence = append(result.Evidence, "Author name match")
	} else {
		result.Evidence = append(result.Evidence, "Provider relevance")
	}
	if query.Type == SearchTypeSeries && result.Work.SeriesID != "" {
		result.Evidence = []string{"Hardcover series membership"}
		if result.Work.SeriesPosition != "" {
			result.Evidence = append(result.Evidence, "Series position "+result.Work.SeriesPosition)
		} else {
			result.Evidence = append(result.Evidence, "Series position unknown")
		}
	}
	if result.Kind == SearchTypeAuthor {
		return result
	}
	if language := normalizeLanguageName(result.Edition.Language); language != "" {
		result.Evidence = append(result.Evidence, language+" edition")
		if !resultMatchesPreferredLanguage(result, query.PreferredLanguage) {
			result.Conflicts = append(result.Conflicts, "Edition language differs from your preference")
		}
	} else {
		result.Evidence = append(result.Evidence, "Edition language unknown")
	}
	if concreteFormat(result.Edition.Format) != FormatAny {
		result.Evidence = append(result.Evidence, string(result.Edition.Format)+" verified")
		if requested := concreteFormat(query.Format); requested != FormatAny && requested != result.Edition.Format {
			result.Conflicts = append(result.Conflicts, "Edition format differs from your preference")
		}
	} else {
		result.Evidence = append(result.Evidence, "Edition format unknown")
	}
	if len(result.Work.Authors) == 0 || firstAuthorName(result) == "" {
		result.Evidence = append(result.Evidence, "Author unknown")
	}
	if result.Edition.CoverURL == "" && result.Work.CoverURL == "" {
		result.Evidence = append(result.Evidence, "Cover unavailable")
	}
	if title, author, explicit := explicitTitleAuthor(query.Query); explicit && discoveryTier(query, result) != 2 && title != "" {
		known := false
		matches := false
		for _, candidate := range result.Work.Authors {
			known = known || candidate.Name != ""
			matches = matches || normalize(candidate.Name) == normalize(author)
		}
		if known && !matches && (normalize(title) == normalize(result.Work.Title) || normalize(title) == normalize(result.Edition.Title)) {
			result.Conflicts = append(result.Conflicts, "Author differs from the requested author")
		}
	}
	if len(result.Conflicts) > 0 {
		result.Confidence = "review"
	}
	return result
}

// ValidateSearchQuery prevents a malformed explicit ISBN from turning into a
// general title query. Other query types may legitimately contain numbers.
func ValidateSearchQuery(query Query) error {
	if query.Type != "" && query.Type != SearchTypeBook {
		return nil
	}
	if lookup, ok := exactBookLookup(query); ok && lookup.isbn != "" {
		return nil
	}
	raw := strings.ToLower(strings.TrimSpace(query.Query))
	for _, prefix := range []string{"isbn:", "isbn ", "isbn-10:", "isbn-13:"} {
		if strings.HasPrefix(raw, prefix) {
			return errors.New("Enter a valid ISBN-10 or ISBN-13")
		}
	}
	if len(isbnCharacters(raw)) >= 10 {
		return errors.New("Enter a valid ISBN-10 or ISBN-13")
	}
	return nil
}

func discoveryTitleMatches(title string, result SearchResult) bool {
	n := normalize(title)
	return n != "" && (n == normalize(result.Work.Title) || n == normalize(result.Edition.Title))
}

func discoveryAuthorAgreement(query Query, result SearchResult) bool {
	if query.Type != SearchTypeBook {
		return false
	}
	if title, author, explicit := explicitTitleAuthor(query.Query); explicit {
		for _, candidate := range result.Work.Authors {
			if normalize(candidate.Name) == normalize(author) && discoveryTitleMatches(title, result) {
				return true
			}
		}
		return false
	}
	// Unstructured "title author" queries are recognized against returned author
	// evidence. Do not guess an author's identity from arbitrary last words.
	n := normalize(query.Query)
	for _, author := range result.Work.Authors {
		a := normalize(author.Name)
		if CanonicalAuthorKey(author.ID) == "" || len(strings.Fields(a)) < 2 {
			continue
		}
		if title, ok := strings.CutSuffix(n, " "+a); ok && discoveryTitleMatches(title, result) {
			return true
		}
	}
	return false
}
