package wanted

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestReviewCollectionTenThousandBooks(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	seed, err := store.CreateWanted(ctx, CreateRequest{Result: detailBook("hardcover-author:7", "Seed"), Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`update wanted_items set status='removed' where id=$1`, seed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into wanted_items(id,work_id,wanted_format,title,author_name,status,monitored,created_at)
 select md5(n::text)::uuid,$1,case when n%2=0 then 'ebook' else 'audiobook' end,'Tied title','Same Name',case when n=10002 then 'removed' when n=10003 then 'ignored' else 'imported' end,n%3<>0,'2020-01-01' from generate_series(1,10003)n`, seed.WorkID); err != nil {
		t.Fatal(err)
	}
	// Distinct works/provider records prevent shared-record caching from making
	// the scale fixture artificially cheap. Writer identity is legitimately shared.
	if _, err = db.Exec(`insert into works(id,title,sort_title) select md5('work:'||id::text)::uuid,'Provider title','provider title' from wanted_items where id<>$1`, seed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into work_authors(work_id,author_id,role) select md5('work:'||wi.id::text)::uuid,wa.author_id,wa.role from wanted_items wi cross join work_authors wa where wi.id<>$1 and wa.work_id=$2`, seed.ID, seed.WorkID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into provider_records(provider,provider_key,entity_type,entity_id,raw,confidence) select 'Hardcover','work:'||wi.id::text,'work',md5('work:'||wi.id::text)::uuid,p.raw,1 from wanted_items wi cross join provider_records p where wi.id<>$1 and p.entity_id=$2 and p.entity_type='work'`, seed.ID, seed.WorkID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`update wanted_items set work_id=md5('work:'||id::text)::uuid where id<>$1`, seed.ID); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	cursor := ""
	firstCursor := ""
	var durations []time.Duration
	for {
		started := time.Now()
		readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		page, err := store.MetadataReviewCollection(readCtx, MetadataReviewQuery{Cursor: cursor})
		cancel()
		durations = append(durations, time.Since(started))
		if err != nil || page.Total != 10001 || page.Filtered != 10001 || len(page.Items) > 100 {
			t.Fatal(page.Total, page.Filtered, len(page.Items), err)
		}
		for _, item := range page.Items {
			if seen[item.WantedItem.ID] || item.WantedItem.Status != "imported" {
				t.Fatal(item)
			}
			seen[item.WantedItem.ID] = true
		}
		cursor = page.NextCursor
		if firstCursor == "" {
			firstCursor = cursor
		}
		if cursor == "" {
			break
		}
	}
	if len(seen) != 10001 {
		t.Fatal(len(seen))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("10,001-book review collection: %d pages, p95 %s", len(durations), durations[(len(durations)*95-1)/100])
	page, err := store.MetadataReviewCollection(ctx, MetadataReviewQuery{Format: "ebook", Search: "Tied"})
	if err != nil || page.Filtered != 5000 || page.Total != 10001 {
		t.Fatal(page.Total, page.Filtered, err)
	}
	if _, err = store.MetadataReviewCollection(ctx, MetadataReviewQuery{Format: "ebook", Cursor: firstCursor}); !errors.Is(err, ErrBookPage) {
		t.Fatal(err)
	}
}

func TestReviewCollectionMatchesDetailsAndOwnerChoices(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	book, err := store.CreateWanted(ctx, CreateRequest{Result: detailBook("hardcover-author:7", "Provider Title"), Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ApplyWantedMetadataCorrection(ctx, book.ID, MetadataCorrectionRequest{FieldName: "title", Value: "Owner Title"}); err != nil {
		t.Fatal(err)
	}
	page, err := store.MetadataReviewCollection(ctx, MetadataReviewQuery{})
	if err != nil || page.Total != 1 {
		t.Fatal(page, err)
	}
	detail, err := store.WantedMetadataProvenance(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(page.Items[0], metadataReviewItem(detail)) {
		t.Fatalf("collection and detail differ: %+v / %+v", page.Items[0], metadataReviewItem(detail))
	}
	oldRevision := page.Items[0].Revision
	if _, err = db.Exec(`update provider_records set confidence=0.91 where entity_id=$1`, book.WorkID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ConfirmMetadataReviewCanonical(ctx, MetadataReviewConfirmRequest{WantedIDs: []string{book.ID}, Revisions: map[string]string{book.ID: oldRevision}}); !errors.Is(err, ErrReviewChanged) {
		t.Fatal("stale rendered evidence accepted", err)
	}
	page, err = store.MetadataReviewCollection(ctx, MetadataReviewQuery{})
	if err != nil || page.Total != 1 {
		t.Fatal(page, err)
	}
	outcome, err := store.ConfirmMetadataReviewCanonical(ctx, MetadataReviewConfirmRequest{WantedIDs: []string{book.ID, book.ID}, Revisions: map[string]string{book.ID: page.Items[0].Revision}})
	if err != nil || outcome.ItemsReviewed != 1 || outcome.FieldsConfirmed != 1 || outcome.Items[0].WantedItem.Title != "Owner Title" {
		t.Fatal(outcome, err)
	}
	page, err = store.MetadataReviewCollection(ctx, MetadataReviewQuery{})
	if err != nil || page.Total != 0 || page.Items == nil {
		t.Fatal(page, err)
	}
	if _, err = store.ClearWantedManualOverrides(ctx, book.ID, []string{"title"}); err != nil {
		t.Fatal(err)
	}
	formatFields := metadataFieldEvidence(WantedItem{Format: "audiobook"}, []ProviderMetadataRecord{{Values: MetadataRecordValues{Format: "ebook"}}})
	formatField, ok := findMetadataField(formatFields, "format")
	if !ok || formatField.Conflict || len(formatField.Candidates) != 1 {
		t.Fatal(formatFields)
	}
	// Unicode values that used to collapse to an empty ASCII key must conflict.
	fields := metadataFieldEvidence(WantedItem{Title: "東京"}, []ProviderMetadataRecord{{Values: MetadataRecordValues{Title: "大阪"}}})
	if len(fields) == 0 || !fields[0].Conflict {
		t.Fatal(fields)
	}
	fields = metadataFieldEvidence(WantedItem{Title: "Café"}, []ProviderMetadataRecord{{Values: MetadataRecordValues{Title: "Cafe\u0301"}}})
	if fields[0].Conflict {
		t.Fatal("canonical Unicode equivalents conflict", fields)
	}
}

func TestReviewCollectionValidatesFiltersBeforePersistence(t *testing.T) {
	service := NewService(nil, nil)
	for _, q := range []MetadataReviewQuery{{Limit: 101}, {Limit: -1}, {Format: "video"}, {Cursor: "bad"}, {Search: strings.Repeat("x", 257)}} {
		if _, err := service.MetadataReviewCollection(context.Background(), q); !errors.Is(err, ErrBookPage) {
			t.Fatal(q, err)
		}
	}
}
