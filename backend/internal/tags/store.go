// Package tags implements the native tag system (M6.4). The tags table owns
// the label vocabulary; wanted_items.tags and author_subscriptions.tags keep
// the 0016 comma-separated text format carrying labels. Creating, renaming,
// or deleting a tag rewrites the label across both text columns in one
// transaction, so tag identity stays consistent everywhere.
package tags

import (
	"context"
	"database/sql"
	"errors"
	"hash/fnv"
	"strconv"
	"strings"
)

// ErrTagNotFound marks lookups for unknown tag ids (HTTP handlers map it to 404).
var ErrTagNotFound = errors.New("tag not found")

// ErrTagExists marks duplicate labels (HTTP handlers map it to 409).
var ErrTagExists = errors.New("tag already exists")

// Tag is the API-facing shape. ID is a stable FNV-1a hash of the row's uuid
// (Readarr-style integer ids for the UI and compat layer); UUID is internal.
type Tag struct {
	ID          int    `json:"id"`
	UUID        string `json:"-"`
	Label       string `json:"label"`
	WantedCount int    `json:"wantedCount"`
	AuthorCount int    `json:"authorCount"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	if db == nil {
		return nil
	}
	return &Store{db: db}
}

func (s *Store) Configured() bool {
	return s != nil && s.db != nil
}

// NormalizeLabel trims and lowercases labels (matching the wanted package).
func NormalizeLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

// StableID hashes a label or uuid onto the compat-style positive int32 space.
func StableID(value string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.TrimSpace(value)))
	return int(hash.Sum32() & 0x7fffffff)
}

// List returns every tag with usage counts aggregated by label across wanted
// items and author subscriptions.
func (s *Store) List(ctx context.Context) ([]Tag, error) {
	if !s.Configured() {
		return nil, errors.New("tags store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `select id::text, label from tags order by label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.UUID, &tag.Label); err != nil {
			return nil, err
		}
		tag.ID = StableID(tag.UUID)
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	wantedCounts, err := s.labelCounts(ctx, "wanted_items")
	if err != nil {
		return nil, err
	}
	authorCounts, err := s.labelCounts(ctx, "author_subscriptions")
	if err != nil {
		return nil, err
	}
	for index := range tags {
		tags[index].WantedCount = wantedCounts[tags[index].Label]
		tags[index].AuthorCount = authorCounts[tags[index].Label]
	}
	return tags, nil
}

// labelCounts counts rows per label token in a comma-list text column.
func (s *Store) labelCounts(ctx context.Context, table string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `select tags from `+table+` where tags <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		for _, label := range SplitLabels(value) {
			counts[label]++
		}
	}
	return counts, rows.Err()
}

// Create inserts a new label.
func (s *Store) Create(ctx context.Context, label string) (Tag, error) {
	if !s.Configured() {
		return Tag{}, errors.New("tags store is unavailable")
	}
	label = NormalizeLabel(label)
	if label == "" {
		return Tag{}, errors.New("tag label is required")
	}
	var tag Tag
	err := s.db.QueryRowContext(ctx, `
		insert into tags (label) values ($1)
		on conflict (label) do nothing
		returning id::text, label
	`, label).Scan(&tag.UUID, &tag.Label)
	if errors.Is(err, sql.ErrNoRows) {
		return Tag{}, ErrTagExists
	}
	if err != nil {
		return Tag{}, err
	}
	tag.ID = StableID(tag.UUID)
	return tag, nil
}

// Rename updates a tag label and rewrites the old label onto the new one in
// both comma-list text columns, all in one transaction.
func (s *Store) Rename(ctx context.Context, idOrLabel string, newLabel string) (Tag, error) {
	if !s.Configured() {
		return Tag{}, errors.New("tags store is unavailable")
	}
	newLabel = NormalizeLabel(newLabel)
	if newLabel == "" {
		return Tag{}, errors.New("tag label is required")
	}
	existing, err := s.resolve(ctx, idOrLabel)
	if err != nil {
		return Tag{}, err
	}
	if existing.Label == newLabel {
		return existing, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tag{}, err
	}
	defer tx.Rollback()

	var renamed Tag
	err = tx.QueryRowContext(ctx, `
		update tags set label = $2 where id::text = $1
		returning id::text, label
	`, existing.UUID, newLabel).Scan(&renamed.UUID, &renamed.Label)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return Tag{}, ErrTagExists
		}
		return Tag{}, err
	}
	for _, table := range []string{"wanted_items", "author_subscriptions"} {
		if err := rewriteLabelColumn(ctx, tx, table, existing.Label, newLabel); err != nil {
			return Tag{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Tag{}, err
	}
	renamed.ID = StableID(renamed.UUID)
	return renamed, nil
}

// Delete removes a tag and strips its label from both text columns in one
// transaction.
func (s *Store) Delete(ctx context.Context, idOrLabel string) error {
	if !s.Configured() {
		return errors.New("tags store is unavailable")
	}
	existing, err := s.resolve(ctx, idOrLabel)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `delete from tags where id::text = $1`, existing.UUID); err != nil {
		return err
	}
	for _, table := range []string{"wanted_items", "author_subscriptions"} {
		if err := rewriteLabelColumn(ctx, tx, table, existing.Label, ""); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// resolve maps a path value (compat-style int id, uuid, or label) onto a row.
func (s *Store) resolve(ctx context.Context, idOrLabel string) (Tag, error) {
	idOrLabel = strings.TrimSpace(idOrLabel)
	if idOrLabel == "" {
		return Tag{}, ErrTagNotFound
	}
	tagRows, err := s.List(ctx)
	if err != nil {
		return Tag{}, err
	}
	numeric, numericErr := strconv.Atoi(idOrLabel)
	for _, tag := range tagRows {
		switch {
		case tag.UUID == idOrLabel,
			tag.Label == NormalizeLabel(idOrLabel),
			numericErr == nil && tag.ID == numeric,
			numericErr == nil && StableID(tag.Label) == numeric:
			return tag, nil
		}
	}
	return Tag{}, ErrTagNotFound
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// rewriteLabelColumn replaces (or removes, when newLabel is empty) a label
// token inside a comma-list text column.
func rewriteLabelColumn(ctx context.Context, tx execer, table string, oldLabel string, newLabel string) error {
	rows, err := tx.QueryContext(ctx, `select id::text, tags from `+table+` where tags <> ''`)
	if err != nil {
		return err
	}
	type update struct {
		id   string
		tags string
	}
	var updates []update
	for rows.Next() {
		var id, value string
		if err := rows.Scan(&id, &value); err != nil {
			rows.Close()
			return err
		}
		rewritten, changed := RewriteLabels(value, oldLabel, newLabel)
		if changed {
			updates = append(updates, update{id: id, tags: rewritten})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range updates {
		if _, err := tx.ExecContext(ctx, `update `+table+` set tags = $2, updated_at = now() where id::text = $1`, item.id, item.tags); err != nil {
			return err
		}
	}
	return nil
}

// SplitLabels parses a comma-list tags column into normalized labels.
func SplitLabels(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == '\t' || r == '\n' || r == '\r'
	})
	seen := map[string]bool{}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		label := NormalizeLabel(part)
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	return out
}

// RewriteLabels replaces oldLabel with newLabel (removing it when newLabel is
// empty) inside a comma-list value; reports whether anything changed.
func RewriteLabels(value string, oldLabel string, newLabel string) (string, bool) {
	oldLabel = NormalizeLabel(oldLabel)
	newLabel = NormalizeLabel(newLabel)
	labels := SplitLabels(value)
	out := make([]string, 0, len(labels))
	seen := map[string]bool{}
	changed := false
	for _, label := range labels {
		if label == oldLabel {
			changed = true
			label = newLabel
		}
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	if !changed {
		return value, false
	}
	return strings.Join(out, ","), true
}
