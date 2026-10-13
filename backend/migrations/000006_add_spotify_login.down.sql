ALTER TABLE pending_spotify_tokens
    DROP COLUMN IF EXISTS spotify_user_id;

DROP TABLE IF EXISTS oauth_login_tickets;
DROP TABLE IF EXISTS spotify_accounts;
