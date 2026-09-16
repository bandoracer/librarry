package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestScanMoveHistoryRoute(t *testing.T) {
	db := testdb.Open(t)
	s := library.NewService(library.NewStore(db), library.Config{}, nil, nil)
	router := NewRouter(Dependencies{Library: s})
	var id string
	if err := db.QueryRow(`insert into library_scan_jobs(scope_key,media_format,roots,state,phase) values('fixture','any','[]','completed','complete') returning id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/api/v1/library/scans/" + id + "/moves", 200},
		{"GET", "/api/v1/library/scans/" + id + "/moves?cursor=invalid", 400},
		{"GET", "/api/v1/library/scans/not-uuid/moves", 400},
		{"GET", "/api/v1/library/scans/00000000-0000-0000-0000-000000000000/moves", 404},
		{"POST", "/api/v1/library/scans/" + id + "/moves", 405},
	} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != tc.status {
			t.Fatal(tc, res.Code, res.Body.String())
		}
		if tc.status == 200 && !strings.Contains(res.Body.String(), `"moves":[]`) {
			t.Fatal(res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	NewRouter(Dependencies{}).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/library/scans/"+id+"/moves", nil))
	if res.Code != 503 {
		t.Fatal(res.Code, res.Body.String())
	}
}
