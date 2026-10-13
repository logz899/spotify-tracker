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

func TestPendingSpotifyProfilePersistsAndConsumesOnce(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, db)
	now := time.Now().UTC()
	raw, hash, err := security.NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	authorization := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{
			AccountID:    "account-pending-" + raw[:12],
			LegacyUserID: "spotify-pending-" + raw[:12],
			DisplayName:  "Pending Listener",
			AvatarURL:    "https://i.scdn.co/image/pending",
			ProfileURL:   "https://open.spotify.com/user/pending",
		},
		Token: models.SpotifyToken{
			AccessToken:  "pending-access-plaintext",
			RefreshToken: "pending-refresh-plaintext",
			TokenType:    "Bearer",
			Expiry:       now.Add(time.Hour),
		},
	}

	if err := db.CreatePendingSpotifyAuthorization(ctx, hash[:], authorization, now.Add(time.Minute)); err != nil {
		t.Fatalf("CreatePendingSpotifyAuthorization() error = %v", err)
	}
	var accountID, spotifyUserID, displayName, avatarURL, profileURL, storedAccess, storedRefresh string
	if err := db.pool.QueryRow(ctx,
		`SELECT account_id, spotify_user_id, display_name, avatar_url, profile_url, access_token, refresh_token
		 FROM pending_spotify_tokens WHERE key_hash = $1`, hash[:],
	).Scan(&accountID, &spotifyUserID, &displayName, &avatarURL, &profileURL, &storedAccess, &storedRefresh); err != nil {
		t.Fatalf("query pending authorization: %v", err)
	}
	if accountID != authorization.Profile.AccountID || spotifyUserID != authorization.Profile.LegacyUserID ||
		displayName != authorization.Profile.DisplayName || avatarURL != authorization.Profile.AvatarURL ||
		profileURL != authorization.Profile.ProfileURL {
		t.Fatalf("stored profile = (%q, %q, %q, %q, %q), want %#v", accountID, spotifyUserID, displayName, avatarURL, profileURL, authorization.Profile)
	}
	if storedAccess == authorization.Token.AccessToken || storedRefresh == authorization.Token.RefreshToken ||
		!strings.HasPrefix(storedAccess, "v1.") || !strings.HasPrefix(storedRefresh, "v1.") {
		t.Fatal("pending Spotify token was not stored as versioned ciphertext")
	}

	if err := db.ConsumePendingSpotifyAuthorization(ctx, hash[:], userID, now); err != nil {
		t.Fatalf("ConsumePendingSpotifyAuthorization() error = %v", err)
	}
	var linkedUserID int
	if err := db.pool.QueryRow(ctx,
		`SELECT user_id FROM spotify_accounts WHERE account_id = $1`, authorization.Profile.AccountID,
	).Scan(&linkedUserID); err != nil {
		t.Fatalf("query Spotify mapping: %v", err)
	}
	if linkedUserID != userID {
		t.Fatalf("linked user ID = %d, want %d", linkedUserID, userID)
	}
	storedToken, err := db.GetSpotifyToken(ctx, userID)
	if err != nil {
		t.Fatalf("GetSpotifyToken() error = %v", err)
	}
	if storedToken.AccessToken != authorization.Token.AccessToken || storedToken.RefreshToken != authorization.Token.RefreshToken {
		t.Fatal("linked Spotify token did not decrypt to the original values")
	}
	if err := db.ConsumePendingSpotifyAuthorization(ctx, hash[:], userID, now); !errors.Is(err, ErrPendingSpotifyTokenInvalid) {
		t.Fatalf("replay error = %v, want ErrPendingSpotifyTokenInvalid", err)
	}
}

func TestSpotifyIdentityConcurrentClaim(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	firstUserID := createTestUser(t, db)
	secondUserID := createTestUser(t, db)
	now := time.Now().UTC()
	raw, _, err := security.NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	accountID := "account-race-" + raw[:12]

	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index, userID := range []int{firstUserID, secondUserID} {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			authorization := models.SpotifyAuthorization{
				Profile: models.SpotifyProfile{AccountID: accountID, LegacyUserID: fmt.Sprintf("spotify-race-%d-%s", index, raw[:12])},
				Token:   models.SpotifyToken{AccessToken: "race-access", TokenType: "Bearer", Expiry: now.Add(time.Hour)},
			}
			results <- db.LinkSpotifyAccount(ctx, userID, authorization)
		}(index)
	}
	wait.Wait()
	close(results)
	assertExactlyOneSuccess(t, results, ErrSpotifyAccountConflict)

	var linkedUserID int
	if err := db.pool.QueryRow(ctx,
		`SELECT user_id FROM spotify_accounts WHERE account_id = $1`, accountID,
	).Scan(&linkedUserID); err != nil {
		t.Fatalf("query winning mapping: %v", err)
	}
	another := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{AccountID: accountID + "-different", LegacyUserID: "spotify-race-different-" + raw[:12]},
		Token:   models.SpotifyToken{AccessToken: "race-access", TokenType: "Bearer", Expiry: now.Add(time.Hour)},
	}
	if err := db.LinkSpotifyAccount(ctx, linkedUserID, another); !errors.Is(err, ErrSpotifyAccountConflict) {
		t.Fatalf("second identity error = %v, want ErrSpotifyAccountConflict", err)
	}
}

func TestSpotifyIdentityBackfillsMatchingLegacyRow(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, db)
	now := time.Now().UTC()
	legacyID := fmt.Sprintf("legacy-backfill-%d", userID)
	if _, err := db.pool.Exec(ctx,
		`INSERT INTO spotify_accounts (user_id, spotify_user_id) VALUES ($1, $2)`,
		userID, legacyID,
	); err != nil {
		t.Fatalf("seed legacy Spotify account: %v", err)
	}

	authorization := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{
			AccountID:    fmt.Sprintf("account-backfill-%d", userID),
			LegacyUserID: legacyID,
			DisplayName:  "Backfilled Listener",
		},
		Token: models.SpotifyToken{AccessToken: "backfill-access", TokenType: "Bearer", Expiry: now.Add(time.Hour)},
	}
	if err := db.LinkSpotifyAccount(ctx, userID, authorization); err != nil {
		t.Fatalf("LinkSpotifyAccount() error = %v", err)
	}

	var accountID, displayName string
	if err := db.pool.QueryRow(ctx,
		`SELECT account_id, display_name FROM spotify_accounts WHERE user_id = $1`, userID,
	).Scan(&accountID, &displayName); err != nil {
		t.Fatalf("query backfilled Spotify account: %v", err)
	}
	if accountID != authorization.Profile.AccountID || displayName != authorization.Profile.DisplayName {
		t.Fatalf("backfilled profile = (%q, %q), want (%q, %q)", accountID, displayName, authorization.Profile.AccountID, authorization.Profile.DisplayName)
	}
}

func TestSpotifyIdentityDoesNotMoveEstablishedAccount(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	firstUserID := createTestUser(t, db)
	secondUserID := createTestUser(t, db)
	now := time.Now().UTC()
	accountID := fmt.Sprintf("account-established-%d", firstUserID)
	first := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{AccountID: accountID, LegacyUserID: fmt.Sprintf("legacy-established-%d", firstUserID), DisplayName: "Owner"},
		Token:   models.SpotifyToken{AccessToken: "owner-access", TokenType: "Bearer", Expiry: now.Add(time.Hour)},
	}
	if err := db.LinkSpotifyAccount(ctx, firstUserID, first); err != nil {
		t.Fatalf("initial LinkSpotifyAccount() error = %v", err)
	}

	conflict := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{AccountID: accountID, LegacyUserID: fmt.Sprintf("legacy-conflict-%d", secondUserID), DisplayName: "Intruder"},
		Token:   models.SpotifyToken{AccessToken: "intruder-access", TokenType: "Bearer", Expiry: now.Add(time.Hour)},
	}
	if err := db.LinkSpotifyAccount(ctx, secondUserID, conflict); !errors.Is(err, ErrSpotifyAccountConflict) {
		t.Fatalf("conflicting LinkSpotifyAccount() error = %v, want ErrSpotifyAccountConflict", err)
	}

	var linkedUserID int
	var displayName string
	if err := db.pool.QueryRow(ctx,
		`SELECT user_id, display_name FROM spotify_accounts WHERE account_id = $1`, accountID,
	).Scan(&linkedUserID, &displayName); err != nil {
		t.Fatalf("query established Spotify account: %v", err)
	}
	if linkedUserID != firstUserID || displayName != first.Profile.DisplayName {
		t.Fatalf("established account changed to user=%d display=%q", linkedUserID, displayName)
	}
}

func TestSpotifyLoginTicketRejectsExpiryReplayAndConcurrentReplay(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, db)
	now := time.Now().UTC()
	raw, hash, err := security.NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	authorization := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{
			AccountID:    "account-ticket-" + raw[:12],
			LegacyUserID: "spotify-ticket-" + raw[:12],
		},
		Token: models.SpotifyToken{
			AccessToken: "ticket-access", TokenType: "Bearer", Expiry: now.Add(time.Hour),
		},
	}
	if err := db.LinkSpotifyAccount(ctx, userID, authorization); err != nil {
		t.Fatalf("LinkSpotifyAccount() error = %v", err)
	}
	if err := db.CompleteLinkedSpotifyLogin(ctx, authorization, hash[:], now.Add(time.Minute)); err != nil {
		t.Fatalf("CompleteLinkedSpotifyLogin() error = %v", err)
	}

	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			consumedUserID, consumeErr := db.ConsumeSpotifyLoginTicket(ctx, hash[:], now)
			if consumeErr == nil && consumedUserID != userID {
				consumeErr = fmt.Errorf("consumed user ID = %d, want %d", consumedUserID, userID)
			}
			results <- consumeErr
		}()
	}
	wait.Wait()
	close(results)
	assertExactlyOneSuccess(t, results, ErrSpotifyLoginTicketInvalid)

	_, expiredHash, _ := security.NewOpaqueToken()
	if err := db.CompleteLinkedSpotifyLogin(ctx, authorization, expiredHash[:], now.Add(-time.Second)); err != nil {
		t.Fatalf("CompleteLinkedSpotifyLogin(expired) error = %v", err)
	}
	if _, err := db.ConsumeSpotifyLoginTicket(ctx, expiredHash[:], now); !errors.Is(err, ErrSpotifyLoginTicketInvalid) {
		t.Fatalf("expired ticket error = %v, want ErrSpotifyLoginTicketInvalid", err)
	}
}

func TestLinkedSpotifyLoginRollsBackOnTicketFailure(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, db)
	now := time.Now().UTC()
	raw, hash, err := security.NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	authorization := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{
			AccountID:    "account-rollback-" + raw[:12],
			LegacyUserID: "spotify-rollback-" + raw[:12],
			DisplayName:  "Initial Name",
		},
		Token: models.SpotifyToken{
			AccessToken: "initial-access", TokenType: "Bearer", Expiry: now.Add(time.Hour),
		},
	}
	if err := db.LinkSpotifyAccount(ctx, userID, authorization); err != nil {
		t.Fatalf("LinkSpotifyAccount() error = %v", err)
	}
	if _, err := db.pool.Exec(ctx,
		`INSERT INTO oauth_login_tickets (key_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		hash[:], userID, now.Add(time.Minute),
	); err != nil {
		t.Fatalf("seed ticket collision: %v", err)
	}

	updated := authorization
	updated.Token.AccessToken = "updated-access"
	updated.Profile.DisplayName = "Updated Name"
	if err := db.CompleteLinkedSpotifyLogin(ctx, updated, hash[:], now.Add(time.Minute)); err == nil {
		t.Fatal("CompleteLinkedSpotifyLogin() succeeded despite ticket hash collision")
	}
	storedToken, err := db.GetSpotifyToken(ctx, userID)
	if err != nil {
		t.Fatalf("GetSpotifyToken() error = %v", err)
	}
	if storedToken.AccessToken != "initial-access" {
		t.Fatalf("access token = %q, want rolled-back initial token", storedToken.AccessToken)
	}
	var displayName string
	if err := db.pool.QueryRow(ctx,
		`SELECT display_name FROM spotify_accounts WHERE account_id = $1`, authorization.Profile.AccountID,
	).Scan(&displayName); err != nil {
		t.Fatalf("query rolled-back profile: %v", err)
	}
	if displayName != authorization.Profile.DisplayName {
		t.Fatalf("display name = %q, want rolled-back %q", displayName, authorization.Profile.DisplayName)
	}
}

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
	raw, hash, _ := security.NewOpaqueToken()
	token := models.SpotifyToken{
		AccessToken:  "access-plaintext",
		RefreshToken: "refresh-plaintext",
		TokenType:    "Bearer",
		Expiry:       now.Add(time.Hour),
	}
	legacyID := "spotify-existing-" + raw[:12]
	authorization := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{AccountID: "account-existing-" + raw[:12], LegacyUserID: legacyID},
		Token:   token,
	}
	if err := db.CreatePendingSpotifyAuthorization(ctx, hash[:], authorization, now.Add(time.Minute)); err != nil {
		t.Fatalf("CreatePendingSpotifyAuthorization() error = %v", err)
	}

	var storedAccess, storedRefresh string
	if err := db.pool.QueryRow(ctx, `SELECT access_token, refresh_token FROM pending_spotify_tokens WHERE key_hash = $1`, hash[:]).Scan(&storedAccess, &storedRefresh); err != nil {
		t.Fatalf("query pending ciphertext: %v", err)
	}
	if storedAccess == token.AccessToken || storedRefresh == token.RefreshToken || !strings.HasPrefix(storedAccess, "v1.") || !strings.HasPrefix(storedRefresh, "v1.") {
		t.Fatal("pending Spotify token was not stored as v1 ciphertext")
	}
	_, wrongHash, _ := security.NewOpaqueToken()
	if err := db.ConsumePendingSpotifyAuthorization(ctx, wrongHash[:], userID, now); !errors.Is(err, ErrPendingSpotifyTokenInvalid) {
		t.Fatalf("wrong hash error = %v, want ErrPendingSpotifyTokenInvalid", err)
	}

	if err := db.ConsumePendingSpotifyAuthorization(ctx, hash[:], userID, now); err != nil {
		t.Fatalf("ConsumePendingSpotifyAuthorization() error = %v", err)
	}
	stored, err := db.GetSpotifyToken(ctx, userID)
	if err != nil {
		t.Fatalf("GetSpotifyToken() error = %v", err)
	}
	if stored.AccessToken != token.AccessToken || stored.RefreshToken != token.RefreshToken {
		t.Fatal("consumed Spotify token did not decrypt to the original values")
	}
	if err := db.ConsumePendingSpotifyAuthorization(ctx, hash[:], userID, now); !errors.Is(err, ErrPendingSpotifyTokenInvalid) {
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

	expiredRaw, expiredHash, _ := security.NewOpaqueToken()
	expiredLegacyID := "spotify-expired-" + expiredRaw[:12]
	expiredAuthorization := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{AccountID: "account-expired-" + expiredRaw[:12], LegacyUserID: expiredLegacyID},
		Token:   token,
	}
	if err := db.CreatePendingSpotifyAuthorization(ctx, expiredHash[:], expiredAuthorization, now.Add(-time.Second)); err != nil {
		t.Fatalf("CreatePendingSpotifyAuthorization(expired) error = %v", err)
	}
	if err := db.ConsumePendingSpotifyAuthorization(ctx, expiredHash[:], userID, now); !errors.Is(err, ErrPendingSpotifyTokenInvalid) {
		t.Fatalf("expired error = %v, want ErrPendingSpotifyTokenInvalid", err)
	}

	raw, hash, _ := security.NewOpaqueToken()
	legacyID := "spotify-concurrent-" + raw[:12]
	authorization := models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{AccountID: "account-concurrent-" + raw[:12], LegacyUserID: legacyID},
		Token:   token,
	}
	if err := db.CreatePendingSpotifyAuthorization(ctx, hash[:], authorization, now.Add(time.Minute)); err != nil {
		t.Fatalf("CreatePendingSpotifyAuthorization() error = %v", err)
	}
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- db.ConsumePendingSpotifyAuthorization(ctx, hash[:], userID, now)
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
	expiredRaw, expiredPending, _ := security.NewOpaqueToken()
	liveRaw, livePending, _ := security.NewOpaqueToken()
	if err := db.CreateOAuthState(ctx, expiredState[:], nil, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateOAuthState(ctx, liveState[:], nil, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePendingSpotifyAuthorization(ctx, expiredPending[:], models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{
			AccountID:    "account-cleanup-expired-" + expiredRaw[:12],
			LegacyUserID: "spotify-cleanup-expired-" + expiredRaw[:12],
		},
		Token: token,
	}, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePendingSpotifyAuthorization(ctx, livePending[:], models.SpotifyAuthorization{
		Profile: models.SpotifyProfile{
			AccountID:    "account-cleanup-live-" + liveRaw[:12],
			LegacyUserID: "spotify-cleanup-live-" + liveRaw[:12],
		},
		Token: token,
	}, now.Add(time.Minute)); err != nil {
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
