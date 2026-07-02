package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Store is the Postgres-backed UserStore.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	if db == nil {
		return nil
	}
	return &Store{db: db}
}

func (s *Store) Configured() bool {
	return s != nil && s.db != nil
}

func (s *Store) UpsertUser(ctx context.Context, username string, passwordHash string) (User, error) {
	if !s.Configured() {
		return User{}, ErrUnavailable
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, errors.New("username is required")
	}
	existing, ok, err := s.GetUser(ctx)
	if err != nil {
		return User{}, err
	}
	if ok {
		row := s.db.QueryRowContext(ctx, `
			update users set username = $2, password_hash = $3, updated_at = now()
			where id = $1
			returning id::text, username, password_hash, created_at, updated_at
		`, existing.ID, username, passwordHash)
		return scanUser(row)
	}
	row := s.db.QueryRowContext(ctx, `
		insert into users (username, password_hash) values ($1, $2)
		returning id::text, username, password_hash, created_at, updated_at
	`, username, passwordHash)
	return scanUser(row)
}

func (s *Store) GetUser(ctx context.Context) (User, bool, error) {
	if !s.Configured() {
		return User{}, false, ErrUnavailable
	}
	row := s.db.QueryRowContext(ctx, `
		select id::text, username, password_hash, created_at, updated_at
		from users
		order by created_at
		limit 1
	`)
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return user, true, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, bool, error) {
	if !s.Configured() {
		return User{}, false, ErrUnavailable
	}
	row := s.db.QueryRowContext(ctx, `
		select id::text, username, password_hash, created_at, updated_at
		from users
		where lower(username) = lower($1)
	`, strings.TrimSpace(username))
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return user, true, nil
}

func (s *Store) CreateSession(ctx context.Context, session Session) error {
	if !s.Configured() {
		return ErrUnavailable
	}
	_, err := s.db.ExecContext(ctx, `
		insert into sessions (token_hash, user_id, expires_at) values ($1, $2, $3)
	`, session.TokenHash, session.UserID, session.ExpiresAt.UTC())
	return err
}

func (s *Store) GetSession(ctx context.Context, tokenHash string) (Session, bool, error) {
	if !s.Configured() {
		return Session{}, false, ErrUnavailable
	}
	var session Session
	err := s.db.QueryRowContext(ctx, `
		select token_hash, user_id::text, expires_at, created_at
		from sessions
		where token_hash = $1
	`, tokenHash).Scan(&session.TokenHash, &session.UserID, &session.ExpiresAt, &session.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	return session, true, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if !s.Configured() {
		return ErrUnavailable
	}
	_, err := s.db.ExecContext(ctx, `delete from sessions where token_hash = $1`, tokenHash)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	if !s.Configured() {
		return ErrUnavailable
	}
	_, err := s.db.ExecContext(ctx, `delete from sessions where expires_at <= $1`, now.UTC())
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return User{}, err
	}
	return user, nil
}
