package database

import (
	"context"
	"fmt"
	"log"
	"spotify-backend/internal/models"
	"spotify-backend/internal/security"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool   *pgxpool.Pool
	cipher *security.TokenCipher
}

func New(databaseURL string, cipher *security.TokenCipher) (*DB, error) {
	if cipher == nil {
		return nil, fmt.Errorf("token cipher is required")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Println("Successfully connected to database")

	return &DB{pool: pool, cipher: cipher}, nil
}

func (db *DB) Close() {
	db.pool.Close()
	log.Println("Database connection closed")
}

func (db *DB) InsertSongs(ctx context.Context, userID int, songs []models.Song) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, song := range songs {
		// Insert/get author
		var authorID int
		authorQuery := `INSERT INTO author (author_name, image_url)
		                VALUES ($1, $2)
		                ON CONFLICT (author_name) DO UPDATE SET author_name = EXCLUDED.author_name
		                RETURNING id`
		err := tx.QueryRow(ctx, authorQuery, song.AuthorName, song.ImageURL).Scan(&authorID)
		if err != nil {
			return fmt.Errorf("failed to insert/get author '%s': %w", song.AuthorName, err)
		}

		// Insert/get album
		var albumID *int
		if song.AlbumName != "" {
			var aid int
			albumQuery := `INSERT INTO albums (album_name, image_url)
			               VALUES ($1, $2)
			               ON CONFLICT (album_name) DO UPDATE SET image_url = EXCLUDED.image_url
			               RETURNING id`
			err = tx.QueryRow(ctx, albumQuery, song.AlbumName, song.ImageURL).Scan(&aid)
			if err != nil {
				return fmt.Errorf("failed to insert/get album '%s': %w", song.AlbumName, err)
			}
			albumID = &aid
		}

		// Insert/get song
		var songID int
		songQuery := `INSERT INTO songs (song_name, image_url, author_id, album_id)
		              VALUES ($1, $2, $3, $4)
		              ON CONFLICT (song_name) DO UPDATE SET image_url = EXCLUDED.image_url, album_id = EXCLUDED.album_id
		              RETURNING id`
		err = tx.QueryRow(ctx, songQuery, song.SongName, song.ImageURL, authorID, albumID).Scan(&songID)
		if err != nil {
			return fmt.Errorf("failed to insert/get song '%s': %w", song.SongName, err)
		}

		// Insert listening history
		historyQuery := `INSERT INTO listening_history (user_id, song_id, author_id, listening_date)
		                 VALUES ($1, $2, $3, $4)
		                 ON CONFLICT (user_id, song_id, author_id)
		                 DO UPDATE SET listening_date = EXCLUDED.listening_date`
		_, err = tx.Exec(ctx, historyQuery, userID, songID, authorID, song.ListeningDate)
		if err != nil {
			return fmt.Errorf("failed to insert listening history for song '%s': %w", song.SongName, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully inserted %d songs for user %d into database", len(songs), userID)
	return nil
}

func (db *DB) GetAllSongs(ctx context.Context, userID int) ([]models.Song, error) {
	// Aggregate listening history by song to calculate play counts
	query := `SELECT
	              s.id,
	              s.song_name,
	              s.image_url,
	              a.id,
	              a.author_name,
	              COALESCE(alb.album_name, ''),
	              COUNT(*) as song_count,
	              MAX(lh.listening_date) as listening_date
	          FROM listening_history lh
	          INNER JOIN songs s ON lh.song_id = s.id
	          INNER JOIN author a ON lh.author_id = a.id
	          LEFT JOIN albums alb ON s.album_id = alb.id
	          WHERE lh.user_id = $1
	          GROUP BY s.id, s.song_name, s.image_url, a.id, a.author_name, alb.album_name
	          ORDER BY song_count DESC, listening_date DESC`

	rows, err := db.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query songs: %w", err)
	}
	defer rows.Close()

	var songs []models.Song
	for rows.Next() {
		var song models.Song
		if err := rows.Scan(&song.SongID, &song.SongName, &song.ImageURL, &song.AuthorID, &song.AuthorName, &song.AlbumName, &song.SongCount, &song.ListeningDate); err != nil {
			return nil, fmt.Errorf("failed to scan song: %w", err)
		}
		songs = append(songs, song)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return songs, nil
}

func (db *DB) CreateUser(ctx context.Context, user *models.User) error {
	query := `INSERT INTO users (username, email, password_hash)
	          VALUES ($1, $2, $3)
	          RETURNING id, created_at, updated_at`

	err := db.pool.QueryRow(ctx, query, user.Username, user.Email, user.PasswordHash).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("Created user: %s (ID: %d)", user.Username, user.ID)
	return nil
}

func (db *DB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `SELECT id, username, email, password_hash, created_at, updated_at
	          FROM users WHERE username = $1`

	user := &models.User{}
	err := db.pool.QueryRow(ctx, query, username).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, username, email, password_hash, created_at, updated_at
	          FROM users WHERE email = $1`

	user := &models.User{}
	err := db.pool.QueryRow(ctx, query, email).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (db *DB) GetAllUsersWithSpotifyTokens(ctx context.Context) ([]models.User, error) {
	query := `SELECT DISTINCT u.id, u.username, u.email, u.created_at, u.updated_at, u.update_status, u.update_date
	          FROM users u
	          INNER JOIN spotify_tokens st ON u.id = st.user_id
	          ORDER BY u.id`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users with Spotify tokens: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt, &user.UpdatedAt, &user.UpdateStatus, &user.UpdateDate); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return users, nil
}

func (db *DB) UpdateUserSyncStatus(ctx context.Context, userID int, status string) error {
	query := `UPDATE users
	          SET update_status = $1,
	              update_date = CURRENT_TIMESTAMP,
	              updated_at = CURRENT_TIMESTAMP
	          WHERE id = $2`

	_, err := db.pool.Exec(ctx, query, status, userID)
	if err != nil {
		return fmt.Errorf("failed to update user sync status: %w", err)
	}

	return nil
}
