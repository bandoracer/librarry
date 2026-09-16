package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestAttentionCountsCompleteRecoveryAndDoNotReturnPrivateData(t *testing.T) {
	db := testdb.Open(t)
	for _, query := range []string{
		`insert into import_reviews(source_path) select '/private-fixture/'||i from generate_series(1,501)i`,
		`insert into import_reviews(source_path,status) values('/private-fixture/resolved','imported')`,
		`insert into import_operations(source_kind,request_key,source_root,destination_root,media_format,import_mode,state,cleanup_state) select 'manual',i::text,'/private-fixture/source','/private-fixture/library','ebook','copy',case when i<=200 then 'failed' else 'committed' end,case when i<=300 then 'blocked' else 'cleaned' end from generate_series(1,501)i`,
		`insert into root_folders(name,path,media_format) values('Fixture','/private-fixture/calibre','ebook')`,
		`insert into calibre_handoffs(source_path,root_folder_id,phase,plan) select '/private-fixture/calibre/'||i,(select id from root_folders limit 1),case when i<=201 then 'uploading' else 'committed' end,'{}' from generate_series(1,301)i`,
		`insert into files(media_format,path,metadata) select 'ebook','/private-fixture/legacy/'||i,'{"wantedId":"absent"}' from generate_series(1,501)i`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	router := NewRouter(Dependencies{Database: db, Config: config.Config{APIKey: "fixture-secret"}})
	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest("GET", "/api/v1/system/attention", nil))
	if unauthorized.Code != 401 {
		t.Fatal(unauthorized.Code)
	}
	read := func() attentionCounts {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/system/attention", nil)
		req.Header.Set("X-Api-Key", "fixture-secret")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		var counts attentionCounts
		if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &counts) != nil || strings.Contains(res.Body.String(), "private-fixture") || res.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(res.Code, res.Body.String())
		}
		return counts
	}
	counts := read()
	if counts.ImportReviews != 501 || counts.ImportOperations != 300 || counts.CalibreHandoffs != 201 || counts.LegacyLinks != 501 || counts.ObservedAt.IsZero() {
		t.Fatal(counts)
	}
	if _, err := db.Exec(`update import_reviews set status='imported';update import_operations set state='committed',cleanup_state='cleaned';update calibre_handoffs set phase='committed';update import_reconciliation_issues set resolved_at=now()`); err != nil {
		t.Fatal(err)
	}
	counts = read()
	if counts.ImportReviews+counts.ImportOperations+counts.CalibreHandoffs+counts.LegacyLinks != 0 {
		t.Fatal(counts)
	}
}

func TestAttentionCountsUnavailableIsNotAnEmptyQueue(t *testing.T) {
	for _, closed := range []bool{false, true} {
		deps := Dependencies{}
		if closed {
			deps.Database = testdb.Open(t)
			deps.Database.Close()
		}
		res := httptest.NewRecorder()
		NewRouter(deps).ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/system/attention", nil))
		if res.Code != 503 || strings.Contains(res.Body.String(), `"importReviews":0`) {
			t.Fatal(res.Code, res.Body.String())
		}
	}
}
