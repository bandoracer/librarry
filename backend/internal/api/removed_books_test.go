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

func TestRemovedBooksAndRestoreAPI(t *testing.T) {
	db := testdb.Open(t)
	var id string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title,status,monitored) values('ebook','Saved','removed',false) returning id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Config: config.Config{APIKey: "fixture-key"}, Wanted: wanted.NewService(wanted.NewStore(db), nil)})
	read := func(method, path, body, key string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-Api-Key", key)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	path := "/api/v1/library/removed-books"
	for _, method := range []string{"GET", "POST"} {
		url := path
		if method == "POST" {
			url = "/api/v1/wanted/" + id + "/restore"
		}
		if r := read(method, url, "{}", ""); r.Code != 401 {
			t.Fatal(method, r.Code)
		}
	}
	r := read("GET", path+"?limit=1", "", "fixture-key")
	var page wanted.RemovedBooks
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &page) != nil || page.Total != 1 || len(page.Books) != 1 || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(r.Code, r.Body.String())
	}
	raw, _ := json.Marshal(wanted.RestoreBookRequest{UpdatedAt: page.Books[0].UpdatedAt})
	for _, body := range []string{"{}", "null", `{"updatedAt":"bad"}`, `{"unexpected":true}`, string(raw) + " {}"} {
		r = read("POST", "/api/v1/wanted/"+id+"/restore", body, "fixture-key")
		if r.Code != 400 {
			t.Fatal(body, r.Code, r.Body.String())
		}
	}
	r = read("POST", "/api/v1/wanted/"+id+"/restore", string(raw), "fixture-key")
	var book wanted.WantedItem
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &book) != nil || book.ID != id || book.Status != "wanted" || book.Monitored {
		t.Fatal(r.Code, r.Body.String())
	}
	r = read("POST", "/api/v1/wanted/"+id+"/restore", string(raw), "fixture-key")
	if r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	r = read("POST", "/api/v1/wanted/00000000-0000-0000-0000-000000000000/restore", string(raw), "fixture-key")
	if r.Code != 404 {
		t.Fatal(r.Code, r.Body.String())
	}
	r = read("GET", path, "", "fixture-key")
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"books":[]`) {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, q := range []string{"status=active", "format=video", "cursor=bad", "limit=0", "limit=101", "limit=bad", "q=one&q=two", "unknown=1"} {
		r = read("GET", path+"?"+q, "", "fixture-key")
		if r.Code != 400 {
			t.Fatal(q, r.Code, r.Body.String())
		}
	}
	db.Close()
	r = read("GET", path, "", "fixture-key")
	if r.Code != 503 {
		t.Fatal(r.Code, r.Body.String())
	}
}
