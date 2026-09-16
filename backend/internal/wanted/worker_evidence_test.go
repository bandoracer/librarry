package wanted

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

type workerFixture struct {
	Acquisition
	searches    []string
	searchError error
	afterSearch func()
	feed        []acquisition.Release
	evidence    acquisition.DownloadEvidence
	grabs       int
}

func (f *workerFixture) Search(_ context.Context, q acquisition.ReleaseSearchQuery) ([]acquisition.Release, error) {
	f.searches = append(f.searches, q.Query)
	if f.afterSearch != nil {
		f.afterSearch()
	}
	return nil, f.searchError
}
func (f *workerFixture) Feed(context.Context, acquisition.ReleaseFeedQuery) ([]acquisition.Release, error) {
	return f.feed, nil
}
func (f *workerFixture) LiveDownloadEvidence(context.Context, acquisition.DownloadListQuery) acquisition.DownloadEvidence {
	return f.evidence
}
func (f *workerFixture) Grab(context.Context, acquisition.DownloadRequest) (acquisition.DownloadStatus, error) {
	f.grabs++
	return acquisition.DownloadStatus{ID: "fixture"}, nil
}
func (f *workerFixture) CategoryForFormat(string) string { return "books" }
func (f *workerFixture) TorrentRoot() string             { return "/fixture/downloads" }

type failingAuthorFixture struct{ calls []string }

func (f *failingAuthorFixture) AuthorBibliography(_ context.Context, q metadata.Query) ([]metadata.SearchResult, error) {
	f.calls = append(f.calls, q.ProviderKey)
	return nil, errors.New("controlled provider failure")
}

func TestWorkerFairnessTraversesTenThousandTiedBooks(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	if _, err := db.Exec(`insert into wanted_items(wanted_format,title,status,monitored) select 'ebook','Book '||n,case when n%2=0 then 'imported' else 'wanted' end,true from generate_series(1,10001)n`); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"monitor", "upgrade"} {
		seen := map[string]bool{}
		for len(seen) < 10001 {
			var items []WantedItem
			var err error
			if kind == "monitor" {
				items, err = store.ListDueWanted(ctx, 200, time.Hour, false)
			} else {
				items, err = store.ListUpgradeWanted(ctx, nil, 200, time.Hour, false)
			}
			if err != nil || len(items) == 0 {
				t.Fatal(kind, len(seen), err)
			}
			for _, item := range items {
				if seen[item.ID] {
					t.Fatal("starved behind repeated book", kind, item.ID)
				}
				seen[item.ID] = true
				if err := store.markWorkerChecked(ctx, item.ID, kind); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	var searched int
	if err := db.QueryRow(`select count(*) from wanted_items where last_search_at is not null or last_upgrade_search_at is not null`).Scan(&searched); err != nil || searched != 0 {
		t.Fatal("checks manufactured search success", searched, err)
	}
}

func TestMonitorSkipsUnknownAndPresentButRecoversImportedMissing(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	present := evidenceBook(t, db, "Present", "ebook")
	unknown := evidenceBook(t, db, "Unknown", "ebook")
	missing := evidenceBook(t, db, "Missing imported", "ebook")
	evidenceFile(t, db, present, "/library/present.epub", "present")
	evidenceFile(t, db, unknown, "/library/unknown.epub", "unknown")
	evidenceFile(t, db, missing, "/library/missing.epub", "missing")
	if _, err := db.Exec(`update wanted_items set status='imported',created_at=case when id=$1 then '2020-01-01'::timestamptz when id=$2 then '2020-01-02'::timestamptz else '2020-01-03'::timestamptz end`, present.ID, unknown.ID); err != nil {
		t.Fatal(err)
	}
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh"}}
	service := NewService(store, fixture)
	first, err := service.Monitor(ctx, MonitorRequest{Limit: 2})
	if err != nil || first.WantedChecked != 2 || len(fixture.searches) != 0 || len(first.Items) != 2 || first.Items[0].SkippedReason == "" {
		t.Fatal(first, err, fixture.searches)
	}
	second, err := service.Monitor(ctx, MonitorRequest{Limit: 2})
	if err != nil || second.WantedChecked != 1 || len(fixture.searches) != 1 || !strings.Contains(fixture.searches[0], missing.Title) {
		t.Fatal(second, err, fixture.searches)
	}
	third, err := service.Monitor(ctx, MonitorRequest{Limit: 2})
	if err != nil || third.WantedChecked != 0 {
		t.Fatal(third, err)
	}
}

func TestFailedSearchAndClientOutageDoNotPinMonitorBatch(t *testing.T) {
	for _, status := range []string{"fresh", "partial", "unavailable"} {
		t.Run(status, func(t *testing.T) {
			db := testdb.Open(t)
			ctx := context.Background()
			store := NewStore(db)
			for i := 0; i < 5; i++ {
				evidenceBook(t, db, fmt.Sprint("Book ", i), "ebook")
			}
			fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: status}, searchError: errors.New("controlled search failure")}
			service := NewService(store, fixture)
			seen := map[string]bool{}
			for i := 0; i < 3; i++ {
				run, err := service.Monitor(ctx, MonitorRequest{Limit: 2, AutoGrab: true})
				if err != nil {
					t.Fatal(err)
				}
				for _, result := range run.Items {
					if seen[result.WantedItem.ID] {
						t.Fatal("repeated first page")
					}
					seen[result.WantedItem.ID] = true
					if status != "fresh" && result.SkippedReason == "" {
						t.Fatal(result)
					}
				}
			}
			if len(seen) != 5 || fixture.grabs != 0 {
				t.Fatal(seen, fixture.grabs)
			}
			if status != "fresh" && len(fixture.searches) != 0 {
				t.Fatal(fixture.searches)
			}
			var successful int
			if err := db.QueryRow(`select count(*) from wanted_items where last_search_at is not null`).Scan(&successful); err != nil || successful != 0 {
				t.Fatal(successful, err)
			}
		})
	}
}

func TestAuthorFailuresAdvanceAttemptsWithoutSuccess(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	if _, err := db.Exec(`insert into author_subscriptions(provider,provider_key,author_name,wanted_format,status,monitor_new_items,missing_book_policy) select 'fixture','author:'||n,'Same name','ebook','monitored',true,'all' from generate_series(1,205)n`); err != nil {
		t.Fatal(err)
	}
	fixture := &failingAuthorFixture{}
	service := NewService(store, nil, fixture)
	for i := 0; i < 2; i++ {
		run, err := service.MonitorAuthors(ctx, AuthorMonitorRequest{Limit: 200})
		if err != nil || run.ErrorCount != run.AuthorsChecked {
			t.Fatal(run, err)
		}
	}
	seen := map[string]bool{}
	for _, id := range fixture.calls {
		if seen[id] {
			t.Fatal("repeated author", id)
		}
		seen[id] = true
	}
	if len(seen) != 205 {
		t.Fatal(len(seen))
	}
	var synced int
	if err := db.QueryRow(`select count(*) from author_subscriptions where last_sync_at is not null`).Scan(&synced); err != nil || synced != 0 {
		t.Fatal(synced, err)
	}
}

func TestAutomaticGrabRechecksOwnerAndFileEvidence(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	book := evidenceBook(t, db, "Stopped book", "ebook")
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh"}}
	service := NewService(store, fixture)
	release := ReleaseDecision{ID: "fixture", Score: 99999}
	if _, err := db.Exec(`update wanted_items set monitored=false where id=$1`, book.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.grabRelease(ctx, book, release, false, "", "monitor", false); err == nil {
		t.Fatal("unmonitored book was grabbed")
	}
	if _, err := db.Exec(`update wanted_items set monitored=true where id=$1`, book.ID); err != nil {
		t.Fatal(err)
	}
	evidenceFile(t, db, book, "/library/stopped.epub", "present")
	if _, err := service.grabRelease(ctx, book, release, false, "", "monitor", false); err == nil {
		t.Fatal("newly present book was grabbed")
	}
	if fixture.grabs != 0 {
		t.Fatal(fixture.grabs)
	}
	fixture.afterSearch = func() {
		if _, err := store.UpdateWanted(ctx, book.ID, WantedUpdateRequest{QualityProfile: "changed"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.searchReleasesForItem(ctx, book, SearchReleasesRequest{}); err == nil || !strings.Contains(err.Error(), "settings changed") {
		t.Fatal(err)
	}
	var staleSuccess bool
	if err := db.QueryRow(`select last_search_at is not null from wanted_items where id=$1`, book.ID).Scan(&staleSuccess); err != nil || staleSuccess {
		t.Fatal("stale provider result advanced search success", staleSuccess, err)
	}
}

func TestFeedTraversesOlderBooksAndSkipsUnmonitoredAndUnverified(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	if _, err := db.Exec(`insert into wanted_items(wanted_format,title,author_name,status,monitored) select 'ebook','Unrelated '||n,'Fixture','wanted',true from generate_series(1,10001)n`); err != nil {
		t.Fatal(err)
	}
	wanted := evidenceBook(t, db, "Target exact book", "ebook")
	off := evidenceBook(t, db, "Target exact book off", "ebook")
	unknown := evidenceBook(t, db, "Target exact book unknown", "ebook")
	evidenceFile(t, db, unknown, "/library/feed-unknown.epub", "unknown")
	if _, err := db.Exec(`update wanted_items set monitored=false where id=$1`, off.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update wanted_items set created_at='2000-01-01',status='imported' where id=$1`, wanted.ID); err != nil {
		t.Fatal(err)
	}
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh"}, feed: []acquisition.Release{{ID: "fixture-feed", Title: "Target exact book unknown off EPUB", Indexer: "fixture", Protocol: "torrent", DownloadURL: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567", Seeders: 5}}}
	run, err := NewService(store, fixture).FeedSync(ctx, FeedSyncRequest{})
	if err != nil || run.MatchedCount != 1 {
		t.Fatal(run, err)
	}
	found, blocked := false, false
	for _, match := range run.Matches {
		if match.WantedItem.ID == off.ID {
			t.Fatal("unmonitored feed match")
		}
		if match.WantedItem.ID == wanted.ID {
			found = true
		}
		if match.WantedItem.ID == unknown.ID && match.SkippedReason != "" {
			blocked = true
		}
	}
	if !found || !blocked {
		t.Fatal(run)
	}
	var notSearched bool
	if err := db.QueryRow(`select last_search_at is null from wanted_items where id=$1`, wanted.ID).Scan(&notSearched); err != nil || !notSearched {
		t.Fatal("feed observation delayed a full indexer search", notSearched, err)
	}
}

func TestFeedMatchResponseCapKeepsEvaluationCounters(t *testing.T) {
	run := FeedSyncRun{MatchedCount: 10001}
	for i := 0; i < 10001; i++ {
		appendFeedMatch(&run, FeedSyncMatch{})
	}
	if len(run.Matches) != 1000 || !run.MatchesTruncated || run.MatchedCount != 10001 {
		t.Fatal(len(run.Matches), run.MatchesTruncated, run.MatchedCount)
	}
}

func TestUpgradeSkipsMissingCopiesAndAdvancesPastIneligibleBatch(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	if _, err := db.Exec(`insert into wanted_items(wanted_format,title,status,monitored,created_at) select 'ebook','Old missing '||n,'imported',true,'2000-01-01' from generate_series(1,205)n`); err != nil {
		t.Fatal(err)
	}
	book := evidenceBook(t, db, "Present upgrade candidate", "ebook")
	evidenceFile(t, db, book, "/library/upgrade.epub", "present")
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh"}}
	service := NewService(store, fixture)
	first, err := service.SearchUpgrades(ctx, UpgradeRequest{Limit: 200})
	if err != nil || first.WantedChecked != 200 || len(fixture.searches) != 0 {
		t.Fatal(first, err, fixture.searches)
	}
	second, err := service.SearchUpgrades(ctx, UpgradeRequest{Limit: 200})
	if err != nil || second.WantedChecked != 6 || len(fixture.searches) != 1 {
		t.Fatal(second, err, fixture.searches)
	}
	var searched int
	if err := db.QueryRow(`select count(*) from wanted_items where last_upgrade_search_at is not null`).Scan(&searched); err != nil || searched != 1 {
		t.Fatal(searched, err)
	}
}

func TestMonitorRecoversPartialAudiobookInsteadOfUpgradingIt(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	book := evidenceBook(t, db, "Incomplete audio", "audiobook")
	a := evidenceFile(t, db, book, "/library/partial1.mp3", "present")
	b := evidenceFile(t, db, book, "/library/partial2.mp3", "missing")
	evidenceManifest(t, db, book, a, b)
	if _, err := db.Exec(`update wanted_items set status='imported' where id=$1`, book.ID); err != nil {
		t.Fatal(err)
	}
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh", Downloads: []acquisition.DownloadStatus{{State: "pausedUP", ImportStatus: "imported", Tags: []string{"wanted:" + book.ID}}}}}
	service := NewService(store, fixture)
	upgrades, err := service.SearchUpgrades(ctx, UpgradeRequest{})
	if err != nil || len(upgrades.Items) != 1 || upgrades.Items[0].SkippedReason == "" || len(fixture.searches) != 0 {
		t.Fatal(upgrades, err, fixture.searches)
	}
	monitor, err := service.Monitor(ctx, MonitorRequest{})
	if err != nil || monitor.ErrorCount != 0 || len(fixture.searches) != 1 {
		t.Fatal(monitor, err, fixture.searches)
	}
}
