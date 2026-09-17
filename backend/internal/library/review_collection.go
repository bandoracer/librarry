package library

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ImportReviewQuery struct {
	Status string `json:"status"`
	Format string `json:"format"`
	Kind   string `json:"kind"`
	Search string `json:"q"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}

type ImportReviewCollection struct {
	Reviews    []ImportReview `json:"reviews"`
	Total      int            `json:"total"`
	Filtered   int            `json:"filtered"`
	Counts     map[string]int `json:"counts"`
	NextCursor string         `json:"nextCursor,omitempty"`
	ObservedAt time.Time      `json:"observedAt"`
}

type importReviewCursor struct {
	Query     ImportReviewQuery `json:"query"`
	CreatedAt time.Time         `json:"createdAt"`
	ID        string            `json:"id"`
}

var ErrImportReviewPage = errors.New("invalid import review page")

func normalizeImportReviewQuery(q ImportReviewQuery) (ImportReviewQuery, importReviewCursor, error) {
	q.Search = strings.TrimSpace(q.Search)
	if q.Status == "" {
		q.Status = "pending"
	}
	if q.Format == "" {
		q.Format = "all"
	}
	if q.Kind == "" {
		q.Kind = "all"
	}
	if q.Limit == 0 {
		q.Limit = 50
	}
	valid := func(value string, values ...string) bool {
		for _, v := range values {
			if v == value {
				return true
			}
		}
		return false
	}
	var c importReviewCursor
	if !valid(q.Status, "pending", "resolved", "all") || !valid(q.Format, "all", "ebook", "audiobook", "unknown") || !valid(q.Kind, "all", "file", "payload") || q.Limit < 1 || q.Limit > 100 || len(q.Search) > 256 || len(q.Cursor) > 2048 {
		return q, c, ErrImportReviewPage
	}
	if q.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		var id pgtype.UUID
		expected := q
		expected.Cursor = ""
		expected.Limit = 0
		if err != nil || json.Unmarshal(raw, &c) != nil || c.Query != expected || c.CreatedAt.IsZero() || id.Scan(c.ID) != nil || !id.Valid {
			return q, c, ErrImportReviewPage
		}
	}
	return q, c, nil
}

func (s *Service) ImportReviewCollection(ctx context.Context, query ImportReviewQuery) (ImportReviewCollection, error) {
	page := ImportReviewCollection{Reviews: []ImportReview{}, Counts: map[string]int{"pending": 0, "resolved": 0}}
	q, c, err := normalizeImportReviewQuery(query)
	if err != nil {
		return page, err
	}
	if !s.Available() {
		return page, errors.New("import review collection requires database persistence")
	}
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	var pending, resolved int
	if err = tx.QueryRowContext(ctx, `select now(),count(*),count(*) filter(where status='pending'),count(*) filter(where status<>'pending') from import_reviews`).Scan(&page.ObservedAt, &page.Total, &pending, &resolved); err != nil {
		return page, err
	}
	page.Counts["pending"] = pending
	page.Counts["resolved"] = resolved
	filter := `($1='all' or ($1='pending' and status='pending') or ($1='resolved' and status<>'pending'))
 and ($2='all' or media_format=$2)
 and ($3='all' or ($3='payload')=coalesce(metadata->'payloadReview'='true'::jsonb,false))
 and ($4='' or strpos(lower(title||' '||author_name||' '||source_path||' '||reason),lower($4))>0)`
	args := []any{q.Status, q.Format, q.Kind, q.Search}
	if err = tx.QueryRowContext(ctx, `select count(*) from import_reviews where `+filter, args...).Scan(&page.Filtered); err != nil {
		return page, err
	}
	rows, err := tx.QueryContext(ctx, `select id,source_path,download_id,coalesce(wanted_item_id::text,''),media_format,title,author_name,coalesce(size_bytes,0),reason,status,decision,destination_path,metadata,created_at,updated_at,resolved_at
 from import_reviews where `+filter+` and (not $5::boolean or (created_at,id)<($6::timestamptz,nullif($7,'')::uuid))
 order by created_at desc,id desc limit $8`, append(args, c.ID != "", c.CreatedAt, c.ID, q.Limit+1)...)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		r, e := scanImportReview(rows)
		if e != nil {
			rows.Close()
			return page, e
		}
		page.Reviews = append(page.Reviews, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if len(page.Reviews) > q.Limit {
		page.Reviews = page.Reviews[:q.Limit]
		last := page.Reviews[q.Limit-1]
		q.Cursor = ""
		q.Limit = 0
		raw, _ := json.Marshal(importReviewCursor{Query: q, CreatedAt: last.CreatedAt, ID: last.ID})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, tx.Commit()
}
