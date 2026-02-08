package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config stores gateway runtime configuration.
type Config struct {
	ServerPort string
	LogLevel   string

	Auth AuthConfig

	Discovery DiscoveryConfig

	Upstream UpstreamConfig

	RateLimit RateLimitConfig
}

// AuthConfig controls gateway authentication behavior.
type AuthConfig struct {
	Enabled     bool
	BearerToken string
}

// DiscoveryConfig controls upstream service discovery.
type DiscoveryConfig struct {
	Enabled                 bool
	Namespace               string
	RefreshInterval         time.Duration
	APIServiceSelector      string
	AnalyzerServiceSelector string
	AnalyzerRequired        bool
}

// UpstreamConfig stores static upstream URLs for non-k8s runs.
type UpstreamConfig struct {
	APIURL      string
	AnalyzerURL string
	Timeout     time.Duration
}

// RateLimitConfig controls global and per-IP limits.
type RateLimitConfig struct {
	RPS   float64
	Burst int
}

// Load reads configuration from environment.
func Load() (*Config, error) {
	cfg := &Config{
		ServerPort: getEnv("SERVER_PORT", "8082"),
		LogLevel:   strings.ToLower(getEnv("LOG_LEVEL", "info")),
		Auth: AuthConfig{
			Enabled:     getEnvBool("AUTH_ENABLED", true),
			BearerToken: getEnv("AUTH_BEARER_TOKEN", ""),
		},
		Discovery: DiscoveryConfig{
			Enabled:                 getEnvBool("K8S_DISCOVERY_ENABLED", true),
			Namespace:               getEnv("K8S_NAMESPACE", "default"),
			RefreshInterval:         getEnvDuration("K8S_DISCOVERY_REFRESH_INTERVAL", 30*time.Second),
			APIServiceSelector:      getEnv("K8S_SERVICE_SELECTOR_API", "app.kubernetes.io/name=monitoring-dashboard"),
			AnalyzerServiceSelector: getEnv("K8S_SERVICE_SELECTOR_ANALYZER", "app.kubernetes.io/name=monitoring-dashboard-release-analyzer"),
			AnalyzerRequired:        getEnvBool("ANALYZER_REQUIRED", true),
		},
		Upstream: UpstreamConfig{
			APIURL:      getEnv("API_UPSTREAM_URL", "http://localhost:8080"),
			AnalyzerURL: getEnv("ANALYZER_UPSTREAM_URL", "http://localhost:8081"),
			Timeout:     getEnvDuration("UPSTREAM_REQUEST_TIMEOUT", 8*time.Second),
		},
		RateLimit: RateLimitConfig{
			RPS:   getEnvFloat("RATE_LIMIT_RPS", 100),
			Burst: getEnvInt("RATE_LIMIT_BURST", 200),
		},
	}

	if cfg.Auth.Enabled && cfg.Auth.BearerToken == "" {
		return nil, fmt.Errorf("AUTH_ENABLED=true requires AUTH_BEARER_TOKEN")
	}

	if cfg.Discovery.RefreshInterval <= 0 {
		return nil, fmt.Errorf("K8S_DISCOVERY_REFRESH_INTERVAL must be positive")
	}

	if cfg.Upstream.Timeout <= 0 {
		return nil, fmt.Errorf("UPSTREAM_REQUEST_TIMEOUT must be positive")
	}

	if cfg.RateLimit.RPS <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_RPS must be positive")
	}
	if cfg.RateLimit.Burst <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_BURST must be positive")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		parsed, err := time.ParseDuration(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
