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

func TestFileCollectionTenThousandFiles(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	testdb.SeedRange(t, db, 10001, `insert into files(media_format,path,title,author_name,import_status,presence_state,updated_at) select case when i<=1500 then 'audiobook' else 'ebook' end,'/collection/'||lpad(i::text,6,'0'),'Title '||(i%5),'Writer',case when i<=1500 then 'imported' else 'available' end,case when i=10000 then 'missing' when i=10001 then 'unknown' else 'present' end,'2026-01-01'::timestamptz from generate_series($1::integer,$2::integer) i`)
	var err error
	var audioID, ebookID string
	if err = db.QueryRow(`insert into wanted_items(wanted_format,title,status) values('audiobook','Large chapter set','imported') returning id::text`).Scan(&audioID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`insert into wanted_items(wanted_format,title,status) values('ebook','Recorded ebooks','imported') returning id::text`).Scan(&ebookID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into file_wanted_links(file_id,wanted_item_id) select id,case when media_format='audiobook' then $1::uuid else $2::uuid end from files`, audioID, ebookID); err != nil {
		t.Fatal(err)
	}
	var durations []time.Duration
	pages := 0
	for _, order := range []string{"path", "title", "updated"} {
		q := FileCollectionQuery{Sort: order, Limit: 100}
		seen := map[string]bool{}
		firstCursor := ""
		previous := ""
		for {
			start := time.Now()
			page, e := store.FileCollection(ctx, q)
			durations = append(durations, time.Since(start))
			pages++
			if e != nil {
				t.Fatal(e)
			}
			if page.Total != 10001 || page.Filtered != 10001 || page.Counts["audiobook"] != 1500 || page.Counts["present"] != 9999 || page.Counts["unknown"] != 1 || len(page.Files) > 100 {
				t.Fatal(page)
			}
			for _, f := range page.Files {
				if seen[f.ID] || f.WantedIDs == nil {
					t.Fatal("duplicate or null links", f.ID)
				}
				seen[f.ID] = true
				if order == "path" && f.Path < previous {
					t.Fatal("path order", f.Path, previous)
				}
				previous = f.Path
			}
			if firstCursor == "" {
				firstCursor = page.NextCursor
			}
			if page.NextCursor == "" {
				break
			}
			q.Cursor = page.NextCursor
		}
		if len(seen) != 10001 {
			t.Fatal(order, len(seen))
		}
		if _, e := store.FileCollection(ctx, FileCollectionQuery{Sort: order, Cursor: firstCursor, Format: "audiobook"}); !errors.Is(e, ErrFilePage) {
			t.Fatal(e)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("10001 files, %d pages, all three sorts, p95 %s", pages, durations[len(durations)*95/100])
	scoped := FileCollectionQuery{WantedID: audioID, Limit: 100}
	chapters := 0
	for {
		p, e := store.FileCollection(ctx, scoped)
		if e != nil || p.Total != 1500 || p.Filtered != 1500 {
			t.Fatal(p, e)
		}
		for _, file := range p.Files {
			if len(file.WantedIDs) != 1 || file.WantedIDs[0] != audioID {
				t.Fatal(file)
			}
			chapters++
		}
		if p.NextCursor == "" {
			break
		}
		scoped.Cursor = p.NextCursor
	}
	if chapters != 1500 {
		t.Fatal("truncated chapter set", chapters)
	}
	for _, test := range []struct {
		q FileCollectionQuery
		n int
	}{{FileCollectionQuery{Format: "audiobook"}, 1500}, {FileCollectionQuery{Presence: "missing"}, 1}, {FileCollectionQuery{Search: "000999"}, 1}, {FileCollectionQuery{Search: "%"}, 0}} {
		p, e := store.FileCollection(ctx, test.q)
		if e != nil || p.Total != 10001 || p.Filtered != test.n {
			t.Fatal(test, p, e)
		}
	}
}

func TestFileCollectionUsesRecordedLinksAndHidesUncommittedFiles(t *testing.T) {
	service, db, _, wantedID := operationFixture(t)
	ctx := context.Background()
	var other string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title) values('ebook','Other') returning id::text`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	file, err := service.TrackFile(ctx, FileRecord{Path: "/fixture/recorded.epub", MediaFormat: "ebook", Title: "Owner title", Metadata: map[string]any{"wantedId": other}})
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately leave a stale JSON hint; the relational correction wins.
	if _, err = db.Exec(`delete from file_wanted_links where file_id=$1`, file.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into file_wanted_links(file_id,wanted_item_id) values($1,$2)`, file.ID, wantedID); err != nil {
		t.Fatal(err)
	}
	p, err := service.FileCollection(ctx, FileCollectionQuery{WantedID: wantedID})
	if err != nil || p.Total != 1 || p.Files[0].WantedIDs[0] != wantedID || p.Files[0].Title != "Owner title" || p.Files[0].PresenceState != "unknown" {
		t.Fatal(p, err)
	}
	p, err = service.FileCollection(ctx, FileCollectionQuery{WantedID: other})
	if err != nil || p.Total != 0 {
		t.Fatal(p, err)
	}
	if _, err = db.Exec(`insert into file_wanted_links(file_id,wanted_item_id) values($1,$2)`, file.ID, other); err != nil {
		t.Fatal(err)
	}
	p, err = service.FileCollection(ctx, FileCollectionQuery{})
	if err != nil || p.Total != 1 || len(p.Files[0].WantedIDs) != 2 {
		t.Fatal(p, err)
	}
	var operationID string
	err = db.QueryRow(`insert into import_operations(download_record_id,wanted_item_id,source_root,destination_root,media_format,import_mode) select id,$1,'/source','/fixture','ebook','copy' from downloads limit 1 returning id::text`, wantedID).Scan(&operationID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into import_operation_files(operation_id,file_order,relative_path,source_path,destination_path,size_bytes,sha256,media_format,file_id) values($1,1,'recorded.epub','/source/recorded.epub',$2,1,repeat('a',64),'ebook',$3)`, operationID, file.Path, file.ID); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"planned", "transferring", "verified", "failed", "review"} {
		if _, err = db.Exec(`update import_operations set state=$1 where id=$2`, state, operationID); err != nil {
			t.Fatal(err)
		}
		p, e := service.FileCollection(ctx, FileCollectionQuery{})
		if e != nil || p.Total != 0 || len(p.Files) != 0 {
			t.Fatal(state, p, e)
		}
	}
	if _, err = db.Exec(`update import_operations set state='committed' where id=$1`, operationID); err != nil {
		t.Fatal(err)
	}
	p, err = service.FileCollection(ctx, FileCollectionQuery{})
	if err != nil || p.Total != 1 {
		t.Fatal(p, err)
	}
}

func TestFileCollectionValidatesBeforePersistence(t *testing.T) {
	service := NewService(nil, Config{}, nil, nil)
	for _, q := range []FileCollectionQuery{{Limit: 101}, {Limit: -1}, {WantedID: "not-an-id"}, {Format: "video"}, {Presence: "unavailable"}, {Sort: "random"}, {Cursor: "bad"}, {Search: strings.Repeat("x", 257)}} {
		if _, e := service.FileCollection(context.Background(), q); !errors.Is(e, ErrFilePage) {
			t.Fatal(q, e)
		}
	}
	if _, e := service.FileCollection(context.Background(), FileCollectionQuery{}); e == nil {
		t.Fatal("unavailable database reported success")
	}
}
