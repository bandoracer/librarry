package wanted

import (
	"context"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/metadata"
)

type Acquisition interface {
	Search(ctx context.Context, query acquisition.ReleaseSearchQuery) ([]acquisition.Release, error)
	Grab(ctx context.Context, request acquisition.DownloadRequest) (acquisition.DownloadStatus, error)
	CategoryForFormat(format string) string
	TorrentRoot() string
}

type CreateRequest struct {
	Result         metadata.SearchResult `json:"result"`
	Format         string                `json:"format,omitempty"`
	QualityProfile string                `json:"qualityProfile,omitempty"`
}

type SearchReleasesRequest struct {
	Limit int `json:"limit,omitempty"`
}

type GrabRequest struct {
	ReleaseID string `json:"releaseId,omitempty"`
	Paused    bool   `json:"paused"`
}

type WantedItem struct {
	ID             string     `json:"id"`
	WorkID         string     `json:"workId,omitempty"`
	EditionID      string     `json:"editionId,omitempty"`
	Title          string     `json:"title"`
	AuthorName     string     `json:"authorName,omitempty"`
	CoverURL       string     `json:"coverUrl,omitempty"`
	Format         string     `json:"format"`
	QualityProfile string     `json:"qualityProfile"`
	Status         string     `json:"status"`
	SourceProvider string     `json:"sourceProvider,omitempty"`
	SourceKey      string     `json:"sourceKey,omitempty"`
	LastSearchAt   *time.Time `json:"lastSearchAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type ReleaseDecision struct {
	ID             string    `json:"id"`
	WantedItemID   string    `json:"wantedItemId"`
	SourceID       string    `json:"sourceId"`
	InfoHash       string    `json:"infoHash,omitempty"`
	Indexer        string    `json:"indexer"`
	Title          string    `json:"title"`
	Protocol       string    `json:"protocol"`
	DownloadURL    string    `json:"downloadUrl"`
	InfoURL        string    `json:"infoUrl,omitempty"`
	SizeBytes      int64     `json:"sizeBytes,omitempty"`
	Seeders        int       `json:"seeders,omitempty"`
	Leechers       int       `json:"leechers,omitempty"`
	Categories     []string  `json:"categories,omitempty"`
	Score          float64   `json:"score"`
	Approved       bool      `json:"approved"`
	RejectedReason string    `json:"rejectedReason,omitempty"`
	PublishedAt    time.Time `json:"publishedAt,omitempty"`
	SearchedAt     time.Time `json:"searchedAt"`
	CreatedAt      time.Time `json:"createdAt"`
}

type SearchOutcome struct {
	WantedItem WantedItem        `json:"wantedItem"`
	Releases   []ReleaseDecision `json:"releases"`
}
