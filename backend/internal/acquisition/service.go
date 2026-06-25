package acquisition

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

var ErrIntegrationNotConfigured = errors.New("integration is not configured")

type IntegrationConfig struct {
	ProwlarrURL       string
	ProwlarrAPIKey    string
	QBittorrentURL    string
	QBittorrentUser   string
	QBittorrentPass   string
	EbookCategory     string
	AudiobookCategory string
	BookTorrentRoot   string
}

type IntegrationHealth struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

type BootstrapResult struct {
	Categories []string `json:"categories"`
	SavePath   string   `json:"savePath"`
}

type Service struct {
	config   IntegrationConfig
	prowlarr *ProwlarrClient
	qbit     *QBittorrentClient
}

func NewService(config IntegrationConfig) *Service {
	longClient := &http.Client{Timeout: 90 * time.Second}
	shortClient := &http.Client{Timeout: 30 * time.Second}
	return &Service{
		config: config,
		prowlarr: NewProwlarrClient(
			config.ProwlarrURL,
			config.ProwlarrAPIKey,
			longClient,
		),
		qbit: NewQBittorrentClient(
			config.QBittorrentURL,
			config.QBittorrentUser,
			config.QBittorrentPass,
			shortClient,
		),
	}
}

func (s *Service) Health(ctx context.Context) []IntegrationHealth {
	return []IntegrationHealth{
		s.prowlarr.Health(ctx),
		s.qbit.Health(ctx),
	}
}

func (s *Service) Bootstrap(ctx context.Context) (BootstrapResult, error) {
	savePath := s.bookTorrentRoot()
	categories := []string{s.ebookCategory(), s.audiobookCategory()}
	for _, category := range categories {
		if err := s.qbit.EnsureCategory(ctx, category, savePath); err != nil {
			return BootstrapResult{}, err
		}
	}
	return BootstrapResult{Categories: categories, SavePath: savePath}, nil
}

func (s *Service) Search(ctx context.Context, query ReleaseSearchQuery) ([]Release, error) {
	return s.prowlarr.Search(ctx, query)
}

func (s *Service) Grab(ctx context.Context, request DownloadRequest) (DownloadStatus, error) {
	if request.Category == "" {
		request.Category = s.categoryForFormat("")
	}
	if request.SavePath == "" {
		request.SavePath = s.bookTorrentRoot()
	}
	if len(request.Tags) == 0 {
		request.Tags = []string{"librarry"}
	}
	return s.qbit.Add(ctx, request)
}

func (s *Service) Downloads(ctx context.Context, tag string) ([]DownloadStatus, error) {
	return s.qbit.List(ctx, tag)
}

func (s *Service) CategoryForFormat(format string) string {
	return s.categoryForFormat(format)
}

func (s *Service) TorrentRoot() string {
	return s.bookTorrentRoot()
}

func (s *Service) categoryForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "audiobook", "audio":
		return s.audiobookCategory()
	default:
		return s.ebookCategory()
	}
}

func (s *Service) ebookCategory() string {
	if strings.TrimSpace(s.config.EbookCategory) != "" {
		return strings.TrimSpace(s.config.EbookCategory)
	}
	return CategoryBooksEbook
}

func (s *Service) audiobookCategory() string {
	if strings.TrimSpace(s.config.AudiobookCategory) != "" {
		return strings.TrimSpace(s.config.AudiobookCategory)
	}
	return CategoryBooksAudiobook
}

func (s *Service) bookTorrentRoot() string {
	if strings.TrimSpace(s.config.BookTorrentRoot) != "" {
		return strings.TrimSpace(s.config.BookTorrentRoot)
	}
	return DefaultTorrentRoot
}
