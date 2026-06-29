package metadata

import (
	"context"
	"errors"
	"sort"
	"strings"
)

type Service struct {
	providers []Provider
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
	return &Service{providers: providers}
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

	var merged []SearchResult
	var providerErrors []ProviderError
	for _, provider := range s.providers {
		results, err := provider.Search(ctx, query)
		if err != nil {
			providerErrors = append(providerErrors, ProviderError{
				Provider: provider.Name(),
				Message:  err.Error(),
			})
			continue
		}
		merged = append(merged, results...)
	}

	merged = mergeEquivalentResults(query, merged)
	merged = filterResultsByPreferredLanguage(merged, query.PreferredLanguage)

	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Score == merged[j].Score {
			return providerRank(merged[i].Provider) < providerRank(merged[j].Provider)
		}
		return merged[i].Score > merged[j].Score
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
