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
