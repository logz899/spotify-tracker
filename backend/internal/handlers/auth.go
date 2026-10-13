package handlers

import (
	"net/http"
	"strings"
	"time"

	"spotify-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"golang.org/x/crypto/bcrypt"
)

type normalizedEmailInput struct {
	Email string `binding:"required,email"`
}

func normalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func validNormalizedEmail(email string) bool {
	return binding.Validator.ValidateStruct(normalizedEmailInput{Email: email}) == nil
}

func (h *Handler) Register(c *gin.Context) {
	var req models.UserRegistration
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	req.Email = normalizeEmail(req.Email)
	if !validNormalizedEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	ctx := c.Request.Context()
	if _, err := h.authStore.GetUserByUsername(ctx, req.Username); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}
	if _, err := h.authStore.GetUserByEmail(ctx, req.Email); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}
	user := &models.User{Username: req.Username, Email: req.Email, PasswordHash: string(hashedPassword)}
	if err := h.authStore.CreateUser(ctx, user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}
	token, err := h.jwtManager.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user":    gin.H{"id": user.ID, "username": user.Username, "email": user.Email},
		"token":   token,
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req models.UserLogin
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	req.Email = normalizeEmail(req.Email)
	if !validNormalizedEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	ctx := c.Request.Context()
	user, err := h.authStore.GetUserByEmail(ctx, req.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
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

func (h *Handler) GetCurrentUser(c *gin.Context) {
	userValue, exists := c.Get("userID")
	userID, ok := userValue.(int)
	if !exists || !ok || userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}
	profile, err := h.profileStore.GetUserProfile(c.Request.Context(), userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Profile is temporarily unavailable"})
		return
	}

	type spotifyResponse struct {
		Linked      bool   `json:"linked"`
		DisplayName string `json:"display_name,omitempty"`
		AvatarURL   string `json:"avatar_url,omitempty"`
		ProfileURL  string `json:"profile_url,omitempty"`
	}
	type syncResponse struct {
		Status       *string    `json:"status"`
		LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	}
	type userResponse struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	}
	var status *string
	if trimmed := strings.TrimSpace(profile.UpdateStatus); trimmed != "" {
		status = &trimmed
	}
	c.JSON(http.StatusOK, struct {
		User    userResponse    `json:"user"`
		Spotify spotifyResponse `json:"spotify"`
		Sync    syncResponse    `json:"sync"`
	}{
		User: userResponse{ID: profile.UserID, Username: profile.Username},
		Spotify: spotifyResponse{
			Linked: profile.SpotifyLinked, DisplayName: profile.SpotifyDisplayName,
			AvatarURL: profile.SpotifyAvatarURL, ProfileURL: profile.SpotifyProfileURL,
		},
		Sync: syncResponse{Status: status, LastSyncedAt: profile.UpdateDate},
	})
}
