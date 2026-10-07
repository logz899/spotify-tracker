package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"spotify-backend/internal/models"

	"golang.org/x/oauth2"
)

func TestSyncAllUsersReturnsZeroSummaryWhenNoUsersExist(t *testing.T) {
	store := &fakeSyncStore{lockAcquired: true}
	service := NewSyncService(store, &fakeSyncSpotifyClient{})

	summary, err := service.SyncAllUsers(context.Background())

	if err != nil {
		t.Fatalf("SyncAllUsers() error = %v", err)
	}
	if summary != (SyncSummary{}) {
		t.Fatalf("summary = %#v, want zero", summary)
	}
	if store.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", store.releaseCalls)
	}
}

func TestSyncAllUsersReportsAllSuccessfulUsers(t *testing.T) {
	store := newFakeSyncStore(1, 2)
	client := &fakeSyncSpotifyClient{songs: []models.Song{{SongName: "Song"}}}
	service := NewSyncService(store, client)

	summary, err := service.SyncAllUsers(context.Background())

	if err != nil {
		t.Fatalf("SyncAllUsers() error = %v", err)
	}
	want := SyncSummary{Attempted: 2, Succeeded: 2}
	if summary != want {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
	if client.calls != 2 || len(store.insertedFor) != 2 {
		t.Fatalf("Spotify calls = %d, inserts = %#v", client.calls, store.insertedFor)
	}
}

func TestSyncAllUsersContinuesAfterPerUserFailureWithoutLeakingSecrets(t *testing.T) {
	store := newFakeSyncStore(1, 2)
	store.users[0].Username = "private-user"
	store.users[0].Email = "private@example.test"
	client := &fakeSyncSpotifyClient{
		songs:       []models.Song{{SongName: "Song"}},
		failOnCall:  1,
		failureText: "access-token-secret",
	}
	service := NewSyncService(store, client)
	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	summary, err := service.SyncAllUsers(context.Background())

	if err != nil {
		t.Fatalf("SyncAllUsers() error = %v", err)
	}
	want := SyncSummary{Attempted: 2, Succeeded: 1, Failed: 1}
	if summary != want {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
	if client.calls != 2 || len(store.insertedFor) != 1 || store.insertedFor[0] != 2 {
		t.Fatalf("later user was not synchronized: calls=%d inserts=%#v", client.calls, store.insertedFor)
	}
	assertDoesNotContain(t, logs.String(), "private-user", "private@example.test", "access-token-secret")
}

func TestSyncAllUsersCountsTokenReadFailureAndContinues(t *testing.T) {
	store := newFakeSyncStore(1, 2)
	store.tokenErrors = map[int]error{1: errors.New("refresh-token-secret")}
	client := &fakeSyncSpotifyClient{songs: []models.Song{{SongName: "Song"}}}
	service := NewSyncService(store, client)

	summary, err := service.SyncAllUsers(context.Background())

	if err != nil {
		t.Fatalf("SyncAllUsers() error = %v", err)
	}
	want := SyncSummary{Attempted: 2, Succeeded: 1, Failed: 1}
	if summary != want {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
	if client.calls != 1 || len(store.insertedFor) != 1 || store.insertedFor[0] != 2 {
		t.Fatalf("unexpected continuation: calls=%d inserts=%#v", client.calls, store.insertedFor)
	}
}

func TestSyncAllUsersMarksFailedWhenCompletionStatusCannotBeSaved(t *testing.T) {
	store := newFakeSyncStore(1, 2)
	store.statusErrors = map[string]error{"1:completed": errors.New("status unavailable")}
	client := &fakeSyncSpotifyClient{songs: []models.Song{{SongName: "Song"}}}
	service := NewSyncService(store, client)

	summary, err := service.SyncAllUsers(context.Background())

	if err != nil {
		t.Fatalf("SyncAllUsers() error = %v", err)
	}
	want := SyncSummary{Attempted: 2, Succeeded: 1, Failed: 1}
	if summary != want {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
	statuses := store.statuses[1]
	if len(statuses) != 3 || statuses[2] != "failed" {
		t.Fatalf("user 1 statuses = %#v, want syncing/completed/failed", statuses)
	}
}

func TestSyncAllUsersStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := newFakeSyncStore(1, 2)
	client := &fakeSyncSpotifyClient{cancel: cancel}
	service := NewSyncService(store, client)

	summary, err := service.SyncAllUsers(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SyncAllUsers() error = %v, want context.Canceled", err)
	}
	if summary.Attempted != 1 || client.calls != 1 {
		t.Fatalf("summary = %#v, Spotify calls = %d, want one attempt", summary, client.calls)
	}
}

func TestSyncAllUsersFailsWhenLockIsUnavailable(t *testing.T) {
	store := &fakeSyncStore{}
	service := NewSyncService(store, &fakeSyncSpotifyClient{})

	summary, err := service.SyncAllUsers(context.Background())

	if !errors.Is(err, ErrSyncAlreadyRunning) {
		t.Fatalf("SyncAllUsers() error = %v, want ErrSyncAlreadyRunning", err)
	}
	if summary != (SyncSummary{}) {
		t.Fatalf("summary = %#v, want zero", summary)
	}
}

func TestSyncAllUsersReturnsJobLevelStoreFailure(t *testing.T) {
	store := &fakeSyncStore{lockAcquired: true, usersErr: errors.New("database unavailable")}
	service := NewSyncService(store, &fakeSyncSpotifyClient{})

	summary, err := service.SyncAllUsers(context.Background())

	if err == nil || !strings.Contains(err.Error(), "list users") {
		t.Fatalf("SyncAllUsers() error = %v, want list users failure", err)
	}
	if summary != (SyncSummary{}) {
		t.Fatalf("summary = %#v, want zero", summary)
	}
}

type fakeSyncStore struct {
	lockAcquired bool
	lockErr      error
	releaseErr   error
	releaseCalls int
	users        []models.User
	usersErr     error
	tokens       map[int]*models.SpotifyToken
	tokenErrors  map[int]error
	statusErrors map[string]error
	statuses     map[int][]string
	insertErrors map[int]error
	insertedFor  []int
}

func newFakeSyncStore(userIDs ...int) *fakeSyncStore {
	store := &fakeSyncStore{
		lockAcquired: true,
		tokens:       make(map[int]*models.SpotifyToken),
	}
	for _, userID := range userIDs {
		store.users = append(store.users, models.User{ID: userID})
		store.tokens[userID] = &models.SpotifyToken{
			UserID:       userID,
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(time.Hour),
		}
	}
	return store
}

func (f *fakeSyncStore) TrySyncLock(context.Context) (bool, func() error, error) {
	return f.lockAcquired, func() error {
		f.releaseCalls++
		return f.releaseErr
	}, f.lockErr
}

func (f *fakeSyncStore) GetAllUsersWithSpotifyTokens(context.Context) ([]models.User, error) {
	return f.users, f.usersErr
}

func (f *fakeSyncStore) GetSpotifyToken(_ context.Context, userID int) (*models.SpotifyToken, error) {
	if err := f.tokenErrors[userID]; err != nil {
		return nil, err
	}
	return f.tokens[userID], nil
}

func (f *fakeSyncStore) InsertSongs(_ context.Context, userID int, _ []models.Song) error {
	if err := f.insertErrors[userID]; err != nil {
		return err
	}
	f.insertedFor = append(f.insertedFor, userID)
	return nil
}

func (f *fakeSyncStore) UpdateUserSyncStatus(_ context.Context, userID int, status string) error {
	if f.statuses == nil {
		f.statuses = make(map[int][]string)
	}
	f.statuses[userID] = append(f.statuses[userID], status)
	return f.statusErrors[fmt.Sprintf("%d:%s", userID, status)]
}

type fakeSyncSpotifyClient struct {
	songs       []models.Song
	failOnCall  int
	failureText string
	cancel      context.CancelFunc
	calls       int
}

func (f *fakeSyncSpotifyClient) GetRecentlyPlayed(ctx context.Context, _ *oauth2.Token) ([]models.Song, error) {
	f.calls++
	if f.cancel != nil {
		f.cancel()
		return nil, ctx.Err()
	}
	if f.calls == f.failOnCall {
		return nil, errors.New(f.failureText)
	}
	return f.songs, nil
}

func assertDoesNotContain(t *testing.T, value string, forbidden ...string) {
	t.Helper()
	for _, item := range forbidden {
		if strings.Contains(value, item) {
			t.Fatalf("value contains sensitive text %q: %s", item, value)
		}
	}
}
