package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Service owns the active auth method plus login/session verification. The
// method lives in memory (seeded from env/persisted config at startup and
// updated by PUT /api/v1/auth/config).
type Service struct {
	store  UserStore
	logger *slog.Logger
	now    func() time.Time

	mu     sync.RWMutex
	method string
}

func NewService(store UserStore, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{store: store, logger: logger, now: func() time.Time { return time.Now().UTC() }, method: MethodNone}
}

// Available reports whether user/session persistence is usable.
func (s *Service) Available() bool {
	return s != nil && s.store != nil && s.store.Configured()
}

func (s *Service) Method() string {
	if s == nil {
		return MethodNone
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.method
}

func (s *Service) SetMethod(method string) {
	if s == nil {
		return
	}
	normalized := NormalizeMethod(strings.ToLower(strings.TrimSpace(method)))
	if normalized == "" {
		normalized = MethodNone
	}
	s.mu.Lock()
	s.method = normalized
	s.mu.Unlock()
}

// EnsureUser seeds or updates the single user row (startup env seed and
// PUT /api/v1/auth/config). An empty password keeps the stored hash and only
// renames the user.
func (s *Service) EnsureUser(ctx context.Context, username string, password string) error {
	if !s.Available() {
		return ErrUnavailable
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username is required")
	}
	if password == "" {
		existing, ok, err := s.store.GetUser(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("password is required to create the user")
		}
		_, err = s.store.UpsertUser(ctx, username, existing.PasswordHash)
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.store.UpsertUser(ctx, username, hash)
	return err
}

// HasUser reports whether a credential row exists.
func (s *Service) HasUser(ctx context.Context) bool {
	if !s.Available() {
		return false
	}
	_, ok, err := s.store.GetUser(ctx)
	return err == nil && ok
}

// VerifyPassword checks Basic/forms credentials against the users table.
func (s *Service) VerifyPassword(ctx context.Context, username string, password string) (User, bool) {
	if !s.Available() {
		return User{}, false
	}
	user, ok, err := s.store.GetUserByUsername(ctx, username)
	if err != nil || !ok {
		return User{}, false
	}
	if !CheckPassword(user.PasswordHash, password) {
		return User{}, false
	}
	return user, true
}

// LoginResult carries the raw session token (cookie value) and its expiry.
type LoginResult struct {
	Token     string
	Username  string
	ExpiresAt time.Time
	Remember  bool
}

// Login verifies credentials and issues a session. RememberMe sessions live
// RememberMeTTL; plain sessions live SessionTTL server-side (the cookie
// itself is a browser-session cookie).
func (s *Service) Login(ctx context.Context, username string, password string, rememberMe bool) (LoginResult, error) {
	if !s.Available() {
		return LoginResult{}, ErrUnavailable
	}
	user, ok := s.VerifyPassword(ctx, username, password)
	if !ok {
		return LoginResult{}, ErrInvalidCredentials
	}
	token, err := newSessionToken()
	if err != nil {
		return LoginResult{}, err
	}
	ttl := SessionTTL
	if rememberMe {
		ttl = RememberMeTTL
	}
	expiresAt := s.now().Add(ttl)
	if err := s.store.CreateSession(ctx, Session{
		TokenHash: HashSessionToken(token),
		UserID:    user.ID,
		ExpiresAt: expiresAt,
	}); err != nil {
		return LoginResult{}, err
	}
	// Opportunistic cleanup keeps the table bounded without a dedicated task.
	_ = s.store.DeleteExpiredSessions(ctx, s.now())
	return LoginResult{Token: token, Username: user.Username, ExpiresAt: expiresAt, Remember: rememberMe}, nil
}

// ValidateSession resolves a raw cookie token to its user. Expired sessions
// are rejected (and deleted).
func (s *Service) ValidateSession(ctx context.Context, token string) (User, bool) {
	if !s.Available() || strings.TrimSpace(token) == "" {
		return User{}, false
	}
	hash := HashSessionToken(token)
	session, ok, err := s.store.GetSession(ctx, hash)
	if err != nil || !ok {
		return User{}, false
	}
	if !session.ExpiresAt.After(s.now()) {
		_ = s.store.DeleteSession(ctx, hash)
		return User{}, false
	}
	user, ok, err := s.store.GetUser(ctx)
	if err != nil || !ok {
		return User{}, false
	}
	return user, true
}

// Logout deletes the session behind a raw cookie token.
func (s *Service) Logout(ctx context.Context, token string) {
	if !s.Available() || strings.TrimSpace(token) == "" {
		return
	}
	_ = s.store.DeleteSession(ctx, HashSessionToken(token))
}

func newSessionToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// HashSessionToken maps the raw cookie value onto the stored token_hash.
func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
