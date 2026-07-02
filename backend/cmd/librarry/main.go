package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/api"
	"github.com/bandoracer/librarry/backend/internal/auth"
	"github.com/bandoracer/librarry/backend/internal/backups"
	"github.com/bandoracer/librarry/backend/internal/calibre"
	compatstore "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/database"
	"github.com/bandoracer/librarry/backend/internal/importlists"
	"github.com/bandoracer/librarry/backend/internal/integrationsettings"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/notify"
	"github.com/bandoracer/librarry/backend/internal/scheduler"
	"github.com/bandoracer/librarry/backend/internal/tags"
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
	var notifyStore *notify.Store
	var authStore *auth.Store
	var tagsStore *tags.Store
	var importListStore *importlists.Store

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
		notifyStore = notify.NewStore(db)
		authStore = auth.NewStore(db)
		tagsStore = tags.NewStore(db)
		importListStore = importlists.NewStore(db)
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
		WithReleaseRestrictionProvider(compatstore.NewReleaseRestrictionProvider(compatStore)).
		WithDefaultSearchLanguage(cfg.StandardSearchLanguage)
	libraryConfig := library.Config{
		EbookRoot:                  cfg.EbookLibraryRoot,
		AudiobookRoot:              cfg.AudiobookLibraryRoot,
		NamingAuthorFolderTemplate: cfg.NamingAuthorFolder,
		NamingBookFolderTemplate:   cfg.NamingBookFolder,
		NamingFileNameTemplate:     cfg.NamingFileName,
		NamingSpaceReplacement:     cfg.NamingSpaceReplacement,
		RenameBooks:                &cfg.RenameBooks,
		StandardSearchLanguage:     cfg.StandardSearchLanguage,
		RecycleBin:                 cfg.RecycleBin,
		RecycleBinRetention:        cfg.RecycleBinRetention,
		ImportExtraFiles:           cfg.ImportExtraFiles,
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
	if libraryService.Available() {
		// Native root folders (when present) win over the env/compat roots.
		if err := libraryService.SyncConfigFromRootFolders(ctx); err != nil {
			logger.Warn("native root folders unavailable", "error", err)
		}
	}
	wantedService.SetDefaultSearchLanguage(libraryConfig.StandardSearchLanguage)
	notifier := notify.NewService(notifyStore, logger)

	// Auth (M6.2): an explicit LIBRARRY_AUTH_METHOD wins at boot; otherwise a
	// UI-persisted method (compat resource auth-config) is restored.
	authService := auth.NewService(authStore, logger)
	authService.SetMethod(bootAuthMethod(ctx, logger, compatStore, cfg))
	if cfg.AuthUsername != "" && cfg.AuthPassword != "" && authService.Available() {
		if err := authService.EnsureUser(ctx, cfg.AuthUsername, cfg.AuthPassword); err != nil {
			logger.Warn("auth user seed failed", "error", err)
		}
	}
	if authService.Method() != auth.MethodNone {
		logger.Info("api authentication enabled", "method", authService.Method())
	}

	// Import lists (M6.3): Hardcover-native list sync.
	importListService := importlists.NewService(
		importListStore,
		wantedService,
		importlists.NewHardcoverClient(nil, cfg.HardcoverToken),
		logger,
	)

	// Backups (M6.6): pg_dump into LIBRARRY_BACKUP_DIR.
	backupService := backups.NewService(backups.Options{
		Dir:         cfg.BackupDir,
		DatabaseURL: cfg.DatabaseURL,
		Logger:      logger,
	})

	// Background workers register with the scheduler registry, which owns the
	// startup-timer/ticker loops and powers the System Tasks view plus manual
	// run-now triggers.
	registry := scheduler.NewRegistry(logger)
	registerTask := func(task scheduler.Task) {
		if err := registry.Register(task); err != nil {
			logger.Error("task registration failed", "task", task.ID, "error", err)
		}
	}
	if cfg.MonitorEnabled && wantedService.Available() {
		registerTask(wantedMonitorTask(logger, wantedService, notifier, cfg))
	}
	if cfg.AuthorMonitorEnabled && wantedService.Available() {
		registerTask(authorMonitorTask(logger, wantedService, cfg))
	}
	if cfg.FeedSyncEnabled && wantedService.Available() {
		registerTask(feedSyncTask(logger, wantedService, notifier, cfg))
	}
	if cfg.FailedDownloadEnabled && wantedService.Available() {
		registerTask(failedDownloadRecoveryTask(logger, wantedService, notifier, cfg))
	}
	if cfg.UpgradeSearchEnabled && wantedService.Available() {
		registerTask(upgradeSearchTask(logger, wantedService, notifier, cfg))
	}
	if cfg.CalibreRefreshEnabled && libraryService.Available() {
		registerTask(calibreConversionRefreshTask(logger, libraryService, cfg))
	}
	if cfg.CompletedImportEnabled && libraryService.Available() {
		registerTask(completedDownloadImportTask(logger, libraryService, acquire, notifier, cfg))
	}
	if importListService.Available() {
		registerTask(importListSyncTask(logger, importListService, cfg))
	}
	if cfg.BackupEnabled && backupService.Available() {
		registerTask(backupTask(logger, backupService, cfg))
	}

	deps := api.Dependencies{
		Logger:      logger,
		Config:      cfg,
		Metadata:    metadataService,
		Acquire:     acquire,
		Wanted:      wantedService,
		Library:     libraryService,
		Notify:      notifier,
		Scheduler:   registry,
		Auth:        authService,
		ImportLists: importListService,
		Tags:        tagsStore,
		Backups:     backupService,
	}
	if compatStore != nil {
		deps.Compat = compatStore
	}
	healthEvaluator := api.NewHealthEvaluator(deps)
	deps.Health = healthEvaluator
	registerTask(healthCheckTask(healthEvaluator))

	var monitorWG sync.WaitGroup
	registry.Start(ctx, &monitorWG)
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

func feedSyncTask(logger *slog.Logger, service *wanted.Service, notifier *notify.Service, cfg config.Config) scheduler.Task {
	interval := cfg.FeedSyncInterval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	logger.Info("feed sync enabled", "interval", interval, "auto_grab", cfg.FeedSyncAutoGrab)
	return scheduler.Task{
		ID:           "feed-sync",
		Name:         "Feed Sync",
		Interval:     interval,
		StartupDelay: 30 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
			outcome, err := service.FeedSync(runCtx, wanted.FeedSyncRequest{
				Trigger:  trigger,
				Limit:    cfg.FeedSyncLimit,
				AutoGrab: cfg.FeedSyncAutoGrab,
				// arr parity: automated grabs start immediately; blocklist backstops failures.
				Paused: false,
			})
			if err != nil {
				logger.Warn("feed sync failed", "trigger", trigger, "error", err)
				return "", err
			}
			notifier.DispatchAll(runCtx, notify.EventsFromFeedSyncRun("feed-sync", outcome))
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
			return fmt.Sprintf(
				"%d releases seen, %d matched, %d grabbed, %d errors",
				outcome.ReleasesSeen, outcome.MatchedCount, outcome.GrabbedCount, outcome.ErrorCount,
			), nil
		},
	}
}

func failedDownloadRecoveryTask(logger *slog.Logger, service *wanted.Service, notifier *notify.Service, cfg config.Config) scheduler.Task {
	interval := cfg.FailedDownloadInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	stalledMinutes := int(cfg.FailedDownloadStalledAge / time.Minute)
	if stalledMinutes <= 0 {
		stalledMinutes = int((24 * time.Hour) / time.Minute)
	}
	logger.Info("failed download recovery enabled", "interval", interval, "auto_grab", cfg.FailedDownloadAutoGrab, "remove_failed", cfg.FailedDownloadRemove)
	return scheduler.Task{
		ID:           "failed-download-recovery",
		Name:         "Failed Download Recovery",
		Interval:     interval,
		StartupDelay: 45 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
			outcome, err := service.RecoverFailedDownloads(runCtx, wanted.FailedDownloadRequest{
				Trigger:           trigger,
				Limit:             cfg.FailedDownloadLimit,
				SearchLimit:       20,
				MinStalledMinutes: stalledMinutes,
				AutoGrab:          cfg.FailedDownloadAutoGrab,
				Paused:            false,
				RemoveFailed:      cfg.FailedDownloadRemove,
				DeleteFailedFiles: cfg.FailedDownloadDeleteFiles,
			})
			if err != nil {
				logger.Warn("failed download recovery failed", "trigger", trigger, "error", err)
				return "", err
			}
			notifier.DispatchAll(runCtx, notify.EventsFromFailedDownloadRun("failed-download-recovery", outcome))
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
			return fmt.Sprintf(
				"%d checked, %d failed, %d replacements grabbed, %d removed, %d errors",
				outcome.DownloadsChecked, outcome.FailedCount, outcome.GrabbedCount, outcome.RemovedCount, outcome.ErrorCount,
			), nil
		},
	}
}

func upgradeSearchTask(logger *slog.Logger, service *wanted.Service, notifier *notify.Service, cfg config.Config) scheduler.Task {
	interval := cfg.UpgradeSearchInterval
	if interval <= 0 {
		interval = 12 * time.Hour
	}
	minSearchIntervalMinutes := int(interval / time.Minute)
	if minSearchIntervalMinutes <= 0 {
		minSearchIntervalMinutes = int((12 * time.Hour) / time.Minute)
	}
	logger.Info("upgrade search enabled", "interval", interval, "auto_grab", cfg.UpgradeSearchAutoGrab, "min_delta", cfg.UpgradeSearchMinDelta)
	return scheduler.Task{
		ID:           "upgrade-search",
		Name:         "Upgrade Search",
		Interval:     interval,
		StartupDelay: 60 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
			outcome, err := service.SearchUpgrades(runCtx, wanted.UpgradeRequest{
				Trigger:                  trigger,
				Limit:                    cfg.UpgradeSearchLimit,
				SearchLimit:              20,
				MinSearchIntervalMinutes: minSearchIntervalMinutes,
				MinScoreDelta:            cfg.UpgradeSearchMinDelta,
				AutoGrab:                 cfg.UpgradeSearchAutoGrab,
				Paused:                   false,
			})
			if err != nil {
				logger.Warn("upgrade search failed", "trigger", trigger, "error", err)
				return "", err
			}
			notifier.DispatchAll(runCtx, notify.EventsFromUpgradeRun("upgrade-search", outcome))
			logger.Info(
				"upgrade search completed",
				"trigger", trigger,
				"status", outcome.Status,
				"wanted_checked", outcome.WantedChecked,
				"upgrades", outcome.UpgradeCount,
				"grabbed", outcome.GrabbedCount,
				"errors", outcome.ErrorCount,
			)
			return fmt.Sprintf(
				"%d wanted checked, %d upgrades, %d grabbed, %d errors",
				outcome.WantedChecked, outcome.UpgradeCount, outcome.GrabbedCount, outcome.ErrorCount,
			), nil
		},
	}
}

func wantedMonitorTask(logger *slog.Logger, service *wanted.Service, notifier *notify.Service, cfg config.Config) scheduler.Task {
	interval := cfg.MonitorInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	searchIntervalMinutes := int(cfg.MonitorSearchInterval / time.Minute)
	if searchIntervalMinutes <= 0 {
		searchIntervalMinutes = int((6 * time.Hour) / time.Minute)
	}
	logger.Info("wanted monitor enabled", "interval", interval, "auto_grab", cfg.MonitorAutoGrab)
	return scheduler.Task{
		ID:           "wanted-monitor",
		Name:         "Wanted Monitor",
		Interval:     interval,
		StartupDelay: 15 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
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
				return "", err
			}
			notifier.DispatchAll(runCtx, notify.EventsFromMonitorRun("wanted-monitor", outcome))
			logger.Info(
				"wanted monitor run completed",
				"trigger", trigger,
				"status", outcome.Status,
				"wanted_checked", outcome.WantedChecked,
				"approved", outcome.ApprovedCount,
				"grabbed", outcome.GrabbedCount,
				"errors", outcome.ErrorCount,
			)
			return fmt.Sprintf(
				"%d wanted checked, %d approved, %d grabbed, %d errors",
				outcome.WantedChecked, outcome.ApprovedCount, outcome.GrabbedCount, outcome.ErrorCount,
			), nil
		},
	}
}

func authorMonitorTask(logger *slog.Logger, service *wanted.Service, cfg config.Config) scheduler.Task {
	interval := cfg.AuthorMonitorInterval
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	syncIntervalMinutes := int(cfg.AuthorMonitorSyncInterval / time.Minute)
	if syncIntervalMinutes <= 0 {
		syncIntervalMinutes = int((24 * time.Hour) / time.Minute)
	}
	logger.Info("author monitor enabled", "interval", interval)
	return scheduler.Task{
		ID:           "author-monitor",
		Name:         "Author Monitor",
		Interval:     interval,
		StartupDelay: 30 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
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
				return "", err
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
			return fmt.Sprintf(
				"%d authors checked, %d items found, %d wanted created, %d errors",
				outcome.AuthorsChecked, outcome.ItemsFound, outcome.WantedCreated, outcome.ErrorCount,
			), nil
		},
	}
}

type calibreConversionRefreshService interface {
	RefreshCalibreConversions(ctx context.Context, request library.CalibreConversionRefreshRequest) (library.CalibreConversionRefreshOutcome, error)
}

type completedDownloadImportService interface {
	ImportCompletedDownloads(ctx context.Context, downloads []acquisition.DownloadStatus, request library.CompletedImportRequest) (library.CompletedImportOutcome, error)
}

type completedDownloadLister interface {
	Downloads(ctx context.Context, query acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error)
}

type completedDownloadClient interface {
	completedDownloadLister
	DownloadAction(ctx context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error)
}

type recycleBinCleaner interface {
	CleanupRecycleBin(now time.Time) (int, error)
}

// completedDownloadImportTask is Librarry's take on arr "Completed Download
// Handling": finished librarry-tagged downloads are imported automatically —
// auto-matched to their wanted item, or queued for review when no match
// exists — without an operator pressing Import. A second phase mirrors the
// arr "Remove Completed" behavior: imported downloads whose client reports
// seeding has finished are deleted with their data (imports use
// hardlink-or-copy, so the library copy survives).
func completedDownloadImportTask(logger *slog.Logger, service completedDownloadImportService, downloads completedDownloadClient, notifier *notify.Service, cfg config.Config) scheduler.Task {
	interval := completedImportInterval(cfg)
	logger.Info("completed download import enabled", "interval", interval, "limit", completedImportLimit(cfg), "remove_after_seeding", cfg.CompletedRemoveEnabled)
	return scheduler.Task{
		ID:           "completed-import",
		Name:         "Completed Download Import",
		Interval:     interval,
		StartupDelay: 45 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			outcome, err := runCompletedDownloadImportOnce(ctx, service, downloads, cfg)
			if err != nil {
				logger.Warn("completed download import failed", "trigger", trigger, "error", err)
				return "", err
			}
			notifier.DispatchAll(ctx, notify.EventsFromCompletedImports("completed-import", outcome))
			removed := 0
			if cfg.CompletedRemoveEnabled {
				removed, err = runCompletedDownloadRemovalOnce(ctx, downloads)
				if err != nil {
					logger.Warn("completed download removal failed", "trigger", trigger, "error", err)
				}
			}
			// Recycle-bin retention cleanup rides the same tick (no-op when
			// LIBRARRY_RECYCLE_BIN is unset).
			recycled := 0
			if cleaner, ok := service.(recycleBinCleaner); ok && strings.TrimSpace(cfg.RecycleBin) != "" {
				recycled, err = cleaner.CleanupRecycleBin(time.Now().UTC())
				if err != nil {
					logger.Warn("recycle bin cleanup failed", "trigger", trigger, "error", err)
				}
			}
			// Unresolved reviews re-count every tick (dedup happens in the store),
			// so only imports, removals, and errors get Info-level noise.
			level := slog.LevelDebug
			if outcome.Imported > 0 || outcome.Errored > 0 || removed > 0 || recycled > 0 {
				level = slog.LevelInfo
			}
			logger.Log(
				ctx,
				level,
				"completed download import finished",
				"trigger", trigger,
				"checked", outcome.Checked,
				"imported", outcome.Imported,
				"auto_matched", outcome.AutoMatched,
				"review_queued", outcome.ReviewQueued,
				"skipped", outcome.Skipped,
				"removed", removed,
				"recycle_bin_purged", recycled,
				"errors", outcome.Errored,
			)
			return fmt.Sprintf(
				"%d checked, %d imported, %d review queued, %d removed, %d errors",
				outcome.Checked, outcome.Imported, outcome.ReviewQueued, removed, outcome.Errored,
			), nil
		},
	}
}

func runCompletedDownloadImportOnce(ctx context.Context, service completedDownloadImportService, lister completedDownloadLister, cfg config.Config) (library.CompletedImportOutcome, error) {
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	rows, err := lister.Downloads(runCtx, acquisition.DownloadListQuery{Tag: "librarry"})
	if err != nil {
		return library.CompletedImportOutcome{}, err
	}
	return service.ImportCompletedDownloads(runCtx, rows, library.CompletedImportRequest{
		Limit:      completedImportLimit(cfg),
		ImportMode: cfg.CompletedImportMode,
	})
}

// runCompletedDownloadRemovalOnce deletes imported, seed-finished downloads
// (with their data) from the download client. It lists fresh state so the
// downloads imported earlier in the same tick are eligible immediately.
func runCompletedDownloadRemovalOnce(ctx context.Context, client completedDownloadClient) (int, error) {
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	rows, err := client.Downloads(runCtx, acquisition.DownloadListQuery{Tag: "librarry"})
	if err != nil {
		return 0, err
	}
	removed := 0
	var firstErr error
	for _, download := range rows {
		if !completedDownloadRemovalEligible(download) {
			continue
		}
		result, err := client.DownloadAction(runCtx, acquisition.DownloadActionRequest{
			Action:      acquisition.DownloadActionDelete,
			Client:      download.Client,
			IDs:         []string{download.ID},
			DeleteFiles: true,
		})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if result.Applied {
			removed++
		}
	}
	return removed, firstErr
}

// completedDownloadRemovalEligible reports whether an imported download has
// finished seeding and can be deleted with its data: qBittorrent parks
// finished torrents in stoppedUP (5.x) or pausedUP (4.x), and Transmission
// reports stopped-and-done torrents as completed.
func completedDownloadRemovalEligible(download acquisition.DownloadStatus) bool {
	if !strings.EqualFold(strings.TrimSpace(download.ImportStatus), "imported") {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(download.State))
	switch state {
	case "stoppedup", "pausedup":
		return true
	}
	if strings.EqualFold(strings.TrimSpace(download.Client), "transmission") {
		return (state == "completed" || state == "stopped") && download.Progress >= 1
	}
	return false
}

func completedImportInterval(cfg config.Config) time.Duration {
	if cfg.CompletedImportInterval <= 0 {
		return time.Minute
	}
	return cfg.CompletedImportInterval
}

func completedImportLimit(cfg config.Config) int {
	if cfg.CompletedImportLimit <= 0 {
		return 50
	}
	return cfg.CompletedImportLimit
}

func calibreConversionRefreshTask(logger *slog.Logger, service calibreConversionRefreshService, cfg config.Config) scheduler.Task {
	interval, _ := calibreConversionRefreshSchedule(cfg)
	logger.Info("calibre conversion refresh enabled", "interval", interval, "limit", calibreConversionRefreshLimit(cfg), "max_attempts", calibreConversionRefreshMaxAttempts(cfg))
	return scheduler.Task{
		ID:           "calibre-refresh",
		Name:         "Calibre Conversion Refresh",
		Interval:     interval,
		StartupDelay: 75 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			outcome, err := runCalibreConversionRefreshOnce(ctx, logger, service, cfg, trigger)
			if err != nil {
				logger.Warn("calibre conversion refresh failed", "trigger", trigger, "error", err)
				return "", err
			}
			logger.Info(
				"calibre conversion refresh completed",
				"trigger", trigger,
				"checked", outcome.Checked,
				"refreshed", outcome.Refreshed,
				"skipped", outcome.Skipped,
				"errors", outcome.Errored,
			)
			return fmt.Sprintf(
				"%d checked, %d refreshed, %d skipped, %d errors",
				outcome.Checked, outcome.Refreshed, outcome.Skipped, outcome.Errored,
			), nil
		},
	}
}

// bootAuthMethod resolves the auth method at startup: explicit env wins, then
// the UI-persisted compat resource, then none.
func bootAuthMethod(ctx context.Context, logger *slog.Logger, compatStore *compatstore.Store, cfg config.Config) string {
	if method := auth.NormalizeMethod(cfg.AuthMethod); method != "" {
		return method
	}
	if cfg.AuthMethod != "" {
		logger.Warn("unrecognized LIBRARRY_AUTH_METHOD; falling back to none", "value", cfg.AuthMethod)
	}
	if compatStore != nil {
		if resource, ok, err := compatStore.GetResource(ctx, "auth-config", 1); err != nil {
			logger.Warn("persisted auth method unavailable", "error", err)
		} else if ok {
			if raw, ok := resource.Payload["method"].(string); ok {
				if method := auth.NormalizeMethod(strings.ToLower(strings.TrimSpace(raw))); method != "" {
					return method
				}
			}
		}
	}
	return auth.MethodNone
}

// importListSyncTask keeps enabled import lists in sync with their Hardcover
// lists/shelves. Manual per-list runs go through POST /api/v1/import-lists/{id}/sync.
func importListSyncTask(logger *slog.Logger, service *importlists.Service, cfg config.Config) scheduler.Task {
	interval := cfg.ImportListSyncInterval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	logger.Info("import list sync enabled", "interval", interval)
	return scheduler.Task{
		ID:           "import-list-sync",
		Name:         "Import List Sync",
		Interval:     interval,
		StartupDelay: 90 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
			outcome, err := service.Sync(runCtx, nil, trigger)
			if err != nil {
				logger.Warn("import list sync failed", "trigger", trigger, "error", err)
				return "", err
			}
			logger.Info(
				"import list sync completed",
				"trigger", trigger,
				"status", outcome.Status,
				"lists_checked", outcome.ListsChecked,
				"entries", outcome.EntriesFound,
				"wanted_created", outcome.WantedCreated,
				"errors", outcome.ErrorCount,
			)
			return fmt.Sprintf(
				"%d lists checked, %d entries, %d wanted created, %d errors",
				outcome.ListsChecked, outcome.EntriesFound, outcome.WantedCreated, outcome.ErrorCount,
			), nil
		},
	}
}

// backupTask runs a scheduled pg_dump and prunes to the retention count.
func backupTask(logger *slog.Logger, service *backups.Service, cfg config.Config) scheduler.Task {
	interval := cfg.BackupInterval
	if interval <= 0 {
		interval = 168 * time.Hour
	}
	retention := cfg.BackupRetention
	if retention <= 0 {
		retention = 4
	}
	logger.Info("scheduled backups enabled", "interval", interval, "retention", retention, "dir", cfg.BackupDir)
	return scheduler.Task{
		ID:           "backup",
		Name:         "Database Backup",
		Interval:     interval,
		StartupDelay: 2 * time.Minute,
		Run: func(ctx context.Context, trigger string) (string, error) {
			runCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
			defer cancel()
			backup, err := service.Create(runCtx)
			if err != nil {
				logger.Warn("scheduled backup failed", "trigger", trigger, "error", err)
				return "", err
			}
			pruned, pruneErr := service.Prune(retention)
			if pruneErr != nil {
				logger.Warn("backup retention prune failed", "trigger", trigger, "error", pruneErr)
			}
			return fmt.Sprintf("%s created (%d bytes), %d pruned", backup.Name, backup.SizeBytes, pruned), nil
		},
	}
}

// healthCheckTask reruns the health rules every 5 minutes; ok-to-bad
// transitions dispatch healthIssue notifications from inside the evaluator.
func healthCheckTask(evaluator *api.HealthEvaluator) scheduler.Task {
	return scheduler.Task{
		ID:           "health-check",
		Name:         "Health Check",
		Interval:     5 * time.Minute,
		StartupDelay: 20 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			checks := evaluator.Evaluate(ctx)
			warnings := 0
			errored := 0
			for _, check := range checks {
				switch check.Severity {
				case "warning":
					warnings++
				case "error":
					errored++
				}
			}
			return fmt.Sprintf("%d checks, %d warnings, %d errors", len(checks), warnings, errored), nil
		},
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
