package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"spotify-backend/internal/models"
	"spotify-backend/internal/security"
)

func TestUserEmailLookupAndUniquenessAreCaseInsensitive(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	raw, _, err := security.NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	suffix := raw[:12]

	user := &models.User{
		Username:     "email-owner-" + suffix,
		Email:        "Listener-" + suffix + "@Example.Test",
		PasswordHash: "unused-test-hash",
	}
	if err := db.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	found, err := db.GetUserByEmail(ctx, strings.ToLower(user.Email))
	if err != nil {
		t.Fatalf("GetUserByEmail(lowercase) error = %v", err)
	}
	if found.ID != user.ID {
		t.Fatalf("GetUserByEmail(lowercase) ID = %d, want %d", found.ID, user.ID)
	}

	duplicate := &models.User{
		Username:     "email-duplicate-" + suffix,
		Email:        strings.ToUpper(user.Email),
		PasswordHash: "unused-test-hash",
	}
	if err := db.CreateUser(ctx, duplicate); err == nil {
		t.Fatal("CreateUser() accepted an email that differs only by case")
	}
}

func TestGetUserProfileLinked(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, db)
	now := time.Now().UTC()
	profile := models.SpotifyProfile{
		AccountID:    fmt.Sprintf("profile-account-%d", userID),
		LegacyUserID: fmt.Sprintf("profile-legacy-%d", userID),
		DisplayName:  "Profile Listener",
		AvatarURL:    "https://i.scdn.co/image/profile",
		ProfileURL:   "https://open.spotify.com/user/profile",
	}
	if err := db.LinkSpotifyAccount(ctx, userID, models.SpotifyAuthorization{
		Profile: profile,
		Token:   models.SpotifyToken{AccessToken: "profile-access", TokenType: "Bearer", Expiry: now.Add(time.Hour)},
	}); err != nil {
		t.Fatalf("LinkSpotifyAccount() error = %v", err)
	}
	if err := db.UpdateUserSyncStatus(ctx, userID, "completed"); err != nil {
		t.Fatalf("UpdateUserSyncStatus() error = %v", err)
	}

	got, err := db.GetUserProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserProfile() error = %v", err)
	}
	if got.UserID != userID || got.Username == "" || got.UpdateStatus != "completed" || got.UpdateDate == nil ||
		!got.SpotifyLinked || got.SpotifyDisplayName != profile.DisplayName || got.SpotifyAvatarURL != profile.AvatarURL ||
		got.SpotifyProfileURL != profile.ProfileURL {
		t.Fatalf("GetUserProfile() = %#v", got)
	}
}

func TestGetUserProfileUnlinked(t *testing.T) {
	db := newIntegrationDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, db)

	got, err := db.GetUserProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserProfile() error = %v", err)
	}
	if got.UserID != userID || got.Username == "" || got.SpotifyLinked || got.SpotifyDisplayName != "" ||
		got.SpotifyAvatarURL != "" || got.SpotifyProfileURL != "" {
		t.Fatalf("GetUserProfile() = %#v, want unlinked profile", got)
	}
}
