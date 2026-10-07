ALTER TABLE listening_history DROP CONSTRAINT IF EXISTS listening_history_user_song_author_unique;
ALTER TABLE listening_history DROP CONSTRAINT IF EXISTS fk_listening_history_user;

DROP TABLE IF EXISTS spotify_tokens;
DROP TABLE IF EXISTS users;
