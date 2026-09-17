package wanted

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

// Book choices deliberately omit live download and file-presence observations.
// Choosing an identity must work when external integrations are unavailable.
type BookChoice struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	AuthorName string `json:"authorName"`
	Format     string `json:"format"`
}
type BookChoicesQuery struct {
	Search, Format, SelectedID, Cursor string
	Limit                              int
}
type BookChoices struct {
	Books      []BookChoice `json:"books"`
	Selected   *BookChoice  `json:"selected,omitempty"`
	Total      int          `json:"total"`
	Filtered   int          `json:"filtered"`
	NextCursor string       `json:"nextCursor,omitempty"`
	ObservedAt time.Time    `json:"observedAt"`
}
type bookChoiceCursor struct {
	Search    string    `json:"q"`
	Format    string    `json:"format"`
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
}

var ErrBookChoices = errors.New("invalid book choices query or cursor")

func normalizeBookChoices(q BookChoicesQuery) (BookChoicesQuery, bookChoiceCursor, error) {
	q.Search = strings.TrimSpace(q.Search)
	if q.Format == "" {
		q.Format = "all"
	}
	if q.Limit == 0 {
		q.Limit = 50
	}
	var c bookChoiceCursor
	validID := func(value string) bool { var id pgtype.UUID; return id.Scan(value) == nil && id.Valid }
	if (q.Format != "all" && q.Format != "ebook" && q.Format != "audiobook") || q.Limit < 1 || q.Limit > 100 || len(q.Search) > 256 || len(q.Cursor) > 2048 || (q.SelectedID != "" && !validID(q.SelectedID)) {
		return q, c, ErrBookChoices
	}
	if q.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		if err != nil || json.Unmarshal(raw, &c) != nil || c.Search != q.Search || c.Format != q.Format || c.CreatedAt.IsZero() || !validID(c.ID) {
			return q, c, ErrBookChoices
		}
	}
	return q, c, nil
}

func (s *Service) BookChoices(ctx context.Context, query BookChoicesQuery) (BookChoices, error) {
	page := BookChoices{Books: []BookChoice{}}
	q, c, err := normalizeBookChoices(query)
	if err != nil {
		return page, err
	}
	if !s.Available() {
		return page, sql.ErrConnDone
	}
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	// Owner edits persist in wanted_items; works is only a fallback for legacy blank titles.
	const base = `with choices as (select wi.id,coalesce(nullif(wi.title,''),w.title,'') as title,coalesce(wi.author_name,'') as author_name,wi.wanted_format,wi.created_at from wanted_items wi left join works w on w.id=wi.work_id where wi.status not in ('removed','ignored')) `
	const filter = `($1='all' or wanted_format=$1) and ($2='' or strpos(lower(title||' '||author_name||' '||id::text),lower($2))>0)`
	if err = tx.QueryRowContext(ctx, base+`select now(),count(*),count(*) filter(where `+filter+`) from choices`, q.Format, q.Search).Scan(&page.ObservedAt, &page.Total, &page.Filtered); err != nil {
		return page, err
	}
	if q.SelectedID != "" {
		var selected BookChoice
		err = tx.QueryRowContext(ctx, base+`select id,title,author_name,wanted_format from choices where id=$1 and ($2='all' or wanted_format=$2)`, q.SelectedID, q.Format).Scan(&selected.ID, &selected.Title, &selected.AuthorName, &selected.Format)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return page, err
		}
		if err == nil {
			page.Selected = &selected
		}
	}
	rows, err := tx.QueryContext(ctx, base+`select id,title,author_name,wanted_format,created_at from choices where `+filter+` and (not $3::boolean or (created_at,id)<($4::timestamptz,nullif($5,'')::uuid)) order by created_at desc,id desc limit $6`, q.Format, q.Search, c.ID != "", c.CreatedAt, c.ID, q.Limit+1)
	if err != nil {
		return page, err
	}
	var last bookChoiceCursor
	for rows.Next() {
		var book BookChoice
		var created time.Time
		if err = rows.Scan(&book.ID, &book.Title, &book.AuthorName, &book.Format, &created); err != nil {
			rows.Close()
			return page, err
		}
		page.Books = append(page.Books, book)
		if len(page.Books) <= q.Limit {
			last = bookChoiceCursor{Search: q.Search, Format: q.Format, CreatedAt: created, ID: book.ID}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if len(page.Books) > q.Limit {
		page.Books = page.Books[:q.Limit]
		raw, _ := json.Marshal(last)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, tx.Commit()
}
