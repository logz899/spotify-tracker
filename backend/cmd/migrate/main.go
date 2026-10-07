package main

import (
	"errors"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	dsn, err := migrationURL(os.LookupEnv)
	if err != nil {
		log.Fatalf("Invalid database configuration: %v", err)
	}
	migrationsPath := getEnv("MIGRATIONS_PATH", "file://migrations")

	log.Printf("Running migrations from: %s", migrationsPath)

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		log.Fatalf("Failed to create migrator: %v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("Warning: source close error: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("Warning: database close error: %v", dbErr)
		}
	}()

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Migration up failed: %v", err)
		}
		v, _, _ := m.Version()
		log.Printf("Migrations applied successfully (version: %d)", v)

	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Migration down failed: %v", err)
		}
		log.Println("All migrations rolled back")

	case "steps":
		if len(os.Args) < 3 {
			log.Fatal("Usage: migrate steps <n>  (positive = up, negative = down)")
		}
		n, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("Invalid steps value: %v", err)
		}
		if err := m.Steps(n); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Migration steps failed: %v", err)
		}
		log.Println("Migration steps completed successfully")

	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("Failed to get version: %v", err)
		}
		log.Printf("Current version: %d  dirty: %v", v, dirty)

	case "force":
		if len(os.Args) < 3 {
			log.Fatal("Usage: migrate force <version>")
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("Invalid version value: %v", err)
		}
		if err := m.Force(v); err != nil {
			log.Fatalf("Force version failed: %v", err)
		}
		log.Println("Migration version forced successfully")

	default:
		log.Fatal("Unknown migration command (valid: up | down | steps <n> | version | force <v>)")
	}
}

func migrationURL(lookup func(string) (string, bool)) (string, error) {
	value := func(key string) string {
		raw, _ := lookup(key)
		return strings.TrimSpace(raw)
	}

	rawURL := value("DATABASE_URL")
	if rawURL == "" {
		return "", errors.New("DATABASE_URL is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("DATABASE_URL must be a valid PostgreSQL URL")
	}
	if (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		return "", errors.New("DATABASE_URL must be a valid PostgreSQL URL")
	}

	if strings.ToLower(value("APP_ENV")) == "production" {
		switch parsed.Query().Get("sslmode") {
		case "require", "verify-ca", "verify-full":
		default:
			return "", errors.New("DATABASE_URL must require TLS in production")
		}
	}

	return "pgx5" + rawURL[len(parsed.Scheme):], nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
