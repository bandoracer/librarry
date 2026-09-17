package wanted

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestAuthorIgnoredReviewSurvivesPolicyAndEditionChanges(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: true, MissingBookPolicy: "future"})
	if err != nil {
		t.Fatal(err)
	}
	candidate := bibliographyCandidate(1, "2000-01-01", "Author")
	candidate.Edition.ID = "hardcover-edition:101"
	fixture := &bibliographyFixture{rows: []metadata.SearchResult{candidate}}
	service := NewService(store, nil, fixture)
	run, err := service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
	if err != nil || run.ErrorCount != 0 || len(run.Items) != 1 || len(run.Items[0].SkippedItems) != 1 {
		t.Fatal(run, err)
	}
	reviewID := run.Items[0].SkippedItems[0].ReviewID
	if _, err := service.ResolveAuthorMetadataReview(ctx, reviewID, AuthorMetadataReviewDecisionRequest{Action: "ignore"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update author_subscriptions set missing_book_policy='all',monitor_new_items=true where id=$1`, sub.ID); err != nil {
		t.Fatal(err)
	}
	fixture.rows[0].Edition.ID = "hardcover-edition:999"
	fixture.rows[0].Work.Title = "Changed provider title"
	for range 2 {
		run, err = service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
		if err != nil || run.ErrorCount != 0 || run.WantedCreated != 0 || run.Items[0].SkippedCount != 1 || !strings.Contains(run.Items[0].SkippedItems[0].Reason, "ignored") {
			t.Fatal(run, err)
		}
	}
	var wantedCount, reviews int
	if err := db.QueryRow(`select (select count(*) from wanted_items),(select count(*) from author_metadata_reviews)`).Scan(&wantedCount, &reviews); err != nil || wantedCount != 0 || reviews != 1 {
		t.Fatal(wantedCount, reviews, err)
	}
}
func TestAuthorExclusionsApplyBeforeFirstPolicyAndIncludeAllRows(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	ctx := context.Background()
	_, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: true, MissingBookPolicy: "first"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into import_list_exclusions(source_key,created_at) select 'hardcover:'||n,now()-n*interval '1 second' from generate_series(1,1100) n`); err != nil {
		t.Fatal(err)
	}
	fixture := &bibliographyFixture{rows: []metadata.SearchResult{bibliographyCandidate(1100, "1990-01-01", "Author"), bibliographyCandidate(1101, "2000-01-01", "Author"), bibliographyCandidate(1102, "2010-01-01", "Author")}}
	run, err := NewService(store, nil, fixture).MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
	if err != nil || run.ErrorCount != 0 || run.WantedCreated != 1 || run.Items[0].WantedItems[0].Title != "Book 1101" {
		t.Fatal(run, err)
	}
}
func TestAuthorMonitorAllSevenPoliciesRepeatWithoutChangingManualTracking(t *testing.T) {
	for _, tc := range []struct {
		policy  string
		created int
	}{{"all", 3}, {"missing", 3}, {"existing", 1}, {"first", 1}, {"latest", 2}, {"future", 1}, {"none", 0}} {
		t.Run(tc.policy, func(t *testing.T) {
			db := testdb.Open(t)
			store := NewStore(db)
			ctx := context.Background()
			_, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: tc.policy != "none", MissingBookPolicy: tc.policy})
			if err != nil {
				t.Fatal(err)
			}
			fixture := &bibliographyFixture{}
			for i, date := range []string{"2000-01-01", "2010-01-01", "2020-01-01", "2099-01-01"} {
				fixture.rows = append(fixture.rows, bibliographyCandidate(i+1, date, "Author"))
			}
			old, err := store.CreateWanted(ctx, CreateRequest{Result: fixture.rows[1], Format: "ebook"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`update wanted_items set monitored=false,title='Manual title',quality_profile='custom' where id=$1`, old.ID); err != nil {
				t.Fatal(err)
			}
			var fileID string
			if err := db.QueryRow(`insert into files(path,media_format,presence_state) values ('/fixture/book.epub','ebook','present') returning id::text`).Scan(&fileID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`insert into file_wanted_links(file_id,wanted_item_id) values ($1,$2)`, fileID, old.ID); err != nil {
				t.Fatal(err)
			}
			service := NewService(store, nil, fixture)
			run, err := service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
			if err != nil || run.ErrorCount != 0 || run.WantedCreated != tc.created {
				t.Fatal(run, err)
			}
			repeat, err := service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
			if err != nil || repeat.ErrorCount != 0 || repeat.WantedCreated != 0 {
				t.Fatal(repeat, err)
			}
			retained, err := store.GetWanted(ctx, old.ID)
			if err != nil || retained.Monitored || retained.Title != "Manual title" || retained.QualityProfile != "custom" {
				t.Fatal(retained, err)
			}
		})
	}
}
func TestAuthorChronologyRejectsImpreciseOrUnknownDates(t *testing.T) {
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	for _, dates := range [][]string{{"", ""}, {"2020", "2020-06-01"}, {"2020-06-01", "2020-06-01"}, {"2020-06", "2020-06-30"}} {
		candidates := []metadata.SearchResult{}
		for i, date := range dates {
			candidates = append(candidates, bibliographyCandidate(i+1, date, "Author"))
		}
		for _, policy := range []string{"first", "latest"} {
			sub := policyTestSubscription(policy)
			pc := buildAuthorPolicyContext(sub, candidates, nil, now)
			for _, candidate := range candidates {
				if allowed, reason := authorResultAllowedByPolicy(sub, candidate, pc); allowed || reason == "" {
					t.Fatal(policy, dates, allowed, reason)
				}
			}
		}
	}
	sub := policyTestSubscription("future")
	for _, date := range []string{"2026", "2026-05"} {
		candidate := bibliographyCandidate(1, date, "Author")
		if allowed, _ := authorResultIsFuture(sub, candidate, now); allowed {
			t.Fatal("coarse date crossed cutoff", date)
		}
	}
	if allowed, reason := authorResultIsFuture(sub, bibliographyCandidate(1, "2026-07", "Author"), now); !allowed {
		t.Fatal(reason)
	}
	original := bibliographyCandidate(1, "", "Author")
	original.Work.FirstPublishYear = 1990
	original.Edition.PublishedDate = "2099-01-01"
	if allowed, _ := authorResultIsFuture(sub, original, now); allowed {
		t.Fatal("reprint date overrode original work year")
	}
}
func TestAuthorFilterLanguageNamesUseCanonicalEquality(t *testing.T) {
	for _, pair := range [][2]string{{"fr", "French"}, {"fra", "French"}, {"deu", "German"}, {"en", "English"}} {
		if reason := languageFilterReason([]string{pair[0]}, pair[1]); reason != "" {
			t.Fatal(pair, reason)
		}
	}
	for _, pair := range [][2]string{{"e", "English"}, {"en", "English-based Creole"}, {"French", "franglish"}} {
		if reason := languageFilterReason([]string{pair[0]}, pair[1]); reason == "" {
			t.Fatal("prefix admitted different language", pair)
		}
	}
}
func TestIgnoredReviewMatchesWorkAliasesButKeepsFormatScope(t *testing.T) {
	result := bibliographyCandidate(1, "2000", "Author")
	sub := AuthorSubscription{ID: "sub", Format: "ebook"}
	rules := authorExclusions{ignored: map[string]map[string]bool{reviewScope(sub.ID, sub.Format): {exclusionWorkKeys(result)[0]: true}}}
	changed := result
	changed.Work.ID = "other:1"
	changed.Work.ProviderIDs = []string{"hardcover:1"}
	changed.Edition.ID = "hardcover-edition:2"
	if reason := rules.reason(sub, changed); reason == "" {
		t.Fatal("lost work alias", fmt.Sprint(changed))
	}
	sub.Format = "audiobook"
	if reason := rules.reason(sub, changed); reason != "" {
		t.Fatal("ebook ignore crossed format", reason)
	}
}

type authorFetchFunc func(metadata.Query) ([]metadata.SearchResult, error)

func (f authorFetchFunc) AuthorBibliography(_ context.Context, q metadata.Query) ([]metadata.SearchResult, error) {
	return f(q)
}
func TestAuthorMonitorReadsDecisionsAfterSlowProviderFetch(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: true, MissingBookPolicy: "all"})
	if err != nil {
		t.Fatal(err)
	}
	candidate := bibliographyCandidate(1, "2000-01-01", "Author")
	review, err := store.UpsertAuthorMetadataReview(ctx, authorMetadataReviewFromSkipped(sub, candidate, "fixture review"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := authorFetchFunc(func(metadata.Query) ([]metadata.SearchResult, error) {
		// Simulate the owner ignoring an existing review while a remote bibliography
		// request is in flight. An exclusion snapshot taken before IO is stale.
		if _, err := store.ResolveAuthorMetadataReview(ctx, review.ID, "ignored", "ignored", ""); err != nil {
			return nil, err
		}
		return []metadata.SearchResult{candidate}, nil
	})
	run, err := NewService(store, nil, fixture).MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
	if err != nil || run.ErrorCount != 0 || run.WantedCreated != 0 || run.Items[0].SkippedCount != 1 {
		t.Fatal(run, err)
	}
}

func TestAuthorMonitorReadsSettingsAfterSlowProviderFetch(t *testing.T) {
	for _, change := range []string{"unmonitor", "remove", "none", "future", "language", "defaults"} {
		t.Run(change, func(t *testing.T) {
			db := testdb.Open(t)
			ctx := context.Background()
			store := NewStore(db)
			sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: true, MissingBookPolicy: "all"})
			if err != nil {
				t.Fatal(err)
			}
			candidate := bibliographyCandidate(1, "2000-01-01", "Author")
			candidate.Edition.Language = "English"
			fixture := authorFetchFunc(func(metadata.Query) ([]metadata.SearchResult, error) {
				request := AuthorUpdateRequest{}
				switch change {
				case "remove":
					return []metadata.SearchResult{candidate}, store.DeleteAuthorSubscription(ctx, sub.ID)
				case "unmonitor":
					value := false
					request.Monitored = &value
				case "none", "future":
					request.MissingBookPolicy = change
				case "language":
					request.AllowedLanguages = []string{"French"}
					request.AllowedLanguagesSet = true
				case "defaults":
					request.QualityProfile = "custom"
					request.Tags = []string{"updated"}
					request.TagsSet = true
				}
				_, err := store.UpdateAuthorSubscription(ctx, sub.ID, request)
				return []metadata.SearchResult{candidate}, err
			})
			run, err := NewService(store, nil, fixture).MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
			if err != nil || run.ErrorCount != 0 || len(run.Items) != 1 {
				t.Fatal(run, err)
			}
			if change == "defaults" {
				if run.WantedCreated != 1 || len(run.Items[0].WantedItems) != 1 {
					t.Fatal(run)
				}
				item := run.Items[0].WantedItems[0]
				if item.QualityProfile != "custom" || len(item.Tags) != 1 || item.Tags[0] != "updated" {
					t.Fatal(item)
				}
			} else if run.WantedCreated != 0 || run.Items[0].SkippedCount != 1 {
				t.Fatal(run)
			}
			current, err := store.GetAuthorSubscription(ctx, sub.ID)
			if err != nil {
				t.Fatal(err)
			}
			if (change == "unmonitor" || change == "remove" || change == "none") && current.LastSyncAt != nil {
				t.Fatal("stopped refresh advanced sync timestamp", current)
			}
		})
	}
}

func TestAuthorSyncTimestampRequiresUnchangedActiveSettings(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	sub, err := store.UpsertAuthorSubscription(ctx, AuthorSubscription{Provider: "Hardcover", ProviderKey: "hardcover-author:7", AuthorName: "Fixture", Format: "ebook", Status: "monitored", MonitorNewItems: true, MissingBookPolicy: "all"})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := store.UpdateAuthorSubscription(ctx, sub.ID, AuthorUpdateRequest{MissingBookPolicy: "future"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkAuthorSubscriptionSynced(ctx, sub.ID, sub.UpdatedAt); err == nil {
		t.Fatal("stale refresh reported success")
	}
	current, err := store.GetAuthorSubscription(ctx, sub.ID)
	if err != nil || current.LastSyncAt != nil {
		t.Fatal(current, err)
	}
	if err := store.MarkAuthorSubscriptionSynced(ctx, sub.ID, changed.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAuthorSubscription(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	current, err = store.GetAuthorSubscription(ctx, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkAuthorSubscriptionSynced(ctx, current.ID, current.UpdatedAt); err == nil {
		t.Fatal("removed subscription reported sync success")
	}
}
