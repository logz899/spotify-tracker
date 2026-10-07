package models

import "time"

// User represents a registered user
type User struct {
	ID           int        `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // Never expose password hash in JSON
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	UpdateStatus string     `json:"update_status"`
	UpdateDate   *time.Time `json:"update_date,omitempty"` // Pointer because it can be NULL
}

// UserRegistration represents registration request
type UserRegistration struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=10"`
}

// UserLogin represents login request
type UserLogin struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SpotifyToken represents a user's Spotify OAuth token
type SpotifyToken struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
