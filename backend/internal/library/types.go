package library

import "time"

type Config struct {
	EbookRoot     string
	AudiobookRoot string
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
	SourcePath string `json:"sourcePath"`
	WantedID   string `json:"wantedId,omitempty"`
	Format     string `json:"format,omitempty"`
	Move       bool   `json:"move,omitempty"`
}

type ImportOutcome struct {
	File            FileRecord `json:"file"`
	DestinationPath string     `json:"destinationPath"`
	Moved           bool       `json:"moved"`
}
