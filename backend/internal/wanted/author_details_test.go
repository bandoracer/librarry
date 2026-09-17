package wanted

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func detailBook(authorID, title string) metadata.SearchResult {
	return metadata.SearchResult{Provider: "Hardcover", Work: metadata.Work{ID: "hardcover:" + title, Title: title, Authors: []metadata.Author{{ID: authorID, Name: "Same Name", Role: "Author"}}}, Edition: metadata.Edition{Format: metadata.FormatEbook}}
}

func TestAuthorDetailSeparatesIdentitiesAndManualNames(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	first, err := store.CreateWanted(ctx, CreateRequest{Result: detailBook("hardcover-author:7", "First"), Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateWanted(ctx, CreateRequest{Result: detailBook("hardcover-author:8", "Second"), Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Authors) != 1 || len(second.Authors) != 1 || first.Authors[0].ID == second.Authors[0].ID {
		t.Fatal(first.Authors, second.Authors)
	}
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Renamed subscription", Format: "ebook", MonitorNewItems: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{sub.ID, first.Authors[0].ID} {
		detail, err := store.AuthorDetail(ctx, key, "", 100)
		if err != nil || len(detail.Books) != 1 || detail.Books[0].ID != first.ID || len(detail.Subscriptions) != 1 {
			t.Fatal(detail, err)
		}
	}
	ambiguous, err := store.AuthorDetail(ctx, "same name", "", 100)
	if err != nil || len(ambiguous.Choices) != 2 || len(ambiguous.Books) != 0 {
		t.Fatal(ambiguous, err)
	}
	if _, err := store.ApplyWantedMetadataCorrection(ctx, first.ID, MetadataCorrectionRequest{FieldName: "author_name", Value: "Corrected Name"}); err != nil {
		t.Fatal(err)
	}
	corrected, err := store.GetWanted(ctx, first.ID)
	if err != nil || len(corrected.Authors) != 0 {
		t.Fatal(corrected, err)
	}
	detail, err := store.AuthorDetail(ctx, first.Authors[0].ID, "", 100)
	if err != nil || detail.TotalBooks != 0 {
		t.Fatal(detail, err)
	}
	detail, err = store.AuthorDetail(ctx, "corrected name", "", 100)
	if err != nil || !detail.Author.NameOnly || len(detail.Books) != 1 || detail.Books[0].ID != first.ID {
		t.Fatal(detail, err)
	}
}

func TestAuthorDetailTraversesBeyondOldCapsWithStableTitleTies(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	book, err := store.CreateWanted(ctx, CreateRequest{Result: detailBook("hardcover-author:7", "Seed"), Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update wanted_items set status='removed' where id=$1`, book.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into wanted_items(work_id,wanted_format,title,author_name,status,monitored)
	 select $1,'ebook',case when n%3=0 then 'Ångström' when n%3=1 then 'Same title' else 'İstanbul' end,'Same Name','imported',false from generate_series(1,10001) n`, book.WorkID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into author_subscriptions(provider,provider_key,author_name,wanted_format,status) select 'fixture','unrelated:'||n,'A '||n,'ebook','monitored' from generate_series(1,600) n`); err != nil {
		t.Fatal(err)
	}
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Z last author", Format: "ebook", MonitorNewItems: true})
	if err != nil {
		t.Fatal(err)
	}
	cursor := ""
	seen := map[string]bool{}
	pages := 0
	for {
		detail, err := store.AuthorDetail(ctx, sub.ID, cursor, 100)
		if err != nil {
			t.Fatal(err)
		}
		if detail.TotalBooks != 10001 || len(detail.Books) > 100 {
			t.Fatal(detail.TotalBooks, len(detail.Books))
		}
		for _, item := range detail.Books {
			if seen[item.ID] || item.Monitored || item.Status != "imported" {
				t.Fatal("duplicate or wrong membership", item)
			}
			seen[item.ID] = true
		}
		pages++
		if detail.NextCursor == "" {
			break
		}
		if pages > 102 {
			t.Fatal("pagination did not finish")
		}
		cursor = detail.NextCursor
	}
	if len(seen) != 10001 || pages != 101 {
		t.Fatal(len(seen), pages)
	}
	if _, err := store.AuthorDetail(ctx, book.Authors[0].ID, cursor, 100); !errors.Is(err, ErrAuthorPage) {
		t.Fatal("cross-author cursor accepted", err)
	}
	for _, cursor := range []string{"not-base64", "e30"} {
		if _, err := store.AuthorDetail(ctx, sub.ID, cursor, 100); !errors.Is(err, ErrAuthorPage) {
			t.Fatal(err)
		}
	}
}

func TestLegacyAuthorNameKeys(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	for i, name := range []string{"The Example", "A Name", "Anne-Marie", "", "李白"} {
		var id string
		if err := db.QueryRow(`insert into wanted_items(wanted_format,title,author_name) values('ebook',$1,$2) returning id::text`, fmt.Sprintf("Book %d", i), name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		detail, err := store.AuthorDetail(ctx, legacyAuthorKey(name), "", 100)
		if err != nil || len(detail.Books) != 1 || detail.Books[0].ID != id {
			t.Fatal(name, detail, err)
		}
	}
}

func TestAuthorDetailPreservesCoauthorsAndConcurrentIdentity(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:8", AuthorName: "Coauthor", Format: "ebook", MonitorNewItems: true})
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errs := make(chan error, 8)
	for i := range 8 {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			result := detailBook("hardcover-author:7", fmt.Sprintf("Concurrent %d", i))
			result.Work.Authors = append(result.Work.Authors, metadata.Author{ID: "hardcover-author:8", Name: "Coauthor", Role: "Writer"}, metadata.Author{ID: "hardcover-author:9", Name: "Narrator", Role: "Narrator"})
			if i%2 == 0 {
				result.Work.Authors[0], result.Work.Authors[1] = result.Work.Authors[1], result.Work.Authors[0]
			}
			_, err := store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook"})
			errs <- err
		}(i)
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow(`select count(*) from authors`).Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	detail, err := store.AuthorDetail(ctx, sub.ID, "", 100)
	if err != nil || detail.TotalBooks != 8 {
		t.Fatal(detail, err)
	}
	for _, book := range detail.Books {
		if len(book.Authors) != 2 {
			t.Fatal(book.Authors)
		}
	}
	var narratorID string
	if err := db.QueryRow(`select entity_id::text from provider_records where provider_key='hardcover-author:9'`).Scan(&narratorID); err != nil {
		t.Fatal(err)
	}
	detail, err = store.AuthorDetail(ctx, narratorID, "", 100)
	if err != nil || detail.TotalBooks != 0 {
		t.Fatal(detail, err)
	}
}

func TestUnlinkedSubscriptionFormatsResolveOneIdentity(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	for _, format := range []string{"ebook", "audiobook"} {
		if _, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Empty Author", Format: format, MonitorNewItems: true}); err != nil {
			t.Fatal(err)
		}
	}
	detail, err := store.AuthorDetail(ctx, "empty author", "", 100)
	if err != nil || len(detail.Choices) != 0 || len(detail.Subscriptions) != 2 || len(detail.Books) != 0 {
		t.Fatal(detail, err)
	}
}
