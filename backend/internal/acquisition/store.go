package acquisition

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

type DownloadStore interface {
	UpsertDownloads(ctx context.Context, downloads []DownloadStatus) error
	ListDownloads(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error)
	MarkDownloadsDeleted(ctx context.Context, ids []string) error
	MarkDownloadFailed(ctx context.Context, id string, reason string) error
	MarkDownloadReplacement(ctx context.Context, id string, replacementID string) error
	MarkDownloadImported(ctx context.Context, id string, fileID string) error
	MarkDownloadImportError(ctx context.Context, id string, message string) error
}

type SQLDownloadStore struct {
	db *sql.DB
}

func NewSQLDownloadStore(db *sql.DB) *SQLDownloadStore {
	if db == nil {
		return nil
	}
	return &SQLDownloadStore{db: db}
}

func (s *SQLDownloadStore) UpsertDownloads(ctx context.Context, downloads []DownloadStatus) error {
	if s == nil || s.db == nil || len(downloads) == 0 {
		return nil
	}
	for _, download := range downloads {
		if strings.TrimSpace(download.ID) == "" {
			continue
		}
		client := strings.TrimSpace(download.Client)
		if client == "" {
			client = "qBittorrent"
		}
		lastSeenAt := download.LastSeenAt
		if lastSeenAt == nil {
			now := time.Now().UTC()
			lastSeenAt = &now
		}
		if _, err := s.db.ExecContext(ctx, `
			insert into downloads (
				client, external_id, name, category, save_path, state, progress, tags,
				size_bytes, downloaded_bytes, uploaded_bytes, download_rate, upload_rate,
				eta_seconds, ratio, seeders, peers, added_at, completed_at, last_activity_at,
				last_seen_at, import_status, failure_reason, failed_at, retry_count,
				replacement_external_id, updated_at
			) values (
				$1, $2, $3, $4, $5, $6, $7, $8,
				$9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
				$21, $22, $23, $24, $25, $26, now()
			)
			on conflict (client, external_id) where external_id is not null do update set
				name = excluded.name,
				category = excluded.category,
				save_path = excluded.save_path,
				state = excluded.state,
				progress = excluded.progress,
				tags = excluded.tags,
				size_bytes = excluded.size_bytes,
				downloaded_bytes = excluded.downloaded_bytes,
				uploaded_bytes = excluded.uploaded_bytes,
				download_rate = excluded.download_rate,
				upload_rate = excluded.upload_rate,
				eta_seconds = excluded.eta_seconds,
				ratio = excluded.ratio,
				seeders = excluded.seeders,
				peers = excluded.peers,
				added_at = coalesce(excluded.added_at, downloads.added_at),
				completed_at = coalesce(excluded.completed_at, downloads.completed_at),
				last_activity_at = coalesce(excluded.last_activity_at, downloads.last_activity_at),
				last_seen_at = excluded.last_seen_at,
				import_status = case
					when downloads.import_status in ('imported', 'error') then downloads.import_status
					when excluded.completed_at is not null or excluded.progress >= 1 then 'ready'
					else downloads.import_status
				end,
				failure_reason = case
					when excluded.failure_reason <> '' then excluded.failure_reason
					else downloads.failure_reason
				end,
				failed_at = coalesce(excluded.failed_at, downloads.failed_at),
				retry_count = greatest(downloads.retry_count, excluded.retry_count),
				replacement_external_id = case
					when excluded.replacement_external_id <> '' then excluded.replacement_external_id
					else downloads.replacement_external_id
				end,
				updated_at = now()
		`, client, download.ID, download.Name, download.Category, download.SavePath, download.State, download.Progress, strings.Join(download.Tags, ","),
			download.SizeBytes, download.DownloadedBytes, download.UploadedBytes, download.DownloadRate, download.UploadRate,
			download.ETASeconds, download.Ratio, download.Seeders, download.Peers, download.AddedAt, download.CompletedAt,
			download.LastActivityAt, lastSeenAt, importStatusForDownload(download), download.FailureReason, download.FailedAt,
			download.RetryCount, download.ReplacementID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLDownloadStore) ListDownloads(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	args := []any{}
	where := []string{"external_id is not null"}
	if client := strings.TrimSpace(query.Client); client != "" {
		args = append(args, client)
		where = append(where, "client = $"+strconv.Itoa(len(args)))
	}
	if tag := strings.TrimSpace(query.Tag); tag != "" {
		args = append(args, tag)
		where = append(where, "tags like '%' || $"+strconv.Itoa(len(args))+" || '%'")
	}
	if category := strings.TrimSpace(query.Category); category != "" {
		args = append(args, category)
		where = append(where, "category = $"+strconv.Itoa(len(args)))
	}
	ids := compactStrings(query.IDs)
	for _, id := range ids {
		args = append(args, id)
	}
	if len(ids) > 0 {
		start := len(args) - len(ids) + 1
		var placeholders []string
		for i := start; i <= len(args); i++ {
			placeholders = append(placeholders, "$"+strconv.Itoa(i))
		}
		if len(placeholders) > 0 {
			where = append(where, "external_id in ("+strings.Join(placeholders, ",")+")")
		}
	}
	sqlText := `
		select
			client, external_id, name, state, progress, save_path, category, tags,
			size_bytes, downloaded_bytes, uploaded_bytes, download_rate, upload_rate,
			eta_seconds, ratio, seeders, peers, added_at, completed_at, last_activity_at,
			last_seen_at, import_status, coalesce(imported_file_id::text, ''), imported_at,
			import_error, failure_reason, failed_at, retry_count, replacement_external_id
		from downloads
		where ` + strings.Join(where, " and ") + `
		order by coalesce(last_seen_at, updated_at, created_at) desc
		limit 200
	`
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var downloads []DownloadStatus
	for rows.Next() {
		var item DownloadStatus
		var tags string
		var addedAt, completedAt, lastActivityAt, lastSeenAt, importedAt, failedAt sql.NullTime
		if err := rows.Scan(
			&item.Client, &item.ID, &item.Name, &item.State, &item.Progress, &item.SavePath, &item.Category, &tags,
			&item.SizeBytes, &item.DownloadedBytes, &item.UploadedBytes, &item.DownloadRate, &item.UploadRate,
			&item.ETASeconds, &item.Ratio, &item.Seeders, &item.Peers, &addedAt, &completedAt,
			&lastActivityAt, &lastSeenAt, &item.ImportStatus, &item.ImportedFileID, &importedAt,
			&item.ImportError, &item.FailureReason, &failedAt, &item.RetryCount, &item.ReplacementID,
		); err != nil {
			return nil, err
		}
		item.Tags = splitTags(tags)
		item.AddedAt = nullableTime(addedAt)
		item.CompletedAt = nullableTime(completedAt)
		item.LastActivityAt = nullableTime(lastActivityAt)
		item.LastSeenAt = nullableTime(lastSeenAt)
		item.ImportedAt = nullableTime(importedAt)
		item.FailedAt = nullableTime(failedAt)
		downloads = append(downloads, item)
	}
	return downloads, rows.Err()
}

func (s *SQLDownloadStore) MarkDownloadsDeleted(ctx context.Context, ids []string) error {
	if s == nil || s.db == nil {
		return nil
	}
	for _, id := range compactStrings(ids) {
		if _, err := s.db.ExecContext(ctx, `
			update downloads
			set state = 'removed', last_seen_at = now(), updated_at = now()
			where external_id = $1
		`, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLDownloadStore) MarkDownloadFailed(ctx context.Context, id string, reason string) error {
	if s == nil || s.db == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		update downloads
		set failure_reason = $2,
			failed_at = coalesce(failed_at, now()),
			updated_at = now()
		where external_id = $1
	`, id, strings.TrimSpace(reason))
	return err
}

func (s *SQLDownloadStore) MarkDownloadReplacement(ctx context.Context, id string, replacementID string) error {
	if s == nil || s.db == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		update downloads
		set retry_count = retry_count + 1,
			replacement_external_id = $2,
			updated_at = now()
		where external_id = $1
	`, id, strings.TrimSpace(replacementID))
	return err
}

func (s *SQLDownloadStore) MarkDownloadImported(ctx context.Context, id string, fileID string) error {
	if s == nil || s.db == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		update downloads
		set import_status = 'imported',
			imported_file_id = nullif($2, '')::uuid,
			imported_at = now(),
			import_error = '',
			updated_at = now()
		where external_id = $1
	`, id, strings.TrimSpace(fileID))
	return err
}

func (s *SQLDownloadStore) MarkDownloadImportError(ctx context.Context, id string, message string) error {
	if s == nil || s.db == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		update downloads
		set import_status = 'error',
			import_error = $2,
			updated_at = now()
		where external_id = $1
	`, id, strings.TrimSpace(message))
	return err
}

func nullableTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func importStatusForDownload(download DownloadStatus) string {
	if strings.TrimSpace(download.ImportStatus) != "" {
		return strings.TrimSpace(download.ImportStatus)
	}
	if download.CompletedAt != nil || download.Progress >= 1 {
		return "ready"
	}
	return "pending"
}
