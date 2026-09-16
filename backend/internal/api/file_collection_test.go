package api

import (
	"context"
	"github.com/bandoracer/librarry/backend/internal/library"
	"net/http/httptest"
	"strings"
	"testing"
)

type fileCollectionFixture struct {
	fakeLibrary
	query library.FileCollectionQuery
}

func (f *fileCollectionFixture) FileCollection(_ context.Context, q library.FileCollectionQuery) (library.FileCollection, error) {
	f.query = q
	return library.FileCollection{Total: 10001, Filtered: 1500, NextCursor: "next"}, nil
}
func TestFileCollectionAPI(t *testing.T) {
	f := &fileCollectionFixture{}
	router := NewRouter(Dependencies{Library: f})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/library/files/collection?q=chapter&wantedId=book&format=audiobook&presence=missing&sort=path&limit=50&cursor=next", nil))
	if res.Code != 200 || f.query.Search != "chapter" || f.query.WantedID != "book" || f.query.Format != "audiobook" || f.query.Presence != "missing" || f.query.Limit != 50 || f.query.Cursor != "next" || !strings.Contains(res.Body.String(), `"files":[]`) || !strings.Contains(res.Body.String(), `"total":10001`) {
		t.Fatal(res.Code, res.Body.String(), f.query)
	}
}
func TestFileCollectionAPIRejectsInvalidInput(t *testing.T) {
	router := NewRouter(Dependencies{Library: library.NewService(nil, library.Config{}, nil, nil)})
	for _, q := range []string{"limit=0", "limit=101", "limit=no", "sort=random", "wantedId=bad", "format=video", "presence=stale", "format=ebook&format=audiobook", "cursor=bad", "unknown=1"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/library/files/collection?"+q, nil))
		if res.Code != 400 {
			t.Fatal(q, res.Code, res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/library/files/collection", nil))
	if res.Code != 503 {
		t.Fatal(res.Code, res.Body.String())
	}
}
