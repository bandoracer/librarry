package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/api"
	"github.com/bandoracer/librarry/backend/internal/calibre"
	compatstore "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/database"
	"github.com/bandoracer/librarry/backend/internal/integrationsettings"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.FromEnv()
	ctx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	var downloadStore acquisition.DownloadStore
	var wantedStore *wanted.Store
	var libraryStore *library.Store
	var compatStore *compatstore.Store

	if cfg.DatabaseURL != "" {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("database unavailable", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		if err := database.ApplyMigrations(ctx, db, cfg.MigrationsDir); err != nil {
			logger.Error("database migrations failed", "error", err)
			os.Exit(1)
		}
		downloadStore = acquisition.NewSQLDownloadStore(db)
		wantedStore = wanted.NewStore(db)
		libraryStore = library.NewStore(db)
		compatStore = compatstore.NewStore(db)
		logger.Info("database migrations applied")
	} else {
		logger.Warn("LIBRARRY_DATABASE_URL is not set; starting without database-backed persistence")
	}

	providers := metadata.DefaultProviders(metadata.ProviderConfig{
		HardcoverToken: cfg.HardcoverToken,
		GoogleAPIKey:   cfg.GoogleBooksAPIKey,
		HTTPTimeout:    12 * time.Second,
	})
	metadataService := metadata.NewService(providers)
	integrationConfig := acquisition.IntegrationConfig{
		ProwlarrURL:       cfg.ProwlarrURL,
		ProwlarrAPIKey:    cfg.ProwlarrAPIKey,
		QBittorrentURL:    cfg.QBittorrentURL,
		QBittorrentUser:   cfg.QBittorrentUser,
		QBittorrentPass:   cfg.QBittorrentPass,
		TransmissionURL:   cfg.TransmissionURL,
		TransmissionUser:  cfg.TransmissionUser,
		TransmissionPass:  cfg.TransmissionPass,
		SABnzbdURL:        cfg.SABnzbdURL,
		SABnzbdAPIKey:     cfg.SABnzbdAPIKey,
		SABnzbdUser:       cfg.SABnzbdUser,
		SABnzbdPass:       cfg.SABnzbdPass,
		EbookCategory:     cfg.EbookCategory,
		AudiobookCategory: cfg.AudiobookCategory,
		BookTorrentRoot:   cfg.BookTorrentRoot,
		DownloadStore:     downloadStore,
	}
	if compatStore != nil {
		var err error
		integrationConfig, err = integrationsettings.FromResources(ctx, compatStore, integrationConfig)
		if err != nil {
			logger.Warn("persisted integration settings unavailable", "error", err)
		}
	}
	acquire := acquisition.NewService(integrationConfig)
	if integrationConfig.QBittorrentURL != "" {
		bootstrapCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		if result, err := acquire.Bootstrap(bootstrapCtx); err != nil {
			logger.Warn("qBittorrent category bootstrap failed", "error", err)
		} else {
			logger.Info("qBittorrent categories ready", "categories", result.Categories, "save_path", result.SavePath)
		}
		cancel()
	}
	wantedService := wanted.NewService(wantedStore, acquire, metadataService).
		WithReleaseRestrictionProvider(compatstore.NewReleaseRestrictionProvider(compatStore))
	libraryConfig := library.Config{
		EbookRoot:                  cfg.EbookLibraryRoot,
		AudiobookRoot:              cfg.AudiobookLibraryRoot,
		NamingAuthorFolderTemplate: cfg.NamingAuthorFolder,
		NamingBookFolderTemplate:   cfg.NamingBookFolder,
		NamingFileNameTemplate:     cfg.NamingFileName,
		NamingSpaceReplacement:     cfg.NamingSpaceReplacement,
	}
	if compatStore != nil {
		roots, err := compatStore.ListRootFolders(ctx)
		if err != nil {
			logger.Warn("persisted library roots unavailable", "error", err)
		} else {
			libraryConfig = library.ConfigWithRootFolders(libraryConfig, roots)
		}
		if resource, ok, err := compatStore.GetResource(ctx, "config-naming", 1); err != nil {
			logger.Warn("persisted library naming unavailable", "error", err)
		} else if ok {
			libraryConfig = library.ConfigWithNamingRecord(libraryConfig, resource.Payload)
		}
	}
	var rootFolders library.RootFolderProvider
	if compatStore != nil {
		rootFolders = compatStore
	}
	libraryService := library.NewService(libraryStore, libraryConfig, wantedStore, downloadStore).WithCalibre(calibre.NewClient(nil), rootFolders)
	var monitorWG sync.WaitGroup
	if cfg.MonitorEnabled && wantedService.Available() {
		monitorWG.Add(1)
		go runWantedMonitor(ctx, &monitorWG, logger, wantedService, cfg)
	}
	if cfg.AuthorMonitorEnabled && wantedService.Available() {
		monitorWG.Add(1)
		go runAuthorMonitor(ctx, &monitorWG, logger, wantedService, cfg)
	}
	if cfg.FeedSyncEnabled && wantedService.Available() {
		monitorWG.Add(1)
		go runWantedFeedSync(ctx, &monitorWG, logger, wantedService, cfg)
	}
	if cfg.FailedDownloadEnabled && wantedService.Available() {
		monitorWG.Add(1)
		go runFailedDownloadRecovery(ctx, &monitorWG, logger, wantedService, cfg)
	}
	if cfg.UpgradeSearchEnabled && wantedService.Available() {
		monitorWG.Add(1)
		go runUpgradeSearch(ctx, &monitorWG, logger, wantedService, cfg)
	}
	if cfg.CalibreRefreshEnabled && libraryService.Available() {
		monitorWG.Add(1)
		go runCalibreConversionRefresh(ctx, &monitorWG, logger, libraryService, cfg)
	}

	deps := api.Dependencies{
		Logger:   logger,
		Config:   cfg,
		Metadata: metadataService,
		Acquire:  acquire,
		Wanted:   wantedService,
		Library:  libraryService,
	}
	if compatStore != nil {
		deps.Compat = compatStore
	}
	router := api.NewRouter(deps)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("librarry api listening", "addr", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	cancelApp()
	monitorWG.Wait()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("api server shutdown failed", "error", err)
		os.Exit(1)
	}
}

func runWantedFeedSync(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, service *wanted.Service, cfg config.Config) {
	defer wg.Done()
	interval := cfg.FeedSyncInterval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	run := func(trigger string) {
		runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		outcome, err := service.FeedSync(runCtx, wanted.FeedSyncRequest{
			Trigger:  trigger,
			Limit:    cfg.FeedSyncLimit,
			AutoGrab: cfg.FeedSyncAutoGrab,
			Paused:   true,
		})
		if err != nil {
			logger.Warn("feed sync failed", "trigger", trigger, "error", err)
			return
		}
		logger.Info(
			"feed sync completed",
			"trigger", trigger,
			"status", outcome.Status,
			"releases_seen", outcome.ReleasesSeen,
			"matched", outcome.MatchedCount,
			"approved", outcome.ApprovedCount,
			"grabbed", outcome.GrabbedCount,
			"errors", outcome.ErrorCount,
		)
	}

	startup := time.NewTimer(30 * time.Second)
	ticker := time.NewTicker(interval)
	defer startup.Stop()
	defer ticker.Stop()
	logger.Info("feed sync enabled", "interval", interval, "auto_grab", cfg.FeedSyncAutoGrab)

	for {
		select {
		case <-ctx.Done():
			return
		case <-startup.C:
			run("scheduled-startup")
		case <-ticker.C:
			run("scheduled")
		}
	}
}

func runFailedDownloadRecovery(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, service *wanted.Service, cfg config.Config) {
	defer wg.Done()
	interval := cfg.FailedDownloadInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	stalledMinutes := int(cfg.FailedDownloadStalledAge / time.Minute)
	if stalledMinutes <= 0 {
		stalledMinutes = int((24 * time.Hour) / time.Minute)
	}
	run := func(trigger string) {
		runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		outcome, err := service.RecoverFailedDownloads(runCtx, wanted.FailedDownloadRequest{
			Trigger:           trigger,
			Limit:             cfg.FailedDownloadLimit,
			SearchLimit:       20,
			MinStalledMinutes: stalledMinutes,
			AutoGrab:          cfg.FailedDownloadAutoGrab,
			Paused:            true,
			RemoveFailed:      cfg.FailedDownloadRemove,
			DeleteFailedFiles: cfg.FailedDownloadDeleteFiles,
		})
		if err != nil {
			logger.Warn("failed download recovery failed", "trigger", trigger, "error", err)
			return
		}
		logger.Info(
			"failed download recovery completed",
			"trigger", trigger,
			"status", outcome.Status,
			"checked", outcome.DownloadsChecked,
			"failed", outcome.FailedCount,
			"replacements", outcome.ReplacementsFound,
			"grabbed", outcome.GrabbedCount,
			"removed", outcome.RemovedCount,
			"errors", outcome.ErrorCount,
		)
	}

	startup := time.NewTimer(45 * time.Second)
	ticker := time.NewTicker(interval)
	defer startup.Stop()
	defer ticker.Stop()
	logger.Info("failed download recovery enabled", "interval", interval, "auto_grab", cfg.FailedDownloadAutoGrab, "remove_failed", cfg.FailedDownloadRemove)

	for {
		select {
		case <-ctx.Done():
			return
		case <-startup.C:
			run("scheduled-startup")
		case <-ticker.C:
			run("scheduled")
		}
	}
}

func runUpgradeSearch(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, service *wanted.Service, cfg config.Config) {
	defer wg.Done()
	interval := cfg.UpgradeSearchInterval
	if interval <= 0 {
		interval = 12 * time.Hour
	}
	minSearchIntervalMinutes := int(interval / time.Minute)
	if minSearchIntervalMinutes <= 0 {
		minSearchIntervalMinutes = int((12 * time.Hour) / time.Minute)
	}
	run := func(trigger string) {
		runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		outcome, err := service.SearchUpgrades(runCtx, wanted.UpgradeRequest{
			Trigger:                  trigger,
			Limit:                    cfg.UpgradeSearchLimit,
			SearchLimit:              20,
			MinSearchIntervalMinutes: minSearchIntervalMinutes,
			MinScoreDelta:            cfg.UpgradeSearchMinDelta,
			AutoGrab:                 cfg.UpgradeSearchAutoGrab,
			Paused:                   true,
		})
		if err != nil {
			logger.Warn("upgrade search failed", "trigger", trigger, "error", err)
			return
		}
		logger.Info(
			"upgrade search completed",
			"trigger", trigger,
			"status", outcome.Status,
			"wanted_checked", outcome.WantedChecked,
			"upgrades", outcome.UpgradeCount,
			"grabbed", outcome.GrabbedCount,
			"errors", outcome.ErrorCount,
		)
	}

	startup := time.NewTimer(60 * time.Second)
	ticker := time.NewTicker(interval)
	defer startup.Stop()
	defer ticker.Stop()
	logger.Info("upgrade search enabled", "interval", interval, "auto_grab", cfg.UpgradeSearchAutoGrab, "min_delta", cfg.UpgradeSearchMinDelta)

	for {
		select {
		case <-ctx.Done():
			return
		case <-startup.C:
			run("scheduled-startup")
		case <-ticker.C:
			run("scheduled")
		}
	}
}

func runWantedMonitor(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, service *wanted.Service, cfg config.Config) {
	defer wg.Done()
	interval := cfg.MonitorInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	searchIntervalMinutes := int(cfg.MonitorSearchInterval / time.Minute)
	if searchIntervalMinutes <= 0 {
		searchIntervalMinutes = int((6 * time.Hour) / time.Minute)
	}

	run := func(trigger string) {
		runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		outcome, err := service.Monitor(runCtx, wanted.MonitorRequest{
			Trigger:                  trigger,
			Limit:                    cfg.MonitorLimit,
			SearchLimit:              20,
			AutoGrab:                 cfg.MonitorAutoGrab,
			MinSearchIntervalMinutes: searchIntervalMinutes,
		})
		if err != nil {
			logger.Warn("wanted monitor run failed", "trigger", trigger, "error", err)
			return
		}
		logger.Info(
			"wanted monitor run completed",
			"trigger", trigger,
			"status", outcome.Status,
			"wanted_checked", outcome.WantedChecked,
			"approved", outcome.ApprovedCount,
			"grabbed", outcome.GrabbedCount,
			"errors", outcome.ErrorCount,
		)
	}

	startup := time.NewTimer(15 * time.Second)
	ticker := time.NewTicker(interval)
	defer startup.Stop()
	defer ticker.Stop()
	logger.Info("wanted monitor enabled", "interval", interval, "auto_grab", cfg.MonitorAutoGrab)

	for {
		select {
		case <-ctx.Done():
			return
		case <-startup.C:
			run("scheduled-startup")
		case <-ticker.C:
			run("scheduled")
		}
	}
}

func runAuthorMonitor(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, service *wanted.Service, cfg config.Config) {
	defer wg.Done()
	interval := cfg.AuthorMonitorInterval
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	syncIntervalMinutes := int(cfg.AuthorMonitorSyncInterval / time.Minute)
	if syncIntervalMinutes <= 0 {
		syncIntervalMinutes = int((24 * time.Hour) / time.Minute)
	}

	run := func(trigger string) {
		runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		outcome, err := service.MonitorAuthors(runCtx, wanted.AuthorMonitorRequest{
			Trigger:                trigger,
			Limit:                  cfg.AuthorMonitorLimit,
			SearchLimit:            20,
			MinSyncIntervalMinutes: syncIntervalMinutes,
		})
		if err != nil {
			logger.Warn("author monitor run failed", "trigger", trigger, "error", err)
			return
		}
		logger.Info(
			"author monitor run completed",
			"trigger", trigger,
			"status", outcome.Status,
			"authors_checked", outcome.AuthorsChecked,
			"items_found", outcome.ItemsFound,
			"wanted_created", outcome.WantedCreated,
			"errors", outcome.ErrorCount,
		)
	}

	startup := time.NewTimer(30 * time.Second)
	ticker := time.NewTicker(interval)
	defer startup.Stop()
	defer ticker.Stop()
	logger.Info("author monitor enabled", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-startup.C:
			run("scheduled-startup")
		case <-ticker.C:
			run("scheduled")
		}
	}
}

type calibreConversionRefreshService interface {
	RefreshCalibreConversions(ctx context.Context, request library.CalibreConversionRefreshRequest) (library.CalibreConversionRefreshOutcome, error)
}

func runCalibreConversionRefresh(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, service calibreConversionRefreshService, cfg config.Config) {
	defer wg.Done()
	interval, _ := calibreConversionRefreshSchedule(cfg)

	run := func(trigger string) {
		outcome, err := runCalibreConversionRefreshOnce(ctx, logger, service, cfg, trigger)
		if err != nil {
			logger.Warn("calibre conversion refresh failed", "trigger", trigger, "error", err)
			return
		}
		logger.Info(
			"calibre conversion refresh completed",
			"trigger", trigger,
			"checked", outcome.Checked,
			"refreshed", outcome.Refreshed,
			"skipped", outcome.Skipped,
			"errors", outcome.Errored,
		)
	}

	startup := time.NewTimer(75 * time.Second)
	ticker := time.NewTicker(interval)
	defer startup.Stop()
	defer ticker.Stop()
	logger.Info("calibre conversion refresh enabled", "interval", interval, "limit", calibreConversionRefreshLimit(cfg), "max_attempts", calibreConversionRefreshMaxAttempts(cfg))

	for {
		select {
		case <-ctx.Done():
			return
		case <-startup.C:
			run("scheduled-startup")
		case <-ticker.C:
			run("scheduled")
		}
	}
}

func runCalibreConversionRefreshOnce(ctx context.Context, logger *slog.Logger, service calibreConversionRefreshService, cfg config.Config, trigger string) (library.CalibreConversionRefreshOutcome, error) {
	if logger == nil {
		logger = slog.Default()
	}
	_, request := calibreConversionRefreshSchedule(cfg)
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	outcome, err := service.RefreshCalibreConversions(runCtx, request)
	if err != nil {
		return library.CalibreConversionRefreshOutcome{}, err
	}
	logger.Debug("calibre conversion refresh run completed", "trigger", trigger, "checked", outcome.Checked, "refreshed", outcome.Refreshed, "skipped", outcome.Skipped, "errors", outcome.Errored)
	return outcome, nil
}

func calibreConversionRefreshSchedule(cfg config.Config) (time.Duration, library.CalibreConversionRefreshRequest) {
	interval := cfg.CalibreRefreshInterval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return interval, library.CalibreConversionRefreshRequest{
		Limit:       calibreConversionRefreshLimit(cfg),
		MaxAttempts: calibreConversionRefreshMaxAttempts(cfg),
	}
}

func calibreConversionRefreshLimit(cfg config.Config) int {
	if cfg.CalibreRefreshLimit <= 0 {
		return 200
	}
	if cfg.CalibreRefreshLimit > 500 {
		return 500
	}
	return cfg.CalibreRefreshLimit
}

func calibreConversionRefreshMaxAttempts(cfg config.Config) int {
	if cfg.CalibreRefreshMaxAttempts <= 0 {
		return 1
	}
	if cfg.CalibreRefreshMaxAttempts > 10 {
		return 10
	}
	return cfg.CalibreRefreshMaxAttempts
}
