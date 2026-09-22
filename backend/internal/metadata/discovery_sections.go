package metadata

import "strings"

// Sections describe discovery usefulness, never acquisition confidence or identity.
// Provider order remains the tiebreaker within a section. In particular, a bare
// exact title is not enough to promote an adaptation or an empty catalog stub.
func assignDiscoverySections(query Query, results []SearchResult) {
	if query.Type != SearchTypeBook && query.Type != SearchTypeSeries {
		return
	}
	terms := discoveryTerms(query.Query)
	anchors := []SearchResult{}
	var focus *SearchResult
	hasExactSeries := false
	for i, result := range results {
		hasExactSeries = hasExactSeries || query.Type == SearchTypeSeries && normalize(result.Work.Series) == normalize(query.Query)
		if firstAuthorName(result) == "" || discoveryCompanion(query, result) {
			continue
		}
		if discoveryPreferredResult(query, result) && (focus == nil || providerRank(result.Provider) < providerRank(focus.Provider) || result.Provider == focus.Provider && result.discoveryRank < focus.discoveryRank) {
			focus = &results[i]
		}
		// Require direct title evidence before using one provider to demote
		// unrelated hits from another. Topic/series-only searches keep provider
		// relevance; descriptions are deliberately not lexical evidence here.
		hits := discoveryTermHits(terms, result.Work.Title+" "+result.Edition.Title)
		if len(terms) > 0 && hits*3 >= len(terms)*2 {
			anchors = append(anchors, result)
		}
	}
	for i := range results {
		r := &results[i]
		r.DiscoverySection = ""
		// A coherent digital edition with strong title evidence can precede an
		// unspecified source record. This does not boost bare exact-title stubs.
		r.discoveryPreferred = discoveryPreferredResult(query, *r)
		if lookup, ok := exactBookLookup(query); ok && lookup.isbn != "" && lookup.matches(*r) {
			continue // An explicit edition lookup always remains visible.
		}
		switch {
		case discoveryCompanion(query, *r):
			r.DiscoverySection = "related"
		case firstAuthorName(*r) == "" || (query.Type == SearchTypeSeries && concreteFormat(r.Edition.Format) == FormatAny):
			r.DiscoverySection = "incomplete"
		case query.Type == SearchTypeSeries && (r.Work.SeriesPosition == "" || hasExactSeries && normalize(r.Work.Series) != normalize(query.Query)):
			r.DiscoverySection = "related"
		case query.Type == SearchTypeBook && focus != nil && !discoverySharesAuthor(*focus, *r):
			r.DiscoverySection = "related"
		case query.Type == SearchTypeBook && len(anchors) > 0 && discoveryTermHits(terms, r.Work.Title+" "+r.Edition.Title+" "+r.Work.Series+" "+firstAuthorName(*r)) == 0:
			// Shared authors keep e.g. The Lightning Thief alongside Percy
			// Jackson's Greek Gods. This comparison never merges their identities.
			sharesAuthor := false
			for _, anchor := range anchors {
				sharesAuthor = sharesAuthor || discoverySharesAuthor(anchor, *r)
			}
			if !sharesAuthor {
				r.DiscoverySection = "related"
			}
		}
	}
}

func discoverySectionRank(section string) int {
	switch section {
	case "related":
		return 1
	case "incomplete":
		return 2
	default:
		return 0
	}
}

func discoveryTerms(value string) []string {
	out := []string{}
	for _, term := range strings.Fields(normalize(value)) {
		switch term {
		case "a", "an", "the", "of", "and", "or", "to", "for", "by", "in", "with", "on", "from":
			continue
		}
		out = appendUniqueStrings(out, term)
	}
	return out
}

func discoveryTermHits(terms []string, value string) int {
	words := " " + normalize(value) + " "
	hits := 0
	for _, term := range terms {
		if strings.Contains(words, " "+term+" ") {
			hits++
		}
	}
	return hits
}

func discoveryCompanion(query Query, result SearchResult) bool {
	title := " " + normalize(result.Work.Title+" "+result.Edition.Title) + " "
	requested := " " + normalize(query.Query) + " "
	for _, marker := range []string{"summary", "summaries", "study guide", "workbook", "companion", "coloring book", "colouring book", "boxed set", "box set", "graphic novel", "graphic history", "journal", "tracker", "collection set", "complete series", "omnibus", "book guide"} {
		if strings.Contains(title, " "+marker+" ") && !strings.Contains(requested, " "+marker+" ") {
			return true
		}
	}
	return false
}

// Subtitle variants count as the same title focus; arbitrary keyword hits do
// not. "Educated" must not promote "... His Educated Rodents".
func discoveryTitleFocus(query Query, result SearchResult) bool {
	title := query.Query
	if explicit, _, ok := explicitTitleAuthor(title); ok {
		title = explicit
	}
	wanted := normalize(title)
	if wanted == "" {
		return false
	}
	for _, candidate := range []string{result.Work.Title, result.Edition.Title} {
		for _, separator := range []string{":", ",", " – ", " — "} {
			candidate = strings.SplitN(candidate, separator, 2)[0]
		}
		if normalize(candidate) == wanted {
			return true
		}
	}
	return false
}

func discoverySharesAuthor(left, right SearchResult) bool {
	for _, a := range left.Work.Authors {
		for _, b := range right.Work.Authors {
			if normalize(a.Name) != "" && normalize(a.Name) == normalize(b.Name) {
				return true
			}
		}
	}
	return false
}

func discoveryPreferredResult(query Query, result SearchResult) bool {
	if _, _, explicit := explicitTitleAuthor(query.Query); explicit && !discoveryAuthorAgreement(query, result) {
		return false
	}
	return firstAuthorName(result) != "" && result.Edition.ID != "" && concreteFormat(result.Edition.Format) != FormatAny && result.Edition.Language != "" && resultMatchesPreferredLanguage(result, query.PreferredLanguage) && discoveryTitleFocus(query, result)
}
