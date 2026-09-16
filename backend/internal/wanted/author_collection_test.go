package wanted

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestAuthorCollectionPagesAllSubscriptionsAndCountsAllBooks(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	s := NewService(store, nil)
	ctx := context.Background()
	book, err := store.CreateWanted(ctx, CreateRequest{Result: detailBook("hardcover-author:7", "Seed"), Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Z final", Format: "ebook", MonitorNewItems: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into author_subscriptions(provider,provider_key,author_name,wanted_format,status) select 'fixture','author:'||n,'Same Name',case when n%2=0 then 'ebook' else 'audiobook' end,case when n%3=0 then 'unmonitored' else 'monitored' end from generate_series(1,1001)n`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into wanted_items(work_id,wanted_format,title,author_name,status,monitored) select $1,'ebook','Imported book','Same Name','imported',true from generate_series(1,10000)n`, book.WorkID); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	cursor := ""
	var firstCursor string
	var slowest time.Duration
	for {
		started := time.Now()
		readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		page, err := s.AuthorCollection(readCtx, AuthorCollectionQuery{Status: "all", Cursor: cursor})
		cancel()
		if elapsed := time.Since(started); elapsed > slowest {
			slowest = elapsed
		}
		if err != nil || page.Total != 1002 || page.Filtered != 1002 || len(page.Authors) > 100 {
			t.Fatal(page.Total, page.Filtered, len(page.Authors), err)
		}
		for _, item := range page.Authors {
			if seen[item.ID] {
				t.Fatal("duplicate", item.ID)
			}
			seen[item.ID] = true
			if item.ID == sub.ID {
				if !item.IdentityLinked || item.TotalBooks != 10001 || item.Counts["missing"] != 10001 {
					t.Fatal(item)
				}
			} else if item.IdentityLinked || item.TotalBooks != 0 {
				t.Fatal("guessed same-name identity", item)
			}
		}
		cursor = page.NextCursor
		if firstCursor == "" {
			firstCursor = cursor
		}
		if cursor == "" {
			break
		}
	}
	if len(seen) != 1002 || !seen[sub.ID] {
		t.Fatal(len(seen))
	}
	t.Logf("1,002 subscriptions and 10,001 books: slowest bounded page %s", slowest)
	for _, q := range []AuthorCollectionQuery{{Search: "Z final", Cursor: firstCursor}, {Format: "ebook", Status: "all", Cursor: firstCursor}, {Status: "monitored", Cursor: firstCursor}} {
		if _, err = s.AuthorCollection(ctx, q); !errors.Is(err, ErrBookPage) {
			t.Fatal("cursor crossed filters", err)
		}
	}
	page, err := s.AuthorCollection(ctx, AuthorCollectionQuery{Search: "z FINAL", Format: "ebook"})
	if err != nil || page.Total != 1002 || page.Filtered != 1 || len(page.Authors) != 1 || page.Authors[0].ID != sub.ID {
		t.Fatal(page, err)
	}
	page, err = s.AuthorCollection(ctx, AuthorCollectionQuery{Status: "unmonitored", Format: "ebook"})
	if err != nil || page.Filtered != 166 {
		t.Fatal(page.Filtered, err)
	}
}

func TestAuthorCollectionRespectsIdentityFormatOverridesAndEvidence(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	addBook := func(author, title, format string) WantedItem {
		item, err := store.CreateWanted(ctx, CreateRequest{Result: detailBook(author, title), Format: format})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	first := addBook("hardcover-author:7", "First", "ebook")
	addBook("hardcover-author:8", "Second", "ebook")
	audio := addBook("hardcover-author:7", "Audio", "audiobook")
	// Duplicate writer roles count once; narrator-only and removed books count zero.
	if _, err := db.Exec(`insert into work_authors(work_id,author_id,role) select work_id,author_id,'writer' from work_authors where work_id=$1`, first.WorkID); err != nil {
		t.Fatal(err)
	}
	narrated := addBook("hardcover-author:7", "Narrated", "ebook")
	if _, err := db.Exec(`update work_authors set role='narrator' where work_id=$1`, narrated.WorkID); err != nil {
		t.Fatal(err)
	}
	removed := addBook("hardcover-author:7", "Removed", "ebook")
	if _, err := db.Exec(`update wanted_items set status='removed' where id=$1`, removed.ID); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"ebook", "audiobook"} {
		_, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Different display name", Format: format, MonitorNewItems: true})
		if err != nil {
			t.Fatal(err)
		}
	}
	chapter := evidenceFile(t, db, audio, "/library/chapter.mp3", "present")
	missing := evidenceFile(t, db, audio, "/library/missing.mp3", "missing")
	evidenceManifest(t, db, audio, chapter, missing)
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh", Downloads: []acquisition.DownloadStatus{{State: "downloading", Tags: []string{"wanted:" + first.ID}}}}}
	s := NewService(store, fixture)
	check := func(ebookState string, expected int) {
		t.Helper()
		page, err := s.AuthorCollection(ctx, AuthorCollectionQuery{})
		if err != nil || len(page.Authors) != 2 {
			t.Fatal(page, err)
		}
		for _, item := range page.Authors {
			if item.Format == "ebook" {
				if item.TotalBooks != expected || item.Counts[ebookState] != expected {
					t.Fatal(item)
				}
			} else if item.TotalBooks != 1 || item.Counts["incomplete"] != 1 {
				t.Fatal(item)
			}
		}
	}
	check("downloading", 1)
	fixture.evidence = acquisition.DownloadEvidence{Status: "unavailable"}
	check("unknown", 1)
	fixture.evidence = acquisition.DownloadEvidence{Status: "fresh"}
	check("missing", 1)
	if _, err := store.ApplyWantedMetadataCorrection(ctx, first.ID, MetadataCorrectionRequest{FieldName: "author_name", Value: "Owner Name"}); err != nil {
		t.Fatal(err)
	}
	check("missing", 0)
	// A second conflicting provider record must never pool distinct authors.
	if _, err := db.Exec(`insert into provider_records(provider,provider_key,entity_type,entity_id,raw,confidence) select 'HARDCOVER','HARDCOVER-AUTHOR:7','author',entity_id,'{}',1 from provider_records where provider_key='hardcover-author:8' and entity_type='author'`); err != nil {
		t.Fatal(err)
	}
	page, err := s.AuthorCollection(ctx, AuthorCollectionQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Authors {
		if item.IdentityLinked || item.TotalBooks != 0 {
			t.Fatal(item)
		}
	}
}

func TestAuthorCollectionRejectsInvalidQueriesBeforeIO(t *testing.T) {
	s := NewService(nil, nil)
	for _, q := range []AuthorCollectionQuery{{Status: "removed"}, {Format: "video"}, {Limit: 101}, {Limit: -1}, {Search: strings.Repeat("x", 257)}, {Cursor: "bad"}, {Cursor: strings.Repeat("x", 8193)}} {
		if _, err := s.AuthorCollection(context.Background(), q); !errors.Is(err, ErrBookPage) {
			t.Fatal(q, err)
		}
	}
	s = NewService(NewStore(testdb.Open(t)), nil)
	page, err := s.AuthorCollection(context.Background(), AuthorCollectionQuery{})
	if err != nil || page.Authors == nil || len(page.Authors) != 0 || page.Total != 0 || page.NextCursor != "" {
		t.Fatal(page, err)
	}
}
