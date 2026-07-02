package library

import (
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

type Config struct {
	EbookRoot                  string
	AudiobookRoot              string
	NamingAuthorFolderTemplate string
	NamingBookFolderTemplate   string
	NamingFileNameTemplate     string
	NamingSpaceReplacement     string
	StandardSearchLanguage     string
	// RecycleBin is the folder deleted/replaced library files move into
	// (empty disables the bin and files are removed outright).
	RecycleBin          string
	RecycleBinRetention time.Duration
	// ImportExtraFiles is a comma-separated extension list (".cue") of
	// sibling files copied alongside imports.
	ImportExtraFiles string
}

type FileRecord struct {
	ID           string         `json:"id,omitempty"`
	EditionID    string         `json:"editionId,omitempty"`
	MediaFormat  string         `json:"mediaFormat"`
	Path         string         `json:"path"`
	SourcePath   string         `json:"sourcePath,omitempty"`
	Title        string         `json:"title,omitempty"`
	AuthorName   string         `json:"authorName,omitempty"`
	Extension    string         `json:"extension,omitempty"`
	SizeBytes    int64          `json:"sizeBytes,omitempty"`
	Checksum     string         `json:"checksum,omitempty"`
	ImportStatus string         `json:"importStatus"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	ModifiedAt   *time.Time     `json:"modifiedAt,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

type FileListQuery struct {
	Format string `json:"format,omitempty"`
	Status string `json:"status,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type ScanRequest struct {
	Format string `json:"format,omitempty"`
	Root   string `json:"root,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type ScanOutcome struct {
	Roots    []string     `json:"roots"`
	Scanned  int          `json:"scanned"`
	Upserted int          `json:"upserted"`
	Skipped  int          `json:"skipped"`
	Files    []FileRecord `json:"files"`
	Errors   []string     `json:"errors,omitempty"`
}

type ImportRequest struct {
	SourcePath     string `json:"sourcePath"`
	WantedID       string `json:"wantedId,omitempty"`
	DownloadID     string `json:"downloadId,omitempty"`
	Format         string `json:"format,omitempty"`
	Move           bool   `json:"move,omitempty"`
	ImportMode     string `json:"importMode,omitempty"`
	ConflictAction string `json:"conflictAction,omitempty"`
	Overwrite      bool   `json:"overwrite,omitempty"`
}

type ImportOutcome struct {
	File            FileRecord `json:"file"`
	DestinationPath string     `json:"destinationPath"`
	Moved           bool       `json:"moved"`
	Imported        bool       `json:"imported"`
	Skipped         bool       `json:"skipped,omitempty"`
	Replaced        bool       `json:"replaced,omitempty"`
	Hardlinked      bool       `json:"hardlinked,omitempty"`
	ImportMode      string     `json:"importMode,omitempty"`
	ConflictAction  string     `json:"conflictAction,omitempty"`
	ConflictPath    string     `json:"conflictPath,omitempty"`
	Message         string     `json:"message,omitempty"`
}

type CompletedImportRequest struct {
	DownloadIDs    []string `json:"downloadIds,omitempty"`
	Move           bool     `json:"move,omitempty"`
	ImportMode     string   `json:"importMode,omitempty"`
	ConflictAction string   `json:"conflictAction,omitempty"`
	Overwrite      bool     `json:"overwrite,omitempty"`
	AutoMatch      *bool    `json:"autoMatch,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

type CompletedImportOutcome struct {
	Checked      int                    `json:"checked"`
	Imported     int                    `json:"imported"`
	AutoMatched  int                    `json:"autoMatched"`
	ReviewQueued int                    `json:"reviewQueued"`
	Skipped      int                    `json:"skipped"`
	Errored      int                    `json:"errored"`
	Results      []DownloadImportResult `json:"results"`
}

type DeleteFilesRequest struct {
	IDs         []string `json:"ids,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	DeleteFiles bool     `json:"deleteFiles,omitempty"`
}

type DeleteFilesOutcome struct {
	Requested int                `json:"requested"`
	Deleted   int                `json:"deleted"`
	Skipped   int                `json:"skipped"`
	Errored   int                `json:"errored"`
	Files     []FileRecord       `json:"files"`
	Results   []DeleteFileResult `json:"results"`
}

type DeleteFileResult struct {
	File    FileRecord `json:"file"`
	Status  string     `json:"status"`
	Message string     `json:"message,omitempty"`
}

type RenameFilesRequest struct {
	IDs       []string `json:"ids,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	Overwrite bool     `json:"overwrite,omitempty"`
}

type RenameFilesOutcome struct {
	Requested int                 `json:"requested"`
	Renamed   int                 `json:"renamed"`
	Skipped   int                 `json:"skipped"`
	Errored   int                 `json:"errored"`
	Previews  []RenameFilePreview `json:"previews"`
	Results   []RenameFileResult  `json:"results,omitempty"`
}

type RenameFilePreview struct {
	File            FileRecord `json:"file"`
	SourcePath      string     `json:"sourcePath"`
	DestinationPath string     `json:"destinationPath"`
	RelativePath    string     `json:"relativePath"`
	Exists          bool       `json:"exists"`
	Noop            bool       `json:"noop"`
}

type RenameFileResult struct {
	Preview RenameFilePreview `json:"preview"`
	File    *FileRecord       `json:"file,omitempty"`
	Status  string            `json:"status"`
	Message string            `json:"message,omitempty"`
}

type CalibreConversionRefreshRequest struct {
	IDs            []string `json:"ids,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	MaxAttempts    int      `json:"maxAttempts,omitempty"`
	IntervalMillis int      `json:"intervalMillis,omitempty"`
	Force          bool     `json:"force,omitempty"`
}

type CalibreConversionRefreshOutcome struct {
	Checked   int                              `json:"checked"`
	Refreshed int                              `json:"refreshed"`
	Skipped   int                              `json:"skipped"`
	Errored   int                              `json:"errored"`
	Results   []CalibreConversionRefreshResult `json:"results"`
}

type CalibreConversionRefreshResult struct {
	File     FileRecord       `json:"file"`
	Status   string           `json:"status"`
	Message  string           `json:"message,omitempty"`
	Statuses []map[string]any `json:"statuses,omitempty"`
}

type DownloadImportResult struct {
	Download    acquisition.DownloadStatus `json:"download"`
	Status      string                     `json:"status"`
	Message     string                     `json:"message,omitempty"`
	SourcePath  string                     `json:"sourcePath,omitempty"`
	WantedID    string                     `json:"wantedId,omitempty"`
	AutoMatched bool                       `json:"autoMatched,omitempty"`
	Import      *ImportOutcome             `json:"import,omitempty"`
	Review      *ImportReview              `json:"review,omitempty"`
}

type ReviewListQuery struct {
	Status string `json:"status,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type ImportReview struct {
	ID              string         `json:"id,omitempty"`
	SourcePath      string         `json:"sourcePath"`
	DownloadID      string         `json:"downloadId,omitempty"`
	WantedID        string         `json:"wantedId,omitempty"`
	MediaFormat     string         `json:"mediaFormat"`
	Title           string         `json:"title,omitempty"`
	AuthorName      string         `json:"authorName,omitempty"`
	SizeBytes       int64          `json:"sizeBytes,omitempty"`
	Reason          string         `json:"reason"`
	Status          string         `json:"status"`
	Decision        string         `json:"decision,omitempty"`
	DestinationPath string         `json:"destinationPath,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	ResolvedAt      *time.Time     `json:"resolvedAt,omitempty"`
}

type ReviewDecisionRequest struct {
	Action         string `json:"action"`
	WantedID       string `json:"wantedId,omitempty"`
	Format         string `json:"format,omitempty"`
	Move           bool   `json:"move,omitempty"`
	ImportMode     string `json:"importMode,omitempty"`
	ConflictAction string `json:"conflictAction,omitempty"`
	Overwrite      bool   `json:"overwrite,omitempty"`
}

type ReviewDecisionOutcome struct {
	Review ImportReview   `json:"review"`
	Import *ImportOutcome `json:"import,omitempty"`
}

type ReviewBulkDecisionRequest struct {
	IDs            []string `json:"ids,omitempty"`
	ReviewIDs      []string `json:"reviewIds,omitempty"`
	Action         string   `json:"action"`
	WantedID       string   `json:"wantedId,omitempty"`
	Format         string   `json:"format,omitempty"`
	Move           bool     `json:"move,omitempty"`
	ImportMode     string   `json:"importMode,omitempty"`
	ConflictAction string   `json:"conflictAction,omitempty"`
	Overwrite      bool     `json:"overwrite,omitempty"`
	Status         string   `json:"status,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

type ReviewBulkDecisionOutcome struct {
	Requested int                        `json:"requested"`
	Resolved  int                        `json:"resolved"`
	Imported  int                        `json:"imported"`
	Skipped   int                        `json:"skipped"`
	Rejected  int                        `json:"rejected"`
	Errored   int                        `json:"errored"`
	Results   []ReviewBulkDecisionResult `json:"results"`
}

type ReviewBulkDecisionResult struct {
	ID      string                 `json:"id"`
	Status  string                 `json:"status"`
	Message string                 `json:"message,omitempty"`
	Outcome *ReviewDecisionOutcome `json:"outcome,omitempty"`
}
