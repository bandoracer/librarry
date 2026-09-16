package library

import (
	"context"
	"database/sql"
	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"testing"
)

func attachImportSelection(t *testing.T, db *sql.DB, wantedID string) string {
	t.Helper()
	var releaseID, intentID string
	if err := db.QueryRow(`insert into releases(wanted_item_id,indexer,title,protocol,download_url,score) values($1,'Fixture','Selected','torrent','',999) returning id::text`, wantedID).Scan(&releaseID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`insert into acquisition_intents(scope_key,request_key,wanted_item_id,client,endpoint_hash,state,external_id,selection,bookkeeping_at) values(repeat('1',64),repeat('2',64),$1,'qBittorrent','fixture','accepted','fixture',jsonb_build_object('releaseId',$2::text,'score',70.5),now()) returning id::text`, wantedID, releaseID).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update downloads set acquisition_intent_id=$1,release_id=$2 where client='qBittorrent' and external_id='fixture'`, intentID, releaseID); err != nil {
		t.Fatal(err)
	}
	return releaseID
}

func TestImportCommitsInstalledReleaseAndHistoryAtomically(t *testing.T) {
	s, db, download, wantedID := operationFixture(t)
	selected := attachImportSelection(t, db, wantedID)
	if _, err := db.Exec(`alter table history_events add constraint fail_import_history check(event_type<>'book_imported')`); err != nil {
		t.Fatal(err)
	}
	result, err := s.ImportCompletedDownloads(context.Background(), []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || result.Errored != 1 {
		t.Fatalf("failure: %+v %v", result, err)
	}
	var count int
	if err := db.QueryRow(`select count(*) from wanted_items where current_release_id is not null or status='imported'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial installed state", count, err)
	}
	if err := db.QueryRow(`select count(*) from files`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if _, err := db.Exec(`alter table history_events drop constraint fail_import_history`); err != nil {
		t.Fatal(err)
	}
	op, err := s.store.operationForDownload(context.Background(), download.Client, download.ID)
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewService(NewStore(db), s.Config(), s.wanted, nil).WithDownloadInspector(newFixtureInspector(t, download))
	imported, err := restarted.RetryImportOperation(context.Background(), op.ID)
	if err != nil || !imported.Imported {
		t.Fatal(imported, err)
	}
	repeat, err := restarted.RetryImportOperation(context.Background(), op.ID)
	if err != nil || !repeat.Skipped {
		t.Fatal(repeat, err)
	}
	var releaseID string
	var score float64
	if err := db.QueryRow(`select current_release_id::text,current_release_score from wanted_items where id=$1`, wantedID).Scan(&releaseID, &score); err != nil || releaseID != selected || score != 70.5 {
		t.Fatal(releaseID, score, err)
	}
	if err := db.QueryRow(`select count(*) from history_events where event_type='book_imported' and data->>'releaseId'=$1 and data->>'operationId'=$2 and jsonb_array_length(data->'fileIds')=1`, selected, op.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("missing/duplicate import history", count, err)
	}
}

func TestImportDoesNotBorrowAnotherBooksReleaseOrCommitPendingBookkeeping(t *testing.T) {
	for _, scenario := range []string{"pending", "foreign"} {
		t.Run(scenario, func(t *testing.T) {
			s, db, download, wantedID := operationFixture(t)
			selected := attachImportSelection(t, db, wantedID)
			if scenario == "pending" {
				if _, err := db.Exec(`update acquisition_intents set bookkeeping_at=null`); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := db.Exec(`update releases set wanted_item_id=null where id=$1`, selected); err != nil {
					t.Fatal(err)
				}
			}
			result, err := s.ImportCompletedDownloads(context.Background(), []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "pending" {
				if result.Errored != 1 {
					t.Fatal("pending acquisition imported", result)
				}
				return
			}
			if result.Imported != 1 {
				t.Fatal(result)
			}
			var releaseID string
			var score float64
			if err := db.QueryRow(`select coalesce(current_release_id::text,''),current_release_score from wanted_items`).Scan(&releaseID, &score); err != nil || releaseID != "" || score != 0 {
				t.Fatal("borrowed foreign release", releaseID, score, err)
			}
		})
	}
}
