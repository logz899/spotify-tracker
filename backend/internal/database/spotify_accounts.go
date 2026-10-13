package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"spotify-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrSpotifyAccountNotLinked = errors.New("spotify account is not linked")
	ErrSpotifyAccountConflict  = errors.New("spotify account link conflicts with an existing account")
)

func (db *DB) LinkSpotifyAccount(ctx context.Context, userID int, authorization models.SpotifyAuthorization) error {
	if userID <= 0 {
		return ErrSpotifyAccountConflict
	}
	profile, err := normalizedSpotifyProfile(authorization.Profile)
	if err != nil {
		return ErrSpotifyAccountConflict
	}
	accessToken, refreshToken, err := db.encryptSpotifyToken(authorization.Token)
	if err != nil {
		return err
	}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Spotify account link transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := linkSpotifyIdentity(ctx, tx, userID, profile); err != nil {
		return err
	}
	if err := upsertEncryptedSpotifyToken(ctx, tx, userID, authorization.Token, accessToken, refreshToken); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Spotify account link transaction: %w", err)
	}
	return nil
}

func (db *DB) CompleteLinkedSpotifyLogin(
	ctx context.Context,
	authorization models.SpotifyAuthorization,
	ticketHash []byte,
	expiresAt time.Time,
) error {
	profile, err := normalizedSpotifyProfile(authorization.Profile)
	if err != nil {
		return ErrSpotifyAccountNotLinked
	}
	if len(ticketHash) != 32 {
		return errors.New("spotify login ticket hash must be 32 bytes")
	}
	accessToken, refreshToken, err := db.encryptSpotifyToken(authorization.Token)
	if err != nil {
		return err
	}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin linked Spotify login transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userID, err := resolveLinkedSpotifyIdentity(ctx, tx, profile)
	if err != nil {
		return err
	}
	if err := upsertEncryptedSpotifyToken(ctx, tx, userID, authorization.Token, accessToken, refreshToken); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO oauth_login_tickets (key_hash, user_id, expires_at)
		 VALUES ($1, $2, $3)`,
		ticketHash, userID, expiresAt,
	); err != nil {
		return fmt.Errorf("create Spotify login ticket: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit linked Spotify login transaction: %w", err)
	}
	return nil
}

func (db *DB) encryptSpotifyToken(token models.SpotifyToken) (string, string, error) {
	accessToken, err := db.cipher.Encrypt(token.AccessToken)
	if err != nil {
		return "", "", fmt.Errorf("encrypt Spotify access token: %w", err)
	}
	refreshToken, err := db.cipher.Encrypt(token.RefreshToken)
	if err != nil {
		return "", "", fmt.Errorf("encrypt Spotify refresh token: %w", err)
	}
	return accessToken, refreshToken, nil
}

func linkSpotifyIdentity(ctx context.Context, tx pgx.Tx, userID int, profile models.SpotifyProfile) error {
	tag, err := tx.Exec(ctx,
		`UPDATE spotify_accounts
		 SET spotify_user_id = $1,
		     display_name = NULLIF($2, ''),
		     avatar_url = NULLIF($3, ''),
		     profile_url = NULLIF($4, ''),
		     updated_at = CURRENT_TIMESTAMP
		 WHERE account_id = $5 AND user_id = $6`,
		profile.LegacyUserID, profile.DisplayName, profile.AvatarURL, profile.ProfileURL, profile.AccountID, userID,
	)
	if isUniqueViolation(err) {
		return ErrSpotifyAccountConflict
	}
	if err != nil {
		return fmt.Errorf("refresh Spotify identity: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	tag, err = tx.Exec(ctx,
		`UPDATE spotify_accounts
		 SET account_id = $1,
		     display_name = NULLIF($2, ''),
		     avatar_url = NULLIF($3, ''),
		     profile_url = NULLIF($4, ''),
		     updated_at = CURRENT_TIMESTAMP
		 WHERE user_id = $5 AND account_id IS NULL AND spotify_user_id = $6`,
		profile.AccountID, profile.DisplayName, profile.AvatarURL, profile.ProfileURL, userID, profile.LegacyUserID,
	)
	if isUniqueViolation(err) {
		return ErrSpotifyAccountConflict
	}
	if err != nil {
		return fmt.Errorf("backfill Spotify identity: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO spotify_accounts
		    (user_id, account_id, spotify_user_id, display_name, avatar_url, profile_url)
		 VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''))`,
		userID, profile.AccountID, profile.LegacyUserID, profile.DisplayName, profile.AvatarURL, profile.ProfileURL,
	)
	if isUniqueViolation(err) {
		return ErrSpotifyAccountConflict
	}
	if err != nil {
		return fmt.Errorf("link Spotify identity: %w", err)
	}
	return nil
}

func resolveLinkedSpotifyIdentity(ctx context.Context, tx pgx.Tx, profile models.SpotifyProfile) (int, error) {
	var userID int
	err := tx.QueryRow(ctx,
		`UPDATE spotify_accounts
		 SET spotify_user_id = $1,
		     display_name = NULLIF($2, ''),
		     avatar_url = NULLIF($3, ''),
		     profile_url = NULLIF($4, ''),
		     updated_at = CURRENT_TIMESTAMP
		 WHERE account_id = $5
		 RETURNING user_id`,
		profile.LegacyUserID, profile.DisplayName, profile.AvatarURL, profile.ProfileURL, profile.AccountID,
	).Scan(&userID)
	if isUniqueViolation(err) {
		return 0, ErrSpotifyAccountConflict
	}
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("resolve linked Spotify account: %w", err)
	}

	err = tx.QueryRow(ctx,
		`UPDATE spotify_accounts
		 SET account_id = $1,
		     display_name = NULLIF($2, ''),
		     avatar_url = NULLIF($3, ''),
		     profile_url = NULLIF($4, ''),
		     updated_at = CURRENT_TIMESTAMP
		 WHERE account_id IS NULL AND spotify_user_id = $5
		 RETURNING user_id`,
		profile.AccountID, profile.DisplayName, profile.AvatarURL, profile.ProfileURL, profile.LegacyUserID,
	).Scan(&userID)
	if isUniqueViolation(err) {
		return 0, ErrSpotifyAccountConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrSpotifyAccountNotLinked
	}
	if err != nil {
		return 0, fmt.Errorf("backfill linked Spotify account: %w", err)
	}
	return userID, nil
}

func upsertEncryptedSpotifyToken(
	ctx context.Context,
	tx pgx.Tx,
	userID int,
	token models.SpotifyToken,
	accessToken string,
	refreshToken string,
) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO spotify_tokens (user_id, access_token, refresh_token, token_type, expiry)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id)
		 DO UPDATE SET access_token = EXCLUDED.access_token,
		               refresh_token = EXCLUDED.refresh_token,
		               token_type = EXCLUDED.token_type,
		               expiry = EXCLUDED.expiry,
		               updated_at = CURRENT_TIMESTAMP`,
		userID, accessToken, refreshToken, token.TokenType, token.Expiry,
	)
	if err != nil {
		return fmt.Errorf("save linked Spotify token: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
