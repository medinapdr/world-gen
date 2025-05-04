package config

import (
	"fmt"
	"os"
)

// Application constants
const (
	DefaultRateLimit    = 100
	DefaultRateWindow   = 60
	DefaultHistoryLimit = 10
)

// AppConfig stores application configurations
type AppConfig struct {
	RateLimit    int
	RateWindow   int
	HistoryLimit int
}

// NewAppConfig creates a new instance of the application configuration
func NewAppConfig() *AppConfig {
	return &AppConfig{
		RateLimit:    getEnvAsInt("RATE_LIMIT", DefaultRateLimit),
		RateWindow:   getEnvAsInt("RATE_WINDOW", DefaultRateWindow),
		HistoryLimit: getEnvAsInt("HISTORY_LIMIT", DefaultHistoryLimit),
	}
}

// getEnvAsInt gets an environment variable as integer
func getEnvAsInt(key string, defaultVal int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := parseInt(value); err == nil {
			return intVal
		}
	}
	return defaultVal
}

// parseInt converts string to int
func parseInt(value string) (int, error) {
	var result int
	_, err := fmt.Sscanf(value, "%d", &result)
	return result, err
}
