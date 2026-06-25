package wanted

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/metadata"
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

func (s *Store) CreateWanted(ctx context.Context, request CreateRequest) (WantedItem, error) {
	if !s.Configured() {
		return WantedItem{}, errors.New("wanted store is unavailable")
	}
	format := wantedFormat(request)
	result := request.Result
	if strings.TrimSpace(result.Work.Title) == "" {
		return WantedItem{}, errors.New("work title is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WantedItem{}, err
	}
	defer tx.Rollback()

	raw, _ := json.Marshal(result)
	workID, err := s.upsertWork(ctx, tx, result, raw)
	if err != nil {
		return WantedItem{}, err
	}
	authorName, err := s.upsertPrimaryAuthor(ctx, tx, result, workID, raw)
	if err != nil {
		return WantedItem{}, err
	}
	editionID, err := s.upsertEdition(ctx, tx, result, workID, format, raw)
	if err != nil {
		return WantedItem{}, err
	}

	sourceProvider := strings.TrimSpace(result.Provider)
	sourceKey := firstNonEmpty(result.Edition.ID, result.Work.ID, result.RawSourceKey)
	qualityProfile := strings.TrimSpace(request.QualityProfile)
	if qualityProfile == "" {
		qualityProfile = "standard"
	}

	var wantedID string
	err = tx.QueryRowContext(ctx, `
		insert into wanted_items (
			work_id, edition_id, wanted_format, quality_profile, status,
			title, author_name, cover_url, metadata_provider, source_key
		) values ($1, $2, $3, $4, 'wanted', $5, $6, $7, $8, $9)
		on conflict (metadata_provider, source_key, wanted_format)
			where metadata_provider <> '' and source_key <> ''
		do update set
			quality_profile = excluded.quality_profile,
			status = case when wanted_items.status = 'removed' then 'wanted' else wanted_items.status end,
			title = excluded.title,
			author_name = excluded.author_name,
			cover_url = excluded.cover_url,
			updated_at = now()
		returning id
	`, workID, editionID, format, qualityProfile, result.Work.Title, authorName, result.Work.CoverURL, sourceProvider, sourceKey).Scan(&wantedID)
	if err != nil {
		return WantedItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return WantedItem{}, err
	}
	return s.GetWanted(ctx, wantedID)
}

func (s *Store) ListWanted(ctx context.Context, status string) ([]WantedItem, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	args := []any{}
	where := ""
	if strings.TrimSpace(status) != "" {
		args = append(args, strings.TrimSpace(status))
		where = "where wi.status = $1"
	}
	rows, err := s.db.QueryContext(ctx, `
		select
			wi.id, wi.work_id, wi.edition_id, coalesce(nullif(wi.title, ''), w.title),
			coalesce(nullif(wi.author_name, ''), ''), coalesce(nullif(wi.cover_url, ''), w.cover_url),
			wi.wanted_format, wi.quality_profile, wi.status, wi.metadata_provider,
			wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
			wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at
		from wanted_items wi
		left join works w on w.id = wi.work_id
		`+where+`
		order by wi.created_at desc
		limit 200
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []WantedItem
	for rows.Next() {
		item, err := scanWanted(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetWanted(ctx context.Context, id string) (WantedItem, error) {
	if !s.Configured() {
		return WantedItem{}, errors.New("wanted store is unavailable")
	}
	row := s.db.QueryRowContext(ctx, `
		select
			wi.id, wi.work_id, wi.edition_id, coalesce(nullif(wi.title, ''), w.title),
			coalesce(nullif(wi.author_name, ''), ''), coalesce(nullif(wi.cover_url, ''), w.cover_url),
			wi.wanted_format, wi.quality_profile, wi.status, wi.metadata_provider,
			wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
			wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at
		from wanted_items wi
		left join works w on w.id = wi.work_id
		where wi.id = $1
	`, id)
	return scanWanted(row)
}

func (s *Store) UpsertReleaseDecisions(ctx context.Context, wantedID string, decisions []ReleaseDecision) ([]ReleaseDecision, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stored := make([]ReleaseDecision, 0, len(decisions))
	for _, decision := range decisions {
		categories := strings.Join(decision.Categories, ",")
		var id string
		var createdAt, searchedAt time.Time
		err := tx.QueryRowContext(ctx, `
			insert into releases (
				wanted_item_id, source_id, info_hash, indexer, title, protocol,
				download_url, info_url, size_bytes, seeders, leechers, categories,
				score, rejected_reason, approved, published_at, searched_at
			) values (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11, $12,
				$13, $14, $15, nullif($16, '0001-01-01T00:00:00Z')::timestamptz, now()
			)
			on conflict (wanted_item_id, source_id) where source_id <> '' do update set
				info_hash = excluded.info_hash,
				indexer = excluded.indexer,
				title = excluded.title,
				protocol = excluded.protocol,
				download_url = excluded.download_url,
				info_url = excluded.info_url,
				size_bytes = excluded.size_bytes,
				seeders = excluded.seeders,
				leechers = excluded.leechers,
				categories = excluded.categories,
				score = excluded.score,
				rejected_reason = excluded.rejected_reason,
				approved = excluded.approved,
				published_at = excluded.published_at,
				searched_at = now()
			returning id, searched_at, created_at
		`, wantedID, decision.SourceID, decision.InfoHash, decision.Indexer, decision.Title, decision.Protocol,
			decision.DownloadURL, decision.InfoURL, nullableInt64(decision.SizeBytes), decision.Seeders, decision.Leechers, categories,
			decision.Score, decision.RejectedReason, decision.Approved, decision.PublishedAt.Format(time.RFC3339)).Scan(&id, &searchedAt, &createdAt)
		if err != nil {
			return nil, err
		}
		decision.ID = id
		decision.WantedItemID = wantedID
		decision.SearchedAt = searchedAt
		decision.CreatedAt = createdAt
		stored = append(stored, decision)
	}
	if _, err := tx.ExecContext(ctx, `update wanted_items set last_search_at = now(), updated_at = now() where id = $1`, wantedID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Store) ListReleaseDecisions(ctx context.Context, wantedID string) ([]ReleaseDecision, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select
			id, wanted_item_id, source_id, coalesce(info_hash, ''), indexer, title,
			protocol, download_url, coalesce(info_url, ''), coalesce(size_bytes, 0),
			coalesce(seeders, 0), leechers, categories, score, approved,
			coalesce(rejected_reason, ''), published_at, searched_at, created_at
		from releases
		where wanted_item_id = $1
		order by approved desc, score desc, seeders desc, created_at desc
		limit 100
	`, wantedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []ReleaseDecision
	for rows.Next() {
		release, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		releases = append(releases, release)
	}
	return releases, rows.Err()
}

func (s *Store) GetReleaseDecision(ctx context.Context, wantedID string, releaseID string) (ReleaseDecision, error) {
	releases, err := s.ListReleaseDecisions(ctx, wantedID)
	if err != nil {
		return ReleaseDecision{}, err
	}
	for _, release := range releases {
		if release.ID == releaseID || release.SourceID == releaseID {
			return release, nil
		}
	}
	return ReleaseDecision{}, sql.ErrNoRows
}

func (s *Store) ListDueWanted(ctx context.Context, limit int, minInterval time.Duration, force bool) ([]WantedItem, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if minInterval <= 0 {
		minInterval = 6 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-minInterval)
	rows, err := s.db.QueryContext(ctx, `
		select
			wi.id, wi.work_id, wi.edition_id, coalesce(nullif(wi.title, ''), w.title),
			coalesce(nullif(wi.author_name, ''), ''), coalesce(nullif(wi.cover_url, ''), w.cover_url),
			wi.wanted_format, wi.quality_profile, wi.status, wi.metadata_provider,
			wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
			wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at
		from wanted_items wi
		left join works w on w.id = wi.work_id
		where wi.status = 'wanted'
			and ($1::boolean or wi.last_search_at is null or wi.last_search_at <= $2)
		order by coalesce(wi.last_search_at, 'epoch'::timestamptz), wi.created_at
		limit $3
	`, force, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []WantedItem
	for rows.Next() {
		item, err := scanWanted(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListUpgradeWanted(ctx context.Context, ids []string, limit int, minInterval time.Duration, force bool) ([]WantedItem, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if minInterval <= 0 {
		minInterval = 12 * time.Hour
	}
	cleanIDs := compactStrings(ids)
	args := []any{force, time.Now().UTC().Add(-minInterval)}
	where := []string{
		"wi.status in ('grabbed', 'imported')",
		"($1::boolean or wi.last_upgrade_search_at is null or wi.last_upgrade_search_at <= $2)",
	}
	if len(cleanIDs) > 0 {
		for _, id := range cleanIDs {
			args = append(args, id)
		}
		start := len(args) - len(cleanIDs) + 1
		var placeholders []string
		for i := start; i <= len(args); i++ {
			placeholders = append(placeholders, "$"+strconv.Itoa(i))
		}
		where = append(where, "wi.id::text in ("+strings.Join(placeholders, ",")+")")
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `
		select
			wi.id, wi.work_id, wi.edition_id, coalesce(nullif(wi.title, ''), w.title),
			coalesce(nullif(wi.author_name, ''), ''), coalesce(nullif(wi.cover_url, ''), w.cover_url),
			wi.wanted_format, wi.quality_profile, wi.status, wi.metadata_provider,
			wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
			wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at
		from wanted_items wi
		left join works w on w.id = wi.work_id
		where `+strings.Join(where, " and ")+`
		order by coalesce(wi.last_upgrade_search_at, 'epoch'::timestamptz), wi.created_at
		limit $`+strconv.Itoa(len(args))+`
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []WantedItem
	for rows.Next() {
		item, err := scanWanted(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) MarkWantedStatus(ctx context.Context, wantedID string, status string) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return errors.New("wanted status is required")
	}
	_, err := s.db.ExecContext(ctx, `update wanted_items set status = $2, updated_at = now() where id = $1`, wantedID, status)
	return err
}

func (s *Store) MarkWantedCurrentRelease(ctx context.Context, wantedID string, release ReleaseDecision) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(wantedID) == "" || strings.TrimSpace(release.ID) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		update wanted_items
		set current_release_id = nullif($2, '')::uuid,
			current_release_score = $3,
			updated_at = now()
		where id = $1
	`, wantedID, release.ID, release.Score)
	return err
}

func (s *Store) MarkWantedUpgradeSearched(ctx context.Context, wantedID string) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(wantedID) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `update wanted_items set last_upgrade_search_at = now(), updated_at = now() where id = $1`, wantedID)
	return err
}

func (s *Store) StartMonitorRun(ctx context.Context, trigger string) (MonitorRun, error) {
	if !s.Configured() {
		return MonitorRun{}, errors.New("wanted store is unavailable")
	}
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = "manual"
	}
	row := s.db.QueryRowContext(ctx, `
		insert into monitor_runs(trigger, status)
		values ($1, 'running')
		returning
			id, trigger, status, wanted_checked, releases_found, approved_count,
			rejected_count, grabbed_count, error_count, message, started_at, finished_at
	`, trigger)
	return scanMonitorRun(row)
}

func (s *Store) FinishMonitorRun(ctx context.Context, run MonitorRun) (MonitorRun, error) {
	if !s.Configured() {
		return MonitorRun{}, errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(run.Status) == "" {
		run.Status = "completed"
	}
	row := s.db.QueryRowContext(ctx, `
		update monitor_runs set
			status = $2,
			wanted_checked = $3,
			releases_found = $4,
			approved_count = $5,
			rejected_count = $6,
			grabbed_count = $7,
			error_count = $8,
			message = $9,
			finished_at = now()
		where id = $1
		returning
			id, trigger, status, wanted_checked, releases_found, approved_count,
			rejected_count, grabbed_count, error_count, message, started_at, finished_at
	`, run.ID, run.Status, run.WantedChecked, run.ReleasesFound, run.ApprovedCount,
		run.RejectedCount, run.GrabbedCount, run.ErrorCount, run.Message)
	finished, err := scanMonitorRun(row)
	if err != nil {
		return MonitorRun{}, err
	}
	finished.Items = run.Items
	return finished, nil
}

func (s *Store) InsertHistoryEvent(ctx context.Context, event HistoryEvent) (HistoryEvent, error) {
	if !s.Configured() {
		return HistoryEvent{}, errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(event.Severity) == "" {
		event.Severity = "info"
	}
	if event.Data == nil {
		event.Data = map[string]any{}
	}
	raw, err := json.Marshal(event.Data)
	if err != nil {
		return HistoryEvent{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		insert into history_events(event_type, entity_type, entity_id, severity, message, data)
		values ($1, $2, $3, $4, $5, $6::jsonb)
		returning id, event_type, entity_type, entity_id, severity, message, data, created_at
	`, event.EventType, event.EntityType, event.EntityID, event.Severity, event.Message, string(raw))
	return scanHistoryEvent(row)
}

func (s *Store) ListHistory(ctx context.Context, query HistoryQuery) ([]HistoryEvent, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	limit := query.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		select id, event_type, entity_type, entity_id, severity, message, data, created_at
		from history_events
		order by created_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []HistoryEvent
	for rows.Next() {
		event, err := scanHistoryEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) StartFeedSyncRun(ctx context.Context, trigger string) (FeedSyncRun, error) {
	if !s.Configured() {
		return FeedSyncRun{}, errors.New("wanted store is unavailable")
	}
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = "manual"
	}
	row := s.db.QueryRowContext(ctx, `
		insert into feed_sync_runs(trigger, status)
		values ($1, 'running')
		returning
			id, trigger, status, releases_seen, matched_count, approved_count,
			rejected_count, grabbed_count, error_count, message, started_at, finished_at
	`, trigger)
	return scanFeedSyncRun(row)
}

func (s *Store) FinishFeedSyncRun(ctx context.Context, run FeedSyncRun) (FeedSyncRun, error) {
	if !s.Configured() {
		return FeedSyncRun{}, errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(run.Status) == "" {
		run.Status = "completed"
	}
	row := s.db.QueryRowContext(ctx, `
		update feed_sync_runs set
			status = $2,
			releases_seen = $3,
			matched_count = $4,
			approved_count = $5,
			rejected_count = $6,
			grabbed_count = $7,
			error_count = $8,
			message = $9,
			finished_at = now()
		where id = $1
		returning
			id, trigger, status, releases_seen, matched_count, approved_count,
			rejected_count, grabbed_count, error_count, message, started_at, finished_at
	`, run.ID, run.Status, run.ReleasesSeen, run.MatchedCount, run.ApprovedCount,
		run.RejectedCount, run.GrabbedCount, run.ErrorCount, run.Message)
	finished, err := scanFeedSyncRun(row)
	if err != nil {
		return FeedSyncRun{}, err
	}
	finished.Matches = run.Matches
	return finished, nil
}

func (s *Store) StartFailedDownloadRun(ctx context.Context, trigger string) (FailedDownloadRun, error) {
	if !s.Configured() {
		return FailedDownloadRun{}, errors.New("wanted store is unavailable")
	}
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = "manual"
	}
	row := s.db.QueryRowContext(ctx, `
		insert into failed_download_runs(trigger, status)
		values ($1, 'running')
		returning
			id, trigger, status, downloads_checked, failed_count, replacements_found,
			grabbed_count, removed_count, error_count, message, started_at, finished_at
	`, trigger)
	return scanFailedDownloadRun(row)
}

func (s *Store) FinishFailedDownloadRun(ctx context.Context, run FailedDownloadRun) (FailedDownloadRun, error) {
	if !s.Configured() {
		return FailedDownloadRun{}, errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(run.Status) == "" {
		run.Status = "completed"
	}
	row := s.db.QueryRowContext(ctx, `
		update failed_download_runs set
			status = $2,
			downloads_checked = $3,
			failed_count = $4,
			replacements_found = $5,
			grabbed_count = $6,
			removed_count = $7,
			error_count = $8,
			message = $9,
			finished_at = now()
		where id = $1
		returning
			id, trigger, status, downloads_checked, failed_count, replacements_found,
			grabbed_count, removed_count, error_count, message, started_at, finished_at
	`, run.ID, run.Status, run.DownloadsChecked, run.FailedCount, run.ReplacementsFound,
		run.GrabbedCount, run.RemovedCount, run.ErrorCount, run.Message)
	finished, err := scanFailedDownloadRun(row)
	if err != nil {
		return FailedDownloadRun{}, err
	}
	finished.Items = run.Items
	return finished, nil
}

func (s *Store) StartUpgradeRun(ctx context.Context, trigger string) (UpgradeRun, error) {
	if !s.Configured() {
		return UpgradeRun{}, errors.New("wanted store is unavailable")
	}
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = "manual"
	}
	row := s.db.QueryRowContext(ctx, `
		insert into upgrade_runs(trigger, status)
		values ($1, 'running')
		returning
			id, trigger, status, wanted_checked, releases_found, upgrade_count,
			grabbed_count, error_count, message, started_at, finished_at
	`, trigger)
	return scanUpgradeRun(row)
}

func (s *Store) FinishUpgradeRun(ctx context.Context, run UpgradeRun) (UpgradeRun, error) {
	if !s.Configured() {
		return UpgradeRun{}, errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(run.Status) == "" {
		run.Status = "completed"
	}
	row := s.db.QueryRowContext(ctx, `
		update upgrade_runs set
			status = $2,
			wanted_checked = $3,
			releases_found = $4,
			upgrade_count = $5,
			grabbed_count = $6,
			error_count = $7,
			message = $8,
			finished_at = now()
		where id = $1
		returning
			id, trigger, status, wanted_checked, releases_found, upgrade_count,
			grabbed_count, error_count, message, started_at, finished_at
	`, run.ID, run.Status, run.WantedChecked, run.ReleasesFound, run.UpgradeCount,
		run.GrabbedCount, run.ErrorCount, run.Message)
	finished, err := scanUpgradeRun(row)
	if err != nil {
		return UpgradeRun{}, err
	}
	finished.Items = run.Items
	return finished, nil
}

func (s *Store) UpsertFeedReleases(ctx context.Context, releases []acquisition.Release) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	for _, release := range releases {
		sourceID := strings.TrimSpace(release.ID)
		if sourceID == "" {
			sourceID = firstNonEmpty(release.InfoHash, release.Title)
		}
		categories := strings.Join(release.Categories, ",")
		if _, err := s.db.ExecContext(ctx, `
			insert into feed_releases (
				source_id, info_hash, indexer, title, protocol, download_url,
				info_url, size_bytes, seeders, leechers, categories, published_at,
				last_seen_at
			) values (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11, nullif($12, '0001-01-01T00:00:00Z')::timestamptz,
				now()
			)
			on conflict (source_id) where source_id <> '' do update set
				info_hash = excluded.info_hash,
				indexer = excluded.indexer,
				title = excluded.title,
				protocol = excluded.protocol,
				download_url = excluded.download_url,
				info_url = excluded.info_url,
				size_bytes = excluded.size_bytes,
				seeders = excluded.seeders,
				leechers = excluded.leechers,
				categories = excluded.categories,
				published_at = excluded.published_at,
				last_seen_at = now()
		`, sourceID, release.InfoHash, release.Indexer, release.Title, release.Protocol, release.DownloadURL,
			release.InfoURL, nullableInt64(release.SizeBytes), release.Seeders, release.Leechers, categories,
			release.PublishedAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) upsertWork(ctx context.Context, tx *sql.Tx, result metadata.SearchResult, raw []byte) (string, error) {
	if id, ok, err := lookupProviderEntity(ctx, tx, result.Provider, result.Work.ID); err != nil || ok {
		if ok {
			_, _ = tx.ExecContext(ctx, `update works set title = $1, sort_title = $2, cover_url = $3, updated_at = now() where id = $4`, result.Work.Title, sortValue(result.Work.Title), result.Work.CoverURL, id)
		}
		return id, err
	}
	var id string
	if err := tx.QueryRowContext(ctx, `
		insert into works(title, sort_title, first_publish_year, description, cover_url)
		values ($1, $2, nullif($3, 0), $4, $5)
		returning id
	`, result.Work.Title, sortValue(result.Work.Title), result.Work.FirstPublishYear, result.Work.Description, result.Work.CoverURL).Scan(&id); err != nil {
		return "", err
	}
	return id, insertProviderRecord(ctx, tx, result.Provider, result.Work.ID, "work", id, raw, result.Score)
}

func (s *Store) upsertPrimaryAuthor(ctx context.Context, tx *sql.Tx, result metadata.SearchResult, workID string, raw []byte) (string, error) {
	if len(result.Work.Authors) == 0 || strings.TrimSpace(result.Work.Authors[0].Name) == "" {
		return "", nil
	}
	author := result.Work.Authors[0]
	authorKey := firstNonEmpty(author.ID, result.Provider+":author:"+normalizeText(author.Name))
	id, ok, err := lookupProviderEntity(ctx, tx, result.Provider, authorKey)
	if err != nil {
		return "", err
	}
	if !ok {
		if err := tx.QueryRowContext(ctx, `
			insert into authors(canonical_name, sort_name)
			values ($1, $2)
			returning id
		`, author.Name, sortValue(author.Name)).Scan(&id); err != nil {
			return "", err
		}
		if err := insertProviderRecord(ctx, tx, result.Provider, authorKey, "author", id, raw, result.Score); err != nil {
			return "", err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		insert into work_authors(work_id, author_id, role)
		values ($1, $2, 'author')
		on conflict do nothing
	`, workID, id); err != nil {
		return "", err
	}
	return author.Name, nil
}

func (s *Store) upsertEdition(ctx context.Context, tx *sql.Tx, result metadata.SearchResult, workID string, format string, raw []byte) (string, error) {
	edition := result.Edition
	editionKey := firstNonEmpty(edition.ID, result.Provider+":edition:"+result.Work.ID+":"+format)
	if id, ok, err := lookupProviderEntity(ctx, tx, result.Provider, editionKey); err != nil || ok {
		return id, err
	}
	title := firstNonEmpty(edition.Title, result.Work.Title)
	mediaFormat := format
	if mediaFormat == "" {
		mediaFormat = "unknown"
	}
	var id string
	if err := tx.QueryRowContext(ctx, `
		insert into editions(work_id, title, media_format, language, publisher, published_date, asin)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id
	`, workID, title, mediaFormat, edition.Language, edition.Publisher, edition.PublishedDate, edition.ASIN).Scan(&id); err != nil {
		return "", err
	}
	if err := insertProviderRecord(ctx, tx, result.Provider, editionKey, "edition", id, raw, result.Score); err != nil {
		return "", err
	}
	for _, isbn := range edition.ISBNs {
		isbn = strings.TrimSpace(isbn)
		if isbn == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			insert into edition_identifiers(edition_id, kind, value)
			values ($1, 'isbn', $2)
			on conflict do nothing
		`, id, isbn); err != nil {
			return "", err
		}
	}
	return id, nil
}

type wantedScanner interface {
	Scan(dest ...any) error
}

func scanWanted(row wantedScanner) (WantedItem, error) {
	var item WantedItem
	var workID, editionID, coverURL, sourceProvider, sourceKey sql.NullString
	var lastSearchAt, lastUpgradeSearchAt sql.NullTime
	if err := row.Scan(
		&item.ID, &workID, &editionID, &item.Title, &item.AuthorName, &coverURL,
		&item.Format, &item.QualityProfile, &item.Status, &sourceProvider,
		&sourceKey, &item.CurrentReleaseID, &item.CurrentReleaseScore,
		&lastSearchAt, &lastUpgradeSearchAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return WantedItem{}, err
	}
	item.WorkID = workID.String
	item.EditionID = editionID.String
	item.CoverURL = coverURL.String
	item.SourceProvider = sourceProvider.String
	item.SourceKey = sourceKey.String
	if lastSearchAt.Valid {
		value := lastSearchAt.Time.UTC()
		item.LastSearchAt = &value
	}
	if lastUpgradeSearchAt.Valid {
		value := lastUpgradeSearchAt.Time.UTC()
		item.LastUpgradeSearchAt = &value
	}
	return item, nil
}

func scanRelease(rows *sql.Rows) (ReleaseDecision, error) {
	var release ReleaseDecision
	var categories string
	var publishedAt sql.NullTime
	if err := rows.Scan(
		&release.ID, &release.WantedItemID, &release.SourceID, &release.InfoHash,
		&release.Indexer, &release.Title, &release.Protocol, &release.DownloadURL,
		&release.InfoURL, &release.SizeBytes, &release.Seeders, &release.Leechers,
		&categories, &release.Score, &release.Approved, &release.RejectedReason,
		&publishedAt, &release.SearchedAt, &release.CreatedAt,
	); err != nil {
		return ReleaseDecision{}, err
	}
	release.Categories = splitComma(categories)
	if publishedAt.Valid {
		release.PublishedAt = publishedAt.Time.UTC()
	}
	return release, nil
}

func scanMonitorRun(row wantedScanner) (MonitorRun, error) {
	var run MonitorRun
	var finishedAt sql.NullTime
	if err := row.Scan(
		&run.ID, &run.Trigger, &run.Status, &run.WantedChecked, &run.ReleasesFound,
		&run.ApprovedCount, &run.RejectedCount, &run.GrabbedCount, &run.ErrorCount,
		&run.Message, &run.StartedAt, &finishedAt,
	); err != nil {
		return MonitorRun{}, err
	}
	if finishedAt.Valid {
		value := finishedAt.Time.UTC()
		run.FinishedAt = &value
	}
	return run, nil
}

func scanFeedSyncRun(row wantedScanner) (FeedSyncRun, error) {
	var run FeedSyncRun
	var finishedAt sql.NullTime
	if err := row.Scan(
		&run.ID, &run.Trigger, &run.Status, &run.ReleasesSeen, &run.MatchedCount,
		&run.ApprovedCount, &run.RejectedCount, &run.GrabbedCount, &run.ErrorCount,
		&run.Message, &run.StartedAt, &finishedAt,
	); err != nil {
		return FeedSyncRun{}, err
	}
	if finishedAt.Valid {
		value := finishedAt.Time.UTC()
		run.FinishedAt = &value
	}
	return run, nil
}

func scanFailedDownloadRun(row wantedScanner) (FailedDownloadRun, error) {
	var run FailedDownloadRun
	var finishedAt sql.NullTime
	if err := row.Scan(
		&run.ID, &run.Trigger, &run.Status, &run.DownloadsChecked, &run.FailedCount,
		&run.ReplacementsFound, &run.GrabbedCount, &run.RemovedCount, &run.ErrorCount,
		&run.Message, &run.StartedAt, &finishedAt,
	); err != nil {
		return FailedDownloadRun{}, err
	}
	if finishedAt.Valid {
		value := finishedAt.Time.UTC()
		run.FinishedAt = &value
	}
	return run, nil
}

func scanUpgradeRun(row wantedScanner) (UpgradeRun, error) {
	var run UpgradeRun
	var finishedAt sql.NullTime
	if err := row.Scan(
		&run.ID, &run.Trigger, &run.Status, &run.WantedChecked, &run.ReleasesFound,
		&run.UpgradeCount, &run.GrabbedCount, &run.ErrorCount, &run.Message,
		&run.StartedAt, &finishedAt,
	); err != nil {
		return UpgradeRun{}, err
	}
	if finishedAt.Valid {
		value := finishedAt.Time.UTC()
		run.FinishedAt = &value
	}
	return run, nil
}

func scanHistoryEvent(row wantedScanner) (HistoryEvent, error) {
	var event HistoryEvent
	var raw []byte
	if err := row.Scan(
		&event.ID, &event.EventType, &event.EntityType, &event.EntityID,
		&event.Severity, &event.Message, &raw, &event.CreatedAt,
	); err != nil {
		return HistoryEvent{}, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &event.Data)
	}
	if event.Data == nil {
		event.Data = map[string]any{}
	}
	return event, nil
}

func lookupProviderEntity(ctx context.Context, tx *sql.Tx, provider string, key string) (string, bool, error) {
	if strings.TrimSpace(provider) == "" || strings.TrimSpace(key) == "" {
		return "", false, nil
	}
	var id string
	err := tx.QueryRowContext(ctx, `
		select entity_id::text from provider_records
		where provider = $1 and provider_key = $2 and entity_id is not null
	`, provider, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

func insertProviderRecord(ctx context.Context, tx *sql.Tx, provider string, key string, entityType string, entityID string, raw []byte, confidence float64) error {
	if strings.TrimSpace(provider) == "" || strings.TrimSpace(key) == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		insert into provider_records(provider, provider_key, entity_type, entity_id, raw, confidence)
		values ($1, $2, $3, $4, $5::jsonb, $6)
		on conflict (provider, provider_key) do update set
			entity_type = excluded.entity_type,
			entity_id = excluded.entity_id,
			raw = excluded.raw,
			confidence = excluded.confidence,
			fetched_at = now()
	`, provider, key, entityType, entityID, string(raw), confidence)
	return err
}

func wantedFormat(request CreateRequest) string {
	format := request.Format
	if format == "" {
		format = string(request.Result.Edition.Format)
	}
	return normalizeFormat(format)
}

func sortValue(value string) string {
	return normalizeText(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func splitComma(value string) []string {
	parts := strings.Split(value, ",")
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func compactStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func nullableInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
