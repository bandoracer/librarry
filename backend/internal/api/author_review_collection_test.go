package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type authorReviewCollectionAPIFixture struct {
	fakeWanted
	query wanted.AuthorMetadataReviewQuery
	err   error
}

func (f *authorReviewCollectionAPIFixture) AuthorReviewCollection(_ context.Context, q wanted.AuthorMetadataReviewQuery) (wanted.AuthorReviewCollection, error) {
	f.query = q
	return wanted.AuthorReviewCollection{Total: 501, Filtered: 201, NextCursor: "next"}, f.err
}
func TestAuthorReviewCollectionAPIQueryAndEmptySerialization(t *testing.T) {
	fixture := &authorReviewCollectionAPIFixture{}
	router := NewRouter(Dependencies{Wanted: fixture})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/authors/metadata/review?q=older&format=ebook&status=pending&limit=25&cursor=opaque", nil))
	if res.Code != 200 || fixture.query.Search != "older" || fixture.query.Format != "ebook" || fixture.query.Status != "pending" || fixture.query.Limit != 25 || fixture.query.Cursor != "opaque" || !strings.Contains(res.Body.String(), `"reviews":[]`) || !strings.Contains(res.Body.String(), `"filtered":201`) {
		t.Fatal(res.Code, res.Body.String(), fixture.query)
	}
}
func TestAuthorReviewCollectionAPIRejectsInvalidQuery(t *testing.T) {
	router := NewRouter(Dependencies{Wanted: wanted.NewService(nil, nil)})
	for _, query := range []string{"limit=0", "limit=101", "limit=abc", "sort=random", "format=video", "status=removed", "state=grabbed", "cursor=bad", "format=ebook&format=audiobook", "unknown=1"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/authors/metadata/review?"+query, nil))
		if res.Code != http.StatusBadRequest {
			t.Fatal(query, res.Code, res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/authors/metadata/review", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatal(res.Code, res.Body.String())
	}
}

func TestAuthorReviewDecisionRejectsMalformedJSON(t *testing.T) {
	router := NewRouter(Dependencies{Wanted: fakeWanted{}})
	for _, body := range []string{`{"action":"wanted","typo":true}`, `{"action":"wanted"} {}`, `{"action":"wanted"`, strings.Repeat(" ", 1<<20) + `{"action":"wanted"}`} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest("POST", "/api/v1/authors/metadata/review/fixture/resolve", strings.NewReader(body)))
		if res.Code != 400 {
			t.Fatal(res.Code, res.Body.String())
		}
	}
}
