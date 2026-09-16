package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type authorCollectionAPIFixture struct {
	fakeWanted
	query wanted.AuthorCollectionQuery
	err   error
}

func (f *authorCollectionAPIFixture) AuthorCollection(_ context.Context, q wanted.AuthorCollectionQuery) (wanted.AuthorCollection, error) {
	f.query = q
	return wanted.AuthorCollection{Total: 501, Filtered: 201, NextCursor: "next"}, f.err
}
func TestAuthorCollectionAPIQueryAndEmptySerialization(t *testing.T) {
	fixture := &authorCollectionAPIFixture{}
	router := NewRouter(Dependencies{Wanted: fixture})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/library/authors?q=older&format=ebook&status=monitored&limit=25&cursor=opaque", nil))
	if res.Code != 200 || fixture.query.Search != "older" || fixture.query.Format != "ebook" || fixture.query.Status != "monitored" || fixture.query.Limit != 25 || fixture.query.Cursor != "opaque" || !strings.Contains(res.Body.String(), `"authors":[]`) || !strings.Contains(res.Body.String(), `"filtered":201`) {
		t.Fatal(res.Code, res.Body.String(), fixture.query)
	}
}
func TestAuthorCollectionAPIRejectsInvalidQuery(t *testing.T) {
	router := NewRouter(Dependencies{Wanted: wanted.NewService(nil, nil)})
	for _, query := range []string{"limit=0", "limit=101", "limit=abc", "sort=random", "format=video", "status=removed", "state=grabbed", "cursor=bad", "format=ebook&format=audiobook", "unknown=1"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/library/authors?"+query, nil))
		if res.Code != http.StatusBadRequest {
			t.Fatal(query, res.Code, res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/library/authors", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatal(res.Code, res.Body.String())
	}
}
