package wanted

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestBookCollectionWholeCollectionCountsFilteringAndPaging(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	s := NewService(NewStore(db), nil)
	if _, err := db.Exec(`insert into wanted_items(id,wanted_format,title,author_name,status,monitored,created_at)
 select md5(n::text)::uuid,case when n%2=0 then 'ebook' else 'audiobook' end,'Tied title','Same Author',case when n=10002 then 'removed' when n=10003 then 'ignored' when n%3=0 then 'imported' else 'wanted' end,n%5<>0,'2020-01-01' from generate_series(1,10003)n`); err != nil {
		t.Fatal(err)
	}
	testdb.SeedRange(t, db, 10003, `insert into files(media_format,path,size_bytes,presence_state,import_status,metadata)
 select wanted_format,'/library/'||id||'.book',10,'missing','imported',jsonb_build_object('wantedId',id::text) from wanted_items
 where id in (select md5(n::text)::uuid from generate_series($1::integer,$2::integer)n)`)
	if _, err := db.Exec(`analyze wanted_items; analyze files; analyze file_wanted_links;`); err != nil {
		t.Fatal(err)
	}

	durations := []time.Duration{}
	for _, order := range []string{"title", "author", "status", "added"} {
		cursor := ""
		seen := map[string]bool{}
		for {
			start := time.Now()
			readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			page, err := s.BookCollection(readCtx, BookCollectionQuery{Sort: order, Cursor: cursor})
			cancel()
			durations = append(durations, time.Since(start))
			if err != nil || page.Total != 10001 || page.Filtered != 10001 || page.RecordedFiles != 10003 || page.Counts["missing"] != 8001 || page.Counts["unmonitored"] != 2000 {
				t.Fatal(page.Total, page.Filtered, page.Counts, err)
			}
			if len(page.Books) > 100 {
				t.Fatal("unbounded page")
			}
			for _, item := range page.Books {
				if seen[item.ID] {
					t.Fatal("duplicate", order, item.ID)
				}
				seen[item.ID] = true
			}
			cursor = page.NextCursor
			if cursor == "" {
				break
			}
		}
		if len(seen) != 10001 {
			t.Fatal("unreachable books", order, len(seen))
		}
	}
	page, err := s.BookCollection(ctx, BookCollectionQuery{Format: "audiobook", Monitor: "unmonitored", Search: "Same Author", State: "unmonitored", Limit: 1})
	if err != nil || page.Total != 10001 || page.Filtered != 1000 || len(page.Books) != 1 {
		t.Fatal(page, err)
	}
	if _, err := s.BookCollection(ctx, BookCollectionQuery{Format: "ebook", Cursor: page.NextCursor}); !errors.Is(err, ErrBookPage) {
		t.Fatal("cursor accepted with different filters", err)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("10,001-book collection reads: %d pages, p95 %s (local service including hydration/counts; no client IO)", len(durations), durations[(len(durations)*95-1)/100])
}

func TestBookCollectionMediaClientAndProfileEvidenceMatchDetails(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	missing := evidenceBook(t, db, "Missing", "ebook")
	complete := evidenceBook(t, db, "Complete", "ebook")
	evidenceFile(t, db, complete, "/library/full.epub", "present")
	partial := evidenceBook(t, db, "Partial", "audiobook")
	a := evidenceFile(t, db, partial, "/library/a.mp3", "present")
	b := evidenceFile(t, db, partial, "/library/b.mp3", "missing")
	evidenceManifest(t, db, partial, a, b)
	unknown := evidenceBook(t, db, "Unknown", "audiobook")
	evidenceFile(t, db, unknown, "/library/legacy.mp3", "present")
	unmonitored := evidenceBook(t, db, "Stopped", "ebook")
	if _, err := db.Exec(`update wanted_items set monitored=false where id=$1`, unmonitored.ID); err != nil {
		t.Fatal(err)
	}
	live := evidenceBook(t, db, "Live", "ebook")
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "fresh", Downloads: []acquisition.DownloadStatus{{State: "downloading", Tags: []string{"wanted:" + live.ID}}, {State: "downloading", Tags: []string{"wanted:invalid"}}}}}
	s := NewService(store, fixture)
	for _, status := range []string{"fresh", "partial", "unavailable", "notConfigured"} {
		fixture.evidence.Status = status
		page, err := s.BookCollection(ctx, BookCollectionQuery{})
		if err != nil || len(page.Books) != 6 {
			t.Fatal(page, err)
		}
		detail := s.AnnotateWantedStates(ctx, append([]WantedItem(nil), page.Books...))
		for i, item := range page.Books {
			if item.DerivedState != detail[i].DerivedState || *item.StateEvidence != *detail[i].StateEvidence {
				t.Fatalf("collection/detail mismatch: %+v / %+v", item.StateEvidence, detail[i].StateEvidence)
			}
		}
		for _, state := range []string{"missing", "incomplete", "unknown", "downloading", "cutoffUnmet", "downloaded", "unmonitored"} {
			filtered, err := s.BookCollection(ctx, BookCollectionQuery{State: state})
			if err != nil || filtered.Filtered != page.Counts[state] || len(filtered.Books) != filtered.Filtered {
				t.Fatal(state, filtered, err)
			}
			for _, item := range filtered.Books {
				if item.DerivedState != state {
					t.Fatal(item)
				}
			}
		}
	}
	if _, err := store.UpdateWanted(ctx, missing.ID, WantedUpdateRequest{Title: "Owner corrected title"}); err != nil {
		t.Fatal(err)
	}
	corrected, err := s.BookCollection(ctx, BookCollectionQuery{Search: "Owner corrected"})
	if err != nil || corrected.Filtered != 1 || corrected.Books[0].ID != missing.ID || corrected.Books[0].Title != "Owner corrected title" {
		t.Fatal(corrected, err)
	}
}

func TestBookCollectionRejectsInvalidFiltersAndKeepsEmptyLists(t *testing.T) {
	s := NewService(NewStore(testdb.Open(t)), nil)
	for _, q := range []BookCollectionQuery{{Format: "video"}, {Monitor: "yes"}, {State: "grabbed"}, {Sort: "random"}, {Limit: 101}, {Limit: -1}, {Cursor: "invalid"}, {Search: strings.Repeat("a", 257)}} {
		if _, err := s.BookCollection(context.Background(), q); !errors.Is(err, ErrBookPage) {
			t.Fatal(q, err)
		}
	}
	page, err := s.BookCollection(context.Background(), BookCollectionQuery{})
	if err != nil || page.Books == nil || len(page.Books) != 0 || page.Filtered != 0 || page.Total != 0 || page.NextCursor != "" {
		t.Fatal(page, err)
	}
}

func TestBookCollectionQualityProfilesAndInstalledScores(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	s := NewService(store, nil)
	book := evidenceBook(t, db, "Installed quality", "ebook")
	evidenceFile(t, db, book, "/library/quality.epub", "present")
	decisions, err := store.UpsertReleaseDecisions(ctx, book.ID, []ReleaseDecision{{SourceID: "fixture-release", Title: "Installed quality EPUB", Score: 500, Approved: true}})
	if err != nil || len(decisions) != 1 {
		t.Fatal(decisions, err)
	}
	for _, check := range []struct {
		score float64
		state string
	}{{500, "downloaded"}, {5000, "downloaded"}, {0, "cutoffUnmet"}} {
		release := decisions[0]
		release.Score = check.score
		if err := store.MarkWantedCurrentRelease(ctx, book.ID, release); err != nil {
			t.Fatal(err)
		}
		page, err := s.BookCollection(ctx, BookCollectionQuery{})
		if err != nil || len(page.Books) != 1 || page.Books[0].DerivedState != check.state || page.Counts[check.state] != 1 {
			t.Fatal(page, err)
		}
	}
	profile := defaultQualityProfile("fixture-profile", "ebook")
	profile.UpgradeAllowed = false
	if _, err := store.UpsertQualityProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateWanted(ctx, book.ID, WantedUpdateRequest{QualityProfile: "fixture-profile"}); err != nil {
		t.Fatal(err)
	}
	page, err := s.BookCollection(ctx, BookCollectionQuery{State: "downloaded"})
	if err != nil || len(page.Books) != 1 || page.Filtered != 1 {
		t.Fatal(page, err)
	}
	if _, err := store.UpdateWanted(ctx, book.ID, WantedUpdateRequest{QualityProfile: "unconfigured-profile"}); err != nil {
		t.Fatal(err)
	}
	page, err = s.BookCollection(ctx, BookCollectionQuery{State: "cutoffUnmet"})
	if err != nil || len(page.Books) != 1 || page.Filtered != 1 {
		t.Fatal(page, err)
	}
}
