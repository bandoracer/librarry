package wanted

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

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrBookPage = errors.New("invalid book page")

type BookCollectionQuery struct {
	Search  string `json:"q"`
	Format  string `json:"format"`
	Monitor string `json:"monitor"`
	State   string `json:"state"`
	Sort    string `json:"sort"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit"`
}
type BookCollection struct {
	Books         []WantedItem   `json:"books"`
	Total         int            `json:"total"`
	Filtered      int            `json:"filtered"`
	Counts        map[string]int `json:"counts"`
	RecordedFiles int            `json:"recordedFiles"`
	NextCursor    string         `json:"nextCursor,omitempty"`
	Downloads     string         `json:"downloads"`
	ObservedAt    time.Time      `json:"observedAt"`
}
type bookCursor struct {
	Query  string `json:"q"`
	Index  int64  `json:"n"`
	First  string `json:"a"`
	Second string `json:"b"`
	ID     string `json:"id"`
}
type collectionProfile struct {
	Name    string  `json:"name"`
	Format  string  `json:"format"`
	Cutoff  float64 `json:"cutoff"`
	Upgrade bool    `json:"upgrade"`
}

func normalizeBookQuery(q BookCollectionQuery) (BookCollectionQuery, bookCursor, error) {
	q.Search = strings.TrimSpace(q.Search)
	defaults := []*string{&q.Format, &q.Monitor, &q.State}
	for _, field := range defaults {
		if *field == "" {
			*field = "all"
		}
	}
	if q.Sort == "" {
		q.Sort = "status"
	}
	if q.Limit == 0 {
		q.Limit = 100
	}
	valid := func(value string, allowed ...string) bool {
		for _, a := range allowed {
			if value == a {
				return true
			}
		}
		return false
	}
	if len(q.Search) > 256 || q.Limit < 1 || q.Limit > 100 || !valid(q.Format, "all", "ebook", "audiobook") || !valid(q.Monitor, "all", "monitored", "unmonitored") || !valid(q.State, "all", "missing", "incomplete", "unknown", "downloading", "cutoffUnmet", "downloaded", "unmonitored") || !valid(q.Sort, "status", "title", "author", "added") {
		return q, bookCursor{}, ErrBookPage
	}
	var cursor bookCursor
	if q.Cursor != "" {
		if len(q.Cursor) > 8192 {
			return q, cursor, ErrBookPage
		}
		raw, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Query != bookQueryKey(q) {
			return q, cursor, ErrBookPage
		}
		var id pgtype.UUID
		if id.Scan(cursor.ID) != nil || !id.Valid {
			return q, cursor, ErrBookPage
		}
	}
	return q, cursor, nil
}
func bookQueryKey(q BookCollectionQuery) string {
	q.Cursor = ""
	q.Limit = 0
	raw, _ := json.Marshal(q)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// BookCollection evaluates counts and a bounded page against one database
// snapshot and one live-client observation. It does not freeze later page requests.
func (s *Service) BookCollection(ctx context.Context, query BookCollectionQuery) (BookCollection, error) {
	page := BookCollection{Books: []WantedItem{}, Counts: map[string]int{"missing": 0, "incomplete": 0, "unknown": 0, "downloading": 0, "cutoffUnmet": 0, "downloaded": 0, "unmonitored": 0}}
	query, cursor, err := normalizeBookQuery(query)
	if err != nil {
		return page, err
	}
	if !s.Available() {
		return page, errors.New("book collection requires database persistence")
	}
	tx, evidenceArgs, err := s.collectionSnapshot(ctx)
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	page.Downloads = evidenceArgs[2].(string)
	page.ObservedAt = time.Now().UTC()
	args := append(evidenceArgs, query.Search, query.Format, query.Monitor, query.State)
	rows, err := tx.QueryContext(ctx, bookCollectionSQL+`select derived_state,count(*),count(*) filter(where `+bookFilterSQL+`) from stateful group by derived_state`, args...)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var state string
		var n, filtered int
		if err := rows.Scan(&state, &n, &filtered); err != nil {
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
	if err = tx.QueryRowContext(ctx, `select count(*) from files`).Scan(&page.RecordedFiles); err != nil {
		return page, err
	}
	indexExpr, firstExpr, secondExpr := "0::bigint", "lower(title)", "lower(author_name)"
	switch query.Sort {
	case "author":
		firstExpr, secondExpr = "lower(author_name)", "lower(title)"
	case "added":
		indexExpr = "-(extract(epoch from created_at)*1000000)::bigint"
		firstExpr, secondExpr = "lower(author_name)", "lower(title)"
	case "status":
		indexExpr = `case derived_state when 'missing' then 0 when 'incomplete' then 1 when 'unknown' then 2 when 'downloading' then 3 when 'cutoffUnmet' then 4 when 'downloaded' then 5 else 6 end::bigint`
		firstExpr, secondExpr = "lower(author_name)", "lower(title)"
	}
	args = append(args, cursor.ID != "", cursor.Index, cursor.First, cursor.Second, cursor.ID, query.Limit+1)
	rows, err = tx.QueryContext(ctx, bookCollectionSQL+`, ordered as (select *,`+indexExpr+` as sort_index,`+firstExpr+` collate "C" as sort_first,`+secondExpr+` collate "C" as sort_second from stateful where `+bookFilterSQL+`)
 , paged as materialized (select * from ordered b
 where not $8::boolean or (b.sort_index,b.sort_first,b.sort_second,b.id)>($9::bigint,$10::text collate "C",$11::text collate "C",nullif($12,'')::uuid)
 order by b.sort_index,b.sort_first,b.sort_second,b.id limit $13)
 select `+wantedDetailColumns+`,b.derived_state,b.file_state,b.file_reason,b.present_files,b.required_files,b.sort_index,b.sort_first,b.sort_second
 from paged b join wanted_items wi on wi.id=b.id left join works w on w.id=wi.work_id
 order by b.sort_index,b.sort_first,b.sort_second,b.id`, args...)
	if err != nil {
		return page, err
	}
	var last bookCursor
	for rows.Next() {
		var state string
		var evidence FileEvidence
		var next bookCursor
		item, err := scanWanted(wantedWithExtra{row: rows, extra: []any{&state, &evidence.State, &evidence.Reason, &evidence.PresentFiles, &evidence.RequiredFiles, &next.Index, &next.First, &next.Second}})
		if err != nil {
			rows.Close()
			return page, err
		}
		item.DerivedState = state
		item.StateEvidence = &BookStateEvidence{Files: evidence, Downloads: page.Downloads, Quality: "available"}
		if evidence.State != "present" && evidence.State != "missing" {
			item.StateEvidence.Message = evidence.Reason
		}
		if page.Downloads == "partial" || page.Downloads == "unavailable" {
			item.StateEvidence.Message = strings.TrimSpace(item.StateEvidence.Message + " Download-client evidence is unavailable or incomplete.")
		}
		page.Books = append(page.Books, item)
		if len(page.Books) <= query.Limit {
			last = next
			last.ID = item.ID
			last.Query = bookQueryKey(query)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if len(page.Books) > query.Limit {
		page.Books = page.Books[:query.Limit]
		raw, _ := json.Marshal(last)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	page.Books, err = attachWantedDetails(ctx, tx, page.Books)
	if err != nil {
		return page, err
	}
	if err = tx.Commit(); err != nil {
		return page, err
	}
	return page, nil
}

type wantedWithExtra struct {
	row   wantedScanner
	extra []any
}

func (s wantedWithExtra) Scan(dest ...any) error { return s.row.Scan(append(dest, s.extra...)...) }

const bookFilterSQL = `($4='' or strpos(lower(title||' '||author_name||' '||quality_profile||' '||metadata_provider||' '||derived_state),lower($4))>0)
 and ($5='all' or wanted_format=$5) and ($6='all' or monitored=($6='monitored')) and ($7='all' or derived_state=$7)`

// Join evidence to wanted IDs before enriching works/profiles. Otherwise new
// work indexes can make a misestimated evidence projection rescan every wanted
// row for every book. Materialize derived states before cursor filtering too.
const bookCollectionSQL = `with profiles as (
 select * from jsonb_to_recordset($1::jsonb) as p(name text,format text,cutoff double precision,upgrade boolean)
), tracked as materialized (
 select wi.id,wi.work_id,wi.title,wi.author_name,wi.wanted_format,wi.quality_profile,
 wi.metadata_provider,wi.monitored,wi.created_at,wi.release_date,wi.current_release_id,wi.current_release_score,
 e.file_state,e.file_reason,e.present_files,e.required_files
 from wanted_items wi join librarry_book_file_evidence(null) e on e.wanted_id=wi.id
 where wi.status not in ('removed','ignored')
), base as (
 select wi.id,coalesce(nullif(wi.title,''),w.title,'') as title,coalesce(wi.author_name,'') as author_name,
 wi.wanted_format,coalesce(wi.quality_profile,'standard') as quality_profile,coalesce(wi.metadata_provider,'') as metadata_provider,wi.monitored,wi.created_at,wi.release_date,
 case when wi.current_release_id is null then 0 else wi.current_release_score end as score,
 coalesce(p.cutoff,d.cutoff) as cutoff,coalesce(p.upgrade,d.upgrade) as upgrade,
 wi.file_state,wi.file_reason,wi.present_files,wi.required_files
 from tracked wi left join works w on w.id=wi.work_id
 left join profiles p on p.name=coalesce(nullif(lower(btrim(wi.quality_profile)),''),'standard') and p.format=wi.wanted_format
 join profiles d on d.name='' and d.format=wi.wanted_format
), stateful as materialized (
 select *,case
 when file_state='present' then case when monitored and upgrade and not(score>0 and score<1000) and score<cutoff then 'cutoffUnmet' else 'downloaded' end
 when id=any($2::uuid[]) then 'downloading'
 when file_state='incomplete' then 'incomplete'
 when file_state in ('unknown','unavailable') or $3 in ('partial','unavailable') then 'unknown'
 when not monitored then 'unmonitored' else 'missing' end as derived_state from base
) `

// collectionSnapshot shares quality and client evidence across native collection reads.
func (s *Service) collectionSnapshot(ctx context.Context) (*sql.Tx, []any, error) {
	downloads := s.liveBookDownloads(ctx)
	switch downloads.Status {
	case "fresh", "notConfigured", "partial", "unavailable":
	default:
		downloads.Status = "unavailable"
	}

	var inFlight []string
	for id, items := range groupDownloadsByWantedID(downloads.Downloads) {
		var parsed pgtype.UUID
		if parsed.Scan(id) != nil || !parsed.Valid {
			continue
		}
		for _, item := range items {
			if downloadSupportsInFlight(item) {
				inFlight = append(inFlight, id)
				break
			}
		}
	}
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	completed := false
	defer func() {
		if !completed {
			tx.Rollback()
		}
	}()
	// These bounded interactive reads spend more time compiling the inlined
	// evidence expression than executing it (over 500 ms of JIT in the 10k
	// fixture). Keep this local to the read transaction, not the database/session.
	if _, err = tx.ExecContext(ctx, `set local jit=off`); err != nil {
		return nil, nil, err
	}
	// Cursor/filter selectivity varies between pages; avoid the generic prepared
	// plan that rescans the evidence set through nested loops.
	if _, err = tx.ExecContext(ctx, `set local plan_cache_mode=force_custom_plan`); err != nil {
		return nil, nil, err
	}
	profiles, err := listQualityProfiles(ctx, tx)
	if err != nil {
		return nil, nil, err
	}
	names := map[string]bool{"": true}
	for _, p := range profiles {
		names[normalizeQualityProfile(p.Name)] = true
	}
	var cutoffs []collectionProfile
	for name := range names {
		for _, format := range []string{"ebook", "audiobook"} {
			p := defaultQualityProfile(name, format)
			if name != "" {
				p = profileFromList(profiles, WantedItem{QualityProfile: name, Format: format})
			}
			cutoffs = append(cutoffs, collectionProfile{Name: name, Format: format, Cutoff: p.CutoffCompositeScore(), Upgrade: p.UpgradeAllowed})
		}
	}
	profileJSON, err := json.Marshal(cutoffs)
	if err != nil {
		return nil, nil, err
	}
	completed = true
	return tx, []any{string(profileJSON), inFlight, downloads.Status}, nil
}
