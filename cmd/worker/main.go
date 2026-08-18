package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"call_analyse_backend/internal/modules/analysis"
	"call_analyse_backend/internal/modules/calls"
	"call_analyse_backend/internal/config"
	"call_analyse_backend/internal/platform/database"
	"call_analyse_backend/internal/integrations/ai"
	"call_analyse_backend/internal/platform/queue"
	"call_analyse_backend/internal/platform/storage"
	"call_analyse_backend/internal/modules/transcription"
	"call_analyse_backend/internal/jobs"

	"github.com/hibiken/asynq"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("worker startup configuration failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg)
	if err != nil {
		logger.Error("worker database startup failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	storageCtx, cancelStorage := context.WithTimeout(ctx, cfg.ProviderTimeout)
	objects, err := storage.NewMinIOStore(storageCtx, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	cancelStorage()
	if err != nil {
		logger.Error("worker object storage startup failed", "error", err)
		os.Exit(1)
	}

	transcriber, analyzer, err := workerProviders(cfg)
	if err != nil {
		logger.Error("worker provider startup failed", "error", err)
		os.Exit(1)
	}

	processor := &worker.Processor{
		Calls:           calls.NewPostgresStore(pool),
		Transcripts:     transcription.NewPostgresStore(pool),
		Analyses:        analysis.NewPostgresStore(pool),
		Transcriber:     transcriber,
		Analyzer:        analyzer,
		Objects:         objects,
		ProviderTimeout: cfg.ProviderTimeout,
	}
	mux := asynq.NewServeMux()
	mux.Handle(queue.TypeProcessCall, worker.NewHandler(processor))
	server := asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.RedisAddr}, asynq.Config{
		Concurrency: 10,
		Queues:      map[string]int{queue.ProcessCallQueue: 1},
	})
	if err := server.Start(mux); err != nil {
		logger.Error("worker Asynq startup failed", "error", err)
		os.Exit(1)
	}

	logger.Info("worker started", "queue", queue.ProcessCallQueue, "max_retry", queue.ProcessCallMaxRetry)
	<-ctx.Done()
	logger.Info("worker shutdown requested")
	server.Shutdown()
}

func workerProviders(cfg config.Config) (transcription.TranscriptionProvider, analysis.AnalysisProvider, error) {
	switch cfg.AIMode {
	case config.AIModeFake:
		return transcription.FakeProvider{}, providers.FakeAnalysisProvider{}, nil
	case config.AIModeGemini:
		gemini, err := providers.NewGemini(providers.GeminiConfig{
			APIKey:             cfg.GeminiAPIKey,
			TranscriptionModel: cfg.GeminiTranscriptionModel,
			AnalysisModel:      cfg.GeminiAnalysisModel,
			Timeout:            cfg.ProviderTimeout,
		})
		if err != nil {
			return nil, nil, err
		}
		return gemini, gemini, nil
	default:
		return nil, nil, fmt.Errorf("unsupported resolved AI mode %q", cfg.AIMode)
	}
}
