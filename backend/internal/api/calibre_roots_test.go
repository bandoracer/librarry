package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
)

func TestRootFolderEndpointsRedactCalibrePassword(t *testing.T) {
	libraryClient := &fakeRootFolderLibrary{
		folders: []library.RootFolder{{
			ID:          "root-1",
			Name:        "Calibre",
			Path:        "/library/calibre",
			MediaFormat: "ebook",
			Calibre: library.RootFolderCalibre{
				Enabled:  true,
				Host:     "calibre.local",
				Port:     8080,
				Username: "reader",
				Password: "super-secret",
				Library:  "Main",
			},
		}},
	}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Library:  libraryClient,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/library/root-folders", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "super-secret") {
		t.Fatalf("expected calibre password redacted on GET, got %s", res.Body.String())
	}
	for _, fragment := range []string{
		`"calibre":{`, `"enabled":true`, `"host":"calibre.local"`, `"port":8080`,
		`"username":"reader"`, `"password":""`, `"library":"Main"`,
	} {
		if !strings.Contains(res.Body.String(), fragment) {
			t.Fatalf("expected %s in root folders payload, got %s", fragment, res.Body.String())
		}
	}

	body := `{"name":"Calibre","path":"/library/calibre","mediaFormat":"ebook",` +
		`"calibre":{"enabled":true,"host":"calibre.local","port":8080,"username":"reader","password":"super-secret"}}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/library/root-folders", strings.NewReader(body))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "super-secret") {
		t.Fatalf("expected calibre password redacted on POST response, got %s", res.Body.String())
	}
	if len(libraryClient.created) != 1 || libraryClient.created[0].Calibre.Password != "super-secret" {
		t.Fatalf("expected service to receive the plaintext password, got %+v", libraryClient.created)
	}
}
