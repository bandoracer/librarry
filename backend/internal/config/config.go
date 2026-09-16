package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr             string `env:"LIBRARRY_LISTEN_ADDR"`
	APIKey                 string `env:"LIBRARRY_API_KEY"`
	DatabaseURL            string `env:"LIBRARRY_DATABASE_URL"`
	MigrationsDir          string `env:"LIBRARRY_MIGRATIONS_DIR"`
	HardcoverToken         string `env:"LIBRARRY_HARDCOVER_TOKEN"`
	GoogleBooksAPIKey      string `env:"LIBRARRY_GOOGLE_BOOKS_API_KEY"`
	ProwlarrURL            string `env:"LIBRARRY_PROWLARR_URL"`
	ProwlarrAPIKey         string `env:"LIBRARRY_PROWLARR_API_KEY"`
	QBittorrentURL         string `env:"LIBRARRY_QBITTORRENT_URL"`
	QBittorrentUser        string `env:"LIBRARRY_QBITTORRENT_USERNAME"`
	QBittorrentPass        string `env:"LIBRARRY_QBITTORRENT_PASSWORD"`
	TransmissionURL        string `env:"LIBRARRY_TRANSMISSION_URL"`
	TransmissionUser       string `env:"LIBRARRY_TRANSMISSION_USERNAME"`
	TransmissionPass       string `env:"LIBRARRY_TRANSMISSION_PASSWORD"`
	SABnzbdURL             string `env:"LIBRARRY_SABNZBD_URL"`
	SABnzbdAPIKey          string `env:"LIBRARRY_SABNZBD_API_KEY"`
	SABnzbdUser            string `env:"LIBRARRY_SABNZBD_USERNAME"`
	SABnzbdPass            string `env:"LIBRARRY_SABNZBD_PASSWORD"`
	EbookCategory          string `env:"LIBRARRY_EBOOK_CATEGORY"`
	AudiobookCategory      string `env:"LIBRARRY_AUDIOBOOK_CATEGORY"`
	BookTorrentRoot        string `env:"LIBRARRY_BOOK_TORRENT_ROOT"`
	EbookLibraryRoot       string `env:"LIBRARRY_EBOOK_LIBRARY_ROOT"`
	AudiobookLibraryRoot   string `env:"LIBRARRY_AUDIOBOOK_LIBRARY_ROOT"`
	NamingAuthorFolder     string `env:"LIBRARRY_NAMING_AUTHOR_FOLDER"`
	NamingBookFolder       string `env:"LIBRARRY_NAMING_BOOK_FOLDER"`
	NamingFileName         string `env:"LIBRARRY_NAMING_FILE_NAME"`
	NamingSpaceReplacement string `env:"LIBRARRY_NAMING_SPACE_REPLACEMENT"`
	// RenameBooks applies the naming templates on import. When false, imports
	// keep the source basename inside the author folder (arr renaming-off
	// behavior). Librarry defaults on; Readarr defaults off.
	RenameBooks               bool          `env:"LIBRARRY_RENAME_BOOKS"`
	StandardSearchLanguage    string        `env:"LIBRARRY_STANDARD_SEARCH_LANGUAGE"`
	RecycleBin                string        `env:"LIBRARRY_RECYCLE_BIN"`
	RecycleBinRetention       time.Duration `env:"LIBRARRY_RECYCLE_BIN_RETENTION"`
	ImportExtraFiles          string        `env:"LIBRARRY_IMPORT_EXTRA_FILES"`
	MonitorEnabled            bool          `env:"LIBRARRY_MONITOR_ENABLED"`
	MonitorInterval           time.Duration `env:"LIBRARRY_MONITOR_INTERVAL"`
	MonitorSearchInterval     time.Duration `env:"LIBRARRY_MONITOR_SEARCH_INTERVAL"`
	MonitorLimit              int           `env:"LIBRARRY_MONITOR_LIMIT"`
	MonitorAutoGrab           bool          `env:"LIBRARRY_MONITOR_AUTO_GRAB"`
	AuthorMonitorEnabled      bool          `env:"LIBRARRY_AUTHOR_MONITOR_ENABLED"`
	AuthorMonitorInterval     time.Duration `env:"LIBRARRY_AUTHOR_MONITOR_INTERVAL"`
	AuthorMonitorSyncInterval time.Duration `env:"LIBRARRY_AUTHOR_MONITOR_SYNC_INTERVAL"`
	AuthorMonitorLimit        int           `env:"LIBRARRY_AUTHOR_MONITOR_LIMIT"`
	FeedSyncEnabled           bool          `env:"LIBRARRY_FEED_SYNC_ENABLED"`
	FeedSyncInterval          time.Duration `env:"LIBRARRY_FEED_SYNC_INTERVAL"`
	FeedSyncLimit             int           `env:"LIBRARRY_FEED_SYNC_LIMIT"`
	FeedSyncAutoGrab          bool          `env:"LIBRARRY_FEED_SYNC_AUTO_GRAB"`
	FailedDownloadEnabled     bool          `env:"LIBRARRY_FAILED_DOWNLOAD_ENABLED"`
	FailedDownloadInterval    time.Duration `env:"LIBRARRY_FAILED_DOWNLOAD_INTERVAL"`
	FailedDownloadStalledAge  time.Duration `env:"LIBRARRY_FAILED_DOWNLOAD_STALLED_AGE"`
	FailedDownloadLimit       int           `env:"LIBRARRY_FAILED_DOWNLOAD_LIMIT"`
	FailedDownloadAutoGrab    bool          `env:"LIBRARRY_FAILED_DOWNLOAD_AUTO_GRAB"`
	FailedDownloadRemove      bool          `env:"LIBRARRY_FAILED_DOWNLOAD_REMOVE"`
	FailedDownloadDeleteFiles bool          `env:"LIBRARRY_FAILED_DOWNLOAD_DELETE_FILES"`
	UpgradeSearchEnabled      bool          `env:"LIBRARRY_UPGRADE_SEARCH_ENABLED"`
	UpgradeSearchInterval     time.Duration `env:"LIBRARRY_UPGRADE_SEARCH_INTERVAL"`
	UpgradeSearchLimit        int           `env:"LIBRARRY_UPGRADE_SEARCH_LIMIT"`
	UpgradeSearchAutoGrab     bool          `env:"LIBRARRY_UPGRADE_SEARCH_AUTO_GRAB"`
	UpgradeSearchMinDelta     float64       `env:"LIBRARRY_UPGRADE_SEARCH_MIN_DELTA"`
	CalibreRefreshEnabled     bool          `env:"LIBRARRY_CALIBRE_REFRESH_ENABLED"`
	CalibreRefreshInterval    time.Duration `env:"LIBRARRY_CALIBRE_REFRESH_INTERVAL"`
	CalibreRefreshLimit       int           `env:"LIBRARRY_CALIBRE_REFRESH_LIMIT"`
	CalibreRefreshMaxAttempts int           `env:"LIBRARRY_CALIBRE_REFRESH_MAX_ATTEMPTS"`
	CompletedImportEnabled    bool          `env:"LIBRARRY_COMPLETED_IMPORT_ENABLED"`
	CompletedImportInterval   time.Duration `env:"LIBRARRY_COMPLETED_IMPORT_INTERVAL"`
	CompletedImportLimit      int           `env:"LIBRARRY_COMPLETED_IMPORT_LIMIT"`
	CompletedImportMode       string        `env:"LIBRARRY_COMPLETED_IMPORT_MODE"`
	CompletedRemoveEnabled    bool          `env:"LIBRARRY_COMPLETED_REMOVE_ENABLED"`
	// AuthMethod is the arr-style API auth mode: "none" (default), "basic",
	// or "forms". Empty means "unset by env" so a UI-persisted method can win.
	AuthMethod             string        `env:"LIBRARRY_AUTH_METHOD"`
	AuthUsername           string        `env:"LIBRARRY_AUTH_USERNAME"`
	AuthPassword           string        `env:"LIBRARRY_AUTH_PASSWORD"`
	ImportListSyncEnabled  bool          `env:"LIBRARRY_IMPORT_LIST_SYNC_ENABLED"`
	ImportListSyncInterval time.Duration `env:"LIBRARRY_IMPORT_LIST_SYNC_INTERVAL"`
	BackupEnabled          bool          `env:"LIBRARRY_BACKUP_ENABLED"`
	BackupInterval         time.Duration `env:"LIBRARRY_BACKUP_INTERVAL"`
	BackupRetention        int           `env:"LIBRARRY_BACKUP_RETENTION"`
	BackupDir              string        `env:"LIBRARRY_BACKUP_DIR"`
	WebOrigin              string        `env:"LIBRARRY_WEB_ORIGIN"`
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
		ImportListSyncEnabled:     envBool("LIBRARRY_IMPORT_LIST_SYNC_ENABLED", true),
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
