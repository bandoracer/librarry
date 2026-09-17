package metadata

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const bibliographyPageSize = 100
const bibliographyMaxRecords = 10000

// Bibliography is separate from ranked, limited search. Monitoring may only
// apply whole-author policies after a provider has completed every page.
type bibliographyProvider interface {
	Bibliography(Context, Query) ([]SearchResult, error)
}

func (s *Service) AuthorBibliography(ctx context.Context, query Query) ([]SearchResult, error) {
	query.Type = SearchTypeAuthorWorks
	query.ProviderKey = strings.TrimSpace(query.ProviderKey)
	providerName := ""
	if openLibraryAuthorKey(query.ProviderKey) != "" {
		providerName = "Open Library"
	}
	if hardcoverAuthorID(query.ProviderKey) > 0 {
		providerName = "Hardcover"
	}
	if providerName == "" {
		return nil, errors.New("author monitoring requires a verified Open Library or Hardcover author ID; select an author from metadata search")
	}
	for index, provider := range s.providers {
		if provider.Name() != providerName {
			continue
		}
		loader, ok := provider.(bibliographyProvider)
		if !ok {
			break
		}
		bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		// Whole traversals bypass limited search caches; a cached first page
		// must never establish complete coverage.
		results, err := loader.Bibliography(bounded, query)
		if err != nil {
			if bounded.Err() == nil {
				s.searchCaches[index].invalidate()
			}
			return nil, err
		}
		if results == nil {
			results = []SearchResult{}
		}
		return results, nil
	}
	return nil, errors.New("the selected author provider does not support complete bibliographies")
}

// CanonicalAuthorKey accepts stable provider identities only, never name hashes.
func CanonicalAuthorKey(key string) string {
	key = strings.TrimSpace(key)
	if id := openLibraryAuthorKey(key); id != "" {
		return "openlibrary:" + id
	}
	if id := hardcoverAuthorID(key); id > 0 {
		return fmt.Sprintf("hardcover-author:%d", id)
	}
	return ""
}

// Only locally authored validation messages pass here; provider bodies stay private.
func providerValidationError(message string) error {
	return &providerFailure{status: "degraded", message: message, reachable: true}
}
