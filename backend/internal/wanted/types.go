package wanted

import (
	"context"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/metadata"
)

type Acquisition interface {
	Search(ctx context.Context, query acquisition.ReleaseSearchQuery) ([]acquisition.Release, error)
	Feed(ctx context.Context, query acquisition.ReleaseFeedQuery) ([]acquisition.Release, error)
	Grab(ctx context.Context, request acquisition.DownloadRequest) (acquisition.DownloadStatus, error)
	Downloads(ctx context.Context, query acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error)
	DownloadAction(ctx context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error)
	MarkDownloadFailed(ctx context.Context, id string, reason string) error
	MarkDownloadReplacement(ctx context.Context, id string, replacementID string) error
	CategoryForFormat(format string) string
	TorrentRoot() string
}

type CreateRequest struct {
	Result         metadata.SearchResult `json:"result"`
	Format         string                `json:"format,omitempty"`
	QualityProfile string                `json:"qualityProfile,omitempty"`
}

type QualityProfile struct {
	ID             string    `json:"id,omitempty"`
	Name           string    `json:"name"`
	MediaFormat    string    `json:"mediaFormat"`
	MinScore       float64   `json:"minScore"`
	CutoffScore    float64   `json:"cutoffScore"`
	MinSeeders     int       `json:"minSeeders"`
	MaxSizeBytes   int64     `json:"maxSizeBytes"`
	PreferredTerms []string  `json:"preferredTerms,omitempty"`
	RequiredTerms  []string  `json:"requiredTerms,omitempty"`
	RejectedTerms  []string  `json:"rejectedTerms,omitempty"`
	PreferredScore float64   `json:"preferredScore"`
	UpgradeAllowed bool      `json:"upgradeAllowed"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type AuthorSubscribeRequest struct {
	Result          metadata.SearchResult `json:"result,omitempty"`
	AuthorName      string                `json:"authorName,omitempty"`
	Provider        string                `json:"provider,omitempty"`
	ProviderKey     string                `json:"providerKey,omitempty"`
	Format          string                `json:"format,omitempty"`
	QualityProfile  string                `json:"qualityProfile,omitempty"`
	MonitorNewItems *bool                 `json:"monitorNewItems,omitempty"`
}

type AuthorSubscription struct {
	ID              string     `json:"id,omitempty"`
	Provider        string     `json:"provider"`
	ProviderKey     string     `json:"providerKey"`
	AuthorName      string     `json:"authorName"`
	Format          string     `json:"format"`
	QualityProfile  string     `json:"qualityProfile"`
	Status          string     `json:"status"`
	MonitorNewItems bool       `json:"monitorNewItems"`
	LastSyncAt      *time.Time `json:"lastSyncAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type AuthorUpdateRequest struct {
	AuthorName      string `json:"authorName,omitempty"`
	QualityProfile  string `json:"qualityProfile,omitempty"`
	Status          string `json:"status,omitempty"`
	MonitorNewItems *bool  `json:"monitorNewItems,omitempty"`
	Monitored       *bool  `json:"monitored,omitempty"`
}

type SearchReleasesRequest struct {
	Limit int `json:"limit,omitempty"`
}

type MonitorRequest struct {
	Trigger                  string `json:"trigger,omitempty"`
	Limit                    int    `json:"limit,omitempty"`
	SearchLimit              int    `json:"searchLimit,omitempty"`
	Force                    bool   `json:"force,omitempty"`
	AutoGrab                 bool   `json:"autoGrab,omitempty"`
	Paused                   bool   `json:"paused,omitempty"`
	MinSearchIntervalMinutes int    `json:"minSearchIntervalMinutes,omitempty"`
}

type FeedSyncRequest struct {
	Trigger  string `json:"trigger,omitempty"`
	Format   string `json:"format,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	AutoGrab bool   `json:"autoGrab,omitempty"`
	Paused   bool   `json:"paused,omitempty"`
}

type FailedDownloadRequest struct {
	Trigger           string   `json:"trigger,omitempty"`
	DownloadIDs       []string `json:"downloadIds,omitempty"`
	Limit             int      `json:"limit,omitempty"`
	SearchLimit       int      `json:"searchLimit,omitempty"`
	MinStalledMinutes int      `json:"minStalledMinutes,omitempty"`
	AutoGrab          bool     `json:"autoGrab,omitempty"`
	Paused            bool     `json:"paused,omitempty"`
	RemoveFailed      bool     `json:"removeFailed,omitempty"`
	DeleteFailedFiles bool     `json:"deleteFailedFiles,omitempty"`
	Force             bool     `json:"force,omitempty"`
}

type UpgradeRequest struct {
	Trigger                  string   `json:"trigger,omitempty"`
	WantedIDs                []string `json:"wantedIds,omitempty"`
	Limit                    int      `json:"limit,omitempty"`
	SearchLimit              int      `json:"searchLimit,omitempty"`
	MinSearchIntervalMinutes int      `json:"minSearchIntervalMinutes,omitempty"`
	MinScoreDelta            float64  `json:"minScoreDelta,omitempty"`
	AutoGrab                 bool     `json:"autoGrab,omitempty"`
	Paused                   bool     `json:"paused,omitempty"`
	Force                    bool     `json:"force,omitempty"`
}

type AuthorMonitorRequest struct {
	Trigger                string `json:"trigger,omitempty"`
	Limit                  int    `json:"limit,omitempty"`
	SearchLimit            int    `json:"searchLimit,omitempty"`
	Force                  bool   `json:"force,omitempty"`
	MinSyncIntervalMinutes int    `json:"minSyncIntervalMinutes,omitempty"`
}

type GrabRequest struct {
	ReleaseID string `json:"releaseId,omitempty"`
	Client    string `json:"client,omitempty"`
	Paused    bool   `json:"paused"`
}

type WantedItem struct {
	ID                  string     `json:"id"`
	WorkID              string     `json:"workId,omitempty"`
	EditionID           string     `json:"editionId,omitempty"`
	Title               string     `json:"title"`
	AuthorName          string     `json:"authorName,omitempty"`
	CoverURL            string     `json:"coverUrl,omitempty"`
	Format              string     `json:"format"`
	QualityProfile      string     `json:"qualityProfile"`
	Status              string     `json:"status"`
	Monitored           bool       `json:"monitored"`
	SourceProvider      string     `json:"sourceProvider,omitempty"`
	SourceKey           string     `json:"sourceKey,omitempty"`
	CurrentReleaseID    string     `json:"currentReleaseId,omitempty"`
	CurrentReleaseScore float64    `json:"currentReleaseScore,omitempty"`
	LastSearchAt        *time.Time `json:"lastSearchAt,omitempty"`
	LastUpgradeSearchAt *time.Time `json:"lastUpgradeSearchAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type WantedUpdateRequest struct {
	Title          string `json:"title,omitempty"`
	AuthorName     string `json:"authorName,omitempty"`
	CoverURL       string `json:"coverUrl,omitempty"`
	QualityProfile string `json:"qualityProfile,omitempty"`
	Status         string `json:"status,omitempty"`
	Monitored      *bool  `json:"monitored,omitempty"`
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

type MonitorRun struct {
	ID            string              `json:"id"`
	Trigger       string              `json:"trigger"`
	Status        string              `json:"status"`
	WantedChecked int                 `json:"wantedChecked"`
	ReleasesFound int                 `json:"releasesFound"`
	ApprovedCount int                 `json:"approvedCount"`
	RejectedCount int                 `json:"rejectedCount"`
	GrabbedCount  int                 `json:"grabbedCount"`
	ErrorCount    int                 `json:"errorCount"`
	Message       string              `json:"message,omitempty"`
	Items         []MonitorItemResult `json:"items,omitempty"`
	StartedAt     time.Time           `json:"startedAt"`
	FinishedAt    *time.Time          `json:"finishedAt,omitempty"`
}

type FeedSyncRun struct {
	ID            string          `json:"id"`
	Trigger       string          `json:"trigger"`
	Status        string          `json:"status"`
	ReleasesSeen  int             `json:"releasesSeen"`
	MatchedCount  int             `json:"matchedCount"`
	ApprovedCount int             `json:"approvedCount"`
	RejectedCount int             `json:"rejectedCount"`
	GrabbedCount  int             `json:"grabbedCount"`
	ErrorCount    int             `json:"errorCount"`
	Message       string          `json:"message,omitempty"`
	Matches       []FeedSyncMatch `json:"matches,omitempty"`
	StartedAt     time.Time       `json:"startedAt"`
	FinishedAt    *time.Time      `json:"finishedAt,omitempty"`
}

type FeedSyncMatch struct {
	WantedItem      WantedItem                  `json:"wantedItem"`
	Release         ReleaseDecision             `json:"release"`
	GrabbedDownload *acquisition.DownloadStatus `json:"grabbedDownload,omitempty"`
	Error           string                      `json:"error,omitempty"`
}

type FailedDownloadRun struct {
	ID                string                 `json:"id"`
	Trigger           string                 `json:"trigger"`
	Status            string                 `json:"status"`
	DownloadsChecked  int                    `json:"downloadsChecked"`
	FailedCount       int                    `json:"failedCount"`
	ReplacementsFound int                    `json:"replacementsFound"`
	GrabbedCount      int                    `json:"grabbedCount"`
	RemovedCount      int                    `json:"removedCount"`
	ErrorCount        int                    `json:"errorCount"`
	Message           string                 `json:"message,omitempty"`
	Items             []FailedDownloadResult `json:"items,omitempty"`
	StartedAt         time.Time              `json:"startedAt"`
	FinishedAt        *time.Time             `json:"finishedAt,omitempty"`
}

type FailedDownloadResult struct {
	Download            acquisition.DownloadStatus  `json:"download"`
	WantedItem          WantedItem                  `json:"wantedItem,omitempty"`
	FailureReason       string                      `json:"failureReason"`
	Removed             bool                        `json:"removed,omitempty"`
	ReplacementReleases []ReleaseDecision           `json:"replacementReleases,omitempty"`
	ReplacementRelease  *ReleaseDecision            `json:"replacementRelease,omitempty"`
	ReplacementDownload *acquisition.DownloadStatus `json:"replacementDownload,omitempty"`
	Error               string                      `json:"error,omitempty"`
}

type UpgradeRun struct {
	ID            string              `json:"id"`
	Trigger       string              `json:"trigger"`
	Status        string              `json:"status"`
	WantedChecked int                 `json:"wantedChecked"`
	ReleasesFound int                 `json:"releasesFound"`
	UpgradeCount  int                 `json:"upgradeCount"`
	GrabbedCount  int                 `json:"grabbedCount"`
	ErrorCount    int                 `json:"errorCount"`
	Message       string              `json:"message,omitempty"`
	Items         []UpgradeItemResult `json:"items,omitempty"`
	StartedAt     time.Time           `json:"startedAt"`
	FinishedAt    *time.Time          `json:"finishedAt,omitempty"`
}

type AuthorMonitorRun struct {
	ID             string                    `json:"id"`
	Trigger        string                    `json:"trigger"`
	Status         string                    `json:"status"`
	AuthorsChecked int                       `json:"authorsChecked"`
	ItemsFound     int                       `json:"itemsFound"`
	WantedCreated  int                       `json:"wantedCreated"`
	ErrorCount     int                       `json:"errorCount"`
	Message        string                    `json:"message,omitempty"`
	Items          []AuthorMonitorItemResult `json:"items,omitempty"`
	StartedAt      time.Time                 `json:"startedAt"`
	FinishedAt     *time.Time                `json:"finishedAt,omitempty"`
}

type AuthorMonitorItemResult struct {
	Subscription  AuthorSubscription `json:"subscription"`
	ResultsFound  int                `json:"resultsFound"`
	WantedCreated int                `json:"wantedCreated"`
	WantedItems   []WantedItem       `json:"wantedItems,omitempty"`
	Error         string             `json:"error,omitempty"`
}

type UpgradeItemResult struct {
	WantedItem      WantedItem                  `json:"wantedItem"`
	CurrentScore    float64                     `json:"currentScore"`
	CutoffScore     float64                     `json:"cutoffScore"`
	ReleasesFound   int                         `json:"releasesFound"`
	UpgradeRelease  *ReleaseDecision            `json:"upgradeRelease,omitempty"`
	GrabbedDownload *acquisition.DownloadStatus `json:"grabbedDownload,omitempty"`
	Error           string                      `json:"error,omitempty"`
}

type MonitorItemResult struct {
	WantedItem      WantedItem                  `json:"wantedItem"`
	ReleasesFound   int                         `json:"releasesFound"`
	ApprovedCount   int                         `json:"approvedCount"`
	RejectedCount   int                         `json:"rejectedCount"`
	GrabbedDownload *acquisition.DownloadStatus `json:"grabbedDownload,omitempty"`
	Error           string                      `json:"error,omitempty"`
}

type HistoryQuery struct {
	Limit int `json:"limit,omitempty"`
}

type HistoryEvent struct {
	ID         string         `json:"id,omitempty"`
	EventType  string         `json:"eventType"`
	EntityType string         `json:"entityType,omitempty"`
	EntityID   string         `json:"entityId,omitempty"`
	Severity   string         `json:"severity"`
	Message    string         `json:"message"`
	Data       map[string]any `json:"data,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
}
