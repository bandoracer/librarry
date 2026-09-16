package wanted

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

type noChoiceNetwork struct{ Acquisition }

func (noChoiceNetwork) LiveDownloadEvidence(context.Context, acquisition.DownloadListQuery) acquisition.DownloadEvidence {
	panic("book choice requested live client evidence")
}
func (noChoiceNetwork) Downloads(context.Context, acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error) {
	panic("book choice requested downloads")
}

func TestBookChoicesTenThousandAndPinnedIdentity(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	service := NewService(store, noChoiceNetwork{})
	ctx := context.Background()
	_, err := db.Exec(`insert into wanted_items(wanted_format,title,author_name,status,monitored,created_at) select case when i<=1500 then 'audiobook' else 'ebook' end,'Fixture '||i,'Writer',case when i=10002 then 'removed' when i=10003 then 'ignored' when i<=100 then 'imported' else 'wanted' end,false,'2026-01-01'::timestamptz from generate_series(1,10003)i`)
	if err != nil {
		t.Fatal(err)
	}
	q := BookChoicesQuery{Limit: 100}
	seen := map[string]bool{}
	var times []time.Duration
	var first BookChoices
	for {
		start := time.Now()
		p, e := service.BookChoices(ctx, q)
		times = append(times, time.Since(start))
		if e != nil {
			t.Fatal(e)
		}
		if p.Total != 10001 || p.Filtered != 10001 || p.ObservedAt.IsZero() || len(p.Books) > 100 {
			t.Fatal(p)
		}
		if first.NextCursor == "" {
			first = p
		}
		for _, book := range p.Books {
			if seen[book.ID] {
				t.Fatal("duplicate", book.ID)
			}
			seen[book.ID] = true
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
	t.Logf("10001 choices, 101 pages, p95 %s", times[len(times)*95/100])
	selected := first.Books[0]
	if _, err = store.UpdateWanted(ctx, selected.ID, WantedUpdateRequest{Title: "Owner title", AuthorName: "Owner author"}); err != nil {
		t.Fatal(err)
	}
	p, err := service.BookChoices(ctx, BookChoicesQuery{Search: "no match", SelectedID: selected.ID})
	if err != nil || p.Filtered != 0 || p.Selected == nil || p.Selected.Title != "Owner title" || p.Selected.AuthorName != "Owner author" || p.Books == nil {
		t.Fatal(p, err)
	}
	p, err = service.BookChoices(ctx, BookChoicesQuery{Search: "OWNER AUTHOR"})
	if err != nil || p.Filtered != 1 {
		t.Fatal(p, err)
	}
	for _, q := range []BookChoicesQuery{{Format: "audiobook", Cursor: first.NextCursor}, {Search: "other", Cursor: first.NextCursor}} {
		if _, err = service.BookChoices(ctx, q); !errors.Is(err, ErrBookChoices) {
			t.Fatal(q, err)
		}
	}
	// A selection is independent of paging/search but must remain active and format-compatible.
	p, err = service.BookChoices(ctx, BookChoicesQuery{SelectedID: selected.ID, Cursor: first.NextCursor, Limit: 1})
	if err != nil || p.Selected == nil || len(p.Books) != 1 || p.Books[0].ID == selected.ID {
		t.Fatal(p, err)
	}
	otherFormat := "ebook"
	if selected.Format == "ebook" {
		otherFormat = "audiobook"
	}
	p, err = service.BookChoices(ctx, BookChoicesQuery{SelectedID: selected.ID, Format: otherFormat})
	if err != nil || p.Selected != nil {
		t.Fatal(p, err)
	}
	anchor := first.Books[99].ID
	if _, err = db.Exec(`delete from wanted_items where id=$1`, anchor); err != nil {
		t.Fatal(err)
	}
	p, err = service.BookChoices(ctx, BookChoicesQuery{Cursor: first.NextCursor, Limit: 1})
	if err != nil || len(p.Books) != 1 || p.Books[0].ID >= anchor {
		t.Fatal(p, err)
	}
	if _, err = db.Exec(`update wanted_items set status='removed' where id=$1`, selected.ID); err != nil {
		t.Fatal(err)
	}
	p, err = service.BookChoices(ctx, BookChoicesQuery{SelectedID: selected.ID, Search: selected.ID})
	if err != nil || p.Selected != nil || p.Filtered != 0 {
		t.Fatal(p, err)
	}
	for _, search := range []string{"%", "_"} {
		p, err = service.BookChoices(ctx, BookChoicesQuery{Search: search})
		if err != nil || p.Filtered != 0 {
			t.Fatal(p, err)
		}
	}
}

func TestBookChoicesInvalidEmptyUnavailable(t *testing.T) {
	service := NewService(NewStore(testdb.Open(t)), nil)
	p, err := service.BookChoices(context.Background(), BookChoicesQuery{})
	if err != nil || p.Books == nil || p.Total != 0 || p.Selected != nil || p.NextCursor != "" {
		t.Fatal(p, err)
	}
	for _, q := range []BookChoicesQuery{{Format: "video"}, {Limit: -1}, {Limit: 101}, {Cursor: "bad"}, {SelectedID: "bad"}, {Search: strings.Repeat("a", 257)}} {
		if _, err = service.BookChoices(context.Background(), q); !errors.Is(err, ErrBookChoices) {
			t.Fatal(q, err)
		}
	}
	service.store.db.Close()
	for _, s := range []*Service{service, NewService(nil, nil), nil} {
		if _, err = s.BookChoices(context.Background(), BookChoicesQuery{}); err == nil {
			t.Fatal("unavailable choices looked empty")
		}
	}
}
