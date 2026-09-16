package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestCompatibilityBooksReachWholeLibraryAndValidateTargets(t *testing.T) {
	db := testdb.Open(t)
	if _, err := db.Exec(`insert into wanted_items(id,wanted_format,title,status,monitored,created_at)
	 select md5(n::text)::uuid,'ebook','Same title',case when n=10002 then 'removed' when n=10003 then 'ignored' else 'imported' end,true,'2000-01-01'::timestamptz+n*interval '1 second' from generate_series(1,10003)n`); err != nil {
		t.Fatal(err)
	}
	var oldID, missingID, inactiveID string
	if err := db.QueryRow(`select md5('1')::uuid::text,md5('2')::uuid::text,md5('10002')::uuid::text`).Scan(&oldID, &missingID, &inactiveID); err != nil {
		t.Fatal(err)
	}
	testdb.SeedRange(t, db, 1201, `insert into files(media_format,path,size_bytes,presence_state,import_status) select 'ebook','/fixture/unrelated-'||n||'.epub',10,'present','imported' from generate_series($1::integer,$2::integer)n`)
	if _, err := db.Exec(`insert into files(media_format,path,size_bytes,presence_state,import_status,metadata,updated_at) values('ebook','/fixture/old.epub',10,'present','imported',jsonb_build_object('wantedId',$1::text),'1999-01-01')`, oldID); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(Dependencies{Config: config.Config{APIKey: "fixture-key"}, Wanted: wanted.NewService(wanted.NewStore(db), nil)})
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-Api-Key", "fixture-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	start := time.Now()
	r := call("GET", "/api/v1/book", "")
	var books []map[string]any
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &books) != nil || len(books) != 10001 {
		t.Fatalf("complete books: %d, %d", r.Code, len(books))
	}
	t.Logf("10001 book resources: %s", time.Since(start))
	r = call("GET", "/api/v1/book/"+strconv.Itoa(stableInt(oldID)), "")
	var record map[string]any
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &record) != nil || record["librarryId"] != oldID || record["hasFile"] != true {
		t.Fatal(r.Code, r.Body.String())
	}
	seen := map[string]bool{}
	for page := 1; page <= 10; page++ {
		r = call("GET", fmt.Sprintf("/api/v1/wanted/missing?page=%d&pageSize=1000", page), "")
		var result struct {
			Total   int `json:"totalRecords"`
			Records []struct {
				ID string `json:"librarryId"`
			} `json:"records"`
		}
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &result) != nil || result.Total != 10000 || len(result.Records) != 1000 {
			t.Fatal(page, r.Code, result.Total, len(result.Records))
		}
		for _, item := range result.Records {
			if seen[item.ID] || item.ID == oldID {
				t.Fatal("duplicate/present book in missing", item.ID)
			}
			seen[item.ID] = true
		}
	}
	if len(seen) != 10000 || !seen[missingID] {
		t.Fatal(len(seen))
	}
	for _, direction := range []string{"ascending", "descending"} {
		r = call("GET", "/api/v1/wanted/missing?sortKey=id&sortDirection="+direction+"&pageSize=1", "")
		var result struct {
			Records []struct {
				ID int `json:"id"`
			} `json:"records"`
		}
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &result) != nil || len(result.Records) != 1 {
			t.Fatal(r.Code, r.Body.String())
		}
		expected := -1
		for id := range seen {
			n := stableInt(id)
			if expected < 0 || (direction == "ascending" && n < expected) || (direction == "descending" && n > expected) {
				expected = n
			}
		}
		if result.Records[0].ID != expected {
			t.Fatal(direction, result.Records[0].ID, expected)
		}
	}
	if _, err := db.Exec(`update wanted_items set release_date='2050-01-02' where id=$1`, missingID); err != nil {
		t.Fatal(err)
	}
	r = call("GET", "/api/v1/wanted/missing?sortKey=releaseDate&sortDirection=descending&pageSize=1", "")
	var dated struct {
		Records []struct {
			ID   string `json:"librarryId"`
			Date string `json:"releaseDate"`
		} `json:"records"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &dated) != nil || len(dated.Records) != 1 || dated.Records[0].ID != missingID || dated.Records[0].Date != "2050-01-02" {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, query := range []string{"page=", "page=1&page=2", "pageSize=2&pageSize=3", "page=-1", "page=bad", "page=99999999999999999999", "pageSize=1001", "sortDirection=wrong", "sortKey=unknown"} {
		r = call("GET", "/api/v1/wanted/missing?"+query, "")
		if r.Code != 400 {
			t.Fatal(query, r.Code)
		}
	}
	r = call("GET", "/api/v1/wanted/missing/"+strconv.Itoa(stableInt(missingID)), "")
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, ids := range [][]string{{missingID, "absent"}, {missingID, inactiveID}, {"Same title"}, {strconv.Itoa(stableInt("Same title"))}} {
		body, _ := json.Marshal(map[string]any{"bookIds": ids, "monitored": false})
		r = call("PUT", "/api/v1/book/monitor", string(body))
		if r.Code != 404 {
			t.Fatal(ids, r.Code, r.Body.String())
		}
		var monitored bool
		if err := db.QueryRow(`select monitored from wanted_items where id=$1`, missingID).Scan(&monitored); err != nil || !monitored {
			t.Fatal("partial monitor", err)
		}
	}
	if _, err := db.Exec(`update wanted_items set metadata_provider='fixture-'||id,source_key='shared-source' where id in ($1,$2)`, oldID, missingID); err != nil {
		t.Fatal(err)
	}
	r = call("GET", "/api/v1/book/shared-source", "")
	if r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	r = call("PUT", "/api/v1/book/monitor", fmt.Sprintf(`{"bookIds":[%d],"monitored":false}`, stableInt(missingID)))
	if r.Code != 202 {
		t.Fatal(r.Code, r.Body.String())
	}
	var monitored bool
	if err := db.QueryRow(`select monitored from wanted_items where id=$1`, missingID).Scan(&monitored); err != nil || monitored {
		t.Fatal(monitored, err)
	}
	// Both edit and delete must validate the entire set before changing the first book.
	for _, method := range []string{"PUT", "DELETE"} {
		r = call(method, "/api/v1/book/editor", fmt.Sprintf(`{"bookIds":[%q,"absent"],"title":"Do not save"}`, oldID))
		if r.Code != 404 {
			t.Fatal(method, r.Code, r.Body.String())
		}
		var title, status string
		if err := db.QueryRow(`select title,status from wanted_items where id=$1`, oldID).Scan(&title, &status); err != nil || title != "Same title" || status != "imported" {
			t.Fatal(title, status, err)
		}
	}
	r = call("PUT", "/api/v1/book/editor", fmt.Sprintf(`{"bookIds":[%q,%q],"title":"Reviewed title"}`, oldID, missingID))
	if r.Code != 202 {
		t.Fatal(r.Code, r.Body.String())
	}
	var edited int
	if err := db.QueryRow(`select count(*) from wanted_items where id in ($1,$2) and title='Reviewed title'`, oldID, missingID).Scan(&edited); err != nil || edited != 2 {
		t.Fatal(edited, err)
	}
	r = call("DELETE", "/api/v1/book/editor", fmt.Sprintf(`{"bookIds":[%q,%q]}`, oldID, missingID))
	if r.Code != 204 {
		t.Fatal(r.Code, r.Body.String())
	}
	if err := db.QueryRow(`select count(*) from wanted_items where id in ($1,$2) and status='removed' and not monitored`, oldID, missingID).Scan(&edited); err != nil || edited != 2 {
		t.Fatal(edited, err)
	}
	db.Close()
	for _, path := range []string{"/api/v1/book", "/api/v1/book/" + oldID, "/api/v1/wanted/missing", "/api/v1/wanted/cutoff"} {
		r = call("GET", path, "")
		if r.Code != 503 {
			t.Fatal(path, r.Code, r.Body.String())
		}
	}
}

func TestCompatibilityNumericCollisionDoesNotChooseABook(t *testing.T) {
	byHash := map[int]string{}
	var a, b string
	for i := 0; i < 500000; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012x", i)
		hash := stableInt(id)
		if prev, ok := byHash[hash]; ok {
			a, b = prev, id
			break
		}
		byHash[hash] = id
	}
	if a == "" {
		t.Fatal("fixture requires a real numeric collision")
	}
	items := []wanted.WantedItem{{ID: a, Status: "wanted"}, {ID: b, Status: "wanted"}}
	if _, err := resolveCompatBooks(items, []string{strconv.Itoa(stableInt(a))}); !errors.Is(err, errCompatBookAmbiguous) {
		t.Fatal(err)
	}
	resolved, err := resolveCompatBooks(items, []string{a})
	if err != nil || len(resolved) != 1 || resolved[0].ID != a {
		t.Fatal(resolved, err)
	}
}

type blockedCompatAcquire struct {
	fakeAcquire
	calls int
}

func (f *blockedCompatAcquire) Search(context.Context, acquisition.ReleaseSearchQuery) ([]acquisition.Release, error) {
	f.calls++
	return nil, errors.New("unexpected search")
}
func (f *blockedCompatAcquire) Grab(context.Context, acquisition.DownloadRequest) (acquisition.DownloadStatus, error) {
	f.calls++
	return acquisition.DownloadStatus{}, errors.New("unexpected grab")
}
func TestCompatibilityInvalidReleaseIdentityDoesNotAct(t *testing.T) {
	acquire := &blockedCompatAcquire{}
	h := NewRouter(Dependencies{Config: config.Config{APIKey: "fixture-key"}, Wanted: fakeWanted{}, Acquire: acquire})
	for _, check := range []struct{ method, path, body string }{
		{"GET", "/api/v1/release?term=Fixture&bookId=absent", ""},
		{"GET", "/api/v1/release?term=Fixture&wantedId=wanted-1&bookId=absent", ""},
		{"POST", "/api/v1/release", `{"bookId":"absent","downloadUrl":"https://example.invalid/fixture.torrent"}`},
		{"POST", "/api/v1/release", `{"wantedId":"wanted-1","bookId":"absent","downloadUrl":"https://example.invalid/fixture.torrent"}`},
	} {
		r := httptest.NewRequest(check.method, check.path, strings.NewReader(check.body))
		r.Header.Set("X-Api-Key", "fixture-key")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 404 || acquire.calls != 0 {
			t.Fatal(check, w.Code, w.Body.String(), acquire.calls)
		}
	}
}
