package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"call_analyse_backend/internal/auth"
	"call_analyse_backend/internal/calls"
	"call_analyse_backend/internal/config"
	"call_analyse_backend/internal/dashboard"
	"call_analyse_backend/internal/database"
	"call_analyse_backend/internal/httpapi"
	"call_analyse_backend/internal/queue"
	"call_analyse_backend/internal/storage"
	"github.com/hibiken/asynq"
)

const storageInitializationTimeout = 15 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("API startup configuration failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, cfg)
	if err != nil {
		logger.Error("API database startup failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	storageCtx, cancel := context.WithTimeout(ctx, storageInitializationTimeout)
	defer cancel()
	objects, err := storage.NewMinIOStore(storageCtx, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	if err != nil {
		logger.Error("API storage initialization failed", "error", "configured MinIO bucket unavailable")
		os.Exit(1)
	}

	tokens, err := auth.NewTokenManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	if err != nil {
		logger.Error("API token configuration failed", "error", err)
		os.Exit(1)
	}
	authStore := auth.NewPostgresStore(pool)
	authService := auth.NewService(authStore, authStore, auth.NewPasswordHasher(), tokens)
	if cfg.BootstrapAdminEmail != "" && cfg.BootstrapAdminPassword != "" {
		if _, _, err := authService.BootstrapAdmin(ctx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword); err != nil {
			logger.Error("API bootstrap admin failed", "error", err)
			os.Exit(1)
		}
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr})
	defer client.Close()
	enqueue := func(enqueueCtx context.Context, callID string) error {
		task, err := queue.NewProcessCallTask(callID)
		if err != nil {
			return err
		}
		_, err = client.EnqueueContext(enqueueCtx, task)
		return err
	}
	callStore := calls.NewPostgresStore(pool)
	callService := calls.NewService(callStore, objects, cfg.MaxUploadBytes)
	dashboardService := dashboard.NewService(dashboard.NewPostgresStore(pool))
	ready := func(readyCtx context.Context) error {
		if err := pool.Ping(readyCtx); err != nil {
			return err
		}
		if err := client.Ping(); err != nil {
			return err
		}
		return objects.Ready(readyCtx)
	}
	handler := httpapi.NewRouter(httpapi.Dependencies{Authentication: authService, Calls: callService, Dashboard: dashboardService, Tokens: tokens, EnqueueCall: enqueue, Ready: ready, MaxUploadBytes: cfg.MaxUploadBytes, RequestTimeout: cfg.ProviderTimeout, Logger: logger})
	httpServer := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HTTPPort), Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if serveErr := httpServer.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			logger.Error("API HTTP server failed", "error", serveErr)
			stop()
		}
	}()
	logger.Info("API started", "port", cfg.HTTPPort)
	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func initializeStorage(ctx context.Context, cfg config.Config) error {
	_, err := storage.NewMinIOStore(
		ctx,
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIOUseSSL,
	)
	return err
}
