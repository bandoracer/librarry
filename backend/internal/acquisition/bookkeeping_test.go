package acquisition

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
)

func addSelectedRelease(t *testing.T, db *sql.DB, request *DownloadRequest, score float64) string {
	t.Helper()
	wantedID := strings.TrimPrefix(request.Tags[1], "wanted:")
	var id string
	if err := db.QueryRow(`insert into releases(wanted_item_id,indexer,title,protocol,download_url,score,source_id) values($1,'Fixture','Selected fixture','torrent','https://fixture.invalid/secret-not-persisted',$2,'source-1') returning id::text`, wantedID, score).Scan(&id); err != nil {
		t.Fatal(err)
	}
	request.Selection = &AcquisitionSelection{ReleaseID: id, Trigger: "upgrade-worker", Forced: true}
	return id
}

func TestAcceptedAcquisitionRepairsHistoryAfterRestartWithoutRegressingInstalledRelease(t *testing.T) {
	s, db, f, request := intentTestService(t)
	selected := addSelectedRelease(t, db, &request, 82.5)
	if _, err := db.Exec(`insert into notification_targets(name,type,settings) values('Fixture','webhook','{"url":"http://127.0.0.1:1/unused"}')`); err != nil {
		t.Fatal(err)
	}
	wantedID := strings.TrimPrefix(request.Tags[1], "wanted:")
	var old string
	if err := db.QueryRow(`insert into releases(wanted_item_id,indexer,title,protocol,download_url,score) values($1,'Fixture','Installed','torrent','',25) returning id::text`, wantedID).Scan(&old); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update wanted_items set status='imported',current_release_id=$1,current_release_score=25`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`alter table history_events add constraint fail_grab_history check(event_type<>'release_grabbed')`); err != nil {
		t.Fatal(err)
	}
	accepted, err := s.Grab(context.Background(), request)
	if err == nil || accepted.AcquisitionID == "" {
		t.Fatalf("expected recoverable bookkeeping failure: %+v %v", accepted, err)
	}
	pending, err := s.AcquisitionRecovery(context.Background())
	if err != nil || len(pending) != 1 || pending[0].State != "accepted" {
		t.Fatalf("lost accepted receipt: %+v %v", pending, err)
	}
	var n int
	if err := db.QueryRow(`select count(*) from downloads where release_id is not null or acquisition_intent_id is not null`).Scan(&n); err != nil || n != 0 {
		t.Fatal("partial bookkeeping committed", n, err)
	}
	if err := db.QueryRow(`select count(*) from notification_deliveries`).Scan(&n); err != nil || n != 0 {
		t.Fatal("rolled back grab enqueued notification", n, err)
	}
	// A later search must not rewrite the decision already sent to the client.
	if _, err := db.Exec(`update releases set score=999 where id=$1`, selected); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`alter table history_events drop constraint fail_grab_history`); err != nil {
		t.Fatal(err)
	}
	restarted := NewService(s.IntegrationConfig())
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			replay, err := restarted.ReconcileAcquisition(context.Background(), accepted.AcquisitionID, "")
			if err != nil || !replay.Deduplicated || replay.ReleaseID != selected {
				t.Errorf("retry: %+v %v", replay, err)
			}
		}()
	}
	wg.Wait()
	for query, want := range map[string]int{
		`select count(*) from history_events where event_type='release_grabbed' and (data->>'score')::numeric=82.5`: 1,
		`select count(*) from notification_deliveries`:                                                              1,
		`select count(*) from acquisition_intents where bookkeeping_at is not null`:                                 1,
		`select count(*) from downloads where release_id is not null and acquisition_intent_id is not null`:         1,
	} {
		if err := db.QueryRow(query).Scan(&n); err != nil || n != want {
			t.Fatalf("%s: %d %v", query, n, err)
		}
	}
	var state, current string
	var score float64
	if err := db.QueryRow(`select status,current_release_id::text,current_release_score from wanted_items where id=$1`, wantedID).Scan(&state, &current, &score); err != nil || state != "imported" || current != old || score != 25 {
		t.Fatalf("installed state changed: %s %s %v %v", state, current, score, err)
	}
	pending, err = restarted.AcquisitionRecovery(context.Background())
	if err != nil || len(pending) != 0 {
		t.Fatal(pending, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.adds != 1 {
		t.Fatal("duplicate client submission", f.adds)
	}
}

func TestAcquisitionSelectionSurvivesAcknowledgementLoss(t *testing.T) {
	s, db, f, request := intentTestService(t)
	selected := addSelectedRelease(t, db, &request, 71)
	f.failAdd = true
	if _, err := s.Grab(context.Background(), request); err == nil {
		t.Fatal("expected uncertain acceptance")
	}
	if _, err := db.Exec(`update releases set score=999 where id=$1`, selected); err != nil {
		t.Fatal(err)
	}
	readyIntentRecovery(t, db)
	intents, err := s.AcquisitionRecovery(context.Background())
	if err != nil || len(intents) != 1 {
		t.Fatal(intents, err)
	}
	result, err := NewService(s.IntegrationConfig()).ReconcileAcquisition(context.Background(), intents[0].ID, "")
	if err != nil || !result.Deduplicated {
		t.Fatal(result, err)
	}
	var score float64
	var status string
	if err := db.QueryRow(`select (data->>'score')::numeric from history_events where event_type='release_grabbed'`).Scan(&score); err != nil || score != 71 {
		t.Fatal(score, err)
	}
	if err := db.QueryRow(`select status from wanted_items`).Scan(&status); err != nil || status != "grabbed" {
		t.Fatal(status, err)
	}
}

func TestAcquisitionBookkeepingPreservesRemovalAndRecoversMissingProjection(t *testing.T) {
	s, db, f, request := intentTestService(t)
	selected := addSelectedRelease(t, db, &request, 65)
	result, err := s.Grab(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update downloads set state='removed'; update wanted_items set status='removed'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReconcileAcquisition(context.Background(), result.AcquisitionID, ""); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`select state from downloads`).Scan(&state); err != nil || state != "removed" {
		t.Fatal("removed download resurrected", state, err)
	}
	if _, err := db.Exec(`delete from downloads`); err != nil {
		t.Fatal(err)
	}
	recovery, err := s.AcquisitionRecovery(context.Background())
	if err != nil || len(recovery) != 1 {
		t.Fatal(recovery, err)
	}
	if _, err := s.ReconcileAcquisition(context.Background(), result.AcquisitionID, ""); err != nil {
		t.Fatal(err)
	}
	var actual string
	var count int
	if err := db.QueryRow(`select release_id::text from downloads`).Scan(&actual); err != nil || actual != selected {
		t.Fatal(actual, err)
	}
	if err := db.QueryRow(`select status from wanted_items`).Scan(&state); err != nil || state != "removed" {
		t.Fatal(state, err)
	}
	if err := db.QueryRow(`select count(*) from history_events where event_type='release_grabbed'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.adds != 1 {
		t.Fatal(f.adds)
	}
}

func TestAcquisitionRejectsForeignReleaseBeforeSubmitting(t *testing.T) {
	s, db, f, request := intentTestService(t)
	selected := addSelectedRelease(t, db, &request, 60)
	if _, err := db.Exec(`update releases set wanted_item_id=null where id=$1`, selected); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Grab(context.Background(), request); err == nil {
		t.Fatal("foreign release accepted")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.adds != 0 {
		t.Fatal(f.adds)
	}
}
