package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestImportRecoveryAPIShowsUnresolvedLinksAndValidatesRetries(t *testing.T) {
	db := testdb.Open(t)
	if _, err := db.Exec(`insert into files(media_format,path,metadata) values('ebook','/fixture/legacy.epub','{"wantedId":"missing-wanted","downloadId":"missing-download"}')`); err != nil {
		t.Fatal(err)
	}
	service := library.NewService(library.NewStore(db), library.Config{}, wanted.NewStore(db), nil)
	router := NewRouter(Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*"}, Library: service})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/library/import-recovery", nil))
	var report library.ImportRecoveryReport
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &report) != nil || report.Operations == nil || len(report.Issues) != 2 || report.Unresolved != 2 {
		t.Fatalf("report: %d %s", res.Code, res.Body.String())
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=oops", "unfinishedOnly=1", "operationsCursor=bad", "calibreCursor=bad", "issuesCursor=bad"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/library/import-recovery?"+query, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid query %s: %d %s", query, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/library/import-recovery?limit=1", nil))
	var first library.ImportRecoveryReport
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &first) != nil || len(first.Issues) != 1 || first.IssuesPage.NextCursor == "" || first.IssuesPage.Total != 2 {
		t.Fatalf("first page: %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/library/import-recovery?limit=1&issuesCursor="+first.IssuesPage.NextCursor, nil))
	var second library.ImportRecoveryReport
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &second) != nil || len(second.Issues) != 1 || second.IssuesPage.NextCursor != "" || second.Issues[0].Kind == first.Issues[0].Kind {
		t.Fatalf("second page: %d %s", response.Code, response.Body.String())
	}
	for id, status := range map[string]int{"invalid": http.StatusBadRequest, "00000000-0000-0000-0000-000000000001": http.StatusNotFound} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/library/import-operations/"+id+"/retry", nil))
		if res.Code != status {
			t.Fatalf("retry %s: %d %s", id, res.Code, res.Body.String())
		}
	}
}
