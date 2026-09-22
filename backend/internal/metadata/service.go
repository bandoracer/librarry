package metadata

import (
	"context"
	"errors"
	"sort"
	"strings"
)

type Service struct {
	providers    []Provider
	searchCaches []*providerSearchCache
}

type ProviderError struct {
	Provider string `json:"provider"`
	Message  string `json:"message"`
}

type SearchOutcome struct {
	Query          Query           `json:"query"`
	Results        []SearchResult  `json:"results"`
	ProviderErrors []ProviderError `json:"providerErrors,omitempty"`
}

func NewService(providers []Provider) *Service {
	s := &Service{providers: append([]Provider(nil), providers...)}
	for range providers {
		s.searchCaches = append(s.searchCaches, newProviderSearchCache())
	}
	return s
}

func (s *Service) Providers() []Provider {
	return append([]Provider(nil), s.providers...)
}

func (s *Service) Health(ctx context.Context) []ProviderHealth {
	healths := make([]ProviderHealth, 0, len(s.providers))
	for _, provider := range s.providers {
		healths = append(healths, provider.Health(ctx))
	}
	return healths
}

func (s *Service) Diagnostics(ctx context.Context) []Diagnostic {
	diagnostics := make([]Diagnostic, 0, len(s.providers))
	for _, provider := range s.providers {
		diagnostics = append(diagnostics, provider.Diagnostics(ctx))
	}
	return diagnostics
}

func (s *Service) Search(ctx context.Context, query Query) ([]SearchResult, error) {
	outcome := s.SearchDetailed(ctx, query)
	if len(outcome.ProviderErrors) > 0 && len(outcome.Results) == 0 {
		return outcome.Results, errors.New(outcome.ProviderErrors[0].Message)
	}
	return outcome.Results, nil
}

func (s *Service) SearchDetailed(ctx context.Context, query Query) SearchOutcome {
	query.Query = strings.TrimSpace(query.Query)
	query.ProviderKey = strings.TrimSpace(query.ProviderKey)
	query.PreferredLanguage = normalizePreferredLanguage(query.PreferredLanguage)
	if query.Type == "" {
		query.Type = SearchTypeBook
	}
	if query.Format == "" {
		query.Format = FormatAny
	}
	if query.Limit <= 0 {
		query.Limit = 10
	}

	if err := ValidateSearchQuery(query); err != nil {
		return SearchOutcome{Query: query, Results: []SearchResult{}, ProviderErrors: []ProviderError{{Provider: "Search", Message: err.Error()}}}
	}
	merged := []SearchResult{}
	var providerErrors []ProviderError
	fallback := []int{}
	requestQuery := query
	lookup, lookupEligible := exactBookLookup(query)
	if lookupEligible && lookup.isbn != "" {
		requestQuery.Query = lookup.isbn
	}
	collect := func(index int, exactOnly bool) {
		provider := s.providers[index]
		results, err := s.searchCaches[index].search(ctx, requestQuery, provider)
		if err != nil {
			providerErrors = append(providerErrors, ProviderError{Provider: provider.Name(), Message: err.Error()})
			var partial *partialSearchError
			if len(results) == 0 || !errors.As(err, &partial) {
				return
			}
		}
		for rank, result := range results {
			result.discoveryRank = rank + 1
			if !discoveryEligible(query, result) || (exactOnly && !lookup.matches(result)) {
				continue
			}
			merged = append(merged, result)
		}
	}
	// Fallback ordering is policy, independent of constructor/provider order.
	for index, provider := range s.providers {
		if query.Type == SearchTypeSeries && provider.Name() != "Hardcover" {
			continue
		}
		if query.Type == SearchTypeAuthorWorks {
			key := CanonicalAuthorKey(query.ProviderKey)
			if strings.HasPrefix(key, "openlibrary:") && provider.Name() != "Open Library" || strings.HasPrefix(key, "hardcover-author:") && provider.Name() != "Hardcover" {
				continue
			}
		}
		if provider.Name() == "Google Books" {
			fallback = append(fallback, index)
			continue
		}
		collect(index, false)
	}
	if lookupEligible && !hasExactPrimaryMatch(query, merged) {
		for _, provider := range fallback {
			collect(provider, true)
		}
	}

	merged = mergeEquivalentResults(query, merged)
	eligible := merged[:0]
	for _, result := range merged {
		if discoveryEligible(query, result) {
			eligible = append(eligible, explainDiscovery(query, result))
		}
	}
	merged = eligible

	sort.SliceStable(merged, func(i, j int) bool {
		if query.Type == SearchTypeSeries {
			return merged[i].discoveryRank < merged[j].discoveryRank
		}
		if query.Type == SearchTypeBook {
			left, right := discoveryTier(query, merged[i]), discoveryTier(query, merged[j])
			if left != right {
				return left > right
			}
			if merged[i].discoveryRank != merged[j].discoveryRank {
				return merged[i].discoveryRank < merged[j].discoveryRank
			}
		} else if merged[i].Score != merged[j].Score {
			return merged[i].Score > merged[j].Score
		}
		if providerRank(merged[i].Provider) != providerRank(merged[j].Provider) {
			return providerRank(merged[i].Provider) < providerRank(merged[j].Provider)
		}
		if merged[i].discoveryRank != merged[j].discoveryRank {
			return merged[i].discoveryRank < merged[j].discoveryRank
		}
		return merged[i].Work.ID+merged[i].Edition.ID < merged[j].Work.ID+merged[j].Edition.ID
	})

	if len(merged) > query.Limit {
		merged = merged[:query.Limit]
	}
	return SearchOutcome{
		Query:          query,
		Results:        merged,
		ProviderErrors: providerErrors,
	}
}

func filterResultsByPreferredLanguage(results []SearchResult, preferred string) []SearchResult {
	if preferred == "" || strings.EqualFold(preferred, "any") {
		return results
	}
	filtered := make([]SearchResult, 0, len(results))
	for _, result := range results {
		if resultMatchesPreferredLanguage(result, preferred) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func resultMatchesPreferredLanguage(result SearchResult, preferred string) bool {
	if preferred = normalizePreferredLanguage(preferred); preferred == "" || preferred == "Any" {
		return true
	}
	language := strings.TrimSpace(result.Edition.Language)
	if language == "" {
		return true
	}
	return languageMatchesPreference(language, preferred)
}

func normalizePreferredLanguage(language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return ""
	}
	switch strings.ToLower(language) {
	case "any", "all", "none", "no preference":
		return "Any"
	case "en", "eng", "english":
		return "English"
	default:
		return language
	}
}

func languageMatchesPreference(language string, preferred string) bool {
	language = normalizeLanguageName(language)
	preferred = normalizeLanguageName(preferred)
	return language == "" || preferred == "" || language == preferred
}

// LanguageName normalizes supported language names/codes without prefix guessing.
func LanguageName(language string) string { return normalizeLanguageName(language) }

func normalizeLanguageName(language string) string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	switch normalized {
	case "en", "eng", "english":
		return "english"
	case "de", "deu", "ger", "german", "deutsch":
		return "german"
	case "fr", "fra", "fre", "french", "francais":
		return "french"
	case "es", "spa", "spanish", "espanol":
		return "spanish"
	case "it", "ita", "italian":
		return "italian"
	case "nl", "dut", "nld", "dutch", "nederlands":
		return "dutch"
	case "ja", "jpn", "japanese":
		return "japanese"
	case "zh", "chi", "zho", "chinese", "mandarin":
		return "chinese"
	case "ko", "kor", "korean":
		return "korean"
	case "pt", "por", "portuguese":
		return "portuguese"
	case "ru", "rus", "russian":
		return "russian"
	case "pl", "pol", "polish":
		return "polish"
	case "sv", "swe", "swedish":
		return "swedish"
	case "da", "dan", "danish":
		return "danish"
	case "no", "nor", "norwegian":
		return "norwegian"
	default:
		return normalized
	}
}

func providerRank(provider string) int {
	switch provider {
	case "Hardcover":
		return 0
	case "Open Library":
		return 1
	case "Google Books":
		return 2
	default:
		return 99
	}
}
