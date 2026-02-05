package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// Server
	Port string
	Host string

	// Security
	SecretKey     string
	AdminUser     string
	AdminPassword string
	TOTPSecret    string

	// Storage
	DataRoot string
	DBPath   string

	// Logging
	LogLevel string

	// Session
	SessionTimeout int // seconds
}

// Load loads configuration from environment variables
func Load() *Config {
	sessionTimeout := 3600 // default 1 hour
	if st := os.Getenv("SESSION_TIMEOUT"); st != "" {
		if parsed, err := strconv.Atoi(st); err == nil {
			sessionTimeout = parsed
		}
	}

	dataRoot := os.Getenv("APP_DATA_ROOT")
	if dataRoot == "" {
		dataRoot = "/data"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/app/data/hardlink-ui.db"
	}

	return &Config{
		Port:           getEnv("PORT", "8000"),
		Host:           getEnv("HOST", "0.0.0.0"),
		SecretKey:      getEnv("APP_SECRET_KEY", "dev_insecure_key"),
		AdminUser:      os.Getenv("APP_ADMIN_USER"),
		AdminPassword:  os.Getenv("APP_ADMIN_PASSWORD"),
		TOTPSecret:     os.Getenv("APP_TOTP_SECRET"),
		DataRoot:       dataRoot,
		DBPath:         dbPath,
		LogLevel:       getEnv("LOG_LEVEL", "INFO"),
		SessionTimeout: sessionTimeout,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
