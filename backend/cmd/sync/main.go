package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"spotify-backend/internal/config"
	"spotify-backend/internal/database"
	"spotify-backend/internal/security"
	"spotify-backend/internal/services"
	"spotify-backend/internal/spotify"

	"github.com/joho/godotenv"
)

const syncJobTimeout = 10 * time.Minute

type syncJob interface {
	SyncAllUsers(context.Context) (services.SyncSummary, error)
}

type dependencies struct {
	job    syncJob
	output io.Writer
}

func main() {
	flag.Parse()
	if err := execute(); err != nil {
		log.Print("Spotify sync job failed")
		os.Exit(commandExitCode(err))
	}
}

func execute() error {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading .env file")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	tokenCipher, err := security.NewTokenCipher(cfg.TokenEncryptionKey)
	if err != nil {
		return fmt.Errorf("initialize token encryption: %w", err)
	}
	db, err := database.New(cfg.DatabaseURL, tokenCipher)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer db.Close()

	spotifyClient := spotify.New(cfg.SpotifyClientID, cfg.SpotifyClientSecret, cfg.SpotifyRedirectURI)
	service := services.NewSyncService(db, spotifyClient)
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, syncJobTimeout)
	defer cancel()
	return run(ctx, dependencies{job: service, output: os.Stdout})
}

func run(ctx context.Context, deps dependencies) error {
	summary, err := deps.job.SyncAllUsers(ctx)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(deps.output, "attempted=%d succeeded=%d failed=%d\n", summary.Attempted, summary.Succeeded, summary.Failed)
	return err
}

func commandExitCode(err error) int {
	if err != nil {
		return 1
	}
	return 0
}
