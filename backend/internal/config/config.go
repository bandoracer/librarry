package config

import (
	"os"
	"strings"
)

type Config struct {
	ListenAddr        string
	DatabaseURL       string
	MigrationsDir     string
	HardcoverToken    string
	GoogleBooksAPIKey string
	ProwlarrURL       string
	ProwlarrAPIKey    string
	QBittorrentURL    string
	QBittorrentUser   string
	QBittorrentPass   string
	EbookCategory     string
	AudiobookCategory string
	BookTorrentRoot   string
	WebOrigin         string
}

func FromEnv() Config {
	return Config{
		ListenAddr:        env("LIBRARRY_LISTEN_ADDR", ":8080"),
		DatabaseURL:       strings.TrimSpace(os.Getenv("LIBRARRY_DATABASE_URL")),
		MigrationsDir:     env("LIBRARRY_MIGRATIONS_DIR", "backend/migrations"),
		HardcoverToken:    strings.TrimSpace(os.Getenv("LIBRARRY_HARDCOVER_TOKEN")),
		GoogleBooksAPIKey: strings.TrimSpace(os.Getenv("LIBRARRY_GOOGLE_BOOKS_API_KEY")),
		ProwlarrURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("LIBRARRY_PROWLARR_URL")), "/"),
		ProwlarrAPIKey:    strings.TrimSpace(os.Getenv("LIBRARRY_PROWLARR_API_KEY")),
		QBittorrentURL:    strings.TrimRight(strings.TrimSpace(os.Getenv("LIBRARRY_QBITTORRENT_URL")), "/"),
		QBittorrentUser:   strings.TrimSpace(os.Getenv("LIBRARRY_QBITTORRENT_USERNAME")),
		QBittorrentPass:   strings.TrimSpace(os.Getenv("LIBRARRY_QBITTORRENT_PASSWORD")),
		EbookCategory:     env("LIBRARRY_EBOOK_CATEGORY", "books-ebook"),
		AudiobookCategory: env("LIBRARRY_AUDIOBOOK_CATEGORY", "books-audiobook"),
		BookTorrentRoot:   env("LIBRARRY_BOOK_TORRENT_ROOT", "/data/torrents/books"),
		WebOrigin:         env("LIBRARRY_WEB_ORIGIN", "http://127.0.0.1:5173"),
	}
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
