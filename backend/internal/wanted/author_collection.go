package wanted

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// The Authors settings collection pages subscriptions, not deduplicated people.
// Each provider/format subscription retains its own monitoring policy.
type AuthorCollectionQuery struct {
	Search string `json:"q"`
	Format string `json:"format"`
	Status string `json:"status"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}
type AuthorCollectionItem struct {
	AuthorSubscription
	Counts         map[string]int `json:"counts"`
	TotalBooks     int            `json:"totalBooks"`
	IdentityLinked bool           `json:"identityLinked"`
}
type AuthorCollection struct {
	Authors    []AuthorCollectionItem `json:"authors"`
	Total      int                    `json:"total"`
	Filtered   int                    `json:"filtered"`
	NextCursor string                 `json:"nextCursor,omitempty"`
	Downloads  string                 `json:"downloads"`
	ObservedAt time.Time              `json:"observedAt"`
}

func (s *Service) AuthorCollection(ctx context.Context, query AuthorCollectionQuery) (AuthorCollection, error) {
	page := AuthorCollection{Authors: []AuthorCollectionItem{}}
	if query.Status == "" {
		query.Status = "monitored"
	}
	// Reuse the native filter/cursor contract, with a separate cursor namespace.
	rawCursor := query.Cursor
	if rawCursor != "" {
		if len(rawCursor) > 8192 {
			return page, ErrBookPage
		}
		raw, err := base64.RawURLEncoding.DecodeString(rawCursor)
		var c bookCursor
		if err != nil || json.Unmarshal(raw, &c) != nil || len(c.Query) < 8 || c.Query[:8] != "authors:" {
			return page, ErrBookPage
		}
		c.Query = c.Query[8:]
		raw, _ = json.Marshal(c)
		query.Cursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	q, cursor, err := normalizeBookQuery(BookCollectionQuery{Search: query.Search, Format: query.Format, Monitor: query.Status, Sort: "author", Cursor: query.Cursor, Limit: query.Limit})
	if err != nil {
		return page, err
	}
	if !s.Available() {
		return page, errors.New("author collection requires database persistence")
	}
	tx, args, err := s.collectionSnapshot(ctx)
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	page.Downloads = args[2].(string)
	page.ObservedAt = time.Now().UTC()
	filter := `status<>'removed' and ($1='' or strpos(lower(author_name||' '||provider||' '||provider_key),lower($1))>0) and ($2='all' or wanted_format=$2) and ($3='all' or status=$3)`
	filters := []any{q.Search, q.Format, q.Monitor}
	if err = tx.QueryRowContext(ctx, `select count(*) filter(where status<>'removed'),count(*) filter(where `+filter+`) from author_subscriptions`, filters...).Scan(&page.Total, &page.Filtered); err != nil {
		return page, err
	}
	rows, err := tx.QueryContext(ctx, `select id,provider,provider_key,author_name,wanted_format,quality_profile,status,monitor_new_items,missing_book_policy,tags,allowed_languages,must_not_contain,skip_missing_isbn,min_pages,coalesce(metadata_profile_id::text,''),coalesce(root_folder_id::text,''),last_sync_at,created_at,updated_at,lower(author_name),wanted_format from author_subscriptions where `+filter+`
 and (not $4::boolean or (lower(author_name) collate "C",wanted_format collate "C",id)>($5::text collate "C",$6::text collate "C",nullif($7,'')::uuid)) order by lower(author_name) collate "C",wanted_format collate "C",id limit $8`, append(filters, cursor.ID != "", cursor.First, cursor.Second, cursor.ID, q.Limit+1)...)
	if err != nil {
		return page, err
	}
	var last bookCursor
	for rows.Next() {
		var next bookCursor
		sub, e := scanAuthorSubscription(wantedWithExtra{row: rows, extra: []any{&next.First, &next.Second}})
		if e != nil {
			rows.Close()
			return page, e
		}
		page.Authors = append(page.Authors, AuthorCollectionItem{AuthorSubscription: sub, Counts: map[string]int{}})
		if len(page.Authors) <= q.Limit {
			last = next
			last.ID = sub.ID
			last.Query = "authors:" + bookQueryKey(q)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if len(page.Authors) > q.Limit {
		page.Authors = page.Authors[:q.Limit]
		raw, _ := json.Marshal(last)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	ids := []string{}
	byID := map[string]*AuthorCollectionItem{}
	for i := range page.Authors {
		item := &page.Authors[i]
		ids = append(ids, item.ID)
		byID[item.ID] = item
	}
	if len(ids) > 0 {
		// Ambiguous legacy provider mappings are not enough evidence to attribute books.
		// Distinct membership prevents multiple aliases or writer links double-counting.
		rows, err = tx.QueryContext(ctx, bookCollectionSQL+`, identities as materialized (
   select s.id,s.wanted_format,case when count(distinct p.entity_id)=1 then min(p.entity_id::text)::uuid end as author_id
   from author_subscriptions s left join provider_records p on p.entity_type='author' and lower(p.provider)=lower(s.provider) and lower(p.provider_key)=lower(s.provider_key)
   where s.id=any($4::uuid[]) group by s.id,s.wanted_format
  ), members as (
   select distinct i.id,b.id as book_id,b.derived_state from identities i
   join work_authors wa on wa.author_id=i.author_id and lower(wa.role) in ('author','writer')
   join wanted_items wi on wi.work_id=wa.work_id and wi.wanted_format=i.wanted_format
   join stateful b on b.id=wi.id where not `+authorManualOverrideSQL+`
  ) select i.id,i.author_id is not null,coalesce(m.derived_state,''),count(m.book_id)
   from identities i left join members m on m.id=i.id group by i.id,i.author_id,m.derived_state`, append(args, ids)...)
		if err != nil {
			return page, err
		}
		for rows.Next() {
			var id, state string
			var linked bool
			var count int
			if err = rows.Scan(&id, &linked, &state, &count); err != nil {
				rows.Close()
				return page, err
			}
			item := byID[id]
			item.IdentityLinked = linked
			item.TotalBooks += count
			if state != "" {
				item.Counts[state] = count
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return page, err
		}
	}
	if err = tx.Commit(); err != nil {
		return page, err
	}
	return page, nil
}
