package wanted

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

type compatEvidenceFixture struct {
	*workerFixture
	reads int
}

func (f *compatEvidenceFixture) LiveDownloadEvidence(ctx context.Context, q acquisition.DownloadListQuery) acquisition.DownloadEvidence {
	f.reads++
	return f.workerFixture.LiveDownloadEvidence(ctx, q)
}

func TestCompatibilityBooksUseNativeFileEvidence(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), nil)
	ctx := context.Background()
	missing := evidenceBook(t, db, "Lost imported book", "ebook")
	present := evidenceBook(t, db, "Present", "ebook")
	evidenceFile(t, db, present, "/fixture/present.epub", "present")
	partial := evidenceBook(t, db, "Partial", "audiobook")
	a := evidenceFile(t, db, partial, "/fixture/a.mp3", "present")
	b := evidenceFile(t, db, partial, "/fixture/b.mp3", "missing")
	evidenceManifest(t, db, partial, a, b)
	unknown := evidenceBook(t, db, "Unverified", "audiobook")
	evidenceFile(t, db, unknown, "/fixture/legacy.mp3", "present")
	if _, err := db.Exec(`update wanted_items set status='imported' where id=$1`, missing.ID); err != nil {
		t.Fatal(err)
	}
	books, err := s.CompatibilityBooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]WantedItem{}
	for _, book := range books {
		byID[book.ID] = book
		if book.CompatibilityProfile == nil {
			t.Fatal("missing profile snapshot")
		}
	}
	for id, state := range map[string]string{missing.ID: "missing", partial.ID: "incomplete", unknown.ID: "unknown"} {
		if byID[id].DerivedState != state {
			t.Fatal(id, byID[id])
		}
	}
	if byID[present.ID].StateEvidence.Files.State != "present" || byID[present.ID].StateEvidence.Files.PresentFiles != 1 {
		t.Fatal(byID[present.ID])
	}
	f := &compatEvidenceFixture{workerFixture: &workerFixture{evidence: acquisition.DownloadEvidence{Status: "unavailable"}}}
	s = NewService(NewStore(db), f)
	books, err = s.CompatibilityBooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, book := range books {
		if book.ID == missing.ID && (book.DerivedState != "unknown" || book.StateEvidence.Downloads != "unavailable") {
			t.Fatal(book)
		}
	}
	if f.reads != 1 {
		t.Fatal("client queried per book", f.reads)
	}
	page, e := s.CompatibilityBookPage(ctx, CompatibilityBookPageQuery{Page: 1, PageSize: 100, State: "missing", SortKey: "title", SortDirection: "ascending"})
	if e != nil || page.Total != 0 || page.Unknown != 2 || page.StateCounts["incomplete"] != 1 || page.Downloads != "unavailable" || f.reads != 2 {
		t.Fatal(page, e, f.reads)
	}
	db.Close()
	if _, err = s.CompatibilityBooks(ctx); err == nil {
		t.Fatal("closed persistence appeared empty")
	}
}

func TestCompatibilityWantedPagesCompleteAndBounded(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), nil)
	ctx := context.Background()
	if _, err := db.Exec(`insert into wanted_items(wanted_format,title,author_name,status,monitored) select 'ebook','Same title','Writer','imported',true from generate_series(1,10001)`); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	times := []time.Duration{}
	for page := 1; page <= 101; page++ {
		start := time.Now()
		result, err := s.CompatibilityBookPage(ctx, CompatibilityBookPageQuery{Page: page, PageSize: 100, State: "missing", SortKey: "title", SortDirection: "ascending"})
		times = append(times, time.Since(start))
		if err != nil || result.Total != 10001 || len(result.Books) > 100 || result.StateCounts["missing"] != 10001 {
			t.Fatal(page, result.Total, err)
		}
		for _, book := range result.Books {
			if seen[book.ID] {
				t.Fatal("duplicate", book.ID)
			}
			seen[book.ID] = true
		}
	}
	if len(seen) != 10001 {
		t.Fatal(len(seen))
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	t.Logf("10001 missing books, 101 offset pages, p95 %s", times[len(times)*95/100])
}

func TestCompatibilityMutationRollsBackAndFencesStaleBooks(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), nil)
	ctx := context.Background()
	a := evidenceBook(t, db, "A", "ebook")
	b := evidenceBook(t, db, "B", "ebook")
	badRoot := "00000000-0000-0000-0000-000000000001"
	request := CompatibilityBookMutation{Books: []CompatibilityBookEdit{{ID: a.ID, UpdatedAt: a.UpdatedAt, Update: WantedUpdateRequest{Title: "Changed A", Tags: []string{"should-roll-back"}, TagsSet: true}}, {ID: b.ID, UpdatedAt: b.UpdatedAt, Update: WantedUpdateRequest{RootFolderID: &badRoot}}}}
	if _, err := s.ApplyCompatibilityBooks(ctx, request); err == nil {
		t.Fatal("invalid second book committed batch")
	}
	current, err := s.Get(ctx, a.ID)
	if err != nil || current.Title != a.Title || !current.UpdatedAt.Equal(a.UpdatedAt) {
		t.Fatal(current, err)
	}
	var tags int
	if err = db.QueryRow(`select count(*) from tags where label='should-roll-back'`).Scan(&tags); err != nil || tags != 0 {
		t.Fatal(tags, err)
	}
	request.Books = request.Books[:1]
	request.Books[0].UpdatedAt = a.UpdatedAt.Add(-time.Second)
	if _, err = s.ApplyCompatibilityBooks(ctx, request); !errors.Is(err, ErrCompatibilityBookSelection) {
		t.Fatal(err)
	}
	request.Books[0].UpdatedAt = a.UpdatedAt
	if _, err = db.Exec(`update wanted_items set status='removed',monitored=false where id=$1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyCompatibilityBooks(ctx, request); !errors.Is(err, ErrCompatibilityBookSelection) {
		t.Fatal(err)
	}
	current, err = s.Get(ctx, a.ID)
	if err != nil || current.Status != "removed" || current.Monitored {
		t.Fatal(current, err)
	}
	request = CompatibilityBookMutation{Delete: true, Books: []CompatibilityBookEdit{{ID: b.ID, UpdatedAt: b.UpdatedAt}}}
	result, err := s.ApplyCompatibilityBooks(ctx, request)
	if err != nil || len(result) != 1 || result[0].Status != "removed" || result[0].Monitored {
		t.Fatal(result, err)
	}
}

func TestCompatibilityMutationConcurrentReviewHasOneWinner(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), nil)
	ctx := context.Background()
	book := evidenceBook(t, db, "Concurrent", "ebook")
	no := false
	request := CompatibilityBookMutation{Books: []CompatibilityBookEdit{{ID: book.ID, UpdatedAt: book.UpdatedAt, Update: WantedUpdateRequest{Monitored: &no}}}}
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.ApplyCompatibilityBooks(ctx, request); errs <- err }()
	}
	wg.Wait()
	close(errs)
	ok, stale := 0, 0
	for err := range errs {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrCompatibilityBookSelection) {
			stale++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || stale != 1 {
		t.Fatal(ok, stale)
	}
}

func TestCompatibilityCutoffSharesNativeQualityEvidence(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	s := NewService(store, nil)
	book := evidenceBook(t, db, "Quality fixture", "ebook")
	evidenceFile(t, db, book, "/library/compat-quality.epub", "present")
	decisions, err := store.UpsertReleaseDecisions(ctx, book.ID, []ReleaseDecision{{SourceID: "compat-quality", Title: "Quality fixture EPUB", Score: 0, Approved: true}})
	if err != nil || len(decisions) != 1 {
		t.Fatal(decisions, err)
	}
	for _, score := range []float64{0, 500, 5000} {
		decision := decisions[0]
		decision.Score = score
		if err := store.MarkWantedCurrentRelease(ctx, book.ID, decision); err != nil {
			t.Fatal(err)
		}
		native, err := s.BookCollection(ctx, BookCollectionQuery{State: "cutoffUnmet"})
		if err != nil {
			t.Fatal(err)
		}
		compat, err := s.CompatibilityBookPage(ctx, CompatibilityBookPageQuery{Page: 1, PageSize: 100, State: "cutoffUnmet", SortKey: "title", SortDirection: "ascending"})
		if err != nil || native.Filtered != compat.Total || len(native.Books) != len(compat.Books) {
			t.Fatal(score, native, compat, err)
		}
		for _, item := range compat.Books {
			if item.ID != book.ID || item.CompatibilityProfile == nil || item.StateEvidence.Files.State != "present" {
				t.Fatal(item)
			}
		}
	}
	// An old imported status and installed release cannot put a missing file in cutoff.
	if _, err := db.Exec(`update wanted_items set status='imported', current_release_score=0 where id=$1`, book.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update files set presence_state='missing' where path='/library/compat-quality.epub'`); err != nil {
		t.Fatal(err)
	}
	page, err := s.CompatibilityBookPage(ctx, CompatibilityBookPageQuery{Page: 1, PageSize: 100, State: "cutoffUnmet", SortKey: "title", SortDirection: "ascending"})
	if err != nil || page.Total != 0 || page.StateCounts["missing"] != 1 {
		t.Fatal(page, err)
	}
}
