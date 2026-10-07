-- Albums table
CREATE TABLE IF NOT EXISTS albums (
    id SERIAL PRIMARY KEY,
    album_name TEXT NOT NULL UNIQUE,
    image_url TEXT NOT NULL
);

-- Add nullable album_id to songs
ALTER TABLE songs ADD COLUMN IF NOT EXISTS album_id INT REFERENCES albums(id);

CREATE INDEX IF NOT EXISTS idx_songs_album_id ON songs(album_id);
