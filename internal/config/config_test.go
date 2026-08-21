package config

import (
	"testing"
	"time"
)

func TestLoadUsesDevelopmentSafeDefaults(t *testing.T) {
	setTestEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[0] != "http://localhost:5173" {
		t.Errorf("CORSAllowedOrigins = %#v, want local frontend origins", cfg.CORSAllowedOrigins)
	}
	if cfg.MaxUploadBytes != 100*1024*1024 {
		t.Errorf("MaxUploadBytes = %d, want %d", cfg.MaxUploadBytes, 100*1024*1024)
	}
	if cfg.AIMode != AIModeFake {
		t.Errorf("AIMode = %q, want %q", cfg.AIMode, AIModeFake)
	}
}

func TestLoadUsesCurrentGeminiModelsByDefault(t *testing.T) {
	setTestEnvironment(t)
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")
	t.Setenv("AI_MODE", "gemini")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GeminiTranscriptionModel != "gemini-3.7-flash" {
		t.Errorf("GeminiTranscriptionModel = %q, want gemini-3.7-flash", cfg.GeminiTranscriptionModel)
	}
	if cfg.GeminiAnalysisModel != "gemini-3.7-flash" {
		t.Errorf("GeminiAnalysisModel = %q, want gemini-3.7-flash", cfg.GeminiAnalysisModel)
	}
}

func TestLoadParsesDurations(t *testing.T) {
	setTestEnvironment(t)
	t.Setenv("JWT_ACCESS_TTL", "20m")
	t.Setenv("JWT_REFRESH_TTL", "168h")
	t.Setenv("PROVIDER_TIMEOUT", "15s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.JWTAccessTTL != 20*time.Minute {
		t.Errorf("JWTAccessTTL = %s, want 20m", cfg.JWTAccessTTL)
	}
	if cfg.JWTRefreshTTL != 168*time.Hour {
		t.Errorf("JWTRefreshTTL = %s, want 168h", cfg.JWTRefreshTTL)
	}
	if cfg.ProviderTimeout != 15*time.Second {
		t.Errorf("ProviderTimeout = %s, want 15s", cfg.ProviderTimeout)
	}
}

func TestLoadRequiresJWTSecretsOutsideTests(t *testing.T) {
	setTestEnvironment(t)
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_ACCESS_SECRET", "")
	t.Setenv("JWT_REFRESH_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected missing JWT secrets outside tests to fail")
	}
}

func TestLoadRequiresNonEmptyDistinctJWTSecretsInAllEnvironments(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		accessSecret  string
		refreshSecret string
	}{
		{name: "missing access", accessSecret: "", refreshSecret: "refresh-secret"},
		{name: "missing refresh", accessSecret: "access-secret", refreshSecret: ""},
		{name: "same secrets", accessSecret: "shared-secret", refreshSecret: "shared-secret"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			setTestEnvironment(t)
			t.Setenv("JWT_ACCESS_SECRET", testCase.accessSecret)
			t.Setenv("JWT_REFRESH_SECRET", testCase.refreshSecret)

			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want JWT secret validation error")
			}
		})
	}
}

func TestLoadRejectsAccessTokenTTLOverOneHour(t *testing.T) {
	setTestEnvironment(t)
	t.Setenv("JWT_ACCESS_TTL", "61m")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want access TTL maximum validation error")
	}
}

func TestLoadRequiresRefreshTTLToExceedAccessTTL(t *testing.T) {
	setTestEnvironment(t)
	t.Setenv("JWT_ACCESS_TTL", "30m")
	t.Setenv("JWT_REFRESH_TTL", "30m")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want refresh TTL ordering validation error")
	}
}

func TestLoadRejectsProductionAutoModeWithoutGeminiKey(t *testing.T) {
	setTestEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("AI_MODE", "auto")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("JWT_ACCESS_SECRET", "access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "refresh-secret")

	if _, err := Load(); err == nil {
		t.Fatal("expected production auto mode without key to fail")
	}
}

func TestLoadRejectsProductionFakeMode(t *testing.T) {
	setTestEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("AI_MODE", "fake")
	t.Setenv("JWT_ACCESS_SECRET", "access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "refresh-secret")

	if _, err := Load(); err == nil {
		t.Fatal("expected production fake mode to fail")
	}
}

func TestLoadFallsBackToFakeForNonProductionAutoModeWithoutGeminiKey(t *testing.T) {
	setTestEnvironment(t)
	t.Setenv("APP_ENV", "development")
	t.Setenv("AI_MODE", "auto")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("JWT_ACCESS_SECRET", "access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "refresh-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AIMode != AIModeFake {
		t.Errorf("AIMode = %q, want %q", cfg.AIMode, AIModeFake)
	}
}

func TestLoadRejectsAutoModeWithoutGeminiKeyOutsideDevelopmentOrTest(t *testing.T) {
	for _, appEnv := range []string{"staging", "unknown"} {
		t.Run(appEnv, func(t *testing.T) {
			setTestEnvironment(t)
			t.Setenv("APP_ENV", appEnv)
			t.Setenv("AI_MODE", "auto")
			t.Setenv("GEMINI_API_KEY", "")
			t.Setenv("JWT_ACCESS_SECRET", "access-secret")
			t.Setenv("JWT_REFRESH_SECRET", "refresh-secret")

			if _, err := Load(); err == nil {
				t.Fatalf("expected APP_ENV=%q auto mode without Gemini key to fail", appEnv)
			}
		})
	}
}

func TestLoadDoesNotProvideDefaultDatabaseOrMinIOCredentials(t *testing.T) {
	setTestEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseURL != "" {
		t.Errorf("DatabaseURL = %q, want empty when DATABASE_URL is unset", cfg.DatabaseURL)
	}
	if cfg.MinIOAccessKey != "" {
		t.Errorf("MinIOAccessKey = %q, want empty when MINIO_ACCESS_KEY is unset", cfg.MinIOAccessKey)
	}
	if cfg.MinIOSecretKey != "" {
		t.Errorf("MinIOSecretKey = %q, want empty when MINIO_SECRET_KEY is unset", cfg.MinIOSecretKey)
	}
}

func setTestEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"APP_ENV", "HTTP_PORT", "DATABASE_URL", "REDIS_ADDR", "CORS_ALLOWED_ORIGINS",
		"MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_BUCKET", "MINIO_USE_SSL",
		"JWT_ACCESS_SECRET", "JWT_REFRESH_SECRET", "JWT_ACCESS_TTL", "JWT_REFRESH_TTL",
		"GEMINI_API_KEY", "GEMINI_TRANSCRIPTION_MODEL", "GEMINI_ANALYSIS_MODEL", "AI_MODE",
		"MAX_UPLOAD_BYTES", "PROVIDER_TIMEOUT", "BOOTSTRAP_ADMIN_EMAIL", "BOOTSTRAP_ADMIN_PASSWORD",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("APP_ENV", "test")
	t.Setenv("JWT_ACCESS_SECRET", "test-access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "test-refresh-secret")
}
