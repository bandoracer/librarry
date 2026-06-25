package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
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

func (s *Store) UpsertFile(ctx context.Context, file FileRecord) (FileRecord, error) {
	if !s.Configured() {
		return FileRecord{}, errors.New("library store is unavailable")
	}
	if strings.TrimSpace(file.Path) == "" {
		return FileRecord{}, errors.New("file path is required")
	}
	if strings.TrimSpace(file.MediaFormat) == "" {
		return FileRecord{}, errors.New("file media format is required")
	}
	if strings.TrimSpace(file.ImportStatus) == "" {
		file.ImportStatus = "available"
	}
	if file.Metadata == nil {
		file.Metadata = map[string]any{}
	}
	raw, err := json.Marshal(file.Metadata)
	if err != nil {
		return FileRecord{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		insert into files (
			edition_id, media_format, path, source_path, title, author_name,
			extension, size_bytes, checksum, import_status, metadata, modified_at
		) values (
			nullif($1, '')::uuid, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11::jsonb, $12
		)
		on conflict (path) do update set
			edition_id = coalesce(excluded.edition_id, files.edition_id),
			media_format = excluded.media_format,
			source_path = excluded.source_path,
			title = excluded.title,
			author_name = excluded.author_name,
			extension = excluded.extension,
			size_bytes = excluded.size_bytes,
			checksum = excluded.checksum,
			import_status = excluded.import_status,
			metadata = excluded.metadata,
			modified_at = excluded.modified_at,
			updated_at = now()
		returning
			id, coalesce(edition_id::text, ''), media_format, path, source_path,
			title, author_name, extension, coalesce(size_bytes, 0), coalesce(checksum, ''),
			import_status, metadata, modified_at, created_at, updated_at
	`, file.EditionID, file.MediaFormat, file.Path, file.SourcePath, file.Title, file.AuthorName,
		file.Extension, nullableInt64(file.SizeBytes), file.Checksum, file.ImportStatus, string(raw), file.ModifiedAt)
	return scanFile(row)
}

func (s *Store) ListFiles(ctx context.Context, query FileListQuery) ([]FileRecord, error) {
	if !s.Configured() {
		return nil, errors.New("library store is unavailable")
	}
	limit := query.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	args := []any{}
	where := []string{}
	if strings.TrimSpace(query.Format) != "" && strings.TrimSpace(query.Format) != "any" {
		args = append(args, strings.TrimSpace(query.Format))
		where = append(where, "media_format = $"+strconv.Itoa(len(args)))
	}
	if strings.TrimSpace(query.Status) != "" {
		args = append(args, strings.TrimSpace(query.Status))
		where = append(where, "import_status = $"+strconv.Itoa(len(args)))
	}
	args = append(args, limit)
	sqlText := `
		select
			id, coalesce(edition_id::text, ''), media_format, path, source_path,
			title, author_name, extension, coalesce(size_bytes, 0), coalesce(checksum, ''),
			import_status, metadata, modified_at, created_at, updated_at
		from files
	`
	if len(where) > 0 {
		sqlText += " where " + strings.Join(where, " and ")
	}
	sqlText += " order by updated_at desc limit $" + strconv.Itoa(len(args))

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []FileRecord
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

type fileScanner interface {
	Scan(dest ...any) error
}

func scanFile(row fileScanner) (FileRecord, error) {
	var file FileRecord
	var raw []byte
	var modifiedAt sql.NullTime
	if err := row.Scan(
		&file.ID, &file.EditionID, &file.MediaFormat, &file.Path, &file.SourcePath,
		&file.Title, &file.AuthorName, &file.Extension, &file.SizeBytes, &file.Checksum,
		&file.ImportStatus, &raw, &modifiedAt, &file.CreatedAt, &file.UpdatedAt,
	); err != nil {
		return FileRecord{}, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &file.Metadata)
	}
	if file.Metadata == nil {
		file.Metadata = map[string]any{}
	}
	if modifiedAt.Valid {
		value := modifiedAt.Time.UTC()
		file.ModifiedAt = &value
	}
	return file, nil
}

func nullableInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
