package main

import (
	"strings"
	"testing"
)

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestMigrationURLConvertsPooledNeonURL(t *testing.T) {
	got, err := migrationURL(lookupFrom(map[string]string{
		"APP_ENV":      "production",
		"DATABASE_URL": "postgresql://app_user:secret@ep-quiet-sky-123-pooler.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require",
	}))
	if err != nil {
		t.Fatalf("migrationURL returned error: %v", err)
	}

	want := "pgx5://app_user:secret@ep-quiet-sky-123-pooler.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require"
	if got != want {
		t.Fatalf("migrationURL = %q, want %q", got, want)
	}
}

func TestMigrationURLPreservesPercentEncodedCredentials(t *testing.T) {
	got, err := migrationURL(lookupFrom(map[string]string{
		"APP_ENV":      "production",
		"DATABASE_URL": "postgres://user%40team:p%40ss%2Fw%3Ard@db.example.test:5432/app?sslmode=verify-full",
	}))
	if err != nil {
		t.Fatalf("migrationURL returned error: %v", err)
	}

	want := "pgx5://user%40team:p%40ss%2Fw%3Ard@db.example.test:5432/app?sslmode=verify-full"
	if got != want {
		t.Fatalf("migrationURL = %q, want %q", got, want)
	}
}

func TestMigrationURLRejectsProductionWithoutTLS(t *testing.T) {
	for name, rawURL := range map[string]string{
		"disabled": "postgres://user:secret@db.example.test/app?sslmode=disable",
		"missing":  "postgres://user:secret@db.example.test/app",
		"prefer":   "postgres://user:secret@db.example.test/app?sslmode=prefer",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := migrationURL(lookupFrom(map[string]string{
				"APP_ENV":      "production",
				"DATABASE_URL": rawURL,
			}))
			if err == nil {
				t.Fatal("expected production URL without TLS to be rejected")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaks credentials: %v", err)
			}
		})
	}
}

func TestMigrationURLAllowsLocalDevelopmentWithoutTLS(t *testing.T) {
	got, err := migrationURL(lookupFrom(map[string]string{
		"DATABASE_URL": "postgres://postgres:postgres@db:5432/postgres?sslmode=disable",
	}))
	if err != nil {
		t.Fatalf("migrationURL returned error: %v", err)
	}
	if got != "pgx5://postgres:postgres@db:5432/postgres?sslmode=disable" {
		t.Fatalf("migrationURL = %q", got)
	}
}

func TestMigrationURLRejectsMissingOrMalformedURL(t *testing.T) {
	for name, rawURL := range map[string]string{
		"missing":      "",
		"wrong scheme": "mysql://user:secret@db.example.test/app?sslmode=require",
		"no host":      "postgres:///app?sslmode=require",
		"unparseable":  "postgres://user:secret@db.example.test:bad port/app",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := migrationURL(lookupFrom(map[string]string{
				"APP_ENV":      "production",
				"DATABASE_URL": rawURL,
			}))
			if err == nil {
				t.Fatal("expected invalid DATABASE_URL to be rejected")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaks credentials: %v", err)
			}
		})
	}
}
