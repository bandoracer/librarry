package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestRegisterValidation(t *testing.T) {
	registry := NewRegistry(testLogger())
	run := func(context.Context, string) (string, error) { return "", nil }
	if err := registry.Register(Task{Name: "x", Interval: time.Minute, Run: run}); err == nil {
		t.Fatal("expected missing id error")
	}
	if err := registry.Register(Task{ID: "x", Run: run}); err == nil {
		t.Fatal("expected missing interval error")
	}
	if err := registry.Register(Task{ID: "x", Interval: time.Minute}); err == nil {
		t.Fatal("expected missing run error")
	}
	if err := registry.Register(Task{ID: "x", Interval: time.Minute, Run: run}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := registry.Register(Task{ID: "x", Interval: time.Minute, Run: run}); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestStartRunsStartupPass(t *testing.T) {
	registry := NewRegistry(testLogger())
	done := make(chan string, 1)
	err := registry.Register(Task{
		ID:           "startup-task",
		Name:         "Startup Task",
		Interval:     time.Hour,
		StartupDelay: time.Millisecond,
		Run: func(_ context.Context, trigger string) (string, error) {
			done <- trigger
			return "1 item processed", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	registry.Start(ctx, &wg)
	select {
	case trigger := <-done:
		if trigger != "scheduled-startup" {
			t.Fatalf("expected scheduled-startup trigger, got %q", trigger)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("startup run never fired")
	}
	waitFor(t, 2*time.Second, func() bool {
		status := registry.Tasks()[0]
		return !status.Running && status.LastRunAt != nil
	})
	status := registry.Tasks()[0]
	if status.LastOutcome != "1 item processed" {
		t.Fatalf("unexpected outcome: %+v", status)
	}
	if status.LastError != "" {
		t.Fatalf("unexpected error recorded: %+v", status)
	}
	if status.NextRunAt == nil {
		t.Fatal("expected a next run time")
	}
	if status.Interval != "1h" {
		t.Fatalf("unexpected interval format: %q", status.Interval)
	}
	cancel()
	wg.Wait()
}

func TestTriggerRunsManuallyAndReportsBusy(t *testing.T) {
	registry := NewRegistry(testLogger())
	release := make(chan struct{})
	started := make(chan struct{})
	err := registry.Register(Task{
		ID:       "manual-task",
		Interval: time.Hour,
		Run: func(_ context.Context, trigger string) (string, error) {
			close(started)
			<-release
			return "manual pass", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := registry.Trigger("missing"); !errors.Is(err, ErrTaskUnknown) {
		t.Fatalf("expected ErrTaskUnknown, got %v", err)
	}
	if err := registry.Trigger("manual-task"); err != nil {
		t.Fatalf("unexpected trigger error: %v", err)
	}
	<-started
	if !registry.Tasks()[0].Running {
		t.Fatal("expected task to report running")
	}
	if err := registry.Trigger("manual-task"); !errors.Is(err, ErrTaskBusy) {
		t.Fatalf("expected ErrTaskBusy while running, got %v", err)
	}
	close(release)
	waitFor(t, 2*time.Second, func() bool {
		status := registry.Tasks()[0]
		return !status.Running && status.LastOutcome == "manual pass"
	})
}

func TestRunRecordsErrors(t *testing.T) {
	registry := NewRegistry(testLogger())
	err := registry.Register(Task{
		ID:       "erroring-task",
		Interval: time.Hour,
		Run: func(context.Context, string) (string, error) {
			return "", errors.New("indexer offline")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Trigger("erroring-task"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 2*time.Second, func() bool {
		status := registry.Tasks()[0]
		return status.LastError == "indexer offline" && status.LastOutcome == "failed"
	})
}

func TestFormatInterval(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Minute:           "30m",
		6 * time.Hour:              "6h",
		12 * time.Hour:             "12h",
		time.Minute:                "1m",
		90 * time.Second:           "1m30s",
		time.Hour + 30*time.Minute: "1h30m",
		5 * time.Minute:            "5m",
		24 * time.Hour:             "24h",
		45 * time.Second:           "45s",
		0:                          "0s",
	}
	for input, expected := range cases {
		if got := FormatInterval(input); got != expected {
			t.Fatalf("FormatInterval(%s): expected %q, got %q", input, expected, got)
		}
	}
}
