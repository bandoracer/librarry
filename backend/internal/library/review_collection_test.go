package library

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestImportReviewCollectionTenThousand(t *testing.T) {
	db := testdb.Open(t)
	service := NewService(NewStore(db), Config{}, nil, nil)
	ctx := context.Background()
	_, err := db.Exec(`insert into import_reviews(source_path,title,author_name,reason,status,media_format,metadata,created_at)
 select '/fixture/'||i, 'Title '||i, 'Writer', 'Needs match',
 case when i<=7000 then 'pending' when i<=8000 then 'imported' when i<=9000 then 'skipped' else 'rejected' end,
 case when i<=1500 then 'audiobook' when i=10001 then 'unknown' else 'ebook' end,
 case when i%2=0 then '{"payloadReview":true}'::jsonb else '{}'::jsonb end,
 '2026-01-01'::timestamptz from generate_series(1,10001)i`)
	if err != nil {
		t.Fatal(err)
	}
	q := ImportReviewQuery{Status: "all", Limit: 100}
	seen := map[string]bool{}
	var times []time.Duration
	var first ImportReviewCollection
	for {
		started := time.Now()
		p, e := service.ImportReviewCollection(ctx, q)
		times = append(times, time.Since(started))
		if e != nil {
			t.Fatal(e)
		}
		if p.Total != 10001 || p.Filtered != 10001 || p.Counts["pending"] != 7000 || p.Counts["resolved"] != 3001 || p.ObservedAt.IsZero() || len(p.Reviews) > 100 {
			t.Fatalf("invalid totals: %+v", p)
		}
		if first.NextCursor == "" {
			first = p
		}
		for _, r := range p.Reviews {
			if seen[r.ID] {
				t.Fatal("duplicate", r.ID)
			}
			seen[r.ID] = true
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
	t.Logf("10001 import reviews, 101 pages, p95 %s", times[len(times)*95/100])
	for _, tt := range []struct {
		q ImportReviewQuery
		n int
	}{
		{ImportReviewQuery{}, 7000}, {ImportReviewQuery{Status: "resolved"}, 3001},
		{ImportReviewQuery{Format: "audiobook"}, 1500}, {ImportReviewQuery{Kind: "payload"}, 3500},
		{ImportReviewQuery{Kind: "file"}, 3500}, {ImportReviewQuery{Status: "all", Format: "unknown"}, 1},
		{ImportReviewQuery{Search: " /fixture/10001 ", Status: "resolved"}, 1},
		{ImportReviewQuery{Search: "WRITER", Format: "audiobook", Kind: "payload"}, 750},
		{ImportReviewQuery{Search: "%"}, 0}, {ImportReviewQuery{Search: "_"}, 0},
	} {
		p, e := service.ImportReviewCollection(ctx, tt.q)
		if e != nil || p.Filtered != tt.n || p.Total != 10001 {
			t.Fatalf("query %+v: filtered=%d total=%d error=%v", tt.q, p.Filtered, p.Total, e)
		}
	}
	for _, q := range []ImportReviewQuery{
		{Cursor: first.NextCursor}, {Status: "all", Format: "ebook", Cursor: first.NextCursor},
		{Status: "all", Kind: "file", Cursor: first.NextCursor}, {Status: "all", Search: "Title", Cursor: first.NextCursor},
	} {
		if _, e := service.ImportReviewCollection(ctx, q); !errors.Is(e, ErrImportReviewPage) {
			t.Fatal("accepted different filter", q, e)
		}
	}
	// Updated progress must not move records; a removed anchor must not break its cursor.
	anchor := first.Reviews[len(first.Reviews)-1].ID
	if _, err = db.Exec(`update import_reviews set updated_at=now()`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`delete from import_reviews where id=$1`, anchor); err != nil {
		t.Fatal(err)
	}
	next, e := service.ImportReviewCollection(ctx, ImportReviewQuery{Status: "all", Limit: 1, Cursor: first.NextCursor})
	if e != nil || len(next.Reviews) != 1 || next.Reviews[0].ID >= anchor || next.Total != 10000 {
		t.Fatal(next, e)
	}
}

func TestImportReviewCollectionEmptyUnavailableAndInvalid(t *testing.T) {
	db := testdb.Open(t)
	service := NewService(NewStore(db), Config{}, nil, nil)
	page, err := service.ImportReviewCollection(context.Background(), ImportReviewQuery{})
	if err != nil || page.Reviews == nil || len(page.Reviews) != 0 || page.Total != 0 || page.NextCursor != "" {
		t.Fatal(page, err)
	}
	for _, q := range []ImportReviewQuery{{Status: "imported"}, {Format: "video"}, {Kind: "folder"}, {Limit: -1}, {Limit: 101}, {Search: strings.Repeat("a", 257)}, {Cursor: "invalid"}} {
		if _, err = service.ImportReviewCollection(context.Background(), q); !errors.Is(err, ErrImportReviewPage) {
			t.Fatal(q, err)
		}
	}
	db.Close()
	for _, s := range []*Service{service, NewService(nil, Config{}, nil, nil), nil} {
		if _, err = s.ImportReviewCollection(context.Background(), ImportReviewQuery{}); err == nil {
			t.Fatal("unavailable collection returned empty success")
		}
	}
}
