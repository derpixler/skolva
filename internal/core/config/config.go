package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	Env              string
	DatabaseURL      string
	JWTSecret        string
	JWTExpiryHours   int
	ZFARequiredRoles []string
	AIProviderURL    string
	AIAPIKey         string
	AIModel          string
	AIGDPRMode       string
	StorageBackend   string
	StorageLocalPath string
	S3Endpoint       string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	SMTPHost         string
	SMTPPort         int
	SMTPUser         string
	SMTPPassword     string
	SMTPFrom         string
	ModulesEnabled   []string
	LogLevel         string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:             getEnv("APP_PORT", "8080"),
		Env:              getEnv("APP_ENV", "development"),
		DatabaseURL:      requireEnv("DATABASE_URL"),
		JWTSecret:        requireEnv("JWT_SECRET"),
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

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return val
}

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

func (c *Config) IsModuleEnabled(name string) bool {
	for _, m := range c.ModulesEnabled {
		if m == name {
			return true
		}
	}
	return false
}
