package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type collectionAPIFixture struct {
	fakeWanted
	query wanted.BookCollectionQuery
	err   error
}

func (f *collectionAPIFixture) BookCollection(_ context.Context, q wanted.BookCollectionQuery) (wanted.BookCollection, error) {
	f.query = q
	return wanted.BookCollection{Counts: map[string]int{}, Total: 501, Filtered: 201, NextCursor: "next"}, f.err
}
func TestBookCollectionAPIQueryAndEmptySerialization(t *testing.T) {
	fixture := &collectionAPIFixture{}
	router := NewRouter(Dependencies{Wanted: fixture})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/library/books?q=older&format=ebook&monitor=monitored&state=missing&sort=title&limit=25&cursor=opaque", nil))
	if res.Code != 200 || fixture.query.Search != "older" || fixture.query.Format != "ebook" || fixture.query.Monitor != "monitored" || fixture.query.State != "missing" || fixture.query.Sort != "title" || fixture.query.Limit != 25 || fixture.query.Cursor != "opaque" || !strings.Contains(res.Body.String(), `"books":[]`) || !strings.Contains(res.Body.String(), `"filtered":201`) {
		t.Fatal(res.Code, res.Body.String(), fixture.query)
	}
}
func TestBookCollectionAPIRejectsInvalidQuery(t *testing.T) {
	router := NewRouter(Dependencies{Wanted: wanted.NewService(nil, nil)})
	for _, query := range []string{"limit=0", "limit=101", "limit=abc", "sort=random", "format=video", "monitor=yes", "state=grabbed", "cursor=bad", "format=ebook&format=audiobook", "unknown=1"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/library/books?"+query, nil))
		if res.Code != http.StatusBadRequest {
			t.Fatal(query, res.Code, res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/library/books", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatal(res.Code, res.Body.String())
	}
}
