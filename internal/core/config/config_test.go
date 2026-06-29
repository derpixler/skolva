package config_test

import (
	"os"
	"testing"

	"github.com/derpixler/skolva/internal/core/config"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.EncryptionKey != "test-encryption-key" {
		t.Errorf("unexpected encryption key: %s", cfg.EncryptionKey)
	}
	if cfg.Port != "8088" {
		t.Errorf("expected port 8088, got %s", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("expected env development, got %s", cfg.Env)
	}
	if cfg.DatabaseURL != "postgres://test:test@localhost:5432/test" {
		t.Errorf("unexpected database url: %s", cfg.DatabaseURL)
	}
	if cfg.JWTExpiryHours != 24 {
		t.Errorf("expected JWTExpiryHours 24, got %d", cfg.JWTExpiryHours)
	}
}

func TestLoadOverrides(t *testing.T) {
	setEnv(t, "DATABASE_URL", "postgres://test:test@localhost:5432/test")
	setEnv(t, "JWT_SECRET", "test-secret")
	setEnv(t, "ENCRYPTION_KEY", "test-encryption-key")
	setEnv(t, "APP_PORT", "9090")
	setEnv(t, "APP_ENV", "production")
	setEnv(t, "JWT_EXPIRY_HOURS", "48")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
	if cfg.Env != "production" {
		t.Errorf("expected env production, got %s", cfg.Env)
	}
	if cfg.JWTExpiryHours != 48 {
		t.Errorf("expected JWTExpiryHours 48, got %d", cfg.JWTExpiryHours)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	_ = os.Unsetenv("DATABASE_URL")
	_ = os.Unsetenv("JWT_SECRET")

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing required env var")
		}
	}()

	_, _ = config.Load()
}

func TestLoadMissingEncryptionKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	t.Setenv("JWT_SECRET", "test-secret")
	_ = os.Unsetenv("ENCRYPTION_KEY")

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing ENCRYPTION_KEY")
		}
	}()

	_, _ = config.Load()
}

func TestModulesEnabled(t *testing.T) {
	setEnv(t, "DATABASE_URL", "postgres://test:test@localhost:5432/test")
	setEnv(t, "JWT_SECRET", "test-secret")
	setEnv(t, "ENCRYPTION_KEY", "test-encryption-key")
	setEnv(t, "MODULES_ENABLED", "auth,groups,billing")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.IsModuleEnabled("auth") {
		t.Error("expected auth to be enabled")
	}
	if !cfg.IsModuleEnabled("groups") {
		t.Error("expected groups to be enabled")
	}
	if !cfg.IsModuleEnabled("billing") {
		t.Error("expected billing to be enabled")
	}
	if cfg.IsModuleEnabled("crm") {
		t.Error("expected crm to be disabled")
	}
}

func TestAIGDPRModeDisabled(t *testing.T) {
	setEnv(t, "DATABASE_URL", "postgres://test:test@localhost:5432/test")
	setEnv(t, "JWT_SECRET", "test-secret")
	setEnv(t, "ENCRYPTION_KEY", "test-encryption-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AIGDPRMode != "disabled" {
		t.Errorf("expected AIGDPRMode disabled, got %s", cfg.AIGDPRMode)
	}
}

func TestStorageBackend(t *testing.T) {
	setEnv(t, "DATABASE_URL", "postgres://test:test@localhost:5432/test")
	setEnv(t, "JWT_SECRET", "test-secret")
	setEnv(t, "ENCRYPTION_KEY", "test-encryption-key")
	setEnv(t, "STORAGE_BACKEND", "s3")
	setEnv(t, "S3_ENDPOINT", "http://minio:9000")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.StorageBackend != "s3" {
		t.Errorf("expected s3, got %s", cfg.StorageBackend)
	}
	if cfg.S3Endpoint != "http://minio:9000" {
		t.Errorf("unexpected S3Endpoint: %s", cfg.S3Endpoint)
	}
}
