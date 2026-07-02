package importlists

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

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

const listColumns = `
	id::text, name, type, enabled, settings, monitor, quality_profile,
	coalesce(root_folder_id::text, ''), search_on_add, last_synced_at,
	created_at, updated_at
`

func (s *Store) ListLists(ctx context.Context) ([]List, error) {
	if !s.Configured() {
		return nil, errors.New("import list store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select `+listColumns+` from import_lists order by created_at, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []List
	for rows.Next() {
		list, err := scanList(rows)
		if err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}
	return lists, rows.Err()
}

func (s *Store) GetList(ctx context.Context, id string) (List, error) {
	if !s.Configured() {
		return List{}, errors.New("import list store is unavailable")
	}
	row := s.db.QueryRowContext(ctx, `
		select `+listColumns+` from import_lists where id::text = $1
	`, strings.TrimSpace(id))
	list, err := scanList(row)
	if errors.Is(err, sql.ErrNoRows) {
		return List{}, ErrListNotFound
	}
	return list, err
}

func (s *Store) CreateList(ctx context.Context, list List) (List, error) {
	if !s.Configured() {
		return List{}, errors.New("import list store is unavailable")
	}
	list = normalizeList(list)
	if list.Name == "" {
		return List{}, errors.New("import list name is required")
	}
	settings, err := marshalSettings(list.Settings)
	if err != nil {
		return List{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		insert into import_lists (
			name, type, enabled, settings, monitor, quality_profile,
			root_folder_id, search_on_add
		) values ($1, $2, $3, $4, $5, $6, nullif($7, '')::uuid, $8)
		returning `+listColumns+`
	`, list.Name, list.Type, list.Enabled, settings, list.Monitor, list.QualityProfile,
		list.RootFolderID, list.SearchOnAdd)
	return scanList(row)
}

func (s *Store) UpdateList(ctx context.Context, id string, list List) (List, error) {
	if !s.Configured() {
		return List{}, errors.New("import list store is unavailable")
	}
	list = normalizeList(list)
	settings, err := marshalSettings(list.Settings)
	if err != nil {
		return List{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		update import_lists set
			name = case when $2 = '' then name else $2 end,
			type = $3, enabled = $4, settings = $5, monitor = $6,
			quality_profile = $7, root_folder_id = nullif($8, '')::uuid,
			search_on_add = $9, updated_at = now()
		where id::text = $1
		returning `+listColumns+`
	`, strings.TrimSpace(id), list.Name, list.Type, list.Enabled, settings,
		list.Monitor, list.QualityProfile, list.RootFolderID, list.SearchOnAdd)
	updated, err := scanList(row)
	if errors.Is(err, sql.ErrNoRows) {
		return List{}, ErrListNotFound
	}
	return updated, err
}

func (s *Store) DeleteList(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("import list store is unavailable")
	}
	result, err := s.db.ExecContext(ctx, `delete from import_lists where id::text = $1`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrListNotFound
	}
	return nil
}

func (s *Store) MarkListSynced(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("import list store is unavailable")
	}
	_, err := s.db.ExecContext(ctx, `
		update import_lists set last_synced_at = now(), updated_at = now() where id::text = $1
	`, strings.TrimSpace(id))
	return err
}

func (s *Store) ListExclusions(ctx context.Context) ([]Exclusion, error) {
	if !s.Configured() {
		return nil, errors.New("import list store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select id::text, title, author_name, source_key, created_at
		from import_list_exclusions
		order by created_at desc
		limit 1000
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exclusions []Exclusion
	for rows.Next() {
		var exclusion Exclusion
		if err := rows.Scan(&exclusion.ID, &exclusion.Title, &exclusion.AuthorName, &exclusion.SourceKey, &exclusion.CreatedAt); err != nil {
			return nil, err
		}
		exclusions = append(exclusions, exclusion)
	}
	return exclusions, rows.Err()
}

func (s *Store) CreateExclusion(ctx context.Context, exclusion Exclusion) (Exclusion, error) {
	if !s.Configured() {
		return Exclusion{}, errors.New("import list store is unavailable")
	}
	exclusion.Title = strings.TrimSpace(exclusion.Title)
	exclusion.AuthorName = strings.TrimSpace(exclusion.AuthorName)
	exclusion.SourceKey = strings.TrimSpace(exclusion.SourceKey)
	if exclusion.SourceKey == "" && exclusion.Title == "" {
		return Exclusion{}, errors.New("exclusion requires a sourceKey or title")
	}
	if exclusion.SourceKey == "" {
		// Title-only exclusions get a deterministic manual key so the unique
		// index still applies per book.
		exclusion.SourceKey = "manual:" + slugValue(exclusion.Title) + ":" + slugValue(exclusion.AuthorName)
	}
	row := s.db.QueryRowContext(ctx, `
		insert into import_list_exclusions (title, author_name, source_key)
		values ($1, $2, $3)
		on conflict (source_key) do update set
			title = excluded.title,
			author_name = excluded.author_name
		returning id::text, title, author_name, source_key, created_at
	`, exclusion.Title, exclusion.AuthorName, exclusion.SourceKey)
	var created Exclusion
	if err := row.Scan(&created.ID, &created.Title, &created.AuthorName, &created.SourceKey, &created.CreatedAt); err != nil {
		return Exclusion{}, err
	}
	return created, nil
}

func (s *Store) DeleteExclusion(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("import list store is unavailable")
	}
	result, err := s.db.ExecContext(ctx, `delete from import_list_exclusions where id::text = $1`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrExclusionNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanList(row rowScanner) (List, error) {
	var list List
	var settings []byte
	var lastSyncedAt sql.NullTime
	if err := row.Scan(
		&list.ID, &list.Name, &list.Type, &list.Enabled, &settings, &list.Monitor,
		&list.QualityProfile, &list.RootFolderID, &list.SearchOnAdd, &lastSyncedAt,
		&list.CreatedAt, &list.UpdatedAt,
	); err != nil {
		return List{}, err
	}
	list.Settings = map[string]string{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &list.Settings); err != nil {
			return List{}, err
		}
	}
	if lastSyncedAt.Valid {
		value := lastSyncedAt.Time.UTC()
		list.LastSyncedAt = &value
	}
	return list, nil
}

func normalizeList(list List) List {
	list.Name = strings.TrimSpace(list.Name)
	list.Type = strings.ToLower(strings.TrimSpace(list.Type))
	if list.Type == "" {
		list.Type = "hardcover"
	}
	list.Monitor = strings.ToLower(strings.TrimSpace(list.Monitor))
	if list.Monitor != "none" {
		list.Monitor = "all"
	}
	list.QualityProfile = strings.TrimSpace(list.QualityProfile)
	if list.QualityProfile == "" {
		list.QualityProfile = "standard"
	}
	list.RootFolderID = strings.TrimSpace(list.RootFolderID)
	if list.Settings == nil {
		list.Settings = map[string]string{}
	}
	return list
}

func marshalSettings(settings map[string]string) ([]byte, error) {
	if settings == nil {
		settings = map[string]string{}
	}
	return json.Marshal(settings)
}

func slugValue(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			builder.WriteRune('-')
		}
	}
	return builder.String()
}
