// Package config loads application configuration from environment variables.
//
// Variables are read once at startup via config.Load(). Missing required
// variables cause a panic (fail-fast for misconfigured deployments).
// Optional variables fall back to sensible defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration derived from environment variables.
type Config struct {
	Port             string   // HTTP listen port, default "8088"
	Env              string   // deployment environment: "development" or "production"
	DatabaseURL      string   // PostgreSQL connection string (required)
	JWTSecret        string   // HS256 signing key (required)
	EncryptionKey    string   // AES-256 key for TOTP secret encryption (required)
	JWTExpiryHours   int      // token lifetime in hours, default 24
	ZFARequiredRoles []string // roles that MUST enable two-factor auth
	AIProviderURL    string   // OpenAI-compatible API base URL
	AIAPIKey         string   // API key for the AI provider
	AIModel          string   // model name, e.g. "gpt-4o"
	AIGDPRMode       string   // "disabled", "strict", or "cloud"
	StorageBackend   string   // "local" or "s3"
	StorageLocalPath string   // local filesystem path when backend=local
	S3Endpoint       string   // S3-compatible endpoint
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	SMTPHost         string // mail server hostname
	SMTPPort         int    // mail server port, default 1025
	SMTPUser         string
	SMTPPassword     string
	SMTPFrom         string   // sender address, default noreply@skolva.org
	ModulesEnabled   []string // comma-separated module list
	LogLevel         string   // zap log level, default "debug"
}

// Load reads configuration from the environment, with an optional .env file.
// Required variables (DATABASE_URL, JWT_SECRET) panic if unset.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:             getEnv("APP_PORT", "8088"),
		Env:              getEnv("APP_ENV", "development"),
		DatabaseURL:      requireEnv("DATABASE_URL"),
		JWTSecret:        requireEnv("JWT_SECRET"),
		EncryptionKey:    requireEnv("ENCRYPTION_KEY"),
		JWTExpiryHours:   getEnvInt("JWT_EXPIRY_HOURS", 24),
		ZFARequiredRoles: getEnvSlice("ZFA_REQUIRED_ROLES", []string{"admin", "vorstand", "kassierer"}),
		AIProviderURL:    getEnv("AI_PROVIDER_URL", ""),
		AIAPIKey:         getEnv("AI_API_KEY", ""),
		AIModel:          getEnv("AI_MODEL", ""),
		AIGDPRMode:       getEnv("AI_GDPR_MODE", "disabled"),
		StorageBackend:   getEnv("STORAGE_BACKEND", "local"),
		StorageLocalPath: getEnv("STORAGE_LOCAL_PATH", "./data"),
		S3Endpoint:       getEnv("S3_ENDPOINT", ""),
		S3AccessKey:      getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:      getEnv("S3_SECRET_KEY", ""),
		S3Bucket:         getEnv("S3_BUCKET", ""),
		SMTPHost:         getEnv("SMTP_HOST", "localhost"),
		SMTPPort:         getEnvInt("SMTP_PORT", 1025),
		SMTPUser:         getEnv("SMTP_USER", ""),
		SMTPPassword:     getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:         getEnv("SMTP_FROM", "noreply@skolva.org"),
		ModulesEnabled:   getEnvSlice("MODULES_ENABLED", []string{"auth", "crm", "groups", "documents", "audit", "sharing"}),
		LogLevel:         getEnv("LOG_LEVEL", "debug"),
	}

	return cfg, nil
}

// IsModuleEnabled reports whether the given module is present in MODULES_ENABLED.
func (c *Config) IsModuleEnabled(name string) bool {
	for _, m := range c.ModulesEnabled {
		if m == name {
			return true
		}
	}
	return false
}

// getEnv returns the environment variable value or the fallback if unset.
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// requireEnv returns the environment variable value or panics if unset.
func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return val
}

// getEnvInt parses an integer environment variable, falling back on error.
func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return i
}

// getEnvSlice splits a comma-separated environment variable into a slice,
// trimming whitespace. Returns fallback if the result would be empty.
func getEnvSlice(key string, fallback []string) []string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parts := strings.Split(val, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
