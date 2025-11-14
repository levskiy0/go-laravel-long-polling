package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Laravel configuration
	LaravelAddr string

	// HTTP server configuration
	HTTPAddr         string
	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration

	// JWT configuration
	JWTSecret    string
	JWTExpiresIn int
	JWTAlgo      string

	// Redis configuration
	RedisAddr     string
	RedisDB       int
	RedisPassword string
	RedisChannel  string

	// Long-polling configuration
	PollTimeout time.Duration

	// Access token secret
	AccessTokenSecret string

	// Logging configuration
	LogLevel  string
	LogFormat string

	// Laravel upstream pool configuration
	LaravelUpstreamWorkers int
	MaxLimit               int

	// CORS configuration
	CORSAllowedOrigins  string
	CORSAllowedMethods  string
	CORSAllowedHeaders  string
	CORSAllowCredentials bool
	CORSMaxAge          int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		LaravelAddr:            getEnv("LARAVEL_ADDR", "http://localhost:8000"),
		HTTPAddr:               getEnv("HTTP_ADDR", ":8085"),
		HTTPReadTimeout:        getDurationEnv("HTTP_READ_TIMEOUT", 30*time.Second),
		HTTPWriteTimeout:       getDurationEnv("HTTP_WRITE_TIMEOUT", 30*time.Second),
		JWTSecret:              getEnv("JWT_SECRET", "super_long_random_secret"),
		JWTExpiresIn:           getIntEnv("JWT_EXPIRES_IN", 3600),
		JWTAlgo:                getEnv("JWT_ALGO", "HS256"),
		RedisAddr:              getEnv("REDIS_ADDR", "redis:6379"),
		RedisDB:                getIntEnv("REDIS_DB", 0),
		RedisPassword:          getEnv("REDIS_PASSWORD", ""),
		RedisChannel:           getEnv("REDIS_CHANNEL", "longpoll:events"),
		PollTimeout:            getDurationEnv("POLL_TIMEOUT", 25*time.Second),
		AccessTokenSecret:      getEnv("ACCESS_TOKEN_SECRET", "shared_secret_between_laravel_and_go"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		LogFormat:              getEnv("LOG_FORMAT", "json"),
		LaravelUpstreamWorkers: getIntEnv("LARAVEL_UPSTREAM_WORKERS", 15),
		MaxLimit:               getIntEnv("MAX_LIMIT", 100),
		CORSAllowedOrigins:     getEnv("CORS_ALLOWED_ORIGINS", "*"),
		CORSAllowedMethods:     getEnv("CORS_ALLOWED_METHODS", "GET,POST,PUT,DELETE,OPTIONS"),
		CORSAllowedHeaders:     getEnv("CORS_ALLOWED_HEADERS", "Content-Type,Authorization,X-Requested-With"),
		CORSAllowCredentials:   getBoolEnv("CORS_ALLOW_CREDENTIALS", true),
		CORSMaxAge:             getIntEnv("CORS_MAX_AGE", 3600),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// validate checks if the configuration is valid
func (c *Config) validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.AccessTokenSecret == "" {
		return fmt.Errorf("ACCESS_TOKEN_SECRET is required")
	}
	if c.LaravelUpstreamWorkers < 1 {
		return fmt.Errorf("LARAVEL_UPSTREAM_WORKERS must be at least 1")
	}
	if c.MaxLimit < 1 || c.MaxLimit > 1000 {
		return fmt.Errorf("MAX_LIMIT must be between 1 and 1000")
	}
	return nil
}

// GetLogLevel returns the slog.Level based on the configured log level
func (c *Config) GetLogLevel() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
