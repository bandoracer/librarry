package wanted

import (
	"testing"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

func TestProviderAliasesIncludeMergedMetadataIdentities(t *testing.T) {
	result := metadata.SearchResult{
		Provider: "Hardcover",
		Work: metadata.Work{
			ID:          "hardcover:123",
			Title:       "Dungeon Crawler Carl",
			ProviderIDs: []string{"hardcover:123", "openlibrary:OL999W", "googlebooks:abc123"},
			Authors: []metadata.Author{{
				ID:          "hardcover-author:matt-dinniman",
				Name:        "Matt Dinniman",
				ProviderIDs: []string{"hardcover-author:matt-dinniman", "openlibrary:OL999A", "googlebooks-author:matt"},
			}},
		},
		Edition: metadata.Edition{
			ID:          "openlibrary:OL999M",
			WorkID:      "openlibrary:OL999W",
			Title:       "Dungeon Crawler Carl",
			Format:      metadata.FormatAudiobook,
			ProviderIDs: []string{"openlibrary:OL999M", "googlebooks:abc123:edition"},
		},
	}

	workAliases := workProviderAliases(result)
	if !hasAlias(workAliases, "Hardcover", "hardcover:123") ||
		!hasAlias(workAliases, "Open Library", "openlibrary:OL999W") ||
		!hasAlias(workAliases, "Google Books", "googlebooks:abc123") {
		t.Fatalf("expected merged work provider aliases, got %+v", workAliases)
	}

	authorAliases := authorProviderAliases(result, result.Work.Authors[0])
	if !hasAlias(authorAliases, "Hardcover", "hardcover-author:matt-dinniman") ||
		!hasAlias(authorAliases, "Open Library", "openlibrary:OL999A") ||
		!hasAlias(authorAliases, "Google Books", "googlebooks-author:matt") {
		t.Fatalf("expected merged author provider aliases, got %+v", authorAliases)
	}

	editionAliases := editionProviderAliases(result, "audiobook")
	if !hasAlias(editionAliases, "Open Library", "openlibrary:OL999M") ||
		!hasAlias(editionAliases, "Google Books", "googlebooks:abc123:edition") {
		t.Fatalf("expected merged edition provider aliases, got %+v", editionAliases)
	}
}

func TestProviderNameFromMetadataKeyFallsBackForSyntheticKeys(t *testing.T) {
	if provider := providerNameFromMetadataKey("/authors/OL123A", "Hardcover"); provider != "Open Library" {
		t.Fatalf("expected Open Library author path, got %q", provider)
	}
	if provider := providerNameFromMetadataKey("custom:edition:key", "Hardcover"); provider != "Hardcover" {
		t.Fatalf("expected fallback provider, got %q", provider)
	}
}

func hasAlias(aliases []providerAlias, provider string, key string) bool {
	for _, alias := range aliases {
		if alias.Provider == provider && alias.Key == key {
			return true
		}
	}
	return false
}
