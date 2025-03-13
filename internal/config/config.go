package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration
type Config struct {
	// Redfish connection settings
	RedfishHost     string
	RedfishUsername string
	RedfishPassword string
	RedfishInsecure bool

	// HTTP server settings
	ListenAddress string
	MetricsPath   string

	// Collection settings
	ScrapeInterval time.Duration
	Timeout        time.Duration
}

// NewConfig creates a new Config with values from environment or defaults
func NewConfig() *Config {
	return &Config{
		RedfishHost:     getEnv("REDFISH_HOST", "http://localhost:5000"),
		RedfishUsername: getEnv("REDFISH_USERNAME", "admin"),
		RedfishPassword: getEnv("REDFISH_PASSWORD", "password"),
		RedfishInsecure: getBoolEnv("REDFISH_INSECURE", true),

		ListenAddress: getEnv("LISTEN_ADDRESS", "localhost:9290"),
		MetricsPath:   getEnv("METRICS_PATH", "/metrics"),

		ScrapeInterval: getDurationEnv("SCRAPE_INTERVAL", 60*time.Second),
		Timeout:        getDurationEnv("TIMEOUT", 30*time.Second),
	}
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getBoolEnv retrieves a boolean environment variable or returns a default value
func getBoolEnv(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return defaultValue
		}
		return b
	}
	return defaultValue
}

// getDurationEnv retrieves a duration environment variable or returns a default value
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		d, err := time.ParseDuration(value)
		if err != nil {
			return defaultValue
		}
		return d
	}
	return defaultValue
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.RedfishHost == "" {
		return fmt.Errorf("REDFISH_HOST must be set")
	}
	if c.RedfishUsername == "" {
		return fmt.Errorf("REDFISH_USERNAME must be set")
	}
	if c.RedfishPassword == "" {
		return fmt.Errorf("REDFISH_PASSWORD must be set")
	}
	return nil
}
