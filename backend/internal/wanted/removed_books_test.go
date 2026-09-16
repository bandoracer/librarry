package wanted

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestRemovedBooksCompleteCollection(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), noChoiceNetwork{})
	ctx := context.Background()
	_, err := db.Exec(`insert into wanted_items(wanted_format,title,author_name,status,created_at) select case when i<=1500 then 'audiobook' else 'ebook' end,'Removed '||i,'Writer',case when i<=10001 then 'removed' else 'ignored' end,'2026-01-01'::timestamptz from generate_series(1,10002)i;insert into wanted_items(wanted_format,title,status) values('ebook','Active','wanted')`)
	if err != nil {
		t.Fatal(err)
	}
	q := RemovedBooksQuery{Limit: 100}
	seen := map[string]bool{}
	var times []time.Duration
	var first RemovedBooks
	for {
		start := time.Now()
		p, e := s.RemovedBooks(ctx, q)
		times = append(times, time.Since(start))
		if e != nil {
			t.Fatal(e)
		}
		if p.Total != 10002 || p.Filtered != 10001 || p.Counts["removed"] != 10001 || p.Counts["ignored"] != 1 || p.ObservedAt.IsZero() {
			t.Fatalf("bad counts %+v", p)
		}
		if first.NextCursor == "" {
			first = p
		}
		for _, b := range p.Books {
			if seen[b.ID] || b.Status != "removed" {
				t.Fatal(b.ID)
			}
			seen[b.ID] = true
		}
		if p.NextCursor == "" {
			break
		}
		q.Cursor = p.NextCursor
	}
	if len(seen) != 10001 || len(times) != 101 {
		t.Fatal(len(seen), len(times))
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	t.Logf("10001 removed books, 101 pages, p95 %s", times[len(times)*95/100])
	for _, tt := range []struct {
		q RemovedBooksQuery
		n int
	}{{RemovedBooksQuery{Status: "all"}, 10002}, {RemovedBooksQuery{Status: "ignored"}, 1}, {RemovedBooksQuery{Format: "audiobook"}, 1500}, {RemovedBooksQuery{Search: "Removed 10001"}, 1}, {RemovedBooksQuery{Search: "%"}, 0}} {
		p, e := s.RemovedBooks(ctx, tt.q)
		if e != nil || p.Filtered != tt.n {
			t.Fatal(tt, p.Filtered, e)
		}
	}
	for _, q := range []RemovedBooksQuery{{Status: "active"}, {Limit: 101}, {Format: "video"}, {Cursor: "bad"}, {Status: "all", Cursor: first.NextCursor}, {Search: "changed", Cursor: first.NextCursor}} {
		if _, e := s.RemovedBooks(ctx, q); !errors.Is(e, ErrRemovedBooks) {
			t.Fatal(q, e)
		}
	}
	if _, err = db.Exec(`update wanted_items set updated_at=now()`); err != nil {
		t.Fatal(err)
	}
	anchor := first.Books[len(first.Books)-1].ID
	if _, err = db.Exec(`delete from wanted_items where id=$1`, anchor); err != nil {
		t.Fatal(err)
	}
	next, e := s.RemovedBooks(ctx, RemovedBooksQuery{Cursor: first.NextCursor, Limit: 1})
	if e != nil || len(next.Books) != 1 || next.Books[0].ID >= anchor {
		t.Fatal(next, e)
	}
}

func TestRestoreBookPreservesRecordsAndRejectsStaleReview(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	s := NewService(store, noChoiceNetwork{})
	ctx := context.Background()
	var id, rootID, fileID string
	if err := db.QueryRow(`insert into root_folders(name,path) values('Saved destination','/fixture/library') returning id`).Scan(&rootID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title,author_name,status,monitored,quality_profile,tags,root_folder_id) values('ebook','Owner title','Owner author','removed',false,'premium','saved-tag',$1) returning id`, rootID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`insert into files(media_format,path,title) values('ebook','/fixture/book.epub','File metadata') returning id`).Scan(&fileID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into file_wanted_links(file_id,wanted_item_id) values($1,$2)`, fileID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateWanted(ctx, id, WantedUpdateRequest{Title: "Protected title"}); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetWanted(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestoreBook(ctx, id, RestoreBookRequest{UpdatedAt: before.UpdatedAt.Add(-time.Second)}); !errors.Is(err, ErrRestoreBookChanged) {
		t.Fatal(err)
	}
	// History failure must roll back the entire restore, including monitoring.
	if _, err = db.Exec(`create function fail_restore_history() returns trigger language plpgsql as $$begin raise exception 'fixture history failure';end$$;create trigger fail_restore_history before insert on history_events for each row execute function fail_restore_history()`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestoreBook(ctx, id, RestoreBookRequest{UpdatedAt: before.UpdatedAt, Monitored: true}); err == nil {
		t.Fatal("history failure accepted")
	}
	unchanged, err := store.GetWanted(ctx, id)
	if err != nil || unchanged.Status != "removed" || unchanged.Monitored || !unchanged.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatal(unchanged, err)
	}
	if _, err = db.Exec(`drop trigger fail_restore_history on history_events`); err != nil {
		t.Fatal(err)
	}
	restored, err := s.RestoreBook(ctx, id, RestoreBookRequest{UpdatedAt: before.UpdatedAt})
	if err != nil || restored.Status != "wanted" || restored.Monitored || restored.RootFolderID != rootID || restored.QualityProfile != "premium" || restored.Title != "Protected title" || len(restored.ManualOverrides) != 1 || len(restored.Tags) != 1 || restored.Tags[0] != "saved-tag" {
		t.Fatal(restored, err)
	}
	if _, err = s.RestoreBook(ctx, id, RestoreBookRequest{UpdatedAt: before.UpdatedAt}); !errors.Is(err, ErrRestoreBookChanged) {
		t.Fatal("replayed restore", err)
	}
	var n int
	if err = db.QueryRow(`select count(*) from file_wanted_links where wanted_item_id=$1 and file_id=$2`, id, fileID).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err = db.QueryRow(`select count(*) from history_events where entity_id=$1 and event_type='wanted_restored'`, id).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	removed, err := s.RemovedBooks(ctx, RemovedBooksQuery{})
	if err != nil || removed.Total != 0 || removed.Books == nil {
		t.Fatal(removed, err)
	}
	choices, err := s.BookChoices(ctx, BookChoicesQuery{SelectedID: id})
	if err != nil || choices.Selected == nil {
		t.Fatal(choices, err)
	}
}

func TestRestoreIgnoredBookConcurrentDecisions(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), nil)
	ctx := context.Background()
	var id string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title,status) values('ebook','Ignored','ignored') returning id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	before, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.RestoreBook(ctx, id, RestoreBookRequest{UpdatedAt: before.UpdatedAt, Monitored: true})
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success, stale := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, ErrRestoreBookChanged) {
			stale++
		} else {
			t.Fatal(e)
		}
	}
	after, err := s.Get(ctx, id)
	if err != nil || after.Status != "wanted" || !after.Monitored || success != 1 || stale != 1 {
		t.Fatal(after, success, stale, err)
	}
	db.Close()
	if _, err = s.RemovedBooks(ctx, RemovedBooksQuery{}); err == nil {
		t.Fatal("closed DB appeared empty")
	}
	if _, err = s.RestoreBook(ctx, "bad", RestoreBookRequest{}); !errors.Is(err, ErrRestoreBook) {
		t.Fatal(err)
	}
}
