package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Metrics    MetricsConfig
	S3         S3Config
	Screenshot ScreenshotConfig
	Security   SecurityConfig
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type MetricsConfig struct {
	CollectionInterval time.Duration
	RetentionDays      int
}

type S3Config struct {
	Enabled         bool
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
	KeyPrefix       string
	URLMode         string
	PresignedTTL    time.Duration
}

type ScreenshotConfig struct {
	MaxPayloadBytes    int64
	MaxArtifactBytes   int
	RateLimitPerMinute int
}

type SecurityConfig struct {
	AllowedOrigins []string
	AuthEnabled    bool
	AuthToken      string
}

func Load() (*Config, error) {
	// Загружаем .env файл (игнорируем ошибку если файла нет)
	_ = godotenv.Load()

	collectionInterval, err := parseDuration(getEnv("METRICS_COLLECTION_INTERVAL", "2s"))
	if err != nil {
		return nil, fmt.Errorf("invalid METRICS_COLLECTION_INTERVAL: %w", err)
	}

	retentionDays, err := strconv.Atoi(getEnv("METRICS_RETENTION_DAYS", "7"))
	if err != nil {
		return nil, fmt.Errorf("invalid METRICS_RETENTION_DAYS: %w", err)
	}

	presignedTTL, err := parseDuration(getEnv("S3_PRESIGNED_TTL", "5m"))
	if err != nil {
		return nil, fmt.Errorf("invalid S3_PRESIGNED_TTL: %w", err)
	}

	maxPayloadMB, err := strconv.Atoi(getEnv("SCREENSHOT_MAX_PAYLOAD_MB", "20"))
	if err != nil {
		return nil, fmt.Errorf("invalid SCREENSHOT_MAX_PAYLOAD_MB: %w", err)
	}

	maxArtifactMB, err := strconv.Atoi(getEnv("SCREENSHOT_MAX_ARTIFACT_MB", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid SCREENSHOT_MAX_ARTIFACT_MB: %w", err)
	}

	rateLimitPerMinute, err := strconv.Atoi(getEnv("SCREENSHOT_RATE_LIMIT_PER_MINUTE", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid SCREENSHOT_RATE_LIMIT_PER_MINUTE: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnv("SERVER_PORT", "8080"),
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "postgres"),
			Database:        getEnv("DB_NAME", "monitoring"),
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 10 * time.Minute,
		},
		Metrics: MetricsConfig{
			CollectionInterval: collectionInterval,
			RetentionDays:      retentionDays,
		},
		S3: S3Config{
			Enabled:         getEnvBool("S3_ENABLED", true),
			Bucket:          getEnv("S3_BUCKET", ""),
			Region:          getEnv("S3_REGION", "ru-central1"),
			Endpoint:        getEnv("S3_ENDPOINT", "https://storage.yandexcloud.net"),
			AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", ""),
			UsePathStyle:    getEnvBool("S3_USE_PATH_STYLE", true),
			KeyPrefix:       getEnv("S3_KEY_PREFIX", "dashboards"),
			URLMode:         getEnv("S3_URL_MODE", "presigned"),
			PresignedTTL:    presignedTTL,
		},
		Screenshot: ScreenshotConfig{
			MaxPayloadBytes:    int64(maxPayloadMB) * 1024 * 1024,
			MaxArtifactBytes:   maxArtifactMB * 1024 * 1024,
			RateLimitPerMinute: rateLimitPerMinute,
		},
		Security: SecurityConfig{
			AllowedOrigins: splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:8080,http://127.0.0.1:8080")),
			AuthEnabled:    getEnvBool("AUTH_ENABLED", false),
			AuthToken:      getEnv("AUTH_BEARER_TOKEN", ""),
		},
	}

	if cfg.Security.AuthEnabled && cfg.Security.AuthToken == "" {
		return nil, fmt.Errorf("AUTH_BEARER_TOKEN is required when AUTH_ENABLED=true")
	}

	return cfg, nil
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.Host, c.Port, c.User, c.Password, c.Database)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}

func splitCSV(raw string) []string {
	items := make([]string, 0)
	current := ""

	for _, r := range raw {
		if r == ',' {
			if current != "" {
				items = append(items, current)
				current = ""
			}
			continue
		}
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			current += string(r)
		}
	}

	if current != "" {
		items = append(items, current)
	}

	return items
}

func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
