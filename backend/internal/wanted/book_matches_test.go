package wanted

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestBookMatchesCompleteIdentityLookup(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	s := NewService(store, noChoiceNetwork{})
	ctx := context.Background()
	result := bibliographyCandidate(1, "1990-01-01", "Original author")
	result.Work.ProviderIDs = []string{"openlibrary:OL1W"}
	book, err := store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook"})
	if err != nil {
		t.Fatal(err)
	}
	book, err = store.UpdateWanted(ctx, book.ID, WantedUpdateRequest{Title: "Owner title", AuthorName: "Owner author"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`insert into wanted_items(wanted_format,title,metadata_provider,source_key,status,created_at) select 'ebook','Owner title','Hardcover','unrelated:'||i,'wanted',now()+interval '1 day' from generate_series(1,10001)i`); err != nil {
		t.Fatal(err)
	}
	candidate := BookMatchCandidate{Key: "merged", Provider: "Open Library", WorkIDs: []string{"openlibrary:OL1W"}, Format: "ebook"}
	candidates := []BookMatchCandidate{candidate, {Key: "audio", Provider: candidate.Provider, WorkIDs: candidate.WorkIDs, Format: "audiobook"}, {Key: "same-title", Provider: "Hardcover", WorkIDs: []string{"adaptation:1"}, Format: "ebook"}, {Key: "no-identity", Format: "ebook"}}
	for i := len(candidates); i < 100; i++ {
		candidates = append(candidates, BookMatchCandidate{Key: fmt.Sprint(i), Provider: "Hardcover", SourceKey: fmt.Sprintf("unrelated:%d", i), Format: "ebook"})
	}
	var timings []time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		matches, e := s.MatchBooks(ctx, candidates)
		timings = append(timings, time.Since(start))
		if e != nil {
			t.Fatal(e)
		}
		if len(matches.Matches) != 100 || matches.ObservedAt.IsZero() {
			t.Fatal(matches)
		}
		for n, m := range matches.Matches {
			if m.Key != candidates[n].Key || m.Books == nil {
				t.Fatal(m)
			}
			expected := 1
			if n >= 1 && n <= 3 {
				expected = 0
			}
			if m.Total != expected || len(m.Books) != expected {
				t.Fatal(n, m)
			}
		}
		if b := matches.Matches[0].Books[0]; b.ID != book.ID || b.Title != "Owner title" || b.AuthorName != "Owner author" {
			t.Fatal(b)
		}
	}
	sort.Slice(timings, func(i, j int) bool { return timings[i] < timings[j] })
	t.Logf("100 candidates against 10002 books, p95 %s", timings[len(timings)*95/100])
	for _, status := range []string{"removed", "ignored"} {
		if _, err = db.Exec(`update wanted_items set status=$1,monitored=false where id=$2`, status, book.ID); err != nil {
			t.Fatal(err)
		}
		matches, e := s.MatchBooks(ctx, []BookMatchCandidate{candidate})
		if e != nil || matches.Matches[0].Total != 1 || matches.Matches[0].Books[0].Status != status {
			t.Fatal(matches, e)
		}
	}
	db.Close()
	if _, err = s.MatchBooks(ctx, candidates); err == nil {
		t.Fatal("closed database appeared empty")
	}
}

func TestBookMatchesAmbiguityAndTypedEvidence(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	s := NewService(store, nil)
	ctx := context.Background()
	result := bibliographyCandidate(1, "1990-01-01", "Author")
	for i := 0; i < 12; i++ {
		result.Edition.ID = fmt.Sprintf("hardcover-edition:%d", i)
		if _, err := store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook"}); err != nil {
			t.Fatal(err)
		}
	}
	candidate := candidateBookIdentity(result, "ebook")
	p, err := s.MatchBooks(ctx, []BookMatchCandidate{candidate})
	if err != nil || p.Matches[0].Total != 12 || len(p.Matches[0].Books) != 10 {
		t.Fatal(p, err)
	}
	// Edition-only evidence is typed and does not treat a work record as an edition.
	candidate.WorkIDs = nil
	p, err = s.MatchBooks(ctx, []BookMatchCandidate{candidate})
	if err != nil || p.Matches[0].Total != 1 {
		t.Fatal(p, err)
	}
	candidate.SourceKey = ""
	candidate.EditionIDs = []string{result.Work.ID}
	// A direct source key still supports legacy work placeholders, but the work
	// provider record must not traverse to its other concrete editions.
	p, err = s.MatchBooks(ctx, []BookMatchCandidate{candidate})
	if err != nil || p.Matches[0].Total != 0 {
		t.Fatal(p, err)
	}
}

func TestPreservedAddRejectsExistingWithoutChangingOwnerState(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	result := bibliographyCandidate(8, "1990-01-01", "Author")
	result.Work.ProviderIDs = []string{"openlibrary:OL8W"}
	before, err := store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook", QualityProfile: "premium", Tags: []string{"owner"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`update wanted_items set status='removed',monitored=false where id=$1`, before.ID); err != nil {
		t.Fatal(err)
	}
	before, err = store.GetWanted(ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	result.Work.Title = "Changed provider title"
	result.Provider = "Open Library"
	result.Work.ID = "openlibrary:OL8W"
	result.Edition.ID = "openlibrary:OL8M"
	_, err = store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook", QualityProfile: "standard", Tags: []string{"new"}, PreserveExisting: true})
	if !errors.Is(err, ErrBookAlreadyTracked) {
		t.Fatal(err)
	}
	after, err := store.GetWanted(ctx, before.ID)
	if err != nil || after.Title != before.Title || after.Status != before.Status || after.Monitored || after.QualityProfile != before.QualityProfile || !after.UpdatedAt.Equal(before.UpdatedAt) || strings.Join(after.Tags, ",") != strings.Join(before.Tags, ",") {
		t.Fatal(before, after, err)
	}
	// Format is part of the tracking identity; adding audio is allowed.
	if _, err = store.CreateWanted(ctx, CreateRequest{Result: result, Format: "audiobook", PreserveExisting: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateWanted(ctx, CreateRequest{Result: metadata.SearchResult{Work: metadata.Work{Title: "No identity"}}, PreserveExisting: true}); !errors.Is(err, ErrBookMatches) {
		t.Fatal(err)
	}
}

func TestPreservedAddConcurrentMergedIdentities(t *testing.T) {
	for _, storedAliases := range []bool{false, true} {
		t.Run(fmt.Sprint("stored aliases: ", storedAliases), func(t *testing.T) {
			db := testdb.Open(t)
			store := NewStore(db)
			ctx := context.Background()
			result := bibliographyCandidate(12, "1990-01-01", "Author")
			result.Work.ProviderIDs = []string{"openlibrary:OL12W"}
			other := result
			other.Provider = "Open Library"
			other.Work.ID = "openlibrary:OL12W"
			other.Work.ProviderIDs = []string{"hardcover:12"}
			other.Edition.ID = "openlibrary:OL12M"
			if storedAliases {
				seed, err := store.CreateWanted(ctx, CreateRequest{Result: result, Format: "ebook"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = db.Exec(`delete from wanted_items where id=$1`, seed.ID); err != nil {
					t.Fatal(err)
				}
				result.Work.ProviderIDs = nil
				other.Work.ProviderIDs = nil
			}

			errorsOut := make(chan error, 2)
			var wg sync.WaitGroup
			for _, r := range []metadata.SearchResult{result, other} {
				wg.Add(1)
				go func(r metadata.SearchResult) {
					defer wg.Done()
					_, e := store.CreateWanted(ctx, CreateRequest{Result: r, Format: "ebook", PreserveExisting: true})
					errorsOut <- e
				}(r)
			}
			wg.Wait()
			close(errorsOut)
			success, conflict := 0, 0
			for e := range errorsOut {
				if e == nil {
					success++
				} else if errors.Is(e, ErrBookAlreadyTracked) {
					conflict++
				} else {
					t.Fatal(e)
				}
			}
			if success != 1 || conflict != 1 {
				t.Fatal(success, conflict)
			}
			var n int
			if err := db.QueryRow(`select count(*) from wanted_items`).Scan(&n); err != nil || n != 1 {
				t.Fatal(n, err)
			}
		})
	}
}

func TestBookMatchesValidation(t *testing.T) {
	valid := BookMatchCandidate{Key: "one", Format: "ebook"}
	for _, candidates := range [][]BookMatchCandidate{{valid, valid}, {{Key: "one", Format: "video"}}, {{Format: "ebook"}}, {{Key: "one", Format: "ebook", SourceKey: strings.Repeat("x", 513)}}, make([]BookMatchCandidate, 101)} {
		if _, err := bookMatchAliases(candidates); !errors.Is(err, ErrBookMatches) {
			t.Fatal(candidates, err)
		}
	}
}
