package wanted

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

func (s *Store) ListReleaseProfiles(ctx context.Context) ([]ReleaseProfile, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select id, name, enabled, required, ignored, preferred, created_at, updated_at
		from release_profiles
		order by name, created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []ReleaseProfile
	for rows.Next() {
		profile, err := scanReleaseProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *Store) UpsertReleaseProfile(ctx context.Context, profile ReleaseProfile) (ReleaseProfile, error) {
	if !s.Configured() {
		return ReleaseProfile{}, errors.New("wanted store is unavailable")
	}
	profile = normalizeReleaseProfile(profile)
	preferred, err := json.Marshal(profile.Preferred)
	if err != nil {
		return ReleaseProfile{}, err
	}
	required := strings.Join(profile.Required, ",")
	ignored := strings.Join(profile.Ignored, ",")
	if profile.ID != "" {
		row := s.db.QueryRowContext(ctx, `
			update release_profiles set
				name = $2,
				enabled = $3,
				required = $4,
				ignored = $5,
				preferred = $6,
				updated_at = now()
			where id::text = $1
			returning id, name, enabled, required, ignored, preferred, created_at, updated_at
		`, profile.ID, profile.Name, profile.Enabled, required, ignored, preferred)
		saved, err := scanReleaseProfile(row)
		if err == nil {
			return saved, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return ReleaseProfile{}, err
		}
	}
	row := s.db.QueryRowContext(ctx, `
		insert into release_profiles (name, enabled, required, ignored, preferred)
		values ($1, $2, $3, $4, $5)
		returning id, name, enabled, required, ignored, preferred, created_at, updated_at
	`, profile.Name, profile.Enabled, required, ignored, preferred)
	return scanReleaseProfile(row)
}

func (s *Store) DeleteReleaseProfile(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("release profile id is required")
	}
	result, err := s.db.ExecContext(ctx, `delete from release_profiles where id::text = $1`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanReleaseProfile(row wantedScanner) (ReleaseProfile, error) {
	var profile ReleaseProfile
	var required, ignored string
	var preferred []byte
	if err := row.Scan(
		&profile.ID, &profile.Name, &profile.Enabled, &required, &ignored,
		&preferred, &profile.CreatedAt, &profile.UpdatedAt,
	); err != nil {
		return ReleaseProfile{}, err
	}
	profile.Required = splitComma(required)
	profile.Ignored = splitComma(ignored)
	profile.Preferred = []PreferredTerm{}
	if len(preferred) > 0 {
		_ = json.Unmarshal(preferred, &profile.Preferred)
	}
	if profile.Required == nil {
		profile.Required = []string{}
	}
	if profile.Ignored == nil {
		profile.Ignored = []string{}
	}
	return profile, nil
}

func (s *Store) ListQualityDefinitions(ctx context.Context) ([]QualityDefinition, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select quality, title, min_size_mb, max_size_mb
		from quality_definitions
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var definitions []QualityDefinition
	for rows.Next() {
		var definition QualityDefinition
		if err := rows.Scan(&definition.Quality, &definition.Title, &definition.MinSizeMB, &definition.MaxSizeMB); err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortQualityDefinitions(definitions)
	return definitions, nil
}

// UpdateQualityDefinitions bulk-updates the per-quality size windows. Unknown
// quality IDs are rejected so misconfigured payloads fail loudly.
func (s *Store) UpdateQualityDefinitions(ctx context.Context, definitions []QualityDefinition) ([]QualityDefinition, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	normalized, unknown := normalizeQualityDefinitions(definitions)
	if unknown != "" {
		return nil, errors.New("unknown quality: " + unknown)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, definition := range normalized {
		if _, err := tx.ExecContext(ctx, `
			insert into quality_definitions (quality, title, min_size_mb, max_size_mb)
			values ($1, $2, $3, $4)
			on conflict (quality) do update set
				title = case when excluded.title <> '' then excluded.title else quality_definitions.title end,
				min_size_mb = excluded.min_size_mb,
				max_size_mb = excluded.max_size_mb
		`, definition.Quality, definition.Title, definition.MinSizeMB, definition.MaxSizeMB); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.ListQualityDefinitions(ctx)
}

func sortQualityDefinitions(definitions []QualityDefinition) {
	sort.SliceStable(definitions, func(i, j int) bool {
		return qualityDefinitionSortIndex(definitions[i].Quality) < qualityDefinitionSortIndex(definitions[j].Quality)
	})
}
