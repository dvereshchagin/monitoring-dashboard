package releaseanalyzer

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port     string
	Interval time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	interval, err := time.ParseDuration(getEnv("ANALYZER_INTERVAL", "30s"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid ANALYZER_INTERVAL: %w", err)
	}

	if interval < 5*time.Second {
		return Config{}, errors.New("ANALYZER_INTERVAL must be >= 5s")
	}

	return Config{
		Port:     getEnv("ANALYZER_PORT", "8081"),
		Interval: interval,
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
