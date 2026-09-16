package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestLibraryRepairPreviewRouteIsReadOnlyAndValidatesCursor(t *testing.T) {
	db := testdb.Open(t)
	s := library.NewService(library.NewStore(db), library.Config{}, nil, nil)
	router := NewRouter(Dependencies{Library: s})
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/api/v1/library/repair-preview", 200},
		{"GET", "/api/v1/library/repair-preview?cursor=invalid", 400},
		{"POST", "/api/v1/library/repair-preview", 405},
	} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != tc.status {
			t.Fatal(tc, res.Code, res.Body.String())
		}
		if tc.status == 200 && (!strings.Contains(res.Body.String(), `"findings":[]`) || !strings.Contains(res.Body.String(), `"readOnly":true`)) {
			t.Fatal(res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	NewRouter(Dependencies{}).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/library/repair-preview", nil))
	if res.Code != 503 {
		t.Fatal(res.Code, res.Body.String())
	}
}
