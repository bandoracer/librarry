package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestSharedWorkerOwnershipAndPersistentDueTime(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	first, second := NewRegistry(testLogger()).WithDatabase(db), NewRegistry(testLogger()).WithDatabase(db)
	task := Task{ID: "fixture", Name: "Fixture", Interval: time.Hour, Run: func(context.Context, string) (string, error) { return "ok", nil }}
	if e := first.Register(task); e != nil {
		t.Fatal(e)
	}
	if e := second.Register(task); e != nil {
		t.Fatal(e)
	}
	claim, e := first.claim(ctx, task, "scheduled-startup")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = second.claim(ctx, task, "manual"); !errors.Is(e, ErrTaskBusy) {
		t.Fatal(e)
	}
	// A very old heartbeat cannot release another process's live lock.
	if _, e = db.Exec(`update worker_task_runs set heartbeat_at=now()-interval '1 day'`); e != nil {
		t.Fatal(e)
	}
	status, e := second.TasksContext(ctx)
	if e != nil || !status[0].Running {
		t.Fatalf("%+v %v", status, e)
	}
	if e = claim.finish("saved outcome", nil); e != nil {
		t.Fatal(e)
	}
	if _, e = second.claim(ctx, task, "scheduled-startup"); !errors.Is(e, errNotDue) {
		t.Fatal("restart repeated scheduled pass", e)
	}
	status, e = second.TasksContext(ctx)
	if e != nil || status[0].Running || status[0].LastOutcome != "saved outcome" || status[0].LastRunAt == nil {
		t.Fatalf("%+v %v", status, e)
	}
	claim, e = second.claim(ctx, task, "manual")
	if e != nil {
		t.Fatal(e)
	}
	if e = claim.finish("manual", nil); e != nil {
		t.Fatal(e)
	}
	runs, e := second.RunHistory(ctx, task.ID)
	if e != nil || len(runs) != 2 || runs[0].Trigger != "manual" {
		t.Fatalf("%+v %v", runs, e)
	}
}
func TestLostWorkerConnectionShowsInterruptedAndCannotOverwriteSuccessor(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	r := NewRegistry(testLogger()).WithDatabase(db)
	task := Task{ID: "lost", Interval: time.Hour, Run: func(context.Context, string) (string, error) { return "", nil }}
	_ = r.Register(task)
	old, e := r.claim(ctx, task, "manual")
	if e != nil {
		t.Fatal(e)
	}
	var killed bool
	if e = db.QueryRow(`select pg_terminate_backend(backend_pid) from worker_task_runs where id=$1`, old.runID).Scan(&killed); e != nil || !killed {
		t.Fatal(e)
	}
	status, e := r.TasksContext(ctx)
	if e != nil || status[0].Running || status[0].RunState != "interrupted" {
		t.Fatalf("%+v %v", status, e)
	}
	next, e := r.claim(ctx, task, "manual")
	if e != nil {
		t.Fatal(e)
	}
	if e = old.finish("stale success", nil); e == nil {
		t.Fatal("old worker wrote success")
	}
	if e = next.finish("new success", nil); e != nil {
		t.Fatal(e)
	}
	runs, e := r.RunHistory(ctx, task.ID)
	if e != nil || len(runs) != 2 || runs[0].Outcome != "new success" || runs[1].State != "interrupted" {
		t.Fatalf("%+v %v", runs, e)
	}
}
func TestTwoRegistriesManualTriggersConvergeAndShutdownWaits(t *testing.T) {
	db := testdb.Open(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	var count atomic.Int32
	task := Task{ID: "shared", Interval: time.Hour, StartupDelay: time.Hour, Run: func(ctx context.Context, _ string) (string, error) {
		count.Add(1)
		close(started)
		<-release
		return "finished", ctx.Err()
	}}
	first, second := NewRegistry(testLogger()).WithDatabase(db), NewRegistry(testLogger()).WithDatabase(db)
	_ = first.Register(task)
	_ = second.Register(task)
	var wg sync.WaitGroup
	first.Start(ctx, &wg)
	second.Start(ctx, &wg)
	if e := first.Trigger(task.ID); e != nil {
		t.Fatal(e)
	}
	<-started
	if e := second.Trigger(task.ID); !errors.Is(e, ErrTaskBusy) {
		t.Fatal(e)
	}
	cancel()
	waited := make(chan struct{})
	go func() { wg.Wait(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("shutdown omitted manual worker")
	case <-time.After(25 * time.Millisecond):
	}
	if e := second.Trigger(task.ID); e == nil {
		t.Fatal("trigger accepted during shutdown")
	}
	close(release)
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	if count.Load() != 1 {
		t.Fatal(count.Load())
	}
	runs, e := first.RunHistory(context.Background(), task.ID)
	if e != nil || runs[0].State != "interrupted" {
		t.Fatalf("%+v %v", runs, e)
	}
}
func TestWorkerPanicReleasesOwnershipAndHistoryIsBounded(t *testing.T) {
	db := testdb.Open(t)
	r := NewRegistry(testLogger()).WithDatabase(db)
	task := Task{ID: "panic", Interval: time.Hour, Run: func(context.Context, string) (string, error) { panic("sensitive panic content") }}
	_ = r.Register(task)
	if e := r.Trigger(task.ID); e != nil {
		t.Fatal(e)
	}
	waitFor(t, 2*time.Second, func() bool { return !r.Tasks()[0].Running })
	runs, e := r.RunHistory(context.Background(), task.ID)
	if e != nil || runs[0].State != "failed" || runs[0].Error != "task panicked; completion is unverified" {
		t.Fatalf("%+v %v", runs, e)
	}
	if _, e = db.Exec(`insert into worker_task_runs(task_id,trigger,backend_pid,state,started_at) select 'panic','fixture',0,'completed',now()+make_interval(secs=>n) from generate_series(1,150)n`); e != nil {
		t.Fatal(e)
	}
	claim, e := r.claim(context.Background(), task, "manual")
	if e != nil {
		t.Fatal(e)
	}
	if e = claim.finish("ok", nil); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = db.QueryRow(`select count(*) from worker_task_runs`).Scan(&count); e != nil || count != 101 {
		t.Fatalf("%d %v", count, e)
	}
}
func TestWorkerDatabaseOutageRefusesRunAndStatus(t *testing.T) {
	db := testdb.Open(t)
	r := NewRegistry(testLogger()).WithDatabase(db)
	var invoked atomic.Bool
	_ = r.Register(Task{ID: "outage", Interval: time.Hour, Run: func(context.Context, string) (string, error) { invoked.Store(true); return "", nil }})
	db.Close()
	if e := r.Trigger("outage"); e == nil {
		t.Fatal("offline claim accepted")
	}
	if _, e := r.TasksContext(context.Background()); e == nil {
		t.Fatal("status invented during outage")
	}
	if invoked.Load() {
		t.Fatal("task invoked without persistence")
	}
}

func TestWorkerCancelsItsBodyWhenCoordinationConnectionIsLost(t *testing.T) {
	db := testdb.Open(t)
	r := NewRegistry(testLogger()).WithDatabase(db)
	started := make(chan struct{})
	stopped := make(chan struct{})
	_ = r.Register(Task{ID: "heartbeat-loss", Interval: time.Hour, Run: func(ctx context.Context, _ string) (string, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return "", ctx.Err()
	}})
	if e := r.Trigger("heartbeat-loss"); e != nil {
		t.Fatal(e)
	}
	<-started
	if _, e := db.Exec(`select pg_terminate_backend(backend_pid) from worker_task_runs where task_id='heartbeat-loss' and state='running'`); e != nil {
		t.Fatal(e)
	}
	select {
	case <-stopped:
	case <-time.After(12 * time.Second):
		t.Fatal("worker kept running after losing coordination")
	}
	waitFor(t, 2*time.Second, func() bool { return !r.Tasks()[0].Running })
}

func TestManualRunReschedulesTheTimerToItsPersistedDueTime(t *testing.T) {
	db := testdb.Open(t)
	r := NewRegistry(testLogger()).WithDatabase(db)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() { cancel(); wg.Wait() }()
	triggered := make(chan string, 4)
	_ = r.Register(Task{ID: "reschedule", Interval: 3 * time.Second, StartupDelay: time.Hour, Run: func(_ context.Context, trigger string) (string, error) { triggered <- trigger; return "done", nil }})
	r.Start(ctx, &wg)
	time.Sleep(time.Second)
	if e := r.Trigger("reschedule"); e != nil {
		t.Fatal(e)
	}
	if trigger := <-triggered; trigger != "manual" {
		t.Fatal(trigger)
	}
	// The old anchored ticker skipped its next tick and waited five seconds
	// after this manual run, even though the shared due time was three seconds.
	select {
	case trigger := <-triggered:
		if trigger == "manual" {
			t.Fatal(trigger)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("scheduler skipped the persisted due time after a manual run")
	}
}
