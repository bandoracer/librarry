package metadata

import (
	"math"
	"sort"
	"strings"
)

func mergeEquivalentResults(query Query, results []SearchResult) []SearchResult {
	if len(results) < 2 {
		return results
	}

	clusters := make([]searchResultCluster, 0, len(results))
	for _, result := range results {
		key := resultMergeKey(result)
		if key == "" {
			clusters = append(clusters, searchResultCluster{results: []SearchResult{result}})
			continue
		}

		merged := false
		for i := range clusters {
			compatible := true
			for _, existing := range clusters[i].results {
				compatible = compatible && resultsCanMerge(query, existing, result)
			}
			if !compatible {
				continue
			}
			clusters[i].results = append(clusters[i].results, result)
			merged = true
			break
		}
		if !merged {
			clusters = append(clusters, searchResultCluster{results: []SearchResult{result}})
		}
	}

	merged := make([]SearchResult, 0, len(clusters))
	for _, cluster := range clusters {
		merged = append(merged, mergeSearchResultCluster(cluster.results))
	}
	return merged
}

type searchResultCluster struct {
	results []SearchResult
}

func resultMergeKey(result SearchResult) string {
	if result.Kind == SearchTypeAuthor {
		name := normalize(firstNonEmpty(firstAuthorName(result), result.Work.Title))
		if name == "" {
			return ""
		}
		return "author:" + name
	}
	if result.Kind != SearchTypeBook && result.Kind != SearchTypeAuthorWorks {
		return ""
	}
	if isbn := firstNormalizedISBN(result.Edition.ISBNs); isbn != "" {
		return "isbn:" + isbn
	}
	title := normalize(firstNonEmpty(result.Work.Title, result.Edition.Title))
	author := normalize(firstAuthorName(result))
	if title == "" || author == "" {
		return ""
	}
	return "work:" + title + "|" + author
}

func resultsCanMerge(query Query, left SearchResult, right SearchResult) bool {
	if query.Type == SearchTypeSeries && left.Work.SeriesID != right.Work.SeriesID {
		return false
	}
	if left.Kind != right.Kind {
		return false
	}
	if left.Kind == SearchTypeAuthor {
		for _, a := range left.Work.Authors {
			for _, b := range right.Work.Authors {
				for _, leftID := range append([]string{a.ID}, a.ProviderIDs...) {
					key := CanonicalAuthorKey(leftID)
					if key == "" {
						continue
					}
					for _, rightID := range append([]string{b.ID}, b.ProviderIDs...) {
						if CanonicalAuthorKey(rightID) == key {
							return true
						}
					}
				}
			}
		}
		return false
	}
	if !formatsCompatible(query, left.Edition.Format, right.Edition.Format) {
		return false
	}
	if left.Edition.Language != "" && right.Edition.Language != "" && normalizeLanguageName(left.Edition.Language) != normalizeLanguageName(right.Edition.Language) {
		return false
	}
	if left.Provider == right.Provider && left.Edition.ID != "" && right.Edition.ID != "" && left.Edition.ID != right.Edition.ID {
		return false
	}
	if firstSharedNormalizedISBN(left.Edition.ISBNs, right.Edition.ISBNs) != "" {
		return true
	}
	if firstNormalizedISBN(left.Edition.ISBNs) != "" && firstNormalizedISBN(right.Edition.ISBNs) != "" {
		return false
	}
	// Discovery may only combine verified identities. Matching title/author text
	// can also describe adaptations, companions, or unrelated catalog records.
	if left.discoveryRank > 0 || right.discoveryRank > 0 {
		return left.Work.ID != "" && left.Work.ID == right.Work.ID && left.Edition.ID == right.Edition.ID
	}
	leftYear := resultYear(left)
	rightYear := resultYear(right)
	if leftYear > 0 && rightYear > 0 && math.Abs(float64(leftYear-rightYear)) > 1 {
		return false
	}
	return normalize(firstNonEmpty(left.Work.Title, left.Edition.Title)) == normalize(firstNonEmpty(right.Work.Title, right.Edition.Title)) &&
		normalize(firstAuthorName(left)) == normalize(firstAuthorName(right))
}

func formatsCompatible(query Query, left MediaFormat, right MediaFormat) bool {
	left = concreteFormat(left)
	right = concreteFormat(right)
	return left == FormatAny || right == FormatAny || left == right
}

func concreteFormat(format MediaFormat) MediaFormat {
	switch format {
	case FormatEbook, FormatAudiobook:
		return format
	default:
		return FormatAny
	}
}

func mergeSearchResultCluster(results []SearchResult) SearchResult {
	if len(results) == 0 {
		return SearchResult{}
	}
	if len(results) == 1 {
		return enrichResultProviderIDs(results[0])
	}

	base := enrichResultProviderIDs(results[0])
	for _, candidate := range results[1:] {
		candidate = enrichResultProviderIDs(candidate)
		if betterSearchResult(candidate, base) {
			base, candidate = candidate, base
		}
		base = mergeSearchResult(base, candidate)
	}

	providerCount := len(resultProviderNames(results))
	if providerCount > 1 {
		base.MatchedOn = appendUniqueStrings(base.MatchedOn, "provider corroboration")
		base.Score += float64(providerCount-1) * 0.03
		if base.Score > 0.99 {
			base.Score = 0.99
		}
		base.Confidence = confidence(base.Score)
	}
	return base
}

func betterSearchResult(candidate SearchResult, current SearchResult) bool {
	scoreDelta := candidate.Score - current.Score
	if scoreDelta > 0.05 {
		return true
	}
	if scoreDelta < -0.05 {
		return false
	}
	if providerRank(candidate.Provider) != providerRank(current.Provider) {
		return providerRank(candidate.Provider) < providerRank(current.Provider)
	}
	return resultRichness(candidate) > resultRichness(current)
}

func resultRichness(result SearchResult) int {
	score := 0
	if result.Work.CoverURL != "" {
		score++
	}
	if result.Work.Description != "" {
		score++
	}
	if result.Work.FirstPublishYear > 0 {
		score++
	}
	if result.Work.Series != "" {
		score++
	}
	if result.Work.SeriesPosition != "" {
		score++
	}
	score += len(result.Work.ProviderIDs)
	score += len(result.Edition.ISBNs)
	if result.Edition.ASIN != "" {
		score++
	}
	if result.Edition.Publisher != "" {
		score++
	}
	if result.Edition.PublishedDate != "" {
		score++
	}
	if result.Edition.Language != "" {
		score++
	}
	return score
}

func mergeSearchResult(base SearchResult, candidate SearchResult) SearchResult {
	if candidate.discoveryRank > 0 && (base.discoveryRank == 0 || candidate.discoveryRank < base.discoveryRank) {
		base.discoveryRank = candidate.discoveryRank
	}
	base.Work = mergeWork(base.Work, candidate.Work)
	base.Edition = mergeEdition(base.Edition, candidate.Edition)
	base.MatchedOn = appendUniqueStrings(base.MatchedOn, candidate.MatchedOn...)
	if base.RawSourceKey == "" {
		base.RawSourceKey = candidate.RawSourceKey
	}
	if candidate.Score > base.Score {
		base.Score = candidate.Score
		base.Confidence = candidate.Confidence
	}
	return base
}

func mergeWork(base Work, candidate Work) Work {
	base.Languages = appendUniqueStrings(base.Languages, candidate.Languages...)
	if base.Subtitle == "" {
		base.Subtitle = candidate.Subtitle
	}
	if base.FirstPublishDate == "" {
		base.FirstPublishDate = candidate.FirstPublishDate
	}
	if base.ID == "" {
		base.ID = candidate.ID
	}
	if base.Title == "" {
		base.Title = candidate.Title
	}
	if base.CoverURL == "" {
		base.CoverURL = candidate.CoverURL
	}
	if len(candidate.Description) > len(base.Description) {
		base.Description = candidate.Description
	}
	if base.FirstPublishYear == 0 || candidate.FirstPublishYear > 0 && candidate.FirstPublishYear < base.FirstPublishYear {
		base.FirstPublishYear = candidate.FirstPublishYear
	}
	if base.Series == "" {
		base.SeriesID, base.Series, base.SeriesPosition = candidate.SeriesID, candidate.Series, candidate.SeriesPosition
	} else if base.SeriesPosition == "" && base.SeriesID != "" && base.SeriesID == candidate.SeriesID {
		base.SeriesPosition = candidate.SeriesPosition
	}
	base.ProviderIDs = appendUniqueStrings(base.ProviderIDs, candidate.ProviderIDs...)
	if candidate.ID != "" {
		base.ProviderIDs = appendUniqueStrings(base.ProviderIDs, candidate.ID)
	}
	base.Authors = mergeAuthors(base.Authors, candidate.Authors)
	return base
}

func mergeAuthors(base []Author, candidates []Author) []Author {
	if len(base) == 0 {
		return enrichAuthorProviderIDs(candidates)
	}
	base = enrichAuthorProviderIDs(base)
	for _, candidate := range enrichAuthorProviderIDs(candidates) {
		if candidate.Name == "" && candidate.ID == "" {
			continue
		}
		merged := false
		for i := range base {
			if authorsCanMerge(base[i], candidate) {
				if base[i].ID == "" || CanonicalAuthorKey(base[i].ID) == "" && CanonicalAuthorKey(candidate.ID) != "" {
					base[i].ID = candidate.ID
				}
				if base[i].Role == "" || strings.EqualFold(base[i].Role, "unknown") {
					base[i].Role = candidate.Role
				}
				if base[i].Name == "" {
					base[i].Name = candidate.Name
				}
				base[i].ProviderIDs = appendUniqueStrings(base[i].ProviderIDs, candidate.ProviderIDs...)
				merged = true
				break
			}
		}
		if !merged {
			base = append(base, candidate)
		}
	}
	return base
}

// Preserve distinct known identities and contributor roles even when names match.
func authorsCanMerge(left, right Author) bool {
	lr, rr := strings.ToLower(left.Role), strings.ToLower(right.Role)
	if lr != "" && rr != "" && lr != "unknown" && rr != "unknown" && lr != rr {
		return false
	}
	lk, rk := CanonicalAuthorKey(left.ID), CanonicalAuthorKey(right.ID)
	if lk != "" && rk != "" {
		if lk == rk {
			return true
		}
		lp, _, _ := strings.Cut(lk, ":")
		rp, _, _ := strings.Cut(rk, ":")
		if lp == rp {
			return false
		}
	}
	if left.ID != "" && left.ID == right.ID {
		return true
	}
	return normalize(left.Name) != "" && normalize(left.Name) == normalize(right.Name)
}

func mergeEdition(base Edition, candidate Edition) Edition {
	if base.CoverURL == "" {
		base.CoverURL = candidate.CoverURL
	}
	if base.AudioSeconds == 0 {
		base.AudioSeconds = candidate.AudioSeconds
	}
	if base.Pages == 0 {
		base.Pages = candidate.Pages
	}
	base.Contributors = mergeAuthors(base.Contributors, candidate.Contributors)
	if base.ID == "" {
		base.ID = candidate.ID
	}
	if base.WorkID == "" {
		base.WorkID = candidate.WorkID
	}
	if base.Title == "" {
		base.Title = candidate.Title
	}
	if concreteFormat(base.Format) == FormatAny && concreteFormat(candidate.Format) != FormatAny {
		base.Format = candidate.Format
	}
	if base.Language == "" {
		base.Language = candidate.Language
	}
	base.ISBNs = appendUniqueStrings(base.ISBNs, candidate.ISBNs...)
	if base.ASIN == "" {
		base.ASIN = candidate.ASIN
	}
	if base.Publisher == "" {
		base.Publisher = candidate.Publisher
	}
	if base.PublishedDate == "" || len(candidate.PublishedDate) > len(base.PublishedDate) {
		base.PublishedDate = firstNonEmpty(candidate.PublishedDate, base.PublishedDate)
	}
	base.ProviderIDs = appendUniqueStrings(base.ProviderIDs, candidate.ProviderIDs...)
	if candidate.ID != "" {
		base.ProviderIDs = appendUniqueStrings(base.ProviderIDs, candidate.ID)
	}
	return base
}

func enrichResultProviderIDs(result SearchResult) SearchResult {
	result.Work.ProviderIDs = appendUniqueStrings(result.Work.ProviderIDs, result.Work.ID)
	result.Work.Authors = enrichAuthorProviderIDs(result.Work.Authors)
	result.Edition.ProviderIDs = appendUniqueStrings(result.Edition.ProviderIDs, result.Edition.ID)
	return result
}

func enrichAuthorProviderIDs(authors []Author) []Author {
	enriched := append([]Author(nil), authors...)
	for i := range enriched {
		if enriched[i].ID != "" {
			enriched[i].ProviderIDs = appendUniqueStrings(enriched[i].ProviderIDs, enriched[i].ID)
		}
	}
	return enriched
}

func firstAuthorName(result SearchResult) string {
	if len(result.Work.Authors) == 0 {
		return ""
	}
	return result.Work.Authors[0].Name
}

func resultYear(result SearchResult) int {
	if result.Work.FirstPublishYear > 0 {
		return result.Work.FirstPublishYear
	}
	return yearFromOpenLibraryDate(result.Edition.PublishedDate)
}

func firstNormalizedISBN(values []string) string {
	for _, value := range values {
		if normalized := normalizeISBN(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func firstSharedNormalizedISBN(left []string, right []string) string {
	seen := map[string]bool{}
	for _, value := range left {
		if normalized := canonicalISBN(value); normalized != "" {
			seen[normalized] = true
		}
	}
	for _, value := range right {
		if normalized := canonicalISBN(value); normalized != "" && seen[normalized] {
			return normalized
		}
	}
	return ""
}

func resultProviderNames(results []SearchResult) []string {
	seen := map[string]bool{}
	var providers []string
	for _, result := range results {
		provider := strings.TrimSpace(result.Provider)
		if provider == "" || seen[strings.ToLower(provider)] {
			continue
		}
		seen[strings.ToLower(provider)] = true
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values)+len(additions))
	for _, value := range append(values, additions...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}
