// Package config manages the application's configuration by parsing environment
// variables into a strongly typed struct.
package config

import (
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config represents the application's complete configuration state.
type Config struct {
	// --- Idonia API Configuration ---
	IdoniaAPIUrl    string `env:"IDONIA_API_URL"`
	IdoniaAPIKey    string `env:"IDONIA_APIKEY,required"`
	IdoniaAPISecret string `env:"IDONIA_SECRET"`
	HackathonRef    string `env:"IDONIA_HACKATHON_REF"`
	// HackathonMagicRef is the reference of the magic link generated
	// It is not used because the API gives the same value as response
	HackathonMagicRef string `env:"IDONIA_MAGIC_REF"`

	// --- Recognition API configuration ---
	RecogAPIUrl string `env:"RECOG_API_URL"`
	RecogAPIKey string `env:"RECOG_APIKEY,required"`

	// --- Server and security configuration ---
	// Port defines the port the HTTP server binds to (default: 8080).
	Port string `env:"PORT" envDefault:"8080"`
	// Env specifies the application environment (defaults to "development").
	Env string `env:"ENV" envDefault:"development"`
	// CSRFKey is the secret used for signing Cross-Site Request Forgery tokens.
	CSRFKey string `env:"CSRF_KEY"`
	// Hosts defines the allowed CORS origins or bound hostnames, parsed from a comma-separated list.
	Hosts []string `env:"HOSTS" envSeparator:","`

	// --- Idempotency configuration ---
	RedisURL string        `env:"REDIS_URL" envDefault:"redis://redis:6379"`
	RedisTTL time.Duration `env:"REDIS_TTL" envDefault:"24h"`

	// --- Logging configuration ---
	// LogLevel sets the minimum log severity
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
	// LogFormat determines the output structure of the logs (e.g., "json", "text").
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`
	// LogRedactKeys is a comma-separated list of sensitive payload keys to mask in the logs.
	LogRedactKeys string `env:"LOG_REDACT_KEYS"`

	// --- Tessera configuration ---
	TesseraLogDir     string `env:"TESSERA_LOG_DIR"`
	TesseraPrivateKey string `env:"TESSERA_PRIVATE_KEY"`
}

// LoadConfig initializes a Config struct by reading the current environment variables.
// It automatically applies struct tag rules (like defaults and required checks)
// Returns an error if any required environment variables are missing or malformed.
func LoadConfig() (*Config, error) {
	var cfg Config

	// Parse environment variables directly into the struct based on `env` tags
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	cfg.HackathonRef = normalizeHackathonRef(cfg.HackathonRef)

	return &cfg, nil
}

// normalizeHackathonRef cleans a raw reference string.
// If the input is a full URL or a multi-segment path (e.g., "https://api.com/ref/123" or "/ref/123"),
// it extracts and returns only the final segment ("123").
func normalizeHackathonRef(raw string) string {
	ref := strings.TrimSpace(raw)
	if ref == "" {
		return ""
	}

	// If the string represents a URL, parse it and isolate the path
	if strings.Contains(ref, "://") {
		if parsed, err := url.Parse(ref); err == nil {
			ref = parsed.Path
		}
	}

	// Strip trailing and leading slashes
	trimmed := strings.Trim(ref, "/")
	if trimmed == "" {
		return ""
	}

	// Split by slash and return the final segment
	parts := strings.Split(trimmed, "/")
	return parts[len(parts)-1]
}
