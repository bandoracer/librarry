package wanted

import (
	"context"
	"database/sql"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func authorTestRoot(t *testing.T, db *sql.DB, path, format string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`insert into root_folders(name,path,media_format) values ($1,$1,$2) returning id::text`, path, format).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAuthorDestinationValidationAndPatchSemantics(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	service := NewService(store, nil, nil)
	ebook := authorTestRoot(t, db, "/ebooks", "ebook")
	audio := authorTestRoot(t, db, "/audio", "audiobook")
	req := AuthorSubscribeRequest{AuthorName: "Fixture", Provider: "Hardcover", ProviderKey: "hardcover-author:7", Format: "ebook", RootFolderID: ebook, QualityProfile: "premium"}
	for _, badRoot := range []string{audio, "not-an-id", "00000000-0000-0000-0000-000000000001"} {
		bad := req
		bad.RootFolderID = badRoot
		if _, err := service.SubscribeAuthor(ctx, bad); err == nil {
			t.Fatal("accepted invalid author destination", badRoot)
		}
	}
	sub, err := service.SubscribeAuthor(ctx, req)
	if err != nil || sub.RootFolderID != ebook {
		t.Fatal(sub, err)
	}
	for _, badRoot := range []string{audio, "not-an-id"} {
		if _, err := service.UpdateAuthorSubscription(ctx, sub.ID, AuthorUpdateRequest{RootFolderID: &badRoot, QualityProfile: "unexpected"}); err == nil {
			t.Fatal("accepted incompatible root update")
		}
		current, err := store.GetAuthorSubscription(ctx, sub.ID)
		if err != nil || current.RootFolderID != ebook || current.QualityProfile != "premium" {
			t.Fatal("failed update changed subscription", current, err)
		}
	}
	updated, err := service.UpdateAuthorSubscription(ctx, sub.ID, AuthorUpdateRequest{MissingBookPolicy: "latest"})
	if err != nil || updated.RootFolderID != ebook {
		t.Fatal("omitted root must be preserved", updated, err)
	}
	blank := ""
	updated, err = service.UpdateAuthorSubscription(ctx, sub.ID, AuthorUpdateRequest{RootFolderID: &blank})
	if err != nil || updated.RootFolderID != "" {
		t.Fatal("empty root must restore format default", updated, err)
	}
}

func TestAuthorNewBooksInheritDestinationProfileAndTagsWithoutChangingExistingBooks(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	rootA := authorTestRoot(t, db, "/books/a", "ebook")
	rootB := authorTestRoot(t, db, "/books/b", "ebook")
	fixture := &bibliographyFixture{rows: []metadata.SearchResult{bibliographyCandidate(1, "2000", "Author"), bibliographyCandidate(2, "2010", "Author")}}
	service := NewService(store, nil, fixture)
	sub, err := service.SubscribeAuthor(ctx, AuthorSubscribeRequest{AuthorName: "Fixture", Provider: "Hardcover", ProviderKey: "hardcover-author:7", Format: "ebook", RootFolderID: rootA, QualityProfile: "premium", Tags: []string{"author-tag"}})
	if err != nil {
		t.Fatal(err)
	}
	existing, err := store.CreateWanted(ctx, CreateRequest{Result: fixture.rows[0], Format: "ebook", RootFolderID: rootB, QualityProfile: "manual", Tags: []string{"manual-tag"}})
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
	if err != nil || run.ErrorCount != 0 || run.WantedCreated != 1 {
		t.Fatal(run, err)
	}
	item := run.Items[0].WantedItems[0]
	if item.RootFolderID != rootA || item.QualityProfile != "premium" || len(item.Tags) != 1 || item.Tags[0] != "author-tag" {
		t.Fatal(item)
	}
	if _, err := service.UpdateAuthorSubscription(ctx, sub.ID, AuthorUpdateRequest{RootFolderID: &rootB, QualityProfile: "new-profile"}); err != nil {
		t.Fatal(err)
	}
	fixture.rows = append(fixture.rows, bibliographyCandidate(3, "2020", "Author"))
	run, err = service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
	if err != nil || run.ErrorCount != 0 || run.WantedCreated != 1 || run.Items[0].WantedItems[0].RootFolderID != rootB || run.Items[0].WantedItems[0].QualityProfile != "new-profile" {
		t.Fatal(run, err)
	}
	retained, err := store.GetWanted(ctx, existing.ID)
	if err != nil || retained.RootFolderID != rootB || retained.QualityProfile != "manual" || len(retained.Tags) != 1 || retained.Tags[0] != "manual-tag" {
		t.Fatal(retained, err)
	}
	retained, err = store.GetWanted(ctx, item.ID)
	if err != nil || retained.RootFolderID != rootA || retained.QualityProfile != "premium" {
		t.Fatal("author update moved a previously created book", retained, err)
	}
}

func TestAuthorReviewRetainsEvaluatedDestination(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	store := NewStore(db)
	rootA := authorTestRoot(t, db, "/books/a", "ebook")
	rootB := authorTestRoot(t, db, "/books/b", "ebook")
	fixture := &bibliographyFixture{rows: []metadata.SearchResult{bibliographyCandidate(1, "2000", "Author")}}
	service := NewService(store, nil, fixture)
	sub, err := service.SubscribeAuthor(ctx, AuthorSubscribeRequest{AuthorName: "Fixture", Provider: "Hardcover", ProviderKey: "hardcover-author:7", Format: "ebook", RootFolderID: rootA, QualityProfile: "premium", MissingBookPolicy: "future"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.MonitorAuthors(ctx, AuthorMonitorRequest{Force: true})
	if err != nil || run.ErrorCount != 0 || len(run.Items) != 1 || len(run.Items[0].SkippedItems) != 1 {
		t.Fatal(run, err)
	}
	reviews, err := service.ListAuthorMetadataReviews(ctx, AuthorMetadataReviewQuery{})
	if err != nil || len(reviews) != 1 || reviews[0].RootFolderID != rootA {
		t.Fatal(reviews, err)
	}
	if _, err := service.UpdateAuthorSubscription(ctx, sub.ID, AuthorUpdateRequest{RootFolderID: &rootB}); err != nil {
		t.Fatal(err)
	}
	decision, err := service.ResolveAuthorMetadataReview(ctx, reviews[0].ID, AuthorMetadataReviewDecisionRequest{Action: "wanted"})
	if err != nil || decision.WantedItem == nil || decision.WantedItem.RootFolderID != rootA || decision.WantedItem.QualityProfile != "premium" {
		t.Fatal(decision, err)
	}
}
