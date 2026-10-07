package services

import (
	"context"
	"errors"
	"fmt"
	"log"

	"spotify-backend/internal/models"

	"golang.org/x/oauth2"
)

var ErrSyncAlreadyRunning = errors.New("spotify sync is already running")

type SyncSummary struct {
	Attempted int
	Succeeded int
	Failed    int
}

type SyncStore interface {
	TrySyncLock(context.Context) (acquired bool, release func() error, err error)
	GetAllUsersWithSpotifyTokens(context.Context) ([]models.User, error)
	GetSpotifyToken(context.Context, int) (*models.SpotifyToken, error)
	InsertSongs(context.Context, int, []models.Song) error
	UpdateUserSyncStatus(context.Context, int, string) error
}

type SyncSpotifyClient interface {
	GetRecentlyPlayed(context.Context, *oauth2.Token) ([]models.Song, error)
}

type SyncService struct {
	store         SyncStore
	spotifyClient SyncSpotifyClient
}

func NewSyncService(store SyncStore, spotifyClient SyncSpotifyClient) *SyncService {
	return &SyncService{store: store, spotifyClient: spotifyClient}
}

func (s *SyncService) SyncAllUsers(ctx context.Context) (summary SyncSummary, resultErr error) {
	acquired, release, err := s.store.TrySyncLock(ctx)
	if err != nil {
		return summary, fmt.Errorf("acquire sync lock: %w", err)
	}
	if !acquired {
		return summary, ErrSyncAlreadyRunning
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("release sync lock: %w", releaseErr)
		}
	}()

	users, err := s.store.GetAllUsersWithSpotifyTokens(ctx)
	if err != nil {
		return summary, fmt.Errorf("list users for sync: %w", err)
	}

	for _, user := range users {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		summary.Attempted++
		if err := s.syncUser(ctx, user.ID); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return summary, ctxErr
			}
			summary.Failed++
			continue
		}
		summary.Succeeded++
	}

	log.Printf("Spotify sync finished: attempted=%d succeeded=%d failed=%d", summary.Attempted, summary.Succeeded, summary.Failed)
	return summary, nil
}

func (s *SyncService) syncUser(ctx context.Context, userID int) error {
	if err := s.store.UpdateUserSyncStatus(ctx, userID, "syncing"); err != nil {
		return err
	}

	spotifyToken, err := s.store.GetSpotifyToken(ctx, userID)
	if err != nil {
		s.markFailed(ctx, userID)
		return err
	}
	oauthToken := &oauth2.Token{
		AccessToken:  spotifyToken.AccessToken,
		RefreshToken: spotifyToken.RefreshToken,
		TokenType:    spotifyToken.TokenType,
		Expiry:       spotifyToken.Expiry,
	}

	songs, err := s.spotifyClient.GetRecentlyPlayed(ctx, oauthToken)
	if err != nil {
		s.markFailed(ctx, userID)
		return err
	}
	if err := s.store.InsertSongs(ctx, userID, songs); err != nil {
		s.markFailed(ctx, userID)
		return err
	}
	if err := s.store.UpdateUserSyncStatus(ctx, userID, "completed"); err != nil {
		s.markFailed(ctx, userID)
		return err
	}
	return nil
}

func (s *SyncService) markFailed(ctx context.Context, userID int) {
	_ = s.store.UpdateUserSyncStatus(ctx, userID, "failed")
}
