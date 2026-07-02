package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr                string
	APIKey                    string
	DatabaseURL               string
	MigrationsDir             string
	HardcoverToken            string
	GoogleBooksAPIKey         string
	ProwlarrURL               string
	ProwlarrAPIKey            string
	QBittorrentURL            string
	QBittorrentUser           string
	QBittorrentPass           string
	TransmissionURL           string
	TransmissionUser          string
	TransmissionPass          string
	SABnzbdURL                string
	SABnzbdAPIKey             string
	SABnzbdUser               string
	SABnzbdPass               string
	EbookCategory             string
	AudiobookCategory         string
	BookTorrentRoot           string
	EbookLibraryRoot          string
	AudiobookLibraryRoot      string
	NamingAuthorFolder        string
	NamingBookFolder          string
	NamingFileName            string
	NamingSpaceReplacement    string
	// RenameBooks applies the naming templates on import. When false, imports
	// keep the source basename inside the author folder (arr renaming-off
	// behavior). Librarry defaults on; Readarr defaults off.
	RenameBooks            bool
	StandardSearchLanguage string
	RecycleBin                string
	RecycleBinRetention       time.Duration
	ImportExtraFiles          string
	MonitorEnabled            bool
	MonitorInterval           time.Duration
	MonitorSearchInterval     time.Duration
	MonitorLimit              int
	MonitorAutoGrab           bool
	AuthorMonitorEnabled      bool
	AuthorMonitorInterval     time.Duration
	AuthorMonitorSyncInterval time.Duration
	AuthorMonitorLimit        int
	FeedSyncEnabled           bool
	FeedSyncInterval          time.Duration
	FeedSyncLimit             int
	FeedSyncAutoGrab          bool
	FailedDownloadEnabled     bool
	FailedDownloadInterval    time.Duration
	FailedDownloadStalledAge  time.Duration
	FailedDownloadLimit       int
	FailedDownloadAutoGrab    bool
	FailedDownloadRemove      bool
	FailedDownloadDeleteFiles bool
	UpgradeSearchEnabled      bool
	UpgradeSearchInterval     time.Duration
	UpgradeSearchLimit        int
	UpgradeSearchAutoGrab     bool
	UpgradeSearchMinDelta     float64
	CalibreRefreshEnabled     bool
	CalibreRefreshInterval    time.Duration
	CalibreRefreshLimit       int
	CalibreRefreshMaxAttempts int
	CompletedImportEnabled    bool
	CompletedImportInterval   time.Duration
	CompletedImportLimit      int
	CompletedImportMode       string
	CompletedRemoveEnabled    bool
	// AuthMethod is the arr-style API auth mode: "none" (default), "basic",
	// or "forms". Empty means "unset by env" so a UI-persisted method can win.
	AuthMethod             string
	AuthUsername           string
	AuthPassword           string
	ImportListSyncInterval time.Duration
	BackupEnabled          bool
	BackupInterval         time.Duration
	BackupRetention        int
	BackupDir              string
	WebOrigin              string
}

func FromEnv() Config {
	return Config{
		ListenAddr:                env("LIBRARRY_LISTEN_ADDR", ":8080"),
		APIKey:                    strings.TrimSpace(os.Getenv("LIBRARRY_API_KEY")),
		DatabaseURL:               strings.TrimSpace(os.Getenv("LIBRARRY_DATABASE_URL")),
		MigrationsDir:             env("LIBRARRY_MIGRATIONS_DIR", "backend/migrations"),
		HardcoverToken:            strings.TrimSpace(os.Getenv("LIBRARRY_HARDCOVER_TOKEN")),
		GoogleBooksAPIKey:         strings.TrimSpace(os.Getenv("LIBRARRY_GOOGLE_BOOKS_API_KEY")),
		ProwlarrURL:               strings.TrimRight(strings.TrimSpace(os.Getenv("LIBRARRY_PROWLARR_URL")), "/"),
		ProwlarrAPIKey:            strings.TrimSpace(os.Getenv("LIBRARRY_PROWLARR_API_KEY")),
		QBittorrentURL:            strings.TrimRight(strings.TrimSpace(os.Getenv("LIBRARRY_QBITTORRENT_URL")), "/"),
		QBittorrentUser:           strings.TrimSpace(os.Getenv("LIBRARRY_QBITTORRENT_USERNAME")),
		QBittorrentPass:           strings.TrimSpace(os.Getenv("LIBRARRY_QBITTORRENT_PASSWORD")),
		TransmissionURL:           strings.TrimRight(strings.TrimSpace(os.Getenv("LIBRARRY_TRANSMISSION_URL")), "/"),
		TransmissionUser:          strings.TrimSpace(os.Getenv("LIBRARRY_TRANSMISSION_USERNAME")),
		TransmissionPass:          strings.TrimSpace(os.Getenv("LIBRARRY_TRANSMISSION_PASSWORD")),
		SABnzbdURL:                strings.TrimRight(strings.TrimSpace(os.Getenv("LIBRARRY_SABNZBD_URL")), "/"),
		SABnzbdAPIKey:             strings.TrimSpace(os.Getenv("LIBRARRY_SABNZBD_API_KEY")),
		SABnzbdUser:               strings.TrimSpace(os.Getenv("LIBRARRY_SABNZBD_USERNAME")),
		SABnzbdPass:               strings.TrimSpace(os.Getenv("LIBRARRY_SABNZBD_PASSWORD")),
		EbookCategory:             env("LIBRARRY_EBOOK_CATEGORY", "books-ebook"),
		AudiobookCategory:         env("LIBRARRY_AUDIOBOOK_CATEGORY", "books-audiobook"),
		BookTorrentRoot:           env("LIBRARRY_BOOK_TORRENT_ROOT", "/data/torrents/books"),
		EbookLibraryRoot:          env("LIBRARRY_EBOOK_LIBRARY_ROOT", "/data/media/books/ebooks"),
		AudiobookLibraryRoot:      env("LIBRARRY_AUDIOBOOK_LIBRARY_ROOT", "/data/media/books/audiobooks"),
		NamingAuthorFolder:        env("LIBRARRY_NAMING_AUTHOR_FOLDER", "{Author}"),
		NamingBookFolder:          env("LIBRARRY_NAMING_BOOK_FOLDER", "{Title}"),
		NamingFileName:            env("LIBRARRY_NAMING_FILE_NAME", "{Title}{Ext}"),
		NamingSpaceReplacement:    strings.TrimSpace(os.Getenv("LIBRARRY_NAMING_SPACE_REPLACEMENT")),
		RenameBooks:               envBool("LIBRARRY_RENAME_BOOKS", true),
		StandardSearchLanguage:    env("LIBRARRY_STANDARD_SEARCH_LANGUAGE", "English"),
		RecycleBin:                strings.TrimSpace(os.Getenv("LIBRARRY_RECYCLE_BIN")),
		RecycleBinRetention:       envDuration("LIBRARRY_RECYCLE_BIN_RETENTION", 168*time.Hour),
		ImportExtraFiles:          env("LIBRARRY_IMPORT_EXTRA_FILES", ".cue"),
		MonitorEnabled:            envBool("LIBRARRY_MONITOR_ENABLED", true),
		MonitorInterval:           envDuration("LIBRARRY_MONITOR_INTERVAL", 30*time.Minute),
		MonitorSearchInterval:     envDuration("LIBRARRY_MONITOR_SEARCH_INTERVAL", 6*time.Hour),
		MonitorLimit:              envInt("LIBRARRY_MONITOR_LIMIT", 50),
		MonitorAutoGrab:           envBool("LIBRARRY_MONITOR_AUTO_GRAB", true),
		AuthorMonitorEnabled:      envBool("LIBRARRY_AUTHOR_MONITOR_ENABLED", true),
		AuthorMonitorInterval:     envDuration("LIBRARRY_AUTHOR_MONITOR_INTERVAL", 6*time.Hour),
		AuthorMonitorSyncInterval: envDuration("LIBRARRY_AUTHOR_MONITOR_SYNC_INTERVAL", 24*time.Hour),
		AuthorMonitorLimit:        envInt("LIBRARRY_AUTHOR_MONITOR_LIMIT", 50),
		FeedSyncEnabled:           envBool("LIBRARRY_FEED_SYNC_ENABLED", true),
		FeedSyncInterval:          envDuration("LIBRARRY_FEED_SYNC_INTERVAL", 15*time.Minute),
		FeedSyncLimit:             envInt("LIBRARRY_FEED_SYNC_LIMIT", 100),
		FeedSyncAutoGrab:          envBool("LIBRARRY_FEED_SYNC_AUTO_GRAB", true),
		FailedDownloadEnabled:     envBool("LIBRARRY_FAILED_DOWNLOAD_ENABLED", true),
		FailedDownloadInterval:    envDuration("LIBRARRY_FAILED_DOWNLOAD_INTERVAL", 30*time.Minute),
		FailedDownloadStalledAge:  envDuration("LIBRARRY_FAILED_DOWNLOAD_STALLED_AGE", 24*time.Hour),
		FailedDownloadLimit:       envInt("LIBRARRY_FAILED_DOWNLOAD_LIMIT", 50),
		FailedDownloadAutoGrab:    envBool("LIBRARRY_FAILED_DOWNLOAD_AUTO_GRAB", true),
		FailedDownloadRemove:      envBool("LIBRARRY_FAILED_DOWNLOAD_REMOVE", true),
		FailedDownloadDeleteFiles: envBool("LIBRARRY_FAILED_DOWNLOAD_DELETE_FILES", false),
		UpgradeSearchEnabled:      envBool("LIBRARRY_UPGRADE_SEARCH_ENABLED", true),
		UpgradeSearchInterval:     envDuration("LIBRARRY_UPGRADE_SEARCH_INTERVAL", 12*time.Hour),
		UpgradeSearchLimit:        envInt("LIBRARRY_UPGRADE_SEARCH_LIMIT", 50),
		UpgradeSearchAutoGrab:     envBool("LIBRARRY_UPGRADE_SEARCH_AUTO_GRAB", true),
		UpgradeSearchMinDelta:     envFloat("LIBRARRY_UPGRADE_SEARCH_MIN_DELTA", 5),
		CalibreRefreshEnabled:     envBool("LIBRARRY_CALIBRE_REFRESH_ENABLED", true),
		CalibreRefreshInterval:    envDuration("LIBRARRY_CALIBRE_REFRESH_INTERVAL", 15*time.Minute),
		CalibreRefreshLimit:       envInt("LIBRARRY_CALIBRE_REFRESH_LIMIT", 200),
		CalibreRefreshMaxAttempts: envInt("LIBRARRY_CALIBRE_REFRESH_MAX_ATTEMPTS", 1),
		CompletedImportEnabled:    envBool("LIBRARRY_COMPLETED_IMPORT_ENABLED", true),
		CompletedImportInterval:   envDuration("LIBRARRY_COMPLETED_IMPORT_INTERVAL", time.Minute),
		CompletedImportLimit:      envInt("LIBRARRY_COMPLETED_IMPORT_LIMIT", 50),
		CompletedImportMode:       env("LIBRARRY_COMPLETED_IMPORT_MODE", "hardlinkOrCopy"),
		CompletedRemoveEnabled:    envBool("LIBRARRY_COMPLETED_REMOVE_ENABLED", true),
		AuthMethod:                strings.ToLower(strings.TrimSpace(os.Getenv("LIBRARRY_AUTH_METHOD"))),
		AuthUsername:              strings.TrimSpace(os.Getenv("LIBRARRY_AUTH_USERNAME")),
		AuthPassword:              os.Getenv("LIBRARRY_AUTH_PASSWORD"),
		ImportListSyncInterval:    envDuration("LIBRARRY_IMPORT_LIST_SYNC_INTERVAL", 24*time.Hour),
		BackupEnabled:             envBool("LIBRARRY_BACKUP_ENABLED", true),
		BackupInterval:            envDuration("LIBRARRY_BACKUP_INTERVAL", 168*time.Hour),
		BackupRetention:           envInt("LIBRARRY_BACKUP_RETENTION", 4),
		BackupDir:                 env("LIBRARRY_BACKUP_DIR", "/config/backups"),
		WebOrigin:                 env("LIBRARRY_WEB_ORIGIN", "http://127.0.0.1:5173"),
	}
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err == nil {
		return parsed
	}
	minutes, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(minutes) * time.Minute
}
