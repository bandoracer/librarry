package acquisition

import (
	"context"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestDownloadMutationIsolatesClientsSharingExternalID(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewSQLDownloadStore(db)
	if err := store.UpsertDownloads(ctx, []DownloadStatus{{Client: "qBittorrent", ID: "same", State: "pausedUP"}, {Client: "Transmission", ID: "same", State: "stopped"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDownloadFailed(ctx, "same", "ambiguous"); err == nil {
		t.Fatal("ambiguous external ID mutated")
	}
	if err := store.MarkDownloadFailed(WithDownloadClient(ctx, "Transmission"), "same", "fixture failure"); err != nil {
		t.Fatal(err)
	}
	rows, err := store.ListDownloads(ctx, DownloadListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Client == "qBittorrent" && row.FailureReason != "" {
			t.Fatal("unrelated client changed")
		}
		if row.Client == "Transmission" && row.FailureReason != "fixture failure" {
			t.Fatal("scoped update missing")
		}
	}
	if err := store.MarkDownloadsDeleted(WithDownloadClient(ctx, "qBittorrent"), []string{"same"}); err != nil {
		t.Fatal(err)
	}
	rows, err = store.ListDownloads(ctx, DownloadListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Client != "Transmission" {
		t.Fatalf("wrong client removed: %+v", rows)
	}
}
