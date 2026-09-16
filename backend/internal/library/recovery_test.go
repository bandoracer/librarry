package library

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestRecoveryPagesTraverseLargeCollections(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), Config{}, nil, nil)
	ctx := context.Background()
	const count = 10001
	// Tied creation times exercise both identity tie-breakers. All data is synthetic.
	for _, query := range []string{
		`insert into import_operations(source_kind,request_key,source_root,destination_root,media_format,import_mode,state,cleanup_state,created_at) select 'manual','fixture-'||i,'/fixture/source','/fixture/library','ebook','copy',case when i%2=0 then 'committed' else 'failed' end,case when i%2=0 then 'cleaned' else 'blocked' end,'2026-01-01' from generate_series(1,10001) i`,
		`insert into import_operation_files(operation_id,file_order,relative_path,source_path,destination_path,size_bytes,sha256,media_format,state) select id,n,'file-'||n,'/fixture/source/file-'||n,'/fixture/library/'||id||'/file-'||n,1,repeat('a',64),'ebook','verified' from import_operations cross join generate_series(1,2) n`,
		`insert into root_folders(name,path,media_format) values('Fixture','/fixture/calibre','ebook')`,
		`insert into calibre_handoffs(source_path,root_folder_id,phase,plan,created_at) select '/fixture/'||i,(select id from root_folders limit 1),case when i%2=0 then 'committed' else 'uploading' end,'{}','2026-01-01' from generate_series(1,10001) i`,
		`insert into files(media_format,path) select 'ebook','/fixture/legacy-'||i from generate_series(1,5001) i`,
		`insert into import_reconciliation_issues(file_id,kind,reason,created_at) select id,k,'Fixture unresolved link','2026-01-01' from files cross join (values('wanted'),('download')) kinds(k)`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	seenOps, seenCalibre, seenIssues := map[string]bool{}, map[string]bool{}, map[string]bool{}
	q := ImportRecoveryQuery{Limit: 100}
	durations := []time.Duration{}
	for page := 0; ; page++ {
		started := time.Now()
		r, err := s.ImportRecoveryPage(ctx, q)
		durations = append(durations, time.Since(started))
		if err != nil {
			t.Fatal(err)
		}
		if r.OperationsPage.Total != count || r.CalibrePage.Total != count || r.IssuesPage.Total != 10002 || r.Unfinished != 5001 || r.CalibreUnfinished != 5001 {
			t.Fatalf("wrong counts: %+v", r.OperationsPage)
		}
		for _, op := range r.Operations {
			if seenOps[op.ID] {
				t.Fatal("duplicate operation")
			}
			seenOps[op.ID] = true
		}
		for _, h := range r.CalibreHandoffs {
			if seenCalibre[h.ID] {
				t.Fatal("duplicate handoff")
			}
			seenCalibre[h.ID] = true
		}
		for _, i := range r.Issues {
			key := i.FileID + ":" + i.Kind
			if seenIssues[key] {
				t.Fatal("duplicate issue")
			}
			seenIssues[key] = true
		}
		if page == 0 {
			// Updating progress must not reorder any of the remaining rows.
			if _, err = db.Exec(`update import_operations set updated_at=now(); update calibre_handoffs set updated_at=now()`); err != nil {
				t.Fatal(err)
			}
		}
		if r.OperationsPage.NextCursor == "" && r.CalibrePage.NextCursor == "" && r.IssuesPage.NextCursor == "" {
			break
		}
		if page > 101 {
			t.Fatal("cursor did not terminate")
		}
		q.OperationsCursor = r.OperationsPage.NextCursor
		q.CalibreCursor = r.CalibrePage.NextCursor
		q.IssuesCursor = r.IssuesPage.NextCursor
	}
	if len(seenOps) != count || len(seenCalibre) != count || len(seenIssues) != 10002 {
		t.Fatalf("missing rows: %d/%d/%d", len(seenOps), len(seenCalibre), len(seenIssues))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("10,001 native + 10,001 Calibre + 10,002 issues: %d pages; page p95 %s", len(durations), durations[len(durations)*95/100])
	r, err := s.ImportRecoveryPage(ctx, ImportRecoveryQuery{Limit: 100, UnfinishedOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.OperationsPage.Total != 5001 || r.CalibrePage.Total != 5001 || r.IssuesPage.Total != 10002 {
		t.Fatal("filtered totals")
	}
	for _, op := range r.Operations {
		if op.State == "committed" {
			t.Fatal("completed operation in unfinished filter")
		}
	}
	for _, h := range r.CalibreHandoffs {
		if h.Phase == "committed" {
			t.Fatal("completed handoff in unfinished filter")
		}
	}
	for _, bad := range []ImportRecoveryQuery{{Limit: 101}, {Limit: -1}, {OperationsCursor: "garbage"}, {OperationsCursor: r.CalibrePage.NextCursor, UnfinishedOnly: true}, {OperationsCursor: r.OperationsPage.NextCursor}} {
		if _, err = s.ImportRecoveryPage(ctx, bad); err != ErrInvalidRecoveryQuery {
			t.Fatalf("bad query accepted: %+v %v", bad, err)
		}
	}
}

func TestRecoveryObservationsNeverInferFailureFromLease(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), Config{}, nil, nil)
	ctx := context.Background()
	for i, lease := range []string{"null", "now()+interval '2 minutes'", "now()-interval '2 minutes'"} {
		_, err := db.Exec(fmt.Sprintf(`insert into import_operations(source_kind,request_key,source_root,destination_root,media_format,import_mode,state,lease_expires_at,updated_at) values('manual',$1,'/fixture/source','/fixture/library','ebook','copy','transferring',%s,now()-interval '1 day')`, lease), fmt.Sprint(i))
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`insert into import_operation_files(operation_id,file_order,relative_path,source_path,destination_path,size_bytes,sha256,media_format,state) select id,1,'book.epub','/fixture/source/book.epub','/fixture/library/book.epub',1,repeat('a',64),'ebook','verified' from import_operations`); err != nil {
		t.Fatal(err)
	}
	r, err := s.ImportRecovery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]bool{}
	for _, op := range r.Operations {
		obs := op.Recovery
		if op.State != "transferring" || obs == nil || obs.TotalFiles != 1 || obs.VerifiedFiles != 1 || !obs.RecordedAt.After(op.UpdatedAt) || obs.ObservedAt.IsZero() {
			t.Fatalf("incorrect observation: %+v %+v", op, obs)
		}
		states[obs.LeaseState] = true
	}
	if !states["held"] || !states["expired"] || !states["none"] {
		t.Fatal(states)
	}
	if _, err = db.Exec(`update import_operations set state='committed',cleanup_state='blocked'`); err != nil {
		t.Fatal(err)
	}
	r, err = s.ImportRecoveryPage(ctx, ImportRecoveryQuery{UnfinishedOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Unfinished != 3 {
		t.Fatal("unfinished manual cleanup hidden")
	}
	for _, op := range r.Operations {
		if op.Recovery.LeasePurpose != "cleanup" || op.Recovery.LeaseState == "not_applicable" {
			t.Fatal("cleanup lease hidden after transfer commits")
		}
	}
	if _, err = db.Exec(`update import_operations set cleanup_state='cleaned'`); err != nil {
		t.Fatal(err)
	}
	r, err = s.ImportRecovery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range r.Operations {
		if op.Recovery.LeaseState != "not_applicable" || op.Recovery.LeasePurpose != "none" || op.Recovery.LeaseExpiresAt != nil {
			t.Fatal("finished operation reports ownership")
		}
	}
}

func TestRecoveryCursorSurvivesDeletedAnchorAndIgnoresNewerRows(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), Config{}, nil, nil)
	ctx := context.Background()
	for _, day := range []string{"01", "02", "03"} {
		if _, err := db.Exec(`insert into import_operations(source_kind,request_key,source_root,destination_root,media_format,import_mode,created_at) values('manual',$1,'/fixture/source','/fixture/library','ebook','copy',$2)`, day, "2026-01-"+day); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.ImportRecoveryPage(ctx, ImportRecoveryQuery{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Operations) != 1 || first.Operations[0].RequestKey != "03" {
		t.Fatal("wrong first page")
	}
	if _, err = db.Exec(`delete from import_operations where request_key='03';update import_operations set updated_at=now(),state='committed',cleanup_state='cleaned' where request_key='02';insert into import_operations(source_kind,request_key,source_root,destination_root,media_format,import_mode) values('manual','new','/fixture/source','/fixture/library','ebook','copy')`); err != nil {
		t.Fatal(err)
	}
	next, err := s.ImportRecoveryPage(ctx, ImportRecoveryQuery{Limit: 1, OperationsCursor: first.OperationsPage.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Operations) != 1 || next.Operations[0].RequestKey != "02" || next.OperationsPage.Total != 3 {
		t.Fatalf("unstable page: %+v", next)
	}
}
