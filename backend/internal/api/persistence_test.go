package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/auth"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestFreshPostgresListAndAuthenticationContracts(t *testing.T) {
	db := testdb.Open(t)
	acquire := acquisition.NewService(acquisition.IntegrationConfig{DownloadStore: acquisition.NewSQLDownloadStore(db)})
	wantedStore := wanted.NewStore(db)
	service := auth.NewService(auth.NewStore(db), slog.Default())
	deps := Dependencies{Logger: slog.Default(), Config: config.Config{WebOrigin: "*"}, Metadata: metadata.NewService(nil), Acquire: acquire, Wanted: wanted.NewService(wantedStore, acquire), Library: library.NewService(library.NewStore(db), library.Config{}, wantedStore, nil), Compat: compatdata.NewStore(db), Auth: service}
	router := NewRouter(deps)
	for path, key := range map[string]string{"/api/v1/wanted?view=library": "wanted", "/api/v1/authors": "authors", "/api/v1/library/files": "files"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		var payload map[string]json.RawMessage
		if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &payload) != nil || string(payload[key]) != "[]" {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPut, "/api/v1/auth/config", strings.NewReader(`{"method":"forms","username":"fixture","password":"fixture-only-password"}`)))
	if res.Code != 200 || service.Method() != auth.MethodForms {
		t.Fatalf("save auth: %d %s", res.Code, res.Body.String())
	}
	saved, ok, err := deps.Compat.GetResource(context.Background(), "auth-config", 1)
	if err != nil || !ok || saved.Payload["method"] != "forms" {
		t.Fatalf("auth was not persisted: %+v %v", saved, err)
	}
	if got := requestStatus(t, router, httptest.NewRequest(http.MethodGet, "/api/v1/wanted", nil)); got != 401 {
		t.Fatalf("auth not enforced: %d", got)
	}
}

func TestStatusReportsStableBuildAndActiveAuthentication(t *testing.T) {
	service := auth.NewService(nil, slog.Default())
	service.SetMethod(auth.MethodForms)
	h := &handler{deps: Dependencies{SchemaMigration: "0029_calibre_roots.sql", Auth: service}}
	var previous string
	for i := 0; i < 2; i++ {
		res := httptest.NewRecorder()
		h.compatSystemStatus(res, httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil))
		var status map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status["authentication"] != "forms" || status["migrationVersion"] != float64(29) {
			t.Fatalf("false status: %+v", status)
		}
		timestamp := status["buildTime"].(string)
		if i > 0 && previous != timestamp {
			t.Fatal("build timestamp changes between requests")
		}
		previous = timestamp
	}
}

func TestDirectBookAndFileLookupBeyondCollectionCaps(t *testing.T) {
	db := testdb.Open(t)
	_, err := db.Exec(`insert into wanted_items(wanted_format,title,author_name,created_at) select 'ebook','Fixture '||n,'Author',now() + n * interval '1 second' from generate_series(1,10000) n`)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title,author_name,status,created_at) values('ebook','Old imported book','Fixture Author','imported','2000-01-01') returning id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`insert into files(media_format,path,title,metadata,updated_at) values('ebook','/fixture/target.epub','Old imported book',jsonb_build_object('wantedId',$1::text),'2000-01-01')`, id)
	if err != nil {
		t.Fatal(err)
	}
	testdb.SeedRange(t, db, 10000, `insert into files(media_format,path,title) select 'ebook','/fixture/other-'||n||'.epub','Other book' from generate_series($1::integer,$2::integer) n`)
	ws := wanted.NewStore(db)
	items, err := ws.ListWanted(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == id {
			t.Fatal("fixture must be beyond collection cap")
		}
	}
	router := NewRouter(Dependencies{Config: config.Config{WebOrigin: "*"}, Metadata: metadata.NewService(nil), Wanted: wanted.NewService(ws, nil), Library: library.NewService(library.NewStore(db), library.Config{}, ws, nil)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/wanted/"+id, nil))
	var item wanted.WantedItem
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &item) != nil || item.ID != id || item.Status != "imported" {
		t.Fatalf("detail: %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/library/files?wantedId="+id, nil))
	var files struct {
		Files []library.FileRecord `json:"files"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &files) != nil || len(files.Files) != 1 || files.Files[0].Path != "/fixture/target.epub" {
		t.Fatalf("file lookup: %d %s", response.Code, response.Body.String())
	}
	if _, err := db.Exec(`update wanted_items set status='removed' where id=$1`, id); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"removed", "ignored"} {
		if _, err := db.Exec(`update wanted_items set status=$1,monitored=false where id=$2`, status, id); err != nil {
			t.Fatal(err)
		}
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/wanted/"+id, nil))
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &item) != nil || item.Status != status || item.Monitored {
			t.Fatalf("inactive detail: %d %s", response.Code, response.Body.String())
		}
	}
	if got := requestStatus(t, router, httptest.NewRequest(http.MethodGet, "/api/v1/wanted/not-a-uuid", nil)); got != 400 {
		t.Fatalf("invalid id returned %d", got)
	}
	if got := requestStatus(t, router, httptest.NewRequest(http.MethodGet, "/api/v1/wanted/00000000-0000-0000-0000-000000000001", nil)); got != 404 {
		t.Fatalf("missing id returned %d", got)
	}
}
