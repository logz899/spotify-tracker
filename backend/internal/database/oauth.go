package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"spotify-backend/internal/models"

	"github.com/jackc/pgx/v5"
)

var (
	ErrOAuthStateInvalid          = errors.New("OAuth state is invalid or expired")
	ErrPendingSpotifyTokenInvalid = errors.New("pending Spotify token is invalid or expired")
	ErrSpotifyLoginTicketInvalid  = errors.New("spotify login ticket is invalid or expired")
)

func (db *DB) CreateOAuthState(ctx context.Context, stateHash []byte, userID *int, expiresAt time.Time) error {
	if len(stateHash) != 32 {
		return errors.New("OAuth state hash must be 32 bytes")
	}
	_, err := db.pool.Exec(ctx,
		`INSERT INTO oauth_states (state_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		stateHash, userID, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create OAuth state: %w", err)
	}
	return nil
}

func (db *DB) ConsumeOAuthState(ctx context.Context, stateHash []byte, now time.Time) (*int, error) {
	if len(stateHash) != 32 {
		return nil, ErrOAuthStateInvalid
	}
	var userID *int
	err := db.pool.QueryRow(ctx,
		`DELETE FROM oauth_states
		 WHERE state_hash = $1 AND expires_at > $2
		 RETURNING user_id`,
		stateHash, now,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOAuthStateInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("consume OAuth state: %w", err)
	}
	return userID, nil
}

func (db *DB) CreatePendingSpotifyAuthorization(ctx context.Context, keyHash []byte, authorization models.SpotifyAuthorization, expiresAt time.Time) error {
	if len(keyHash) != 32 {
		return errors.New("pending Spotify token hash must be 32 bytes")
	}
	profile, err := normalizedSpotifyProfile(authorization.Profile)
	if err != nil {
		return err
	}
	accessToken, refreshToken, err := db.encryptSpotifyToken(authorization.Token)
	if err != nil {
		return err
	}
	_, err = db.pool.Exec(ctx,
		`INSERT INTO pending_spotify_tokens
		    (key_hash, account_id, spotify_user_id, display_name, avatar_url, profile_url,
		     access_token, refresh_token, token_type, token_expiry, expires_at)
		 VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9, $10, $11)`,
		keyHash, profile.AccountID, profile.LegacyUserID, profile.DisplayName, profile.AvatarURL, profile.ProfileURL,
		accessToken, refreshToken, authorization.Token.TokenType, authorization.Token.Expiry, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create pending Spotify token: %w", err)
	}
	return nil
}

func (db *DB) ConsumePendingSpotifyAuthorization(ctx context.Context, keyHash []byte, userID int, now time.Time) error {
	if len(keyHash) != 32 || userID <= 0 {
		return ErrPendingSpotifyTokenInvalid
	}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin pending Spotify token transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var profile models.SpotifyProfile
	var accessToken, refreshToken, tokenType string
	var tokenExpiry time.Time
	err = tx.QueryRow(ctx,
		`DELETE FROM pending_spotify_tokens
		 WHERE key_hash = $1 AND expires_at > $2
		 RETURNING account_id, spotify_user_id, COALESCE(display_name, ''),
		           COALESCE(avatar_url, ''), COALESCE(profile_url, ''),
		           access_token, refresh_token, token_type, token_expiry`,
		keyHash, now,
	).Scan(&profile.AccountID, &profile.LegacyUserID, &profile.DisplayName, &profile.AvatarURL, &profile.ProfileURL,
		&accessToken, &refreshToken, &tokenType, &tokenExpiry)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPendingSpotifyTokenInvalid
	}
	if err != nil {
		return fmt.Errorf("consume pending Spotify token: %w", err)
	}

	profile, err = normalizedSpotifyProfile(profile)
	if err != nil {
		return ErrPendingSpotifyTokenInvalid
	}
	if err := linkSpotifyIdentity(ctx, tx, userID, profile); err != nil {
		return err
	}
	if err := upsertEncryptedSpotifyToken(ctx, tx, userID, models.SpotifyToken{
		TokenType: tokenType,
		Expiry:    tokenExpiry,
	}, accessToken, refreshToken); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit pending Spotify token transaction: %w", err)
	}
	return nil
}

func normalizedSpotifyProfile(profile models.SpotifyProfile) (models.SpotifyProfile, error) {
	profile.AccountID = strings.TrimSpace(profile.AccountID)
	profile.LegacyUserID = strings.TrimSpace(profile.LegacyUserID)
	profile.DisplayName = strings.TrimSpace(profile.DisplayName)
	profile.AvatarURL = strings.TrimSpace(profile.AvatarURL)
	profile.ProfileURL = strings.TrimSpace(profile.ProfileURL)
	if profile.AccountID == "" {
		return models.SpotifyProfile{}, errors.New("spotify account ID is required")
	}
	if profile.LegacyUserID == "" {
		return models.SpotifyProfile{}, errors.New("spotify legacy user ID is required")
	}
	return profile, nil
}

func (db *DB) ConsumeSpotifyLoginTicket(ctx context.Context, keyHash []byte, now time.Time) (int, error) {
	if len(keyHash) != 32 {
		return 0, ErrSpotifyLoginTicketInvalid
	}
	var userID int
	err := db.pool.QueryRow(ctx,
		`DELETE FROM oauth_login_tickets
		 WHERE key_hash = $1 AND expires_at > $2
		 RETURNING user_id`,
		keyHash, now,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrSpotifyLoginTicketInvalid
	}
	if err != nil {
		return 0, fmt.Errorf("consume Spotify login ticket: %w", err)
	}
	return userID, nil
}

func (db *DB) DeleteExpiredOAuthSessions(ctx context.Context, now time.Time) (int64, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin OAuth cleanup transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	states, err := tx.Exec(ctx, `DELETE FROM oauth_states WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired OAuth states: %w", err)
	}
	pending, err := tx.Exec(ctx, `DELETE FROM pending_spotify_tokens WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired pending Spotify tokens: %w", err)
	}
	tickets, err := tx.Exec(ctx, `DELETE FROM oauth_login_tickets WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired Spotify login tickets: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit OAuth cleanup transaction: %w", err)
	}
	return states.RowsAffected() + pending.RowsAffected() + tickets.RowsAffected(), nil
}
