package importlists

import (
	"context"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestEntryToSearchResultMapsHardcoverIdentity(t *testing.T) {
	entry := Entry{
		SourceKey:   "hardcover:12345",
		Title:       "Project Hail Mary",
		AuthorName:  "Andy Weir",
		CoverURL:    "https://covers.example/phm.jpg",
		ReleaseDate: "2026-07-04",
	}
	result := EntryToSearchResult(entry, "ebook")
	if result.Provider != "Hardcover" || result.Work.ID != "hardcover:12345" || result.RawSourceKey != "hardcover:12345" {
		t.Fatalf("unexpected identity mapping: %+v", result)
	}
	if result.Work.Title != "Project Hail Mary" || result.Work.CoverURL != entry.CoverURL {
		t.Fatalf("unexpected work mapping: %+v", result.Work)
	}
	if len(result.Work.Authors) != 1 || result.Work.Authors[0].Name != "Andy Weir" {
		t.Fatalf("unexpected authors: %+v", result.Work.Authors)
	}
	if result.Edition.PublishedDate != "2026-07-04" || string(result.Edition.Format) != "ebook" {
		t.Fatalf("unexpected edition: %+v", result.Edition)
	}
	// The wanted store derives sourceKey = firstNonEmpty(Edition.ID, Work.ID),
	// so Edition.ID must stay empty for dedupe against search-added books.
	if result.Edition.ID != "" {
		t.Fatalf("expected empty edition id, got %q", result.Edition.ID)
	}
}

func TestEntryExcludedMatchesSourceKeyAndTitle(t *testing.T) {
	exclusions := []Exclusion{
		{SourceKey: "hardcover:99", Title: "Ignored Book"},
		{Title: "Skip Me", AuthorName: "Andy Weir", SourceKey: "manual:skip-me:andy-weir"},
		{Title: "Any Author Skip", SourceKey: "manual:any-author-skip:"},
	}
	if excluded, reason := EntryExcluded(Entry{SourceKey: "hardcover:99", Title: "Other"}, exclusions); !excluded || reason != "excluded by source key" {
		t.Fatalf("expected source-key exclusion, got %v %q", excluded, reason)
	}
	if excluded, _ := EntryExcluded(Entry{SourceKey: "hardcover:1", Title: "skip me", AuthorName: "ANDY WEIR"}, exclusions); !excluded {
		t.Fatal("expected case-insensitive title+author exclusion")
	}
	if excluded, _ := EntryExcluded(Entry{SourceKey: "hardcover:1", Title: "Skip Me", AuthorName: "Someone Else"}, exclusions); excluded {
		t.Fatal("expected author mismatch to not exclude")
	}
	if excluded, _ := EntryExcluded(Entry{SourceKey: "hardcover:2", Title: "Any Author Skip", AuthorName: "Whoever"}, exclusions); !excluded {
		t.Fatal("expected title-only exclusion to match any author")
	}
	if excluded, _ := EntryExcluded(Entry{SourceKey: "hardcover:3", Title: "Fresh"}, exclusions); excluded {
		t.Fatal("expected unexcluded entry to pass")
	}
}

type fakeGateway struct {
	existing map[string]bool
	creates  []wanted.CreateRequest
	updates  []wanted.WantedUpdateRequest
	searches []string
}

func (f *fakeGateway) Create(_ context.Context, request wanted.CreateRequest) (wanted.WantedItem, error) {
	f.creates = append(f.creates, request)
	return wanted.WantedItem{ID: "wanted-" + request.Result.Work.ID, Title: request.Result.Work.Title}, nil
}

func (f *fakeGateway) UpdateWanted(_ context.Context, id string, request wanted.WantedUpdateRequest) (wanted.WantedItem, error) {
	f.updates = append(f.updates, request)
	return wanted.WantedItem{ID: id}, nil
}

func (f *fakeGateway) SearchReleases(_ context.Context, wantedID string, _ wanted.SearchReleasesRequest) (wanted.SearchOutcome, error) {
	f.searches = append(f.searches, wantedID)
	return wanted.SearchOutcome{}, nil
}

func (f *fakeGateway) WantedSourceKeySet(context.Context) (map[string]bool, error) {
	if f.existing == nil {
		return map[string]bool{}, nil
	}
	return f.existing, nil
}

type fakeFetcher struct {
	entries []Entry
}

func (f fakeFetcher) FetchList(context.Context, map[string]string, int) ([]Entry, error) {
	return f.entries, nil
}

// syncOneList drives the per-list sync logic without Postgres.
func syncOneList(t *testing.T, list List, entries []Entry, exclusions []Exclusion, gateway *fakeGateway) SyncOutcome {
	t.Helper()
	service := NewService(nil, gateway, fakeFetcher{entries: entries}, nil)
	outcome := SyncOutcome{Status: "completed"}
	existing, err := gateway.WantedSourceKeySet(context.Background())
	if err != nil {
		t.Fatalf("source keys: %v", err)
	}
	service.syncList(context.Background(), list, exclusions, existing, &outcome)
	return outcome
}

func TestSyncListDedupesAndHonorsExclusions(t *testing.T) {
	gateway := &fakeGateway{existing: map[string]bool{
		wanted.SourceIdentity("Hardcover", "hardcover:1", "ebook"): true,
	}}
	list := List{ID: "list-1", Name: "TBR", Type: "hardcover", Enabled: true, Monitor: "all", QualityProfile: "retail"}
	entries := []Entry{
		{SourceKey: "hardcover:1", Title: "Already Tracked", AuthorName: "A"},
		{SourceKey: "hardcover:2", Title: "Excluded Book", AuthorName: "B"},
		{SourceKey: "hardcover:3", Title: "Fresh Book", AuthorName: "C"},
	}
	exclusions := []Exclusion{{SourceKey: "hardcover:2"}}

	outcome := syncOneList(t, list, entries, exclusions, gateway)

	if outcome.EntriesFound != 3 || outcome.WantedCreated != 1 ||
		outcome.SkippedExisting != 1 || outcome.SkippedExcluded != 1 || outcome.ErrorCount != 0 {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if len(gateway.creates) != 1 || gateway.creates[0].Result.Work.Title != "Fresh Book" {
		t.Fatalf("expected only the fresh book to be created, got %+v", gateway.creates)
	}
	if gateway.creates[0].QualityProfile != "retail" {
		t.Fatalf("expected list quality profile to propagate, got %+v", gateway.creates[0])
	}
	if len(gateway.updates) != 0 {
		t.Fatalf("expected no monitor/root update for monitor=all without root, got %+v", gateway.updates)
	}
	if len(gateway.searches) != 0 {
		t.Fatal("expected no searches without searchOnAdd")
	}
}

func TestSyncListMonitorNoneRootFolderAndSearchOnAdd(t *testing.T) {
	gateway := &fakeGateway{}
	list := List{
		ID: "list-2", Name: "Shelf", Type: "hardcover", Enabled: true,
		Monitor: "none", RootFolderID: "root-7", QualityProfile: "standard",
		SearchOnAdd: true,
	}
	outcome := syncOneList(t, list, []Entry{{SourceKey: "hardcover:9", Title: "Quiet Add"}}, nil, gateway)

	if outcome.WantedCreated != 1 {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if len(gateway.updates) != 1 {
		t.Fatalf("expected one update, got %+v", gateway.updates)
	}
	update := gateway.updates[0]
	if update.Monitored == nil || *update.Monitored {
		t.Fatalf("expected monitored=false for monitor=none, got %+v", update)
	}
	if update.RootFolderID == nil || *update.RootFolderID != "root-7" {
		t.Fatalf("expected root folder pin, got %+v", update)
	}
	// monitor=none suppresses search-on-add (nothing to grab for an
	// unmonitored book).
	if len(gateway.searches) != 0 {
		t.Fatalf("expected no searches for unmonitored adds, got %+v", gateway.searches)
	}

	// With monitor=all, search-on-add fires.
	gateway = &fakeGateway{}
	list.Monitor = "all"
	outcome = syncOneList(t, list, []Entry{{SourceKey: "hardcover:10", Title: "Search Me"}}, nil, gateway)
	if outcome.SearchesStarted != 1 || len(gateway.searches) != 1 {
		t.Fatalf("expected search-on-add, got %+v searches=%v", outcome, gateway.searches)
	}
}

func TestSyncListSameSourceKeyOnlyCreatedOnce(t *testing.T) {
	gateway := &fakeGateway{}
	list := List{ID: "list-3", Name: "Dupes", Type: "hardcover", Enabled: true, Monitor: "all"}
	entries := []Entry{
		{SourceKey: "hardcover:5", Title: "Same Book"},
		{SourceKey: "hardcover:5", Title: "Same Book"},
	}
	outcome := syncOneList(t, list, entries, nil, gateway)
	if outcome.WantedCreated != 1 || outcome.SkippedExisting != 1 {
		t.Fatalf("expected in-run dedupe, got %+v", outcome)
	}
}

func TestListFormatFromSettings(t *testing.T) {
	if got := listFormat(List{Settings: map[string]string{"format": "audiobook"}}); got != "audiobook" {
		t.Fatalf("expected audiobook, got %s", got)
	}
	if got := listFormat(List{Settings: map[string]string{}}); got != "ebook" {
		t.Fatalf("expected ebook default, got %s", got)
	}
}

func TestEntryFromHardcoverBookToleratesShapes(t *testing.T) {
	entry, ok := entryFromHardcoverBook(map[string]any{
		"id":           float64(42),
		"title":        "The Martian",
		"release_date": "2014-02-11",
		"cached_image": map[string]any{"url": "https://img.example/m.jpg"},
		"cached_contributors": []any{
			map[string]any{"author": map[string]any{"name": "Andy Weir"}},
		},
	})
	if !ok {
		t.Fatal("expected entry")
	}
	if entry.SourceKey != "hardcover:42" || entry.AuthorName != "Andy Weir" ||
		entry.CoverURL != "https://img.example/m.jpg" || entry.ReleaseDate != "2014-02-11" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if _, ok := entryFromHardcoverBook(map[string]any{"id": "1"}); ok {
		t.Fatal("expected title-less book to be skipped")
	}
}
