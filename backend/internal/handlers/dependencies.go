package handlers

import (
	"context"
	"time"

	"spotify-backend/internal/models"

	"golang.org/x/oauth2"
)

type AuthStore interface {
	GetUserByID(context.Context, int) (*models.User, error)
	GetUserByUsername(context.Context, string) (*models.User, error)
	GetUserByEmail(context.Context, string) (*models.User, error)
	CreateUser(context.Context, *models.User) error
}

type OAuthStore interface {
	CreateOAuthState(context.Context, []byte, *int, time.Time) error
	ConsumeOAuthState(context.Context, []byte, time.Time) (*int, error)
	CreatePendingSpotifyAuthorization(context.Context, []byte, models.SpotifyAuthorization, time.Time) error
	ConsumePendingSpotifyAuthorization(context.Context, []byte, int, time.Time) error
	LinkSpotifyAccount(context.Context, int, models.SpotifyAuthorization) error
	CompleteLinkedSpotifyLogin(context.Context, models.SpotifyAuthorization, []byte, time.Time) error
	ConsumeSpotifyLoginTicket(context.Context, []byte, time.Time) (int, error)
	DeleteExpiredOAuthSessions(context.Context, time.Time) (int64, error)
}

type SpotifyOAuthClient interface {
	GetAuthURL(string) string
	GetToken(context.Context, string) (*oauth2.Token, error)
	GetCurrentUserProfile(context.Context, *oauth2.Token) (models.SpotifyProfile, error)
}

type SongStore interface {
	GetAllSongs(context.Context, int) ([]models.Song, error)
}

type ProfileStore interface {
	GetUserProfile(context.Context, int) (*models.UserProfile, error)
}

type ManualSyncStore interface {
	GetSpotifyToken(context.Context, int) (*models.SpotifyToken, error)
	InsertSongs(context.Context, int, []models.Song) error
	UpdateUserSyncStatus(context.Context, int, string) error
}

type SpotifySongClient interface {
	GetRecentlyPlayed(context.Context, *oauth2.Token) ([]models.Song, error)
}
