package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// ErrTargetNotFound marks lookups for unknown target ids (HTTP handlers map
// it to 404).
var ErrTargetNotFound = errors.New("notification target not found")

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

const targetColumns = `
	id::text, name, type, settings, on_grab, on_import, on_upgrade,
	on_download_failure, on_health_issue, enabled, created_at, updated_at
`

func (s *Store) ListTargets(ctx context.Context) ([]Target, error) {
	if !s.Configured() {
		return nil, errors.New("notification store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select `+targetColumns+`
		from notification_targets
		order by created_at, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []Target
	for rows.Next() {
		target, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *Store) GetTarget(ctx context.Context, id string) (Target, error) {
	if !s.Configured() {
		return Target{}, errors.New("notification store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Target{}, ErrTargetNotFound
	}
	row := s.db.QueryRowContext(ctx, `
		select `+targetColumns+`
		from notification_targets
		where id::text = $1
	`, id)
	target, err := scanTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, ErrTargetNotFound
	}
	return target, err
}

func (s *Store) CreateTarget(ctx context.Context, target Target) (Target, error) {
	if !s.Configured() {
		return Target{}, errors.New("notification store is unavailable")
	}
	settings, err := marshalSettings(target.Settings)
	if err != nil {
		return Target{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		insert into notification_targets (
			name, type, settings, on_grab, on_import, on_upgrade,
			on_download_failure, on_health_issue, enabled
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		returning `+targetColumns+`
	`, target.Name, target.Type, settings,
		target.Triggers.OnGrab, target.Triggers.OnImport, target.Triggers.OnUpgrade,
		target.Triggers.OnDownloadFailure, target.Triggers.OnHealthIssue, target.Enabled)
	return scanTarget(row)
}

func (s *Store) UpdateTarget(ctx context.Context, id string, target Target) (Target, error) {
	if !s.Configured() {
		return Target{}, errors.New("notification store is unavailable")
	}
	settings, err := marshalSettings(target.Settings)
	if err != nil {
		return Target{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		update notification_targets set
			name = $2, type = $3, settings = $4, on_grab = $5, on_import = $6,
			on_upgrade = $7, on_download_failure = $8, on_health_issue = $9,
			enabled = $10, updated_at = now()
		where id::text = $1
		returning `+targetColumns+`
	`, strings.TrimSpace(id), target.Name, target.Type, settings,
		target.Triggers.OnGrab, target.Triggers.OnImport, target.Triggers.OnUpgrade,
		target.Triggers.OnDownloadFailure, target.Triggers.OnHealthIssue, target.Enabled)
	updated, err := scanTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, ErrTargetNotFound
	}
	return updated, err
}

func (s *Store) DeleteTarget(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("notification store is unavailable")
	}
	result, err := s.db.ExecContext(ctx, `
		delete from notification_targets where id::text = $1
	`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTargetNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTarget(row rowScanner) (Target, error) {
	var target Target
	var settings []byte
	if err := row.Scan(
		&target.ID, &target.Name, &target.Type, &settings,
		&target.Triggers.OnGrab, &target.Triggers.OnImport, &target.Triggers.OnUpgrade,
		&target.Triggers.OnDownloadFailure, &target.Triggers.OnHealthIssue,
		&target.Enabled, &target.CreatedAt, &target.UpdatedAt,
	); err != nil {
		return Target{}, err
	}
	target.Settings = map[string]string{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &target.Settings); err != nil {
			return Target{}, err
		}
	}
	return target, nil
}

func marshalSettings(settings map[string]string) ([]byte, error) {
	if settings == nil {
		settings = map[string]string{}
	}
	return json.Marshal(settings)
}
