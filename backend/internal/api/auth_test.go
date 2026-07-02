package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/auth"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/metadata"
)

type memoryAuthStore struct {
	user     *auth.User
	sessions map[string]auth.Session
}

func newMemoryAuthStore() *memoryAuthStore {
	return &memoryAuthStore{sessions: map[string]auth.Session{}}
}

func (m *memoryAuthStore) Configured() bool { return true }

func (m *memoryAuthStore) UpsertUser(_ context.Context, username string, passwordHash string) (auth.User, error) {
	m.user = &auth.User{ID: "user-1", Username: username, PasswordHash: passwordHash}
	return *m.user, nil
}

func (m *memoryAuthStore) GetUser(context.Context) (auth.User, bool, error) {
	if m.user == nil {
		return auth.User{}, false, nil
	}
	return *m.user, true, nil
}

func (m *memoryAuthStore) GetUserByUsername(_ context.Context, username string) (auth.User, bool, error) {
	if m.user == nil || !strings.EqualFold(m.user.Username, username) {
		return auth.User{}, false, nil
	}
	return *m.user, true, nil
}

func (m *memoryAuthStore) CreateSession(_ context.Context, session auth.Session) error {
	m.sessions[session.TokenHash] = session
	return nil
}

func (m *memoryAuthStore) GetSession(_ context.Context, tokenHash string) (auth.Session, bool, error) {
	session, ok := m.sessions[tokenHash]
	return session, ok, nil
}

func (m *memoryAuthStore) DeleteSession(_ context.Context, tokenHash string) error {
	delete(m.sessions, tokenHash)
	return nil
}

func (m *memoryAuthStore) DeleteExpiredSessions(_ context.Context, now time.Time) error {
	for hash, session := range m.sessions {
		if !session.ExpiresAt.After(now) {
			delete(m.sessions, hash)
		}
	}
	return nil
}

func newAuthTestRouter(t *testing.T, method string, apiKey string) (http.Handler, *auth.Service) {
	t.Helper()
	service := auth.NewService(newMemoryAuthStore(), slog.Default())
	service.SetMethod(method)
	if err := service.EnsureUser(context.Background(), "ryan", "correct horse"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	router := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*", APIKey: apiKey},
		Metadata: metadata.NewService(nil),
		Wanted:   fakeWanted{},
		Auth:     service,
	})
	return router, service
}

func requestStatus(t *testing.T, router http.Handler, req *http.Request) int {
	t.Helper()
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res.Code
}

func TestAuthMiddlewareFormsMatrix(t *testing.T) {
	router, _ := newAuthTestRouter(t, auth.MethodForms, "secret")

	// Open paths stay reachable without any credentials.
	for _, path := range []string{"/healthz", "/ping", "/api/v1/auth/status"} {
		if got := requestStatus(t, router, httptest.NewRequest(http.MethodGet, path, nil)); got != http.StatusOK {
			t.Fatalf("expected open path %s to return 200, got %d", path, got)
		}
	}

	// Protected API paths reject anonymous requests.
	if got := requestStatus(t, router, httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)); got != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", got)
	}

	// API keys always bypass method auth (compat clients).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	req.Header.Set("X-Api-Key", "secret")
	if got := requestStatus(t, router, req); got != http.StatusOK {
		t.Fatalf("expected 200 with api key, got %d", got)
	}

	// Login issues the session cookie; bad credentials are 401.
	badLogin := httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(`{"username":"ryan","password":"nope"}`))
	if got := requestStatus(t, router, badLogin); got != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad login, got %d", got)
	}
	login := httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(`{"username":"ryan","password":"correct horse","rememberMe":true}`))
	loginRes := httptest.NewRecorder()
	router.ServeHTTP(loginRes, login)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", loginRes.Code, loginRes.Body.String())
	}
	var cookie *http.Cookie
	for _, candidate := range loginRes.Result().Cookies() {
		if candidate.Name == auth.SessionCookieName {
			cookie = candidate
		}
	}
	if cookie == nil || cookie.Value == "" || !cookie.HttpOnly {
		t.Fatalf("expected HttpOnly session cookie, got %+v", cookie)
	}
	if cookie.MaxAge < int((29 * 24 * time.Hour).Seconds()) {
		t.Fatalf("expected rememberMe cookie to be ~30d, got %d", cookie.MaxAge)
	}

	// The session cookie authenticates API requests.
	authed := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	authed.AddCookie(cookie)
	if got := requestStatus(t, router, authed); got != http.StatusOK {
		t.Fatalf("expected 200 with session cookie, got %d", got)
	}

	// auth/status reports authenticated with the cookie.
	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	statusReq.AddCookie(cookie)
	statusRes := httptest.NewRecorder()
	router.ServeHTTP(statusRes, statusReq)
	var status map[string]any
	if err := json.NewDecoder(statusRes.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status["method"] != "forms" || status["authenticated"] != true || status["username"] != "ryan" {
		t.Fatalf("unexpected auth status: %+v", status)
	}

	// Logout invalidates the session.
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/logout", nil)
	logoutReq.AddCookie(cookie)
	logoutReq.Header.Set("X-Api-Key", "secret")
	if got := requestStatus(t, router, logoutReq); got != http.StatusOK {
		t.Fatalf("expected logout 200, got %d", got)
	}
	replay := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	replay.AddCookie(cookie)
	if got := requestStatus(t, router, replay); got != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", got)
	}
}

func TestAuthMiddlewareBasicMatrix(t *testing.T) {
	router, _ := newAuthTestRouter(t, auth.MethodBasic, "")

	anonymous := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, anonymous)
	if res.Code != http.StatusUnauthorized || !strings.Contains(res.Header().Get("WWW-Authenticate"), "Basic") {
		t.Fatalf("expected Basic challenge, got %d %q", res.Code, res.Header().Get("WWW-Authenticate"))
	}

	wrong := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	wrong.SetBasicAuth("ryan", "nope")
	if got := requestStatus(t, router, wrong); got != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad basic credentials, got %d", got)
	}

	good := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	good.SetBasicAuth("ryan", "correct horse")
	if got := requestStatus(t, router, good); got != http.StatusOK {
		t.Fatalf("expected 200 with basic credentials, got %d", got)
	}
}

func TestAuthMiddlewareNonePreservesAPIKeyContract(t *testing.T) {
	// Method none + API key keeps the pre-M6 behavior: key required on /api/*.
	router, _ := newAuthTestRouter(t, auth.MethodNone, "secret")
	if got := requestStatus(t, router, httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)); got != http.StatusUnauthorized {
		t.Fatalf("expected 401 without api key, got %d", got)
	}
	keyed := httptest.NewRequest(http.MethodGet, "/api/v1/system/status?apikey=secret", nil)
	if got := requestStatus(t, router, keyed); got != http.StatusOK {
		t.Fatalf("expected 200 with apikey query, got %d", got)
	}

	// Method none without a key stays fully open.
	open, _ := newAuthTestRouter(t, auth.MethodNone, "")
	if got := requestStatus(t, open, httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)); got != http.StatusOK {
		t.Fatalf("expected open install to return 200, got %d", got)
	}
}

func TestFeedAuthRequiresAPIKeyWhenConfigured(t *testing.T) {
	// API key set: the feed needs ?apikey=.
	router, _ := newAuthTestRouter(t, auth.MethodNone, "secret")
	if got := requestStatus(t, router, httptest.NewRequest(http.MethodGet, "/feed/v1/calendar.ics", nil)); got != http.StatusUnauthorized {
		t.Fatalf("expected 401 feed without apikey, got %d", got)
	}
	if got := requestStatus(t, router, httptest.NewRequest(http.MethodGet, "/feed/v1/calendar.ics?apikey=secret", nil)); got != http.StatusOK {
		t.Fatalf("expected 200 feed with apikey, got %d", got)
	}

	// Forms auth without an API key: the feed stays blocked (sessions do not
	// apply to calendar apps).
	formsRouter, _ := newAuthTestRouter(t, auth.MethodForms, "")
	if got := requestStatus(t, formsRouter, httptest.NewRequest(http.MethodGet, "/feed/v1/calendar.ics", nil)); got != http.StatusUnauthorized {
		t.Fatalf("expected 401 feed under forms auth without key, got %d", got)
	}

	// Fully open install: feed is open.
	open, _ := newAuthTestRouter(t, auth.MethodNone, "")
	if got := requestStatus(t, open, httptest.NewRequest(http.MethodGet, "/feed/v1/calendar.ics", nil)); got != http.StatusOK {
		t.Fatalf("expected 200 open feed, got %d", got)
	}
}

func TestAuthConfigEndpointSwitchesMethod(t *testing.T) {
	router, service := newAuthTestRouter(t, auth.MethodNone, "secret")
	req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/config", strings.NewReader(`{"method":"forms"}`))
	req.Header.Set("X-Api-Key", "secret")
	if got := requestStatus(t, router, req); got != http.StatusOK {
		t.Fatalf("expected auth config 200, got %d", got)
	}
	if service.Method() != auth.MethodForms {
		t.Fatalf("expected method forms, got %s", service.Method())
	}

	// Switching on auth without any user fails loudly.
	empty := auth.NewService(newMemoryAuthStore(), slog.Default())
	bare := NewRouter(Dependencies{
		Logger:   slog.Default(),
		Config:   config.Config{WebOrigin: "*"},
		Metadata: metadata.NewService(nil),
		Auth:     empty,
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/auth/config", strings.NewReader(`{"method":"forms"}`))
	if got := requestStatus(t, bare, req); got != http.StatusBadRequest {
		t.Fatalf("expected 400 enabling auth without user, got %d", got)
	}
}
