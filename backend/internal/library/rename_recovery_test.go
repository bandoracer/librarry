package library

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestRenameCommitFailureRetainsSourceAndResumes(t *testing.T) {
	db := testdb.Open(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "Old folder", "old.epub")
	if err := os.MkdirAll(filepath.Dir(source), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("controlled book bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	service := NewService(NewStore(db), Config{EbookRoot: root, NamingAuthorFolderTemplate: "{Author}", NamingBookFolderTemplate: "{Title}", NamingFileNameTemplate: "{Title}{Ext}"}, nil, nil)
	original, err := service.TrackFile(context.Background(), FileRecord{Path: source, SourcePath: "/original/download.epub", MediaFormat: "ebook", Title: "Owner title", AuthorName: "Owner author", Extension: ".epub", ImportStatus: "imported", Metadata: map[string]any{"ownerNote": "keep this"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`create function reject_rename_commit() returns trigger language plpgsql as $$ begin if new.path<>old.path then raise exception 'controlled rename commit failure'; end if; return new; end $$; create trigger fail_rename_commit before update on files for each row execute function reject_rename_commit()`); err != nil {
		t.Fatal(err)
	}
	first, err := service.RenameFiles(context.Background(), RenameFilesRequest{IDs: []string{original.ID}})
	if err != nil || first.Errored != 1 {
		t.Fatal(first, err)
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != "controlled book bytes" {
		t.Fatalf("source must survive a failed rename commit: %q %v", data, err)
	}
	scanned, err := service.Scan(context.Background(), ScanRequest{Root: root})
	if err != nil || scanned.Upserted != 0 || scanned.Skipped < 2 {
		t.Fatal("scan exposed or rewrote a pending rename", scanned, err)
	}
	if _, err = db.Exec(`drop trigger fail_rename_commit on files`); err != nil {
		t.Fatal(err)
	}
	service = NewService(NewStore(db), service.Config(), nil, nil)
	retried, err := service.RenameFiles(context.Background(), RenameFilesRequest{IDs: []string{original.ID}})
	if err != nil || retried.Renamed != 1 || len(retried.Results) != 1 || retried.Results[0].File == nil {
		t.Fatal(retried, err)
	}
	file := retried.Results[0].File
	if file.ID != original.ID || file.SourcePath != original.SourcePath || file.Title != original.Title || file.Metadata["ownerNote"] != "keep this" {
		t.Fatal(file)
	}
	if data, err := os.ReadFile(file.Path); err != nil || string(data) != "controlled book bytes" {
		t.Fatal(string(data), err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("committed rename left source", err)
	}
	var count int
	if err = db.QueryRow(`select count(*) from files`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestRenameRetainsLinksAndVerifiesOriginalReceipt(t *testing.T) {
	service, db, download, wantedID := operationFixture(t)
	ctx := context.Background()
	imported, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || imported.Imported != 1 {
		t.Fatal(imported, err)
	}
	original := imported.Results[0].Import.File
	receipt, err := service.store.operationForDownload(ctx, download.Client, download.ID)
	if err != nil {
		t.Fatal(err)
	}
	var other string
	if err = db.QueryRow(`insert into wanted_items(wanted_format,title,status,monitored) values('ebook','Owner second association','removed',false) returning id::text`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	// JSON still names the original wanted ID, while relational links contain both.
	if _, err = db.Exec(`insert into file_wanted_links(file_id,wanted_item_id) values($1,$2)`, original.ID, other); err != nil {
		t.Fatal(err)
	}
	config := service.Config()
	config.NamingFileNameTemplate = "Renamed {Title}{Ext}"
	service.Reconfigure(config)
	renamed, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{original.ID}})
	if err != nil || renamed.Renamed != 1 {
		t.Fatal(renamed, err)
	}
	file := renamed.Results[0].File
	if file.ID != original.ID || file.Metadata["importMode"] != original.Metadata["importMode"] || file.Metadata["importOperationId"] != original.Metadata["importOperationId"] {
		t.Fatal(file, original)
	}
	var links int
	if err = db.QueryRow(`select count(*) from file_wanted_links where file_id=$1`, original.ID).Scan(&links); err != nil || links != 2 {
		t.Fatal(links, err)
	}
	var retained bool
	if err = db.QueryRow(`select status='removed' and not monitored from wanted_items where id=$1`, other).Scan(&retained); err != nil || !retained {
		t.Fatal(retained, err)
	}
	replay, err := service.committedOperationOutcome(ctx, receipt)
	if err != nil || replay.File.Path != file.Path || replay.File.ID != original.ID {
		t.Fatal(replay, err)
	}
	if err = service.verifyOperationCleanup(ctx, receipt, download, nil); err != nil {
		t.Fatal(err)
	}
	// A second name and a return to the first name create distinct operations.
	config.NamingFileNameTemplate = "Again {Title}{Ext}"
	service.Reconfigure(config)
	again, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{original.ID}})
	if err != nil || again.Renamed != 1 {
		t.Fatal(again, err)
	}
	config.NamingFileNameTemplate = "Renamed {Title}{Ext}"
	service.Reconfigure(config)
	back, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{original.ID}})
	if err != nil || back.Renamed != 1 {
		t.Fatal(back, err)
	}
	config.NamingFileNameTemplate = "Again {Title}{Ext}"
	service.Reconfigure(config)
	repeat, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{original.ID}})
	if err != nil || repeat.Renamed != 1 || repeat.Results[0].OperationID == again.Results[0].OperationID {
		t.Fatal(repeat, err)
	}
	if err = service.verifyOperationCleanup(ctx, receipt, download, nil); err != nil {
		t.Fatal(err)
	}
	var importedStatus string
	if err = db.QueryRow(`select status from wanted_items where id=$1`, wantedID).Scan(&importedStatus); err != nil || importedStatus != "imported" {
		t.Fatal(importedStatus, err)
	}
	// Matching hashes at an unjournaled location cannot authorize source deletion.
	if _, err = db.Exec(`update files set path=path||'.unverified' where id=$1`, original.ID); err != nil {
		t.Fatal(err)
	}
	if err = service.verifyOperationCleanup(ctx, receipt, download, nil); err == nil {
		t.Fatal("unverified path authorized cleanup")
	}
}

func TestRenameCleanupFailureHidesOldSourceAndRetries(t *testing.T) {
	service, db, download, wantedID := operationFixture(t)
	ctx := context.Background()
	imported, err := service.Import(ctx, ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID, ImportMode: "copy"})
	if err != nil {
		t.Fatal(err)
	}
	original := imported.File
	config := service.Config()
	config.NamingFileNameTemplate = "Renamed {Title}{Ext}"
	service.Reconfigure(config)
	// Fail lease acquisition after the rename visibility transaction commits.
	if _, err = db.Exec(`create function reject_rename_cleanup() returns trigger language plpgsql as $$ begin if new.metadata ? 'renameFileId' and old.state='committed' and new.lease_token is not null then raise exception 'controlled cleanup failure'; end if; return new; end $$; create trigger fail_rename_cleanup before update on import_operations for each row execute function reject_rename_cleanup()`); err != nil {
		t.Fatal(err)
	}
	result, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{original.ID}})
	if err != nil || result.Errored != 1 || result.Results[0].OperationID == "" {
		t.Fatal(result, err)
	}
	op, err := service.store.getOperation(ctx, result.Results[0].OperationID)
	if err != nil || op.State != "committed" {
		t.Fatal(op, err)
	}
	for _, path := range []string{original.Path, op.Files[0].DestinationPath} {
		if _, err = os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = service.store.UpsertFile(ctx, FileRecord{Path: original.Path, MediaFormat: "ebook"}); err == nil {
		t.Fatal("old source accepted as another file")
	}
	scanned, err := service.Scan(ctx, ScanRequest{Root: config.EbookRoot})
	if err != nil || scanned.Skipped < 1 {
		t.Fatal(scanned, err)
	}
	var count int
	if err = db.QueryRow(`select count(*) from files`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if _, err = db.Exec(`drop trigger fail_rename_cleanup on import_operations`); err != nil {
		t.Fatal(err)
	}
	service = NewService(NewStore(db), config, service.wanted, nil)
	resumed, err := service.RetryImportOperation(ctx, op.ID)
	if err != nil || resumed.File.ID != original.ID {
		t.Fatal(resumed, err)
	}
	if _, err = os.Stat(original.Path); !os.IsNotExist(err) {
		t.Fatal("source retained after cleanup", err)
	}
	if err = db.QueryRow(`select count(*) from history_events where event_type='file_renamed'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if _, err = service.committedOperationOutcome(ctx, mustOperation(t, service, imported.OperationID)); err != nil {
		t.Fatal(err)
	}
}

func mustOperation(t *testing.T, service *Service, id string) ImportOperation {
	t.Helper()
	op, err := service.store.getOperation(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func TestRenameRequiresCurrentPreviewAndRetainsConflict(t *testing.T) {
	service, db, download, wantedID := operationFixture(t)
	ctx := context.Background()
	imported, err := service.Import(ctx, ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID, ImportMode: "copy"})
	if err != nil {
		t.Fatal(err)
	}
	config := service.Config()
	config.NamingFileNameTemplate = "Renamed {Title}{Ext}"
	service.Reconfigure(config)
	preview, err := service.PreviewRenameFiles(ctx, RenameFilesRequest{IDs: []string{imported.File.ID}})
	if err != nil || len(preview.Previews) != 1 {
		t.Fatal(preview, err)
	}
	p := preview.Previews[0]
	if err = os.WriteFile(p.DestinationPath, []byte("another book"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{imported.File.ID}, Revisions: map[string]string{imported.File.ID: p.Revision}})
	if err != nil || result.Errored != 1 {
		t.Fatal(result, err)
	}
	if bytes, err := os.ReadFile(p.DestinationPath); err != nil || string(bytes) != "another book" {
		t.Fatal(string(bytes), err)
	}
	if _, err = os.Stat(imported.File.Path); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{imported.File.ID}, Revisions: map[string]string{}}); err == nil {
		t.Fatal("missing revision accepted")
	}
	// Legacy overwrite requests also retain existing bytes.
	result, err = service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{imported.File.ID}, Overwrite: true})
	if err != nil || result.Errored != 1 {
		t.Fatal(result, err)
	}
	var count int
	if err = db.QueryRow(`select count(*) from history_events where event_type='file_renamed'`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestRenameRetainsChapterAndCompanionLayouts(t *testing.T) {
	for _, name := range []string{"single-companion", "numbered-chapter", "disc-directory", "sibling-audio"} {
		t.Run(name, func(t *testing.T) {
			service, _, _, _ := operationFixture(t)
			ctx := context.Background()
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			config := service.Config()
			config.EbookRoot = root
			config.AudiobookRoot = root
			service.Reconfigure(config)
			source := filepath.Join(root, "Old book", "Book.m4b")
			format := "audiobook"
			if name == "single-companion" {
				source = filepath.Join(root, "Old book", "Book.epub")
				format = "ebook"
			}
			if name == "numbered-chapter" {
				source = filepath.Join(root, "Old book", "01.mp3")
			}
			if name == "disc-directory" {
				source = filepath.Join(root, "Old book", "Disc 1", "Book.mp3")
			}
			if err = os.MkdirAll(filepath.Dir(source), 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(source, []byte("fixture bytes"), 0644); err != nil {
				t.Fatal(err)
			}
			if name == "single-companion" || name == "sibling-audio" {
				extra := "cover.jpg"
				if name == "sibling-audio" {
					extra = "Other chapter.mp3"
				}
				if err = os.WriteFile(filepath.Join(filepath.Dir(source), extra), []byte("companion bytes"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			file, err := service.TrackFile(ctx, FileRecord{Path: source, Title: "New book", AuthorName: "Owner", MediaFormat: format, Extension: filepath.Ext(source)})
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{file.ID}})
			if err != nil || result.Skipped != 1 || result.Renamed != 0 || result.Previews[0].Reason != renameSetReason {
				t.Fatal(result, err)
			}
			if _, err = os.Stat(source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRenameSavedPlanPreservesConcurrentOwnerCorrections(t *testing.T) {
	service, db, download, wantedID := operationFixture(t)
	ctx := context.Background()
	imported, err := service.Import(ctx, ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID, ImportMode: "copy"})
	if err != nil {
		t.Fatal(err)
	}
	config := service.Config()
	config.NamingFileNameTemplate = "Renamed {Title}{Ext}"
	service.Reconfigure(config)
	if _, err = db.Exec(`create function reject_rename_history() returns trigger language plpgsql as $$ begin if new.event_type='file_renamed' then raise exception 'controlled history failure'; end if; return new; end $$; create trigger fail_rename_history before insert on history_events for each row execute function reject_rename_history()`); err != nil {
		t.Fatal(err)
	}
	first, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{imported.File.ID}})
	if err != nil || first.Errored != 1 {
		t.Fatal(first, err)
	}
	op := mustOperation(t, service, first.Results[0].OperationID)
	if _, err = db.Exec(`update files set title='Corrected owner title',metadata=metadata||'{"ownerNote":"corrected"}'::jsonb where id=$1`, imported.File.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`drop trigger fail_rename_history on history_events`); err != nil {
		t.Fatal(err)
	}
	config.NamingFileNameTemplate = "Different {Title}{Ext}"
	service.Reconfigure(config)
	preview, err := service.PreviewRenameFiles(ctx, RenameFilesRequest{IDs: []string{imported.File.ID}})
	if err != nil || len(preview.Previews) != 1 {
		t.Fatal(preview, err)
	}
	revision := preview.Previews[0].Revision
	var group sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{imported.File.ID}, Revisions: map[string]string{imported.File.ID: revision}})
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	// All contenders resume the same saved target, never overwrite owner fields.
	fileRows, err := service.store.FindFiles(ctx, []string{imported.File.ID}, nil)
	if err != nil || len(fileRows) != 1 {
		t.Fatal(fileRows, err)
	}
	file := fileRows[0]
	if file.Path != op.Files[0].DestinationPath || file.Title != "Corrected owner title" || file.Metadata["ownerNote"] != "corrected" {
		t.Fatal(file)
	}
	var count int
	if err = db.QueryRow(`select count(*) from import_operations where metadata ? 'renameFileId'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if err = db.QueryRow(`select count(*) from history_events where event_type='file_renamed'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestRenameRejectsChangedBytesAndExtension(t *testing.T) {
	service, _, download, wantedID := operationFixture(t)
	ctx := context.Background()
	imported, err := service.Import(ctx, ImportRequest{SourcePath: filepath.Join(download.SavePath, download.Name), WantedID: wantedID, ImportMode: "copy"})
	if err != nil {
		t.Fatal(err)
	}
	config := service.Config()
	config.NamingFileNameTemplate = "Renamed {Title}.pdf"
	service.Reconfigure(config)
	result, err := service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{imported.File.ID}})
	if err != nil || result.Errored != 1 || !strings.Contains(result.Results[0].Message, "file extension") {
		t.Fatal(result, err)
	}
	config.NamingFileNameTemplate = "Renamed {Title}{Ext}"
	service.Reconfigure(config)
	if err = os.WriteFile(imported.File.Path, []byte("owner changed the book"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err = service.RenameFiles(ctx, RenameFilesRequest{IDs: []string{imported.File.ID}})
	if err != nil || result.Errored != 1 || !strings.Contains(result.Results[0].Message, "bytes changed") {
		t.Fatal(result, err)
	}
	if data, err := os.ReadFile(imported.File.Path); err != nil || string(data) != "owner changed the book" {
		t.Fatal(string(data), err)
	}
}
