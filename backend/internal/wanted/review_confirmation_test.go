package wanted

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestReviewConfirmationExactSelectionAllAndRollback(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	seed, err := store.CreateWanted(ctx, CreateRequest{Result: detailBook("hardcover-author:7", "Provider"), Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`update wanted_items set status='removed' where id=$1`, seed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into wanted_items(id,work_id,wanted_format,title,author_name,status,created_at) select md5(n::text)::uuid,$1,'ebook','Owner title','Same Name','imported','2020-01-01' from generate_series(1,501)n`, seed.WorkID); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`select id::text from wanted_items where status='imported' order by id`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	selected := ids[301:]
	if _, err = store.ConfirmMetadataReviewCanonical(ctx, MetadataReviewConfirmRequest{WantedIDs: append(append([]string{}, selected[:199]...), "00000000-0000-0000-0000-000000000000")}); !errors.Is(err, ErrReviewSelection) {
		t.Fatal(err)
	}
	var n int
	if err = db.QueryRow(`select count(*) from manual_overrides where reason=$1`, manualOverrideReasonCanonicalAccepted).Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	// A late write failure must roll back earlier confirmations as well.
	if _, err = db.Exec(fmt.Sprintf(`create function reject_review() returns trigger language plpgsql as $$ begin if new.entity_id='%s'::uuid then raise exception 'fixture confirmation failure'; end if;return new;end $$;create trigger reject_review before insert on manual_overrides for each row execute function reject_review()`, selected[len(selected)-1])); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ConfirmMetadataReviewCanonical(ctx, MetadataReviewConfirmRequest{WantedIDs: selected}); err == nil {
		t.Fatal("expected fixture failure")
	}
	if err = db.QueryRow(`select count(*) from manual_overrides where reason=$1`, manualOverrideReasonCanonicalAccepted).Scan(&n); err != nil || n != 0 {
		t.Fatal("partial confirmation", n, err)
	}
	if _, err = db.Exec(`drop trigger reject_review on manual_overrides;drop function reject_review()`); err != nil {
		t.Fatal(err)
	}
	outcome, err := store.ConfirmMetadataReviewCanonical(ctx, MetadataReviewConfirmRequest{WantedIDs: selected})
	if err != nil || outcome.ItemsReviewed != 200 || outcome.FieldsConfirmed != 200 {
		t.Fatal(outcome.ItemsReviewed, outcome.FieldsConfirmed, err)
	}
	if err = db.QueryRow(`select count(*) from manual_overrides where entity_id=any($1::uuid[])`, ids[:301]).Scan(&n); err != nil || n != 0 {
		t.Fatal("unselected changed", n, err)
	}
	outcome, err = store.ConfirmMetadataReviewCanonical(ctx, MetadataReviewConfirmRequest{All: true})
	if err != nil || outcome.ItemsReviewed != 301 {
		t.Fatal(outcome.ItemsReviewed, err)
	}
	page, err := store.MetadataReviewCollection(ctx, MetadataReviewQuery{})
	if err != nil || page.Total != 0 {
		t.Fatal(page.Total, err)
	}
	outcome, err = store.ConfirmMetadataReviewCanonical(ctx, MetadataReviewConfirmRequest{WantedIDs: []string{ids[0], seed.ID}})
	if err != nil || outcome.ItemsReviewed != 0 || outcome.SkippedItems != 2 {
		t.Fatal(outcome, err)
	}
}

func TestReviewConfirmationPreservesConcurrentOwnerCorrection(t *testing.T) {
	for _, bypass := range []bool{false, true} {
		for _, clear := range []bool{false, true} {
			t.Run(fmt.Sprintf("clear=%t/bypass=%t", clear, bypass), func(t *testing.T) { reviewConcurrentOwnerChoice(t, clear, bypass) })
		}
	}
}

func reviewConcurrentOwnerChoice(t *testing.T, clear, bypass bool) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	book, err := store.CreateWanted(ctx, CreateRequest{Result: detailBook("hardcover-author:7", "Provider Title"), Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ApplyWantedMetadataCorrection(ctx, book.ID, MetadataCorrectionRequest{FieldName: "title", Value: "Owner Title"}); err != nil {
		t.Fatal(err)
	}
	lock, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err = lock.ExecContext(ctx, `select pg_advisory_lock(784923)`); err != nil {
		t.Fatal(err)
	}
	defer lock.ExecContext(context.Background(), `select pg_advisory_unlock(784923)`)
	if _, err = db.ExecContext(ctx, `create function pause_confirmation() returns trigger language plpgsql as $$ begin if position('update manual_overrides set reason=' in current_query())>0 then perform pg_advisory_xact_lock(784923);end if;return null;end $$;create trigger pause_confirmation before update on manual_overrides for each statement execute function pause_confirmation()`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, e := store.ConfirmMetadataReviewCanonical(ctx, MetadataReviewConfirmRequest{WantedIDs: []string{book.ID}})
		finished <- e
	}()
	waitFor := func(condition string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			var waiting bool
			if err = db.QueryRowContext(ctx, `select exists(select 1 from pg_stat_activity where datname=current_database() and `+condition+`)`).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("operation did not reach expected lock", condition)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitFor(`wait_event='advisory'`)
	ownerDone := make(chan error, 1)
	if bypass {
		// Model an older/noncooperating writer that does not lock the book first.
		if clear {
			_, err = db.ExecContext(ctx, `delete from manual_overrides where entity_id=$1 and field_name='title'`, book.ID)
		} else {
			_, err = db.ExecContext(ctx, `update manual_overrides set value='"New Owner Title"',reason='manual correction' where entity_id=$1 and field_name='title'`, book.ID)
		}
		if err != nil {
			t.Fatal(err)
		}
	} else {
		go func() {
			var e error
			if clear {
				_, e = store.ClearWantedManualOverrides(ctx, book.ID, []string{"title"})
			} else {
				_, e = store.ApplyWantedMetadataCorrection(ctx, book.ID, MetadataCorrectionRequest{FieldName: "title", Value: "New Owner Title"})
			}
			ownerDone <- e
		}()
		waitFor(`wait_event_type='Lock' and query like 'select id from wanted_items%for update'`)
	}
	if _, err = lock.ExecContext(ctx, `select pg_advisory_unlock(784923)`); err != nil {
		t.Fatal(err)
	}
	err = <-finished
	if bypass {
		if !errors.Is(err, ErrReviewChanged) {
			t.Fatal("expected stale confirmation", err)
		}
	} else {
		if err != nil {
			t.Fatal(err)
		}
		if err = <-ownerDone; err != nil {
			t.Fatal("owner mutation failed", err)
		}
	}
	detail, err := store.WantedMetadataProvenance(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	field, ok := findMetadataField(detail.Fields, "title")
	if clear {
		if !ok || field.Protected || field.ReviewResolved {
			t.Fatal("cleared choice resurrected", field)
		}
	} else if !ok || field.CanonicalValue != "New Owner Title" || field.ReviewResolved || !field.Conflict {
		t.Fatal(field)
	}
}

func TestReviewConfirmationRejectsInvalidSelectionBeforePersistence(t *testing.T) {
	service := NewService(nil, nil)
	for _, request := range []MetadataReviewConfirmRequest{{}, {All: true, WantedIDs: []string{"bad"}}, {WantedIDs: []string{""}}, {WantedIDs: []string{"bad"}}, {WantedIDs: strings.Split(strings.Repeat("id,", 200)+"id", ",")}} {
		if _, err := service.ConfirmMetadataReviewCanonical(context.Background(), request); !errors.Is(err, ErrReviewSelection) {
			t.Fatal(request, err)
		}
	}
}
