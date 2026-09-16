package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestImportReviewCollectionAPI(t *testing.T) {
	db := testdb.Open(t)
	if _, err := db.Exec(`insert into import_reviews(source_path,status,media_format,title) values('/fixture/1','imported','ebook','Title One'),('/fixture/2','skipped','audiobook','Title Two'),('/fixture/3','rejected','unknown','Title Three')`); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Config: config.Config{APIKey: "fixture-key"}, Library: library.NewService(library.NewStore(db), library.Config{}, nil, nil)})
	read := func(path, key string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-Api-Key", key)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	path := "/api/v1/library/import-reviews?view=collection&status=resolved&limit=1"
	if r := read(path, ""); r.Code != 401 {
		t.Fatal(r.Code)
	}
	r := read(path, "fixture-key")
	var page library.ImportReviewCollection
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &page) != nil || page.Total != 3 || page.Filtered != 3 || len(page.Reviews) != 1 || page.NextCursor == "" || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(r.Code, r.Body.String())
	}
	r = read(path+"&cursor="+page.NextCursor, "fixture-key")
	var next library.ImportReviewCollection
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &next) != nil || next.Reviews[0].ID == page.Reviews[0].ID {
		t.Fatal(r.Code, r.Body.String())
	}
	r = read(path+"&q=absent", "fixture-key")
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"reviews":[]`) {
		t.Fatal(r.Code, r.Body.String())
	}
	r = read("/api/v1/library/import-reviews?status=imported", "fixture-key")
	if r.Code != 200 || !strings.Contains(r.Body.String(), "Title One") {
		t.Fatal("legacy list changed", r.Code, r.Body.String())
	}
}

func TestImportReviewCollectionAPIInvalidAndUnavailable(t *testing.T) {
	router := NewRouter(Dependencies{Library: library.NewService(nil, library.Config{}, nil, nil)})
	for _, query := range []string{"limit=0", "limit=101", "limit=no", "status=imported", "kind=folder", "format=video", "cursor=bad", "status=all&status=pending", "unknown=1", "view=other"} {
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest("GET", "/api/v1/library/import-reviews?view=collection&"+query, nil))
		if r.Code != 400 {
			t.Fatal(query, r.Code, r.Body.String())
		}
	}
	for _, deps := range []Dependencies{{}, {Library: fakeLibrary{}}, {Library: library.NewService(nil, library.Config{}, nil, nil)}} {
		r := httptest.NewRecorder()
		NewRouter(deps).ServeHTTP(r, httptest.NewRequest("GET", "/api/v1/library/import-reviews?view=collection", nil))
		if r.Code != 503 || strings.Contains(r.Body.String(), `"reviews":[]`) {
			t.Fatal(r.Code, r.Body.String())
		}
	}
}
