package wanted

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestConcreteEditionDoesNotReuseWorkFormatPlaceholder(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	result := bibliographyCandidate(1, "1990-01-01", "Author")
	old, err := store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	result.Edition = metadata.Edition{ID: "hardcover-edition:101", WorkID: "hardcover:1", Title: "First edition", Format: metadata.FormatEbook, ISBNs: []string{"9780142437247"}, Pages: 301, Contributors: []metadata.Author{{ID: "hardcover-author:8", Name: "Translator", Role: "Translator"}}}
	first, err := store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	result.Edition.ID = "hardcover-edition:102"
	result.Edition.Title = "Another edition"
	result.Edition.ISBNs = []string{"9780593135204"}
	second, err := store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if first.WorkID != old.WorkID || second.WorkID != old.WorkID || first.EditionID == old.EditionID || second.EditionID == first.EditionID || second.EditionID == old.EditionID {
		t.Fatal(old, first, second)
	}
	var isbn string
	if err := db.QueryRow(`select value from edition_identifiers where edition_id=$1 and kind='isbn'`, first.EditionID).Scan(&isbn); err != nil || isbn != "9780142437247" {
		t.Fatal(isbn, err)
	}
	var raw []byte
	if err := db.QueryRow(`select raw from provider_records where provider='Hardcover' and provider_key='hardcover-edition:101' and entity_id=$1`, first.EditionID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var evidence metadata.SearchResult
	if err := json.Unmarshal(raw, &evidence); err != nil || evidence.Edition.Pages != 301 || len(evidence.Edition.Contributors) != 1 || evidence.Edition.Contributors[0].Role != "Translator" {
		t.Fatal(evidence, err)
	}
	// Automatic monitoring still reuses existing work/format tracking rather than
	// adding a different provider-selected default edition behind the user.
	repeated, err := store.CreateWanted(ctx, CreateRequest{OnlyIfUntracked: true, Result: result, Format: "ebook"})
	if err != nil || !repeated.WasAlreadyTracked() || repeated.ID != old.ID {
		t.Fatal(repeated, err)
	}
}
func TestHardcoverEditionAliasRetainsProviderAfterMerge(t *testing.T) {
	result := metadata.SearchResult{Provider: "Open Library", Work: metadata.Work{ID: "openlibrary:OL1W"}, Edition: metadata.Edition{ID: "openlibrary:OL1M", ProviderIDs: []string{"hardcover-edition:101"}}}
	aliases := editionProviderAliases(result, "ebook")
	if !hasAlias(aliases, "Hardcover", "hardcover-edition:101") || hasAlias(aliases, "Open Library", "Open Library:edition:openlibrary:OL1W:ebook") {
		t.Fatal(aliases)
	}
}
