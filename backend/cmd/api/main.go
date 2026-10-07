package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"spotify-backend/internal/auth"
	"spotify-backend/internal/config"
	"spotify-backend/internal/database"
	"spotify-backend/internal/handlers"
	"spotify-backend/internal/middleware"
	"spotify-backend/internal/security"
	"spotify-backend/internal/spotify"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file (base configuration)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading .env file")
	}

	// Load and validate configuration before creating external clients.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}
	gin.SetMode(cfg.GinMode)

	tokenCipher, err := security.NewTokenCipher(cfg.TokenEncryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize token encryption: %v", err)
	}

	jwtManager, err := auth.NewManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, 12*time.Hour)
	if err != nil {
		log.Fatalf("Failed to initialize authentication: %v", err)
	}

	// Initialize database connection
	db, err := database.New(cfg.DatabaseURL, tokenCipher)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Spotify client
	spotifyClient := spotify.New(cfg.SpotifyClientID, cfg.SpotifyClientSecret, cfg.SpotifyRedirectURI)

	// Initialize handlers.
	h := handlers.New(db, spotifyClient, cfg, jwtManager)

	// Create Gin router
	router := gin.Default()
	router.Use(middleware.SecurityHeaders(), middleware.MaxBodyBytes(1<<20))

	// Configure CORS from environment (CORS_ALLOWED_ORIGINS in ConfigMap)
	router.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSAllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API routes
	v1 := router.Group("/api/v1")
	{
		// Health check
		v1.GET("/health", h.HealthCheck)

		// Public authentication routes
		authLimiter, err := middleware.NewIPRateLimiter(10, time.Minute, 10*time.Minute, 10_000)
		if err != nil {
			log.Fatalf("Failed to initialize auth rate limiter: %v", err)
		}
		authRoutes := v1.Group("/auth")
		authRoutes.Use(authLimiter.Middleware())
		{
			authRoutes.POST("/register", h.Register)
			authRoutes.POST("/login", h.Login)

			// Spotify OAuth routes (with optional auth)
			authRoutes.GET("/spotify/login", middleware.OptionalAuth(jwtManager), h.SpotifyLogin)
			authRoutes.GET("/spotify/callback", middleware.OptionalAuth(jwtManager), h.SpotifyCallback)
		}

		// Protected routes (require JWT authentication)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(jwtManager))
		{
			// User routes
			protected.GET("/user/me", h.GetCurrentUser)

			// Spotify routes
			protected.POST("/auth/link-spotify", h.LinkPendingSpotify)

			// Songs routes
			songs := protected.Group("/songs")
			{
				songs.GET("/recent", h.GetRecentSongs)
				songs.GET("/all", h.GetAllSongs)
			}
		}
	}

	// Root endpoint (for backward compatibility)
	router.GET("/", h.GetRecentSongs)

	// Create HTTP server
	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
