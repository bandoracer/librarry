// Package auth implements arr-parity in-app authentication for the API:
// method none (open), basic (RFC Basic against the users table), or forms
// (session cookie). API keys always bypass method auth so Readarr-compatible
// clients keep working. Only the API enforces auth; the nginx-served static
// UI stays public and gates itself on GET /api/v1/auth/status.
package auth

import (
	"context"
	"errors"
	"time"
)

const (
	// MethodNone leaves the API open (API key still enforced when set).
	MethodNone = "none"
	// MethodBasic requires RFC Basic credentials against the users table.
	MethodBasic = "basic"
	// MethodForms requires the librarry_session cookie issued by POST /api/v1/login.
	MethodForms = "forms"

	// SessionCookieName is the forms-auth cookie.
	SessionCookieName = "librarry_session"

	// RememberMeTTL is the session lifetime when the login asks to be
	// remembered; SessionTTL covers plain (browser-session cookie) logins.
	RememberMeTTL = 30 * 24 * time.Hour
	SessionTTL    = 24 * time.Hour
)

// ErrInvalidCredentials marks failed logins (HTTP handlers map it to 401).
var ErrInvalidCredentials = errors.New("invalid username or password")

// ErrUnavailable marks auth operations without database persistence.
var ErrUnavailable = errors.New("authentication requires database persistence")

type User struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// UserStore persists the single-user credential row and its sessions. It is
// an interface so the session/login logic can be tested without Postgres.
type UserStore interface {
	Configured() bool
	// UpsertUser seeds or updates the single user row.
	UpsertUser(ctx context.Context, username string, passwordHash string) (User, error)
	// GetUser returns the single user row when present.
	GetUser(ctx context.Context) (User, bool, error)
	GetUserByUsername(ctx context.Context, username string) (User, bool, error)
	CreateSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, tokenHash string) (Session, bool, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) error
}

// NormalizeMethod maps arbitrary input onto none|basic|forms ("" stays ""
// so callers can distinguish "unset" from an explicit none).
func NormalizeMethod(method string) string {
	switch method {
	case MethodBasic, "http", "httpbasic":
		return MethodBasic
	case MethodForms, "form", "session", "cookie":
		return MethodForms
	case MethodNone, "disabled", "off", "open":
		return MethodNone
	default:
		return ""
	}
}
