package wanted

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
			wi.source_key, wi.last_search_at, wi.created_at, wi.updated_at
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
			wi.source_key, wi.last_search_at, wi.created_at, wi.updated_at
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
	var lastSearchAt sql.NullTime
	if err := row.Scan(
		&item.ID, &workID, &editionID, &item.Title, &item.AuthorName, &coverURL,
		&item.Format, &item.QualityProfile, &item.Status, &sourceProvider,
		&sourceKey, &lastSearchAt, &item.CreatedAt, &item.UpdatedAt,
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

func nullableInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
