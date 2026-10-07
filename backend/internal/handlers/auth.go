package handlers

import (
	"net/http"

	"spotify-backend/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) Register(c *gin.Context) {
	var req models.UserRegistration
	if err := c.ShouldBindJSON(&req); err != nil {
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
	ctx := c.Request.Context()
	user, err := h.authStore.GetUserByUsername(ctx, req.Username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
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
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{"user": gin.H{"id": userID, "username": username}})
}
