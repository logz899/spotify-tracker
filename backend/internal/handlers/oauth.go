package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"spotify-backend/internal/database"
	"spotify-backend/internal/models"
	"spotify-backend/internal/security"

	"github.com/gin-gonic/gin"
)

const (
	oauthSessionTTL = 10 * time.Minute
	loginTicketTTL  = 2 * time.Minute
)

func (h *Handler) SpotifyLogin(c *gin.Context) {
	rawState, stateHash, err := security.NewOpaqueToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to start Spotify authorization"})
		return
	}
	var userID *int
	if value, exists := c.Get("userID"); exists {
		if id, ok := value.(int); ok && id > 0 {
			userID = &id
		}
	}
	now := h.now().UTC()
	if err := h.oauthStore.CreateOAuthState(c.Request.Context(), stateHash[:], userID, now.Add(oauthSessionTTL)); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to start Spotify authorization"})
		return
	}
	_, _ = h.oauthStore.DeleteExpiredOAuthSessions(c.Request.Context(), now)
	authURL := h.oauthClient.GetAuthURL(rawState)
	if c.GetHeader("Accept") == "application/json" {
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (h *Handler) SpotifyCallback(c *gin.Context) {
	code := c.Query("code")
	rawState := c.Query("state")
	if code == "" || security.ValidateOpaqueToken(rawState) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Spotify authorization response"})
		return
	}
	now := h.now().UTC()
	stateHash := security.HashOpaqueToken(rawState)
	userID, err := h.oauthStore.ConsumeOAuthState(c.Request.Context(), stateHash[:], now)
	if errors.Is(err, database.ErrOAuthStateInvalid) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Spotify authorization expired or was already used"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Spotify authorization is temporarily unavailable"})
		return
	}

	oauthToken, err := h.oauthClient.GetToken(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Spotify authorization failed"})
		return
	}
	profile, err := h.oauthClient.GetCurrentUserProfile(c.Request.Context(), oauthToken)
	if err != nil || strings.TrimSpace(profile.AccountID) == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Unable to verify Spotify account"})
		return
	}
	authorization := models.SpotifyAuthorization{
		Profile: profile,
		Token: models.SpotifyToken{
			AccessToken: oauthToken.AccessToken, RefreshToken: oauthToken.RefreshToken,
			TokenType: oauthToken.TokenType, Expiry: oauthToken.Expiry,
		},
	}
	frontendURL := strings.TrimRight(h.cfg.FrontendURL, "/")
	if userID != nil {
		if err := h.oauthStore.LinkSpotifyAccount(c.Request.Context(), *userID, authorization); errors.Is(err, database.ErrSpotifyAccountConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "Spotify account cannot be linked"})
			return
		} else if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to save Spotify authorization"})
			return
		}
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"/#/home?spotify_linked=true")
		return
	}

	rawTicket, ticketHash, err := security.NewOpaqueToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to complete Spotify authorization"})
		return
	}
	err = h.oauthStore.CompleteLinkedSpotifyLogin(c.Request.Context(), authorization, ticketHash[:], now.Add(loginTicketTTL))
	if err == nil {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"/#/?spotify_login=true&ticket="+rawTicket)
		return
	}
	if !errors.Is(err, database.ErrSpotifyAccountNotLinked) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to save Spotify authorization"})
		return
	}

	rawKey, keyHash, err := security.NewOpaqueToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to complete Spotify authorization"})
		return
	}
	if err := h.oauthStore.CreatePendingSpotifyAuthorization(c.Request.Context(), keyHash[:], authorization, now.Add(oauthSessionTTL)); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to save Spotify authorization"})
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, frontendURL+"/#/register?spotify_connected=true&key="+rawKey)
}

func (h *Handler) LinkPendingSpotify(c *gin.Context) {
	userValue, exists := c.Get("userID")
	userID, ok := userValue.(int)
	if !exists || !ok || userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}
	var req struct {
		PendingKey string `json:"pending_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || security.ValidateOpaqueToken(req.PendingKey) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Spotify session"})
		return
	}
	hash := security.HashOpaqueToken(req.PendingKey)
	err := h.oauthStore.ConsumePendingSpotifyAuthorization(c.Request.Context(), hash[:], userID, h.now().UTC())
	if errors.Is(err, database.ErrPendingSpotifyTokenInvalid) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Spotify session expired or was already used"})
		return
	}
	if errors.Is(err, database.ErrSpotifyAccountConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "Spotify account cannot be linked"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Spotify session is temporarily unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Spotify account linked successfully", "success": true})
}

func (h *Handler) ExchangeSpotifySession(c *gin.Context) {
	var req struct {
		Ticket string `json:"ticket" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || security.ValidateOpaqueToken(req.Ticket) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Spotify login session"})
		return
	}

	hash := security.HashOpaqueToken(req.Ticket)
	userID, err := h.oauthStore.ConsumeSpotifyLoginTicket(c.Request.Context(), hash[:], h.now().UTC())
	if errors.Is(err, database.ErrSpotifyLoginTicketInvalid) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Spotify login session"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Spotify login is temporarily unavailable"})
		return
	}

	user, err := h.authStore.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Spotify login is temporarily unavailable"})
		return
	}
	token, err := h.jwtManager.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user":    gin.H{"id": user.ID, "username": user.Username, "email": user.Email},
		"token":   token,
	})
}
