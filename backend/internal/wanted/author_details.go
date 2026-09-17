package wanted

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrAuthorPage = errors.New("invalid author page")

type AuthorIdentity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider,omitempty"`
	ProviderKey string `json:"providerKey,omitempty"`
	NameOnly    bool   `json:"nameOnly,omitempty"`
}

type AuthorDetail struct {
	Author           AuthorIdentity       `json:"author"`
	Subscriptions    []AuthorSubscription `json:"subscriptions"`
	Books            []WantedItem         `json:"books"`
	TotalBooks       int                  `json:"totalBooks"`
	NextCursor       string               `json:"nextCursor,omitempty"`
	Choices          []AuthorIdentity     `json:"choices"`
	ChoicesTruncated bool                 `json:"choicesTruncated,omitempty"`
}

type authorPageCursor struct {
	Author string `json:"author"`
	Title  string `json:"title"`
	ID     string `json:"id"`
}

const authorManualOverrideSQL = `exists(select 1 from manual_overrides mo where mo.entity_type='wanted_item' and mo.entity_id=wi.id and mo.field_name='author_name')`
const authorLinkedSQL = `exists(select 1 from work_authors wa where wa.work_id=wi.work_id and lower(wa.role) in ('author','writer'))`
const authorVisibleBookSQL = `wi.status not in ('removed','ignored')`
const wantedDetailColumns = `wi.id, wi.work_id, wi.edition_id, coalesce(nullif(wi.title, ''), w.title),
 coalesce(nullif(wi.author_name, ''), ''), coalesce(nullif(wi.cover_url, ''), w.cover_url),
 wi.wanted_format, wi.quality_profile, wi.status, wi.monitored, wi.metadata_provider,
 wi.source_key, coalesce(wi.current_release_id::text, ''), wi.current_release_score,
 coalesce(wi.root_folder_id::text, ''), wi.series, wi.series_position, wi.first_publish_year,
 wi.tags, wi.release_date, wi.last_search_at, wi.last_upgrade_search_at, wi.created_at, wi.updated_at`

// Match the historical web name key only for resolving old URLs. Never use it
// as evidence that two identified people are the same author.
func authorNameSQL(column string) string {
	return `coalesce(nullif(regexp_replace(btrim(regexp_replace(lower(coalesce(nullif(btrim(` + column + `),''),'Unknown author')),'[^a-z0-9]+',' ','g')),'^(a|an|the)\s+','','g'),''),'unknown-author')`
}

var authorKeySeparators = regexp.MustCompile(`[^a-z0-9]+`)
var authorKeyArticle = regexp.MustCompile(`^(a|an|the)\s+`)

func legacyAuthorKey(name string) string {
	if strings.TrimSpace(name) == "" {
		name = "Unknown author"
	}
	key := authorKeyArticle.ReplaceAllString(strings.TrimSpace(authorKeySeparators.ReplaceAllString(strings.ToLower(name), " ")), "")
	if key == "" {
		return "unknown-author"
	}
	return key
}

func (s *Service) AuthorDetail(ctx context.Context, key, cursor string, limit int) (AuthorDetail, error) {
	if !s.Available() {
		return AuthorDetail{}, errors.New("wanted service requires database persistence")
	}
	detail, err := s.store.AuthorDetail(ctx, key, cursor, limit)
	if err == nil {
		detail.Books = s.AnnotateWantedStates(ctx, detail.Books)
	}
	return detail, err
}

func (s *Store) AuthorDetail(ctx context.Context, key, cursor string, limit int) (AuthorDetail, error) {
	detail := AuthorDetail{Subscriptions: []AuthorSubscription{}, Books: []WantedItem{}, Choices: []AuthorIdentity{}}
	if !s.Configured() {
		return detail, errors.New("wanted store is unavailable")
	}
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 100 || len(key) > 512 || len(cursor) > 4096 {
		return detail, ErrAuthorPage
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return detail, ErrAuthorPage
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return detail, err
	}
	defer tx.Rollback()
	var canonicalID, provider, providerKey string
	var parsed pgtype.UUID
	isUUID := parsed.Scan(key) == nil && parsed.Valid
	if isUUID {
		// Subscription links work before any book has been acquired or tracked.
		err = tx.QueryRowContext(ctx, `select author_name,provider,provider_key from author_subscriptions where id::text=$1 and status <> 'removed'`, key).Scan(&detail.Author.Name, &provider, &providerKey)
		if err == nil {
			detail.Author = AuthorIdentity{ID: key, Name: detail.Author.Name, Provider: provider, ProviderKey: providerKey}
			err = tx.QueryRowContext(ctx, `select entity_id::text from provider_records where entity_type='author' and lower(provider)=lower($1) and lower(provider_key)=lower($2) and entity_id is not null`, provider, providerKey).Scan(&canonicalID)
			if errors.Is(err, sql.ErrNoRows) {
				err = nil
			}
		} else if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `select id::text,canonical_name from authors where id::text=$1`, key).Scan(&canonicalID, &detail.Author.Name)
			detail.Author.ID = canonicalID
		}
		if err != nil {
			return detail, err
		}
		if detail.Author.ProviderKey == "" && canonicalID != "" {
			err := tx.QueryRowContext(ctx, `select provider,provider_key from provider_records where entity_type='author' and entity_id=$1::uuid order by provider,provider_key limit 1`, canonicalID).Scan(&detail.Author.Provider, &detail.Author.ProviderKey)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return detail, err
			}
		}
	} else if strings.HasPrefix(key, "name:") {
		detail.Author = AuthorIdentity{ID: key, Name: strings.TrimPrefix(key, "name:"), NameOnly: true}
	} else {
		choices, truncated, err := authorNameChoices(ctx, tx, key)
		if err != nil {
			return detail, err
		}
		if len(choices) == 0 {
			return detail, sql.ErrNoRows
		}
		if len(choices) != 1 || truncated {
			detail.Author = AuthorIdentity{ID: key, Name: key}
			detail.Choices, detail.ChoicesTruncated = choices, truncated
			return detail, tx.Commit()
		}
		// Resolve the unique stable link in a new snapshot; the final identity
		// lookup, count and page still share their own coherent read snapshot.
		if err := tx.Rollback(); err != nil {
			return detail, err
		}
		return s.AuthorDetail(ctx, choices[0].ID, cursor, limit)
	}

	if canonicalID != "" {
		rows, err := tx.QueryContext(ctx, `select s.id::text from author_subscriptions s where s.status<>'removed' and exists(select 1 from provider_records p where p.entity_type='author' and p.entity_id::text=$1 and lower(p.provider)=lower(s.provider) and lower(p.provider_key)=lower(s.provider_key)) order by s.wanted_format,s.id`, canonicalID)
		if err != nil {
			return detail, err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return detail, err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return detail, err
		}
		for _, id := range ids {
			sub, err := authorSubscriptionInSnapshot(ctx, tx, id)
			if err != nil {
				return detail, err
			}
			detail.Subscriptions = append(detail.Subscriptions, sub)
		}
	} else if providerKey != "" {
		rows, err := tx.QueryContext(ctx, `select id::text from author_subscriptions where lower(provider)=lower($1) and lower(provider_key)=lower($2) and status<>'removed' order by wanted_format,id`, provider, providerKey)
		if err != nil {
			return detail, err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return detail, err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return detail, err
		}
		for _, id := range ids {
			sub, err := authorSubscriptionInSnapshot(ctx, tx, id)
			if err != nil {
				return detail, err
			}
			detail.Subscriptions = append(detail.Subscriptions, sub)
		}
	}
	where := authorVisibleBookSQL + ` and false`
	identity := canonicalID
	if canonicalID != "" {
		where = authorVisibleBookSQL + ` and not ` + authorManualOverrideSQL + ` and exists(select 1 from work_authors wa where wa.work_id=wi.work_id and wa.author_id=$1::uuid and lower(wa.role) in ('author','writer'))`
	}
	if detail.Author.NameOnly {
		identity = strings.TrimPrefix(key, "name:")
		where = authorVisibleBookSQL + ` and (` + authorManualOverrideSQL + ` or not ` + authorLinkedSQL + `) and ` + authorNameSQL("wi.author_name") + `=$1`
		if err := tx.QueryRowContext(ctx, `select coalesce(nullif(btrim(wi.author_name),''),'Unknown author') from wanted_items wi where `+where+` order by wi.id limit 1`, identity).Scan(&detail.Author.Name); err != nil {
			return detail, err
		}
	}
	// Keep a bound identity placeholder even for an unlinked subscription.
	if canonicalID == "" && !detail.Author.NameOnly {
		where += ` and $1::text=$1::text`
	}
	if err := tx.QueryRowContext(ctx, `select count(*) from wanted_items wi where `+where, identity).Scan(&detail.TotalBooks); err != nil {
		return detail, err
	}
	pageCursor := authorPageCursor{Author: key}
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(raw, &pageCursor) != nil || pageCursor.Author != key || pageCursor.ID == "" {
			return detail, ErrAuthorPage
		}
		var id pgtype.UUID
		if id.Scan(pageCursor.ID) != nil || !id.Valid {
			return detail, ErrAuthorPage
		}
	}
	rows, err := tx.QueryContext(ctx, `select `+wantedDetailColumns+` from wanted_items wi left join works w on w.id=wi.work_id where `+where+`
		and ($2='' or (lower(coalesce(nullif(wi.title,''),w.title,'')),wi.id::text)>(lower($3),$2))
		order by lower(coalesce(nullif(wi.title,''),w.title,'')),wi.id limit $4`, identity, pageCursor.ID, pageCursor.Title, limit+1)
	if err != nil {
		return detail, err
	}
	for rows.Next() {
		item, err := scanWanted(rows)
		if err != nil {
			rows.Close()
			return detail, err
		}
		detail.Books = append(detail.Books, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return detail, err
	}
	if len(detail.Books) > limit {
		detail.Books = detail.Books[:limit]
		last := detail.Books[len(detail.Books)-1]
		raw, _ := json.Marshal(authorPageCursor{Author: key, Title: last.Title, ID: last.ID})
		detail.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	detail.Books, err = attachWantedDetails(ctx, tx, detail.Books)
	if err != nil {
		return detail, err
	}
	return detail, tx.Commit()
}

func authorSubscriptionInSnapshot(ctx context.Context, tx *sql.Tx, id string) (AuthorSubscription, error) {
	return scanAuthorSubscription(tx.QueryRowContext(ctx, `select id,provider,provider_key,author_name,wanted_format,quality_profile,status,monitor_new_items,missing_book_policy,tags,allowed_languages,must_not_contain,skip_missing_isbn,min_pages,coalesce(metadata_profile_id::text,''),coalesce(root_folder_id::text,''),last_sync_at,created_at,updated_at from author_subscriptions where id::text=$1`, id))
}

func authorNameChoices(ctx context.Context, tx *sql.Tx, key string) ([]AuthorIdentity, bool, error) {
	rows, err := tx.QueryContext(ctx, `
		select coalesce(p.entity_id::text,(select min(s2.id::text) from author_subscriptions s2 where lower(s2.provider)=lower(s.provider) and lower(s2.provider_key)=lower(s.provider_key) and s2.status<>'removed')),s.author_name,s.provider,false,s.provider_key from author_subscriptions s
		left join provider_records p on p.entity_type='author' and lower(p.provider)=lower(s.provider) and lower(p.provider_key)=lower(s.provider_key)
		where s.status<>'removed' and `+authorNameSQL("s.author_name")+`=$1
		union
		select a.id::text,a.canonical_name,coalesce(p.provider,'Library'),false,coalesce(p.provider_key,'') from authors a left join lateral(select provider,provider_key from provider_records where entity_type='author' and entity_id=a.id order by provider,provider_key limit 1) p on true where `+authorNameSQL("a.canonical_name")+`=$1
		and exists(select 1 from work_authors wa join wanted_items wi on wi.work_id=wa.work_id where wa.author_id=a.id and lower(wa.role) in ('author','writer') and `+authorVisibleBookSQL+` and not `+authorManualOverrideSQL+`)
		union
		select 'name:'||$1,min(coalesce(nullif(btrim(wi.author_name),''),'Unknown author')),'Unidentified',true,'' from wanted_items wi where `+authorVisibleBookSQL+` and (`+authorManualOverrideSQL+` or not `+authorLinkedSQL+`) and `+authorNameSQL("wi.author_name")+`=$1 having count(*)>0
		order by 1,3 limit 201`, key)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	choices := []AuthorIdentity{}
	seen := map[string]bool{}
	count := 0
	for rows.Next() {
		var choice AuthorIdentity
		if err := rows.Scan(&choice.ID, &choice.Name, &choice.Provider, &choice.NameOnly, &choice.ProviderKey); err != nil {
			return nil, false, err
		}
		count++
		if !seen[choice.ID] {
			choices = append(choices, choice)
			seen[choice.ID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := count > 200
	if len(choices) > 100 {
		choices = choices[:100]
		truncated = true
	}
	sort.Slice(choices, func(i, j int) bool {
		if choices[i].Name == choices[j].Name {
			return choices[i].ID < choices[j].ID
		}
		return choices[i].Name < choices[j].Name
	})
	return choices, truncated, nil
}
