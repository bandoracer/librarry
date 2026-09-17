package wanted

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"time"
)

type MetadataReviewQuery struct {
	Search string `json:"q"`
	Format string `json:"format"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}

func (s *Service) MetadataReviewCollection(ctx context.Context, q MetadataReviewQuery) (MetadataReviewQueue, error) {
	if _, _, err := normalizeReviewQuery(q); err != nil {
		return MetadataReviewQueue{}, err
	}
	if !s.Available() {
		return MetadataReviewQueue{}, sql.ErrConnDone
	}
	return s.store.MetadataReviewCollection(ctx, q)
}

func normalizeReviewQuery(query MetadataReviewQuery) (BookCollectionQuery, bookCursor, error) {
	if query.Cursor != "" {
		if len(query.Cursor) > 8192 {
			return BookCollectionQuery{}, bookCursor{}, ErrBookPage
		}
		raw, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		var c bookCursor
		if err != nil || json.Unmarshal(raw, &c) != nil || !strings.HasPrefix(c.Query, "review:") {
			return BookCollectionQuery{}, c, ErrBookPage
		}
		c.Query = strings.TrimPrefix(c.Query, "review:")
		raw, _ = json.Marshal(c)
		query.Cursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return normalizeBookQuery(BookCollectionQuery{Search: query.Search, Format: query.Format, Sort: "title", Cursor: query.Cursor, Limit: query.Limit})
}

// Review membership uses the same field evidence as book details. Counts inspect
// every active tracked book, with batched database reads and no provider requests.
func (s *Store) MetadataReviewCollection(ctx context.Context, query MetadataReviewQuery) (MetadataReviewQueue, error) {
	page := MetadataReviewQueue{Items: []MetadataReviewItem{}, GeneratedAt: time.Now().UTC()}
	q, cursor, err := normalizeReviewQuery(query)
	if err != nil {
		return page, err
	}
	if !s.Configured() {
		return page, sql.ErrConnDone
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `set local jit=off`); err != nil {
		return page, err
	}
	if _, err = tx.ExecContext(ctx, `set local plan_cache_mode=force_custom_plan`); err != nil {
		return page, err
	}
	items, keys, err := reviewBooks(ctx, tx, nil, false)
	if err != nil {
		return page, err
	}
	records, err := reviewRecords(ctx, tx, items)
	if err != nil {
		return page, err
	}
	var last bookCursor
	for i, item := range items {
		if err = ctx.Err(); err != nil {
			return page, err
		}
		provenance := reviewProvenance(item, records[item.ID], page.GeneratedAt)
		review := metadataReviewSummary(provenance)
		if !metadataReviewRequiresOperator(review) {
			continue
		}
		page.Total++
		page.ConflictCount += review.ConflictCount
		if q.Format != "all" && q.Format != item.Format {
			continue
		}
		if q.Search != "" && !strings.Contains(strings.ToLower(item.Title+" "+item.AuthorName+" "+item.SourceProvider+" "+item.QualityProfile), strings.ToLower(q.Search)) {
			continue
		}
		page.Filtered++
		key := keys[i]
		if cursor.ID != "" && !reviewKeyAfter(key, cursor) {
			continue
		}
		if len(page.Items) <= q.Limit {
			review.Revision = metadataReviewRevision(provenance)
			page.Items = append(page.Items, review)
		}
		if len(page.Items) <= q.Limit {
			last = key
			last.Query = "review:" + bookQueryKey(q)
		}
	}
	if len(page.Items) > q.Limit {
		page.Items = page.Items[:q.Limit]
		raw, _ := json.Marshal(last)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	if err = tx.Commit(); err != nil {
		return page, err
	}
	return page, nil
}

func reviewKeyAfter(a, b bookCursor) bool {
	if a.First != b.First {
		return a.First > b.First
	}
	if a.Second != b.Second {
		return a.Second > b.Second
	}
	return a.ID > b.ID
}

func reviewBooks(ctx context.Context, tx *sql.Tx, ids []string, lock bool) ([]WantedItem, []bookCursor, error) {
	where := authorVisibleBookSQL
	var args []any
	if ids != nil {
		where = `wi.id=any($1::uuid[])`
		args = []any{ids}
	}
	order := `lower(coalesce(nullif(wi.title,''),w.title,'')) collate "C",lower(wi.author_name) collate "C",wi.id`
	if lock {
		order = `wi.id`
	}
	query := `select ` + wantedDetailColumns + `,lower(coalesce(nullif(wi.title,''),w.title,'')),lower(wi.author_name) from wanted_items wi left join works w on w.id=wi.work_id where ` + where + ` order by ` + order
	if lock {
		query += ` for update of wi`
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	items := []WantedItem{}
	keys := []bookCursor{}
	for rows.Next() {
		var key bookCursor
		item, e := scanWanted(wantedWithExtra{row: rows, extra: []any{&key.First, &key.Second}})
		if e != nil {
			rows.Close()
			return nil, nil, e
		}
		key.ID = item.ID
		items = append(items, item)
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	items, err = attachWantedDetails(ctx, tx, items)
	return items, keys, err
}

func reviewRecords(ctx context.Context, reader wantedDetailReader, items []WantedItem) (map[string][]ProviderMetadataRecord, error) {
	result := map[string][]ProviderMetadataRecord{}
	if len(items) == 0 {
		return result, nil
	}
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	rows, err := reader.QueryContext(ctx, `with targets as materialized (
 select id,work_id,edition_id from wanted_items where id=any($1::uuid[])
 ), entities as (
 select id as wanted_id,work_id as entity_id from targets where work_id is not null
 union select id,edition_id from targets where edition_id is not null
 union select t.id,wa.author_id from targets t join work_authors wa on wa.work_id=t.work_id
 ) select e.wanted_id::text,p.id::text,p.provider,p.provider_key,p.entity_type,coalesce(p.entity_id::text,''),p.confidence,p.fetched_at,p.raw
 from entities e join provider_records p on p.entity_id=e.entity_id
 order by e.wanted_id,case p.entity_type when 'work' then 0 when 'edition' then 1 when 'author' then 2 else 3 end,p.confidence desc,p.fetched_at desc,p.provider,p.provider_key,p.id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := map[string]MetadataRecordValues{}
	for rows.Next() {
		var id string
		var p ProviderMetadataRecord
		var raw []byte
		if err = rows.Scan(&id, &p.ID, &p.Provider, &p.ProviderKey, &p.EntityType, &p.EntityID, &p.Confidence, &p.FetchedAt, &raw); err != nil {
			return nil, err
		}
		var ok bool
		p.Values, ok = values[p.ID]
		if !ok {
			p.Values = metadataValuesFromProviderRaw(raw)
			values[p.ID] = p.Values
		}
		result[id] = append(result[id], p)
	}
	return result, rows.Err()
}

func reviewProvenance(item WantedItem, records []ProviderMetadataRecord, at time.Time) MetadataProvenance {
	if records == nil {
		records = []ProviderMetadataRecord{}
	}
	return MetadataProvenance{WantedItem: item, Records: records, Fields: metadataFieldEvidence(item, records), ManualOverrides: item.ManualOverrides, GeneratedAt: at}
}

func (s *Store) metadataProvenanceSnapshot(ctx context.Context, id string) (MetadataProvenance, error) {
	if !s.Configured() {
		return MetadataProvenance{}, sql.ErrConnDone
	}
	var parsed pgtype.UUID
	if parsed.Scan(strings.TrimSpace(id)) != nil || !parsed.Valid {
		return MetadataProvenance{}, sql.ErrNoRows
	}
	value, _ := parsed.Value()
	id = value.(string)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return MetadataProvenance{}, err
	}
	defer tx.Rollback()
	items, _, err := reviewBooks(ctx, tx, []string{id}, false)
	if err != nil {
		return MetadataProvenance{}, err
	}
	if len(items) == 0 {
		return MetadataProvenance{}, sql.ErrNoRows
	}
	records, err := reviewRecords(ctx, tx, items)
	if err != nil {
		return MetadataProvenance{}, err
	}
	detail := reviewProvenance(items[0], records[id], time.Now().UTC())
	if err = tx.Commit(); err != nil {
		return MetadataProvenance{}, err
	}
	return detail, nil
}
