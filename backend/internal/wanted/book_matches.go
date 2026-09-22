package wanted

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

var ErrBookMatches = errors.New("invalid book identity lookup")
var ErrBookAlreadyTracked = errors.New("this provider identity is already saved; refresh the library check and open the existing book")

// BookMatchCandidate contains identities only. Titles and ISBN similarity cannot
// establish that an owner-selected work is the same tracking target.
type BookMatchCandidate struct {
	Key        string   `json:"key"`
	Provider   string   `json:"provider"`
	WorkIDs    []string `json:"workIds"`
	EditionIDs []string `json:"editionIds"`
	SourceKey  string   `json:"sourceKey"`
	Format     string   `json:"format"`
}

type BookMatch struct {
	Key   string       `json:"key"`
	Total int          `json:"total"`
	Books []WantedItem `json:"books"`
}

type BookMatches struct {
	Matches    []BookMatch `json:"matches"`
	ObservedAt time.Time   `json:"observedAt"`
}

type bookMatchAlias struct {
	Key      string `json:"key"`
	Kind     string `json:"kind"`
	Provider string `json:"provider"`
	Source   string `json:"source"`
	Format   string `json:"format"`
}

func candidateBookIdentity(result metadata.SearchResult, format string) BookMatchCandidate {
	return BookMatchCandidate{Key: "add", Provider: result.Provider, Format: format,
		WorkIDs:    append([]string{result.Work.ID}, result.Work.ProviderIDs...),
		EditionIDs: append([]string{result.Edition.ID}, result.Edition.ProviderIDs...),
		SourceKey:  firstNonEmpty(result.Edition.ID, result.Work.ID, result.RawSourceKey)}
}

func bookMatchAliases(candidates []BookMatchCandidate) ([]bookMatchAlias, error) {
	if len(candidates) > 100 {
		return nil, fmt.Errorf("%w: at most 100 candidates", ErrBookMatches)
	}
	aliases := []bookMatchAlias{}
	seen := map[string]bool{}
	for _, c := range candidates {
		if c.Key == "" || len(c.Key) > 2048 || seen[c.Key] || len(c.Provider) > 64 ||
			(c.Format != "ebook" && c.Format != "audiobook") || len(c.WorkIDs)+len(c.EditionIDs) > 64 || len(c.SourceKey) > 512 {
			return nil, ErrBookMatches
		}
		seen[c.Key] = true
		for _, id := range append(append([]string{}, c.WorkIDs...), c.EditionIDs...) {
			if len(id) > 512 {
				return nil, ErrBookMatches
			}
		}
		add := func(kind, provider, source string) {
			provider, source = strings.TrimSpace(provider), strings.TrimSpace(source)
			if provider != "" && source != "" {
				aliases = append(aliases, bookMatchAlias{c.Key, kind, provider, source, c.Format})
			}
		}
		add("source", c.Provider, c.SourceKey)
		for _, a := range providerAliases(c.Provider, c.WorkIDs) {
			add("work", a.Provider, a.Key)
			add("source", a.Provider, a.Key)
			add("source", a.Provider, a.Key+":edition") // legacy work/format placeholders
		}
		for _, a := range providerAliases(c.Provider, c.EditionIDs) {
			add("edition", a.Provider, a.Key)
			add("source", a.Provider, a.Key)
		}
	}
	return aliases, nil
}

func (s *Service) MatchBooks(ctx context.Context, candidates []BookMatchCandidate) (BookMatches, error) {
	if _, err := bookMatchAliases(candidates); err != nil {
		return BookMatches{}, err
	}
	if !s.Available() {
		return BookMatches{}, sql.ErrConnDone
	}
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return BookMatches{}, err
	}
	defer tx.Rollback()
	result, err := matchBooks(ctx, tx, candidates)
	if err != nil {
		return BookMatches{}, err
	}
	if err = tx.Commit(); err != nil {
		return BookMatches{}, err
	}
	unique := map[string]WantedItem{}
	for _, m := range result.Matches {
		for _, book := range m.Books {
			unique[book.ID] = book
		}
	}
	books := make([]WantedItem, 0, len(unique))
	for _, book := range unique {
		books = append(books, book)
	}
	for _, book := range s.AnnotateWantedStates(ctx, books) {
		unique[book.ID] = book
	}
	for i := range result.Matches {
		for j, book := range result.Matches[i].Books {
			result.Matches[i].Books[j] = unique[book.ID]
		}
	}
	return result, nil
}

// Different saved aliases can already point to the same local work even when
// the two incoming candidates do not advertise each other's provider IDs.
func lockMatchedWorks(ctx context.Context, tx *sql.Tx, aliases []bookMatchAlias, format string) error {
	raw, err := json.Marshal(aliases)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `with aliases as (
	 select * from jsonb_to_recordset($1::jsonb) as a(kind text,provider text,source text)
	), works as (
	 select pr.entity_id as id from aliases a join provider_records pr
	 on pr.provider=a.provider and pr.provider_key=a.source and pr.entity_type='work' where a.kind='work'
	 union
	 select e.work_id from aliases a join provider_records pr
	 on pr.provider=a.provider and pr.provider_key=a.source and pr.entity_type='edition'
	 join editions e on e.id=pr.entity_id where a.kind='edition'
	) select id::text from works where id is not null order by id`, string(raw))
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err = tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, "wanted-existing-work:"+id+"|"+format); err != nil {
			return err
		}
	}
	return nil
}

func matchBooks(ctx context.Context, tx *sql.Tx, candidates []BookMatchCandidate) (BookMatches, error) {
	aliases, err := bookMatchAliases(candidates)
	if err != nil {
		return BookMatches{}, err
	}
	result := BookMatches{Matches: []BookMatch{}}
	positions := map[string]int{}
	for _, c := range candidates {
		positions[c.Key] = len(result.Matches)
		result.Matches = append(result.Matches, BookMatch{Key: c.Key, Books: []WantedItem{}})
	}
	if err = tx.QueryRowContext(ctx, `select now()`).Scan(&result.ObservedAt); err != nil {
		return result, err
	}
	raw, err := json.Marshal(aliases)
	if err != nil {
		return result, err
	}
	// Every join uses persisted, typed provider identities. Inactive records are
	// deliberately included so lookup never turns a removal into an implicit add.
	rows, err := tx.QueryContext(ctx, `with aliases as (
		select * from jsonb_to_recordset($1::jsonb) as a(key text, kind text, provider text, source text, format text)
	), ids as (
		select a.key, wi.id from aliases a join wanted_items wi
		 on wi.metadata_provider=a.provider and wi.source_key=a.source and wi.wanted_format=a.format where a.kind='source'
		union
		select a.key, wi.id from aliases a join provider_records pr
		 on pr.provider=a.provider and pr.provider_key=a.source and pr.entity_type='work'
		 join wanted_items wi on wi.work_id=pr.entity_id and wi.wanted_format=a.format where a.kind='work'
		union
		select a.key, wi.id from aliases a join provider_records pr
		 on pr.provider=a.provider and pr.provider_key=a.source and pr.entity_type='edition'
		 join wanted_items wi on wi.edition_id=pr.entity_id and wi.wanted_format=a.format where a.kind='edition'
	), ranked as (
		select ids.key, wi.id, count(*) over(partition by ids.key) as total,
		 row_number() over(partition by ids.key order by wi.status in ('removed','ignored'), wi.created_at, wi.id) as position
		 from ids join wanted_items wi on wi.id=ids.id
	) select key,id::text,total from ranked where position<=10 order by key,position`, string(raw))
	if err != nil {
		return result, err
	}
	type hit struct {
		key, id string
		total   int
	}
	hits := []hit{}
	ids := []string{}
	seenIDs := map[string]bool{}
	for rows.Next() {
		var h hit
		if err = rows.Scan(&h.key, &h.id, &h.total); err != nil {
			rows.Close()
			return result, err
		}
		hits = append(hits, h)
		if !seenIDs[h.id] {
			ids = append(ids, h.id)
			seenIDs[h.id] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(ids) == 0 {
		return result, nil
	} // nil IDs mean all active books to reviewBooks.
	books, _, err := reviewBooks(ctx, tx, ids, false)
	if err != nil {
		return result, err
	}
	byID := map[string]WantedItem{}
	for _, book := range books {
		byID[book.ID] = book
	}
	for _, h := range hits {
		match := &result.Matches[positions[h.key]]
		match.Total = h.total
		match.Books = append(match.Books, byID[h.id])
	}
	return result, nil
}
