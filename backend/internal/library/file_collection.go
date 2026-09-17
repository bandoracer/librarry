package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrFilePage = errors.New("invalid file collection filters or cursor; refresh the collection")

type FileCollectionQuery struct {
	Search   string `json:"q"`
	WantedID string `json:"wantedId"`
	Format   string `json:"format"`
	Presence string `json:"presence"`
	Sort     string `json:"sort"`
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit"`
}
type FileCollectionItem struct {
	FileRecord
	WantedIDs []string `json:"wantedIds"`
}
type FileCollection struct {
	Files      []FileCollectionItem `json:"files"`
	Total      int                  `json:"total"`
	Filtered   int                  `json:"filtered"`
	Counts     map[string]int       `json:"counts"`
	NextCursor string               `json:"nextCursor,omitempty"`
	ObservedAt time.Time            `json:"observedAt"`
}
type fileCursor struct {
	Query  string `json:"q"`
	Index  int64  `json:"n"`
	First  string `json:"a"`
	Second string `json:"b"`
	ID     string `json:"id"`
}

func fileQueryKey(q FileCollectionQuery) string {
	q.Cursor = ""
	q.Limit = 0
	raw, _ := json.Marshal(q)
	sum := sha256.Sum256(raw)
	return "files:" + hex.EncodeToString(sum[:])
}
func normalizeFileQuery(q FileCollectionQuery) (FileCollectionQuery, fileCursor, error) {
	q.Search = strings.TrimSpace(q.Search)
	q.WantedID = strings.ToLower(strings.TrimSpace(q.WantedID))
	if q.Format == "" || q.Format == "any" {
		q.Format = "all"
	}
	if q.Presence == "" {
		q.Presence = "all"
	}
	if q.Sort == "" {
		q.Sort = "path"
	}
	if q.Limit == 0 {
		q.Limit = 100
	}
	valid := func(s string, values ...string) bool {
		for _, v := range values {
			if s == v {
				return true
			}
		}
		return false
	}
	if len(q.Search) > 256 || q.Limit < 1 || q.Limit > 100 || q.WantedID != "" && !repairUUID.MatchString(q.WantedID) || !valid(q.Format, "all", "ebook", "audiobook") || !valid(q.Presence, "all", "present", "missing", "unknown") || !valid(q.Sort, "path", "title", "updated") {
		return q, fileCursor{}, ErrFilePage
	}
	var cursor fileCursor
	if q.Cursor != "" {
		if len(q.Cursor) > 8192 {
			return q, cursor, ErrFilePage
		}
		raw, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Query != fileQueryKey(q) || !repairUUID.MatchString(cursor.ID) {
			return q, cursor, ErrFilePage
		}
	}
	return q, cursor, nil
}

// FileCollection reports recorded presence only; it never probes media or calls
// providers. Book membership comes from relational links, not legacy JSON hints.
func (s *Service) FileCollection(ctx context.Context, q FileCollectionQuery) (FileCollection, error) {
	if _, _, err := normalizeFileQuery(q); err != nil {
		return FileCollection{}, err
	}
	if !s.Available() {
		return FileCollection{}, sql.ErrConnDone
	}
	return s.store.FileCollection(ctx, q)
}

const visibleFileScope = `with visible as materialized (
 select f.id,f.path,f.title,f.author_name,f.media_format,f.import_status,f.presence_state,f.updated_at from files f
 where not exists(select 1 from import_operation_files pf join import_operations op on op.id=pf.operation_id where pf.destination_path=f.path and op.state<>'committed')
 and ($1='' or exists(select 1 from file_wanted_links fl where fl.file_id=f.id and fl.wanted_item_id=nullif($1,'')::uuid))
)`
const fileCollectionFilter = `($2='all' or media_format=$2) and ($3='all' or presence_state=$3) and ($4='' or strpos(lower(path||' '||title||' '||author_name),lower($4))>0)`

type fileWithExtra struct {
	row   fileScanner
	extra []any
}

func (s fileWithExtra) Scan(dest ...any) error { return s.row.Scan(append(dest, s.extra...)...) }

func (s *Store) FileCollection(ctx context.Context, query FileCollectionQuery) (FileCollection, error) {
	page := FileCollection{Files: []FileCollectionItem{}, Counts: map[string]int{}, ObservedAt: time.Now().UTC()}
	q, cursor, err := normalizeFileQuery(query)
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
	args := []any{q.WantedID, q.Format, q.Presence, q.Search}
	var imported, ebook, audio, present, missing, unknown int
	err = tx.QueryRowContext(ctx, visibleFileScope+` select count(*),count(*) filter(where `+fileCollectionFilter+`),count(*) filter(where import_status='imported'),count(*) filter(where media_format='ebook'),count(*) filter(where media_format='audiobook'),count(*) filter(where presence_state='present'),count(*) filter(where presence_state='missing'),count(*) filter(where presence_state='unknown') from visible`, args...).Scan(&page.Total, &page.Filtered, &imported, &ebook, &audio, &present, &missing, &unknown)
	if err != nil {
		return page, err
	}
	page.Counts = map[string]int{"imported": imported, "ebook": ebook, "audiobook": audio, "present": present, "missing": missing, "unknown": unknown}
	rows, err := tx.QueryContext(ctx, visibleFileScope+`, ordered as (
 select id,case when $5='updated' then -(extract(epoch from updated_at)*1000000)::bigint else 0 end as sort_index,
 case when $5='path' then path when $5='title' then lower(title) else '' end collate "C" as first,
 case when $5='title' then path else '' end collate "C" as second
 from visible where `+fileCollectionFilter+`
 ), page as materialized (
 select * from ordered where (not $6::boolean or (sort_index,first,second,id)>($7::bigint,$8::text collate "C",$9::text collate "C",nullif($10,'')::uuid)) order by sort_index,first,second,id limit $11
 ) select f.id,coalesce(f.edition_id::text,''),f.media_format,f.path,f.source_path,f.title,f.author_name,f.extension,coalesce(f.size_bytes,0),coalesce(f.checksum,''),f.import_status,f.metadata,f.modified_at,f.created_at,f.updated_at,f.presence_state,
 coalesce((select jsonb_agg(fl.wanted_item_id::text order by fl.wanted_item_id) from file_wanted_links fl where fl.file_id=f.id),'[]'::jsonb),p.sort_index,p.first,p.second
 from page p join files f on f.id=p.id order by p.sort_index,p.first,p.second,p.id`, append(args, q.Sort, cursor.ID != "", cursor.Index, cursor.First, cursor.Second, cursor.ID, q.Limit+1)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var last fileCursor
	for rows.Next() {
		var links []byte
		var next fileCursor
		file, e := scanFile(fileWithExtra{row: rows, extra: []any{&links, &next.Index, &next.First, &next.Second}})
		if e != nil {
			return page, e
		}
		ids := []string{}
		if e = json.Unmarshal(links, &ids); e != nil {
			return page, e
		}
		page.Files = append(page.Files, FileCollectionItem{FileRecord: file, WantedIDs: ids})
		if len(page.Files) <= q.Limit {
			next.Query = fileQueryKey(q)
			next.ID = file.ID
			last = next
		}
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	rows.Close()
	if len(page.Files) > q.Limit {
		page.Files = page.Files[:q.Limit]
		raw, _ := json.Marshal(last)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, tx.Commit()
}
