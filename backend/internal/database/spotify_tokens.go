package database

import (
	"context"
	"fmt"

	"spotify-backend/internal/models"
)

func (db *DB) SaveSpotifyToken(ctx context.Context, token *models.SpotifyToken) error {
	accessToken, err := db.cipher.Encrypt(token.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt Spotify access token: %w", err)
	}
	refreshToken, err := db.cipher.Encrypt(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt Spotify refresh token: %w", err)
	}

	query := `INSERT INTO spotify_tokens (user_id, access_token, refresh_token, token_type, expiry)
	          VALUES ($1, $2, $3, $4, $5)
	          ON CONFLICT (user_id)
	          DO UPDATE SET access_token = EXCLUDED.access_token,
	                        refresh_token = EXCLUDED.refresh_token,
	                        token_type = EXCLUDED.token_type,
	                        expiry = EXCLUDED.expiry,
	                        updated_at = CURRENT_TIMESTAMP
	          RETURNING id, created_at, updated_at`

	err = db.pool.QueryRow(ctx, query, token.UserID, accessToken, refreshToken, token.TokenType, token.Expiry).
		Scan(&token.ID, &token.CreatedAt, &token.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save Spotify token: %w", err)
	}
	return nil
}

func (db *DB) GetSpotifyToken(ctx context.Context, userID int) (*models.SpotifyToken, error) {
	query := `SELECT id, user_id, access_token, refresh_token, token_type, expiry, created_at, updated_at
	          FROM spotify_tokens WHERE user_id = $1`

	token := &models.SpotifyToken{}
	var accessToken, refreshToken string
	err := db.pool.QueryRow(ctx, query, userID).
		Scan(&token.ID, &token.UserID, &accessToken, &refreshToken, &token.TokenType, &token.Expiry, &token.CreatedAt, &token.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get Spotify token: %w", err)
	}
	token.AccessToken, err = db.cipher.Decrypt(accessToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt Spotify access token: %w", err)
	}
	token.RefreshToken, err = db.cipher.Decrypt(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt Spotify refresh token: %w", err)
	}
	return token, nil
}
