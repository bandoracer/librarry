package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestBookChoicesAPI(t *testing.T) {
	db := testdb.Open(t)
	var id string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title,author_name) values('ebook','Saved book','Author') returning id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Config: config.Config{APIKey: "fixture-key"}, Wanted: wanted.NewService(wanted.NewStore(db), nil)})
	read := func(query, key string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/library/book-choices?"+query, nil)
		req.Header.Set("X-Api-Key", key)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	if r := read("", ""); r.Code != 401 {
		t.Fatal(r.Code)
	}
	r := read("q=no-match&format=ebook&limit=1&selectedId="+id, "fixture-key")
	var page wanted.BookChoices
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &page) != nil || page.Total != 1 || page.Filtered != 0 || page.Selected == nil || page.Selected.ID != id || page.Books == nil || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, q := range []string{"limit=0", "limit=101", "limit=bad", "format=video", "selectedId=bad", "cursor=bad", "q=one&q=two", "unknown=1"} {
		r = read(q, "fixture-key")
		if r.Code != 400 {
			t.Fatal(q, r.Code, r.Body.String())
		}
	}
	db.Close()
	r = read("", "fixture-key")
	if r.Code != 503 || strings.Contains(r.Body.String(), `"books":[]`) {
		t.Fatal(r.Code, r.Body.String())
	}
}
