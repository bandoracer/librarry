package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func ageEvent(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if _, err := db.Exec(`update notification_events set created_at=now()-interval '100 days',compat_context='{"fixture":"saved context"}' where id=$1`, id); err != nil {
		t.Fatal(err)
	}
}
func ageResolution(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if _, err := db.Exec(`update notification_deliveries set resolved_at=now()-interval '91 days' where id=$1`, id); err != nil {
		t.Fatal(err)
	}
}

func TestRetentionCompactsResolvedHistoryWithoutReplayingEvent(t *testing.T) {
	var sends atomic.Int32
	s, db, target := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { sends.Add(1); w.WriteHeader(204) })
	fixtureHistory(t, db, "book_imported")
	runOutbox(t, s)
	d := onlyDelivery(t, s)
	if d.ResolvedAt == nil {
		t.Fatal("acceptance did not start retention", d)
	}
	ageEvent(t, db, d.EventID)
	ageResolution(t, db, d.ID)
	var source string
	if err := db.QueryRow(`select source_key from notification_events where id=$1`, d.EventID).Scan(&source); err != nil {
		t.Fatal(err)
	}
	report, err := s.PruneHistory(context.Background())
	if err != nil || report.Events != 1 || report.Deliveries != 1 || report.Accepted != 1 || report.Attempts != 1 {
		t.Fatal(report, err)
	}
	var archived bool
	var payload, compat, summary string
	if err = db.QueryRow(`select archived_at is not null,event::text,compat_context::text,retention_summary::text from notification_events where id=$1 and source_key=$2`, d.EventID, source).Scan(&archived, &payload, &compat, &summary); err != nil || !archived || payload != "{}" || compat != "{}" {
		t.Fatal(archived, payload, compat, err)
	}
	var receipt RetentionReport
	if err = json.Unmarshal([]byte(summary), &receipt); err != nil || receipt.Deliveries != 1 || receipt.Attempts != 1 {
		t.Fatal(summary, err)
	}
	target.ID = ""
	target.Name = "Added after compaction"
	if _, err = s.CreateTarget(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`select enqueue_native_notification($1,'{"type":"import","title":"Old event replay"}')`, source); err != nil {
		t.Fatal(err)
	}
	runOutbox(t, s)
	page, err := s.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 0 || sends.Load() != 1 {
		t.Fatal(page, sends.Load(), err)
	}
	var histories int
	if err = db.QueryRow(`select count(*) from history_events where event_type='book_imported'`).Scan(&histories); err != nil || histories != 1 {
		t.Fatal("domain history altered", histories, err)
	}
}

func TestRetentionPreservesUnresolvedAndRecentlyResolvedStates(t *testing.T) {
	for _, state := range []string{"pending", "sending", "retry", "failed", "uncertain", "cancelled", "accepted"} {
		t.Run(state, func(t *testing.T) {
			s, db, _ := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("maintenance contacted receiver") })
			fixtureHistory(t, db, "book_imported")
			d := onlyDelivery(t, s)
			ageEvent(t, db, d.EventID)
			if _, err := db.Exec(`update notification_deliveries set state=$2,updated_at=now()-interval '100 days',resolved_at=case when $2='accepted' then now() else null end where id=$1`, d.ID, state); err != nil {
				t.Fatal(err)
			}
			report, err := s.PruneHistory(context.Background())
			if err != nil || report.Events != 0 {
				t.Fatal(report, err)
			}
			if onlyDelivery(t, s).State != state {
				t.Fatal("state changed")
			}
		})
	}
}

func TestRetentionRequiresEveryRecipientResolvedAndKeepsReviewWindow(t *testing.T) {
	s, db, _ := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("maintenance contacted receiver") })
	if _, err := db.Exec(`insert into compat_resources(resource_type,compat_id,name,payload) values('notification',98765,'Fixture webhook','{"enable":true,"onReleaseImport":true}')`); err != nil {
		t.Fatal(err)
	}
	fixtureHistory(t, db, "book_imported")
	page, err := s.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 2 {
		t.Fatal(page, err)
	}
	var stopped Delivery
	for _, d := range page.Items {
		ageEvent(t, db, d.EventID)
		if d.TargetKind == "native" {
			if _, err = db.Exec(`update notification_deliveries set state='accepted',resolved_at=now()-interval '100 days' where id=$1`, d.ID); err != nil {
				t.Fatal(err)
			}
		} else {
			stopped = d
			if _, err = db.Exec(`update notification_deliveries set state='cancelled',updated_at=now()-interval '100 days' where id=$1`, d.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	report, err := s.PruneHistory(context.Background())
	if err != nil || report.Events != 0 {
		t.Fatal("automated stop is not reviewed", report, err)
	}
	page, err = s.Deliveries(context.Background(), 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range page.Items {
		if d.ID == stopped.ID {
			stopped = d
		}
	}
	if err = s.ResolveDelivery(context.Background(), stopped.ID, resolution(stopped, "cancel")); err != nil {
		t.Fatal(err)
	}
	report, err = s.PruneHistory(context.Background())
	if err != nil || report.Events != 0 {
		t.Fatal("review window shortened", report, err)
	}
	ageResolution(t, db, stopped.ID)
	report, err = s.PruneHistory(context.Background())
	if err != nil || report.Events != 1 || report.Accepted != 1 || report.Cancelled != 1 || report.Actions != 1 {
		t.Fatal(report, err)
	}
}

func TestRetentionSkipsOwnersAndRechecksRetry(t *testing.T) {
	s, db, _ := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("maintenance contacted receiver") })
	fixtureHistory(t, db, "book_imported")
	d := onlyDelivery(t, s)
	ageEvent(t, db, d.EventID)
	if err := s.ResolveDelivery(context.Background(), d.ID, resolution(d, "cancel")); err != nil {
		t.Fatal(err)
	}
	ageResolution(t, db, d.ID)
	conn, err := s.lockDelivery(context.Background(), d.ID)
	if err != nil {
		t.Fatal(err)
	}
	report, err := s.PruneHistory(context.Background())
	releaseDelivery(conn, d.ID)
	if err != nil || report.Events != 0 || report.Skipped != 1 {
		t.Fatal(report, err)
	}
	d = onlyDelivery(t, s)
	if err = s.ResolveDelivery(context.Background(), d.ID, resolution(d, "retry")); err != nil {
		t.Fatal(err)
	}
	d = onlyDelivery(t, s)
	if d.State != "pending" || d.ResolvedAt != nil {
		t.Fatal("retry retained resolution", d)
	}
	// A candidate selected before retry must be revalidated under the locks.
	report, err = s.pruneEvent(context.Background(), d.EventID)
	if err != nil || report.Events != 0 || report.Skipped != 1 {
		t.Fatal(report, err)
	}
	if err = s.ResolveDelivery(context.Background(), d.ID, resolution(d, "cancel")); err != nil {
		t.Fatal(err)
	}
	ageResolution(t, db, d.ID)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`select id from notification_events where id=$1 for update`, d.EventID); err != nil {
		t.Fatal(err)
	}
	report, err = s.PruneHistory(context.Background())
	tx.Rollback()
	if err != nil || report.Skipped != 1 {
		t.Fatal(report, err)
	}
	var wg sync.WaitGroup
	reports := make(chan RetentionReport, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := s.PruneHistory(context.Background()); reports <- r; errs <- e }()
	}
	wg.Wait()
	close(reports)
	close(errs)
	total := 0
	for r := range reports {
		total += r.Events
	}
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if total != 1 {
		t.Fatal("duplicate compaction", total)
	}
}

func TestRetentionRollbackAndBatchBound(t *testing.T) {
	s, db, _ := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	fixtureHistory(t, db, "book_imported")
	runOutbox(t, s)
	d := onlyDelivery(t, s)
	ageEvent(t, db, d.EventID)
	ageResolution(t, db, d.ID)
	if _, err := db.Exec(`create function reject_archive() returns trigger language plpgsql as $$ begin raise exception 'fixture archive unavailable';end $$;create trigger reject_archive before update on notification_events for each row execute function reject_archive()`); err != nil {
		t.Fatal(err)
	}
	report, err := s.PruneHistory(context.Background())
	if err == nil || report.Events != 0 {
		t.Fatal(report, err)
	}
	if onlyDelivery(t, s).State != "accepted" {
		t.Fatal("rollback lost receipt")
	}
	var attempts int
	if err = db.QueryRow(`select count(*) from notification_delivery_attempts`).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatal(attempts, err)
	}
	if _, err = db.Exec(`drop trigger reject_archive on notification_events; insert into notification_events(source_key,event,created_at) select 'no-recipients:'||n,'{"type":"import"}',now()-interval '101 days' from generate_series(1,105)n`); err != nil {
		t.Fatal(err)
	}
	report, err = s.PruneHistory(context.Background())
	if err != nil || report.Events != 100 || report.Deliveries != 0 {
		t.Fatal(report, err)
	}
	report, err = s.PruneHistory(context.Background())
	if err != nil || report.Events != 6 || report.Deliveries != 1 {
		t.Fatal(report, err)
	}
	report, err = s.PruneHistory(context.Background())
	if err != nil || report.Events != 0 {
		t.Fatal(report, err)
	}
}

func TestRetentionUpgradePreservesUnreviewedLegacyCancellations(t *testing.T) {
	db := testdb.OpenThrough(t, "0050_worker_diagnostics.sql")
	if _, err := db.Exec(`insert into notification_events(source_key,event) values('legacy-retention','{"type":"import"}');insert into notification_deliveries(event_id,target_id,target_name,target_type,target_revision,state,updated_at) select e.id,gen_random_uuid(),'Legacy','webhook',now(),s,now()-interval '100 days' from notification_events e cross join unnest(array['accepted','cancelled','failed','uncertain'])s where e.source_key='legacy-retention'`); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../migrations/0051_notification_retention.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(raw)); err != nil {
		t.Fatal(err)
	}
	var accepted, others int
	if err = db.QueryRow(`select count(*) filter(where state='accepted' and resolved_at=updated_at),count(*) filter(where state<>'accepted' and resolved_at is null) from notification_deliveries`).Scan(&accepted, &others); err != nil || accepted != 1 || others != 3 {
		t.Fatal(accepted, others, err)
	}
}
