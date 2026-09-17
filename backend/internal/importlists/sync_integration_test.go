package importlists

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type failingListFetcher struct{}

func (failingListFetcher) FetchList(context.Context, map[string]string, int) ([]Entry, error) {
	return []Entry{{SourceKey: "hardcover:1", Title: "Partial book"}}, errors.New("page two failed")
}
func TestListSyncFailedFetchDoesNotAddPartialEntries(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	list, err := store.CreateList(ctx, List{Name: "Fixture", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, wanted.NewService(wanted.NewStore(db), nil, nil), failingListFetcher{}, nil)
	out, err := service.Sync(ctx, []string{list.ID}, "test")
	if err != nil || out.ErrorCount != 1 || out.WantedCreated != 0 || out.Status != "completed_with_errors" {
		t.Fatal(out, err)
	}
	var count int
	db.QueryRow(`select count(*) from wanted_items`).Scan(&count)
	got, err := store.GetList(ctx, list.ID)
	if err != nil || got.LastSyncedAt != nil || count != 0 {
		t.Fatal(got, count, err)
	}
}
func TestListSyncPreservesExistingAndCommitsDefaultsWithBook(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	ws := wanted.NewStore(db)
	var root string
	if err := db.QueryRow(`insert into root_folders(name,path,media_format) values ('Fixture','/fixture','ebook') returning id::text`).Scan(&root); err != nil {
		t.Fatal(err)
	}
	prior := EntryToSearchResult(Entry{SourceKey: "hardcover:1", Title: "Original"}, "ebook")
	prior.Edition.ID = "hardcover:1:edition"
	old, err := ws.CreateWanted(ctx, wanted.CreateRequest{Result: prior, Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`update wanted_items set title='Manual title',status='removed',monitored=true,quality_profile='custom' where id=$1`, old.ID); err != nil {
		t.Fatal(err)
	}
	// The insert itself must already contain list defaults; a second write is
	// too late because an acquisition worker could observe a monitored row.
	if _, err = db.Exec(`create function require_list_defaults() returns trigger language plpgsql as $$ begin
 if new.title='New book' and (new.monitored or new.root_folder_id is null) then raise exception 'missing initial defaults'; end if; return new; end $$;
 create trigger require_list_defaults before insert on wanted_items for each row execute function require_list_defaults()`); err != nil {
		t.Fatal(err)
	}
	list, err := store.CreateList(ctx, List{Name: "Fixture", Enabled: true, Monitor: "none", RootFolderID: root})
	if err != nil {
		t.Fatal(err)
	}
	rows := []Entry{{SourceKey: "hardcover:1", Title: "Provider title"}, {SourceKey: "hardcover:2", Title: "New book"}}
	service := NewService(store, wanted.NewService(ws, nil, nil), fakeFetcher{entries: rows}, nil)
	out, err := service.Sync(ctx, []string{list.ID}, "test")
	if err != nil || out.ErrorCount != 0 || out.SkippedExisting != 1 || out.WantedCreated != 1 {
		t.Fatal(out, err)
	}
	preserved, err := ws.GetWanted(ctx, old.ID)
	if err != nil || preserved.Title != "Manual title" || preserved.Status != "removed" || preserved.QualityProfile != "custom" || !preserved.Monitored || preserved.RootFolderID != "" {
		t.Fatal(preserved, err)
	}
	var monitored bool
	var savedRoot string
	if err = db.QueryRow(`select monitored,root_folder_id::text from wanted_items where source_key='hardcover:2'`).Scan(&monitored, &savedRoot); err != nil || monitored || savedRoot != root {
		t.Fatal(monitored, savedRoot, err)
	}
	got, err := store.GetList(ctx, list.ID)
	if err != nil || got.LastSyncedAt == nil {
		t.Fatal(got, err)
	}
}
func TestListSyncFailedPersistenceRetriesWithoutFalseSuccess(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	list, err := store.CreateList(ctx, List{Name: "Fixture", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	rows := []Entry{{SourceKey: "hardcover:1", Title: "First"}, {SourceKey: "hardcover:2", Title: "Second"}}
	service := NewService(store, wanted.NewService(wanted.NewStore(db), nil, nil), fakeFetcher{entries: rows}, nil)
	if _, err = db.Exec(`create function fail_list_second() returns trigger language plpgsql as $$ begin if new.title='Second' then raise exception 'fixture write failure'; end if; return new; end $$;
 create trigger fail_list_second before insert on wanted_items for each row execute function fail_list_second()`); err != nil {
		t.Fatal(err)
	}
	out, err := service.Sync(ctx, []string{list.ID}, "test")
	if err != nil || out.ErrorCount != 1 || out.WantedCreated != 1 {
		t.Fatal(out, err)
	}
	got, err := store.GetList(ctx, list.ID)
	if err != nil || got.LastSyncedAt != nil {
		t.Fatal(got, err)
	}
	if _, err = db.Exec(`drop trigger fail_list_second on wanted_items`); err != nil {
		t.Fatal(err)
	}
	out, err = service.Sync(ctx, []string{list.ID}, "test")
	if err != nil || out.ErrorCount != 0 || out.WantedCreated != 1 || out.SkippedExisting != 1 {
		t.Fatal(out, err)
	}
	got, err = store.GetList(ctx, list.ID)
	if err != nil || got.LastSyncedAt == nil {
		t.Fatal(got, err)
	}
	// A failed sync timestamp is a persistence error too.
	if _, err = db.Exec(`create function fail_list_stamp() returns trigger language plpgsql as $$ begin raise exception 'fixture timestamp failure'; end $$;
 create trigger fail_list_stamp before update on import_lists for each row execute function fail_list_stamp()`); err != nil {
		t.Fatal(err)
	}
	out, err = service.Sync(ctx, []string{list.ID}, "test")
	if err != nil || out.ErrorCount != 1 || out.Status != "completed_with_errors" {
		t.Fatal(out, err)
	}
}
func TestListSyncHonorsAllExclusionsAndEntireCatalog(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	if _, err := db.Exec(`insert into import_list_exclusions(source_key,created_at) select 'hardcover:'||n,now()-n*interval '1 second' from generate_series(1,1100) n`); err != nil {
		t.Fatal(err)
	}
	list, err := store.CreateList(ctx, List{Name: "Fixture", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	rows := []Entry{{SourceKey: "hardcover:1100", Title: "Old excluded item"}}
	for i := 1101; i <= 1305; i++ {
		rows = append(rows, Entry{SourceKey: fmt.Sprintf("hardcover:%d", i), Title: fmt.Sprintf("Book %d", i)})
	}
	service := NewService(store, wanted.NewService(wanted.NewStore(db), nil, nil), fakeFetcher{entries: rows}, nil)
	out, err := service.Sync(ctx, []string{list.ID}, "test")
	if err != nil || out.ErrorCount != 0 || out.EntriesFound != 206 || out.WantedCreated != 205 || out.SkippedExcluded != 1 {
		t.Fatal(out, err)
	}
	repeat, err := service.Sync(ctx, []string{list.ID}, "test")
	if err != nil || repeat.ErrorCount != 0 || repeat.WantedCreated != 0 || repeat.SkippedExisting != 205 || repeat.SkippedExcluded != 1 {
		t.Fatal(repeat, err)
	}
}
func TestConcurrentListSyncCreatesOneTrackedWork(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	list, err := store.CreateList(ctx, List{Name: "Fixture", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, wanted.NewService(wanted.NewStore(db), nil, nil), fakeFetcher{entries: []Entry{{SourceKey: "hardcover:1", Title: "Book"}}}, nil)
	var wg sync.WaitGroup
	var created atomic.Int32
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := service.Sync(ctx, []string{list.ID}, "test")
			if err != nil || out.ErrorCount != 0 {
				t.Error(out, err)
				return
			}
			created.Add(int32(out.WantedCreated))
		}()
	}
	wg.Wait()
	var count int
	db.QueryRow(`select count(*) from wanted_items`).Scan(&count)
	if count != 1 || created.Load() != 1 {
		t.Fatal(count, created.Load())
	}
}
