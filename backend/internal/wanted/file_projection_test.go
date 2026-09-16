package wanted

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/database"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestFileProjectionUpgradePreservesExistingManifestEvidence(t *testing.T) {
	db := testdb.OpenThrough(t, "0042_worker_check_fairness.sql")
	book := evidenceBook(t, db, "Existing chapter set", "audiobook")
	a := evidenceFile(t, db, book, "/library/existing-a.mp3", "present")
	b := evidenceFile(t, db, book, "/library/existing-b.mp3", "missing")
	op := evidenceManifest(t, db, book, a, b)
	if err := database.ApplyMigrations(context.Background(), db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	result, err := NewStore(db).WantedFileEvidence(context.Background(), []string{book.ID})
	if err != nil || result[book.ID].State != "incomplete" || result[book.ID].RequiredFiles != 2 || result[book.ID].PresentFiles != 1 {
		t.Fatal(result, err)
	}
	var linked int
	if err := db.QueryRow(`select count(*) from import_operation_files where operation_id=$1 and file_id in ($2,$3) and wanted_item_id=$4`, op, a, b, book.ID).Scan(&linked); err != nil || linked != 2 {
		t.Fatal("upgrade changed manifest identities", linked, err)
	}
}

func TestFileProjectionCollectionCountsAndStablePaging(t *testing.T) {
	db := testdb.Open(t)
	if _, err := db.Exec(`insert into wanted_items(id,wanted_format,title) select md5(n::text)::uuid,'ebook','Tied title' from generate_series(1,10001)n`); err != nil {
		t.Fatal(err)
	}
	testdb.SeedRange(t, db, 10001, `insert into files(media_format,path,size_bytes,presence_state,import_status,metadata)
 select 'ebook','/library/'||n||'.epub',10,case n%3 when 0 then 'present' when 1 then 'missing' else 'unknown' end,'imported',jsonb_build_object('wantedId',(md5(n::text)::uuid)::text) from generate_series($1::integer,$2::integer)n`)
	if _, err := db.Exec(`analyze wanted_items; analyze files; analyze file_wanted_links;`); err != nil {
		t.Fatal(err)
	}

	var present, missing, unknown, total int
	if err := db.QueryRow(`select count(*) filter(where file_state='present'),count(*) filter(where file_state='missing'),count(*) filter(where file_state='unknown'),count(*) from librarry_book_file_evidence(null)`).Scan(&present, &missing, &unknown, &total); err != nil || total != 10001 || present != 3333 || missing != 3334 || unknown != 3334 {
		t.Fatal(present, missing, unknown, total, err)
	}
	if err := db.QueryRow(`select count(*) from librarry_book_file_evidence('{}')`).Scan(&total); err != nil || total != 0 {
		t.Fatal("empty scope broadened", total, err)
	}
	seen := map[string]bool{}
	var cursor string
	var durations []time.Duration
	for {
		started := time.Now()
		rows, err := db.Query(`select wanted_id from librarry_book_file_evidence(null) where file_state='present' and ($1='' or wanted_id>nullif($1,'')::uuid) order by wanted_id limit 100`, cursor)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			if seen[id] || id <= cursor {
				t.Fatal("duplicate or unordered page", id, cursor)
			}
			seen[id] = true
			cursor = id
			n++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		durations = append(durations, time.Since(started))
		if n == 0 {
			break
		}
	}
	if len(seen) != 3333 {
		t.Fatal("unreachable books", len(seen))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("10,001-record SQL file-state paging: %d pages, p95 %s (database only; not full API latency)", len(durations), durations[(len(durations)*95-1)/100])
}
