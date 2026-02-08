package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server          ServerConfig
	Database        DatabaseConfig
	Redis           RedisConfig
	Metrics         MetricsConfig
	S3              S3Config
	Dynamo          DynamoConfig
	Screenshot      ScreenshotConfig
	Security        SecurityConfig
	ReleaseAnalyzer ReleaseAnalyzerConfig
	CloudWatch      CloudWatchConfig
	NATS            NATSConfig
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
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type RedisConfig struct {
	Enabled         bool
	Host            string
	Port            string
	Password        string
	DB              int
	MaxRetries      int
	PoolSize        int
	MinIdleConns    int
	CacheTTL        time.Duration
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
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
	MaxPayloadBytes      int64
	MaxArtifactBytes     int
	RateLimitPerMinute   int
	AuthEnabled          bool
	MetadataTTLDays      int
	MetadataWriteStrict  bool
	MetadataFallbackToS3 bool
}

type DynamoConfig struct {
	Enabled                 bool
	TableScreenshotMetadata string
	Region                  string
	Endpoint                string
	AccessKeyID             string
	SecretAccessKey         string
	StrongReads             bool
}

type SecurityConfig struct {
	AllowedOrigins []string
	AuthEnabled    bool
	AuthToken      string
}

type ReleaseAnalyzerConfig struct {
	BaseURL        string
	RequestTimeout time.Duration
}

type CloudWatchConfig struct {
	MetricsEnabled bool
	LogsEnabled    bool

	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string // Optional for LocalStack

	// Metrics
	MetricsNamespace        string
	MetricsBufferSize       int
	MetricsFlushInterval    time.Duration
	MetricsStorageResolution int32
	MetricsDimensions       map[string]string

	// Logs
	LogGroupName      string
	LogStreamName     string
	LogsBufferSize    int
	LogsFlushInterval time.Duration
}

type NATSConfig struct {
	Enabled bool
	URL     string
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

	metadataTTLDays, err := strconv.Atoi(getEnv("SCREENSHOT_METADATA_TTL_DAYS", "0"))
	if err != nil {
		return nil, fmt.Errorf("invalid SCREENSHOT_METADATA_TTL_DAYS: %w", err)
	}

	releaseAnalyzerRequestTimeout, err := parseDuration(getEnv("RELEASE_ANALYZER_REQUEST_TIMEOUT", "6s"))
	if err != nil {
		return nil, fmt.Errorf("invalid RELEASE_ANALYZER_REQUEST_TIMEOUT: %w", err)
	}

	// CloudWatch configuration
	cwMetricsFlushInterval, err := parseDuration(getEnv("CLOUDWATCH_METRICS_FLUSH_INTERVAL", "10s"))
	if err != nil {
		return nil, fmt.Errorf("invalid CLOUDWATCH_METRICS_FLUSH_INTERVAL: %w", err)
	}

	cwLogsFlushInterval, err := parseDuration(getEnv("CLOUDWATCH_LOGS_FLUSH_INTERVAL", "5s"))
	if err != nil {
		return nil, fmt.Errorf("invalid CLOUDWATCH_LOGS_FLUSH_INTERVAL: %w", err)
	}

	cwMetricsBufferSize, err := strconv.Atoi(getEnv("CLOUDWATCH_METRICS_BUFFER_SIZE", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid CLOUDWATCH_METRICS_BUFFER_SIZE: %w", err)
	}

	cwLogsBufferSize, err := strconv.Atoi(getEnv("CLOUDWATCH_LOGS_BUFFER_SIZE", "50"))
	if err != nil {
		return nil, fmt.Errorf("invalid CLOUDWATCH_LOGS_BUFFER_SIZE: %w", err)
	}

	cwStorageResolution, err := strconv.Atoi(getEnv("CLOUDWATCH_METRICS_STORAGE_RESOLUTION", "60"))
	if err != nil {
		return nil, fmt.Errorf("invalid CLOUDWATCH_METRICS_STORAGE_RESOLUTION: %w", err)
	}

	redisCacheTTL, err := parseDuration(getEnv("REDIS_CACHE_TTL", "60s"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_CACHE_TTL: %w", err)
	}

	redisDialTimeout, err := parseDuration(getEnv("REDIS_DIAL_TIMEOUT", "5s"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DIAL_TIMEOUT: %w", err)
	}

	redisReadTimeout, err := parseDuration(getEnv("REDIS_READ_TIMEOUT", "3s"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_READ_TIMEOUT: %w", err)
	}

	redisWriteTimeout, err := parseDuration(getEnv("REDIS_WRITE_TIMEOUT", "3s"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_WRITE_TIMEOUT: %w", err)
	}

	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}

	redisMaxRetries, err := strconv.Atoi(getEnv("REDIS_MAX_RETRIES", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_MAX_RETRIES: %w", err)
	}

	redisPoolSize, err := strconv.Atoi(getEnv("REDIS_POOL_SIZE", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_POOL_SIZE: %w", err)
	}

	redisMinIdleConns, err := strconv.Atoi(getEnv("REDIS_MIN_IDLE_CONNS", "20"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_MIN_IDLE_CONNS: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnv("SERVER_PORT", "8080"),
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "postgres"),
			Database:        getEnv("DB_NAME", "monitoring"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    100,
			MaxIdleConns:    50,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 2 * time.Minute,
		},
		Redis: RedisConfig{
			Enabled:      getEnvBool("REDIS_ENABLED", false),
			Host:         getEnv("REDIS_HOST", "localhost"),
			Port:         getEnv("REDIS_PORT", "6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           redisDB,
			MaxRetries:   redisMaxRetries,
			PoolSize:     redisPoolSize,
			MinIdleConns: redisMinIdleConns,
			CacheTTL:     redisCacheTTL,
			DialTimeout:  redisDialTimeout,
			ReadTimeout:  redisReadTimeout,
			WriteTimeout: redisWriteTimeout,
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
		Dynamo: DynamoConfig{
			Enabled:                 getEnvBool("DYNAMO_ENABLED", false),
			TableScreenshotMetadata: getEnv("DYNAMO_TABLE_SCREENSHOT_METADATA", "dashboard_screenshot_metadata"),
			Region:                  getEnv("DYNAMO_REGION", "us-east-1"),
			Endpoint:                getEnv("DYNAMO_ENDPOINT", ""),
			AccessKeyID:             getEnv("DYNAMO_ACCESS_KEY_ID", ""),
			SecretAccessKey:         getEnv("DYNAMO_SECRET_ACCESS_KEY", ""),
			StrongReads:             getEnvBool("DYNAMO_STRONG_READS", false),
		},
		Screenshot: ScreenshotConfig{
			MaxPayloadBytes:      int64(maxPayloadMB) * 1024 * 1024,
			MaxArtifactBytes:     maxArtifactMB * 1024 * 1024,
			RateLimitPerMinute:   rateLimitPerMinute,
			AuthEnabled:          getEnvBool("SCREENSHOT_AUTH_ENABLED", true),
			MetadataTTLDays:      metadataTTLDays,
			MetadataWriteStrict:  getEnvBool("SCREENSHOT_METADATA_WRITE_STRICT", false),
			MetadataFallbackToS3: getEnvBool("SCREENSHOT_METADATA_FALLBACK_TO_S3", true),
		},
		Security: SecurityConfig{
			AllowedOrigins: splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:8080,http://127.0.0.1:8080")),
			AuthEnabled:    getEnvBool("AUTH_ENABLED", false),
			AuthToken:      getEnv("AUTH_BEARER_TOKEN", ""),
		},
		ReleaseAnalyzer: ReleaseAnalyzerConfig{
			BaseURL:        normalizeReleaseAnalyzerBaseURL(getEnv("RELEASE_ANALYZER_BASE_URL", "http://localhost:8081")),
			RequestTimeout: releaseAnalyzerRequestTimeout,
		},
		CloudWatch: CloudWatchConfig{
			MetricsEnabled:           getEnvBool("CLOUDWATCH_METRICS_ENABLED", false),
			LogsEnabled:              getEnvBool("CLOUDWATCH_LOGS_ENABLED", false),
			Region:                   getEnv("CLOUDWATCH_REGION", "us-east-1"),
			AccessKeyID:              getEnv("CLOUDWATCH_ACCESS_KEY_ID", ""),
			SecretAccessKey:          getEnv("CLOUDWATCH_SECRET_ACCESS_KEY", ""),
			Endpoint:                 getEnv("CLOUDWATCH_ENDPOINT", ""),
			MetricsNamespace:         getEnv("CLOUDWATCH_METRICS_NAMESPACE", "MonitoringDashboard/System"),
			MetricsBufferSize:        cwMetricsBufferSize,
			MetricsFlushInterval:     cwMetricsFlushInterval,
			MetricsStorageResolution: int32(cwStorageResolution),
			MetricsDimensions:        parseDimensions(getEnv("CLOUDWATCH_METRICS_DIMENSIONS", "")),
			LogGroupName:             getEnv("CLOUDWATCH_LOG_GROUP", "/aws/monitoring-dashboard"),
			LogStreamName:            getEnv("CLOUDWATCH_LOG_STREAM", "application"),
			LogsBufferSize:           cwLogsBufferSize,
			LogsFlushInterval:        cwLogsFlushInterval,
		},
		NATS: NATSConfig{
			Enabled: getEnvBool("NATS_ENABLED", false),
			URL:     getEnv("NATS_URL", "nats://nats:4222"),
		},
	}

	if cfg.Security.AuthEnabled && cfg.Security.AuthToken == "" {
		return nil, fmt.Errorf("AUTH_BEARER_TOKEN is required when AUTH_ENABLED=true")
	}

	return cfg, nil
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
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

func normalizeReleaseAnalyzerBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "http://localhost:8081"
	}
	return strings.TrimRight(trimmed, "/")
}

// parseDimensions parses a comma-separated key=value string into a map.
// Example: "Environment=production,Host=server-01" → {"Environment": "production", "Host": "server-01"}
func parseDimensions(raw string) map[string]string {
	dimensions := make(map[string]string)
	if raw == "" {
		return dimensions
	}

	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" && value != "" {
				dimensions[key] = value
			}
		}
	}

	return dimensions
}
