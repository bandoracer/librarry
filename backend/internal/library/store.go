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

func (s *Store) FindFiles(ctx context.Context, ids []string, paths []string) ([]FileRecord, error) {
	if !s.Configured() {
		return nil, errors.New("library store is unavailable")
	}
	where, args := fileIdentifierWhere(ids, paths)
	if len(where) == 0 {
		return nil, errors.New("at least one file id or path is required")
	}
	sqlText := `
		select
			id, coalesce(edition_id::text, ''), media_format, path, source_path,
			title, author_name, extension, coalesce(size_bytes, 0), coalesce(checksum, ''),
			import_status, metadata, modified_at, created_at, updated_at
		from files
		where ` + strings.Join(where, " or ") + `
		order by updated_at desc
	`
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFiles(rows)
}

func (s *Store) DeleteFiles(ctx context.Context, ids []string, paths []string) ([]FileRecord, error) {
	if !s.Configured() {
		return nil, errors.New("library store is unavailable")
	}
	where, args := fileIdentifierWhere(ids, paths)
	if len(where) == 0 {
		return nil, errors.New("at least one file id or path is required")
	}
	sqlText := `
		delete from files
		where ` + strings.Join(where, " or ") + `
		returning
			id, coalesce(edition_id::text, ''), media_format, path, source_path,
			title, author_name, extension, coalesce(size_bytes, 0), coalesce(checksum, ''),
			import_status, metadata, modified_at, created_at, updated_at
	`
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFiles(rows)
}

func (s *Store) CreateImportReview(ctx context.Context, review ImportReview) (ImportReview, error) {
	if !s.Configured() {
		return ImportReview{}, errors.New("library store is unavailable")
	}
	if strings.TrimSpace(review.SourcePath) == "" {
		return ImportReview{}, errors.New("review source path is required")
	}
	if strings.TrimSpace(review.MediaFormat) == "" {
		review.MediaFormat = "unknown"
	}
	if strings.TrimSpace(review.Status) == "" {
		review.Status = "pending"
	}
	if review.Metadata == nil {
		review.Metadata = map[string]any{}
	}
	existing, err := s.findPendingImportReviewBySource(ctx, review.SourcePath)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ImportReview{}, err
	}
	raw, err := json.Marshal(review.Metadata)
	if err != nil {
		return ImportReview{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		insert into import_reviews (
			source_path, download_id, wanted_item_id, media_format, title, author_name,
			size_bytes, reason, status, decision, destination_path, metadata
		) values (
			$1, $2, nullif($3, '')::uuid, $4, $5, $6,
			$7, $8, $9, $10, $11, $12::jsonb
		)
		on conflict do nothing
		returning
			id, source_path, download_id, coalesce(wanted_item_id::text, ''), media_format,
			title, author_name, coalesce(size_bytes, 0), reason, status, decision,
			destination_path, metadata, created_at, updated_at, resolved_at
	`, strings.TrimSpace(review.SourcePath), strings.TrimSpace(review.DownloadID), strings.TrimSpace(review.WantedID),
		review.MediaFormat, review.Title, review.AuthorName, nullableInt64(review.SizeBytes), review.Reason,
		review.Status, review.Decision, review.DestinationPath, string(raw))
	created, err := scanImportReview(row)
	if err == nil {
		return created, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ImportReview{}, err
	}
	return s.findPendingImportReviewBySource(ctx, review.SourcePath)
}

func (s *Store) ListImportReviews(ctx context.Context, query ReviewListQuery) ([]ImportReview, error) {
	if !s.Configured() {
		return nil, errors.New("library store is unavailable")
	}
	limit := query.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []any{}
	where := []string{}
	status := strings.TrimSpace(query.Status)
	if status == "" {
		status = "pending"
	}
	if status != "all" {
		args = append(args, status)
		where = append(where, "status = $"+strconv.Itoa(len(args)))
	}
	args = append(args, limit)
	sqlText := `
		select
			id, source_path, download_id, coalesce(wanted_item_id::text, ''), media_format,
			title, author_name, coalesce(size_bytes, 0), reason, status, decision,
			destination_path, metadata, created_at, updated_at, resolved_at
		from import_reviews
	`
	if len(where) > 0 {
		sqlText += " where " + strings.Join(where, " and ")
	}
	sqlText += " order by created_at desc limit $" + strconv.Itoa(len(args))

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []ImportReview
	for rows.Next() {
		review, err := scanImportReview(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (s *Store) GetImportReview(ctx context.Context, id string) (ImportReview, error) {
	if !s.Configured() {
		return ImportReview{}, errors.New("library store is unavailable")
	}
	if strings.TrimSpace(id) == "" {
		return ImportReview{}, errors.New("review id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		select
			id, source_path, download_id, coalesce(wanted_item_id::text, ''), media_format,
			title, author_name, coalesce(size_bytes, 0), reason, status, decision,
			destination_path, metadata, created_at, updated_at, resolved_at
		from import_reviews
		where id = $1
	`, strings.TrimSpace(id))
	return scanImportReview(row)
}

func (s *Store) ResolveImportReview(ctx context.Context, id string, status string, decision string, destinationPath string, wantedID string) (ImportReview, error) {
	if !s.Configured() {
		return ImportReview{}, errors.New("library store is unavailable")
	}
	if strings.TrimSpace(id) == "" {
		return ImportReview{}, errors.New("review id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		update import_reviews set
			status = $2,
			decision = $3,
			destination_path = $4,
			wanted_item_id = coalesce(nullif($5, '')::uuid, wanted_item_id),
			updated_at = now(),
			resolved_at = now()
		where id = $1
		returning
			id, source_path, download_id, coalesce(wanted_item_id::text, ''), media_format,
			title, author_name, coalesce(size_bytes, 0), reason, status, decision,
			destination_path, metadata, created_at, updated_at, resolved_at
	`, strings.TrimSpace(id), strings.TrimSpace(status), strings.TrimSpace(decision), strings.TrimSpace(destinationPath), strings.TrimSpace(wantedID))
	return scanImportReview(row)
}

func (s *Store) findPendingImportReviewBySource(ctx context.Context, sourcePath string) (ImportReview, error) {
	row := s.db.QueryRowContext(ctx, `
		select
			id, source_path, download_id, coalesce(wanted_item_id::text, ''), media_format,
			title, author_name, coalesce(size_bytes, 0), reason, status, decision,
			destination_path, metadata, created_at, updated_at, resolved_at
		from import_reviews
		where source_path = $1 and status = 'pending'
		order by created_at desc
		limit 1
	`, strings.TrimSpace(sourcePath))
	return scanImportReview(row)
}

type fileScanner interface {
	Scan(dest ...any) error
}

func scanFiles(rows *sql.Rows) ([]FileRecord, error) {
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

func fileIdentifierWhere(ids []string, paths []string) ([]string, []any) {
	args := []any{}
	where := []string{}
	idPlaceholders := placeholdersForValues(&args, compactStrings(ids))
	if len(idPlaceholders) > 0 {
		where = append(where, "id::text in ("+strings.Join(idPlaceholders, ",")+")")
	}
	pathPlaceholders := placeholdersForValues(&args, compactStrings(paths))
	if len(pathPlaceholders) > 0 {
		where = append(where, "path in ("+strings.Join(pathPlaceholders, ",")+")")
	}
	return where, args
}

func placeholdersForValues(args *[]any, values []string) []string {
	placeholders := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		*args = append(*args, value)
		placeholders = append(placeholders, "$"+strconv.Itoa(len(*args)))
	}
	return placeholders
}

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		compacted = append(compacted, value)
	}
	return compacted
}

func scanImportReview(row fileScanner) (ImportReview, error) {
	var review ImportReview
	var raw []byte
	var resolvedAt sql.NullTime
	if err := row.Scan(
		&review.ID, &review.SourcePath, &review.DownloadID, &review.WantedID, &review.MediaFormat,
		&review.Title, &review.AuthorName, &review.SizeBytes, &review.Reason, &review.Status, &review.Decision,
		&review.DestinationPath, &raw, &review.CreatedAt, &review.UpdatedAt, &resolvedAt,
	); err != nil {
		return ImportReview{}, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &review.Metadata)
	}
	if review.Metadata == nil {
		review.Metadata = map[string]any{}
	}
	if resolvedAt.Valid {
		value := resolvedAt.Time.UTC()
		review.ResolvedAt = &value
	}
	return review, nil
}

func nullableInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
