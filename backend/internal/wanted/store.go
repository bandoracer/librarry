package wanted

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
			title, author_name, cover_url, metadata_provider, source_key, tags
		) values ($1, $2, $3, $4, 'wanted', $5, $6, $7, $8, $9, $10)
		on conflict (metadata_provider, source_key, wanted_format)
			where metadata_provider <> '' and source_key <> ''
		do update set
			quality_profile = case
				when exists (
					select 1 from manual_overrides mo
					where mo.entity_type = 'wanted_item'
						and mo.entity_id = wanted_items.id
						and mo.field_name = 'quality_profile'
				) then wanted_items.quality_profile
				else excluded.quality_profile
			end,
			status = case when wanted_items.status = 'removed' then 'wanted' else wanted_items.status end,
			title = case
				when exists (
					select 1 from manual_overrides mo
					where mo.entity_type = 'wanted_item'
						and mo.entity_id = wanted_items.id
						and mo.field_name = 'title'
				) then wanted_items.title
				else excluded.title
			end,
			author_name = case
				when exists (
					select 1 from manual_overrides mo
					where mo.entity_type = 'wanted_item'
						and mo.entity_id = wanted_items.id
						and mo.field_name = 'author_name'
				) then wanted_items.author_name
				else excluded.author_name
			end,
			cover_url = case
				when exists (
					select 1 from manual_overrides mo
					where mo.entity_type = 'wanted_item'
						and mo.entity_id = wanted_items.id
						and mo.field_name = 'cover_url'
				) then wanted_items.cover_url
				else excluded.cover_url
			end,
			tags = case when excluded.tags <> '' then excluded.tags else wanted_items.tags end,
			updated_at = now()
		returning id
	`, workID, editionID, format, qualityProfile, result.Work.Title, authorName, result.Work.CoverURL, sourceProvider, sourceKey, intTagsString(request.Tags)).Scan(&wantedID)
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
			wi.wanted_format, wi.quality_profile, wi.status, wi.monitored, wi.metadata_provider,
			wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
			wi.tags, wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.attachWantedManualOverrides(ctx, items)
}

func (s *Store) ListQualityProfiles(ctx context.Context) ([]QualityProfile, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select
			id, name, media_format, min_score, cutoff_score, min_seeders,
			max_size_bytes, preferred_terms, required_terms, rejected_terms,
			preferred_score, upgrade_allowed, created_at, updated_at
		from quality_profiles
		order by name, media_format
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []QualityProfile
	for rows.Next() {
		profile, err := scanQualityProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *Store) GetQualityProfile(ctx context.Context, name string, format string) (QualityProfile, error) {
	if !s.Configured() {
		return QualityProfile{}, errors.New("wanted store is unavailable")
	}
	name = normalizeQualityProfile(name)
	format = normalizeProfileFormat(format)
	row := s.db.QueryRowContext(ctx, `
		select
			id, name, media_format, min_score, cutoff_score, min_seeders,
			max_size_bytes, preferred_terms, required_terms, rejected_terms,
			preferred_score, upgrade_allowed, created_at, updated_at
		from quality_profiles
		where name = $1 and media_format in ($2, 'any')
		order by case when media_format = $2 then 0 else 1 end
		limit 1
	`, name, format)
	return scanQualityProfile(row)
}

func (s *Store) UpsertQualityProfile(ctx context.Context, profile QualityProfile) (QualityProfile, error) {
	if !s.Configured() {
		return QualityProfile{}, errors.New("wanted store is unavailable")
	}
	profile = normalizeProfileForStorage(profile)
	if strings.TrimSpace(profile.Name) == "" {
		return QualityProfile{}, errors.New("quality profile name is required")
	}
	if strings.TrimSpace(profile.ID) != "" {
		row := s.db.QueryRowContext(ctx, `
			update quality_profiles set
				name = $2,
				media_format = $3,
				min_score = $4,
				cutoff_score = $5,
				min_seeders = $6,
				max_size_bytes = $7,
				preferred_terms = $8,
				required_terms = $9,
				rejected_terms = $10,
				preferred_score = $11,
				upgrade_allowed = $12,
				updated_at = now()
			where id::text = $1
			returning
				id, name, media_format, min_score, cutoff_score, min_seeders,
				max_size_bytes, preferred_terms, required_terms, rejected_terms,
				preferred_score, upgrade_allowed, created_at, updated_at
		`, profile.ID, profile.Name, profile.MediaFormat, profile.MinScore, profile.CutoffScore, profile.MinSeeders, profile.MaxSizeBytes,
			strings.Join(cleanTerms(profile.PreferredTerms), ","), strings.Join(cleanTerms(profile.RequiredTerms), ","),
			strings.Join(cleanTerms(profile.RejectedTerms), ","), profile.PreferredScore, profile.UpgradeAllowed)
		saved, err := scanQualityProfile(row)
		if err == nil {
			return saved, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return QualityProfile{}, err
		}
	}
	row := s.db.QueryRowContext(ctx, `
		insert into quality_profiles (
			name, media_format, min_score, cutoff_score, min_seeders, max_size_bytes,
			preferred_terms, required_terms, rejected_terms, preferred_score, upgrade_allowed
		) values (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11
		)
		on conflict (name, media_format) do update set
			min_score = excluded.min_score,
			cutoff_score = excluded.cutoff_score,
			min_seeders = excluded.min_seeders,
			max_size_bytes = excluded.max_size_bytes,
			preferred_terms = excluded.preferred_terms,
			required_terms = excluded.required_terms,
			rejected_terms = excluded.rejected_terms,
			preferred_score = excluded.preferred_score,
			upgrade_allowed = excluded.upgrade_allowed,
			updated_at = now()
		returning
			id, name, media_format, min_score, cutoff_score, min_seeders,
			max_size_bytes, preferred_terms, required_terms, rejected_terms,
			preferred_score, upgrade_allowed, created_at, updated_at
	`, profile.Name, profile.MediaFormat, profile.MinScore, profile.CutoffScore, profile.MinSeeders, profile.MaxSizeBytes,
		strings.Join(cleanTerms(profile.PreferredTerms), ","), strings.Join(cleanTerms(profile.RequiredTerms), ","),
		strings.Join(cleanTerms(profile.RejectedTerms), ","), profile.PreferredScore, profile.UpgradeAllowed)
	return scanQualityProfile(row)
}

func (s *Store) DeleteQualityProfile(ctx context.Context, idOrName string) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	idOrName = strings.TrimSpace(idOrName)
	if idOrName == "" {
		return errors.New("quality profile id is required")
	}
	result, err := s.db.ExecContext(ctx, `
		delete from quality_profiles
		where id::text = $1 or lower(name) = lower($1)
	`, idOrName)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpsertAuthorSubscription(ctx context.Context, subscription AuthorSubscription) (AuthorSubscription, error) {
	if !s.Configured() {
		return AuthorSubscription{}, errors.New("wanted store is unavailable")
	}
	subscription = normalizeAuthorSubscription(subscription)
	if strings.TrimSpace(subscription.AuthorName) == "" {
		return AuthorSubscription{}, errors.New("author name is required")
	}
	row := s.db.QueryRowContext(ctx, `
		insert into author_subscriptions (
			provider, provider_key, author_name, wanted_format, quality_profile,
			status, monitor_new_items, missing_book_policy, tags
		) values (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9
		)
		on conflict (provider, provider_key, wanted_format)
			where provider <> '' and provider_key <> ''
		do update set
			author_name = excluded.author_name,
			quality_profile = excluded.quality_profile,
			status = case when author_subscriptions.status = 'removed' then 'monitored' else author_subscriptions.status end,
			monitor_new_items = excluded.monitor_new_items,
			missing_book_policy = excluded.missing_book_policy,
			tags = case when excluded.tags <> '' then excluded.tags else author_subscriptions.tags end,
			updated_at = now()
		returning
			id, provider, provider_key, author_name, wanted_format, quality_profile,
			status, monitor_new_items, missing_book_policy, tags, last_sync_at, created_at, updated_at
	`, subscription.Provider, subscription.ProviderKey, subscription.AuthorName, subscription.Format,
		subscription.QualityProfile, subscription.Status, subscription.MonitorNewItems, subscription.MissingBookPolicy, intTagsString(subscription.Tags))
	return scanAuthorSubscription(row)
}

func (s *Store) ListAuthorSubscriptions(ctx context.Context, status string) ([]AuthorSubscription, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	args := []any{}
	where := ""
	if strings.TrimSpace(status) != "" && strings.TrimSpace(status) != "all" {
		args = append(args, strings.TrimSpace(status))
		where = "where status = $1"
	}
	rows, err := s.db.QueryContext(ctx, `
		select
			id, provider, provider_key, author_name, wanted_format, quality_profile,
			status, monitor_new_items, missing_book_policy, tags, last_sync_at, created_at, updated_at
		from author_subscriptions
		`+where+`
		order by author_name, wanted_format
		limit 500
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []AuthorSubscription
	for rows.Next() {
		subscription, err := scanAuthorSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}
	return subscriptions, rows.Err()
}

func (s *Store) UpdateAuthorSubscription(ctx context.Context, id string, request AuthorUpdateRequest) (AuthorSubscription, error) {
	if !s.Configured() {
		return AuthorSubscription{}, errors.New("wanted store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return AuthorSubscription{}, errors.New("author subscription id is required")
	}
	status := strings.TrimSpace(request.Status)
	if request.Monitored != nil {
		if *request.Monitored {
			status = "monitored"
		} else {
			status = "unmonitored"
		}
	}
	qualityProfile := strings.TrimSpace(request.QualityProfile)
	if qualityProfile != "" {
		qualityProfile = normalizeQualityProfile(qualityProfile)
	}
	monitorNewItems := sql.NullBool{}
	if request.MonitorNewItems != nil {
		monitorNewItems.Valid = true
		monitorNewItems.Bool = *request.MonitorNewItems
	}
	missingBookPolicy := sql.NullString{}
	if strings.TrimSpace(request.MissingBookPolicy) != "" {
		missingBookPolicy.Valid = true
		missingBookPolicy.String = normalizeAuthorMissingBookPolicy(request.MissingBookPolicy, true)
		monitorNewItems.Valid = true
		monitorNewItems.Bool = missingBookPolicy.String != "none"
	} else if request.MonitorNewItems != nil {
		missingBookPolicy.Valid = true
		missingBookPolicy.String = normalizeAuthorMissingBookPolicy("", *request.MonitorNewItems)
	}
	tags := sql.NullString{}
	if request.TagsSet {
		tags.Valid = true
		tags.String = intTagsString(request.Tags)
	}
	row := s.db.QueryRowContext(ctx, `
		update author_subscriptions set
			author_name = case when $2 = '' then author_name else $2 end,
			quality_profile = case when $3 = '' then quality_profile else $3 end,
			status = case when $4 = '' then status else $4 end,
			monitor_new_items = coalesce($5, monitor_new_items),
			tags = coalesce($6, tags),
			missing_book_policy = coalesce($7, missing_book_policy),
			updated_at = now()
		where id::text = $1
		returning
			id, provider, provider_key, author_name, wanted_format, quality_profile,
			status, monitor_new_items, missing_book_policy, tags, last_sync_at, created_at, updated_at
	`, id, strings.TrimSpace(request.AuthorName), qualityProfile, status, monitorNewItems, tags, missingBookPolicy)
	return scanAuthorSubscription(row)
}

func (s *Store) DeleteAuthorSubscription(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("author subscription id is required")
	}
	result, err := s.db.ExecContext(ctx, `
		update author_subscriptions
		set status = 'removed',
			monitor_new_items = false,
			missing_book_policy = 'none',
			updated_at = now()
		where id::text = $1
	`, id)
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

func (s *Store) ListDueAuthorSubscriptions(ctx context.Context, limit int, minInterval time.Duration, force bool, authorIDs []string, providerKeys []string) ([]AuthorSubscription, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if minInterval <= 0 {
		minInterval = 24 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-minInterval)
	authorIDList := strings.Join(compactStrings(authorIDs), ",")
	providerKeyList := strings.Join(compactStrings(providerKeys), ",")
	rows, err := s.db.QueryContext(ctx, `
		select
			id, provider, provider_key, author_name, wanted_format, quality_profile,
			status, monitor_new_items, missing_book_policy, tags, last_sync_at, created_at, updated_at
		from author_subscriptions
		where status = 'monitored'
			and monitor_new_items = true
			and missing_book_policy <> 'none'
			and ($1::boolean or last_sync_at is null or last_sync_at <= $2)
			and (
				($4 = '' and $5 = '')
				or ($4 <> '' and id::text = any(string_to_array($4, ',')))
				or ($5 <> '' and provider_key = any(string_to_array($5, ',')))
			)
		order by coalesce(last_sync_at, 'epoch'::timestamptz), author_name
		limit $3
		`, force, cutoff, limit, authorIDList, providerKeyList)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []AuthorSubscription
	for rows.Next() {
		subscription, err := scanAuthorSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}
	return subscriptions, rows.Err()
}

func (s *Store) MarkAuthorSubscriptionSynced(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `update author_subscriptions set last_sync_at = now(), updated_at = now() where id = $1`, id)
	return err
}

func (s *Store) StartAuthorMonitorRun(ctx context.Context, trigger string) (AuthorMonitorRun, error) {
	if !s.Configured() {
		return AuthorMonitorRun{}, errors.New("wanted store is unavailable")
	}
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = "manual"
	}
	row := s.db.QueryRowContext(ctx, `
		insert into author_subscription_runs(trigger, status)
		values ($1, 'running')
		returning
			id, trigger, status, authors_checked, items_found, wanted_created,
			error_count, message, started_at, finished_at
	`, trigger)
	return scanAuthorMonitorRun(row)
}

func (s *Store) FinishAuthorMonitorRun(ctx context.Context, run AuthorMonitorRun) (AuthorMonitorRun, error) {
	if !s.Configured() {
		return AuthorMonitorRun{}, errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(run.Status) == "" {
		run.Status = "completed"
	}
	row := s.db.QueryRowContext(ctx, `
		update author_subscription_runs set
			status = $2,
			authors_checked = $3,
			items_found = $4,
			wanted_created = $5,
			error_count = $6,
			message = $7,
			finished_at = now()
		where id = $1
		returning
			id, trigger, status, authors_checked, items_found, wanted_created,
			error_count, message, started_at, finished_at
	`, run.ID, run.Status, run.AuthorsChecked, run.ItemsFound, run.WantedCreated, run.ErrorCount, run.Message)
	finished, err := scanAuthorMonitorRun(row)
	if err != nil {
		return AuthorMonitorRun{}, err
	}
	finished.Items = run.Items
	return finished, nil
}

func (s *Store) GetWanted(ctx context.Context, id string) (WantedItem, error) {
	if !s.Configured() {
		return WantedItem{}, errors.New("wanted store is unavailable")
	}
	row := s.db.QueryRowContext(ctx, `
		select
			wi.id, wi.work_id, wi.edition_id, coalesce(nullif(wi.title, ''), w.title),
			coalesce(nullif(wi.author_name, ''), ''), coalesce(nullif(wi.cover_url, ''), w.cover_url),
			wi.wanted_format, wi.quality_profile, wi.status, wi.monitored, wi.metadata_provider,
			wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
			wi.tags, wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at
		from wanted_items wi
		left join works w on w.id = wi.work_id
		where wi.id = $1
	`, id)
	item, err := scanWanted(row)
	if err != nil {
		return WantedItem{}, err
	}
	items, err := s.attachWantedManualOverrides(ctx, []WantedItem{item})
	if err != nil {
		return WantedItem{}, err
	}
	if len(items) == 0 {
		return WantedItem{}, sql.ErrNoRows
	}
	return items[0], nil
}

func (s *Store) UpdateWanted(ctx context.Context, id string, request WantedUpdateRequest) (WantedItem, error) {
	if !s.Configured() {
		return WantedItem{}, errors.New("wanted store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return WantedItem{}, errors.New("wanted item id is required")
	}
	qualityProfile := strings.TrimSpace(request.QualityProfile)
	if qualityProfile != "" {
		qualityProfile = normalizeQualityProfile(qualityProfile)
	}
	monitored := sql.NullBool{}
	if request.Monitored != nil {
		monitored.Valid = true
		monitored.Bool = *request.Monitored
	}
	tags := sql.NullString{}
	if request.TagsSet {
		tags.Valid = true
		tags.String = intTagsString(request.Tags)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WantedItem{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		update wanted_items set
			title = case when $2 = '' then title else $2 end,
			author_name = case when $3 = '' then author_name else $3 end,
			cover_url = case when $4 = '' then cover_url else $4 end,
			quality_profile = case when $5 = '' then quality_profile else $5 end,
			monitored = coalesce($6::boolean, monitored),
			status = case
				when $7 <> '' then $7
				when coalesce($6::boolean, monitored) = true and status in ('removed', 'ignored') then 'wanted'
				else status
			end,
			tags = coalesce($8, tags),
			updated_at = now()
		where id::text = $1
	`, id, strings.TrimSpace(request.Title), strings.TrimSpace(request.AuthorName), strings.TrimSpace(request.CoverURL),
		qualityProfile, monitored, strings.TrimSpace(request.Status), tags)
	if err != nil {
		return WantedItem{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return WantedItem{}, err
	}
	if affected == 0 {
		return WantedItem{}, sql.ErrNoRows
	}
	if title := strings.TrimSpace(request.Title); title != "" {
		if err := upsertWantedManualOverride(ctx, tx, id, "title", title); err != nil {
			return WantedItem{}, err
		}
	}
	if authorName := strings.TrimSpace(request.AuthorName); authorName != "" {
		if err := upsertWantedManualOverride(ctx, tx, id, "author_name", authorName); err != nil {
			return WantedItem{}, err
		}
	}
	if coverURL := strings.TrimSpace(request.CoverURL); coverURL != "" {
		if err := upsertWantedManualOverride(ctx, tx, id, "cover_url", coverURL); err != nil {
			return WantedItem{}, err
		}
	}
	if qualityProfile != "" {
		if err := upsertWantedManualOverride(ctx, tx, id, "quality_profile", qualityProfile); err != nil {
			return WantedItem{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return WantedItem{}, err
	}
	return s.GetWanted(ctx, id)
}

func upsertWantedManualOverride(ctx context.Context, tx *sql.Tx, wantedID string, fieldName string, value string) error {
	_, err := tx.ExecContext(ctx, `
		insert into manual_overrides(entity_type, entity_id, field_name, value, reason)
		select 'wanted_item', id, $2, to_jsonb($3::text), 'manual wanted metadata correction'
		from wanted_items
		where id::text = $1
		on conflict (entity_type, entity_id, field_name) do update set
			value = excluded.value,
			reason = excluded.reason,
			updated_at = now()
	`, strings.TrimSpace(wantedID), strings.TrimSpace(fieldName), strings.TrimSpace(value))
	return err
}

func (s *Store) ListWantedManualOverrides(ctx context.Context, wantedID string) ([]ManualOverride, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	wantedID = strings.TrimSpace(wantedID)
	if wantedID == "" {
		return nil, errors.New("wanted item id is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		select field_name, value, coalesce(reason, ''), created_at, updated_at
		from manual_overrides
		where entity_type = 'wanted_item'
			and entity_id::text = $1
		order by field_name
	`, wantedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var overrides []ManualOverride
	for rows.Next() {
		var override ManualOverride
		var rawValue []byte
		if err := rows.Scan(&override.FieldName, &rawValue, &override.Reason, &override.CreatedAt, &override.UpdatedAt); err != nil {
			return nil, err
		}
		override.Value = manualOverrideValueString(rawValue)
		overrides = append(overrides, override)
	}
	return overrides, rows.Err()
}

func (s *Store) ClearWantedManualOverrides(ctx context.Context, wantedID string, fields []string) (WantedItem, error) {
	if !s.Configured() {
		return WantedItem{}, errors.New("wanted store is unavailable")
	}
	wantedID = strings.TrimSpace(wantedID)
	if wantedID == "" {
		return WantedItem{}, errors.New("wanted item id is required")
	}
	normalizedFields, err := normalizeWantedOverrideFields(fields)
	if err != nil {
		return WantedItem{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WantedItem{}, err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, `select exists(select 1 from wanted_items where id::text = $1)`, wantedID).Scan(&exists); err != nil {
		return WantedItem{}, err
	}
	if !exists {
		return WantedItem{}, sql.ErrNoRows
	}
	for _, field := range normalizedFields {
		if _, err := tx.ExecContext(ctx, `
			delete from manual_overrides
			where entity_type = 'wanted_item'
				and entity_id::text = $1
				and field_name = $2
		`, wantedID, field); err != nil {
			return WantedItem{}, err
		}
		if err := resetWantedFieldFromProvider(ctx, tx, wantedID, field); err != nil {
			return WantedItem{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return WantedItem{}, err
	}
	return s.GetWanted(ctx, wantedID)
}

func (s *Store) WantedMetadataProvenance(ctx context.Context, wantedID string) (MetadataProvenance, error) {
	if !s.Configured() {
		return MetadataProvenance{}, errors.New("wanted store is unavailable")
	}
	wantedID = strings.TrimSpace(wantedID)
	if wantedID == "" {
		return MetadataProvenance{}, errors.New("wanted item id is required")
	}
	item, err := s.GetWanted(ctx, wantedID)
	if err != nil {
		return MetadataProvenance{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		with target_entities as (
			select wi.work_id as entity_id
			from wanted_items wi
			where wi.id::text = $1 and wi.work_id is not null
			union
			select wi.edition_id as entity_id
			from wanted_items wi
			where wi.id::text = $1 and wi.edition_id is not null
			union
			select wa.author_id as entity_id
			from wanted_items wi
			join work_authors wa on wa.work_id = wi.work_id
			where wi.id::text = $1
		)
		select
			pr.id::text, pr.provider, pr.provider_key, pr.entity_type,
			coalesce(pr.entity_id::text, ''), pr.confidence, pr.fetched_at, pr.raw
		from provider_records pr
		join target_entities te on te.entity_id = pr.entity_id
		order by
			case pr.entity_type
				when 'work' then 0
				when 'edition' then 1
				when 'author' then 2
				else 3
			end,
			pr.confidence desc,
			pr.fetched_at desc
	`, wantedID)
	if err != nil {
		return MetadataProvenance{}, err
	}
	defer rows.Close()

	records := []ProviderMetadataRecord{}
	for rows.Next() {
		var record ProviderMetadataRecord
		var raw []byte
		if err := rows.Scan(
			&record.ID, &record.Provider, &record.ProviderKey, &record.EntityType,
			&record.EntityID, &record.Confidence, &record.FetchedAt, &raw,
		); err != nil {
			return MetadataProvenance{}, err
		}
		record.Values = metadataValuesFromProviderRaw(raw)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return MetadataProvenance{}, err
	}
	return MetadataProvenance{
		WantedItem:      item,
		Records:         records,
		Fields:          metadataFieldEvidence(item, records),
		ManualOverrides: item.ManualOverrides,
		GeneratedAt:     time.Now().UTC(),
	}, nil
}

func (s *Store) WantedMetadataReviewQueue(ctx context.Context) (MetadataReviewQueue, error) {
	if !s.Configured() {
		return MetadataReviewQueue{}, errors.New("wanted store is unavailable")
	}
	items, err := s.ListWanted(ctx, "")
	if err != nil {
		return MetadataReviewQueue{}, err
	}
	reviewItems := []MetadataReviewItem{}
	for _, item := range items {
		if wantedItemReviewSkipped(item) {
			continue
		}
		provenance, err := s.WantedMetadataProvenance(ctx, item.ID)
		if err != nil {
			return MetadataReviewQueue{}, err
		}
		review := metadataReviewItem(provenance)
		if review.ConflictCount == 0 && review.ProtectedCount == 0 {
			continue
		}
		reviewItems = append(reviewItems, review)
	}
	return MetadataReviewQueue{
		Items:       reviewItems,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func (s *Store) ApplyWantedMetadataCorrection(ctx context.Context, wantedID string, request MetadataCorrectionRequest) (MetadataProvenance, error) {
	if !s.Configured() {
		return MetadataProvenance{}, errors.New("wanted store is unavailable")
	}
	wantedID = strings.TrimSpace(wantedID)
	if wantedID == "" {
		return MetadataProvenance{}, errors.New("wanted item id is required")
	}
	field, value, err := metadataCorrectionFieldValue(request)
	if err != nil {
		return MetadataProvenance{}, err
	}
	switch field {
	case "title", "author_name", "cover_url", "quality_profile":
		update, err := metadataCorrectionUpdateRequest(MetadataCorrectionRequest{FieldName: field, Value: value})
		if err != nil {
			return MetadataProvenance{}, err
		}
		if _, err := s.UpdateWanted(ctx, wantedID, update); err != nil {
			return MetadataProvenance{}, err
		}
	default:
		if err := s.upsertWantedManualOverride(ctx, wantedID, field, value); err != nil {
			return MetadataProvenance{}, err
		}
	}
	return s.WantedMetadataProvenance(ctx, wantedID)
}

func (s *Store) upsertWantedManualOverride(ctx context.Context, wantedID string, fieldName string, value string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsertWantedManualOverride(ctx, tx, wantedID, fieldName, value); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func resetWantedFieldFromProvider(ctx context.Context, tx *sql.Tx, wantedID string, field string) error {
	switch field {
	case "title":
		_, err := tx.ExecContext(ctx, `
			update wanted_items wi
			set title = coalesce(nullif(w.title, ''), wi.title),
				updated_at = now()
			from works w
			where wi.work_id = w.id and wi.id::text = $1
		`, wantedID)
		return err
	case "author_name":
		_, err := tx.ExecContext(ctx, `
			update wanted_items wi
			set author_name = coalesce(nullif((
					select a.canonical_name
					from work_authors wa
					join authors a on a.id = wa.author_id
					where wa.work_id = wi.work_id
					order by case when wa.role = 'author' then 0 else 1 end, a.canonical_name
					limit 1
				), ''), wi.author_name),
				updated_at = now()
			where wi.id::text = $1
		`, wantedID)
		return err
	case "cover_url":
		_, err := tx.ExecContext(ctx, `
			update wanted_items wi
			set cover_url = coalesce(nullif(w.cover_url, ''), wi.cover_url),
				updated_at = now()
			from works w
			where wi.work_id = w.id and wi.id::text = $1
		`, wantedID)
		return err
	case "quality_profile":
		_, err := tx.ExecContext(ctx, `
			update wanted_items
			set quality_profile = 'standard',
				updated_at = now()
			where id::text = $1
		`, wantedID)
		return err
	case "language", "publisher", "published_date", "series", "series_position", "isbn":
		return nil
	default:
		return fmt.Errorf("unsupported wanted override field %q", field)
	}
}

func (s *Store) attachWantedManualOverrides(ctx context.Context, items []WantedItem) ([]WantedItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	for index := range items {
		overrides, err := s.ListWantedManualOverrides(ctx, items[index].ID)
		if err != nil {
			return nil, err
		}
		items[index].ManualOverrides = overrides
	}
	return items, nil
}

func metadataValuesFromProviderRaw(raw []byte) MetadataRecordValues {
	var result metadata.SearchResult
	if len(raw) == 0 || json.Unmarshal(raw, &result) != nil {
		return MetadataRecordValues{}
	}
	values := MetadataRecordValues{
		Title:            firstNonEmpty(result.Edition.Title, result.Work.Title),
		CoverURL:         result.Work.CoverURL,
		Format:           string(result.Edition.Format),
		Language:         result.Edition.Language,
		Publisher:        result.Edition.Publisher,
		PublishedDate:    result.Edition.PublishedDate,
		FirstPublishYear: result.Work.FirstPublishYear,
		ISBNs:            append([]string(nil), result.Edition.ISBNs...),
		Series:           result.Work.Series,
		SeriesPosition:   result.Work.SeriesPosition,
		MatchedOn:        append([]string(nil), result.MatchedOn...),
		SourceKey:        firstNonEmpty(result.Edition.ID, result.Work.ID, result.RawSourceKey),
	}
	if len(result.Work.Authors) > 0 {
		values.AuthorName = result.Work.Authors[0].Name
	}
	return values
}

type metadataFieldSpec struct {
	name      string
	label     string
	canonical func(WantedItem) string
	values    func(MetadataRecordValues) []string
}

func metadataFieldEvidence(item WantedItem, records []ProviderMetadataRecord) []MetadataFieldEvidence {
	overrideValues := make(map[string]string, len(item.ManualOverrides))
	for _, override := range item.ManualOverrides {
		overrideValues[strings.TrimSpace(override.FieldName)] = strings.TrimSpace(override.Value)
	}
	specs := []metadataFieldSpec{
		{name: "title", label: "Title", canonical: func(item WantedItem) string { return item.Title }, values: func(values MetadataRecordValues) []string { return oneValue(values.Title) }},
		{name: "author_name", label: "Author", canonical: func(item WantedItem) string { return item.AuthorName }, values: func(values MetadataRecordValues) []string { return oneValue(values.AuthorName) }},
		{name: "cover_url", label: "Cover", canonical: func(item WantedItem) string { return item.CoverURL }, values: func(values MetadataRecordValues) []string { return oneValue(values.CoverURL) }},
		{name: "format", label: "Format", canonical: func(item WantedItem) string { return item.Format }, values: func(values MetadataRecordValues) []string { return oneValue(values.Format) }},
		{name: "quality_profile", label: "Quality Profile", canonical: func(item WantedItem) string { return item.QualityProfile }},
		{name: "language", label: "Language", values: func(values MetadataRecordValues) []string { return oneValue(values.Language) }},
		{name: "publisher", label: "Publisher", values: func(values MetadataRecordValues) []string { return oneValue(values.Publisher) }},
		{name: "published_date", label: "Published", values: func(values MetadataRecordValues) []string { return oneValue(values.PublishedDate) }},
		{name: "series", label: "Series", values: func(values MetadataRecordValues) []string { return oneValue(values.Series) }},
		{name: "series_position", label: "Series Position", values: func(values MetadataRecordValues) []string { return oneValue(values.SeriesPosition) }},
		{name: "isbn", label: "ISBNs", values: func(values MetadataRecordValues) []string { return values.ISBNs }},
	}
	evidence := make([]MetadataFieldEvidence, 0, len(specs))
	for _, spec := range specs {
		canonical := ""
		overrideValue, protected := overrideValues[spec.name]
		if protected {
			canonical = strings.TrimSpace(overrideValue)
		} else if spec.canonical != nil {
			canonical = strings.TrimSpace(spec.canonical(item))
		}
		candidates := metadataFieldCandidates(spec, records)
		if canonical == "" && len(candidates) == 0 && !protected {
			continue
		}
		source := ""
		if protected {
			source = "manual_override"
		} else if canonical != "" {
			source = "wanted"
		}
		evidence = append(evidence, MetadataFieldEvidence{
			FieldName:       spec.name,
			Label:           spec.label,
			CanonicalValue:  canonical,
			CanonicalSource: source,
			Protected:       protected,
			Conflict:        metadataFieldHasConflict(canonical, candidates, protected),
			Candidates:      candidates,
		})
	}
	return evidence
}

func metadataFieldCandidates(spec metadataFieldSpec, records []ProviderMetadataRecord) []MetadataFieldCandidate {
	if spec.values == nil {
		return nil
	}
	candidates := []MetadataFieldCandidate{}
	seen := map[string]bool{}
	for _, record := range records {
		for _, value := range spec.values(record.Values) {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			key := strings.Join([]string{record.Provider, record.ProviderKey, normalizeText(value)}, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, MetadataFieldCandidate{
				Provider:    record.Provider,
				ProviderKey: record.ProviderKey,
				EntityType:  record.EntityType,
				Value:       value,
				Confidence:  record.Confidence,
				FetchedAt:   record.FetchedAt,
				MatchedOn:   append([]string(nil), record.Values.MatchedOn...),
			})
		}
	}
	return candidates
}

func metadataFieldHasConflict(canonical string, candidates []MetadataFieldCandidate, protected bool) bool {
	distinct := map[string]bool{}
	canonicalKey := normalizeText(canonical)
	for _, candidate := range candidates {
		key := normalizeText(candidate.Value)
		if key == "" {
			continue
		}
		distinct[key] = true
		if protected && canonicalKey != "" && key != canonicalKey {
			return true
		}
	}
	if canonicalKey != "" {
		for key := range distinct {
			if key != canonicalKey {
				return true
			}
		}
		return false
	}
	return len(distinct) > 1
}

func oneValue(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return []string{value}
}

func metadataReviewItem(provenance MetadataProvenance) MetadataReviewItem {
	fields := []MetadataFieldEvidence{}
	conflictCount := 0
	protectedCount := 0
	candidateCount := 0
	for _, field := range provenance.Fields {
		if field.Conflict {
			conflictCount++
		}
		if field.Protected {
			protectedCount++
		}
		candidateCount += len(field.Candidates)
		if field.Conflict || field.Protected {
			fields = append(fields, field)
		}
	}
	return MetadataReviewItem{
		WantedItem:     provenance.WantedItem,
		Fields:         fields,
		ConflictCount:  conflictCount,
		ProtectedCount: protectedCount,
		RecordCount:    len(provenance.Records),
		CandidateCount: candidateCount,
		LastFetchedAt:  latestProviderRecordFetchedAt(provenance.Records),
	}
}

func wantedItemReviewSkipped(item WantedItem) bool {
	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case "removed", "ignored", "imported":
		return true
	default:
		return false
	}
}

func latestProviderRecordFetchedAt(records []ProviderMetadataRecord) *time.Time {
	var latest *time.Time
	for _, record := range records {
		fetchedAt := record.FetchedAt.UTC()
		if latest == nil || fetchedAt.After(*latest) {
			value := fetchedAt
			latest = &value
		}
	}
	return latest
}

func metadataCorrectionUpdateRequest(request MetadataCorrectionRequest) (WantedUpdateRequest, error) {
	field, value, err := metadataCorrectionFieldValue(request)
	if err != nil {
		return WantedUpdateRequest{}, err
	}
	switch field {
	case "title":
		return WantedUpdateRequest{Title: value}, nil
	case "author_name":
		return WantedUpdateRequest{AuthorName: value}, nil
	case "cover_url":
		return WantedUpdateRequest{CoverURL: value}, nil
	case "quality_profile":
		return WantedUpdateRequest{QualityProfile: value}, nil
	default:
		return WantedUpdateRequest{}, fmt.Errorf("unsupported wanted metadata field %q", request.FieldName)
	}
}

func metadataCorrectionFieldValue(request MetadataCorrectionRequest) (string, string, error) {
	field := normalizeWantedOverrideField(request.FieldName)
	if !validWantedOverrideField(field) {
		return "", "", fmt.Errorf("unsupported wanted metadata field %q", request.FieldName)
	}
	value := strings.TrimSpace(request.Value)
	if value == "" {
		return "", "", errors.New("metadata correction value is required")
	}
	return field, value, nil
}

func normalizeWantedOverrideFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return allWantedOverrideFields(), nil
	}
	seen := map[string]bool{}
	var normalized []string
	for _, field := range fields {
		field = normalizeWantedOverrideField(field)
		if field == "" {
			return nil, errors.New("manual override field is required")
		}
		if !validWantedOverrideField(field) {
			return nil, fmt.Errorf("unsupported wanted override field %q", field)
		}
		if seen[field] {
			continue
		}
		seen[field] = true
		normalized = append(normalized, field)
	}
	return normalized, nil
}

func normalizeWantedOverrideField(field string) string {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "title":
		return "title"
	case "author", "authorname", "author_name":
		return "author_name"
	case "cover", "coverurl", "cover_url":
		return "cover_url"
	case "quality", "qualityprofile", "quality_profile":
		return "quality_profile"
	case "lang", "language":
		return "language"
	case "publisher":
		return "publisher"
	case "published", "publisheddate", "published_date", "publicationdate", "publication_date":
		return "published_date"
	case "series":
		return "series"
	case "seriesposition", "series_position", "seriesindex", "series_index":
		return "series_position"
	case "isbn", "isbns":
		return "isbn"
	default:
		return strings.ToLower(strings.TrimSpace(field))
	}
}

func validWantedOverrideField(field string) bool {
	switch field {
	case "title", "author_name", "cover_url", "quality_profile", "language", "publisher", "published_date", "series", "series_position", "isbn":
		return true
	default:
		return false
	}
}

func allWantedOverrideFields() []string {
	return []string{"title", "author_name", "cover_url", "quality_profile", "language", "publisher", "published_date", "series", "series_position", "isbn"}
}

func manualOverrideValueString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	return strings.TrimSpace(string(raw))
}

func (s *Store) DeleteWanted(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("wanted store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("wanted item id is required")
	}
	result, err := s.db.ExecContext(ctx, `
		update wanted_items
		set status = 'removed',
			monitored = false,
			updated_at = now()
		where id::text = $1
	`, id)
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
			wi.wanted_format, wi.quality_profile, wi.status, wi.monitored, wi.metadata_provider,
			wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
			wi.tags, wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at
		from wanted_items wi
		left join works w on w.id = wi.work_id
		where wi.status = 'wanted'
			and wi.monitored = true
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
		"wi.monitored = true",
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
			wi.wanted_format, wi.quality_profile, wi.status, wi.monitored, wi.metadata_provider,
			wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
			wi.tags, wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at
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

func (s *Store) UpsertAuthorMetadataReview(ctx context.Context, review AuthorMetadataReview) (AuthorMetadataReview, error) {
	if !s.Configured() {
		return AuthorMetadataReview{}, errors.New("wanted store is unavailable")
	}
	review = normalizeAuthorMetadataReview(review)
	if strings.TrimSpace(review.AuthorSubscriptionID) == "" {
		return AuthorMetadataReview{}, errors.New("author subscription id is required")
	}
	if strings.TrimSpace(review.CandidateKey) == "" {
		return AuthorMetadataReview{}, errors.New("author metadata review candidate key is required")
	}
	raw, err := json.Marshal(review.Result)
	if err != nil {
		return AuthorMetadataReview{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		insert into author_metadata_reviews (
			author_subscription_id, provider, candidate_key, title, author_name,
			wanted_format, quality_profile, tags, policy, reason, status, decision,
			wanted_item_id, result
		) values (
			nullif($1, '')::uuid, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11, $12,
			nullif($13, '')::uuid, $14::jsonb
		)
		on conflict (author_subscription_id, candidate_key, wanted_format)
		do update set
			provider = excluded.provider,
			title = excluded.title,
			author_name = excluded.author_name,
			quality_profile = excluded.quality_profile,
			tags = excluded.tags,
			policy = excluded.policy,
			reason = excluded.reason,
			result = excluded.result,
			updated_at = now()
		where author_metadata_reviews.status = 'pending'
		returning
			id, coalesce(author_subscription_id::text, ''), provider, candidate_key,
			title, author_name, wanted_format, quality_profile, tags, policy, reason,
			status, decision, coalesce(wanted_item_id::text, ''), result,
			created_at, updated_at, resolved_at
	`, review.AuthorSubscriptionID, review.Provider, review.CandidateKey, review.Title, review.AuthorName,
		review.Format, review.QualityProfile, intTagsString(review.Tags), review.Policy, review.Reason,
		review.Status, review.Decision, review.WantedID, string(raw))
	saved, err := scanAuthorMetadataReview(row)
	if errors.Is(err, sql.ErrNoRows) {
		return s.findAuthorMetadataReviewByCandidate(ctx, review.AuthorSubscriptionID, review.CandidateKey, review.Format)
	}
	return saved, err
}

func (s *Store) findAuthorMetadataReviewByCandidate(ctx context.Context, authorSubscriptionID string, candidateKey string, format string) (AuthorMetadataReview, error) {
	row := s.db.QueryRowContext(ctx, `
		select
			id, coalesce(author_subscription_id::text, ''), provider, candidate_key,
			title, author_name, wanted_format, quality_profile, tags, policy, reason,
			status, decision, coalesce(wanted_item_id::text, ''), result,
			created_at, updated_at, resolved_at
		from author_metadata_reviews
		where author_subscription_id = nullif($1, '')::uuid
			and candidate_key = $2
			and wanted_format = $3
	`, strings.TrimSpace(authorSubscriptionID), strings.TrimSpace(candidateKey), normalizeFormat(format))
	return scanAuthorMetadataReview(row)
}

func (s *Store) ListAuthorMetadataReviews(ctx context.Context, query AuthorMetadataReviewQuery) ([]AuthorMetadataReview, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
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
			id, coalesce(author_subscription_id::text, ''), provider, candidate_key,
			title, author_name, wanted_format, quality_profile, tags, policy, reason,
			status, decision, coalesce(wanted_item_id::text, ''), result,
			created_at, updated_at, resolved_at
		from author_metadata_reviews
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

	reviews := []AuthorMetadataReview{}
	for rows.Next() {
		review, err := scanAuthorMetadataReview(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (s *Store) GetAuthorMetadataReview(ctx context.Context, id string) (AuthorMetadataReview, error) {
	if !s.Configured() {
		return AuthorMetadataReview{}, errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(id) == "" {
		return AuthorMetadataReview{}, errors.New("author metadata review id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		select
			id, coalesce(author_subscription_id::text, ''), provider, candidate_key,
			title, author_name, wanted_format, quality_profile, tags, policy, reason,
			status, decision, coalesce(wanted_item_id::text, ''), result,
			created_at, updated_at, resolved_at
		from author_metadata_reviews
		where id::text = $1
	`, strings.TrimSpace(id))
	return scanAuthorMetadataReview(row)
}

func (s *Store) ResolveAuthorMetadataReview(ctx context.Context, id string, status string, decision string, wantedID string) (AuthorMetadataReview, error) {
	if !s.Configured() {
		return AuthorMetadataReview{}, errors.New("wanted store is unavailable")
	}
	if strings.TrimSpace(id) == "" {
		return AuthorMetadataReview{}, errors.New("author metadata review id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		update author_metadata_reviews set
			status = $2,
			decision = $3,
			wanted_item_id = coalesce(nullif($4, '')::uuid, wanted_item_id),
			updated_at = now(),
			resolved_at = now()
		where id::text = $1
		returning
			id, coalesce(author_subscription_id::text, ''), provider, candidate_key,
			title, author_name, wanted_format, quality_profile, tags, policy, reason,
			status, decision, coalesce(wanted_item_id::text, ''), result,
			created_at, updated_at, resolved_at
	`, strings.TrimSpace(id), strings.TrimSpace(status), strings.TrimSpace(decision), strings.TrimSpace(wantedID))
	return scanAuthorMetadataReview(row)
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
	var tags string
	if err := row.Scan(
		&item.ID, &workID, &editionID, &item.Title, &item.AuthorName, &coverURL,
		&item.Format, &item.QualityProfile, &item.Status, &item.Monitored, &sourceProvider,
		&sourceKey, &item.CurrentReleaseID, &item.CurrentReleaseScore,
		&tags, &lastSearchAt, &lastUpgradeSearchAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return WantedItem{}, err
	}
	item.WorkID = workID.String
	item.EditionID = editionID.String
	item.CoverURL = coverURL.String
	item.SourceProvider = sourceProvider.String
	item.SourceKey = sourceKey.String
	item.Tags = splitIntTags(tags)
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

func scanQualityProfile(row wantedScanner) (QualityProfile, error) {
	var profile QualityProfile
	var preferredTerms, requiredTerms, rejectedTerms string
	if err := row.Scan(
		&profile.ID, &profile.Name, &profile.MediaFormat, &profile.MinScore,
		&profile.CutoffScore, &profile.MinSeeders, &profile.MaxSizeBytes,
		&preferredTerms, &requiredTerms, &rejectedTerms, &profile.PreferredScore,
		&profile.UpgradeAllowed, &profile.CreatedAt, &profile.UpdatedAt,
	); err != nil {
		return QualityProfile{}, err
	}
	profile.PreferredTerms = splitComma(preferredTerms)
	profile.RequiredTerms = splitComma(requiredTerms)
	profile.RejectedTerms = splitComma(rejectedTerms)
	return profile, nil
}

func scanAuthorSubscription(row wantedScanner) (AuthorSubscription, error) {
	var subscription AuthorSubscription
	var lastSyncAt sql.NullTime
	var tags string
	if err := row.Scan(
		&subscription.ID, &subscription.Provider, &subscription.ProviderKey,
		&subscription.AuthorName, &subscription.Format, &subscription.QualityProfile,
		&subscription.Status, &subscription.MonitorNewItems, &subscription.MissingBookPolicy, &tags, &lastSyncAt,
		&subscription.CreatedAt, &subscription.UpdatedAt,
	); err != nil {
		return AuthorSubscription{}, err
	}
	subscription.Tags = splitIntTags(tags)
	subscription.MissingBookPolicy = normalizeAuthorMissingBookPolicy(subscription.MissingBookPolicy, subscription.MonitorNewItems)
	if lastSyncAt.Valid {
		value := lastSyncAt.Time.UTC()
		subscription.LastSyncAt = &value
	}
	return subscription, nil
}

func scanAuthorMetadataReview(row wantedScanner) (AuthorMetadataReview, error) {
	var review AuthorMetadataReview
	var tags string
	var raw []byte
	var resolvedAt sql.NullTime
	if err := row.Scan(
		&review.ID, &review.AuthorSubscriptionID, &review.Provider, &review.CandidateKey,
		&review.Title, &review.AuthorName, &review.Format, &review.QualityProfile,
		&tags, &review.Policy, &review.Reason, &review.Status, &review.Decision,
		&review.WantedID, &raw, &review.CreatedAt, &review.UpdatedAt, &resolvedAt,
	); err != nil {
		return AuthorMetadataReview{}, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &review.Result)
	}
	review.Tags = splitIntTags(tags)
	if resolvedAt.Valid {
		value := resolvedAt.Time.UTC()
		review.ResolvedAt = &value
	}
	return review, nil
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

func scanAuthorMonitorRun(row wantedScanner) (AuthorMonitorRun, error) {
	var run AuthorMonitorRun
	var finishedAt sql.NullTime
	if err := row.Scan(
		&run.ID, &run.Trigger, &run.Status, &run.AuthorsChecked, &run.ItemsFound,
		&run.WantedCreated, &run.ErrorCount, &run.Message, &run.StartedAt, &finishedAt,
	); err != nil {
		return AuthorMonitorRun{}, err
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

func normalizeProfileForStorage(profile QualityProfile) QualityProfile {
	if strings.TrimSpace(profile.Name) != "" {
		profile.Name = normalizeQualityProfile(profile.Name)
	}
	profile.MediaFormat = normalizeProfileFormat(profile.MediaFormat)
	if profile.MinScore <= 0 {
		profile.MinScore = 60
	}
	if profile.MinScore > 100 {
		profile.MinScore = 100
	}
	if profile.CutoffScore <= 0 {
		profile.CutoffScore = 85
	}
	if profile.CutoffScore > 100 {
		profile.CutoffScore = 100
	}
	if profile.MinSeeders < 0 {
		profile.MinSeeders = 0
	}
	if profile.PreferredScore <= 0 {
		profile.PreferredScore = 8
	}
	if len(cleanTerms(profile.RejectedTerms)) == 0 {
		profile.RejectedTerms = []string{"summary", "review"}
	}
	return profile
}

func normalizeAuthorSubscription(subscription AuthorSubscription) AuthorSubscription {
	subscription.Provider = strings.TrimSpace(subscription.Provider)
	subscription.ProviderKey = strings.TrimSpace(subscription.ProviderKey)
	subscription.AuthorName = strings.TrimSpace(subscription.AuthorName)
	subscription.Format = normalizeFormat(subscription.Format)
	subscription.QualityProfile = normalizeQualityProfile(subscription.QualityProfile)
	subscription.Tags = compactRestrictionTags(subscription.Tags)
	subscription.MissingBookPolicy = normalizeAuthorMissingBookPolicy(subscription.MissingBookPolicy, subscription.MonitorNewItems)
	subscription.MonitorNewItems = subscription.MissingBookPolicy != "none"
	if strings.TrimSpace(subscription.Status) == "" {
		subscription.Status = "monitored"
	}
	return subscription
}

func normalizeAuthorMetadataReview(review AuthorMetadataReview) AuthorMetadataReview {
	review.AuthorSubscriptionID = strings.TrimSpace(review.AuthorSubscriptionID)
	review.Provider = strings.TrimSpace(firstNonEmpty(review.Provider, review.Result.Provider))
	review.CandidateKey = strings.TrimSpace(review.CandidateKey)
	if review.CandidateKey == "" {
		review.CandidateKey = authorMetadataReviewCandidateKey(review.Result)
	}
	review.Title = strings.TrimSpace(firstNonEmpty(review.Title, review.Result.Work.Title, review.Result.Edition.Title))
	review.AuthorName = strings.TrimSpace(firstNonEmpty(review.AuthorName, firstResultAuthorName(review.Result)))
	review.Format = normalizeFormat(firstNonEmpty(review.Format, string(review.Result.Edition.Format)))
	review.QualityProfile = normalizeQualityProfile(review.QualityProfile)
	review.Tags = compactRestrictionTags(review.Tags)
	review.Policy = normalizeAuthorMissingBookPolicy(review.Policy, true)
	review.Reason = strings.TrimSpace(review.Reason)
	if strings.TrimSpace(review.Status) == "" {
		review.Status = "pending"
	}
	review.Status = strings.TrimSpace(review.Status)
	review.Decision = strings.TrimSpace(review.Decision)
	return review
}

func authorMetadataReviewCandidateKey(result metadata.SearchResult) string {
	return strings.Join([]string{
		strings.TrimSpace(result.Provider),
		firstNonEmpty(strings.TrimSpace(result.Edition.ID), strings.TrimSpace(result.Work.ID), strings.TrimSpace(result.RawSourceKey), normalizeText(result.Work.Title)),
	}, ":")
}

func firstResultAuthorName(result metadata.SearchResult) string {
	if len(result.Work.Authors) == 0 {
		return ""
	}
	return result.Work.Authors[0].Name
}

func normalizeAuthorMissingBookPolicy(policy string, monitorNewItems bool) string {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(policy), "_", "")) {
	case "all", "allbooks", "missing", "missingbooks", "existing", "existingbooks":
		return "all"
	case "future", "futurebooks", "new", "newbooks", "newitems":
		return "future"
	case "none", "no", "off", "unmonitored":
		return "none"
	default:
		if monitorNewItems {
			return "all"
		}
		return "none"
	}
}

func normalizeProfileFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "audiobook", "audio":
		return "audiobook"
	case "ebook", "book":
		return "ebook"
	default:
		return "any"
	}
}

func cleanTerms(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeText(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func intTagsString(tags []int) string {
	tags = compactRestrictionTags(tags)
	if len(tags) == 0 {
		return ""
	}
	values := make([]string, 0, len(tags))
	for _, tag := range tags {
		values = append(values, strconv.Itoa(tag))
	}
	return strings.Join(values, ",")
}

func splitIntTags(value string) []int {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	tags := make([]int, 0, len(parts))
	for _, part := range parts {
		tag, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil {
			tags = append(tags, tag)
		}
	}
	return compactRestrictionTags(tags)
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
