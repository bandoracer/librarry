package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func createScanFiles(t *testing.T, root string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("Book-%05d.mp3", i)), []byte(fmt.Sprintf("controlled audio %d", i)), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func finishScanForTest(t *testing.T, s *Service, id string) ScanOutcome {
	t.Helper()
	for n := 0; n < 100; n++ {
		out, err := s.runScanBatch(context.Background(), id, 1000)
		if err != nil {
			t.Fatal(out, err)
		}
		if !out.HasMore {
			return out
		}
	}
	t.Fatal("scan did not finish")
	return ScanOutcome{}
}
func TestResumableScanPassesTenThousandFilesAfterServiceRestart(t *testing.T) {
	db := testdb.Open(t)
	root := t.TempDir()
	createScanFiles(t, root, 10001)
	s := NewService(NewStore(db), Config{}, nil, nil)
	first, err := s.Scan(context.Background(), ScanRequest{Root: root, Limit: 500})
	if err != nil || !first.HasMore || first.Scanned == 0 || first.Scanned >= 10001 {
		t.Fatal(first, err)
	}
	restarted := NewService(NewStore(db), Config{}, nil, nil)
	final := finishScanForTest(t, restarted, first.JobID)
	if final.State != "completed" || final.Scanned != 10001 || final.Upserted != 10001 {
		t.Fatalf("truncated scan: %+v", final)
	}
	var count int
	if err := db.QueryRow(`select count(*) from files where presence_state='present' and last_seen_scan_id=$1`, first.JobID).Scan(&count); err != nil || count != 10001 {
		t.Fatal(count, err)
	}
	repeat, err := restarted.Scan(context.Background(), ScanRequest{Root: root, Limit: 5000})
	if err != nil {
		t.Fatal(err)
	}
	repeat = finishScanForTest(t, restarted, repeat.JobID)
	if repeat.Scanned != 10001 {
		t.Fatal(repeat.Scanned)
	}
	if err := db.QueryRow(`select count(*) from files`).Scan(&count); err != nil || count != 10001 {
		t.Fatal("duplicate files", count, err)
	}
}
func TestScanCancellationCannotApplyPartialMissingResults(t *testing.T) {
	db := testdb.Open(t)
	root := t.TempDir()
	createScanFiles(t, root, 3)
	s := NewService(NewStore(db), Config{}, nil, nil)
	out, err := s.Scan(context.Background(), ScanRequest{Root: root})
	if err != nil || out.State != "completed" {
		t.Fatal(out, err)
	}
	if err := os.Remove(filepath.Join(root, "Book-00000.mp3")); err != nil {
		t.Fatal(err)
	}
	job, err := s.StartScan(context.Background(), ScanRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	// Discovery completes, then reconciliation can stage absence without applying it.
	partial, err := s.runScanBatch(context.Background(), job.ID, 4)
	if err != nil || !partial.HasMore {
		t.Fatal(partial, err)
	}
	var staged int
	if err := db.QueryRow(`select count(*) from library_scan_absent where job_id=$1`, job.ID).Scan(&staged); err != nil || staged != 1 {
		t.Fatal("missing evidence was not staged", staged, err)
	}
	if _, err := s.CancelScan(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	var missing int
	if err := db.QueryRow(`select count(*) from files where presence_state='missing'`).Scan(&missing); err != nil || missing != 0 {
		t.Fatal("cancelled scan changed presence", missing, err)
	}
	complete, err := s.Scan(context.Background(), ScanRequest{Root: root})
	if err != nil || complete.Missing != 1 {
		t.Fatal(complete, err)
	}
	if err := db.QueryRow(`select count(*) from files where presence_state='missing'`).Scan(&missing); err != nil || missing != 1 {
		t.Fatal(missing, err)
	}
}
func TestScanUnavailableRootRetainsPresenceAndCanResume(t *testing.T) {
	db := testdb.Open(t)
	parent := t.TempDir()
	root := filepath.Join(parent, "mounted")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	createScanFiles(t, root, 3)
	s := NewService(NewStore(db), Config{}, nil, nil)
	if _, err := s.Scan(context.Background(), ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	job, err := s.StartScan(context.Background(), ScanRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	offline := filepath.Join(parent, "offline")
	if err := os.Rename(root, offline); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runScanBatch(context.Background(), job.ID, 100); err == nil {
		t.Fatal("unavailable root reported success")
	}
	var n int
	if err := db.QueryRow(`select count(*) from files where presence_state='present'`).Scan(&n); err != nil || n != 3 {
		t.Fatal(n, err)
	}
	if err := os.Rename(offline, root); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RetryScan(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	final := finishScanForTest(t, s, job.ID)
	if final.Missing != 0 || final.Scanned != 3 {
		t.Fatal(final)
	}
}
func TestScanChangedRootRequiresExplicitAcceptance(t *testing.T) {
	db := testdb.Open(t)
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	createScanFiles(t, root, 1)
	s := NewService(NewStore(db), Config{}, nil, nil)
	if _, err := s.Scan(context.Background(), ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, filepath.Join(parent, "old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartScan(context.Background(), ScanRequest{Root: root}); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatal(err)
	}
	out, err := s.Scan(context.Background(), ScanRequest{Root: root, AcceptRootChange: true})
	if err != nil || out.Missing != 1 {
		t.Fatal(out, err)
	}
}
func TestScanNestedDeviceDisappearanceDoesNotMarkMissing(t *testing.T) {
	db := testdb.Open(t)
	root := t.TempDir()
	createScanFiles(t, root, 1)
	s := NewService(NewStore(db), Config{}, nil, nil)
	if _, err := s.Scan(context.Background(), ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update files set scan_device='previous-nested-device'`); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "Book-00000.mp3")); err != nil {
		t.Fatal(err)
	}
	out, err := s.Scan(context.Background(), ScanRequest{Root: root})
	if err == nil || out.State != "failed" {
		t.Fatal(out, err)
	}
	var state string
	if err := db.QueryRow(`select presence_state from files`).Scan(&state); err != nil || state != "present" {
		t.Fatal(state, err)
	}
}

func TestScanRechecksStagedAbsenceAfterFileReappears(t *testing.T) {
	db := testdb.Open(t)
	root := t.TempDir()
	createScanFiles(t, root, 1)
	s := NewService(NewStore(db), Config{}, nil, nil)
	if _, err := s.Scan(context.Background(), ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Book-00000.mp3")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	job, err := s.StartScan(context.Background(), ScanRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	partial, err := s.runScanBatch(context.Background(), job.ID, 2)
	if err != nil || !partial.HasMore {
		t.Fatal(partial, err)
	}
	var count int
	if err := db.QueryRow(`select count(*) from library_scan_absent where job_id=$1`, job.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	createScanFiles(t, root, 1)
	final := finishScanForTest(t, s, job.ID)
	if final.Missing != 0 {
		t.Fatal(final)
	}
	var state string
	if err := db.QueryRow(`select presence_state from files`).Scan(&state); err != nil || state != "present" {
		t.Fatal(state, err)
	}
}
func TestScanExpiredCancellationAndClaimAreRecoverable(t *testing.T) {
	db := testdb.Open(t)
	root := t.TempDir()
	createScanFiles(t, root, 1)
	s := NewService(NewStore(db), Config{}, nil, nil)
	job, err := s.StartScan(context.Background(), ScanRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.claimScan(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.claimScan(context.Background(), job.ID); err != ErrScanBusy {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update library_scan_jobs set lease_expires_at=now()-interval '1 second' where id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	next, err := s.claimScan(context.Background(), job.ID)
	if err != nil || next.LeaseToken == first.LeaseToken {
		t.Fatal(next, err)
	}
	if _, err := s.CancelScan(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update library_scan_jobs set lease_expires_at=now()-interval '1 second' where id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RunPendingScans(context.Background()); err != nil {
		t.Fatal(err)
	}
	final, err := s.GetScan(context.Background(), job.ID)
	if err != nil || final.State != "cancelled" {
		t.Fatal(final, err)
	}
}

func TestScanFinalPersistenceFailureIsAtomicAndRetryable(t *testing.T) {
	db := testdb.Open(t)
	root := t.TempDir()
	createScanFiles(t, root, 1)
	s := NewService(NewStore(db), Config{}, nil, nil)
	if _, err := s.Scan(context.Background(), ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "Book-00000.mp3")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`alter table files add constraint fail_missing_commit check(presence_state<>'missing')`); err != nil {
		t.Fatal(err)
	}
	out, err := s.Scan(context.Background(), ScanRequest{Root: root})
	if err == nil || out.State != "failed" {
		t.Fatal(out, err)
	}
	var state string
	if err := db.QueryRow(`select presence_state from files`).Scan(&state); err != nil || state != "present" {
		t.Fatal(state, err)
	}
	if _, err := db.Exec(`alter table files drop constraint fail_missing_commit`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RetryScan(context.Background(), out.JobID); err != nil {
		t.Fatal(err)
	}
	final := finishScanForTest(t, s, out.JobID)
	if final.Missing != 1 {
		t.Fatal(final)
	}
	var retained int
	if err := db.QueryRow(`select count(*) from library_scan_entries where job_id=$1`, out.JobID).Scan(&retained); err != nil || retained != 0 {
		t.Fatal("completed queue retained", retained, err)
	}
}
func TestScanFileRemovedBetweenBatchesDoesNotTrapResume(t *testing.T) {
	db := testdb.Open(t)
	root := t.TempDir()
	createScanFiles(t, root, 1)
	s := NewService(NewStore(db), Config{}, nil, nil)
	first, err := s.Scan(context.Background(), ScanRequest{Root: root, Limit: 1})
	if err != nil || !first.HasMore {
		t.Fatal(first, err)
	}
	if err := os.Remove(filepath.Join(root, "Book-00000.mp3")); err != nil {
		t.Fatal(err)
	}
	final := finishScanForTest(t, s, first.JobID)
	if final.State != "completed" || final.Scanned != 0 || final.Skipped != 1 {
		t.Fatal(final)
	}
}
func TestScanWithoutPersistenceDoesNotPanic(t *testing.T) {
	s := NewService(nil, Config{}, nil, nil)
	jobs, err := s.ListScans(context.Background())
	if err != nil || jobs == nil || len(jobs) != 0 {
		t.Fatal(jobs, err)
	}
	if _, err := s.StartScan(context.Background(), ScanRequest{}); err == nil {
		t.Fatal("nonpersistent scan accepted")
	}
	if _, err := s.CancelScan(context.Background(), "missing"); err == nil {
		t.Fatal("nonpersistent cancellation accepted")
	}
}

func TestScanInterruptedDirectoryEnumerationResumesWithoutDuplicateEntries(t *testing.T) {
	db := testdb.Open(t)
	root := t.TempDir()
	createScanFiles(t, root, 600)
	s := NewService(NewStore(db), Config{}, nil, nil)
	job, err := s.StartScan(context.Background(), ScanRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create function interrupt_scan_directory() returns trigger language plpgsql as $$ begin if new.kind='file' and (select count(*) from library_scan_entries where job_id=new.job_id)>=257 then raise exception 'injected enumeration interruption'; end if; return new; end $$; create trigger interrupt_scan_directory before insert on library_scan_entries for each row execute function interrupt_scan_directory()`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runScanBatch(context.Background(), job.ID, 500); err == nil {
		t.Fatal("injected directory write failure was ignored")
	}
	var queued int
	if err := db.QueryRow(`select count(*) from library_scan_entries where job_id=$1`, job.ID).Scan(&queued); err != nil || queued != 257 {
		t.Fatal("first directory page not durable", queued, err)
	}
	if _, err := db.Exec(`drop trigger interrupt_scan_directory on library_scan_entries`); err != nil {
		t.Fatal(err)
	}
	restarted := NewService(NewStore(db), Config{}, nil, nil)
	if _, err := restarted.RetryScan(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	final := finishScanForTest(t, restarted, job.ID)
	if final.Scanned != 600 {
		t.Fatal(final)
	}
	var files int
	if err := db.QueryRow(`select count(*) from files`).Scan(&files); err != nil || files != 600 {
		t.Fatal(files, err)
	}
}
