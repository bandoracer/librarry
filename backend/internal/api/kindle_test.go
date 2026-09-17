package api

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/kindle"
	"github.com/bandoracer/librarry/backend/internal/testdb"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKindleUnavailable(t *testing.T) {
	h := &handler{}
	w := httptest.NewRecorder()
	h.kindleSettings(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
func TestKindleSettingsAPIAndAuth(t *testing.T) {
	db := testdb.Open(t)
	s := kindle.New(db, kindle.Settings{Port: 465, TLSMode: "implicit", Password: "fixture-super-secret"}, t.TempDir())
	router := NewRouter(Dependencies{Logger: slog.Default(), Config: config.Config{APIKey: "fixture-api-key", WebOrigin: "*"}, Kindle: s})
	request := func(method, path, body string, authorized bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if authorized {
			r.Header.Set("X-Api-Key", "fixture-api-key")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/v1/kindle/settings", "/api/v1/kindle/deliveries"} {
		w := request("GET", path, "", false)
		if w.Code != 401 {
			t.Fatal("unprotected route", w.Code)
		}
	}
	plainRequest := httptest.NewRequest("POST", "/api/v1/kindle/test", strings.NewReader(`{"requestId":"plain-text-123456789"}`))
	plainRequest.Header.Set("Content-Type", "text/plain")
	plainRequest.Header.Set("X-Api-Key", "fixture-api-key")
	plainResponse := httptest.NewRecorder()
	router.ServeHTTP(plainResponse, plainRequest)
	if plainResponse.Code != 415 {
		t.Fatal("non-JSON send accepted", plainResponse.Code)
	}
	w := request("GET", "/api/v1/kindle/settings", "", true)
	if w.Code != 200 || strings.Contains(w.Body.String(), "fixture-super-secret") || !strings.Contains(w.Body.String(), `"passwordConfigured":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("GET", "/api/v1/kindle/deliveries", "", true)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", "/api/v1/kindle/test", `{"requestId":"test-request-123456"}`, true)
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, body := range []string{`{"requestId":"test-request-123456","recipient":"injected@example.org"}`, `{"requestId":"test-request-123456"} {}`, strings.Repeat("x", 17000)} {
		w = request("POST", "/api/v1/kindle/test", body, true)
		if w.Code != 400 {
			t.Fatal("invalid body accepted", w.Code)
		}
	}
	v := kindle.Settings{Port: 465, TLSMode: "implicit", Host: "smtp.example.org", Username: "sender", From: "books@example.org", Recipient: "reader@kindle.com", Enabled: true}
	body, _ := json.Marshal(v)
	w = request("PUT", "/api/v1/kindle/settings", string(body), true)
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte("fixture-super-secret")) {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, e := s.Settings(context.Background())
	if e != nil || saved.Password != "fixture-super-secret" {
		t.Fatal("blank password not retained", e)
	}
}
