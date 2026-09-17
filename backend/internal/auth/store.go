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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(761906101)`); err != nil {
		return User{}, err
	}
	existing, err := scanUser(tx.QueryRowContext(ctx, `select id::text, username, password_hash, created_at, updated_at from users order by created_at limit 1 for update`))
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}
	var user User
	if exists {
		user, err = scanUser(tx.QueryRowContext(ctx, `update users set username=$2, password_hash=$3, updated_at=now() where id=$1 returning id::text, username, password_hash, created_at, updated_at`, existing.ID, username, passwordHash))
	} else {
		user, err = scanUser(tx.QueryRowContext(ctx, `insert into users(username,password_hash) values($1,$2) returning id::text, username, password_hash, created_at, updated_at`, username, passwordHash))
	}
	if err != nil {
		return User{}, err
	}
	if exists && (username != existing.Username || passwordHash != existing.PasswordHash) {
		if _, err := tx.ExecContext(ctx, `delete from sessions where user_id=$1`, existing.ID); err != nil {
			return User{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Share the configuration lock: a login racing a password change either
	// commits first and is revoked, or fails against the new credential hash.
	if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(761906101)`); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		insert into sessions (token_hash, user_id, expires_at)
		select $1, id, $3 from users where id=$2 and password_hash=$4
	`, session.TokenHash, session.UserID, session.ExpiresAt.UTC(), session.CredentialHash)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrInvalidCredentials
	}
	return tx.Commit()
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
