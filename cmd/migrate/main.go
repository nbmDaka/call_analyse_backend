package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"call_analyse_backend/internal/config"
	"call_analyse_backend/internal/database"
	"call_analyse_backend/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("migration startup configuration failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg)
	if err != nil {
		logger.Error("migration database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.RunMigrations(ctx, pool, migrations.FS); err != nil {
		logger.Error("migration execution failed", "error", err)
		os.Exit(1)
	}

	logger.Info("migrations applied")
}
