package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestWorkerDiagnosticsRetainFailuresBeyondRoutineHistory(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	r := NewRegistry(testLogger()).WithDatabase(db)
	task := Task{ID: "diagnostics", Interval: time.Hour, Run: func(context.Context, string) (string, error) { return "ok", nil }}
	if err := r.Register(task); err != nil {
		t.Fatal(err)
	}
	claim, err := r.claim(ctx, task, "manual")
	if err != nil {
		t.Fatal(err)
	}
	failedID := claim.runID
	if err = claim.finish("provider outage", errors.New("fixture provider unavailable")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into worker_task_runs(task_id,trigger,backend_pid,state,started_at,finished_at) select 'diagnostics','fixture',0,'completed',now()+make_interval(secs=>n),now()+make_interval(secs=>n+1) from generate_series(1,200)n`); err != nil {
		t.Fatal(err)
	}
	claim, err = r.claim(ctx, task, "manual")
	if err != nil {
		t.Fatal(err)
	}
	successID := claim.runID
	if err = claim.finish("20 checked", nil, RunDetails{Counts: map[string]int{"checked": 20}, OperationIDs: []string{"00000000-0000-0000-0000-000000000123"}}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`select count(*) from worker_task_runs where task_id='diagnostics' and state='completed'`).Scan(&count); err != nil || count != 100 {
		t.Fatal(count, err)
	}
	page, err := r.RunHistoryPage(ctx, task.ID, "unreviewed", 25, 0)
	if err != nil || page.Total != 1 || page.Runs[0].ID != failedID {
		t.Fatal(page, err)
	}
	statuses, err := r.TasksContext(ctx)
	if err != nil || statuses[0].LastSuccessAt == nil || statuses[0].LastSuccessRunID != successID || statuses[0].UnreviewedFailures != 1 || statuses[0].Details.Counts["checked"] != 20 || statuses[0].DurationMS == nil {
		t.Fatal(statuses, err)
	}
	// The successful run has a deliberately older timestamp than the fixtures;
	// its identity must survive pruning and a later degraded pass.
	claim, err = r.claim(ctx, task, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if err = claim.finish("partially checked", nil, RunDetails{Counts: map[string]int{"checked": 5, "errors": 2}, Errors: 2, NextAction: "Review provider health."}); err != nil {
		t.Fatal(err)
	}
	statuses, err = r.TasksContext(ctx)
	if err != nil || statuses[0].RunState != "degraded" || statuses[0].LastSuccessRunID != successID || statuses[0].UnreviewedFailures != 2 {
		t.Fatal(statuses, err)
	}
	page, err = r.RunHistoryPage(ctx, task.ID, "all", 25, 100)
	if err != nil || page.Total != 102 || len(page.Runs) != 2 {
		t.Fatal(page, err)
	}
	// Rebuild the registry to prove these values do not depend on local state.
	restarted := NewRegistry(testLogger()).WithDatabase(db)
	_ = restarted.Register(task)
	statuses, err = restarted.TasksContext(ctx)
	if err != nil || statuses[0].LastSuccessRunID != successID || statuses[0].Details.Errors != 2 {
		t.Fatal(statuses, err)
	}
}

func TestRunReviewProtectsActiveWorkAndExpiresOnlyReviewedFailures(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	r := NewRegistry(testLogger()).WithDatabase(db)
	task := Task{ID: "review", Interval: time.Hour, Run: func(context.Context, string) (string, error) { return "ok", nil }}
	_ = r.Register(task)
	claim, err := r.claim(ctx, task, "manual")
	if err != nil {
		t.Fatal(err)
	}
	id := claim.runID
	if err = r.ReviewRun(ctx, task.ID, id, RunReview{Reviewed: true, ExpectedState: "interrupted"}); !errors.Is(err, ErrRunConflict) {
		t.Fatal("active work reviewed", err)
	}
	if err = claim.finish("failure", errors.New("fixture")); err != nil {
		t.Fatal(err)
	}
	request := RunReview{Reviewed: true, ExpectedState: "failed"}
	if err = r.ReviewRun(ctx, task.ID, id, request); err != nil {
		t.Fatal(err)
	}
	if err = r.ReviewRun(ctx, task.ID, id, request); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale review accepted", err)
	}
	page, err := r.RunHistoryPage(ctx, task.ID, "unreviewed", 25, 0)
	if err != nil || page.Total != 0 {
		t.Fatal(page, err)
	}
	all, err := r.RunHistoryPage(ctx, task.ID, "all", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.ReviewRun(ctx, task.ID, id, RunReview{Reviewed: false, ExpectedState: "failed", ExpectedReviewedAt: all.Runs[0].ReviewedAt}); err != nil {
		t.Fatal(err)
	}
	page, err = r.RunHistoryPage(ctx, task.ID, "unreviewed", 25, 0)
	if err != nil || page.Total != 1 {
		t.Fatal(page, err)
	}
	if _, err = db.Exec(`update worker_task_runs set reviewed_at=now()-interval '91 days',started_at=now()-interval '100 days' where id=$1`, id); err != nil {
		t.Fatal(err)
	}
	var unreviewed string
	if err = db.QueryRow(`insert into worker_task_runs(task_id,trigger,backend_pid,state,started_at) values('review','fixture',0,'failed',now()-interval '100 days') returning id::text`).Scan(&unreviewed); err != nil {
		t.Fatal(err)
	}
	claim, err = r.claim(ctx, task, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if err = claim.finish("ok", nil); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`select count(*) from worker_task_runs where id=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatal("old reviewed failure retained", count, err)
	}
	if err = db.QueryRow(`select count(*) from worker_task_runs where id=$1`, unreviewed).Scan(&count); err != nil || count != 1 {
		t.Fatal("unreviewed evidence deleted", count, err)
	}
}

func TestInterruptedRunReviewAndDetailsThroughRegistry(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	r := NewRegistry(testLogger()).WithDatabase(db)
	task := Task{ID: "details", Interval: time.Hour, Run: func(ctx context.Context, _ string) (string, error) {
		RecordRunDetails(ctx, RunDetails{Counts: map[string]int{"checked": 4, "invalid count": 7, "negative": -1}, OperationIDs: []string{"https://private.invalid/secret", "00000000-0000-0000-0000-000000000123"}, Errors: 1, NextAction: "Review recovery."})
		return "partial", nil
	}}
	_ = r.Register(task)
	if err := r.Trigger(task.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 2*time.Second, func() bool { return !r.Tasks()[0].Running })
	page, err := r.RunHistoryPage(ctx, task.ID, "unreviewed", 25, 0)
	if err != nil || page.Total != 1 || page.Runs[0].State != "degraded" || len(page.Runs[0].Details.Counts) != 1 || len(page.Runs[0].Details.OperationIDs) != 1 {
		t.Fatal(page, err)
	}
	claim, err := r.claim(ctx, task, "manual")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseWorkerConnection(claim.conn, task.ID)
	var killed bool
	if err = db.QueryRow(`select pg_terminate_backend(backend_pid) from worker_task_runs where id=$1`, claim.runID).Scan(&killed); err != nil || !killed {
		t.Fatal(err)
	}
	if err = r.ReviewRun(ctx, task.ID, claim.runID, RunReview{Reviewed: true, ExpectedState: "interrupted"}); err != nil {
		t.Fatal(err)
	}
	page, err = r.RunHistoryPage(ctx, task.ID, "all", 25, 0)
	if err != nil || page.Runs[0].ReviewedAt == nil || page.Runs[0].FinishedAt != nil || page.Runs[0].DurationMS != nil {
		t.Fatal(page, err)
	}
}

func TestHistoricalRunCannotBorrowCurrentOwnersSessionLock(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	registry := NewRegistry(testLogger()).WithDatabase(db)
	task := Task{ID: "reused-pid", Interval: time.Hour, Run: func(context.Context, string) (string, error) { return "ok", nil }}
	if err := registry.Register(task); err != nil {
		t.Fatal(err)
	}
	claim, err := registry.claim(ctx, task, "manual")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseWorkerConnection(claim.conn, task.ID)
	var historicID string
	if err = db.QueryRow(`insert into worker_task_runs(task_id,trigger,backend_pid,state,started_at) select task_id,'fixture',backend_pid,'running',now()-interval '1 day' from worker_task_runs where id=$1 returning id::text`, claim.runID).Scan(&historicID); err != nil {
		t.Fatal(err)
	}
	page, err := registry.RunHistoryPage(ctx, task.ID, "unreviewed", 100, 0)
	if err != nil || page.Total != 1 || page.Runs[0].ID != historicID || page.Runs[0].State != "interrupted" {
		t.Fatal(page, err)
	}
	statuses, err := registry.TasksContext(ctx)
	if err != nil || !statuses[0].Running || statuses[0].UnreviewedFailures != 1 {
		t.Fatal(statuses, err)
	}
	if err = registry.ReviewRun(ctx, task.ID, historicID, RunReview{Reviewed: true, ExpectedState: "interrupted"}); err != nil {
		t.Fatal(err)
	}
	// Reviewing the stale identity must not modify or release the current owner.
	if _, err = registry.claim(ctx, task, "manual"); !errors.Is(err, ErrTaskBusy) {
		t.Fatal(err)
	}
	if err = claim.finish("current success", nil); err != nil {
		t.Fatal(err)
	}
	statuses, err = registry.TasksContext(ctx)
	if err != nil || statuses[0].LastSuccessRunID != claim.runID || statuses[0].UnreviewedFailures != 0 {
		t.Fatal(statuses, err)
	}
}
