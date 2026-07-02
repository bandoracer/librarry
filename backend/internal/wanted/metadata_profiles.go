package wanted

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrMetadataProfileInUse marks a delete refused because author subscriptions
// still reference the profile (HTTP handlers map it to 409).
var ErrMetadataProfileInUse = errors.New("metadata profile is in use")

// MetadataProfile is a named, reusable author add-filter set. When an author
// subscription references a profile, the profile filters win over the legacy
// per-author override columns.
type MetadataProfile struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	AllowedLanguages []string  `json:"allowedLanguages"`
	MustNotContain   []string  `json:"mustNotContain"`
	SkipMissingISBN  bool      `json:"skipMissingIsbn"`
	MinPages         int       `json:"minPages"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// normalizeMetadataProfile trims and dedupes profile input; the name is the
// only required field (a profile with no filters is valid — that is the seeded
// "Standard" profile).
func normalizeMetadataProfile(profile MetadataProfile) (MetadataProfile, error) {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Name = strings.TrimSpace(profile.Name)
	if profile.Name == "" {
		return MetadataProfile{}, errors.New("metadata profile name is required")
	}
	profile.AllowedLanguages = normalizeFilterTerms(profile.AllowedLanguages)
	profile.MustNotContain = normalizeFilterTerms(profile.MustNotContain)
	if profile.MinPages < 0 {
		profile.MinPages = 0
	}
	return profile, nil
}

// applyMetadataProfileFilters resolves the effective add-filters for an author
// monitor pass: the profile replaces every per-author filter column wholesale
// (profile wins even when a profile field is empty), leaving the stored
// override columns untouched for subscriptions without a profile.
func applyMetadataProfileFilters(subscription AuthorSubscription, profile MetadataProfile) AuthorSubscription {
	if strings.TrimSpace(subscription.MetadataProfileID) == "" {
		return subscription
	}
	subscription.AllowedLanguages = normalizeFilterTerms(profile.AllowedLanguages)
	subscription.MustNotContain = normalizeFilterTerms(profile.MustNotContain)
	subscription.SkipMissingISBN = profile.SkipMissingISBN
	subscription.MinPages = max(profile.MinPages, 0)
	return subscription
}

const metadataProfileColumns = `
	id::text, name, allowed_languages, must_not_contain,
	skip_missing_isbn, min_pages, created_at, updated_at
`

func (s *Store) ListMetadataProfiles(ctx context.Context) ([]MetadataProfile, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select `+metadataProfileColumns+`
		from metadata_profiles
		order by name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []MetadataProfile
	for rows.Next() {
		profile, err := scanMetadataProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *Store) GetMetadataProfile(ctx context.Context, id string) (MetadataProfile, error) {
	if !s.Configured() {
		return MetadataProfile{}, errors.New("wanted store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return MetadataProfile{}, errors.New("metadata profile id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		select `+metadataProfileColumns+`
		from metadata_profiles
		where id::text = $1
	`, id)
	return scanMetadataProfile(row)
}

func (s *Store) CreateMetadataProfile(ctx context.Context, profile MetadataProfile) (MetadataProfile, error) {
	if !s.Configured() {
		return MetadataProfile{}, errors.New("wanted store is unavailable")
	}
	profile, err := normalizeMetadataProfile(profile)
	if err != nil {
		return MetadataProfile{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		insert into metadata_profiles (
			name, allowed_languages, must_not_contain, skip_missing_isbn, min_pages
		) values ($1, $2, $3, $4, $5)
		returning `+metadataProfileColumns+`
	`, profile.Name, joinFilterTerms(profile.AllowedLanguages), joinFilterTerms(profile.MustNotContain),
		profile.SkipMissingISBN, profile.MinPages)
	created, err := scanMetadataProfile(row)
	if err != nil {
		return MetadataProfile{}, metadataProfileNameError(err)
	}
	return created, nil
}

func (s *Store) UpdateMetadataProfile(ctx context.Context, id string, profile MetadataProfile) (MetadataProfile, error) {
	if !s.Configured() {
		return MetadataProfile{}, errors.New("wanted store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return MetadataProfile{}, errors.New("metadata profile id is required")
	}
	profile, err := normalizeMetadataProfile(profile)
	if err != nil {
		return MetadataProfile{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		update metadata_profiles set
			name = $2,
			allowed_languages = $3,
			must_not_contain = $4,
			skip_missing_isbn = $5,
			min_pages = $6,
			updated_at = now()
		where id::text = $1
		returning `+metadataProfileColumns+`
	`, id, profile.Name, joinFilterTerms(profile.AllowedLanguages), joinFilterTerms(profile.MustNotContain),
		profile.SkipMissingISBN, profile.MinPages)
	updated, err := scanMetadataProfile(row)
	if err != nil {
		return MetadataProfile{}, metadataProfileNameError(err)
	}
	return updated, nil
}

// DeleteMetadataProfile refuses (ErrMetadataProfileInUse) while author
// subscriptions still reference the profile.
func (s *Store) DeleteMetadataProfile(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("metadata profile id is required")
	}
	var references int
	if err := s.db.QueryRowContext(ctx, `
		select count(*) from author_subscriptions where metadata_profile_id::text = $1
	`, id).Scan(&references); err != nil {
		return err
	}
	if references > 0 {
		return ErrMetadataProfileInUse
	}
	result, err := s.db.ExecContext(ctx, `delete from metadata_profiles where id::text = $1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ensureMetadataProfileExists validates an author subscription's profile
// reference before writes so callers get an explainable error instead of a
// foreign-key violation.
func (s *Store) ensureMetadataProfileExists(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		select exists(select 1 from metadata_profiles where id::text = $1)
	`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("metadata profile not found")
	}
	return nil
}

func metadataProfileNameError(err error) error {
	if err != nil && strings.Contains(err.Error(), "duplicate key") {
		return errors.New("metadata profile name is already in use")
	}
	return err
}

func scanMetadataProfile(row wantedScanner) (MetadataProfile, error) {
	var profile MetadataProfile
	var allowedLanguages, mustNotContain string
	if err := row.Scan(
		&profile.ID, &profile.Name, &allowedLanguages, &mustNotContain,
		&profile.SkipMissingISBN, &profile.MinPages, &profile.CreatedAt, &profile.UpdatedAt,
	); err != nil {
		return MetadataProfile{}, err
	}
	profile.AllowedLanguages = splitFilterTerms(allowedLanguages)
	profile.MustNotContain = splitFilterTerms(mustNotContain)
	return profile, nil
}

// Service ---------------------------------------------------------------------

func (s *Service) ListMetadataProfiles(ctx context.Context) ([]MetadataProfile, error) {
	if !s.Available() {
		return []MetadataProfile{}, nil
	}
	profiles, err := s.store.ListMetadataProfiles(ctx)
	if err != nil {
		return nil, err
	}
	if profiles == nil {
		profiles = []MetadataProfile{}
	}
	return profiles, nil
}

func (s *Service) CreateMetadataProfile(ctx context.Context, profile MetadataProfile) (MetadataProfile, error) {
	if !s.Available() {
		return MetadataProfile{}, errors.New("wanted service requires database persistence")
	}
	return s.store.CreateMetadataProfile(ctx, profile)
}

func (s *Service) UpdateMetadataProfile(ctx context.Context, id string, profile MetadataProfile) (MetadataProfile, error) {
	if !s.Available() {
		return MetadataProfile{}, errors.New("wanted service requires database persistence")
	}
	return s.store.UpdateMetadataProfile(ctx, id, profile)
}

func (s *Service) DeleteMetadataProfile(ctx context.Context, id string) error {
	if !s.Available() {
		return errors.New("wanted service requires database persistence")
	}
	return s.store.DeleteMetadataProfile(ctx, id)
}

// resolveAuthorSubscriptionFilters loads the referenced metadata profile (when
// set) and swaps its filters in for the monitor pass. Lookup failures fall
// back to the stored per-author columns and surface as an explainable error.
func (s *Service) resolveAuthorSubscriptionFilters(ctx context.Context, subscription AuthorSubscription) (AuthorSubscription, error) {
	if strings.TrimSpace(subscription.MetadataProfileID) == "" {
		return subscription, nil
	}
	profile, err := s.store.GetMetadataProfile(ctx, subscription.MetadataProfileID)
	if err != nil {
		return subscription, errors.New("metadata profile lookup failed: " + err.Error())
	}
	return applyMetadataProfileFilters(subscription, profile), nil
}
