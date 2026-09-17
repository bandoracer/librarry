package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/importlists"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type listTaskFetcher struct{ fail bool }

func (f *listTaskFetcher) FetchList(context.Context, map[string]string, int) ([]importlists.Entry, error) {
	if f.fail {
		return nil, errors.New("fixture pagination failure")
	}
	return []importlists.Entry{}, nil
}
func TestImportListTaskReportsIncompleteSyncAsFailure(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := importlists.NewStore(db)
	if _, err := store.CreateList(ctx, importlists.List{Name: "Fixture", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	fetcher := &listTaskFetcher{fail: true}
	service := importlists.NewService(store, wanted.NewService(wanted.NewStore(db), nil, nil), fetcher, nil)
	task := importListSyncTask(slog.New(slog.NewTextHandler(io.Discard, nil)), service, config.Config{})
	message, err := task.Run(ctx, "test")
	if err == nil || !strings.Contains(message, "1 errors") {
		t.Fatal(message, err)
	}
	fetcher.fail = false
	message, err = task.Run(ctx, "test")
	if err != nil || !strings.Contains(message, "0 errors") {
		t.Fatal(message, err)
	}
}
