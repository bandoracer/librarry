package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func TestBookRenamePreservesDiscsCompanionsAndReceipts(t *testing.T) {
	service, download, wantedID := audiobookFixture(t)
	ctx := context.Background()
	playlist := filepath.Join(download.SavePath, download.Name, "playlist.m3u")
	if err := os.WriteFile(playlist, []byte("#EXTM3U\nDisc 1/1.mp3\nDisc 1/2.mp3\nDisc 2/1.mp3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	service.WithDownloadInspector(newFixtureInspector(t, download))
	imported, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || imported.Imported != 1 {
		t.Fatal(imported, err)
	}
	original := mustOperation(t, service, imported.Results[0].Import.OperationID)
	if _, err = service.store.db.Exec(`update wanted_items set title='Renamed Fixture Book',monitored=false where id=$1`, wantedID); err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewBookRename(ctx, wantedID)
	if err != nil || preview.MediaFiles != 4 || preview.CompanionFiles != 3 || preview.Noop {
		t.Fatal(preview, err)
	}
	if !strings.HasSuffix(preview.DestinationFolder, "Renamed Fixture Book") {
		t.Fatal(preview)
	}
	if _, err = service.store.db.Exec(`create function fail_book_rename() returns trigger language plpgsql as $$ begin if old.path<>new.path then raise exception 'controlled book rename commit failure'; end if; return new; end $$; create trigger book_rename_failure before update on files for each row execute function fail_book_rename()`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RenameBook(ctx, wantedID, preview.Revision); err == nil {
		t.Fatal("failed group commit succeeded")
	}
	for _, f := range original.Files {
		if err = verifyManifestPath(f.DestinationPath, f); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = service.store.db.QueryRow(`select count(*) from file_rename_claims`).Scan(&count); err != nil || count != 4 {
		t.Fatal(count, err)
	}
	if _, err = service.store.db.Exec(`drop trigger book_rename_failure on files`); err != nil {
		t.Fatal(err)
	}
	service = NewService(service.store, service.Config(), service.wanted, nil).WithDownloadInspector(service.inspector)
	resumed, err := service.PreviewBookRename(ctx, wantedID)
	if err != nil || resumed.OperationID == "" || resumed.DestinationFolder != preview.DestinationFolder {
		t.Fatal(resumed, err)
	}
	result, err := service.RenameBook(ctx, wantedID, resumed.Revision)
	if err != nil || len(result.Files) != 4 {
		t.Fatal(result, err)
	}
	for _, old := range original.Files {
		if _, err = os.Stat(old.DestinationPath); !os.IsNotExist(err) {
			t.Fatal("old book file retained", old.DestinationPath, err)
		}
	}
	for i, file := range result.Files {
		if file.ID != imported.Results[0].Import.Files[i].ID {
			t.Fatal("file identity changed", file)
		}
	}
	if bytes, err := os.ReadFile(filepath.Join(preview.DestinationFolder, "playlist.m3u")); err != nil || !strings.Contains(string(bytes), "Disc 2/1.mp3") {
		t.Fatal(string(bytes), err)
	}
	if err = service.VerifyCompletedDownload(ctx, download, nil); err != nil {
		t.Fatal(err)
	}
	replay, err := service.runImportOperation(ctx, original)
	if err != nil || len(replay.Files) != 4 {
		t.Fatal(replay, err)
	}
	if err = service.store.db.QueryRow(`select count(*) from file_rename_claims`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	var unchanged bool
	if err = service.store.db.QueryRow(`select status='imported' and not monitored from wanted_items where id=$1`, wantedID).Scan(&unchanged); err != nil || !unchanged {
		t.Fatal(unchanged, err)
	}
	// Revisit a historical path twice; each new change remains a distinct move.
	for _, title := range []string{"Fixture Book", "Renamed Fixture Book", "Fixture Book", "Renamed Fixture Book"} {
		if _, err = service.store.db.Exec(`update wanted_items set title=$2,updated_at=now() where id=$1`, wantedID, title); err != nil {
			t.Fatal(err)
		}
		p, err := service.PreviewBookRename(ctx, wantedID)
		if err != nil || p.Noop {
			t.Fatal(p, err)
		}
		if _, err = service.RenameBook(ctx, wantedID, p.Revision); err != nil {
			t.Fatal(err)
		}
	}
	if err = service.VerifyCompletedDownload(ctx, download, nil); err != nil {
		t.Fatal("receipt after repeated folder moves", err)
	}
}

func TestBookRenameRejectsUnknownFilesAndUnsafeReferences(t *testing.T) {
	for _, scenario := range []string{"unrecorded file", "absolute playlist", "escaping playlist", "missing reference", "changed content", "stale preview"} {
		t.Run(scenario, func(t *testing.T) {
			service, download, wantedID := audiobookFixture(t)
			ctx := context.Background()
			if strings.Contains(scenario, "playlist") || scenario == "missing reference" {
				reference := "/unowned/book.mp3"
				if scenario == "escaping playlist" {
					reference = "../../other.mp3"
				}
				if scenario == "missing reference" {
					reference = "missing.mp3"
				}
				if err := os.WriteFile(filepath.Join(download.SavePath, download.Name, "playlist.m3u"), []byte(reference), 0644); err != nil {
					t.Fatal(err)
				}
				service.WithDownloadInspector(newFixtureInspector(t, download))
			}
			imported, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
			if err != nil || imported.Imported != 1 {
				t.Fatal(imported, err)
			}
			if _, err = service.store.db.Exec(`update wanted_items set title='Changed Book' where id=$1`, wantedID); err != nil {
				t.Fatal(err)
			}
			original := mustOperation(t, service, imported.Results[0].Import.OperationID)
			if scenario == "unrecorded file" {
				if err = os.WriteFile(filepath.Join(filepath.Dir(original.Files[len(original.Files)-1].DestinationPath), "private-notes.txt"), []byte("keep"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "changed content" {
				if err = os.WriteFile(original.Files[0].DestinationPath, []byte("changed"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			p, err := service.PreviewBookRename(ctx, wantedID)
			if scenario == "stale preview" {
				if err != nil {
					t.Fatal(err)
				}
				if _, err = service.store.db.Exec(`update wanted_items set title='Another owner correction',updated_at=now() where id=$1`, wantedID); err != nil {
					t.Fatal(err)
				}
				if _, err = service.RenameBook(ctx, wantedID, p.Revision); err == nil {
					t.Fatal("stale folder preview applied")
				}
			} else if err == nil {
				t.Fatal("unsafe folder preview accepted", p)
			}
			for _, f := range original.Files {
				if _, err = os.Stat(f.DestinationPath); err != nil {
					t.Fatal("original removed", err)
				}
			}
			var count int
			if err = service.store.db.QueryRow(`select count(*) from file_rename_claims`).Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
		})
	}
}

func TestBookRenameFencesChangedMembership(t *testing.T) {
	service, download, wantedID := audiobookFixture(t)
	ctx := context.Background()
	imported, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || imported.Imported != 1 {
		t.Fatal(imported, err)
	}
	if _, err = service.store.db.Exec(`update wanted_items set title='Renamed Folder' where id=$1`, wantedID); err != nil {
		t.Fatal(err)
	}
	_, plan, err := service.planBookRename(ctx, wantedID)
	if err != nil {
		t.Fatal(err)
	}
	op, err := service.store.planOperation(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	first := imported.Results[0].Import.Files[0]
	if _, err = service.store.db.Exec(`delete from file_wanted_links where file_id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.runImportOperation(ctx, op); err == nil {
		t.Fatal("changed book membership committed")
	}
	for _, f := range op.Files {
		if _, err = os.Stat(f.SourcePath); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = service.store.db.Exec(`insert into file_wanted_links(file_id,wanted_item_id) values($1,$2)`, first.ID, wantedID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.runImportOperation(ctx, mustOperation(t, service, op.ID)); err != nil {
		t.Fatal(err)
	}
}

func TestBookRenameRecoversPartiallyRemovedSources(t *testing.T) {
	service, download, wantedID := audiobookFixture(t)
	ctx := context.Background()
	imported, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || imported.Imported != 1 {
		t.Fatal(imported, err)
	}
	if _, err = service.store.db.Exec(`update wanted_items set title='Renamed Folder' where id=$1`, wantedID); err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewBookRename(ctx, wantedID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.store.db.Exec(`create function reject_partial_rename_cleanup() returns trigger language plpgsql as $$ begin if new.source_removed and new.file_order=2 then raise exception 'controlled source removal bookkeeping failure'; end if; return new; end $$; create trigger partial_rename_cleanup before update on import_operation_files for each row execute function reject_partial_rename_cleanup()`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RenameBook(ctx, wantedID, preview.Revision); err == nil {
		t.Fatal("cleanup failure hidden")
	}
	p, err := service.PreviewBookRename(ctx, wantedID)
	if err != nil || p.OperationID == "" {
		t.Fatal(p, err)
	}
	op := mustOperation(t, service, p.OperationID)
	if op.State != "committed" || op.CleanupState != "blocked" {
		t.Fatal(op)
	}
	for _, f := range op.Files {
		if err = verifyManifestPath(f.DestinationPath, f); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = os.Stat(op.Files[0].SourcePath); !os.IsNotExist(err) {
		t.Fatal("first source was not removed", err)
	}
	if _, err = os.Stat(op.Files[3].SourcePath); err != nil {
		t.Fatal("later source unexpectedly removed", err)
	}
	var state string
	var present, required int
	if err = service.store.db.QueryRow(`select file_state,present_files,required_files from librarry_book_file_evidence(array[$1::uuid])`, wantedID).Scan(&state, &present, &required); err != nil || state != "present" || present != 4 || required != 4 {
		t.Fatal(state, present, required, err)
	}
	if _, err = service.store.db.Exec(`drop trigger partial_rename_cleanup on import_operation_files`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RetryImportOperation(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	for _, f := range op.Files {
		if _, err = os.Stat(f.SourcePath); !os.IsNotExist(err) {
			t.Fatal("old source retained", err)
		}
	}
}

func TestBookRenameFollowsVerifiedScanMove(t *testing.T) {
	service, download, wantedID := audiobookFixture(t)
	ctx := context.Background()
	imported, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || imported.Imported != 1 {
		t.Fatal(imported, err)
	}
	if _, err = service.Scan(ctx, ScanRequest{Root: service.Config().AudiobookRoot}); err != nil {
		t.Fatal(err)
	}
	first := imported.Results[0].Import.Files[0]
	moved := filepath.Join(filepath.Dir(first.Path), "Renamed chapter.mp3")
	if err = os.Rename(first.Path, moved); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Scan(ctx, ScanRequest{Root: service.Config().AudiobookRoot}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = service.store.db.QueryRow(`select count(*) from library_scan_moves where file_id=$1`, first.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if _, err = service.store.db.Exec(`update wanted_items set title='Renamed Folder' where id=$1`, wantedID); err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewBookRename(ctx, wantedID)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Files[0].RelativePath != filepath.Join("Disc 1", "Renamed chapter.mp3") {
		t.Fatal(preview.Files[0])
	}
	if _, err = service.RenameBook(ctx, wantedID, preview.Revision); err != nil {
		t.Fatal(err)
	}
	if err = service.VerifyCompletedDownload(ctx, download, nil); err != nil {
		t.Fatal(err)
	}
}

func TestBookRenameIncludesChaptersBeyondPresentationPage(t *testing.T) {
	service, download, wantedID := audiobookFixture(t)
	ctx := context.Background()
	for i := 0; i < 147; i++ {
		path := filepath.Join(download.SavePath, download.Name, "Disc 2", fmt.Sprintf("chapter-%03d.mp3", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("unique chapter %d", i)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	service.WithDownloadInspector(newFixtureInspector(t, download))
	imported, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || imported.Imported != 1 {
		t.Fatal(imported, err)
	}
	if _, err = service.store.db.Exec(`update wanted_items set title='Long Renamed Book',updated_at=now() where id=$1`, wantedID); err != nil {
		t.Fatal(err)
	}
	p, err := service.PreviewBookRename(ctx, wantedID)
	if err != nil || p.MediaFiles != 151 || p.CompanionFiles != 2 {
		t.Fatal(p, err)
	}
	result, err := service.RenameBook(ctx, wantedID, p.Revision)
	if err != nil || len(result.Files) != 151 {
		t.Fatal(result, err)
	}
	for i, file := range result.Files {
		if file.ID != imported.Results[0].Import.Files[i].ID {
			t.Fatal("chapter order or identity changed", i)
		}
	}
	for _, file := range p.Files {
		if _, err = os.Stat(file.SourcePath); !os.IsNotExist(err) {
			t.Fatal("source retained", err)
		}
		if err = verifyManifestPath(file.DestinationPath, file); err != nil {
			t.Fatal(err)
		}
	}
	if err = service.VerifyCompletedDownload(ctx, download, nil); err != nil {
		t.Fatal(err)
	}
}

func TestBookRenameRetainsConflictingOwnershipAndLayouts(t *testing.T) {
	for _, scenario := range []string{"shared book file", "tracked companion", "Calibre file", "unknown target file", "source symlink"} {
		t.Run(scenario, func(t *testing.T) {
			service, download, wantedID := audiobookFixture(t)
			ctx := context.Background()
			imported, err := service.ImportCompletedDownloads(ctx, []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
			if err != nil || imported.Imported != 1 {
				t.Fatal(imported, err)
			}
			if _, err = service.store.db.Exec(`update wanted_items set title='Renamed Conflict Book' where id=$1`, wantedID); err != nil {
				t.Fatal(err)
			}
			preview, err := service.PreviewBookRename(ctx, wantedID)
			if err != nil {
				t.Fatal(err)
			}
			first := imported.Results[0].Import.Files[0]
			switch scenario {
			case "shared book file":
				_, err = service.store.db.Exec(`with other as (insert into wanted_items(wanted_format,title) values('audiobook','Other book') returning id) insert into file_wanted_links(file_id,wanted_item_id) select $1,id from other`, first.ID)
			case "tracked companion":
				for _, file := range preview.Files {
					if file.Format == "sidecar" {
						_, err = service.store.db.Exec(`insert into files(media_format,path,import_status) values('ebook',$1,'imported')`, file.SourcePath)
						break
					}
				}
			case "Calibre file":
				_, err = service.store.db.Exec(`update files set metadata=metadata||'{"calibreId":7}'::jsonb where id=$1`, first.ID)
			case "unknown target file":
				err = os.MkdirAll(preview.DestinationFolder, 0755)
				if err == nil {
					err = os.WriteFile(filepath.Join(preview.DestinationFolder, "owner-notes.txt"), []byte("retain"), 0644)
				}
			case "source symlink":
				err = os.Symlink(t.TempDir(), filepath.Join(preview.SourceFolder, "external"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = service.RenameBook(ctx, wantedID, preview.Revision); err == nil {
				t.Fatal("unsafe book layout moved")
			}
			for _, file := range preview.Files {
				if err = verifyManifestPath(file.SourcePath, file); err != nil {
					t.Fatal("original changed", err)
				}
			}
		})
	}
}
