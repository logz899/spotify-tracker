package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	Development = "development"
	Production  = "production"
)

type Config struct {
	AppEnv     string
	GinMode    string
	ServerPort string

	FrontendURL        string
	CORSAllowedOrigins []string

	SpotifyClientID     string
	SpotifyClientSecret string
	SpotifyRedirectURI  string

	DatabaseURL        string
	JWTSecret          []byte
	JWTIssuer          string
	JWTAudience        string
	TokenEncryptionKey []byte
}

// Load reads configuration from the process environment.
func Load() (*Config, error) {
	return LoadFrom(os.LookupEnv)
}

// LoadFrom reads configuration through an injected lookup function so callers
// can validate startup behavior without mutating the process environment.
func LoadFrom(lookup func(string) (string, bool)) (*Config, error) {
	if lookup == nil {
		return nil, errors.New("environment lookup is required")
	}

	value := func(key, fallback string) string {
		if raw, ok := lookup(key); ok && strings.TrimSpace(raw) != "" {
			return strings.TrimSpace(raw)
		}
		return fallback
	}

	encodedKey := value("TOKEN_ENCRYPTION_KEY", "")
	var encryptionKey []byte
	if encodedKey != "" {
		decoded, err := base64.StdEncoding.DecodeString(encodedKey)
		if err != nil {
			return nil, fmt.Errorf("TOKEN_ENCRYPTION_KEY must be valid base64: %w", err)
		}
		encryptionKey = decoded
	}

	cfg := &Config{
		AppEnv:              strings.ToLower(value("APP_ENV", Development)),
		GinMode:             strings.ToLower(value("GIN_MODE", "debug")),
		ServerPort:          value("SERVER_PORT", "8000"),
		FrontendURL:         value("FRONTEND_URL", "http://localhost:3000"),
		CORSAllowedOrigins:  splitCommaSeparated(value("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		SpotifyClientID:     value("SPOTIFY_CLIENT_ID", ""),
		SpotifyClientSecret: value("SPOTIFY_CLIENT_SECRET", ""),
		SpotifyRedirectURI:  value("SPOTIFY_REDIRECT_URI", ""),
		DatabaseURL:         value("DATABASE_URL", ""),
		JWTSecret:           []byte(value("JWT_SECRET", "")),
		JWTIssuer:           value("JWT_ISSUER", ""),
		JWTAudience:         value("JWT_AUDIENCE", ""),
		TokenEncryptionKey:  encryptionKey,
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate rejects configuration that would make authentication, token
// storage, browser access, or production database transport unsafe.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("configuration is required")
	}
	if c.AppEnv != Development && c.AppEnv != Production && c.AppEnv != "test" {
		return fmt.Errorf("APP_ENV must be %q, %q, or %q", Development, Production, "test")
	}
	port, err := strconv.Atoi(c.ServerPort)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("SERVER_PORT must be a valid TCP port")
	}

	required := []struct {
		name  string
		value string
	}{
		{"FRONTEND_URL", c.FrontendURL},
		{"SPOTIFY_CLIENT_ID", c.SpotifyClientID},
		{"SPOTIFY_CLIENT_SECRET", c.SpotifyClientSecret},
		{"SPOTIFY_REDIRECT_URI", c.SpotifyRedirectURI},
		{"DATABASE_URL", c.DatabaseURL},
		{"JWT_ISSUER", c.JWTIssuer},
		{"JWT_AUDIENCE", c.JWTAudience},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 bytes")
	}
	if len(c.TokenEncryptionKey) != 32 {
		return errors.New("TOKEN_ENCRYPTION_KEY must decode to exactly 32 bytes")
	}
	if len(c.CORSAllowedOrigins) == 0 {
		return errors.New("CORS_ALLOWED_ORIGINS must contain at least one origin")
	}

	if _, err := url.ParseRequestURI(c.FrontendURL); err != nil {
		return fmt.Errorf("FRONTEND_URL is invalid: %w", err)
	}
	if _, err := url.ParseRequestURI(c.SpotifyRedirectURI); err != nil {
		return fmt.Errorf("SPOTIFY_REDIRECT_URI is invalid: %w", err)
	}
	databaseURL, err := url.Parse(c.DatabaseURL)
	if err != nil || databaseURL.Host == "" || (databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql") {
		return errors.New("DATABASE_URL must be a valid PostgreSQL URL")
	}

	for _, origin := range c.CORSAllowedOrigins {
		if origin == "*" || strings.Contains(origin, "*") {
			return errors.New("CORS wildcard origins are forbidden")
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" {
			return fmt.Errorf("invalid CORS origin %q", origin)
		}
	}

	if c.AppEnv == Production {
		if c.GinMode != "release" {
			return errors.New("GIN_MODE must be release in production")
		}
		sslMode := databaseURL.Query().Get("sslmode")
		if sslMode != "require" && sslMode != "verify-ca" && sslMode != "verify-full" {
			return errors.New("DATABASE_URL must require TLS in production")
		}
	}

	return nil
}

func splitCommaSeparated(raw string) []string {
	var values []string
	for _, item := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
