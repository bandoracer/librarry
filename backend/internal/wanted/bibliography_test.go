package wanted

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

type bibliographyFixture struct {
	rows []metadata.SearchResult
	err  error
}

func (p *bibliographyFixture) AuthorBibliography(context.Context, metadata.Query) ([]metadata.SearchResult, error) {
	return p.rows, p.err
}
func bibliographyCandidate(id int, date, role string) metadata.SearchResult {
	return metadata.SearchResult{Provider: "Hardcover", Kind: metadata.SearchTypeBook, Work: metadata.Work{ID: fmt.Sprintf("hardcover:%d", id), Title: fmt.Sprintf("Book %d", id), FirstPublishDate: date, Authors: []metadata.Author{{ID: "hardcover-author:7", Name: "Fixture", Role: role}}}, RawSourceKey: fmt.Sprint(id)}
}
func TestAuthorMonitorUsesWholeBibliographyAndRetriesFailuresWithoutSync(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: true, MissingBookPolicy: "all"})
	if err != nil {
		t.Fatal(err)
	}
	fixture := &bibliographyFixture{}
	for i := 1; i <= 205; i++ {
		fixture.rows = append(fixture.rows, bibliographyCandidate(i, "2020-01-01", "Author"))
	}
	service := NewService(store, nil, fixture)
	fixture.err = errors.New("page 2 failed")
	run, err := service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true, SearchLimit: 1})
	if err != nil || run.ErrorCount != 1 || run.WantedCreated != 0 {
		t.Fatal(run, err)
	}
	var count int
	var unsynced bool
	if err := db.QueryRow(`select count(*) from wanted_items`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`select last_sync_at is null from author_subscriptions where id=$1`, sub.ID).Scan(&unsynced); err != nil {
		t.Fatal(err)
	}
	if count != 0 || !unsynced {
		t.Fatal(count, unsynced)
	}
	fixture.err = nil
	run, err = service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true, SearchLimit: 1})
	if err != nil || run.ErrorCount != 0 || run.ItemsFound != 205 || run.WantedCreated != 205 {
		t.Fatal(run.ItemsFound, run.WantedCreated, run.ErrorCount, err)
	}
	if repeated, err := service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true}); err != nil || repeated.WantedCreated != 0 {
		t.Fatal(repeated.WantedCreated, err)
	}
	if err := db.QueryRow(`select count(*) from wanted_items`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 205 {
		t.Fatal("repeat sync duplicated bibliography", count)
	}
}
func TestAuthorLatestPolicyUsesEligibleWorksAndOriginalPublication(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	_, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: true, MissingBookPolicy: "latest"})
	if err != nil {
		t.Fatal(err)
	}
	older := bibliographyCandidate(1, "2000-01-01", "Author")
	older.Edition.PublishedDate = "2024-01-01"
	newest := bibliographyCandidate(2, "2020-01-01", "Author")
	illustration := bibliographyCandidate(3, "2025-01-01", "Illustrator")
	unknown := bibliographyCandidate(4, "2025-02-01", "unknown")
	foreign := bibliographyCandidate(5, "2025-03-01", "Author")
	foreign.Work.Authors[0].ID = "hardcover-author:8"
	fixture := &bibliographyFixture{rows: []metadata.SearchResult{older, newest, illustration, unknown, foreign}}
	run, err := NewService(store, nil, fixture).MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
	if err != nil || run.ErrorCount != 0 || run.WantedCreated != 1 || run.ItemsFound != 4 {
		t.Fatal(run, err)
	}
	var title string
	if err := db.QueryRow(`select title from wanted_items`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Book 2" {
		t.Fatal("filtered contribution or reprint displaced latest original", title)
	}
	var reviews int
	if err := db.QueryRow(`select count(*) from author_metadata_reviews where reason like 'Selected person%'`).Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if reviews != 2 {
		t.Fatal("unverified writing credits not reviewed", reviews)
	}
}

func TestAuthorMonitorPersistenceFailureBacksOffWithoutFalseSuccess(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: true, MissingBookPolicy: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create function reject_author_fixture() returns trigger language plpgsql as $$ begin if new.title='Book 2' then raise exception 'fixture persistence failure'; end if; return new; end $$; create trigger reject_author_fixture before insert on wanted_items for each row execute function reject_author_fixture()`); err != nil {
		t.Fatal(err)
	}
	fixture := &bibliographyFixture{rows: []metadata.SearchResult{bibliographyCandidate(1, "2020-01-01", "Author"), bibliographyCandidate(2, "2021-01-01", "Author")}}
	service := NewService(store, nil, fixture)
	run, err := service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
	if err != nil || run.ErrorCount != 1 || run.WantedCreated != 1 {
		t.Fatal(run, err)
	}
	var unsynced bool
	if err := db.QueryRow(`select last_sync_at is null from author_subscriptions where id=$1`, sub.ID).Scan(&unsynced); err != nil {
		t.Fatal(err)
	}
	if !unsynced {
		t.Fatal("failed persistence marked author synced")
	}
	if _, err := db.Exec(`drop trigger reject_author_fixture on wanted_items`); err != nil {
		t.Fatal(err)
	}
	run, err = service.MonitorAuthors(ctx, AuthorMonitorRequest{})
	if err != nil || run.AuthorsChecked != 0 {
		t.Fatal("failed author must not pin every scheduled batch", run, err)
	}
	// An operator retry bypasses check backoff after the underlying failure is fixed.
	run, err = service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
	if err != nil || run.ErrorCount != 0 || run.AuthorsChecked != 1 {
		t.Fatal(run, err)
	}
	var count int
	if err := db.QueryRow(`select count(*) from wanted_items`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("retry duplicated committed first book", count)
	}
}

func TestAuthorSyncPreservesTrackedEditionAndRemovedManualState(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: true, MissingBookPolicy: "all"})
	if err != nil {
		t.Fatal(err)
	}
	original := bibliographyCandidate(1, "2020-01-01", "Author")
	original.Edition.ID = "hardcover:1:edition"
	item, err := store.CreateWanted(ctx, CreateRequest{Result: original, Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update wanted_items set status='removed',monitored=false,title='Manual title',quality_profile='custom' where id=$1`, item.ID); err != nil {
		t.Fatal(err)
	}
	fixture := &bibliographyFixture{rows: []metadata.SearchResult{bibliographyCandidate(1, "2020-01-01", "Author")}}
	run, err := NewService(store, nil, fixture).MonitorAuthors(ctx, AuthorMonitorRequest{Force: true, AuthorIDs: []string{sub.ID}})
	if err != nil || run.ErrorCount != 0 || run.WantedCreated != 0 || run.Items[0].SkippedCount != 1 {
		t.Fatal(run, err)
	}
	got, err := store.GetWanted(ctx, item.ID)
	if err != nil || got.Title != "Manual title" || got.QualityProfile != "custom" || got.Status != "removed" || got.Monitored {
		t.Fatal(got, err)
	}
	var count int
	if err := db.QueryRow(`select count(*) from wanted_items`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("legacy edition duplicated as work", count)
	}
}

func TestConcurrentBibliographyAddsCreateOneTrackedWork(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	var created atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, err := store.CreateWanted(ctx, CreateRequest{OnlyIfUntracked: true, Result: bibliographyCandidate(1, "2020-01-01", "Author"), Format: "ebook"})
			if err != nil {
				t.Error(err)
				return
			}
			if !item.alreadyTracked {
				created.Add(1)
			}
		}()
	}
	wg.Wait()
	var count int
	if err := db.QueryRow(`select count(*) from wanted_items`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 || created.Load() != 1 {
		t.Fatal(count, created.Load())
	}
}
