-- Users table for authentication
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    update_date TIMESTAMP
);

-- Spotify tokens (one-to-one with users)
CREATE TABLE IF NOT EXISTS spotify_tokens (
    id SERIAL PRIMARY KEY,
    user_id INTEGER UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_token TEXT NOT NULL,
    refresh_token TEXT,
    token_type VARCHAR(50),
    expiry TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add FK from listening_history.user_id -> users.id now that users exists
ALTER TABLE listening_history
    ADD CONSTRAINT fk_listening_history_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Unique constraint required for ON CONFLICT (user_id, song_id, author_id) upserts
ALTER TABLE listening_history
    ADD CONSTRAINT listening_history_user_song_author_unique
    UNIQUE (user_id, song_id, author_id);
