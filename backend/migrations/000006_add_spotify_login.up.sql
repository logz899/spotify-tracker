CREATE TABLE spotify_accounts (
    spotify_user_id TEXT PRIMARY KEY,
    user_id INTEGER UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT spotify_accounts_user_id_not_blank
        CHECK (btrim(spotify_user_id) <> '')
);

CREATE TABLE oauth_login_tickets (
    key_hash BYTEA PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT oauth_login_ticket_hash_length
        CHECK (octet_length(key_hash) = 32)
);

CREATE INDEX oauth_login_tickets_expires_at_idx
    ON oauth_login_tickets (expires_at);

ALTER TABLE pending_spotify_tokens
    ADD COLUMN spotify_user_id TEXT;

DELETE FROM pending_spotify_tokens;

ALTER TABLE pending_spotify_tokens
    ALTER COLUMN spotify_user_id SET NOT NULL,
    ADD CONSTRAINT pending_spotify_tokens_user_id_not_blank
        CHECK (btrim(spotify_user_id) <> '');
