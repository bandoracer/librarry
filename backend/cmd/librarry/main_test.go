package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
)

func TestCalibreConversionRefreshScheduleDefaultsAndBounds(t *testing.T) {
	interval, request := calibreConversionRefreshSchedule(config.Config{})
	if interval != 15*time.Minute {
		t.Fatalf("expected default interval, got %s", interval)
	}
	if request.Limit != 200 || request.MaxAttempts != 1 || request.Force {
		t.Fatalf("unexpected default request: %+v", request)
	}

	interval, request = calibreConversionRefreshSchedule(config.Config{
		CalibreRefreshInterval:    2 * time.Hour,
		CalibreRefreshLimit:       1000,
		CalibreRefreshMaxAttempts: 99,
	})
	if interval != 2*time.Hour {
		t.Fatalf("expected configured interval, got %s", interval)
	}
	if request.Limit != 500 || request.MaxAttempts != 10 {
		t.Fatalf("expected bounded request, got %+v", request)
	}
}

func TestRunCalibreConversionRefreshOnceUsesConfiguredRequest(t *testing.T) {
	service := &fakeCalibreConversionRefreshService{
		outcome: library.CalibreConversionRefreshOutcome{Checked: 3, Refreshed: 2, Skipped: 1},
	}
	outcome, err := runCalibreConversionRefreshOnce(
		context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		service,
		config.Config{
			CalibreRefreshLimit:       17,
			CalibreRefreshMaxAttempts: 4,
		},
		"scheduled",
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Checked != 3 || outcome.Refreshed != 2 || outcome.Skipped != 1 {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if len(service.requests) != 1 {
		t.Fatalf("expected one refresh request, got %d", len(service.requests))
	}
	request := service.requests[0]
	if request.Limit != 17 || request.MaxAttempts != 4 || request.Force {
		t.Fatalf("unexpected refresh request: %+v", request)
	}
}

type fakeCalibreConversionRefreshService struct {
	requests []library.CalibreConversionRefreshRequest
	outcome  library.CalibreConversionRefreshOutcome
}

func (f *fakeCalibreConversionRefreshService) RefreshCalibreConversions(_ context.Context, request library.CalibreConversionRefreshRequest) (library.CalibreConversionRefreshOutcome, error) {
	f.requests = append(f.requests, request)
	return f.outcome, nil
}
