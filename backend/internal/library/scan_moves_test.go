package library

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestScanMoveRetainsImportedIdentityLinksOverridesAndHistory(t *testing.T) {
	s, db, download, wantedID := operationFixture(t)
	ctx := context.Background()
	imported, err := s.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || imported.Imported != 1 {
		t.Fatal(imported, err)
	}
	file := imported.Results[0].Import.File
	root := s.Config().EbookRoot
	if _, err := db.Exec(`update files set title='Manually chosen title',author_name='Manual author',metadata=metadata||'{"manualOverride":true}' where id=$1`, file.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	var metadata string
	if err := db.QueryRow(`select metadata::text from files where id=$1`, file.ID).Scan(&metadata); err != nil {
		t.Fatal(err)
	}
	op, err := s.store.operationForDownload(ctx, download.Client, download.ID)
	if err != nil {
		t.Fatal(err)
	}
	movedPath := filepath.Join(root, "renamed.epub")
	if err := os.Rename(file.Path, movedPath); err != nil {
		t.Fatal(err)
	}
	scan, err := s.Scan(ctx, ScanRequest{Root: root})
	if err != nil || scan.State != "completed" || scan.Moved != 1 || scan.Missing != 0 {
		t.Fatal(scan, err)
	}
	if len(scan.Files) != 1 || scan.Files[0].ID != file.ID {
		t.Fatal("scan returned a discarded discovery ID", scan.Files)
	}
	var id, title, author, path, source, actualMeta string
	if err := db.QueryRow(`select id::text,title,author_name,path,source_path,metadata::text from files`).Scan(&id, &title, &author, &path, &source, &actualMeta); err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(movedPath)
	if id != file.ID || title != "Manually chosen title" || author != "Manual author" || path != canonical || source != file.SourcePath || actualMeta != metadata {
		t.Fatal(id, title, author, path, source, actualMeta)
	}
	for query, want := range map[string]int{
		`select count(*) from files`: 1, `select count(*) from file_wanted_links where wanted_item_id='` + wantedID + `'`: 1,
		`select count(*) from file_download_links where file_id='` + file.ID + `'`: 1,
		`select count(*) from downloads where imported_file_id='` + file.ID + `'`:  1,
		`select count(*) from library_scan_moves where file_id='` + file.ID + `'`:  1,
	} {
		var n int
		if err := db.QueryRow(query).Scan(&n); err != nil || n != want {
			t.Fatal(query, n, err)
		}
	}
	after, err := s.store.getOperation(ctx, op.ID)
	if err != nil || after.Files[0].DestinationPath != op.Files[0].DestinationPath || after.Files[0].FileID != file.ID || after.Files[0].SHA256 != op.Files[0].SHA256 {
		t.Fatal(after, err)
	}
	// Original cleanup receipts cannot follow a moved file implicitly.
	if err := s.VerifyCompletedDownload(ctx, download, nil); err == nil {
		t.Fatal("move silently authorized old-destination cleanup")
	}
	if _, err := os.Stat(filepath.Join(download.SavePath, download.Name)); err != nil {
		t.Fatal("download source lost", err)
	}
	repeat, err := s.Scan(ctx, ScanRequest{Root: root})
	if err != nil || repeat.Moved != 0 || repeat.Missing != 0 {
		t.Fatal(repeat, err)
	}
}

func movedScanFixture(t *testing.T) (*Service, string, string, string) {
	t.Helper()
	db := testdb.Open(t)
	root := t.TempDir()
	old := filepath.Join(root, "old.epub")
	next := filepath.Join(root, "new.epub")
	if err := os.WriteFile(old, []byte("controlled moved-file content"), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewService(NewStore(db), Config{}, nil, nil)
	out, err := s.Scan(context.Background(), ScanRequest{Root: root})
	if err != nil || len(out.Files) != 1 {
		t.Fatal(out, err)
	}
	if err := os.Rename(old, next); err != nil {
		t.Fatal(err)
	}
	return s, root, out.Files[0].ID, next
}

// Stop after discovery and absence staging, before the atomic completion step.
func stageMoveForTest(t *testing.T, s *Service, root string) ScanJob {
	t.Helper()
	job, err := s.StartScan(context.Background(), ScanRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		out, err := s.runScanBatch(context.Background(), job.ID, 1)
		if err != nil || out.State == "completed" {
			t.Fatal("move completed before staged check", out, err)
		}
		var n int
		if err := s.store.db.QueryRow(`select count(*) from library_scan_absent where job_id=$1`, job.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			return job
		}
	}
	t.Fatal("no staged absence")
	return job
}
func TestScanMoveWaitsForCommitAndResumesAfterFailure(t *testing.T) {
	s, root, id, _ := movedScanFixture(t)
	ctx := context.Background()
	job := stageMoveForTest(t, s, root)
	if _, err := s.store.db.Exec(`alter table library_scan_moves add constraint injected_move_failure check(false)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runScanBatch(ctx, job.ID, 100); err == nil {
		t.Fatal("expected failed commit")
	}
	var count int
	var path, presence string
	if err := s.store.db.QueryRow(`select count(*) from files`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	if err := s.store.db.QueryRow(`select path,presence_state from files where id=$1`, id).Scan(&path, &presence); err != nil || !strings.HasSuffix(path, "old.epub") || presence != "present" {
		t.Fatal(path, presence, err)
	}
	if _, err := s.store.db.Exec(`alter table library_scan_moves drop constraint injected_move_failure`); err != nil {
		t.Fatal(err)
	}
	restarted := NewService(s.store, Config{}, nil, nil)
	if _, err := restarted.RetryScan(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	out := finishScanForTest(t, restarted, job.ID)
	if out.Moved != 1 || out.Missing != 0 {
		t.Fatal(out)
	}
}
func TestScanMoveCancellationLeavesIdentitiesUnchanged(t *testing.T) {
	s, root, id, _ := movedScanFixture(t)
	job := stageMoveForTest(t, s, root)
	if _, err := s.CancelScan(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	var path string
	if err := s.store.db.QueryRow(`select path from files where id=$1`, id).Scan(&path); err != nil || !strings.HasSuffix(path, "old.epub") {
		t.Fatal(path, err)
	}
	// An untouched discovery from the cancelled job can participate next time.
	next, err := s.Scan(context.Background(), ScanRequest{Root: root})
	if err != nil || next.Moved != 1 {
		t.Fatal(next, err)
	}
}
func TestScanMoveRetainsAmbiguousAndManuallyChangedDiscoveries(t *testing.T) {
	for _, variant := range []string{"copy", "title", "metadata", "assignment", "calibre", "calibreRoot", "inode", "content"} {
		t.Run(variant, func(t *testing.T) {
			s, root, id, next := movedScanFixture(t)
			ctx := context.Background()
			if variant == "copy" {
				b, err := os.ReadFile(next)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "copy.epub"), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			job := stageMoveForTest(t, s, root)
			switch variant {
			case "title":
				_, err := s.store.db.Exec(`update files set title='Manual correction' where id<>$1`, id)
				if err != nil {
					t.Fatal(err)
				}
			case "metadata":
				_, err := s.store.db.Exec(`update files set metadata=metadata||'{"manualOverride":true}' where id<>$1`, id)
				if err != nil {
					t.Fatal(err)
				}
			case "assignment":
				_, err := s.store.db.Exec(`with w as(insert into wanted_items(wanted_format,title) values('ebook','Assigned') returning id) insert into file_wanted_links(file_id,wanted_item_id) select f.id,w.id from files f,w where f.id<>$1`, id)
				if err != nil {
					t.Fatal(err)
				}
			case "calibre":
				_, err := s.store.db.Exec(`update files set metadata=metadata||'{"calibreId":1}' where id=$1`, id)
				if err != nil {
					t.Fatal(err)
				}
			case "calibreRoot":
				_, err := s.store.db.Exec(`insert into root_folders(name,path,media_format,use_calibre) values('Calibre',$1,'ebook',true)`, root)
				if err != nil {
					t.Fatal(err)
				}
			case "inode":
				b, err := os.ReadFile(next)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(next, next+".retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(next, b, 0600); err != nil {
					t.Fatal(err)
				}
			case "content":
				if err := os.WriteFile(next, []byte("modified moved-file content!!"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			out, err := s.runScanBatch(ctx, job.ID, 100)
			if variant == "inode" || variant == "content" {
				if err == nil {
					t.Fatal("changed filesystem observation accepted", out)
				}
			} else if err != nil || out.Moved != 0 || out.Missing != 1 {
				t.Fatal(out, err)
			}
			var path string
			if err := s.store.db.QueryRow(`select path from files where id=$1`, id).Scan(&path); err != nil || !strings.HasSuffix(path, "old.epub") {
				t.Fatal(path, err)
			}
		})
	}
}

func TestScanMoveRechecksIdentityAndFilesystemAtCommit(t *testing.T) {
	for _, variant := range []string{"assignment", "originalOverride", "destinationReplaced", "sourceReappeared", "cancel"} {
		t.Run(variant, func(t *testing.T) {
			s, root, id, next := movedScanFixture(t)
			ctx := context.Background()
			queued := stageMoveForTest(t, s, root)
			job, err := s.claimScan(ctx, queued.ID)
			if err != nil {
				t.Fatal(err)
			}
			moves, err := s.prepareScanMoves(ctx, job)
			if err != nil || len(moves) != 1 {
				t.Fatal(moves, err)
			}
			switch variant {
			case "assignment":
				_, err = s.store.db.Exec(`with w as(insert into wanted_items(wanted_format,title) values('ebook','Concurrent assignment') returning id) insert into file_wanted_links(file_id,wanted_item_id) select $1,w.id from w`, moves[0].DiscoveredID)
			case "originalOverride":
				_, err = s.store.db.Exec(`update files set title='Concurrent original correction',updated_at=now() where id=$1`, id)
			case "destinationReplaced":
				err = os.Rename(next, next+".retained")
				if err == nil {
					err = os.WriteFile(next, []byte("controlled moved-file content"), 0600)
				}
			case "sourceReappeared":
				err = os.WriteFile(filepath.Join(root, "old.epub"), []byte("controlled moved-file content"), 0600)
			case "cancel":
				_, err = s.CancelScan(ctx, job.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			err = s.scanTransaction(ctx, job, func(tx *sql.Tx) error { var err error; n, err = s.commitScanMoves(ctx, tx, job, moves); return err })
			if n != 0 {
				t.Fatal("stale move was applied", n)
			}
			if variant == "assignment" || variant == "originalOverride" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("changed move proof or cancellation accepted")
			}
			var path string
			if err := s.store.db.QueryRow(`select path from files where id=$1`, id).Scan(&path); err != nil || !strings.HasSuffix(path, "old.epub") {
				t.Fatal(path, err)
			}
		})
	}
}

func TestScanMoveHistoryPaginatesAndSurvivesFileRemoval(t *testing.T) {
	s, root, _, _ := movedScanFixture(t)
	ctx := context.Background()
	out, err := s.Scan(ctx, ScanRequest{Root: root})
	if err != nil || out.Moved != 1 {
		t.Fatal(out, err)
	}
	history, err := s.ScanMoveHistory(ctx, out.JobID, "")
	if err != nil || len(history.Moves) != 1 || history.NextCursor != "" {
		t.Fatal(history, err)
	}
	// Historical IDs and paths remain reviewable after later deliberate deletion.
	if _, err := s.store.db.Exec(`delete from files`); err != nil {
		t.Fatal(err)
	}
	history, err = s.ScanMoveHistory(ctx, out.JobID, "")
	if err != nil || len(history.Moves) != 1 {
		t.Fatal(history, err)
	}
	if _, err := s.store.db.Exec(`insert into library_scan_moves(job_id,file_id,discovered_file_id,previous_path,current_path,sha256,size_bytes) select $1,gen_random_uuid(),gen_random_uuid(),'/old/'||n,'/new/'||n,repeat('a',64),10 from generate_series(1,201)n`, out.JobID); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 3; i++ {
		page, err := s.ScanMoveHistory(ctx, out.JobID, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range page.Moves {
			if seen[m.FileID] {
				t.Fatal("duplicate history", m)
			}
			seen[m.FileID] = true
		}
		cursor = page.NextCursor
	}
	if len(seen) != 202 || cursor != "" {
		t.Fatal(len(seen), cursor)
	}
	if _, err := s.ScanMoveHistory(ctx, out.JobID, "invalid"); err != ErrScanMoveCursor {
		t.Fatal(err)
	}
}

func TestScanMovesAcrossConfiguredRootsAndRestartedBatches(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	oldRoot := t.TempDir()
	newRoot := t.TempDir()
	for _, root := range []string{oldRoot, newRoot} {
		if _, err := db.Exec(`insert into root_folders(name,path,media_format) values('Fixture',$1,'audiobook')`, root); err != nil {
			t.Fatal(err)
		}
	}
	createScanFiles(t, oldRoot, 205)
	s := NewService(NewStore(db), Config{}, nil, nil)
	first, err := s.Scan(ctx, ScanRequest{Format: "audiobook"})
	if err != nil || first.Scanned != 205 {
		t.Fatal(first, err)
	}
	var originalIDs string
	if err := db.QueryRow(`select string_agg(id::text,',' order by id) from files`).Scan(&originalIDs); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(oldRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(oldRoot, entry.Name()), filepath.Join(newRoot, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	started, err := s.Scan(ctx, ScanRequest{Format: "audiobook", Limit: 20})
	if err != nil || !started.HasMore || started.Moved != 0 {
		t.Fatal(started, err)
	}
	restarted := NewService(NewStore(db), Config{}, nil, nil)
	final := finishScanForTest(t, restarted, started.JobID)
	if final.Moved != 205 || final.Missing != 0 {
		t.Fatal(final)
	}
	var retainedIDs string
	if err := db.QueryRow(`select string_agg(id::text,',' order by id) from files`).Scan(&retainedIDs); err != nil || retainedIDs != originalIDs {
		t.Fatal("original identities lost", err)
	}
}

func TestScanMoveWaitsForConcurrentAssignmentAndRetainsIt(t *testing.T) {
	s, root, _, _ := movedScanFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	queued := stageMoveForTest(t, s, root)
	job, err := s.claimScan(ctx, queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	moves, err := s.prepareScanMoves(ctx, job)
	if err != nil || len(moves) != 1 {
		t.Fatal(moves, err)
	}
	// An uncommitted FK assignment holds a key-share lock on the discovery.
	assignment, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer assignment.Rollback()
	if _, err := assignment.ExecContext(ctx, `with w as(insert into wanted_items(wanted_format,title) values('ebook','Concurrent book') returning id) insert into file_wanted_links(file_id,wanted_item_id) select $1,w.id from w`, moves[0].DiscoveredID); err != nil {
		t.Fatal(err)
	}
	complete := make(chan error, 1)
	go func() {
		complete <- s.scanTransaction(ctx, job, func(tx *sql.Tx) error {
			n, err := s.commitScanMoves(ctx, tx, job, moves)
			if err == nil && n != 0 {
				return fmt.Errorf("concurrent assignment was discarded by %d moves", n)
			}
			return err
		})
	}()
	// Confirm an actual database wait, rather than relying on goroutine timing.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		err := s.store.db.QueryRowContext(ctx, `select exists(select 1 from pg_stat_activity where datname=current_database() and wait_event_type='Lock' and query like 'select id::text from files%for update')`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-complete:
			t.Fatal("move did not wait for assignment", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("move did not reach the row fence")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := assignment.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-complete; err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.store.db.QueryRow(`select count(*) from file_wanted_links where file_id=$1`, moves[0].DiscoveredID).Scan(&count); err != nil || count != 1 {
		t.Fatal("manual association lost", count, err)
	}
}
