package auth

import (
	"context"
	"testing"
	"time"
)

type memoryStore struct {
	user     *User
	sessions map[string]Session
}

func newMemoryStore() *memoryStore {
	return &memoryStore{sessions: map[string]Session{}}
}

func (m *memoryStore) Configured() bool { return true }

func (m *memoryStore) UpsertUser(_ context.Context, username string, passwordHash string) (User, error) {
	if m.user == nil {
		m.user = &User{ID: "user-1", Username: username, PasswordHash: passwordHash}
	} else {
		m.user.Username = username
		m.user.PasswordHash = passwordHash
	}
	return *m.user, nil
}

func (m *memoryStore) GetUser(context.Context) (User, bool, error) {
	if m.user == nil {
		return User{}, false, nil
	}
	return *m.user, true, nil
}

func (m *memoryStore) GetUserByUsername(_ context.Context, username string) (User, bool, error) {
	if m.user == nil || m.user.Username != username {
		return User{}, false, nil
	}
	return *m.user, true, nil
}

func (m *memoryStore) CreateSession(_ context.Context, session Session) error {
	m.sessions[session.TokenHash] = session
	return nil
}

func (m *memoryStore) GetSession(_ context.Context, tokenHash string) (Session, bool, error) {
	session, ok := m.sessions[tokenHash]
	return session, ok, nil
}

func (m *memoryStore) DeleteSession(_ context.Context, tokenHash string) error {
	delete(m.sessions, tokenHash)
	return nil
}

func (m *memoryStore) DeleteExpiredSessions(_ context.Context, now time.Time) error {
	for hash, session := range m.sessions {
		if !session.ExpiresAt.After(now) {
			delete(m.sessions, hash)
		}
	}
	return nil
}

func newTestService(t *testing.T) (*Service, *memoryStore) {
	t.Helper()
	store := newMemoryStore()
	service := NewService(store, nil)
	if err := service.EnsureUser(context.Background(), "ryan", "correct horse"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return service, store
}

func TestLoginIssuesSessionAndValidates(t *testing.T) {
	service, store := newTestService(t)

	if _, err := service.Login(context.Background(), "ryan", "wrong", false); err != ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	result, err := service.Login(context.Background(), "ryan", "correct horse", true)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.Token == "" || !result.Remember {
		t.Fatalf("unexpected login result: %+v", result)
	}
	if _, ok := store.sessions[HashSessionToken(result.Token)]; !ok {
		t.Fatalf("expected session row keyed by token hash")
	}
	user, ok := service.ValidateSession(context.Background(), result.Token)
	if !ok || user.Username != "ryan" {
		t.Fatalf("expected valid session for ryan, got %+v ok=%v", user, ok)
	}
	// Remember-me sessions get the long TTL.
	session := store.sessions[HashSessionToken(result.Token)]
	if ttl := time.Until(session.ExpiresAt); ttl < RememberMeTTL-time.Hour {
		t.Fatalf("expected ~30d expiry, got %s", ttl)
	}

	service.Logout(context.Background(), result.Token)
	if _, ok := service.ValidateSession(context.Background(), result.Token); ok {
		t.Fatal("expected session to be gone after logout")
	}
}

func TestExpiredSessionsAreRejectedAndDeleted(t *testing.T) {
	service, store := newTestService(t)
	result, err := service.Login(context.Background(), "ryan", "correct horse", false)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	// Time travel past the plain-session TTL.
	service.now = func() time.Time { return time.Now().UTC().Add(SessionTTL + time.Minute) }
	if _, ok := service.ValidateSession(context.Background(), result.Token); ok {
		t.Fatal("expected expired session to be rejected")
	}
	if _, ok := store.sessions[HashSessionToken(result.Token)]; ok {
		t.Fatal("expected expired session row to be deleted")
	}
}

func TestEnsureUserKeepsPasswordOnRename(t *testing.T) {
	service, store := newTestService(t)
	originalHash := store.user.PasswordHash
	if err := service.EnsureUser(context.Background(), "admin", ""); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if store.user.Username != "admin" || store.user.PasswordHash != originalHash {
		t.Fatalf("expected rename to keep hash, got %+v", store.user)
	}
	if _, ok := service.VerifyPassword(context.Background(), "admin", "correct horse"); !ok {
		t.Fatal("expected old password to work after rename")
	}
}

func TestNormalizeMethod(t *testing.T) {
	cases := map[string]string{
		"none": MethodNone, "off": MethodNone, "basic": MethodBasic,
		"forms": MethodForms, "session": MethodForms, "bogus": "", "": "",
	}
	for input, want := range cases {
		if got := NormalizeMethod(input); got != want {
			t.Fatalf("NormalizeMethod(%q) = %q, want %q", input, got, want)
		}
	}
}
