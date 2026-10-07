DROP INDEX IF EXISTS idx_listening_history_author_id;
DROP INDEX IF EXISTS idx_listening_history_song_id;
DROP INDEX IF EXISTS idx_listening_history_user_id;
DROP INDEX IF EXISTS idx_listening_history_listening_date;
DROP INDEX IF EXISTS idx_songs_author_id;

DROP TABLE IF EXISTS listening_history;
DROP TABLE IF EXISTS songs;
DROP TABLE IF EXISTS author;
