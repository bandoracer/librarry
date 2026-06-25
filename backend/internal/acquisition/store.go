package acquisition

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type DownloadStore interface {
	UpsertDownloads(ctx context.Context, downloads []DownloadStatus) error
	ListDownloads(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error)
	MarkDownloadsDeleted(ctx context.Context, ids []string) error
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
		lastSeenAt := download.LastSeenAt
		if lastSeenAt == nil {
			now := time.Now().UTC()
			lastSeenAt = &now
		}
		if _, err := s.db.ExecContext(ctx, `
			insert into downloads (
				client, external_id, name, category, save_path, state, progress, tags,
				size_bytes, downloaded_bytes, uploaded_bytes, download_rate, upload_rate,
				eta_seconds, ratio, seeders, peers, added_at, completed_at, last_seen_at,
				updated_at
			) values (
				'qBittorrent', $1, $2, $3, $4, $5, $6, $7,
				$8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, now()
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
				last_seen_at = excluded.last_seen_at,
				updated_at = now()
		`, download.ID, download.Name, download.Category, download.SavePath, download.State, download.Progress, strings.Join(download.Tags, ","),
			download.SizeBytes, download.DownloadedBytes, download.UploadedBytes, download.DownloadRate, download.UploadRate,
			download.ETASeconds, download.Ratio, download.Seeders, download.Peers, download.AddedAt, download.CompletedAt, lastSeenAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLDownloadStore) ListDownloads(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		select
			external_id, name, state, progress, save_path, category, tags,
			size_bytes, downloaded_bytes, uploaded_bytes, download_rate, upload_rate,
			eta_seconds, ratio, seeders, peers, added_at, completed_at, last_seen_at
		from downloads
		where client = 'qBittorrent'
			and external_id is not null
			and ($1 = '' or tags like '%' || $1 || '%')
			and ($2 = '' or category = $2)
		order by coalesce(last_seen_at, updated_at, created_at) desc
		limit 200
	`, strings.TrimSpace(query.Tag), strings.TrimSpace(query.Category))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var downloads []DownloadStatus
	for rows.Next() {
		var item DownloadStatus
		var tags string
		var addedAt, completedAt, lastSeenAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.Name, &item.State, &item.Progress, &item.SavePath, &item.Category, &tags,
			&item.SizeBytes, &item.DownloadedBytes, &item.UploadedBytes, &item.DownloadRate, &item.UploadRate,
			&item.ETASeconds, &item.Ratio, &item.Seeders, &item.Peers, &addedAt, &completedAt, &lastSeenAt,
		); err != nil {
			return nil, err
		}
		item.Tags = splitTags(tags)
		item.AddedAt = nullableTime(addedAt)
		item.CompletedAt = nullableTime(completedAt)
		item.LastSeenAt = nullableTime(lastSeenAt)
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
			where client = 'qBittorrent' and external_id = $1
		`, id); err != nil {
			return err
		}
	}
	return nil
}

func nullableTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}
