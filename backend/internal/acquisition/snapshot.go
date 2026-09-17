package acquisition

import (
	"context"
	"sync"
	"sync/atomic"
)

// Service swaps immutable generations. An in-flight operation retains the
// clients and persistence handle it started with, including nested operations.
type Service struct {
	current     atomic.Pointer[integrationState]
	reconfigure sync.Mutex
}

func NewService(config IntegrationConfig) *Service {
	s := &Service{}
	s.current.Store(newIntegrationState(config))
	return s
}

func (s *Service) Reconfigure(config IntegrationConfig) {
	if s == nil {
		return
	}
	s.reconfigure.Lock()
	defer s.reconfigure.Unlock()
	if config.DownloadStore == nil {
		config.DownloadStore = s.current.Load().store
	}
	s.current.Store(newIntegrationState(config))
}

func (s *Service) IntegrationConfig() IntegrationConfig {
	if s == nil {
		return IntegrationConfig{}
	}
	return s.current.Load().IntegrationConfig()
}

func (s *Service) Health(ctx context.Context) []IntegrationHealth {
	return s.current.Load().Health(ctx)
}

func (s *Service) Bootstrap(ctx context.Context) (BootstrapResult, error) {
	return s.current.Load().Bootstrap(ctx)
}

func (s *Service) Search(ctx context.Context, query ReleaseSearchQuery) ([]Release, error) {
	return s.current.Load().Search(ctx, query)
}

func (s *Service) Feed(ctx context.Context, query ReleaseFeedQuery) ([]Release, error) {
	return s.current.Load().Feed(ctx, query)
}

func (s *Service) Grab(ctx context.Context, request DownloadRequest) (DownloadStatus, error) {
	return s.current.Load().Grab(ctx, request)
}

func (s *Service) Downloads(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error) {
	return s.current.Load().Downloads(ctx, query)
}

func (s *Service) DownloadDetails(ctx context.Context, id string, client string) (DownloadDetails, error) {
	return s.current.Load().DownloadDetails(ctx, id, client)
}

func (s *Service) DownloadAction(ctx context.Context, request DownloadActionRequest) (DownloadActionResult, error) {
	return s.current.Load().DownloadAction(ctx, request)
}

func (s *Service) DownloadFileAction(ctx context.Context, id string, request DownloadFileActionRequest) (DownloadFileActionResult, error) {
	return s.current.Load().DownloadFileAction(ctx, id, request)
}

func (s *Service) DownloadTrackerAction(ctx context.Context, id string, request DownloadTrackerActionRequest) (DownloadTrackerActionResult, error) {
	return s.current.Load().DownloadTrackerAction(ctx, id, request)
}

func (s *Service) DownloadResources(ctx context.Context, client string) (DownloadResources, error) {
	return s.current.Load().DownloadResources(ctx, client)
}

func (s *Service) DownloadPreferences(ctx context.Context, client string) (DownloadPreferences, error) {
	return s.current.Load().DownloadPreferences(ctx, client)
}

func (s *Service) UpdateDownloadPreferences(ctx context.Context, request DownloadPreferencesUpdate) (DownloadPreferences, error) {
	return s.current.Load().UpdateDownloadPreferences(ctx, request)
}

func (s *Service) DownloadCategoryAction(ctx context.Context, request DownloadCategoryActionRequest) (DownloadResourceActionResult, error) {
	return s.current.Load().DownloadCategoryAction(ctx, request)
}

func (s *Service) DownloadTagAction(ctx context.Context, request DownloadTagActionRequest) (DownloadResourceActionResult, error) {
	return s.current.Load().DownloadTagAction(ctx, request)
}

func (s *Service) MarkDownloadFailed(ctx context.Context, id string, reason string) error {
	return s.current.Load().MarkDownloadFailed(ctx, id, reason)
}

func (s *Service) ClearDownloadFailure(ctx context.Context, id string) error {
	return s.current.Load().ClearDownloadFailure(ctx, id)
}

func (s *Service) MarkDownloadReplacement(ctx context.Context, id string, replacementID string) error {
	return s.current.Load().MarkDownloadReplacement(ctx, id, replacementID)
}

func (s *Service) CategoryForFormat(format string) string {
	return s.current.Load().CategoryForFormat(format)
}

func (s *Service) TorrentRoot() string {
	return s.current.Load().TorrentRoot()
}
