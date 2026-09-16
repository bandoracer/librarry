package wanted

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestUpgradeSelectionProcessesEveryBookAndOnlySelectedBooks(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	var ids []string
	for i := 0; i < 201; i++ {
		item := evidenceBook(t, db, fmt.Sprintf("Selected %03d", i), "ebook")
		evidenceFile(t, db, item, fmt.Sprintf("/library/selected-%03d.epub", i), "present")
		if i < 200 {
			ids = append(ids, item.ID)
		}
	}
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh"}}
	run, err := NewService(store, fixture).SearchUpgrades(ctx, UpgradeRequest{WantedIDs: ids, Limit: 50, Force: true})
	if err != nil || run.WantedChecked != 200 || len(run.Items) != 200 || len(fixture.searches) != 200 {
		t.Fatal("selection truncated", run.WantedChecked, len(run.Items), len(fixture.searches), err)
	}
	for i, result := range run.Items {
		if result.WantedItem.ID != ids[i] || result.Error != "" || result.SkippedReason != "" {
			t.Fatal("selection scope/order changed", i, result)
		}
	}
	var checked, searched int
	if err := db.QueryRow(`select count(*) filter(where last_upgrade_checked_at is not null),count(*) filter(where last_upgrade_search_at is not null) from wanted_items`).Scan(&checked, &searched); err != nil || checked != 200 || searched != 200 {
		t.Fatal("outside selection touched or selected book omitted", checked, searched, err)
	}
}

func TestUpgradeSelectionReportsIneligibleBooksWithoutTouchingClocks(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	var ids []string
	for _, state := range []string{"unmonitored", "removed", "ignored", "recent", "missing"} {
		book := evidenceBook(t, db, state, "ebook")
		ids = append(ids, book.ID)
	}
	if _, err := db.Exec(`update wanted_items set monitored=title<>'unmonitored',status=case when title in ('removed','ignored') then title else status end,last_upgrade_search_at=case when title='recent' then now() else null end`); err != nil {
		t.Fatal(err)
	}
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh"}}
	service := NewService(NewStore(db), fixture)
	run, err := service.SearchUpgrades(ctx, UpgradeRequest{WantedIDs: append(ids, ids[0]), Limit: 1})
	if err != nil || len(run.Items) != 5 || run.WantedChecked != 5 || len(fixture.searches) != 0 {
		t.Fatal(run, err)
	}
	for i, result := range run.Items {
		if result.WantedItem.ID != ids[i] || result.SkippedReason == "" {
			t.Fatal(result)
		}
	}
	var checked int
	if err := db.QueryRow(`select count(*) from wanted_items where last_upgrade_checked_at is not null`).Scan(&checked); err != nil || checked != 1 {
		t.Fatal(checked, err)
	}
	forced, err := service.SearchUpgrades(ctx, UpgradeRequest{WantedIDs: []string{ids[3]}, Force: true})
	if err != nil || forced.WantedChecked != 1 || strings.Contains(forced.Items[0].SkippedReason, "not due") {
		t.Fatal(forced, err)
	}
}

func TestInvalidUpgradeSelectionCannotStartOrBroadenRun(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	book := evidenceBook(t, db, "Must remain untouched", "ebook")
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh"}}
	service := NewService(NewStore(db), fixture)
	for _, ids := range [][]string{{" "}, {"invalid"}, {book.ID, "00000000-0000-0000-0000-000000000001"}, make([]string, 201)} {
		if _, err := service.SearchUpgrades(ctx, UpgradeRequest{WantedIDs: ids, Force: true}); !errors.Is(err, ErrInvalidUpgradeRequest) {
			t.Fatal("invalid selection accepted", err)
		}
	}
	var runs, checked int
	if err := db.QueryRow(`select (select count(*) from upgrade_runs),(select count(*) from wanted_items where last_upgrade_checked_at is not null)`).Scan(&runs, &checked); err != nil || runs != 0 || checked != 0 || len(fixture.searches) != 0 {
		t.Fatal("invalid selection caused work", runs, checked, err)
	}
}

func TestNormalizeUpgradeSelection(t *testing.T) {
	id := "abcdefab-1234-1234-1234-abcdefabcdef"
	normalized, err := NormalizeUpgradeRequest(UpgradeRequest{WantedIDs: []string{" " + strings.ToUpper(id) + " ", id}, Limit: 1})
	if err != nil || len(normalized.WantedIDs) != 1 || normalized.WantedIDs[0] != id || normalized.Limit != 1 {
		t.Fatal(normalized, err)
	}
	batch, err := NormalizeUpgradeRequest(UpgradeRequest{})
	if err != nil || len(batch.WantedIDs) != 0 || batch.Limit != 50 {
		t.Fatal(batch, err)
	}
	for _, limit := range []int{-1, 201} {
		if _, err := NormalizeUpgradeRequest(UpgradeRequest{Limit: limit}); !errors.Is(err, ErrInvalidUpgradeRequest) {
			t.Fatal(err)
		}
	}
}
