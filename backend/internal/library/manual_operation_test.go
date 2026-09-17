package library

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManualImportsAreDurableAndIdempotent(t *testing.T) {
	for _, mode := range []string{"copy", "hardlink", "hardlinkOrCopy", "move"} {
		t.Run(mode, func(t *testing.T) {
			service, db, download, wantedID := operationFixture(t)
			request := ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID, ImportMode: mode}
			extra := request.SourcePath[:len(request.SourcePath)-len(filepath.Ext(request.SourcePath))] + ".cue"
			if err := os.WriteFile(extra, []byte("associated cue"), 0644); err != nil {
				t.Fatal(err)
			}
			outcome, err := service.Import(context.Background(), request)
			if err != nil || !outcome.Imported || outcome.OperationID == "" {
				t.Fatalf("manual import: %+v %v", outcome, err)
			}
			if mode == "move" && (outcome.File.Metadata["move"] != true || !outcome.Moved) {
				t.Fatalf("successful move response retained stale metadata: %+v", outcome)
			}
			op, err := service.store.getOperation(context.Background(), outcome.OperationID)
			if err != nil || op.SourceKind != "manual" || op.DownloadRecordID != "" || len(op.Files) != 2 || op.CleanupState != "cleaned" {
				t.Fatalf("manual receipt: %+v %v", op, err)
			}
			for _, file := range op.Files {
				if err := verifyManifestPath(file.DestinationPath, file); err != nil {
					t.Fatal(err)
				}
				_, statErr := os.Stat(file.SourcePath)
				if mode == "move" && !errors.Is(statErr, os.ErrNotExist) {
					t.Fatal("move retained source", statErr)
				}
				if mode != "move" && statErr != nil {
					t.Fatal("copy lost source", statErr)
				}
			}
			repeat, err := service.Import(context.Background(), request)
			if err != nil || repeat.OperationID != outcome.OperationID || !repeat.Skipped {
				t.Fatalf("retry duplicated import: %+v %v", repeat, err)
			}
			var count int
			if err := db.QueryRow(`select count(*) from files`).Scan(&count); err != nil || count != 1 {
				t.Fatalf("file count %d %v", count, err)
			}
		})
	}
}

func TestManualMoveResumesAfterAtomicDatabaseFailure(t *testing.T) {
	service, db, download, wantedID := operationFixture(t)
	request := ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID, ImportMode: "move"}
	if _, err := db.Exec(`alter table wanted_items add constraint fail_import_status check(status<>'imported')`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Import(context.Background(), request); err == nil {
		t.Fatal("failed database commit reported success")
	}
	report, err := service.ImportRecovery(context.Background())
	if err != nil || len(report.Operations) != 1 || report.Operations[0].State != "failed" {
		t.Fatalf("recovery: %+v %v", report, err)
	}
	op := report.Operations[0]
	if _, err := os.Stat(request.SourcePath); err != nil {
		t.Fatal("source removed before commit", err)
	}
	var files int
	if err := db.QueryRow(`select count(*) from files`).Scan(&files); err != nil || files != 0 {
		t.Fatal("partial record committed", files, err)
	}
	if _, err := db.Exec(`alter table wanted_items drop constraint fail_import_status`); err != nil {
		t.Fatal(err)
	}
	restarted := NewService(service.store, service.Config(), service.wanted, service.downloads)
	outcome, err := restarted.Import(context.Background(), request)
	if err != nil || outcome.OperationID != op.ID || !outcome.Moved {
		t.Fatalf("restart: %+v %v", outcome, err)
	}
	if _, err := os.Stat(request.SourcePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("move did not finish")
	}
}

func TestManualUnassociatedAndInPlaceImport(t *testing.T) {
	service, db, download, _ := operationFixture(t)
	if _, err := db.Exec(`insert into notification_targets(name,type,settings) values('Fixture','webhook','{"url":"http://127.0.0.1:1/unused"}')`); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(download.SavePath, download.Name)
	result, err := service.Import(context.Background(), ImportRequest{SourcePath: source})
	if err != nil || result.File.ID == "" {
		t.Fatalf("unassociated import: %+v %v", result, err)
	}
	if replay, err := service.Import(context.Background(), ImportRequest{SourcePath: source}); err != nil || !replay.Skipped {
		t.Fatal(replay, err)
	}
	var notifications int
	if err := db.QueryRow(`select count(*) from notification_deliveries d join notification_events e on e.id=d.event_id where e.event->>'type'='import'`).Scan(&notifications); err != nil || notifications != 1 {
		t.Fatal("unassigned import notification missing or duplicated", notifications, err)
	}
	inPlace, err := service.Import(context.Background(), ImportRequest{SourcePath: result.DestinationPath, ImportMode: "move"})
	if err != nil || inPlace.DestinationPath != result.DestinationPath {
		t.Fatalf("in-place import: %+v %v", inPlace, err)
	}
	if _, err := os.Stat(result.DestinationPath); err != nil {
		t.Fatal("in-place source removed", err)
	}
}

func TestManualReplacementKeepsPreviousFileUntilCommit(t *testing.T) {
	service, db, download, wantedID := operationFixture(t)
	request := ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID, ImportMode: "copy", ConflictAction: "replace"}
	first, err := service.Import(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	oldBytes, err := os.ReadFile(first.DestinationPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(request.SourcePath, []byte("replacement book bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create function fail_manual_commit() returns trigger language plpgsql as $$ begin if new.state='committed' then raise exception 'injected commit failure'; end if; return new; end $$; create trigger fail_manual_commit before update of state on import_operations for each row execute function fail_manual_commit()`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Import(context.Background(), request); err == nil {
		t.Fatal("replacement commit failure ignored")
	}
	report, err := service.ImportRecovery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	visible, err := service.ListFiles(context.Background(), FileListQuery{})
	if err != nil || len(visible) != 0 {
		t.Fatalf("unfinished replacement exposed: %+v %v", visible, err)
	}
	failed := report.Operations[0]
	if failed.State != "failed" || failed.Files[0].PreviousPath == "" {
		t.Fatalf("missing replacement journal: %+v", failed)
	}
	if data, err := os.ReadFile(failed.Files[0].PreviousPath); err != nil || string(data) != string(oldBytes) {
		t.Fatalf("previous file lost: %s %v", data, err)
	}
	if _, err := db.Exec(`drop trigger fail_manual_commit on import_operations`); err != nil {
		t.Fatal(err)
	}
	result, err := service.RetryImportOperation(context.Background(), failed.ID)
	if err != nil || !result.Replaced || result.File.ID != first.File.ID {
		t.Fatalf("replacement retry: %+v %v", result, err)
	}
	if data, err := os.ReadFile(result.DestinationPath); err != nil || string(data) != "replacement book bytes" {
		t.Fatalf("replacement bytes %s %v", data, err)
	}
	if _, err := os.Stat(failed.Files[0].PreviousPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old copy not disposed after commit: %v", err)
	}
}

func TestManualReplacementCleanupCanRetryUnavailableRecycleBin(t *testing.T) {
	service, _, download, wantedID := operationFixture(t)
	request := ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID, ConflictAction: "replace"}
	if _, err := service.Import(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(request.SourcePath, []byte("replacement"), 0644); err != nil {
		t.Fatal(err)
	}
	config := service.Config()
	config.RecycleBin = filepath.Join(download.SavePath, "bin")
	if err := os.WriteFile(config.RecycleBin, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}
	service.Reconfigure(config)
	if _, err := service.Import(context.Background(), request); err == nil {
		t.Fatal("unavailable bin ignored")
	}
	report, err := service.ImportRecovery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	op := report.Operations[0]
	if op.State != "committed" || op.CleanupError == "" || report.Unfinished != 1 {
		t.Fatalf("cleanup hidden: %+v", report)
	}
	if err := verifyPreviousImportFile(op.Files[0]); err != nil {
		t.Fatal("unusable recycle bin lost previous file", err)
	}
	if err := os.Remove(config.RecycleBin); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RetryImportOperation(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
	report, err = service.ImportRecovery(context.Background())
	if err != nil || report.Unfinished != 0 {
		t.Fatalf("cleanup not recovered: %+v %v", report, err)
	}
}

func TestManualMoveRetriesUnacknowledgedSourceRemoval(t *testing.T) {
	service, db, download, wantedID := operationFixture(t)
	request := ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID, ImportMode: "move"}
	if _, err := db.Exec(`create function fail_move_ack() returns trigger language plpgsql as $$ begin if new.source_removed then raise exception 'injected removal acknowledgement failure'; end if; return new; end $$; create trigger fail_move_ack before update of source_removed on import_operation_files for each row execute function fail_move_ack()`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Import(context.Background(), request); err == nil {
		t.Fatal("cleanup persistence error was hidden")
	}
	report, err := service.ImportRecovery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	op := report.Operations[0]
	if op.State != "committed" || op.CleanupState != "blocked" || report.Unfinished != 1 {
		t.Fatalf("cleanup state: %+v", report)
	}
	if _, err := os.Stat(request.SourcePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("fault did not follow physical removal")
	}
	if err := verifyManifestPath(op.Files[0].DestinationPath, op.Files[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`drop trigger fail_move_ack on import_operation_files`); err != nil {
		t.Fatal(err)
	}
	outcome, err := service.Import(context.Background(), request)
	if err != nil || !outcome.Moved || outcome.OperationID != op.ID {
		t.Fatalf("retry: %+v %v", outcome, err)
	}
}

func TestFilesystemFenceBlocksLeaseTakeoverDuringMutation(t *testing.T) {
	service, op := plannedStageFixture(t)
	ctx := context.Background()
	token, err := service.store.claimOperation(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	op.LeaseToken = token
	if _, err := service.store.db.Exec(`update import_operations set lease_expires_at=clock_timestamp()+interval '1 second' where id=$1`, op.ID); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- service.store.withOperationFence(ctx, op, func(tx *sql.Tx) error {
			close(entered)
			<-release
			return nil
		})
	}()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("fence setup: %v", err)
	}
	// Let the previously committed lease expire while the filesystem section
	// still owns the row. A competing claim must wait, rather than take over.
	time.Sleep(1100 * time.Millisecond)
	short, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	_, claimErr := service.store.claimOperation(short, op.ID)
	cancel()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if claimErr == nil || errors.Is(claimErr, ErrImportBusy) {
		t.Fatalf("takeover did not wait on filesystem fence: %v", claimErr)
	}
	if _, err := service.store.claimOperation(ctx, op.ID); !errors.Is(err, ErrImportBusy) {
		t.Fatalf("fence did not retain ownership: %v", err)
	}
}

func TestManualInPlacePreservesExistingBookIdentity(t *testing.T) {
	service, _, download, wantedID := operationFixture(t)
	original, err := service.Import(context.Background(), ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID})
	if err != nil {
		t.Fatal(err)
	}
	adopted, err := service.Import(context.Background(), ImportRequest{SourcePath: original.DestinationPath, ImportMode: "move"})
	if err != nil || adopted.File.ID != original.File.ID || adopted.DestinationPath != original.DestinationPath || adopted.Moved {
		t.Fatalf("in-place identity: %+v %v", adopted, err)
	}
	if adopted.File.Metadata["wantedId"] != wantedID || adopted.File.Title != original.File.Title {
		t.Fatalf("book identity lost: %+v", adopted.File)
	}
}
