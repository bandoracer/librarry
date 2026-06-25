package compat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type RootFolder struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Path        string         `json:"path"`
	MediaFormat string         `json:"mediaFormat"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

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

func (s *Store) ListRootFolders(ctx context.Context) ([]RootFolder, error) {
	if !s.Configured() {
		return nil, errors.New("compat store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select id::text, name, path, media_format, metadata, created_at, updated_at
		from compat_root_folders
		order by name, path
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []RootFolder
	for rows.Next() {
		folder, err := scanRootFolder(rows)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	return folders, rows.Err()
}

func (s *Store) CreateRootFolder(ctx context.Context, folder RootFolder) (RootFolder, error) {
	if !s.Configured() {
		return RootFolder{}, errors.New("compat store is unavailable")
	}
	folder = normalizeRootFolder(folder)
	if folder.Path == "" {
		return RootFolder{}, errors.New("root folder path is required")
	}
	raw, err := json.Marshal(folder.Metadata)
	if err != nil {
		return RootFolder{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		insert into compat_root_folders (name, path, media_format, metadata)
		values ($1, $2, $3, $4::jsonb)
		on conflict (path) do update set
			name = excluded.name,
			media_format = excluded.media_format,
			metadata = compat_root_folders.metadata || excluded.metadata,
			updated_at = now()
		returning id::text, name, path, media_format, metadata, created_at, updated_at
	`, folder.Name, folder.Path, folder.MediaFormat, string(raw))
	return scanRootFolder(row)
}

func (s *Store) DeleteRootFolder(ctx context.Context, id string) (bool, error) {
	if !s.Configured() {
		return false, errors.New("compat store is unavailable")
	}
	result, err := s.db.ExecContext(ctx, `delete from compat_root_folders where id::text = $1`, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

type rootFolderScanner interface {
	Scan(dest ...any) error
}

func scanRootFolder(row rootFolderScanner) (RootFolder, error) {
	var folder RootFolder
	var raw []byte
	if err := row.Scan(
		&folder.ID, &folder.Name, &folder.Path, &folder.MediaFormat,
		&raw, &folder.CreatedAt, &folder.UpdatedAt,
	); err != nil {
		return RootFolder{}, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &folder.Metadata)
	}
	if folder.Metadata == nil {
		folder.Metadata = map[string]any{}
	}
	return folder, nil
}

func normalizeRootFolder(folder RootFolder) RootFolder {
	folder.ID = strings.TrimSpace(folder.ID)
	folder.Name = strings.TrimSpace(folder.Name)
	folder.Path = strings.TrimSpace(folder.Path)
	folder.MediaFormat = normalizeMediaFormat(folder.MediaFormat)
	if folder.Name == "" {
		folder.Name = defaultRootFolderName(folder.Path)
	}
	if folder.Metadata == nil {
		folder.Metadata = map[string]any{}
	}
	return folder
}

func normalizeMediaFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "ebook", "audiobook", "mixed":
		return strings.ToLower(strings.TrimSpace(format))
	default:
		return "mixed"
	}
}

func defaultRootFolderName(path string) string {
	path = strings.TrimRight(strings.TrimSpace(path), `/\`)
	if path == "" {
		return "Books"
	}
	if idx := strings.LastIndexAny(path, `/\`); idx >= 0 {
		path = path[idx+1:]
	}
	if path == "" || path == "." {
		return "Books"
	}
	return path
}
