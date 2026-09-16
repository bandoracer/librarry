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
	for id, status := range map[string]int{"invalid": http.StatusBadRequest, "00000000-0000-0000-0000-000000000001": http.StatusNotFound} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/library/import-operations/"+id+"/retry", nil))
		if res.Code != status {
			t.Fatalf("retry %s: %d %s", id, res.Code, res.Body.String())
		}
	}
}
