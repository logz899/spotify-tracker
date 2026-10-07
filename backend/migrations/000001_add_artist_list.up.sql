-- Author table
CREATE TABLE IF NOT EXISTS author (
    id SERIAL PRIMARY KEY,
    author_name TEXT NOT NULL UNIQUE,
    image_url TEXT NOT NULL
);

-- Songs table (album_id added in migration 003)
CREATE TABLE IF NOT EXISTS songs (
    id SERIAL PRIMARY KEY,
    song_name TEXT NOT NULL UNIQUE,
    image_url TEXT NOT NULL,
    author_id INT NOT NULL REFERENCES author(id)
);

-- Listening history table
-- user_id FK to users added in migration 002 after the users table exists
CREATE TABLE IF NOT EXISTS listening_history (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    song_id INT NOT NULL REFERENCES songs(id),
    author_id INT NOT NULL REFERENCES author(id),
    listening_date TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_songs_author_id ON songs(author_id);
CREATE INDEX IF NOT EXISTS idx_listening_history_listening_date ON listening_history(listening_date);
CREATE INDEX IF NOT EXISTS idx_listening_history_user_id ON listening_history(user_id);
CREATE INDEX IF NOT EXISTS idx_listening_history_song_id ON listening_history(song_id);
CREATE INDEX IF NOT EXISTS idx_listening_history_author_id ON listening_history(author_id);
