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

func TestBookMatchesAPI(t *testing.T) {
	db := testdb.Open(t)
	if _, err := db.Exec(`insert into wanted_items(wanted_format,title,metadata_provider,source_key,status) values('ebook','Saved','Hardcover','hardcover:1','removed')`); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Config: config.Config{APIKey: "fixture-key"}, Wanted: wanted.NewService(wanted.NewStore(db), nil)})
	post := func(path, body, key string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("X-Api-Key", key)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	path := "/api/v1/library/book-matches"
	body := `{"candidates":[{"key":"one","provider":"Hardcover","workIds":["hardcover:1"],"format":"ebook"},{"key":"missing","format":"ebook"}]}`
	if r := post(path, body, ""); r.Code != 401 {
		t.Fatal(r.Code)
	}
	r := post(path, body, "fixture-key")
	var result wanted.BookMatches
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &result) != nil || len(result.Matches) != 2 || result.Matches[0].Total != 1 || result.Matches[0].Books[0].Status != "removed" || result.Matches[1].Books == nil || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(r.Code, r.Body.String())
	}
	r = post(path, `{"candidates":[]}`, "fixture-key")
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"matches":[]`) {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, bad := range []string{"{}", "null", `{"candidates":null}`, body + " {}", `{"candidates":[],"unknown":true}`, `{"candidates":[{"key":"one","format":"bad"}]}`, `{"candidates":[{"key":"one","format":"ebook","title":"not an identity"}]}`, `{"candidates":[{"key":"one","format":"ebook"},{"key":"one","format":"ebook"}]}`, strings.Repeat(" ", 1<<20) + body} {
		r = post(path, bad, "fixture-key")
		if r.Code != 400 {
			t.Fatal(len(bad), r.Code, r.Body.String())
		}
	}
	r = post("/api/v1/wanted", `{"result":{"provider":"Hardcover","work":{"id":"hardcover:1","title":"Changed"}},"format":"ebook","preserveExisting":true}`, "fixture-key")
	if r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	var title, status string
	if err := db.QueryRow(`select title,status from wanted_items`).Scan(&title, &status); err != nil || title != "Saved" || status != "removed" {
		t.Fatal(title, status, err)
	}
	db.Close()
	r = post(path, body, "fixture-key")
	if r.Code != 503 || strings.Contains(r.Body.String(), `"matches":[]`) {
		t.Fatal(r.Code, r.Body.String())
	}
}
