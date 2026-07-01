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

type ReleaseFeedQuery struct {
	Format string `json:"format,omitempty"`
	Limit  int    `json:"limit,omitempty"`
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
	Client     string   `json:"client,omitempty"`
	ReleaseURL string   `json:"releaseUrl"`
	InfoHash   string   `json:"infoHash,omitempty"`
	Title      string   `json:"title,omitempty"`
	Protocol   string   `json:"protocol,omitempty"`
	Category   string   `json:"category,omitempty"`
	SavePath   string   `json:"savePath,omitempty"`
	Paused     bool     `json:"paused"`
	Tags       []string `json:"tags,omitempty"`
	UploadName string   `json:"uploadName,omitempty"`
	UploadData []byte   `json:"-"`
}

type DownloadStatus struct {
	Client          string     `json:"client,omitempty"`
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
	LastActivityAt  *time.Time `json:"lastActivityAt,omitempty"`
	LastSeenAt      *time.Time `json:"lastSeenAt,omitempty"`
	ImportStatus    string     `json:"importStatus,omitempty"`
	ImportedFileID  string     `json:"importedFileId,omitempty"`
	ImportedAt      *time.Time `json:"importedAt,omitempty"`
	ImportError     string     `json:"importError,omitempty"`
	FailureReason   string     `json:"failureReason,omitempty"`
	FailedAt        *time.Time `json:"failedAt,omitempty"`
	RetryCount      int        `json:"retryCount,omitempty"`
	ReplacementID   string     `json:"replacementId,omitempty"`
	// Wanted-item linkage resolved at the API layer from `wanted:<id>` tags;
	// never persisted by the download store.
	WantedID     string `json:"wantedId,omitempty"`
	WantedTitle  string `json:"wantedTitle,omitempty"`
	WantedAuthor string `json:"wantedAuthor,omitempty"`
}

type DownloadDetails struct {
	Status     DownloadStatus     `json:"status"`
	Properties DownloadProperties `json:"properties,omitempty"`
	Files      []DownloadFile     `json:"files,omitempty"`
	Trackers   []DownloadTracker  `json:"trackers,omitempty"`
	Peers      []DownloadPeer     `json:"peers,omitempty"`
}

type DownloadProperties struct {
	SavePath           string     `json:"savePath,omitempty"`
	CreationDate       *time.Time `json:"creationDate,omitempty"`
	AdditionDate       *time.Time `json:"additionDate,omitempty"`
	CompletionDate     *time.Time `json:"completionDate,omitempty"`
	TotalSizeBytes     int64      `json:"totalSizeBytes,omitempty"`
	TotalDownloaded    int64      `json:"totalDownloaded,omitempty"`
	TotalUploaded      int64      `json:"totalUploaded,omitempty"`
	DownloadLimit      int64      `json:"downloadLimit,omitempty"`
	UploadLimit        int64      `json:"uploadLimit,omitempty"`
	DownloadSpeed      int64      `json:"downloadSpeed,omitempty"`
	UploadSpeed        int64      `json:"uploadSpeed,omitempty"`
	ETASeconds         int64      `json:"etaSeconds,omitempty"`
	Ratio              float64    `json:"ratio,omitempty"`
	Connections        int        `json:"connections,omitempty"`
	ConnectionsLimit   int        `json:"connectionsLimit,omitempty"`
	TimeElapsedSeconds int64      `json:"timeElapsedSeconds,omitempty"`
	SeedingTimeSeconds int64      `json:"seedingTimeSeconds,omitempty"`
	PieceSizeBytes     int64      `json:"pieceSizeBytes,omitempty"`
	PiecesHave         int        `json:"piecesHave,omitempty"`
	PiecesTotal        int        `json:"piecesTotal,omitempty"`
	ReannounceSeconds  int64      `json:"reannounceSeconds,omitempty"`
	CreatedBy          string     `json:"createdBy,omitempty"`
	Comment            string     `json:"comment,omitempty"`
}

type DownloadFile struct {
	ID           int     `json:"id"`
	ExternalID   string  `json:"externalId,omitempty"`
	Name         string  `json:"name"`
	SizeBytes    int64   `json:"sizeBytes,omitempty"`
	Progress     float64 `json:"progress"`
	Priority     int     `json:"priority"`
	Availability float64 `json:"availability,omitempty"`
	IsSeed       bool    `json:"isSeed,omitempty"`
	FirstPiece   int     `json:"firstPiece,omitempty"`
	LastPiece    int     `json:"lastPiece,omitempty"`
}

type DownloadTracker struct {
	URL        string `json:"url"`
	StatusCode int    `json:"statusCode"`
	Status     string `json:"status"`
	Tier       int    `json:"tier,omitempty"`
	Message    string `json:"message,omitempty"`
	Peers      int    `json:"peers,omitempty"`
	Seeds      int    `json:"seeds,omitempty"`
	Leeches    int    `json:"leeches,omitempty"`
	Downloads  int    `json:"downloads,omitempty"`
}

type DownloadCategory struct {
	Name     string `json:"name"`
	SavePath string `json:"savePath,omitempty"`
}

type DownloadResources struct {
	Client     string             `json:"client"`
	Categories []DownloadCategory `json:"categories"`
	Tags       []string           `json:"tags"`
}

type DownloadPreferences struct {
	Client                       string `json:"client"`
	SavePath                     string `json:"savePath,omitempty"`
	TempPathEnabled              bool   `json:"tempPathEnabled,omitempty"`
	TempPath                     string `json:"tempPath,omitempty"`
	StartPaused                  bool   `json:"startPaused"`
	DownloadLimit                int64  `json:"downloadLimit"`
	UploadLimit                  int64  `json:"uploadLimit"`
	AlternativeDownloadLimit     int64  `json:"alternativeDownloadLimit,omitempty"`
	AlternativeUploadLimit       int64  `json:"alternativeUploadLimit,omitempty"`
	SpeedScheduleEnabled         bool   `json:"speedScheduleEnabled"`
	QueueingEnabled              bool   `json:"queueingEnabled"`
	MaxActiveDownloads           int    `json:"maxActiveDownloads"`
	MaxActiveUploads             int    `json:"maxActiveUploads"`
	MaxActiveTorrents            int    `json:"maxActiveTorrents"`
	LibrarryPreferenceWriteScope string `json:"librarryPreferenceWriteScope"`
}

type DownloadPreferencesUpdate struct {
	Client                   string  `json:"client,omitempty"`
	SavePath                 *string `json:"savePath,omitempty"`
	TempPathEnabled          *bool   `json:"tempPathEnabled,omitempty"`
	TempPath                 *string `json:"tempPath,omitempty"`
	StartPaused              *bool   `json:"startPaused,omitempty"`
	DownloadLimit            *int64  `json:"downloadLimit,omitempty"`
	UploadLimit              *int64  `json:"uploadLimit,omitempty"`
	AlternativeDownloadLimit *int64  `json:"alternativeDownloadLimit,omitempty"`
	AlternativeUploadLimit   *int64  `json:"alternativeUploadLimit,omitempty"`
	SpeedScheduleEnabled     *bool   `json:"speedScheduleEnabled,omitempty"`
	QueueingEnabled          *bool   `json:"queueingEnabled,omitempty"`
	MaxActiveDownloads       *int    `json:"maxActiveDownloads,omitempty"`
	MaxActiveUploads         *int    `json:"maxActiveUploads,omitempty"`
	MaxActiveTorrents        *int    `json:"maxActiveTorrents,omitempty"`
}

type DownloadCategoryActionRequest struct {
	Client   string `json:"client,omitempty"`
	Action   string `json:"action"`
	Name     string `json:"name"`
	NewName  string `json:"newName,omitempty"`
	SavePath string `json:"savePath,omitempty"`
}

type DownloadTagActionRequest struct {
	Client  string   `json:"client,omitempty"`
	Action  string   `json:"action"`
	Names   []string `json:"names"`
	NewName string   `json:"newName,omitempty"`
}

type DownloadResourceActionResult struct {
	Action    string             `json:"action"`
	Client    string             `json:"client"`
	Applied   bool               `json:"applied"`
	Message   string             `json:"message,omitempty"`
	Resources *DownloadResources `json:"resources,omitempty"`
}

type DownloadPeer struct {
	ID               string  `json:"id"`
	IP               string  `json:"ip"`
	Port             int     `json:"port,omitempty"`
	Client           string  `json:"client,omitempty"`
	Connection       string  `json:"connection,omitempty"`
	Country          string  `json:"country,omitempty"`
	CountryCode      string  `json:"countryCode,omitempty"`
	Flags            string  `json:"flags,omitempty"`
	FlagsDescription string  `json:"flagsDescription,omitempty"`
	Progress         float64 `json:"progress,omitempty"`
	Relevance        float64 `json:"relevance,omitempty"`
	DownloadRate     int64   `json:"downloadRate,omitempty"`
	UploadRate       int64   `json:"uploadRate,omitempty"`
	DownloadedBytes  int64   `json:"downloadedBytes,omitempty"`
	UploadedBytes    int64   `json:"uploadedBytes,omitempty"`
	Files            string  `json:"files,omitempty"`
}

type DownloadClient interface {
	Name() string
	Add(ctx context.Context, request DownloadRequest) (DownloadStatus, error)
	Status(ctx context.Context, id string) (DownloadStatus, error)
}

type DownloadListQuery struct {
	IDs      []string
	Client   string
	Tag      string
	Category string
}

type DownloadActionRequest struct {
	Action        string   `json:"action"`
	Client        string   `json:"client,omitempty"`
	IDs           []string `json:"ids"`
	DeleteFiles   bool     `json:"deleteFiles,omitempty"`
	Category      string   `json:"category,omitempty"`
	SavePath      string   `json:"savePath,omitempty"`
	Name          string   `json:"name,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	ForceStart    bool     `json:"forceStart,omitempty"`
	DownloadLimit int64    `json:"downloadLimit,omitempty"`
	UploadLimit   int64    `json:"uploadLimit,omitempty"`
}

type DownloadActionResult struct {
	Action    string           `json:"action"`
	IDs       []string         `json:"ids"`
	Applied   bool             `json:"applied"`
	Message   string           `json:"message,omitempty"`
	Downloads []DownloadStatus `json:"downloads,omitempty"`
}

type DownloadFileActionRequest struct {
	DownloadID string `json:"downloadId,omitempty"`
	Client     string `json:"client,omitempty"`
	Action     string `json:"action"`
	IDs        []int  `json:"ids"`
	Priority   int    `json:"priority,omitempty"`
}

type DownloadFileActionResult struct {
	Action     string           `json:"action"`
	DownloadID string           `json:"downloadId"`
	IDs        []int            `json:"ids"`
	Priority   int              `json:"priority"`
	Applied    bool             `json:"applied"`
	Message    string           `json:"message,omitempty"`
	Download   *DownloadDetails `json:"download,omitempty"`
}

type DownloadTrackerActionRequest struct {
	Client      string   `json:"client,omitempty"`
	Action      string   `json:"action"`
	URLs        []string `json:"urls,omitempty"`
	URL         string   `json:"url,omitempty"`
	OriginalURL string   `json:"originalUrl,omitempty"`
	NewURL      string   `json:"newUrl,omitempty"`
}

type DownloadTrackerActionResult struct {
	Action     string           `json:"action"`
	DownloadID string           `json:"downloadId"`
	URLs       []string         `json:"urls,omitempty"`
	Applied    bool             `json:"applied"`
	Message    string           `json:"message,omitempty"`
	Download   *DownloadDetails `json:"download,omitempty"`
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
	DownloadActionSetDownloadLimit = "setDownloadLimit"
	DownloadActionSetUploadLimit   = "setUploadLimit"
	DownloadActionForceStart       = "forceStart"
	DownloadActionToggleSequential = "toggleSequential"
	DownloadActionToggleFirstLast  = "toggleFirstLastPiece"
	DownloadActionRename           = "rename"
	DownloadActionAddTags          = "addTags"
	DownloadActionRemoveTags       = "removeTags"
)

const (
	DownloadTrackerActionAdd    = "add"
	DownloadTrackerActionEdit   = "edit"
	DownloadTrackerActionRemove = "remove"
)

const (
	DownloadFileActionSkip   = "skip"
	DownloadFileActionNormal = "normal"
	DownloadFileActionHigh   = "high"
	DownloadFileActionMax    = "max"
)

type QBittorrentConfig struct {
	BaseURL  string
	Username string
	Password string
}

type SABnzbdConfig struct {
	BaseURL  string
	APIKey   string
	Username string
	Password string
}

type TransmissionConfig struct {
	BaseURL  string
	Username string
	Password string
}

const (
	CategoryBooksEbook     = "books-ebook"
	CategoryBooksAudiobook = "books-audiobook"
	DefaultEbookRoot       = "/data/media/books/ebooks"
	DefaultAudiobookRoot   = "/data/media/books/audiobooks"
	DefaultTorrentRoot     = "/data/torrents/books"
)
