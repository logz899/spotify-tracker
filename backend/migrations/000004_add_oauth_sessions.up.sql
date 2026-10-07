CREATE TABLE oauth_states (
    state_hash BYTEA PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT oauth_states_hash_length CHECK (octet_length(state_hash) = 32)
);

CREATE INDEX oauth_states_expires_at_idx ON oauth_states (expires_at);

CREATE TABLE pending_spotify_tokens (
    key_hash BYTEA PRIMARY KEY,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_type VARCHAR(50) NOT NULL,
    token_expiry TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT pending_spotify_tokens_hash_length CHECK (octet_length(key_hash) = 32)
);

CREATE INDEX pending_spotify_tokens_expires_at_idx
    ON pending_spotify_tokens (expires_at);
