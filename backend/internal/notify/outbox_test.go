package notify

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func outboxFixture(t *testing.T, handler http.HandlerFunc) (*Service, *sql.DB, Target) {
	t.Helper()
	db := testdb.Open(t)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	s := NewService(NewStore(db), nil)
	target, err := s.CreateTarget(context.Background(), Target{Name: "Fixture", Type: TargetTypeWebhook, Settings: map[string]string{"url": server.URL + "/secret-url", "authorization": "Bearer fixture-secret"}, Enabled: true, Triggers: DefaultTriggers()})
	if err != nil {
		t.Fatal(err)
	}
	return s, db, target
}
func fixtureHistory(t *testing.T, db *sql.DB, kind string) {
	t.Helper()
	if _, err := db.Exec(`insert into history_events(event_type,message,data) values($1,'Fixture committed', '{"title":"Walden","trigger":"upgrade","paths":["/library/Walden.epub"],"downloadUrl":"https://secret.invalid/token"}')`, kind); err != nil {
		t.Fatal(err)
	}
}
func onlyDelivery(t *testing.T, s *Service) Delivery {
	t.Helper()
	page, err := s.Deliveries(context.Background(), 25, 0)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page: %+v %v", page, err)
	}
	return page.Items[0]
}
func runOutbox(t *testing.T, s *Service) {
	t.Helper()
	if _, err := s.RunPending(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func resolution(d Delivery, action string) DeliveryResolution {
	return DeliveryResolution{Action: action, Confirm: true, ExpectedUpdatedAt: d.UpdatedAt, ExpectedTargetRevision: d.CurrentTargetRevision}
}

func TestOutboxAtomicCaptureAndTargetSnapshot(t *testing.T) {
	var sends atomic.Int32
	s, db, target := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		if r.Header.Get("Idempotency-Key") != "" {
			t.Error("transport may replay POSTs marked idempotent")
		}
		if r.Header.Get("X-Librarry-Delivery-ID") == "" {
			t.Error("missing delivery identity")
		}
		w.WriteHeader(204)
	})
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`insert into history_events(event_type,message) values('release_grabbed','rolled back')`); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	page, err := s.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 0 {
		t.Fatal(page, err)
	}
	fixtureHistory(t, db, "release_grabbed")
	d := onlyDelivery(t, s)
	if d.State != "pending" || d.Event.Type != EventUpgrade || strings.Contains(d.Event.Message, "secret") {
		t.Fatal(d)
	}
	var body string
	if err = db.QueryRow(`select event::text from notification_events`).Scan(&body); err != nil || strings.Contains(body, "secret.invalid") {
		t.Fatal("unfiltered history leaked", body, err)
	}
	target.ID = ""
	target.Name = "Later connection"
	if _, err = s.CreateTarget(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	restarted := NewService(NewStore(db), nil)
	runOutbox(t, restarted)
	runOutbox(t, restarted)
	d = onlyDelivery(t, s)
	if d.State != "accepted" || d.Attempts != 1 || sends.Load() != 1 {
		t.Fatal(d, sends.Load())
	}
}
func TestOutboxChangedDisabledDeletedTargetsRequireReview(t *testing.T) {
	for _, change := range []string{"changed", "disabled", "deleted"} {
		t.Run(change, func(t *testing.T) {
			var sends atomic.Int32
			s, db, target := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { sends.Add(1); w.WriteHeader(204) })
			fixtureHistory(t, db, "book_imported")
			if change == "deleted" {
				if err := s.DeleteTarget(context.Background(), target.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				target.Name = "Changed"
				if change == "disabled" {
					target.Enabled = false
				}
				if _, err := s.UpdateTarget(context.Background(), target.ID, target); err != nil {
					t.Fatal(err)
				}
			}
			runOutbox(t, s)
			d := onlyDelivery(t, s)
			if d.State != "cancelled" || sends.Load() != 0 {
				t.Fatal(d)
			}
			request := resolution(d, "retry")
			request.Confirm = false
			if err := s.ResolveDelivery(context.Background(), d.ID, request); !errors.Is(err, ErrDeliveryConfirmation) {
				t.Fatal(err)
			}
			if change == "changed" {
				request = resolution(d, "retry")
				if err := s.ResolveDelivery(context.Background(), d.ID, request); err != nil {
					t.Fatal(err)
				}
				if err := s.ResolveDelivery(context.Background(), d.ID, request); !errors.Is(err, ErrDeliveryConflict) {
					t.Fatal("stale resolution accepted", err)
				}
				runOutbox(t, s)
				if sends.Load() != 1 {
					t.Fatal(sends.Load())
				}
			} else if err := s.ResolveDelivery(context.Background(), d.ID, resolution(d, "retry")); !errors.Is(err, ErrDeliveryConflict) {
				t.Fatal(err)
			}
		})
	}
}
func TestOutboxUncertainAcceptanceDoesNotResend(t *testing.T) {
	var sends atomic.Int32
	s, db, _ := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		conn.Close()
	})
	fixtureHistory(t, db, "release_grabbed")
	runOutbox(t, s)
	d := onlyDelivery(t, s)
	if d.State != "uncertain" || d.Attempts != 1 || strings.Contains(d.Message, "secret") {
		t.Fatal(d)
	}
	runOutbox(t, NewService(NewStore(db), nil))
	if sends.Load() != 1 {
		t.Fatal("blind resend", sends.Load())
	}
	if err := s.ResolveDelivery(context.Background(), d.ID, resolution(d, "accepted")); err != nil {
		t.Fatal(err)
	}
	if onlyDelivery(t, s).State != "accepted" || sends.Load() != 1 {
		t.Fatal("confirmation sent request")
	}
	var actions int
	if err := db.QueryRow(`select count(*) from notification_delivery_actions where action='accepted' and previous_state='uncertain'`).Scan(&actions); err != nil || actions != 1 {
		t.Fatal(actions, err)
	}
}
func TestOutboxRejectsRedirectAndBoundsRateRetries(t *testing.T) {
	for _, code := range []int{302, 400, 429, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var sends atomic.Int32
			s, db, _ := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) {
				sends.Add(1)
				w.Header().Set("Location", "http://127.0.0.1:1/credential")
				w.Header().Set("Retry-After", "3600")
				w.WriteHeader(code)
			})
			fixtureHistory(t, db, "book_imported")
			runOutbox(t, s)
			d := onlyDelivery(t, s)
			expected := "failed"
			if code == 429 {
				expected = "retry"
				if time.Until(d.NextAttemptAt) < 59*time.Minute {
					t.Fatal("ignored backoff", d)
				}
			}
			if code == 500 {
				expected = "uncertain"
			}
			if d.State != expected || sends.Load() != 1 {
				t.Fatal(d, sends.Load())
			}
			if code == 429 {
				for range 4 {
					if _, err := db.Exec(`update notification_deliveries set next_attempt_at=now()-interval '1 second'`); err != nil {
						t.Fatal(err)
					}
					runOutbox(t, s)
				}
				d = onlyDelivery(t, s)
				if d.State != "failed" || d.Attempts != 5 {
					t.Fatal(d)
				}
			}
		})
	}
}
func TestOutboxConcurrentSendAndLostSession(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var sends atomic.Int32
	s, db, _ := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		close(entered)
		<-release
		w.WriteHeader(204)
	})
	fixtureHistory(t, db, "release_grabbed")
	d := onlyDelivery(t, s)
	done := make(chan error, 1)
	go func() { done <- s.processDelivery(context.Background(), d.ID) }()
	<-entered
	if err := NewService(NewStore(db), nil).processDelivery(context.Background(), d.ID); !errors.Is(err, ErrDeliveryConflict) {
		t.Fatal(err)
	}
	if err := s.ResolveDelivery(context.Background(), d.ID, resolution(d, "cancel")); !errors.Is(err, ErrDeliveryConflict) {
		t.Fatal("active delivery resolved", err)
	}
	// Kill only the database session holding this fixture's advisory lock.
	var killed bool
	if err := db.QueryRow(`select pg_terminate_backend(pid) from pg_locks where locktype='advisory' and database=(select oid from pg_database where datname=current_database()) and classid::bigint=((hashtextextended('librarry-notification:'||$1,0)>>32)&4294967295) and objid::bigint=(hashtextextended('librarry-notification:'||$1,0)&4294967295) and objsubid=1`, d.ID).Scan(&killed); err != nil || !killed {
		t.Fatal(err)
	}
	runOutbox(t, NewService(NewStore(db), nil))
	if onlyDelivery(t, s).State != "uncertain" {
		t.Fatal("lost sender not recognized")
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("stale sender saved result")
	}
	if d = onlyDelivery(t, s); d.State != "uncertain" || sends.Load() != 1 {
		t.Fatal(d, sends.Load())
	}
}
func TestOutboxAcceptedResponseWithFailedSaveRequiresReview(t *testing.T) {
	var sends atomic.Int32
	s, db, _ := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { sends.Add(1); w.WriteHeader(204) })
	fixtureHistory(t, db, "book_imported")
	if _, err := db.Exec(`alter table notification_deliveries add constraint fixture_save_failure check(state<>'accepted')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunPending(context.Background()); err == nil {
		t.Fatal("save failure hidden")
	}
	if _, err := db.Exec(`alter table notification_deliveries drop constraint fixture_save_failure`); err != nil {
		t.Fatal(err)
	}
	runOutbox(t, NewService(NewStore(db), nil))
	d := onlyDelivery(t, s)
	if d.State != "uncertain" || sends.Load() != 1 {
		t.Fatal(d, sends.Load())
	}
}
func TestOutboxHealthEpisodesPersistAcrossServices(t *testing.T) {
	s, db, target := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	target.Triggers.OnHealthIssue = true
	if _, err := s.UpdateTarget(context.Background(), target.ID, target); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := NewService(NewStore(db), nil).ObserveHealth(context.Background(), "disk", "warning", "Disk", "Low disk"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	d := onlyDelivery(t, s)
	if d.Event.Type != EventHealthIssue {
		t.Fatal(d)
	}
	for _, severity := range []string{"error", "ok", "warning"} {
		if err := s.ObserveHealth(context.Background(), "disk", severity, "Disk", "Fixture"); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 2 {
		t.Fatal(page, err)
	}
}
func TestOutboxFailureTransitionsAndFanoutFiltering(t *testing.T) {
	s, db, target := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	target.Triggers.OnDownloadFailure = false
	if _, err := s.UpdateTarget(context.Background(), target.ID, target); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := db.QueryRow(`insert into downloads(client,external_id,category,save_path,state,name,failed_at,failure_reason) values('fixture','one','','','error','Walden',now(),'Fixture failure') returning id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	page, err := s.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 0 {
		t.Fatal(page, err)
	}
	target.Triggers.OnDownloadFailure = true
	if _, err := s.UpdateTarget(context.Background(), target.ID, target); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update downloads set failed_at=coalesce(failed_at,now()) where id=$1`, id); err != nil {
		t.Fatal(err)
	}
	page, err = s.Deliveries(context.Background(), 25, 0)
	if err != nil || page.Total != 0 {
		t.Fatal("new target got old failure", page, err)
	}
	if _, err := db.Exec(`update downloads set failed_at=null where id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update downloads set failed_at=now() where id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if d := onlyDelivery(t, s); d.Event.Type != EventDownloadFailure {
		t.Fatal(d)
	}
}

func TestOutboxHealthCaptureFailureDoesNotConsumeTransition(t *testing.T) {
	s, db, target := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	target.Triggers.OnHealthIssue = true
	if _, err := s.UpdateTarget(context.Background(), target.ID, target); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`alter table notification_events add constraint fixture_enqueue_failure check(event->>'type'<>'healthIssue')`); err != nil {
		t.Fatal(err)
	}
	if err := s.ObserveHealth(context.Background(), "disk", "warning", "Disk", "Fixture"); err == nil {
		t.Fatal("lost enqueue failure")
	}
	var states int
	if err := db.QueryRow(`select count(*) from notification_health_states`).Scan(&states); err != nil || states != 0 {
		t.Fatal("consumed failed transition", states, err)
	}
	if _, err := db.Exec(`alter table notification_events drop constraint fixture_enqueue_failure`); err != nil {
		t.Fatal(err)
	}
	if err := s.ObserveHealth(context.Background(), "disk", "warning", "Disk", "Fixture"); err != nil {
		t.Fatal(err)
	}
	if d := onlyDelivery(t, s); d.Event.Type != EventHealthIssue {
		t.Fatal(d)
	}
}

func TestOutboxExplicitRetryKeepsIdentityAndAttemptAudit(t *testing.T) {
	var attempt atomic.Int32
	ids := make(chan string, 2)
	s, db, target := outboxFixture(t, func(w http.ResponseWriter, r *http.Request) {
		ids <- r.Header.Get("X-Librarry-Delivery-ID")
		if attempt.Add(1) == 1 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(204)
	})
	// Disabled and non-matching connections are excluded at capture time.
	target.ID = ""
	target.Name = "Disabled"
	target.Enabled = false
	if _, err := s.CreateTarget(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	target.Name = "Non matching"
	target.Enabled = true
	target.Triggers.OnUpgrade = false
	if _, err := s.CreateTarget(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	fixtureHistory(t, db, "release_grabbed")
	runOutbox(t, s)
	d := onlyDelivery(t, s)
	if d.State != "uncertain" {
		t.Fatal(d)
	}
	if err := s.ResolveDelivery(context.Background(), d.ID, resolution(d, "retry")); err != nil {
		t.Fatal(err)
	}
	runOutbox(t, NewService(NewStore(db), nil))
	d = onlyDelivery(t, s)
	if d.State != "accepted" || d.Attempts != 2 {
		t.Fatal(d)
	}
	if first, second := <-ids, <-ids; first != second || first != d.ID {
		t.Fatal(first, second, d.ID)
	}
	var count int
	if err := db.QueryRow(`select count(*) from notification_delivery_attempts where state in ('uncertain','accepted')`).Scan(&count); err != nil || count != 2 {
		t.Fatal("lost attempt history", count, err)
	}
}

func TestNotificationRetryAfterDoesNotSendBeforeLongServerBackoff(t *testing.T) {
	now := time.Now()
	for _, header := range []string{"86401", now.Add(48 * time.Hour).UTC().Format(http.TimeFormat), "9223372036854775807"} {
		if delay := notificationRetryDelay(header, 1, now); delay <= 24*time.Hour {
			t.Fatal("long wait must require review", header, delay)
		}
	}
	if delay := notificationRetryDelay("invalid", 3, now); delay != 4*time.Minute {
		t.Fatal(delay)
	}
}
