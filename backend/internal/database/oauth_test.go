package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"spotify-backend/internal/models"
	"spotify-backend/internal/security"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var (
	migrateOnce sync.Once
	migrateErr  error
)

func TestOAuthStateConsumeRejectsExpiryReplayAndWrongHash(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, db)
	now := time.Now().UTC()

	_, validHash, err := security.NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	if err := db.CreateOAuthState(ctx, validHash[:], &userID, now.Add(time.Minute)); err != nil {
		t.Fatalf("CreateOAuthState() error = %v", err)
	}
	consumedUserID, err := db.ConsumeOAuthState(ctx, validHash[:], now)
	if err != nil {
		t.Fatalf("ConsumeOAuthState() error = %v", err)
	}
	if consumedUserID == nil || *consumedUserID != userID {
		t.Fatalf("ConsumeOAuthState() user = %v, want %d", consumedUserID, userID)
	}
	if _, err := db.ConsumeOAuthState(ctx, validHash[:], now); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("replay error = %v, want ErrOAuthStateInvalid", err)
	}

	_, wrongHash, _ := security.NewOpaqueToken()
	if _, err := db.ConsumeOAuthState(ctx, wrongHash[:], now); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("wrong hash error = %v, want ErrOAuthStateInvalid", err)
	}

	_, expiredHash, _ := security.NewOpaqueToken()
	if err := db.CreateOAuthState(ctx, expiredHash[:], nil, now.Add(-time.Second)); err != nil {
		t.Fatalf("CreateOAuthState(expired) error = %v", err)
	}
	if _, err := db.ConsumeOAuthState(ctx, expiredHash[:], now); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("expired error = %v, want ErrOAuthStateInvalid", err)
	}
}

func TestOAuthStateConcurrentConsumeAllowsExactlyOne(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	_, hash, _ := security.NewOpaqueToken()
	if err := db.CreateOAuthState(ctx, hash[:], nil, now.Add(time.Minute)); err != nil {
		t.Fatalf("CreateOAuthState() error = %v", err)
	}

	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := db.ConsumeOAuthState(ctx, hash[:], now)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	assertExactlyOneSuccess(t, results, ErrOAuthStateInvalid)
}

func TestPendingSpotifyTokenEncryptsAndConsumesOnce(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, db)
	now := time.Now().UTC()
	_, hash, _ := security.NewOpaqueToken()
	token := models.SpotifyToken{
		AccessToken:  "access-plaintext",
		RefreshToken: "refresh-plaintext",
		TokenType:    "Bearer",
		Expiry:       now.Add(time.Hour),
	}
	if err := db.CreatePendingSpotifyToken(ctx, hash[:], token, now.Add(time.Minute)); err != nil {
		t.Fatalf("CreatePendingSpotifyToken() error = %v", err)
	}

	var storedAccess, storedRefresh string
	if err := db.pool.QueryRow(ctx, `SELECT access_token, refresh_token FROM pending_spotify_tokens WHERE key_hash = $1`, hash[:]).Scan(&storedAccess, &storedRefresh); err != nil {
		t.Fatalf("query pending ciphertext: %v", err)
	}
	if storedAccess == token.AccessToken || storedRefresh == token.RefreshToken || !strings.HasPrefix(storedAccess, "v1.") || !strings.HasPrefix(storedRefresh, "v1.") {
		t.Fatal("pending Spotify token was not stored as v1 ciphertext")
	}
	_, wrongHash, _ := security.NewOpaqueToken()
	if err := db.ConsumePendingSpotifyToken(ctx, wrongHash[:], userID, now); !errors.Is(err, ErrPendingSpotifyTokenInvalid) {
		t.Fatalf("wrong hash error = %v, want ErrPendingSpotifyTokenInvalid", err)
	}

	if err := db.ConsumePendingSpotifyToken(ctx, hash[:], userID, now); err != nil {
		t.Fatalf("ConsumePendingSpotifyToken() error = %v", err)
	}
	stored, err := db.GetSpotifyToken(ctx, userID)
	if err != nil {
		t.Fatalf("GetSpotifyToken() error = %v", err)
	}
	if stored.AccessToken != token.AccessToken || stored.RefreshToken != token.RefreshToken {
		t.Fatal("consumed Spotify token did not decrypt to the original values")
	}
	if err := db.ConsumePendingSpotifyToken(ctx, hash[:], userID, now); !errors.Is(err, ErrPendingSpotifyTokenInvalid) {
		t.Fatalf("replay error = %v, want ErrPendingSpotifyTokenInvalid", err)
	}
}

func TestPendingSpotifyTokenRejectsExpiryAndConcurrentReplay(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, db)
	now := time.Now().UTC()
	token := models.SpotifyToken{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", Expiry: now.Add(time.Hour)}

	_, expiredHash, _ := security.NewOpaqueToken()
	if err := db.CreatePendingSpotifyToken(ctx, expiredHash[:], token, now.Add(-time.Second)); err != nil {
		t.Fatalf("CreatePendingSpotifyToken(expired) error = %v", err)
	}
	if err := db.ConsumePendingSpotifyToken(ctx, expiredHash[:], userID, now); !errors.Is(err, ErrPendingSpotifyTokenInvalid) {
		t.Fatalf("expired error = %v, want ErrPendingSpotifyTokenInvalid", err)
	}

	_, hash, _ := security.NewOpaqueToken()
	if err := db.CreatePendingSpotifyToken(ctx, hash[:], token, now.Add(time.Minute)); err != nil {
		t.Fatalf("CreatePendingSpotifyToken() error = %v", err)
	}
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- db.ConsumePendingSpotifyToken(ctx, hash[:], userID, now)
		}()
	}
	wait.Wait()
	close(results)
	assertExactlyOneSuccess(t, results, ErrPendingSpotifyTokenInvalid)
}

func TestOAuthCleanupDeletesOnlyExpiredSessions(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	token := models.SpotifyToken{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", Expiry: now.Add(time.Hour)}
	if _, err := db.DeleteExpiredOAuthSessions(ctx, now); err != nil {
		t.Fatalf("initial OAuth cleanup: %v", err)
	}

	_, expiredState, _ := security.NewOpaqueToken()
	_, liveState, _ := security.NewOpaqueToken()
	_, expiredPending, _ := security.NewOpaqueToken()
	_, livePending, _ := security.NewOpaqueToken()
	if err := db.CreateOAuthState(ctx, expiredState[:], nil, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateOAuthState(ctx, liveState[:], nil, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePendingSpotifyToken(ctx, expiredPending[:], token, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePendingSpotifyToken(ctx, livePending[:], token, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	deleted, err := db.DeleteExpiredOAuthSessions(ctx, now)
	if err != nil {
		t.Fatalf("DeleteExpiredOAuthSessions() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted rows = %d, want 2", deleted)
	}
	for table, hash := range map[string][]byte{
		"oauth_states":           liveState[:],
		"pending_spotify_tokens": livePending[:],
	} {
		var count int
		query := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s = $1", table, map[string]string{"oauth_states": "state_hash", "pending_spotify_tokens": "key_hash"}[table])
		if err := db.pool.QueryRow(ctx, query, hash).Scan(&count); err != nil {
			t.Fatalf("query live %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("live %s count = %d, want 1", table, count)
		}
	}
}

func newIntegrationDB(t *testing.T) *DB {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	migrateOnce.Do(func() { migrateErr = applyTestMigrations(databaseURL) })
	if migrateErr != nil {
		t.Fatalf("apply migrations: %v", migrateErr)
	}
	cipher, err := security.NewTokenCipher([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("NewTokenCipher() error = %v", err)
	}
	db, err := New(databaseURL, cipher)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return db
}

func applyTestMigrations(databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return err
	}
	parsed.Scheme = "pgx5"
	migrationsPath, err := filepath.Abs("../../migrations")
	if err != nil {
		return err
	}
	migrator, err := migrate.New((&url.URL{Scheme: "file", Path: migrationsPath}).String(), parsed.String())
	if err != nil {
		return err
	}
	defer migrator.Close()
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func createTestUser(t *testing.T, db *DB) int {
	t.Helper()
	raw, _, err := security.NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	var id int
	err = db.pool.QueryRow(context.Background(),
		`INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		"test-"+raw, fmt.Sprintf("%s@example.test", raw), "unused-test-hash",
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return id
}

func assertExactlyOneSuccess(t *testing.T, results <-chan error, expectedFailure error) {
	t.Helper()
	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, expectedFailure) {
			failures++
		} else {
			t.Fatalf("unexpected consume error: %v", err)
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("consume results = %d success/%d expected failure, want 1/1", successes, failures)
	}
}
