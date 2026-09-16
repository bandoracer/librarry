package wanted

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func authorReviewFixture(t *testing.T, db *sql.DB, n int) AuthorMetadataReview {
	t.Helper()
	raw, _ := json.Marshal(bibliographyCandidate(n, "2000", "Author"))
	review, err := scanAuthorMetadataReview(db.QueryRow(`insert into author_metadata_reviews(candidate_key,title,author_name,provider,result,quality_profile,tags) values($1,'Candidate','Author','Hardcover',$2,'premium','review-tag') returning `+authorReviewColumns, string(raw), string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	return review
}

func TestAuthorReviewCollectionTraversesEveryCandidate(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	_, err := db.Exec(`insert into author_metadata_reviews(candidate_key,title,author_name,provider,created_at) select i::text,'Candidate '||i,'Author','Hardcover','2026-01-01'::timestamptz from generate_series(1,10001) i`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`insert into author_metadata_reviews(candidate_key,title,status,wanted_format) values('old','Audio hidden','ignored','audiobook')`)
	if err != nil {
		t.Fatal(err)
	}
	query := AuthorMetadataReviewQuery{Limit: 100}
	seen := map[string]bool{}
	var durations []time.Duration
	firstCursor := ""
	for {
		start := time.Now()
		page, e := store.AuthorReviewCollection(ctx, query)
		durations = append(durations, time.Since(start))
		if e != nil {
			t.Fatal(e)
		}
		if page.Total != 10002 || page.Filtered != 10001 || page.Counts["ignored"] != 1 || len(page.Reviews) > 100 {
			t.Fatal(page)
		}
		for _, r := range page.Reviews {
			if seen[r.ID] || len(r.Revision) != 64 {
				t.Fatal("duplicate or missing revision", r.ID)
			}
			seen[r.ID] = true
		}
		if firstCursor == "" {
			firstCursor = page.NextCursor
		}
		if page.NextCursor == "" {
			break
		}
		query.Cursor = page.NextCursor
	}
	if len(seen) != 10001 {
		t.Fatal(len(seen))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("10001 candidates, %d pages, p95 %s", len(durations), durations[len(durations)*95/100])
	for _, q := range []AuthorMetadataReviewQuery{{Cursor: firstCursor, Status: "all"}, {Cursor: firstCursor, Search: "changed"}, {Cursor: firstCursor, Format: "audiobook"}, {Status: "invalid"}, {Limit: 101}, {Cursor: "bad"}} {
		if _, e := store.AuthorReviewCollection(ctx, q); !errors.Is(e, ErrBookPage) {
			t.Fatal(q, e)
		}
	}
	page, err := store.AuthorReviewCollection(ctx, AuthorMetadataReviewQuery{Status: "all", Format: "audiobook", Search: "hidden"})
	if err != nil || page.Filtered != 1 || len(page.Reviews) != 1 {
		t.Fatal(page, err)
	}
}

func TestAuthorReviewResolutionAtomicReplayAndOwnerSettings(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	review := authorReviewFixture(t, db, 1)
	root := authorTestRoot(t, db, "/saved", "ebook")
	if _, err := db.Exec(`update author_metadata_reviews set root_folder_id=$1 where id=$2`, root, review.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveAuthorReview(ctx, review.ID, AuthorMetadataReviewDecisionRequest{Action: "wanted", Revision: review.Revision}); !errors.Is(err, ErrAuthorReviewChanged) {
		t.Fatal(err)
	}
	review, _ = store.GetAuthorMetadataReview(ctx, review.ID)
	if _, err := db.Exec(`create function reject_review_history() returns trigger language plpgsql as $$ begin raise exception 'controlled history failure'; end $$; create trigger fail_review_history before insert on history_events for each row execute function reject_review_history()`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveAuthorReview(ctx, review.ID, AuthorMetadataReviewDecisionRequest{Action: "wanted", Revision: review.Revision}); err == nil {
		t.Fatal("history failure accepted")
	}
	var count int
	if err := db.QueryRow(`select count(*) from wanted_items`).Scan(&count); err != nil || count != 0 {
		t.Fatal("book escaped rollback", count, err)
	}
	current, _ := store.GetAuthorMetadataReview(ctx, review.ID)
	if current.Status != "pending" {
		t.Fatal(current)
	}
	if _, err := db.Exec(`drop trigger fail_review_history on history_events`); err != nil {
		t.Fatal(err)
	}
	outcome, err := store.ResolveAuthorReview(ctx, review.ID, AuthorMetadataReviewDecisionRequest{Action: "wanted", Revision: review.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.AlreadyTracked || outcome.WantedItem.RootFolderID != root || outcome.WantedItem.QualityProfile != "premium" || outcome.WantedItem.Tags[0] != "review-tag" {
		t.Fatal(outcome)
	}
	replay, err := store.ResolveAuthorReview(ctx, review.ID, AuthorMetadataReviewDecisionRequest{Action: "wanted", Revision: review.Revision})
	if err != nil || !replay.Replayed || replay.WantedItem.ID != outcome.WantedItem.ID {
		t.Fatal(replay, err)
	}
	if _, err := store.ResolveAuthorReview(ctx, review.ID, AuthorMetadataReviewDecisionRequest{Action: "ignore"}); !errors.Is(err, ErrAuthorReviewChanged) {
		t.Fatal(err)
	}
	if err := db.QueryRow(`select count(*) from history_events`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	// A new review for an already tracked work must not restore removed state,
	// change monitoring, or replace the owner's destination/profile/tags.
	ownerRoot := authorTestRoot(t, db, "/owner", "ebook")
	if _, err := db.Exec(`update wanted_items set root_folder_id=$1,quality_profile='owner',tags='owner-tag',status='removed',monitored=false where id=$2`, ownerRoot, outcome.WantedItem.ID); err != nil {
		t.Fatal(err)
	}
	next := authorReviewFixture(t, db, 1)
	preserved, err := store.ResolveAuthorReview(ctx, next.ID, AuthorMetadataReviewDecisionRequest{Action: "wanted", Revision: next.Revision})
	if err != nil {
		t.Fatal(err)
	}
	item := preserved.WantedItem
	if !preserved.AlreadyTracked || item.ID != outcome.WantedItem.ID || item.RootFolderID != ownerRoot || item.QualityProfile != "owner" || item.Status != "removed" || item.Monitored || item.Tags[0] != "owner-tag" {
		t.Fatal(preserved)
	}
}

func TestAuthorReviewConcurrentDecisions(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{false: "opposing", true: "same"}[same], func(t *testing.T) {
			db := testdb.Open(t)
			store := NewStore(db)
			ctx := context.Background()
			review := authorReviewFixture(t, db, 1)
			actions := []string{"wanted", "ignore"}
			if same {
				actions[1] = "wanted"
			}
			var wg sync.WaitGroup
			start := make(chan struct{})
			results := make([]AuthorMetadataReviewDecision, 2)
			errs := make([]error, 2)
			for i := range actions {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					results[i], errs[i] = store.ResolveAuthorReview(ctx, review.ID, AuthorMetadataReviewDecisionRequest{Action: actions[i], Revision: review.Revision})
				}(i)
			}
			close(start)
			wg.Wait()
			successes, replays, conflicts := 0, 0, 0
			for i, e := range errs {
				if e == nil {
					successes++
					if results[i].Replayed {
						replays++
					}
				} else if errors.Is(e, ErrAuthorReviewChanged) {
					conflicts++
				} else {
					t.Fatal(e)
				}
			}
			if same && (successes != 2 || replays != 1) || !same && (successes != 1 || conflicts != 1) {
				t.Fatal(successes, replays, conflicts)
			}
			var count int
			if err := db.QueryRow(`select count(*) from history_events`).Scan(&count); err != nil || count != 1 {
				t.Fatal(count, err)
			}
			current, err := store.GetAuthorMetadataReview(ctx, review.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = db.QueryRow(`select count(*) from wanted_items`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if current.Status == "ignored" && count != 0 || current.Status == "wanted" && count != 1 {
				t.Fatal(current, count)
			}
		})
	}
}
