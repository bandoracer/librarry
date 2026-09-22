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
		if firstAuthorName(result) == "" || discoveryCompanion(query, result) || discoveryContentKind(result) != "" || discoveryRequestsContent(query, "graphic") {
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
		kind := discoveryContentKind(*r)
		r.ContentLabel = discoveryContentLabel(kind)
		if kind == "" && focus != nil && discoveryPossibleGraphic(*focus, *r) {
			kind = "graphic"
			r.ContentLabel = "Possible graphic adaptation"
		}
		// A coherent digital edition with strong title evidence can precede an
		// unspecified source record. This does not boost bare exact-title stubs.
		r.discoveryPreferred = discoveryPreferredResult(query, *r)
		if lookup, ok := exactBookLookup(query); ok && lookup.isbn != "" && lookup.matches(*r) {
			continue // An explicit edition lookup always remains visible.
		}
		switch {
		case kind != "" && !discoveryRequestsContent(query, kind) && (focus != nil || query.Type == SearchTypeSeries):
			r.DiscoverySection = "related"
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
	for _, marker := range []string{"summary", "summaries", "study guide", "workbook", "companion", "coloring book", "colouring book", "boxed set", "box set", "journal", "tracker", "collection set", "complete series", "omnibus", "book guide"} {
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
	// Explicit content suffixes describe the requested form, not the title.
	for _, suffix := range []string{" graphic novel", " graphic novels", " graphic history", " comics", " comic", " manga", " abridged", " abridgment", " abridgement"} {
		wanted = strings.TrimSuffix(wanted, suffix)
	}
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
	if firstAuthorName(result) != "" && discoveryTitleFocus(query, result) && discoveryRequestsContent(query, "graphic") && (discoveryContentKind(result) == "graphic" || discoveryGraphicSubjects(result)) {
		return true
	}
	return firstAuthorName(result) != "" && result.Edition.ID != "" && concreteFormat(result.Edition.Format) != FormatAny && result.Edition.Language != "" && resultMatchesPreferredLanguage(result, query.PreferredLanguage) && discoveryTitleFocus(query, result)
}

// Content evidence changes presentation only, never provider or edition identity.
func discoveryContentKind(result SearchResult) string {
	if result.Work.ContentType == "graphic_novel" || discoveryHasPhrase(result.Work.Title+" "+result.Edition.Title, "graphic novel", "graphic history") {
		return "graphic"
	}
	// Word boundaries keep "unabridged" from matching "abridged". Do not infer
	// abridgment from page count, duration, or work-level edition summaries.
	if discoveryHasPhrase(result.Edition.EditionInformation+" "+result.Edition.Title, "abridged") {
		return "abridged"
	}
	return ""
}

func discoveryContentLabel(kind string) string {
	switch kind {
	case "graphic":
		return "Graphic novel"
	case "abridged":
		return "Abridged"
	}
	return ""
}

func discoveryRequestsContent(query Query, kind string) bool {
	if kind == "graphic" {
		return discoveryHasPhrase(query.Query, "graphic", "comic", "comics", "manga")
	}
	return discoveryHasPhrase(query.Query, "abridged", "abridgment", "abridgement")
}

func discoveryHasPhrase(value string, phrases ...string) bool {
	value = " " + normalize(value) + " "
	for _, phrase := range phrases {
		if strings.Contains(value, " "+phrase+" ") {
			return true
		}
	}
	return false
}

func discoveryPossibleGraphic(focus, result SearchResult) bool {
	// Work subjects and genres aggregate editions: the original Hobbit and
	// Sapiens both have comic subjects. Require a separate work with a shared
	// original author AND an additional credited author before suggesting an
	// adaptation. Even this is a qualified hint, not an asserted format.
	if focus.Provider != "Hardcover" || (result.Provider != "Open Library" && result.Provider != "Hardcover") || !discoverySharesAuthor(focus, result) {
		return false
	}
	for _, left := range append([]string{focus.Work.ID}, focus.Work.ProviderIDs...) {
		for _, right := range append([]string{result.Work.ID}, result.Work.ProviderIDs...) {
			if left != "" && left == right {
				return false
			}
		}
	}
	if !discoveryGraphicSubjects(result) {
		return false
	}
	for _, author := range result.Work.Authors {
		known := false
		for _, original := range focus.Work.Authors {
			known = known || normalize(original.Name) == normalize(author.Name)
		}
		if !known && normalize(author.Name) != "" {
			return true
		}
	}
	return false
}

func discoveryGraphicSubjects(result SearchResult) bool {
	for _, subject := range result.Work.Subjects {
		if discoveryHasPhrase(subject, "graphic novels", "graphic novel", "comic books", "comics") {
			return true
		}
	}
	return false
}
