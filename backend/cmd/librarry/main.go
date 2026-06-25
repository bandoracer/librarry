package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/api"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/database"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.FromEnv()
	ctx := context.Background()
	var downloadStore acquisition.DownloadStore
	var wantedStore *wanted.Store

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
		logger.Info("database migrations applied")
	} else {
		logger.Warn("LIBRARRY_DATABASE_URL is not set; starting without database-backed persistence")
	}

	providers := metadata.DefaultProviders(metadata.ProviderConfig{
		HardcoverToken: cfg.HardcoverToken,
		GoogleAPIKey:   cfg.GoogleBooksAPIKey,
		HTTPTimeout:    12 * time.Second,
	})
	acquire := acquisition.NewService(acquisition.IntegrationConfig{
		ProwlarrURL:       cfg.ProwlarrURL,
		ProwlarrAPIKey:    cfg.ProwlarrAPIKey,
		QBittorrentURL:    cfg.QBittorrentURL,
		QBittorrentUser:   cfg.QBittorrentUser,
		QBittorrentPass:   cfg.QBittorrentPass,
		EbookCategory:     cfg.EbookCategory,
		AudiobookCategory: cfg.AudiobookCategory,
		BookTorrentRoot:   cfg.BookTorrentRoot,
		DownloadStore:     downloadStore,
	})
	if cfg.QBittorrentURL != "" {
		bootstrapCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		if result, err := acquire.Bootstrap(bootstrapCtx); err != nil {
			logger.Warn("qBittorrent category bootstrap failed", "error", err)
		} else {
			logger.Info("qBittorrent categories ready", "categories", result.Categories, "save_path", result.SavePath)
		}
		cancel()
	}
	wantedService := wanted.NewService(wantedStore, acquire)

	router := api.NewRouter(api.Dependencies{
		Logger:   logger,
		Config:   cfg,
		Metadata: metadata.NewService(providers),
		Acquire:  acquire,
		Wanted:   wantedService,
	})

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("api server shutdown failed", "error", err)
		os.Exit(1)
	}
}
