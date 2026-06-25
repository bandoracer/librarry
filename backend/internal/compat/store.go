package compat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
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

type Resource struct {
	ID           string         `json:"id"`
	ResourceType string         `json:"resourceType"`
	CompatID     int            `json:"compatId"`
	Name         string         `json:"name"`
	Payload      map[string]any `json:"payload"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
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

func (s *Store) ListResources(ctx context.Context, resourceType string) ([]Resource, error) {
	if !s.Configured() {
		return nil, errors.New("compat store is unavailable")
	}
	resourceType = normalizeResourceType(resourceType)
	if resourceType == "" {
		return nil, errors.New("resource type is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		select id::text, resource_type, compat_id, name, payload, created_at, updated_at
		from compat_resources
		where resource_type = $1 and deleted_at is null
		order by compat_id
	`, resourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resources []Resource
	for rows.Next() {
		resource, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, rows.Err()
}

func (s *Store) GetResource(ctx context.Context, resourceType string, compatID int) (Resource, bool, error) {
	if !s.Configured() {
		return Resource{}, false, errors.New("compat store is unavailable")
	}
	resourceType = normalizeResourceType(resourceType)
	if resourceType == "" || compatID <= 0 {
		return Resource{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		select id::text, resource_type, compat_id, name, payload, created_at, updated_at
		from compat_resources
		where resource_type = $1 and compat_id = $2 and deleted_at is null
	`, resourceType, compatID)
	resource, err := scanResource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Resource{}, false, nil
	}
	if err != nil {
		return Resource{}, false, err
	}
	return resource, true, nil
}

func (s *Store) UpsertResource(ctx context.Context, resource Resource) (Resource, error) {
	if !s.Configured() {
		return Resource{}, errors.New("compat store is unavailable")
	}
	resource = normalizeResource(resource)
	if resource.ResourceType == "" {
		return Resource{}, errors.New("resource type is required")
	}
	raw, err := json.Marshal(resource.Payload)
	if err != nil {
		return Resource{}, err
	}
	if resource.CompatID <= 0 {
		row := s.db.QueryRowContext(ctx, `
			with next_resource as (
				select coalesce(max(compat_id), 0) + 1 as compat_id
				from compat_resources
				where resource_type = $1
			)
			insert into compat_resources (resource_type, compat_id, name, payload)
			select $1, compat_id, $2, $3::jsonb
			from next_resource
			returning id::text, resource_type, compat_id, name, payload, created_at, updated_at
		`, resource.ResourceType, resource.Name, string(raw))
		return scanResource(row)
	}
	row := s.db.QueryRowContext(ctx, `
		insert into compat_resources (resource_type, compat_id, name, payload)
		values ($1, $2, $3, $4::jsonb)
		on conflict (resource_type, compat_id) do update set
			name = excluded.name,
			payload = excluded.payload,
			deleted_at = null,
			updated_at = now()
		returning id::text, resource_type, compat_id, name, payload, created_at, updated_at
	`, resource.ResourceType, resource.CompatID, resource.Name, string(raw))
	return scanResource(row)
}

func (s *Store) DeleteResource(ctx context.Context, resourceType string, compatID int) (bool, error) {
	if !s.Configured() {
		return false, errors.New("compat store is unavailable")
	}
	resourceType = normalizeResourceType(resourceType)
	if resourceType == "" || compatID <= 0 {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, `
		update compat_resources
		set deleted_at = now(), updated_at = now()
		where resource_type = $1 and compat_id = $2 and deleted_at is null
	`, resourceType, compatID)
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

func scanResource(row rootFolderScanner) (Resource, error) {
	var resource Resource
	var raw []byte
	if err := row.Scan(
		&resource.ID, &resource.ResourceType, &resource.CompatID, &resource.Name,
		&raw, &resource.CreatedAt, &resource.UpdatedAt,
	); err != nil {
		return Resource{}, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &resource.Payload)
	}
	if resource.Payload == nil {
		resource.Payload = map[string]any{}
	}
	return resource, nil
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

func normalizeResource(resource Resource) Resource {
	resource.ResourceType = normalizeResourceType(resource.ResourceType)
	resource.Name = strings.TrimSpace(resource.Name)
	if resource.Payload == nil {
		resource.Payload = map[string]any{}
	}
	if resource.Name == "" {
		resource.Name = resourceName(resource.Payload)
	}
	return resource
}

func normalizeResourceType(resourceType string) string {
	return strings.ToLower(strings.TrimSpace(resourceType))
}

func resourceName(payload map[string]any) string {
	for _, key := range []string{"name", "label", "implementation", "host"} {
		if value := payloadText(payload[key]); value != "" {
			return value
		}
	}
	return ""
}

func payloadText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
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
