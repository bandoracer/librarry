package wanted

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type RemovedBooksQuery struct {
	Search, Format, Status, Cursor string
	Limit                          int
}
type RemovedBooks struct {
	Books      []WantedItem   `json:"books"`
	Total      int            `json:"total"`
	Filtered   int            `json:"filtered"`
	Counts     map[string]int `json:"counts"`
	NextCursor string         `json:"nextCursor,omitempty"`
	ObservedAt time.Time      `json:"observedAt"`
}
type removedBookCursor struct {
	Status   string           `json:"status"`
	Position bookChoiceCursor `json:"position"`
}

var ErrRemovedBooks = errors.New("invalid removed book filters or cursor")
var ErrRestoreBook = errors.New("valid book ID and reviewed updatedAt are required")
var ErrRestoreBookChanged = errors.New("Book changed or is already active; refresh before restoring")

type RestoreBookRequest struct {
	UpdatedAt time.Time `json:"updatedAt"`
	Monitored bool      `json:"monitored"`
}

func normalizeRemovedBooks(q RemovedBooksQuery) (RemovedBooksQuery, bookChoiceCursor, error) {
	if q.Status == "" {
		q.Status = "removed"
	}
	if q.Status != "removed" && q.Status != "ignored" && q.Status != "all" {
		return q, bookChoiceCursor{}, ErrRemovedBooks
	}
	cursor := ""
	if q.Cursor != "" {
		var c removedBookCursor
		if len(q.Cursor) > 4096 {
			return q, c.Position, ErrRemovedBooks
		}
		raw, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		if err != nil || json.Unmarshal(raw, &c) != nil || c.Status != q.Status {
			return q, c.Position, ErrRemovedBooks
		}
		raw, _ = json.Marshal(c.Position)
		cursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	normalized, c, err := normalizeBookChoices(BookChoicesQuery{Search: q.Search, Format: q.Format, Cursor: cursor, Limit: q.Limit})
	q.Search = normalized.Search
	q.Format = normalized.Format
	q.Limit = normalized.Limit
	if err != nil {
		return q, c, ErrRemovedBooks
	}
	return q, c, nil
}

func (s *Service) RemovedBooks(ctx context.Context, query RemovedBooksQuery) (RemovedBooks, error) {
	page := RemovedBooks{Books: []WantedItem{}, Counts: map[string]int{"removed": 0, "ignored": 0}}
	q, c, err := normalizeRemovedBooks(query)
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
	var removed, ignored int
	if err = tx.QueryRowContext(ctx, `select now(),count(*),count(*) filter(where status='removed'),count(*) filter(where status='ignored') from wanted_items where status in ('removed','ignored')`).Scan(&page.ObservedAt, &page.Total, &removed, &ignored); err != nil {
		return page, err
	}
	page.Counts["removed"] = removed
	page.Counts["ignored"] = ignored
	filter := `wi.status in ('removed','ignored') and ($1='all' or wi.status=$1) and ($2='all' or wi.wanted_format=$2) and ($3='' or strpos(lower(coalesce(nullif(wi.title,''),w.title,'')||' '||wi.author_name||' '||wi.id::text),lower($3))>0)`
	args := []any{q.Status, q.Format, q.Search}
	if err = tx.QueryRowContext(ctx, `select count(*) from wanted_items wi left join works w on w.id=wi.work_id where `+filter, args...).Scan(&page.Filtered); err != nil {
		return page, err
	}
	rows, err := tx.QueryContext(ctx, `select `+wantedDetailColumns+` from wanted_items wi left join works w on w.id=wi.work_id where `+filter+` and (not $4::boolean or (wi.created_at,wi.id)<($5::timestamptz,nullif($6,'')::uuid)) order by wi.created_at desc,wi.id desc limit $7`, append(args, c.ID != "", c.CreatedAt, c.ID, q.Limit+1)...)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		item, e := scanWanted(rows)
		if e != nil {
			rows.Close()
			return page, e
		}
		page.Books = append(page.Books, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if len(page.Books) > q.Limit {
		page.Books = page.Books[:q.Limit]
		last := page.Books[q.Limit-1]
		raw, _ := json.Marshal(removedBookCursor{Status: q.Status, Position: bookChoiceCursor{Search: q.Search, Format: q.Format, CreatedAt: last.CreatedAt, ID: last.ID}})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	page.Books, err = attachWantedDetails(ctx, tx, page.Books)
	if err != nil {
		return page, err
	}
	return page, tx.Commit()
}

// Restore changes only tracking state and the explicitly chosen monitoring flag.
// File links, metadata, roots, profiles, author policy and history are retained.
func (s *Service) RestoreBook(ctx context.Context, id string, request RestoreBookRequest) (WantedItem, error) {
	var parsed pgtype.UUID
	if parsed.Scan(id) != nil || !parsed.Valid || request.UpdatedAt.IsZero() {
		return WantedItem{}, ErrRestoreBook
	}
	if !s.Available() {
		return WantedItem{}, sql.ErrConnDone
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return WantedItem{}, err
	}
	defer tx.Rollback()
	items, _, err := reviewBooks(ctx, tx, []string{id}, true)
	if err != nil {
		return WantedItem{}, err
	}
	if len(items) == 0 {
		return WantedItem{}, sql.ErrNoRows
	}
	item := items[0]
	if (item.Status != "removed" && item.Status != "ignored") || !item.UpdatedAt.Equal(request.UpdatedAt) {
		return WantedItem{}, ErrRestoreBookChanged
	}
	if _, err = tx.ExecContext(ctx, `update wanted_items set status='wanted',monitored=$2,updated_at=now() where id=$1`, id, request.Monitored); err != nil {
		return WantedItem{}, err
	}
	data, _ := json.Marshal(map[string]any{"previousStatus": item.Status, "monitored": request.Monitored, "title": item.Title})
	if _, err = tx.ExecContext(ctx, `insert into history_events(event_type,entity_type,entity_id,severity,message,data) values('wanted_restored','wanted_item',$1,'info','Book restored to Library',$2::jsonb)`, id, string(data)); err != nil {
		return WantedItem{}, err
	}
	item, err = wantedInTransaction(ctx, tx, id)
	if err != nil {
		return WantedItem{}, err
	}
	return item, tx.Commit()
}
