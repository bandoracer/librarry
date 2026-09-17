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

func TestBlockedTasksRemainVisibleWithoutStartingOrClaiming(t *testing.T) {
	for _, reason := range []string{"disabled", "unavailable"} {
		t.Run(reason, func(t *testing.T) {
			r := NewRegistry(testLogger())
			var calls atomic.Int32
			task := Task{ID: reason, Interval: time.Millisecond, StartupDelay: time.Millisecond, Run: func(context.Context, string) (string, error) { calls.Add(1); return "unexpected", nil }}
			expected := ErrTaskDisabled
			if reason == "disabled" {
				task.DisabledReason = "Disabled by fixture configuration."
			} else {
				task.UnavailableReason = "Database persistence is required."
				expected = ErrTaskUnavailable
			}
			if err := r.Register(task); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wg sync.WaitGroup
			r.Start(ctx, &wg)
			if err := r.Trigger(task.ID); !errors.Is(err, expected) {
				t.Fatal(err)
			}
			if _, err := r.claim(ctx, task, "manual"); !errors.Is(err, expected) {
				t.Fatal(err)
			}
			time.Sleep(15 * time.Millisecond)
			cancel()
			wg.Wait()
			status := r.Tasks()[0]
			if calls.Load() != 0 || status.Running || status.NextRunAt != nil || status.LastRunAt != nil || status.LastFinishedAt != nil {
				t.Fatal(status, calls.Load())
			}
			if status.Enabled != (reason != "disabled") || status.Available != (reason != "unavailable") {
				t.Fatal(status)
			}
		})
	}
}

func TestDisabledInstanceRetainsHistoryAndObservesEnabledPeer(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	active := NewRegistry(testLogger()).WithDatabase(db)
	task := Task{ID: "fixture", Interval: time.Hour, Run: func(context.Context, string) (string, error) { return "ok", nil }}
	if err := active.Register(task); err != nil {
		t.Fatal(err)
	}
	claim, err := active.claim(ctx, task, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if err = claim.finish("old failure", errors.New("fixture failure")); err != nil {
		t.Fatal(err)
	}
	inactive := NewRegistry(testLogger()).WithDatabase(db)
	disabled := task
	disabled.DisabledReason = "Disabled on this instance."
	disabled.Run = nil
	if err = inactive.Register(disabled); err != nil {
		t.Fatal(err)
	}
	states, err := inactive.TasksContext(ctx)
	if err != nil || states[0].Enabled || !states[0].Available || states[0].NextRunAt != nil || states[0].LastFinishedAt == nil || states[0].RunState != "failed" || states[0].UnreviewedFailures != 1 {
		t.Fatal(states, err)
	}
	runs, err := inactive.RunHistory(ctx, task.ID)
	if err != nil || len(runs) != 1 {
		t.Fatal(runs, err)
	}
	if err = inactive.ReviewRun(ctx, task.ID, runs[0].ID, RunReview{Reviewed: true, ExpectedState: "failed"}); err != nil {
		t.Fatal(err)
	}
	claim, err = active.claim(ctx, task, "manual")
	if err != nil {
		t.Fatal(err)
	}
	states, err = inactive.TasksContext(ctx)
	if err != nil || states[0].Enabled || !states[0].Running || states[0].NextRunAt != nil || states[0].LastFinishedAt != nil {
		t.Fatal(states, err)
	}
	if err = inactive.Trigger(task.ID); !errors.Is(err, ErrTaskDisabled) {
		t.Fatal(err)
	}
	if err = claim.finish("peer success", nil); err != nil {
		t.Fatal(err)
	}
	states, err = inactive.TasksContext(ctx)
	if err != nil || states[0].Running || states[0].LastSuccessRunID != claim.runID || states[0].LastFinishedAt == nil {
		t.Fatal(states, err)
	}
	// Enabling a replacement process preserves due time and saved history.
	restarted := NewRegistry(testLogger()).WithDatabase(db)
	_ = restarted.Register(task)
	states, err = restarted.TasksContext(ctx)
	if err != nil || !states[0].Enabled || states[0].NextRunAt == nil || states[0].LastSuccessRunID != claim.runID {
		t.Fatal(states, err)
	}
	if _, err = restarted.claim(ctx, task, "scheduled-startup"); !errors.Is(err, errNotDue) {
		t.Fatal("restart repeated completed work", err)
	}
}
