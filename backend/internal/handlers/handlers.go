package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"spotify-backend/internal/auth"
	"spotify-backend/internal/config"
	"spotify-backend/internal/database"
	"spotify-backend/internal/models"
	"spotify-backend/internal/spotify"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

type Handler struct {
	cfg               *config.Config
	jwtManager        *auth.Manager
	authStore         AuthStore
	oauthStore        OAuthStore
	oauthClient       SpotifyOAuthClient
	songStore         SongStore
	profileStore      ProfileStore
	manualSyncStore   ManualSyncStore
	spotifySongClient SpotifySongClient
	now               func() time.Time
}

func New(db *database.DB, spotifyClient *spotify.Client, cfg *config.Config, jwtManager *auth.Manager) *Handler {
	return &Handler{
		cfg:               cfg,
		jwtManager:        jwtManager,
		authStore:         db,
		oauthStore:        db,
		oauthClient:       spotifyClient,
		songStore:         db,
		profileStore:      db,
		manualSyncStore:   db,
		spotifySongClient: spotifyClient,
		now:               time.Now,
	}
}

func (h *Handler) GetRecentSongs(c *gin.Context) {
	ctx := c.Request.Context()
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}
	userIDInt := userID.(int)

	spotifyToken, err := h.manualSyncStore.GetSpotifyToken(ctx, userIDInt)
	if err != nil {
		log.Printf("No Spotify token found for user %d", userIDInt)
		c.JSON(http.StatusOK, gin.H{"message": []models.Song{}, "notice": "Please connect your Spotify account first"})
		return
	}
	oauthToken := &oauth2.Token{
		AccessToken: spotifyToken.AccessToken, RefreshToken: spotifyToken.RefreshToken,
		TokenType: spotifyToken.TokenType, Expiry: spotifyToken.Expiry,
	}
	if err := h.manualSyncStore.UpdateUserSyncStatus(ctx, userIDInt, "syncing"); err != nil {
		log.Printf("Unable to start Spotify sync for user %d", userIDInt)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Sync is temporarily unavailable"})
		return
	}
	songs, err := h.spotifySongClient.GetRecentlyPlayed(ctx, oauthToken)
	if err != nil {
		_ = h.manualSyncStore.UpdateUserSyncStatus(ctx, userIDInt, "failed")
		log.Printf("Spotify recently-played request failed for user %d", userIDInt)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch songs from Spotify"})
		return
	}
	if err := h.manualSyncStore.InsertSongs(ctx, userIDInt, songs); err != nil {
		_ = h.manualSyncStore.UpdateUserSyncStatus(ctx, userIDInt, "failed")
		log.Printf("Song persistence failed for user %d", userIDInt)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save songs to database"})
		return
	}
	if err := h.manualSyncStore.UpdateUserSyncStatus(ctx, userIDInt, "completed"); err != nil {
		_ = h.manualSyncStore.UpdateUserSyncStatus(ctx, userIDInt, "failed")
		log.Printf("Unable to complete Spotify sync for user %d", userIDInt)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Sync is temporarily unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": songs})
}

func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *Handler) GetAllSongs(c *gin.Context) {
	ctx := c.Request.Context()
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}
	userIDInt := userID.(int)

	songs, err := h.songStore.GetAllSongs(ctx, userIDInt)
	if err != nil {
		log.Printf("Song query failed for user %d", userIDInt)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Songs are temporarily unavailable"})
		return
	}
	if songs == nil {
		songs = []models.Song{}
	}
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Retrieved %d songs", len(songs)),
		"data":    songs, "cached": false,
	})
}
