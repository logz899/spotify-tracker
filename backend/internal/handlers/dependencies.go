package handlers

import (
	"context"
	"time"

	"spotify-backend/internal/models"

	"golang.org/x/oauth2"
)

type AuthStore interface {
	GetUserByUsername(context.Context, string) (*models.User, error)
	GetUserByEmail(context.Context, string) (*models.User, error)
	CreateUser(context.Context, *models.User) error
}

type OAuthStore interface {
	CreateOAuthState(context.Context, []byte, *int, time.Time) error
	ConsumeOAuthState(context.Context, []byte, time.Time) (*int, error)
	CreatePendingSpotifyToken(context.Context, []byte, models.SpotifyToken, time.Time) error
	ConsumePendingSpotifyToken(context.Context, []byte, int, time.Time) error
	SaveSpotifyToken(context.Context, *models.SpotifyToken) error
	DeleteExpiredOAuthSessions(context.Context, time.Time) (int64, error)
}

type SpotifyOAuthClient interface {
	GetAuthURL(string) string
	GetToken(context.Context, string) (*oauth2.Token, error)
}

type SongStore interface {
	GetAllSongs(context.Context, int) ([]models.Song, error)
}
