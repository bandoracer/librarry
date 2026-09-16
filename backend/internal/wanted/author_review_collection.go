package wanted

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

type AuthorReviewCollection struct {
	Reviews    []AuthorMetadataReview `json:"reviews"`
	Total      int                    `json:"total"`
	Filtered   int                    `json:"filtered"`
	Counts     map[string]int         `json:"counts"`
	NextCursor string                 `json:"nextCursor,omitempty"`
	ObservedAt time.Time              `json:"observedAt"`
}

const authorReviewColumns = `id,coalesce(author_subscription_id::text,''),provider,candidate_key,title,author_name,wanted_format,quality_profile,tags,policy,reason,status,decision,coalesce(wanted_item_id::text,''),result,created_at,updated_at,resolved_at,coalesce(root_folder_id::text,'')`

func authorReviewRevision(review AuthorMetadataReview) string {
	review.Revision = ""
	raw, _ := json.Marshal(review)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func normalizeAuthorReviewQuery(query AuthorMetadataReviewQuery) (BookCollectionQuery, bookCursor, string, error) {
	if query.Status == "" {
		query.Status = "pending"
	}
	switch query.Status {
	case "pending", "wanted", "ignored", "all":
	default:
		return BookCollectionQuery{}, bookCursor{}, "", ErrBookPage
	}
	prefix := "author-review:" + query.Status + ":"
	if query.Cursor != "" {
		if len(query.Cursor) > 8192 {
			return BookCollectionQuery{}, bookCursor{}, "", ErrBookPage
		}
		raw, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		var cursor bookCursor
		if err != nil || json.Unmarshal(raw, &cursor) != nil || !strings.HasPrefix(cursor.Query, prefix) {
			return BookCollectionQuery{}, cursor, "", ErrBookPage
		}
		cursor.Query = strings.TrimPrefix(cursor.Query, prefix)
		raw, _ = json.Marshal(cursor)
		query.Cursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	q, cursor, err := normalizeBookQuery(BookCollectionQuery{Search: query.Search, Format: query.Format, Sort: "added", Cursor: query.Cursor, Limit: query.Limit})
	return q, cursor, query.Status, err
}

func (s *Service) AuthorReviewCollection(ctx context.Context, query AuthorMetadataReviewQuery) (AuthorReviewCollection, error) {
	if _, _, _, err := normalizeAuthorReviewQuery(query); err != nil {
		return AuthorReviewCollection{}, err
	}
	if !s.Available() {
		return AuthorReviewCollection{}, sql.ErrConnDone
	}
	return s.store.AuthorReviewCollection(ctx, query)
}

func (s *Store) AuthorReviewCollection(ctx context.Context, query AuthorMetadataReviewQuery) (AuthorReviewCollection, error) {
	page := AuthorReviewCollection{Reviews: []AuthorMetadataReview{}, Counts: map[string]int{"pending": 0, "wanted": 0, "ignored": 0}, ObservedAt: time.Now().UTC()}
	q, cursor, status, err := normalizeAuthorReviewQuery(query)
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
	filter := `($1='all' or status=$1) and ($2='all' or wanted_format=$2) and ($3='' or strpos(lower(title||' '||author_name||' '||provider||' '||reason||' '||policy),lower($3))>0)`
	args := []any{status, q.Format, q.Search}
	rows, err := tx.QueryContext(ctx, `select status,count(*),count(*) filter(where `+filter+`) from author_metadata_reviews group by status`, args...)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var state string
		var n, filtered int
		if err = rows.Scan(&state, &n, &filtered); err != nil {
			rows.Close()
			return page, err
		}
		page.Counts[state] = n
		page.Total += n
		page.Filtered += filtered
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	rows, err = tx.QueryContext(ctx, `select `+authorReviewColumns+`,-(extract(epoch from created_at)*1000000)::bigint from author_metadata_reviews where `+filter+`
 and (not $4::boolean or (-(extract(epoch from created_at)*1000000)::bigint,id)>($5::bigint,nullif($6,'')::uuid)) order by created_at desc,id limit $7`, append(args, cursor.ID != "", cursor.Index, cursor.ID, q.Limit+1)...)
	if err != nil {
		return page, err
	}
	var last bookCursor
	for rows.Next() {
		var index int64
		review, e := scanAuthorMetadataReview(wantedWithExtra{row: rows, extra: []any{&index}})
		if e != nil {
			rows.Close()
			return page, e
		}
		page.Reviews = append(page.Reviews, review)
		if len(page.Reviews) <= q.Limit {
			last = bookCursor{Query: "author-review:" + status + ":" + bookQueryKey(q), Index: index, ID: review.ID}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if len(page.Reviews) > q.Limit {
		page.Reviews = page.Reviews[:q.Limit]
		raw, _ := json.Marshal(last)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	if err = tx.Commit(); err != nil {
		return page, err
	}
	return page, nil
}
