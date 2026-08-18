// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"call_analyse_backend/internal/modules/auth"
)

const (
	AIModeAuto   = "auto"
	AIModeFake   = "fake"
	AIModeGemini = "gemini"
)

const (
	defaultRedisAddr   = "redis:6379"
	defaultMinIOHost   = "minio:9000"
	defaultMinIOBucket = "call-audio"
)

// Config is shared by the API, worker, and migration runtimes.
type Config struct {
	AppEnv string

	HTTPPort           int
	CORSAllowedOrigins []string
	DatabaseURL        string
	RedisAddr          string

	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration

	GeminiAPIKey             string
	GeminiTranscriptionModel string
	GeminiAnalysisModel      string
	AIMode                   string

	MaxUploadBytes  int64
	ProviderTimeout time.Duration

	BootstrapAdminEmail    string
	BootstrapAdminPassword string
}

// Load parses and validates process configuration. AIMode contains the resolved
// provider mode: auto selects Gemini when a key is configured and fake only in
// development or test when no key is configured.
func Load() (Config, error) {
	appEnv := strings.ToLower(envOrDefault("APP_ENV", "development"))

	httpPort, err := parseInt("HTTP_PORT", envOrDefault("HTTP_PORT", "8080"))
	if err != nil {
		return Config{}, err
	}
	if httpPort < 1 || httpPort > 65535 {
		return Config{}, fmt.Errorf("HTTP_PORT must be between 1 and 65535")
	}

	minIOUseSSL, err := parseBool("MINIO_USE_SSL", envOrDefault("MINIO_USE_SSL", "false"))
	if err != nil {
		return Config{}, err
	}

	accessTTL, err := parseDuration("JWT_ACCESS_TTL", envOrDefault("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return Config{}, err
	}
	refreshTTL, err := parseDuration("JWT_REFRESH_TTL", envOrDefault("JWT_REFRESH_TTL", "168h"))
	if err != nil {
		return Config{}, err
	}
	providerTimeout, err := parseDuration("PROVIDER_TIMEOUT", envOrDefault("PROVIDER_TIMEOUT", "60s"))
	if err != nil {
		return Config{}, err
	}
	if providerTimeout <= 0 {
		return Config{}, fmt.Errorf("provider duration must be greater than zero")
	}

	maxUploadBytes, err := parseInt64("MAX_UPLOAD_BYTES", envOrDefault("MAX_UPLOAD_BYTES", "104857600"))
	if err != nil {
		return Config{}, err
	}
	if maxUploadBytes <= 0 {
		return Config{}, fmt.Errorf("MAX_UPLOAD_BYTES must be greater than zero")
	}

	accessSecret := os.Getenv("JWT_ACCESS_SECRET")
	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")
	if err := auth.ValidateTokenConfiguration(accessSecret, refreshSecret, accessTTL, refreshTTL); err != nil {
		return Config{}, fmt.Errorf("invalid JWT configuration: %w", err)
	}

	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	aiMode, err := resolveAIMode(appEnv, envOrDefault("AI_MODE", AIModeAuto), geminiAPIKey)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppEnv: appEnv,

		HTTPPort:           httpPort,
		CORSAllowedOrigins: parseCSV(envOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisAddr:          envOrDefault("REDIS_ADDR", defaultRedisAddr),

		MinIOEndpoint:  envOrDefault("MINIO_ENDPOINT", defaultMinIOHost),
		MinIOAccessKey: os.Getenv("MINIO_ACCESS_KEY"),
		MinIOSecretKey: os.Getenv("MINIO_SECRET_KEY"),
		MinIOBucket:    envOrDefault("MINIO_BUCKET", defaultMinIOBucket),
		MinIOUseSSL:    minIOUseSSL,

		JWTAccessSecret:  accessSecret,
		JWTRefreshSecret: refreshSecret,
		JWTAccessTTL:     accessTTL,
		JWTRefreshTTL:    refreshTTL,

		GeminiAPIKey:             geminiAPIKey,
		GeminiTranscriptionModel: envOrDefault("GEMINI_TRANSCRIPTION_MODEL", "gemini-2.0-flash"),
		GeminiAnalysisModel:      envOrDefault("GEMINI_ANALYSIS_MODEL", "gemini-2.0-flash"),
		AIMode:                   aiMode,

		MaxUploadBytes:  maxUploadBytes,
		ProviderTimeout: providerTimeout,

		BootstrapAdminEmail:    os.Getenv("BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
	}, nil
}

func parseCSV(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func resolveAIMode(appEnv, requestedMode, geminiAPIKey string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(requestedMode)) {
	case AIModeAuto:
		if geminiAPIKey != "" {
			return AIModeGemini, nil
		}
		if appEnv != "development" && appEnv != "test" {
			return "", fmt.Errorf("GEMINI_API_KEY is required when APP_ENV=%q and AI_MODE=auto", appEnv)
		}
		return AIModeFake, nil
	case AIModeFake:
		if appEnv == "production" {
			return "", fmt.Errorf("AI_MODE=fake is not allowed when APP_ENV=production")
		}
		return AIModeFake, nil
	case AIModeGemini:
		if geminiAPIKey == "" {
			return "", fmt.Errorf("GEMINI_API_KEY is required when AI_MODE=gemini")
		}
		return AIModeGemini, nil
	default:
		return "", fmt.Errorf("AI_MODE must be one of %q, %q, or %q", AIModeAuto, AIModeFake, AIModeGemini)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseInt(key, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func parseInt64(key, value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func parseBool(key, value string) (bool, error) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func parseDuration(key, value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}
