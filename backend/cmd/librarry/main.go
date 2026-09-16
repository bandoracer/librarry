package main

import (
	"context"
	"database/sql"
	"errors"
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
	"github.com/bandoracer/librarry/backend/internal/providerhttp"
	"github.com/bandoracer/librarry/backend/internal/scheduler"
	"github.com/bandoracer/librarry/backend/internal/tags"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := config.ValidateEnvironment(); err != nil {
		logger.Error("invalid environment configuration", "error", err)
		os.Exit(1)
	}
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
	var schemaMigration string
	var schedulerDB *sql.DB

	if cfg.DatabaseURL != "" {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("database unavailable", "error", err)
			os.Exit(1)
		}
		defer db.Close()
		schedulerDB = db

		if err := database.ApplyMigrations(ctx, db, cfg.MigrationsDir); err != nil {
			logger.Error("database migrations failed", "error", err)
			os.Exit(1)
		}
		if err := db.QueryRowContext(ctx, "select coalesce(max(version), '') from schema_migrations").Scan(&schemaMigration); err != nil {
			logger.Error("read applied migration", "error", err)
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

	// Auth (M6.2): an explicit LIBRARRY_AUTH_METHOD wins at boot; otherwise a
	// UI-persisted method (compat resource auth-config) is restored.
	authService := auth.NewService(authStore, logger)
	method, err := bootAuthMethod(ctx, compatStore, cfg)
	if err != nil {
		logger.Error("authentication configuration failed", "error", err)
		os.Exit(1)
	}
	authService.SetMethod(method)
	if method != auth.MethodNone && !authService.Available() {
		logger.Error("configured authentication requires database persistence")
		os.Exit(1)
	}
	if cfg.AuthUsername != "" && cfg.AuthPassword != "" && authService.Available() {
		if err := authService.EnsureUser(ctx, cfg.AuthUsername, cfg.AuthPassword); err != nil {
			logger.Error("auth user seed failed", "error", err)
			os.Exit(1)
		}
	}
	if authService.Method() != auth.MethodNone && !authService.HasUser(ctx) {
		logger.Error("configured authentication requires a usable user; supply LIBRARRY_AUTH_USERNAME and LIBRARRY_AUTH_PASSWORD")
		os.Exit(1)
	}
	if authService.Method() != auth.MethodNone {
		logger.Info("api authentication enabled", "method", authService.Method())
	}

	providerClient := providerhttp.NewClient(12 * time.Second)
	providers := metadata.DefaultProviders(metadata.ProviderConfig{
		HTTPClient:     providerClient,
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
	libraryService := library.NewService(libraryStore, libraryConfig, wantedStore, downloadStore).WithCalibre(calibre.NewClient(nil), rootFolders).WithDownloadInspector(acquire)
	if libraryService.Available() {
		// Native root folders (when present) win over the env/compat roots.
		if err := libraryService.SyncConfigFromRootFolders(ctx); err != nil {
			logger.Warn("native root folders unavailable", "error", err)
		}
	}
	wantedService.SetDefaultSearchLanguage(libraryConfig.StandardSearchLanguage)
	notifier := notify.NewService(notifyStore, logger)

	// Import lists (M6.3): Hardcover-native list sync.
	importListService := importlists.NewService(
		importListStore,
		wantedService,
		importlists.NewHardcoverClient(providerClient, cfg.HardcoverToken),
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
	registry := scheduler.NewRegistry(logger).WithDatabase(schedulerDB)
	registerTask := func(task scheduler.Task) {
		if err := registry.Register(task); err != nil {
			logger.Error("task registration failed", "task", task.ID, "error", err)
		} else {
			logger.Info("background task registered", "task", task.ID, "enabled", task.DisabledReason == "", "available", task.UnavailableReason == "", "disabled_reason", task.DisabledReason, "unavailable_reason", task.UnavailableReason)
		}
	}
	registerTask(taskPolicy(scheduler.Task{ID: "history-maintenance", Name: "History Maintenance", Interval: time.Hour, StartupDelay: time.Minute, Run: func(runCtx context.Context, trigger string) (string, error) {
		report, err := notifier.PruneHistory(runCtx)
		reviewedRuns := 0
		if err == nil {
			reviewedRuns, err = registry.PruneReviewedHistory(runCtx)
		}
		scheduler.RecordRunDetails(runCtx, scheduler.RunDetails{Counts: map[string]int{"eventsCompacted": report.Events, "deliveriesPruned": report.Deliveries, "attemptsPruned": report.Attempts, "actionsPruned": report.Actions, "busySkipped": report.Skipped, "reviewedRunsPruned": reviewedRuns}, NextAction: "Review database availability and history maintenance logs."})
		return fmt.Sprintf("Compacted %d notification events, %d deliveries and %d reviewed worker runs", report.Events, report.Deliveries, reviewedRuns), err
	}}, true, notifier.Available(), "", "Database persistence is required."))
	registerTask(taskPolicy(scheduler.Task{ID: "notification-delivery", Name: "Notification Delivery", Interval: 15 * time.Second, StartupDelay: 3 * time.Second, Run: func(runCtx context.Context, trigger string) (string, error) {
		report, err := notifier.RunPendingDetailed(runCtx)
		scheduler.RecordRunDetails(runCtx, scheduler.RunDetails{Counts: map[string]int{"processed": report.Processed, "accepted": report.Accepted, "retry": report.Retry, "failed": report.Failed, "uncertain": report.Uncertain, "cancelled": report.Cancelled}, Errors: report.Failed + report.Uncertain, NextAction: "Review notification delivery in Settings Connect."})
		return fmt.Sprintf("Processed %d notification deliveries", report.Processed), err
	}}, true, notifier.Available(), "", "Database persistence is required."))
	registerTask(taskPolicy(scheduler.Task{ID: "library-scan", Name: "Library Scan Progress", Interval: 5 * time.Second, StartupDelay: 2 * time.Second, Run: func(runCtx context.Context, trigger string) (string, error) {
		err := libraryService.RunPendingScans(runCtx)
		return "Advanced pending library scans", err
	}}, true, libraryService.Available(), "", "Database persistence is required."))
	registerTask(taskPolicy(wantedMonitorTask(logger, wantedService, cfg), cfg.MonitorEnabled, wantedService.Available(), "LIBRARRY_MONITOR_ENABLED", "Database persistence is required."))
	registerTask(taskPolicy(authorMonitorTask(logger, wantedService, cfg), cfg.AuthorMonitorEnabled, wantedService.Available(), "LIBRARRY_AUTHOR_MONITOR_ENABLED", "Database persistence is required."))
	registerTask(taskPolicy(feedSyncTask(logger, wantedService, cfg), cfg.FeedSyncEnabled, wantedService.Available(), "LIBRARRY_FEED_SYNC_ENABLED", "Database persistence is required."))
	registerTask(taskPolicy(failedDownloadRecoveryTask(logger, wantedService, cfg), cfg.FailedDownloadEnabled, wantedService.Available(), "LIBRARRY_FAILED_DOWNLOAD_ENABLED", "Database persistence is required."))
	registerTask(taskPolicy(upgradeSearchTask(logger, wantedService, cfg), cfg.UpgradeSearchEnabled, wantedService.Available(), "LIBRARRY_UPGRADE_SEARCH_ENABLED", "Database persistence is required."))
	registerTask(taskPolicy(calibreConversionRefreshTask(logger, libraryService, cfg), cfg.CalibreRefreshEnabled, libraryService.Available(), "LIBRARRY_CALIBRE_REFRESH_ENABLED", "Database persistence is required."))
	registerTask(taskPolicy(completedDownloadImportTask(logger, libraryService, acquire, cfg), cfg.CompletedImportEnabled, libraryService.Available(), "LIBRARRY_COMPLETED_IMPORT_ENABLED", "Database persistence is required."))
	registerTask(taskPolicy(importListSyncTask(logger, importListService, cfg), cfg.ImportListSyncEnabled, importListService.Available(), "LIBRARRY_IMPORT_LIST_SYNC_ENABLED", "Database persistence is required."))
	registerTask(taskPolicy(backupTask(logger, backupService, cfg), cfg.BackupEnabled, backupService.Available(), "LIBRARRY_BACKUP_ENABLED", "Database persistence and a backup directory are required."))

	deps := api.Dependencies{
		SchemaMigration: schemaMigration,
		Logger:          logger,
		Config:          cfg,
		Metadata:        metadataService,
		Acquire:         acquire,
		Wanted:          wantedService,
		Library:         libraryService,
		Notify:          notifier,
		Scheduler:       registry,
		Auth:            authService,
		ImportLists:     importListService,
		Tags:            tagsStore,
		Backups:         backupService,
	}
	if compatStore != nil {
		deps.Compat = compatStore
	}
	healthEvaluator := api.NewHealthEvaluator(deps)
	deps.Health = healthEvaluator
	registerTask(healthCheckTask(healthEvaluator))

	router := api.NewRouter(deps)
	var monitorWG sync.WaitGroup
	registry.Start(ctx, &monitorWG)

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

func feedSyncTask(logger *slog.Logger, service *wanted.Service, cfg config.Config) scheduler.Task {
	interval := cfg.FeedSyncInterval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	logger.Debug("feed sync configuration", "interval", interval, "auto_grab", cfg.FeedSyncAutoGrab)
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
			scheduler.RecordRunDetails(runCtx, scheduler.RunDetails{Counts: map[string]int{"releasesSeen": outcome.ReleasesSeen, "matched": outcome.MatchedCount, "grabbed": outcome.GrabbedCount, "errors": outcome.ErrorCount}, Errors: outcome.ErrorCount, NextAction: "Review feed history and provider health.", OperationIDs: []string{outcome.ID}})
			if err != nil {
				logger.Warn("feed sync failed", "trigger", trigger, "error", err)
				return "", err
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
			return fmt.Sprintf(
				"%d releases seen, %d matched, %d grabbed, %d errors",
				outcome.ReleasesSeen, outcome.MatchedCount, outcome.GrabbedCount, outcome.ErrorCount,
			), nil
		},
	}
}

func failedDownloadRecoveryTask(logger *slog.Logger, service *wanted.Service, cfg config.Config) scheduler.Task {
	interval := cfg.FailedDownloadInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	stalledMinutes := int(cfg.FailedDownloadStalledAge / time.Minute)
	if stalledMinutes <= 0 {
		stalledMinutes = int((24 * time.Hour) / time.Minute)
	}
	logger.Debug("failed download recovery configuration", "interval", interval, "auto_grab", cfg.FailedDownloadAutoGrab, "remove_failed", cfg.FailedDownloadRemove)
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
			scheduler.RecordRunDetails(runCtx, scheduler.RunDetails{Counts: map[string]int{"checked": outcome.DownloadsChecked, "failed": outcome.FailedCount, "grabbed": outcome.GrabbedCount, "removed": outcome.RemovedCount, "errors": outcome.ErrorCount}, Errors: outcome.ErrorCount, NextAction: "Review Activity recovery and client health.", OperationIDs: []string{outcome.ID}})
			if err != nil {
				logger.Warn("failed download recovery failed", "trigger", trigger, "error", err)
				return "", err
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
			return fmt.Sprintf(
				"%d checked, %d failed, %d replacements grabbed, %d removed, %d errors",
				outcome.DownloadsChecked, outcome.FailedCount, outcome.GrabbedCount, outcome.RemovedCount, outcome.ErrorCount,
			), nil
		},
	}
}

func upgradeSearchTask(logger *slog.Logger, service *wanted.Service, cfg config.Config) scheduler.Task {
	interval := cfg.UpgradeSearchInterval
	if interval <= 0 {
		interval = 12 * time.Hour
	}
	minSearchIntervalMinutes := int(interval / time.Minute)
	if minSearchIntervalMinutes <= 0 {
		minSearchIntervalMinutes = int((12 * time.Hour) / time.Minute)
	}
	logger.Debug("upgrade search configuration", "interval", interval, "auto_grab", cfg.UpgradeSearchAutoGrab, "min_delta", cfg.UpgradeSearchMinDelta)
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
			scheduler.RecordRunDetails(runCtx, scheduler.RunDetails{Counts: map[string]int{"checked": outcome.WantedChecked, "upgrades": outcome.UpgradeCount, "grabbed": outcome.GrabbedCount, "errors": outcome.ErrorCount}, Errors: outcome.ErrorCount, NextAction: "Review upgrade history and provider health.", OperationIDs: []string{outcome.ID}})
			if err != nil {
				logger.Warn("upgrade search failed", "trigger", trigger, "error", err)
				return "", err
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
			return fmt.Sprintf(
				"%d wanted checked, %d upgrades, %d grabbed, %d errors",
				outcome.WantedChecked, outcome.UpgradeCount, outcome.GrabbedCount, outcome.ErrorCount,
			), nil
		},
	}
}

func wantedMonitorTask(logger *slog.Logger, service *wanted.Service, cfg config.Config) scheduler.Task {
	interval := cfg.MonitorInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	searchIntervalMinutes := int(cfg.MonitorSearchInterval / time.Minute)
	if searchIntervalMinutes <= 0 {
		searchIntervalMinutes = int((6 * time.Hour) / time.Minute)
	}
	logger.Debug("wanted monitor configuration", "interval", interval, "auto_grab", cfg.MonitorAutoGrab)
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
			scheduler.RecordRunDetails(runCtx, scheduler.RunDetails{Counts: map[string]int{"checked": outcome.WantedChecked, "approved": outcome.ApprovedCount, "grabbed": outcome.GrabbedCount, "errors": outcome.ErrorCount}, Errors: outcome.ErrorCount, NextAction: "Review wanted history and provider health.", OperationIDs: []string{outcome.ID}})
			if err != nil {
				logger.Warn("wanted monitor run failed", "trigger", trigger, "error", err)
				return "", err
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
	logger.Debug("author monitor configuration", "interval", interval)
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
			scheduler.RecordRunDetails(runCtx, scheduler.RunDetails{Counts: map[string]int{"checked": outcome.AuthorsChecked, "found": outcome.ItemsFound, "created": outcome.WantedCreated, "errors": outcome.ErrorCount}, Errors: outcome.ErrorCount, NextAction: "Review author monitoring history.", OperationIDs: []string{outcome.ID}})
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
func completedDownloadImportTask(logger *slog.Logger, service completedDownloadImportService, downloads completedDownloadClient, cfg config.Config) scheduler.Task {
	interval := completedImportInterval(cfg)
	logger.Debug("completed download import configuration", "interval", interval, "limit", completedImportLimit(cfg), "remove_after_seeding", cfg.CompletedRemoveEnabled)
	return scheduler.Task{
		ID:           "completed-import",
		Name:         "Completed Download Import",
		Interval:     interval,
		StartupDelay: 45 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			outcome, err := runCompletedDownloadImportOnce(ctx, service, downloads, cfg)
			removed, recycled, cleanupErrors := 0, 0, 0
			defer func() {
				operationIDs := []string{}
				for _, item := range outcome.Results {
					if item.Import != nil {
						operationIDs = append(operationIDs, item.Import.OperationID, item.Import.CalibreHandoffID)
					}
				}
				scheduler.RecordRunDetails(ctx, scheduler.RunDetails{Counts: map[string]int{"checked": outcome.Checked, "imported": outcome.Imported, "reviewQueued": outcome.ReviewQueued, "removed": removed, "recycled": recycled, "errors": outcome.Errored + cleanupErrors}, Errors: outcome.Errored + cleanupErrors, OperationIDs: operationIDs, NextAction: "Review Imports recovery and download-client status."})
			}()
			if err != nil {
				logger.Warn("completed download import failed", "trigger", trigger, "error", err)
				return "", err
			}
			if cfg.CompletedRemoveEnabled {
				removed, err = runCompletedDownloadRemovalOnce(ctx, downloads, service)
				if err != nil {
					cleanupErrors++
					logger.Warn("completed download removal failed", "trigger", trigger, "error", err)
				}
			}
			// Recycle-bin retention cleanup rides the same tick (no-op when
			// LIBRARRY_RECYCLE_BIN is unset).
			if cleaner, ok := service.(recycleBinCleaner); ok && strings.TrimSpace(cfg.RecycleBin) != "" {
				recycled, err = cleaner.CleanupRecycleBin(time.Now().UTC())
				if err != nil {
					cleanupErrors++
					logger.Warn("recycle bin cleanup failed", "trigger", trigger, "error", err)
				}
			}

			// Unresolved reviews re-count every tick (dedup happens in the store),
			// so only imports, removals, and errors get Info-level noise.
			level := slog.LevelDebug
			if outcome.Imported > 0 || outcome.Errored+cleanupErrors > 0 || removed > 0 || recycled > 0 {
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
				"errors", outcome.Errored+cleanupErrors,
			)
			return fmt.Sprintf(
				"%d checked, %d imported, %d review queued, %d removed, %d errors",
				outcome.Checked, outcome.Imported, outcome.ReviewQueued, removed, outcome.Errored+cleanupErrors,
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
type completedDownloadVerifier interface {
	VerifyCompletedDownload(context.Context, acquisition.DownloadStatus, []acquisition.DownloadFile) error
}
type completedCleanupRecorder interface {
	RecordCompletedCleanup(context.Context, acquisition.DownloadStatus, error) error
}
type completedDownloadInspector interface {
	DownloadDetails(context.Context, string, string) (acquisition.DownloadDetails, error)
}

func runCompletedDownloadRemovalOnce(ctx context.Context, client completedDownloadClient, service any) (int, error) {
	verifier, ok := service.(completedDownloadVerifier)
	if !ok {
		return 0, fmt.Errorf("completed cleanup requires import verification")
	}
	inspector, ok := client.(completedDownloadInspector)
	if !ok {
		return 0, fmt.Errorf("completed cleanup requires client file inventory")
	}
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
		details, err := inspector.DownloadDetails(runCtx, download.ID, download.Client)
		if err == nil {
			err = verifier.VerifyCompletedDownload(runCtx, download, details.Files)
		}
		if err != nil {
			if recorder, ok := service.(completedCleanupRecorder); ok {
				if persistErr := recorder.RecordCompletedCleanup(runCtx, download, err); persistErr != nil && firstErr == nil {
					firstErr = fmt.Errorf("persist cleanup failure: %w", persistErr)
				}
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("cleanup retained %s: %w", download.ID, err)
			}
			continue
		}
		result, err := client.DownloadAction(runCtx, acquisition.DownloadActionRequest{
			Action:      acquisition.DownloadActionDelete,
			Client:      download.Client,
			IDs:         []string{download.ID},
			DeleteFiles: true,
		})
		if err == nil && !result.Applied {
			err = fmt.Errorf("download client did not apply cleanup")
		}
		if recorder, ok := service.(completedCleanupRecorder); ok {
			if persistErr := recorder.RecordCompletedCleanup(runCtx, download, err); persistErr != nil && firstErr == nil {
				firstErr = fmt.Errorf("persist remote cleanup outcome: %w", persistErr)
			}
		}
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
// finished seeding. A stopped state alone is insufficient: the client must
// also report a satisfied seed goal. Import verification is a separate gate.
func completedDownloadRemovalEligible(download acquisition.DownloadStatus) bool {
	if !download.SeedGoalMet || download.Progress < 1 || !strings.EqualFold(strings.TrimSpace(download.ImportStatus), "imported") {
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
	logger.Debug("calibre conversion refresh configuration", "interval", interval, "limit", calibreConversionRefreshLimit(cfg), "max_attempts", calibreConversionRefreshMaxAttempts(cfg))
	return scheduler.Task{
		ID:           "calibre-refresh",
		Name:         "Calibre Conversion Refresh",
		Interval:     interval,
		StartupDelay: 75 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			outcome, err := runCalibreConversionRefreshOnce(ctx, logger, service, cfg, trigger)
			scheduler.RecordRunDetails(ctx, scheduler.RunDetails{Counts: map[string]int{"checked": outcome.Checked, "refreshed": outcome.Refreshed, "skipped": outcome.Skipped, "errors": outcome.Errored}, Errors: outcome.Errored, NextAction: "Review Calibre handoffs in Imports."})
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
func bootAuthMethod(ctx context.Context, compatStore *compatstore.Store, cfg config.Config) (string, error) {
	if cfg.AuthMethod != "" {
		if method := auth.NormalizeMethod(cfg.AuthMethod); method != "" {
			return method, nil
		}
		return "", fmt.Errorf("LIBRARRY_AUTH_METHOD must be none, basic, or forms")
	}
	if compatStore != nil {
		resource, ok, err := compatStore.GetResource(ctx, "auth-config", 1)
		if err != nil {
			return "", fmt.Errorf("read persisted authentication: %w", err)
		}
		if ok {
			raw, _ := resource.Payload["method"].(string)
			if method := auth.NormalizeMethod(raw); method != "" {
				return method, nil
			}
			return "", fmt.Errorf("persisted authentication method is invalid")
		}
	}
	return auth.MethodNone, nil
}

// importListSyncTask keeps enabled import lists in sync with their Hardcover
// lists/shelves. Manual per-list runs go through POST /api/v1/import-lists/{id}/sync.
func importListSyncTask(logger *slog.Logger, service *importlists.Service, cfg config.Config) scheduler.Task {
	interval := cfg.ImportListSyncInterval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	logger.Debug("import list sync configuration", "interval", interval)
	return scheduler.Task{
		ID:           "import-list-sync",
		Name:         "Import List Sync",
		Interval:     interval,
		StartupDelay: 90 * time.Second,
		Run: func(ctx context.Context, trigger string) (string, error) {
			runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
			outcome, err := service.Sync(runCtx, nil, trigger)
			scheduler.RecordRunDetails(runCtx, scheduler.RunDetails{Counts: map[string]int{"checked": outcome.ListsChecked, "found": outcome.EntriesFound, "created": outcome.WantedCreated, "errors": outcome.ErrorCount}, Errors: outcome.ErrorCount, NextAction: "Review import-list history and provider health."})
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
			message := fmt.Sprintf(
				"%d lists checked, %d entries, %d wanted created, %d errors",
				outcome.ListsChecked, outcome.EntriesFound, outcome.WantedCreated, outcome.ErrorCount,
			)
			if outcome.ErrorCount > 0 {
				return message, errors.New(outcome.Message)
			}
			return message, nil
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
	logger.Debug("scheduled backups configuration", "interval", interval, "retention", retention, "dir", cfg.BackupDir)
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
			errors := 0
			if pruneErr != nil {
				errors = 1
			}
			scheduler.RecordRunDetails(ctx, scheduler.RunDetails{Counts: map[string]int{"created": 1, "bytes": int(backup.SizeBytes), "pruned": pruned, "errors": errors}, Errors: errors, NextAction: "Review backup storage and retention permissions."})
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
			scheduler.RecordRunDetails(ctx, scheduler.RunDetails{Counts: map[string]int{"checked": len(checks), "warnings": warnings, "errors": errored}, Errors: errored, NextAction: "Review System health checks."})
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

func taskPolicy(task scheduler.Task, enabled, available bool, setting, unavailableReason string) scheduler.Task {
	if !enabled {
		task.DisabledReason = setting + " is false. Enable it and restart this API instance to resume scheduling."
	}
	if !available {
		task.UnavailableReason = unavailableReason
	}
	return task
}
