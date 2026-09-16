package library

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func operationFixture(t *testing.T) (*Service, *sql.DB, acquisition.DownloadStatus, string) {
	t.Helper()
	db := testdb.Open(t)
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "downloads")
	if err := os.Mkdir(sourceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceRoot, "Fixture.epub")
	if err := os.WriteFile(source, []byte("complete public domain fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	var wantedID string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title,author_name,status) values('ebook','Fixture','Author','grabbed') returning id::text`).Scan(&wantedID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into downloads(client,external_id,category,save_path,state) values('qBittorrent','fixture','books',$1,'pausedUP')`, sourceRoot); err != nil {
		t.Fatal(err)
	}
	service := NewService(NewStore(db), Config{EbookRoot: filepath.Join(root, "library")}, wanted.NewStore(db), nil)
	download := acquisition.DownloadStatus{Client: "qBittorrent", ID: "fixture", Name: "Fixture.epub", SavePath: sourceRoot, Category: "books", Progress: 1, State: "pausedUP", Tags: []string{"librarry", "wanted:" + wantedID}}
	return service, db, download, wantedID
}

func TestCompletedOperationCommitsAtomicallyAndRetriesWithoutDuplicates(t *testing.T) {
	service, db, download, wantedID := operationFixture(t)
	ctx := context.Background()
	first, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || first.Imported != 1 {
		t.Fatalf("first: %+v %v", first, err)
	}
	op, err := service.store.operationForDownload(ctx, download.Client, download.ID)
	if err != nil || op.State != "committed" || op.CleanupState != "blocked" || len(op.Files) != 1 {
		t.Fatalf("operation: %+v %v", op, err)
	}
	for query, expected := range map[string]int{
		`select count(*) from files`: 1, `select count(*) from file_wanted_links`: 1,
		`select count(*) from file_download_links`:                                                       1,
		`select count(*) from wanted_items where status='imported'`:                                      1,
		`select count(*) from downloads where import_status='imported' and imported_file_id is not null`: 1,
	} {
		var n int
		if err := db.QueryRow(query).Scan(&n); err != nil || n != expected {
			t.Fatalf("%s: %d %v", query, n, err)
		}
	}
	// Naming changes and stale caller import status must not create another copy.
	service.Reconfigure(Config{EbookRoot: filepath.Join(t.TempDir(), "different-root")})
	second, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{})
	if err != nil || second.Skipped != 1 || second.Results[0].Import.File.ID != first.Results[0].Import.File.ID {
		t.Fatalf("retry: %+v %v", second, err)
	}
	files, err := service.ListFiles(ctx, FileListQuery{WantedID: wantedID})
	if err != nil || len(files) != 1 {
		t.Fatalf("files: %+v %v", files, err)
	}
	if _, err := os.Stat(filepath.Join(download.SavePath, download.Name)); err != nil {
		t.Fatal("source lost", err)
	}
}

func TestPublishedImportRecoversAfterDatabaseFailureAndStaysHidden(t *testing.T) {
	service, db, download, _ := operationFixture(t)
	ctx := context.Background()
	// Fail the last DB projection, after file and wanted writes in the transaction.
	if _, err := db.Exec(`alter table downloads add constraint inject_commit_failure check(import_status <> 'imported')`); err != nil {
		t.Fatal(err)
	}
	result, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || result.Errored != 1 {
		t.Fatalf("failure: %+v %v", result, err)
	}
	op, err := service.store.operationForDownload(ctx, download.Client, download.ID)
	if err != nil || op.State != "failed" || op.LastError == "" {
		t.Fatalf("failure not durable: %+v %v", op, err)
	}
	if _, err := os.Stat(op.Files[0].DestinationPath); err != nil {
		t.Fatal("publication did not occur", err)
	}
	scan, err := service.Scan(ctx, ScanRequest{Root: service.Config().EbookRoot})
	if err != nil || scan.Upserted != 0 || scan.Skipped != 1 {
		t.Fatalf("partial file exposed: %+v %v", scan, err)
	}
	if _, err := service.store.UpsertFile(ctx, FileRecord{Path: op.Files[0].DestinationPath, MediaFormat: "ebook"}); err == nil {
		t.Fatal("direct writer bypassed visibility guard")
	}
	var n int
	if err := db.QueryRow(`select count(*) from files`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial DB commit: %d %v", n, err)
	}
	var state string
	if err := db.QueryRow(`select status from wanted_items`).Scan(&state); err != nil || state != "grabbed" {
		t.Fatalf("wanted changed: %s %v", state, err)
	}
	if _, err := db.Exec(`alter table downloads drop constraint inject_commit_failure`); err != nil {
		t.Fatal(err)
	}
	// A new process resumes the persisted destination, even with different config.
	restarted := NewService(NewStore(db), Config{EbookRoot: filepath.Join(t.TempDir(), "changed")}, wanted.NewStore(db), nil)
	result, err = restarted.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "hardlink"})
	if err != nil || result.Imported != 1 || result.Results[0].Import.DestinationPath != op.Files[0].DestinationPath {
		t.Fatalf("resume: %+v %v", result, err)
	}
	resumed, err := service.store.getOperation(ctx, op.ID)
	if err != nil || resumed.Attempts != 2 || resumed.State != "committed" {
		t.Fatalf("resumed: %+v %v", resumed, err)
	}
}

func TestRecoveryRejectsChangedSourceAndDestination(t *testing.T) {
	for _, target := range []string{"source", "destination"} {
		t.Run(target, func(t *testing.T) {
			service, db, download, _ := operationFixture(t)
			ctx := context.Background()
			if _, err := db.Exec(`alter table downloads add constraint inject_commit_failure check(import_status <> 'imported')`); err != nil {
				t.Fatal(err)
			}
			if _, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{}); err != nil {
				t.Fatal(err)
			}
			op, err := service.store.operationForDownload(ctx, download.Client, download.ID)
			if err != nil {
				t.Fatal(err)
			}
			path := op.Files[0].SourcePath
			if target == "destination" {
				path = op.Files[0].DestinationPath
			}
			if err := os.WriteFile(path, []byte("modified"), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`alter table downloads drop constraint inject_commit_failure`); err != nil {
				t.Fatal(err)
			}
			result, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{})
			if err != nil || result.Errored != 1 {
				t.Fatalf("mutation accepted: %+v %v", result, err)
			}
			var n int
			if err := db.QueryRow(`select count(*) from files`).Scan(&n); err != nil || n != 0 {
				t.Fatalf("bad file committed: %d %v", n, err)
			}
		})
	}
}

func TestImportLeaseFencesStaleWorkers(t *testing.T) {
	service, db, download, _ := operationFixture(t)
	ctx := context.Background()
	if _, err := db.Exec(`alter table downloads add constraint inject_commit_failure check(import_status <> 'imported')`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{}); err != nil {
		t.Fatal(err)
	}
	op, err := service.store.operationForDownload(ctx, download.Client, download.ID)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.store.claimOperation(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.store.claimOperation(ctx, op.ID); !errors.Is(err, ErrImportBusy) {
		t.Fatalf("concurrent claim: %v", err)
	}
	if _, err := db.Exec(`update import_operations set lease_expires_at=now()-interval '1 second' where id=$1`, op.ID); err != nil {
		t.Fatal(err)
	}
	second, err := service.store.claimOperation(ctx, op.ID)
	if err != nil || first == second {
		t.Fatalf("new lease: %s %v", second, err)
	}
	op.LeaseToken = first
	if err := service.store.markOperationFile(ctx, op, op.Files[0].ID); !errors.Is(err, ErrImportBusy) {
		t.Fatalf("stale verification: %v", err)
	}
	if _, err := service.store.commitOperation(ctx, op, []FileRecord{{Path: op.Files[0].DestinationPath, MediaFormat: "ebook"}}); !errors.Is(err, ErrImportBusy) {
		t.Fatalf("stale commit: %v", err)
	}
	service.store.failOperation(ctx, op.ID, first, "stale failure")
	if err := service.store.renewOperation(ctx, op.ID, second); err != nil {
		t.Fatal("stale failure invalidated winner", err)
	}
}

func TestOperationCleanupRechecksFilesAndRecordsFailures(t *testing.T) {
	service, db, download, _ := operationFixture(t)
	ctx := context.Background()
	result, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || result.Imported != 1 {
		t.Fatalf("import: %+v %v", result, err)
	}
	op, err := service.store.operationForDownload(ctx, download.Client, download.ID)
	if err != nil {
		t.Fatal(err)
	}
	download.ImportedFileID = op.Files[0].FileID
	inventory := []acquisition.DownloadFile{{Name: download.Name, SizeBytes: op.Files[0].SizeBytes, Progress: 1}}
	if err := service.VerifyCompletedDownload(ctx, download, inventory); err != nil {
		t.Fatal(err)
	}
	op, err = service.store.getOperation(ctx, op.ID)
	if err != nil || op.CleanupState != "eligible" {
		t.Fatalf("not eligible: %+v %v", op, err)
	}
	if err := service.RecordCompletedCleanup(ctx, download, errors.New("client unavailable")); err != nil {
		t.Fatal(err)
	}
	op, err = service.store.getOperation(ctx, op.ID)
	if err != nil || op.State != "committed" || op.CleanupState != "blocked" || op.CleanupError != "client unavailable" {
		t.Fatalf("cleanup failure: %+v %v", op, err)
	}
	// Removing the relationship blocks cleanup even with the legacy receipt intact.
	if _, err := db.Exec(`delete from file_wanted_links where file_id=$1`, op.Files[0].FileID); err != nil {
		t.Fatal(err)
	}
	if err := service.VerifyCompletedDownload(ctx, download, inventory); err == nil {
		t.Fatal("unlinked file allowed cleanup")
	}
	if _, err := os.Stat(op.Files[0].SourcePath); err != nil {
		t.Fatal("source removed", err)
	}
}

func TestOperationRequiresEverySidecarBeforeCommit(t *testing.T) {
	service, db, download, _ := operationFixture(t)
	payload := filepath.Join(download.SavePath, "Fixture")
	if err := os.Mkdir(payload, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(download.SavePath, download.Name), filepath.Join(payload, "Fixture.epub")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payload, "Fixture.cue"), []byte("fixture cue"), 0644); err != nil {
		t.Fatal(err)
	}
	download.Name = "Fixture"
	service.Reconfigure(Config{EbookRoot: service.Config().EbookRoot, ImportExtraFiles: ".cue"})
	if _, err := db.Exec(`alter table downloads add constraint inject_commit_failure check(import_status <> 'imported')`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{}); err != nil {
		t.Fatal(err)
	}
	op, err := service.store.operationForDownload(ctx, download.Client, download.ID)
	if err != nil || len(op.Files) != 2 {
		t.Fatalf("manifest: %+v %v", op, err)
	}
	if err := os.Remove(op.Files[1].SourcePath); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`alter table downloads drop constraint inject_commit_failure`); err != nil {
		t.Fatal(err)
	}
	result, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{})
	if err != nil || result.Errored != 1 {
		t.Fatalf("missing sidecar accepted: %+v %v", result, err)
	}
}
