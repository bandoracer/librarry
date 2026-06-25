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
	DownloadStore     DownloadStore
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
	store    DownloadStore
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
		store: config.DownloadStore,
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
	status, err := s.qbit.Add(ctx, request)
	if err != nil {
		return DownloadStatus{}, err
	}
	_ = s.storeDownloads(ctx, []DownloadStatus{status})
	return status, nil
}

func (s *Service) Downloads(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error) {
	statuses, err := s.qbit.List(ctx, query)
	if err == nil {
		_ = s.storeDownloads(ctx, statuses)
		statuses = s.mergeStoredDownloadState(ctx, statuses, query)
		return statuses, nil
	}
	if s.store != nil {
		stored, storeErr := s.store.ListDownloads(ctx, query)
		if storeErr == nil && len(stored) > 0 {
			return stored, nil
		}
	}
	return nil, err
}

func (s *Service) DownloadAction(ctx context.Context, request DownloadActionRequest) (DownloadActionResult, error) {
	result, err := s.qbit.Action(ctx, request)
	if err != nil {
		return DownloadActionResult{}, err
	}
	if normalizeAction(request.Action) == DownloadActionDelete {
		if s.store != nil {
			_ = s.store.MarkDownloadsDeleted(ctx, result.IDs)
		}
	} else {
		statuses, listErr := s.qbit.List(ctx, DownloadListQuery{IDs: result.IDs})
		if listErr == nil {
			_ = s.storeDownloads(ctx, statuses)
			result.Downloads = statuses
		}
	}
	return result, nil
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

func (s *Service) storeDownloads(ctx context.Context, downloads []DownloadStatus) error {
	if s.store == nil {
		return nil
	}
	return s.store.UpsertDownloads(ctx, downloads)
}

func (s *Service) mergeStoredDownloadState(ctx context.Context, downloads []DownloadStatus, query DownloadListQuery) []DownloadStatus {
	if s.store == nil || len(downloads) == 0 {
		return downloads
	}
	ids := make([]string, 0, len(downloads))
	for _, download := range downloads {
		if strings.TrimSpace(download.ID) != "" {
			ids = append(ids, download.ID)
		}
	}
	stored, err := s.store.ListDownloads(ctx, DownloadListQuery{
		IDs:      ids,
		Tag:      query.Tag,
		Category: query.Category,
	})
	if err != nil || len(stored) == 0 {
		return downloads
	}
	byID := make(map[string]DownloadStatus, len(stored))
	for _, item := range stored {
		byID[item.ID] = item
	}
	for i := range downloads {
		if item, ok := byID[downloads[i].ID]; ok {
			downloads[i].ImportStatus = item.ImportStatus
			downloads[i].ImportedFileID = item.ImportedFileID
			downloads[i].ImportedAt = item.ImportedAt
			downloads[i].ImportError = item.ImportError
		}
	}
	return downloads
}
