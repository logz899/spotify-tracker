package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadFromValidDevelopmentConfig(t *testing.T) {
	values := validValues("development")
	values["GIN_MODE"] = "debug"
	values["DATABASE_URL"] = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	values["FRONTEND_URL"] = "http://localhost:3000"
	values["CORS_ALLOWED_ORIGINS"] = "http://localhost:3000"
	values["SPOTIFY_REDIRECT_URI"] = "http://127.0.0.1:8000/api/v1/auth/spotify/callback"

	cfg, err := LoadFrom(mapLookup(values))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if cfg.ServerPort != "8000" {
		t.Fatalf("ServerPort = %q, want 8000", cfg.ServerPort)
	}
	if len(cfg.TokenEncryptionKey) != 32 {
		t.Fatalf("decoded token key length = %d, want 32", len(cfg.TokenEncryptionKey))
	}
}

func TestLoadFromValidProductionConfig(t *testing.T) {
	if _, err := LoadFrom(mapLookup(validValues("production"))); err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
}

func TestProductionRejectsUnsafeConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{"missing Spotify secret", func(v map[string]string) { delete(v, "SPOTIFY_CLIENT_SECRET") }, "SPOTIFY_CLIENT_SECRET"},
		{"missing JWT secret", func(v map[string]string) { delete(v, "JWT_SECRET") }, "JWT_SECRET"},
		{"missing JWT issuer", func(v map[string]string) { delete(v, "JWT_ISSUER") }, "JWT_ISSUER"},
		{"missing JWT audience", func(v map[string]string) { delete(v, "JWT_AUDIENCE") }, "JWT_AUDIENCE"},
		{"missing database URL", func(v map[string]string) { delete(v, "DATABASE_URL") }, "DATABASE_URL"},
		{"missing encryption key", func(v map[string]string) { delete(v, "TOKEN_ENCRYPTION_KEY") }, "TOKEN_ENCRYPTION_KEY"},
		{"wildcard CORS", func(v map[string]string) { v["CORS_ALLOWED_ORIGINS"] = "*" }, "wildcard"},
		{"debug Gin mode", func(v map[string]string) { v["GIN_MODE"] = "debug" }, "GIN_MODE"},
		{"database without TLS", func(v map[string]string) { v["DATABASE_URL"] = "postgres://user:pass@db.example/app?sslmode=disable" }, "TLS"},
		{"malformed encryption key", func(v map[string]string) { v["TOKEN_ENCRYPTION_KEY"] = "not-base64" }, "base64"},
		{"short encryption key", func(v map[string]string) {
			v["TOKEN_ENCRYPTION_KEY"] = base64.StdEncoding.EncodeToString([]byte("short"))
		}, "32 bytes"},
		{"short JWT secret", func(v map[string]string) { v["JWT_SECRET"] = "too-short" }, "32 bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := validValues("production")
			tt.mutate(values)
			_, err := LoadFrom(mapLookup(values))
			if err == nil {
				t.Fatal("LoadFrom() error = nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadFrom() error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func validValues(environment string) map[string]string {
	return map[string]string{
		"APP_ENV":               environment,
		"GIN_MODE":              "release",
		"SERVER_PORT":           "8000",
		"FRONTEND_URL":          "https://example.github.io",
		"CORS_ALLOWED_ORIGINS":  "https://example.github.io",
		"SPOTIFY_CLIENT_ID":     "test-client-id",
		"SPOTIFY_CLIENT_SECRET": strings.Repeat("s", 32),
		"SPOTIFY_REDIRECT_URI":  "https://api.example.test/api/v1/auth/spotify/callback",
		"DATABASE_URL":          "postgres://user:pass@db.example/app?sslmode=require",
		"JWT_SECRET":            strings.Repeat("j", 32),
		"JWT_ISSUER":            "spotify-song-rank",
		"JWT_AUDIENCE":          "spotify-song-rank-web",
		"TOKEN_ENCRYPTION_KEY":  base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))),
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
