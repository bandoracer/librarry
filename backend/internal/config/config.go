package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr            string
	DatabaseURL           string
	MigrationsDir         string
	HardcoverToken        string
	GoogleBooksAPIKey     string
	ProwlarrURL           string
	ProwlarrAPIKey        string
	QBittorrentURL        string
	QBittorrentUser       string
	QBittorrentPass       string
	EbookCategory         string
	AudiobookCategory     string
	BookTorrentRoot       string
	EbookLibraryRoot      string
	AudiobookLibraryRoot  string
	MonitorEnabled        bool
	MonitorInterval       time.Duration
	MonitorSearchInterval time.Duration
	MonitorLimit          int
	MonitorAutoGrab       bool
	FeedSyncEnabled       bool
	FeedSyncInterval      time.Duration
	FeedSyncLimit         int
	FeedSyncAutoGrab      bool
	WebOrigin             string
}

func FromEnv() Config {
	return Config{
		ListenAddr:            env("LIBRARRY_LISTEN_ADDR", ":8080"),
		DatabaseURL:           strings.TrimSpace(os.Getenv("LIBRARRY_DATABASE_URL")),
		MigrationsDir:         env("LIBRARRY_MIGRATIONS_DIR", "backend/migrations"),
		HardcoverToken:        strings.TrimSpace(os.Getenv("LIBRARRY_HARDCOVER_TOKEN")),
		GoogleBooksAPIKey:     strings.TrimSpace(os.Getenv("LIBRARRY_GOOGLE_BOOKS_API_KEY")),
		ProwlarrURL:           strings.TrimRight(strings.TrimSpace(os.Getenv("LIBRARRY_PROWLARR_URL")), "/"),
		ProwlarrAPIKey:        strings.TrimSpace(os.Getenv("LIBRARRY_PROWLARR_API_KEY")),
		QBittorrentURL:        strings.TrimRight(strings.TrimSpace(os.Getenv("LIBRARRY_QBITTORRENT_URL")), "/"),
		QBittorrentUser:       strings.TrimSpace(os.Getenv("LIBRARRY_QBITTORRENT_USERNAME")),
		QBittorrentPass:       strings.TrimSpace(os.Getenv("LIBRARRY_QBITTORRENT_PASSWORD")),
		EbookCategory:         env("LIBRARRY_EBOOK_CATEGORY", "books-ebook"),
		AudiobookCategory:     env("LIBRARRY_AUDIOBOOK_CATEGORY", "books-audiobook"),
		BookTorrentRoot:       env("LIBRARRY_BOOK_TORRENT_ROOT", "/data/torrents/books"),
		EbookLibraryRoot:      env("LIBRARRY_EBOOK_LIBRARY_ROOT", "/data/media/books/ebooks"),
		AudiobookLibraryRoot:  env("LIBRARRY_AUDIOBOOK_LIBRARY_ROOT", "/data/media/books/audiobooks"),
		MonitorEnabled:        envBool("LIBRARRY_MONITOR_ENABLED", true),
		MonitorInterval:       envDuration("LIBRARRY_MONITOR_INTERVAL", 30*time.Minute),
		MonitorSearchInterval: envDuration("LIBRARRY_MONITOR_SEARCH_INTERVAL", 6*time.Hour),
		MonitorLimit:          envInt("LIBRARRY_MONITOR_LIMIT", 50),
		MonitorAutoGrab:       envBool("LIBRARRY_MONITOR_AUTO_GRAB", false),
		FeedSyncEnabled:       envBool("LIBRARRY_FEED_SYNC_ENABLED", true),
		FeedSyncInterval:      envDuration("LIBRARRY_FEED_SYNC_INTERVAL", 15*time.Minute),
		FeedSyncLimit:         envInt("LIBRARRY_FEED_SYNC_LIMIT", 100),
		FeedSyncAutoGrab:      envBool("LIBRARRY_FEED_SYNC_AUTO_GRAB", false),
		WebOrigin:             env("LIBRARRY_WEB_ORIGIN", "http://127.0.0.1:5173"),
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
