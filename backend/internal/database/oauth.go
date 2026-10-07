package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"spotify-backend/internal/models"

	"github.com/jackc/pgx/v5"
)

var (
	ErrOAuthStateInvalid          = errors.New("OAuth state is invalid or expired")
	ErrPendingSpotifyTokenInvalid = errors.New("pending Spotify token is invalid or expired")
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

func (db *DB) CreatePendingSpotifyToken(ctx context.Context, keyHash []byte, token models.SpotifyToken, expiresAt time.Time) error {
	if len(keyHash) != 32 {
		return errors.New("pending Spotify token hash must be 32 bytes")
	}
	accessToken, err := db.cipher.Encrypt(token.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt Spotify access token: %w", err)
	}
	refreshToken, err := db.cipher.Encrypt(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt Spotify refresh token: %w", err)
	}
	_, err = db.pool.Exec(ctx,
		`INSERT INTO pending_spotify_tokens
		    (key_hash, access_token, refresh_token, token_type, token_expiry, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		keyHash, accessToken, refreshToken, token.TokenType, token.Expiry, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create pending Spotify token: %w", err)
	}
	return nil
}

func (db *DB) ConsumePendingSpotifyToken(ctx context.Context, keyHash []byte, userID int, now time.Time) error {
	if len(keyHash) != 32 || userID <= 0 {
		return ErrPendingSpotifyTokenInvalid
	}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin pending Spotify token transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var accessToken, refreshToken, tokenType string
	var tokenExpiry time.Time
	err = tx.QueryRow(ctx,
		`DELETE FROM pending_spotify_tokens
		 WHERE key_hash = $1 AND expires_at > $2
		 RETURNING access_token, refresh_token, token_type, token_expiry`,
		keyHash, now,
	).Scan(&accessToken, &refreshToken, &tokenType, &tokenExpiry)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPendingSpotifyTokenInvalid
	}
	if err != nil {
		return fmt.Errorf("consume pending Spotify token: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO spotify_tokens (user_id, access_token, refresh_token, token_type, expiry)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id)
		 DO UPDATE SET access_token = EXCLUDED.access_token,
		               refresh_token = EXCLUDED.refresh_token,
		               token_type = EXCLUDED.token_type,
		               expiry = EXCLUDED.expiry,
		               updated_at = CURRENT_TIMESTAMP`,
		userID, accessToken, refreshToken, tokenType, tokenExpiry,
	)
	if err != nil {
		return fmt.Errorf("link pending Spotify token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit pending Spotify token transaction: %w", err)
	}
	return nil
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
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit OAuth cleanup transaction: %w", err)
	}
	return states.RowsAffected() + pending.RowsAffected(), nil
}
