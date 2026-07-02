// Package importlists implements Hardcover-native import lists (M6.3): a
// store for list definitions and exclusions, a Hardcover list/shelf client,
// and a sync service that maps list entries onto wanted items while honoring
// exclusions, monitor mode, quality profile, root folder, and search-on-add.
package importlists

import (
	"context"
	"errors"
	"time"
)

// ErrListNotFound marks lookups for unknown list ids (HTTP handlers map it to 404).
var ErrListNotFound = errors.New("import list not found")

// ErrExclusionNotFound marks lookups for unknown exclusion ids.
var ErrExclusionNotFound = errors.New("import list exclusion not found")

// List is an import list definition. Settings is schema-driven per type;
// Hardcover lists use settings.listId (a Hardcover list/shelf id) and may set
// settings.format (ebook|audiobook, default ebook).
type List struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Enabled        bool              `json:"enabled"`
	Settings       map[string]string `json:"settings"`
	Monitor        string            `json:"monitor"`
	QualityProfile string            `json:"qualityProfile"`
	RootFolderID   string            `json:"rootFolderId,omitempty"`
	SearchOnAdd    bool              `json:"searchOnAdd"`
	LastSyncedAt   *time.Time        `json:"lastSyncedAt,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

// Exclusion suppresses a book from every list sync (native and compat).
type Exclusion struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	AuthorName string    `json:"authorName"`
	SourceKey  string    `json:"sourceKey"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Entry is one book resolved from a list provider.
type Entry struct {
	SourceKey   string
	Title       string
	AuthorName  string
	CoverURL    string
	ReleaseDate string
}

// ListFetcher resolves a list definition's settings into entries.
type ListFetcher interface {
	FetchList(ctx context.Context, settings map[string]string, limit int) ([]Entry, error)
}

// SyncItem is the per-entry outcome of a sync run.
type SyncItem struct {
	ListID     string `json:"listId"`
	ListName   string `json:"listName"`
	Title      string `json:"title"`
	AuthorName string `json:"authorName,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	WantedID   string `json:"wantedId,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SyncOutcome summarizes one sync run.
type SyncOutcome struct {
	Status          string     `json:"status"`
	Trigger         string     `json:"trigger,omitempty"`
	ListsChecked    int        `json:"listsChecked"`
	EntriesFound    int        `json:"entriesFound"`
	WantedCreated   int        `json:"wantedCreated"`
	SkippedExisting int        `json:"skippedExisting"`
	SkippedExcluded int        `json:"skippedExcluded"`
	SearchesStarted int        `json:"searchesStarted"`
	ErrorCount      int        `json:"errorCount"`
	Message         string     `json:"message,omitempty"`
	Items           []SyncItem `json:"items,omitempty"`
	StartedAt       time.Time  `json:"startedAt"`
	FinishedAt      time.Time  `json:"finishedAt"`
}
