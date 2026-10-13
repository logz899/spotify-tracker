package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"spotify-backend/internal/security"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
)

func TestProfileMigrationUpPreservesLegacyData(t *testing.T) {
	databaseURL := newEphemeralMigrationDatabase(t)
	migrator := newProfileMigrator(t, databaseURL)
	if err := migrator.Migrate(6); err != nil {
		t.Fatalf("migrate to version 6: %v", err)
	}

	conn := connectMigrationDatabase(t, databaseURL)
	seed := seedLegacySpotifyData(t, conn)
	if _, err := conn.Exec(context.Background(),
		`INSERT INTO pending_spotify_tokens
		    (key_hash, access_token, refresh_token, token_type, token_expiry, expires_at, spotify_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		make([]byte, 32), "pending-access", "pending-refresh", "Bearer", time.Now().Add(time.Hour), time.Now().Add(time.Minute), "pending-legacy",
	); err != nil {
		t.Fatalf("seed pending Spotify token: %v", err)
	}

	if err := migrator.Steps(1); err != nil {
		t.Fatalf("migrate to version 7: %v", err)
	}

	assertSpotifyRowsPreserved(t, conn, seed)
	if got := primaryKeyColumn(t, conn, "spotify_accounts"); got != "user_id" {
		t.Fatalf("spotify_accounts primary key = %q, want user_id", got)
	}
	for _, column := range []string{"account_id", "display_name", "avatar_url", "profile_url"} {
		if !columnExists(t, conn, "spotify_accounts", column) {
			t.Fatalf("spotify_accounts.%s does not exist", column)
		}
		if !columnExists(t, conn, "pending_spotify_tokens", column) {
			t.Fatalf("pending_spotify_tokens.%s does not exist", column)
		}
	}
	var pendingCount int
	if err := conn.QueryRow(context.Background(), `SELECT count(*) FROM pending_spotify_tokens`).Scan(&pendingCount); err != nil {
		t.Fatalf("count pending Spotify tokens: %v", err)
	}
	if pendingCount != 0 {
		t.Fatalf("pending Spotify token count = %d, want 0", pendingCount)
	}
}

func TestProfileMigrationDownPreservesCompatibleData(t *testing.T) {
	databaseURL := newEphemeralMigrationDatabase(t)
	migrator := newProfileMigrator(t, databaseURL)
	if err := migrator.Migrate(7); err != nil {
		t.Fatalf("migrate to version 7: %v", err)
	}

	conn := connectMigrationDatabase(t, databaseURL)
	seed := seedLegacySpotifyData(t, conn)
	if _, err := conn.Exec(context.Background(),
		`UPDATE spotify_accounts
		 SET account_id = $1, display_name = $2, avatar_url = $3, profile_url = $4
		 WHERE user_id = $5`,
		"account-migration", "Migration Listener", "https://i.scdn.co/image/migration", "https://open.spotify.com/user/migration", seed.userID,
	); err != nil {
		t.Fatalf("add compatible profile data: %v", err)
	}

	if err := migrator.Steps(-1); err != nil {
		t.Fatalf("migrate down to version 6: %v", err)
	}

	assertSpotifyRowsPreserved(t, conn, seed)
	if got := primaryKeyColumn(t, conn, "spotify_accounts"); got != "spotify_user_id" {
		t.Fatalf("spotify_accounts primary key = %q, want spotify_user_id", got)
	}
	for _, column := range []string{"account_id", "display_name", "avatar_url", "profile_url"} {
		if columnExists(t, conn, "spotify_accounts", column) {
			t.Fatalf("spotify_accounts.%s still exists after down migration", column)
		}
	}
}

type migrationSeed struct {
	userID        int
	spotifyUserID string
	accessToken   string
}

func newEphemeralMigrationDatabase(t *testing.T) string {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	_, hash, err := security.NewOpaqueToken()
	if err != nil {
		t.Fatalf("create database suffix: %v", err)
	}
	databaseName := fmt.Sprintf("spotify_profile_%x", hash[:8])
	identifier := pgx.Identifier{databaseName}.Sanitize()

	admin, err := pgx.Connect(context.Background(), adminURL)
	if err != nil {
		t.Fatalf("connect to migration admin database: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "CREATE DATABASE "+identifier); err != nil {
		admin.Close(context.Background())
		t.Fatalf("create ephemeral database: %v", err)
	}
	if err := admin.Close(context.Background()); err != nil {
		t.Fatalf("close migration admin connection: %v", err)
	}

	target := *parsed
	target.Path = "/" + databaseName
	targetURL := target.String()
	t.Cleanup(func() {
		cleanup, cleanupErr := pgx.Connect(context.Background(), adminURL)
		if cleanupErr != nil {
			t.Errorf("connect for ephemeral database cleanup: %v", cleanupErr)
			return
		}
		defer cleanup.Close(context.Background())
		if _, cleanupErr = cleanup.Exec(context.Background(), "DROP DATABASE IF EXISTS "+identifier+" WITH (FORCE)"); cleanupErr != nil {
			t.Errorf("drop ephemeral database %s: %v", databaseName, cleanupErr)
		}
	})
	return targetURL
}

func newProfileMigrator(t *testing.T, databaseURL string) *migrate.Migrate {
	t.Helper()
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse migration database URL: %v", err)
	}
	parsed.Scheme = "pgx5"
	migrationsPath, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatalf("resolve migrations path: %v", err)
	}
	migrator, err := migrate.New((&url.URL{Scheme: "file", Path: migrationsPath}).String(), parsed.String())
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	t.Cleanup(func() {
		sourceErr, databaseErr := migrator.Close()
		if sourceErr != nil {
			t.Errorf("close migration source: %v", sourceErr)
		}
		if databaseErr != nil {
			t.Errorf("close migration database: %v", databaseErr)
		}
	})
	return migrator
}

func connectMigrationDatabase(t *testing.T, databaseURL string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect to ephemeral database: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Errorf("close ephemeral database connection: %v", err)
		}
	})
	return conn
}

func seedLegacySpotifyData(t *testing.T, conn *pgx.Conn) migrationSeed {
	t.Helper()
	ctx := context.Background()
	seed := migrationSeed{spotifyUserID: "legacy-migration-user", accessToken: "encrypted-access"}
	if err := conn.QueryRow(ctx,
		`INSERT INTO users (username, email, password_hash)
		 VALUES ('migration-user', 'migration@example.test', 'unused-hash') RETURNING id`,
	).Scan(&seed.userID); err != nil {
		t.Fatalf("seed migration user: %v", err)
	}
	if _, err := conn.Exec(ctx,
		`INSERT INTO spotify_accounts (spotify_user_id, user_id) VALUES ($1, $2)`,
		seed.spotifyUserID, seed.userID,
	); err != nil {
		t.Fatalf("seed legacy Spotify account: %v", err)
	}
	if _, err := conn.Exec(ctx,
		`INSERT INTO spotify_tokens (user_id, access_token, refresh_token, token_type, expiry)
		 VALUES ($1, $2, $3, 'Bearer', $4)`,
		seed.userID, seed.accessToken, "encrypted-refresh", time.Now().Add(time.Hour),
	); err != nil {
		t.Fatalf("seed Spotify token: %v", err)
	}
	return seed
}

func assertSpotifyRowsPreserved(t *testing.T, conn *pgx.Conn, seed migrationSeed) {
	t.Helper()
	ctx := context.Background()
	var username, spotifyUserID, accessToken string
	if err := conn.QueryRow(ctx,
		`SELECT u.username, sa.spotify_user_id, st.access_token
		 FROM users u
		 JOIN spotify_accounts sa ON sa.user_id = u.id
		 JOIN spotify_tokens st ON st.user_id = u.id
		 WHERE u.id = $1`, seed.userID,
	).Scan(&username, &spotifyUserID, &accessToken); err != nil {
		t.Fatalf("query preserved migration data: %v", err)
	}
	if username != "migration-user" || spotifyUserID != seed.spotifyUserID || accessToken != seed.accessToken {
		t.Fatalf("preserved rows = (%q, %q, %q), want migration-user/%q/%q", username, spotifyUserID, accessToken, seed.spotifyUserID, seed.accessToken)
	}
}

func primaryKeyColumn(t *testing.T, conn *pgx.Conn, table string) string {
	t.Helper()
	var column string
	err := conn.QueryRow(context.Background(),
		`SELECT a.attname
		 FROM pg_index i
		 JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		 WHERE i.indrelid = $1::regclass AND i.indisprimary`, table,
	).Scan(&column)
	if err != nil {
		t.Fatalf("query %s primary key: %v", table, err)
	}
	return column
}

func columnExists(t *testing.T, conn *pgx.Conn, table, column string) bool {
	t.Helper()
	var exists bool
	err := conn.QueryRow(context.Background(),
		`SELECT EXISTS (
		    SELECT 1 FROM information_schema.columns
		    WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2
		)`, table, column,
	).Scan(&exists)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("query column %s.%s: %v", table, column, err)
	}
	return exists
}
