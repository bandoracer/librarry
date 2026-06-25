package acquisition

import (
	"context"
	"time"
)

type ReleaseSearchQuery struct {
	Query     string   `json:"query"`
	Author    string   `json:"author,omitempty"`
	ISBN      string   `json:"isbn,omitempty"`
	Format    string   `json:"format,omitempty"`
	Languages []string `json:"languages,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

type Release struct {
	ID          string    `json:"id"`
	InfoHash    string    `json:"infoHash,omitempty"`
	Indexer     string    `json:"indexer"`
	Title       string    `json:"title"`
	SizeBytes   int64     `json:"sizeBytes,omitempty"`
	Seeders     int       `json:"seeders,omitempty"`
	Leechers    int       `json:"leechers,omitempty"`
	DownloadURL string    `json:"downloadUrl"`
	InfoURL     string    `json:"infoUrl,omitempty"`
	Protocol    string    `json:"protocol"`
	Categories  []string  `json:"categories,omitempty"`
	PublishedAt time.Time `json:"publishedAt,omitempty"`
}

type IndexerClient interface {
	Name() string
	Search(ctx context.Context, query ReleaseSearchQuery) ([]Release, error)
}

type DownloadRequest struct {
	ReleaseURL string   `json:"releaseUrl"`
	InfoHash   string   `json:"infoHash,omitempty"`
	Title      string   `json:"title,omitempty"`
	Category   string   `json:"category,omitempty"`
	SavePath   string   `json:"savePath,omitempty"`
	Paused     bool     `json:"paused"`
	Tags       []string `json:"tags,omitempty"`
}

type DownloadStatus struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	State           string     `json:"state"`
	Progress        float64    `json:"progress"`
	SavePath        string     `json:"savePath"`
	Category        string     `json:"category"`
	Tags            []string   `json:"tags,omitempty"`
	SizeBytes       int64      `json:"sizeBytes,omitempty"`
	DownloadedBytes int64      `json:"downloadedBytes,omitempty"`
	UploadedBytes   int64      `json:"uploadedBytes,omitempty"`
	DownloadRate    int64      `json:"downloadRate,omitempty"`
	UploadRate      int64      `json:"uploadRate,omitempty"`
	ETASeconds      int64      `json:"etaSeconds,omitempty"`
	Ratio           float64    `json:"ratio,omitempty"`
	Seeders         int        `json:"seeders,omitempty"`
	Peers           int        `json:"peers,omitempty"`
	AddedAt         *time.Time `json:"addedAt,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	LastSeenAt      *time.Time `json:"lastSeenAt,omitempty"`
	ImportStatus    string     `json:"importStatus,omitempty"`
	ImportedFileID  string     `json:"importedFileId,omitempty"`
	ImportedAt      *time.Time `json:"importedAt,omitempty"`
	ImportError     string     `json:"importError,omitempty"`
}

type DownloadClient interface {
	Name() string
	Add(ctx context.Context, request DownloadRequest) (DownloadStatus, error)
	Status(ctx context.Context, id string) (DownloadStatus, error)
}

type DownloadListQuery struct {
	IDs      []string
	Tag      string
	Category string
}

type DownloadActionRequest struct {
	Action      string   `json:"action"`
	IDs         []string `json:"ids"`
	DeleteFiles bool     `json:"deleteFiles,omitempty"`
	Category    string   `json:"category,omitempty"`
	SavePath    string   `json:"savePath,omitempty"`
}

type DownloadActionResult struct {
	Action    string           `json:"action"`
	IDs       []string         `json:"ids"`
	Applied   bool             `json:"applied"`
	Message   string           `json:"message,omitempty"`
	Downloads []DownloadStatus `json:"downloads,omitempty"`
}

const (
	DownloadActionStart            = "start"
	DownloadActionStop             = "stop"
	DownloadActionDelete           = "delete"
	DownloadActionRecheck          = "recheck"
	DownloadActionIncreasePriority = "increasePriority"
	DownloadActionDecreasePriority = "decreasePriority"
	DownloadActionTopPriority      = "topPriority"
	DownloadActionBottomPriority   = "bottomPriority"
	DownloadActionSetCategory      = "setCategory"
	DownloadActionSetLocation      = "setLocation"
)

type QBittorrentConfig struct {
	BaseURL  string
	Username string
	Password string
}

type SABnzbdConfig struct {
	BaseURL string
	APIKey  string
}

const (
	CategoryBooksEbook     = "books-ebook"
	CategoryBooksAudiobook = "books-audiobook"
	DefaultEbookRoot       = "/data/media/books/ebooks"
	DefaultAudiobookRoot   = "/data/media/books/audiobooks"
	DefaultTorrentRoot     = "/data/torrents/books"
)
