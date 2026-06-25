package acquisition

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

var ErrIntegrationNotConfigured = errors.New("integration is not configured")
var ErrDownloadNotFound = errors.New("download not found")
var ErrDownloadDetailsUnsupported = errors.New("download details are only supported for qBittorrent downloads")

type IntegrationConfig struct {
	ProwlarrURL       string
	ProwlarrAPIKey    string
	QBittorrentURL    string
	QBittorrentUser   string
	QBittorrentPass   string
	SABnzbdURL        string
	SABnzbdAPIKey     string
	SABnzbdUser       string
	SABnzbdPass       string
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
	sab      *SABnzbdClient
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
		sab: NewSABnzbdClient(
			config.SABnzbdURL,
			config.SABnzbdAPIKey,
			config.SABnzbdUser,
			config.SABnzbdPass,
			shortClient,
		),
		store: config.DownloadStore,
	}
}

func (s *Service) Health(ctx context.Context) []IntegrationHealth {
	return []IntegrationHealth{
		s.prowlarr.Health(ctx),
		s.qbit.Health(ctx),
		s.sab.Health(ctx),
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

func (s *Service) Feed(ctx context.Context, query ReleaseFeedQuery) ([]Release, error) {
	return s.prowlarr.Feed(ctx, query)
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
	client := s.downloadClientForRequest(request)
	status, err := client.Add(ctx, request)
	if err != nil {
		return DownloadStatus{}, err
	}
	_ = s.storeDownloads(ctx, []DownloadStatus{status})
	return status, nil
}

func (s *Service) Downloads(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error) {
	var statuses []DownloadStatus
	var firstErr error

	if s.includeClient(query, s.qbit.Name()) && s.qbit.Configured() {
		qbitStatuses, err := s.qbit.List(ctx, query)
		if err != nil {
			firstErr = err
		} else {
			statuses = append(statuses, qbitStatuses...)
		}
	}
	if s.includeClient(query, s.sab.Name()) && s.sab.Configured() {
		sabStatuses, err := s.sab.List(ctx, query)
		if err != nil && firstErr == nil {
			firstErr = err
		} else if err == nil {
			statuses = append(statuses, sabStatuses...)
		}
	}

	if len(statuses) > 0 {
		_ = s.storeDownloads(ctx, statuses)
		return s.mergeStoredDownloadState(ctx, statuses, query), nil
	}

	if s.store != nil {
		stored, storeErr := s.store.ListDownloads(ctx, query)
		if storeErr == nil && len(stored) > 0 {
			return stored, nil
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return []DownloadStatus{}, nil
}

func (s *Service) DownloadDetails(ctx context.Context, id string, client string) (DownloadDetails, error) {
	client = strings.TrimSpace(client)
	if client != "" && strings.EqualFold(client, s.sab.Name()) {
		return DownloadDetails{}, ErrDownloadDetailsUnsupported
	}
	if client != "" && !strings.EqualFold(client, s.qbit.Name()) {
		return DownloadDetails{}, ErrDownloadDetailsUnsupported
	}
	return s.qbit.Details(ctx, id)
}

func (s *Service) DownloadAction(ctx context.Context, request DownloadActionRequest) (DownloadActionResult, error) {
	ids := compactStrings(request.IDs)
	if len(ids) == 0 {
		return DownloadActionResult{}, errors.New("at least one download id is required")
	}

	qbitIDs, sabIDs := s.partitionDownloadIDs(ctx, ids)
	if len(qbitIDs) == 0 && len(sabIDs) == 0 {
		if s.sab.Configured() && !s.qbit.Configured() {
			sabIDs = ids
		} else {
			qbitIDs = ids
		}
	}

	action := normalizeAction(request.Action)
	if len(sabIDs) > 0 && !sabSupportsAction(action) {
		return DownloadActionResult{}, errors.New("SABnzbd supports only start, stop, and delete actions")
	}

	var appliedIDs []string
	var refreshed []DownloadStatus
	if len(qbitIDs) > 0 {
		qbitRequest := request
		qbitRequest.IDs = qbitIDs
		result, err := s.qbit.Action(ctx, qbitRequest)
		if err != nil {
			return DownloadActionResult{}, err
		}
		appliedIDs = append(appliedIDs, result.IDs...)
	}
	if len(sabIDs) > 0 {
		sabRequest := request
		sabRequest.IDs = sabIDs
		result, err := s.sab.Action(ctx, sabRequest)
		if err != nil {
			return DownloadActionResult{}, err
		}
		appliedIDs = append(appliedIDs, result.IDs...)
	}

	if action == DownloadActionDelete {
		if s.store != nil {
			_ = s.store.MarkDownloadsDeleted(ctx, appliedIDs)
		}
	} else {
		statuses, listErr := s.Downloads(ctx, DownloadListQuery{IDs: appliedIDs})
		if listErr == nil {
			refreshed = statuses
		}
	}
	return DownloadActionResult{Action: action, IDs: appliedIDs, Applied: true, Downloads: refreshed}, nil
}

func (s *Service) DownloadFileAction(ctx context.Context, id string, request DownloadFileActionRequest) (DownloadFileActionResult, error) {
	request.DownloadID = strings.TrimSpace(id)
	result, err := s.qbit.FileAction(ctx, request)
	if err != nil {
		return DownloadFileActionResult{}, err
	}
	details, detailErr := s.qbit.Details(ctx, id)
	if detailErr == nil {
		result.Download = &details
	}
	return result, nil
}

func (s *Service) DownloadTrackerAction(ctx context.Context, id string, request DownloadTrackerActionRequest) (DownloadTrackerActionResult, error) {
	result, err := s.qbit.TrackerAction(ctx, id, request)
	if err != nil {
		return DownloadTrackerActionResult{}, err
	}
	details, detailErr := s.qbit.Details(ctx, id)
	if detailErr == nil {
		result.Download = &details
	}
	return result, nil
}

func (s *Service) MarkDownloadFailed(ctx context.Context, id string, reason string) error {
	if s.store == nil {
		return nil
	}
	return s.store.MarkDownloadFailed(ctx, id, reason)
}

func (s *Service) MarkDownloadReplacement(ctx context.Context, id string, replacementID string) error {
	if s.store == nil {
		return nil
	}
	return s.store.MarkDownloadReplacement(ctx, id, replacementID)
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

func (s *Service) downloadClientForRequest(request DownloadRequest) downloadClient {
	if s.shouldUseSAB(request) {
		return s.sab
	}
	return s.qbit
}

func (s *Service) shouldUseSAB(request DownloadRequest) bool {
	protocol := strings.ToLower(strings.TrimSpace(request.Protocol))
	switch protocol {
	case "usenet", "nzb", "newznab":
		return true
	case "torrent", "torznab":
		return false
	}
	urlText := strings.ToLower(strings.TrimSpace(request.ReleaseURL))
	if strings.Contains(urlText, ".nzb") || strings.Contains(urlText, "t=get") && strings.Contains(urlText, "apikey=") {
		return true
	}
	return s.sab.Configured() && !s.qbit.Configured()
}

func (s *Service) includeClient(query DownloadListQuery, client string) bool {
	requested := strings.TrimSpace(query.Client)
	return requested == "" || strings.EqualFold(requested, client)
}

func (s *Service) partitionDownloadIDs(ctx context.Context, ids []string) ([]string, []string) {
	statuses, err := s.Downloads(ctx, DownloadListQuery{IDs: ids})
	if err != nil {
		return nil, nil
	}
	clientsByID := make(map[string]string, len(statuses))
	for _, status := range statuses {
		clientsByID[status.ID] = status.Client
	}
	var qbitIDs []string
	var sabIDs []string
	for _, id := range ids {
		client := clientsByID[id]
		switch {
		case strings.EqualFold(client, s.sab.Name()):
			sabIDs = append(sabIDs, id)
		case strings.EqualFold(client, s.qbit.Name()):
			qbitIDs = append(qbitIDs, id)
		}
	}
	return qbitIDs, sabIDs
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
		Client:   query.Client,
		Tag:      query.Tag,
		Category: query.Category,
	})
	if err != nil || len(stored) == 0 {
		return downloads
	}
	byID := make(map[string]DownloadStatus, len(stored))
	for _, item := range stored {
		byID[downloadStateKey(item.Client, item.ID)] = item
	}
	for i := range downloads {
		if item, ok := byID[downloadStateKey(downloads[i].Client, downloads[i].ID)]; ok {
			downloads[i].ImportStatus = item.ImportStatus
			downloads[i].ImportedFileID = item.ImportedFileID
			downloads[i].ImportedAt = item.ImportedAt
			downloads[i].ImportError = item.ImportError
			downloads[i].FailureReason = item.FailureReason
			downloads[i].FailedAt = item.FailedAt
			downloads[i].RetryCount = item.RetryCount
			downloads[i].ReplacementID = item.ReplacementID
		}
	}
	return downloads
}

type downloadClient interface {
	Add(ctx context.Context, request DownloadRequest) (DownloadStatus, error)
}

func downloadStateKey(client string, id string) string {
	return strings.ToLower(strings.TrimSpace(client)) + ":" + strings.TrimSpace(id)
}

func sabSupportsAction(action string) bool {
	switch action {
	case DownloadActionStart, DownloadActionStop, DownloadActionDelete:
		return true
	default:
		return false
	}
}
