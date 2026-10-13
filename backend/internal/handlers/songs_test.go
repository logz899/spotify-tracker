package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"spotify-backend/internal/models"

	"golang.org/x/oauth2"
)

func TestGetAllSongsReturnsDatabaseResultsWithoutCache(t *testing.T) {
	store := &fakeSongStore{songs: []models.Song{{SongID: 7, SongName: "Test Song", AuthorName: "Test Artist"}}}
	handler := &Handler{songStore: store}
	response := performHandlerRequest(http.MethodGet, "/api/v1/songs/all", "", intPointer(42), handler.GetAllSongs)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Message string        `json:"message"`
		Data    []models.Song `json:"data"`
		Cached  bool          `json:"cached"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Message != "Retrieved 1 songs" || body.Cached || len(body.Data) != 1 || body.Data[0].SongName != "Test Song" {
		t.Fatalf("unexpected response: %#v", body)
	}
	if store.requestedUserID != 42 {
		t.Fatalf("database user ID = %d, want 42", store.requestedUserID)
	}
}

func TestGetAllSongsReturnsEmptyArray(t *testing.T) {
	handler := &Handler{songStore: &fakeSongStore{}}
	response := performHandlerRequest(http.MethodGet, "/api/v1/songs/all", "", intPointer(42), handler.GetAllSongs)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Data []models.Song `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data == nil || len(body.Data) != 0 {
		t.Fatalf("data = %#v, want empty JSON array", body.Data)
	}
}

func TestGetAllSongsMapsDatabaseFailureToServiceUnavailable(t *testing.T) {
	handler := &Handler{songStore: &fakeSongStore{err: errors.New("sensitive database detail")}}
	response := performHandlerRequest(http.MethodGet, "/api/v1/songs/all", "", intPointer(42), handler.GetAllSongs)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Body.String() != `{"error":"Songs are temporarily unavailable"}` {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestManualSyncRecordsSyncingThenCompleted(t *testing.T) {
	store := &fakeManualSyncStore{token: testStoredSpotifyToken()}
	client := &fakeSpotifySongClient{songs: []models.Song{{SongName: "Synced Song"}}}
	handler := &Handler{manualSyncStore: store, spotifySongClient: client}
	response := performHandlerRequest(http.MethodGet, "/api/v1/songs/recent", "", intPointer(42), handler.GetRecentSongs)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := strings.Join(store.statuses, ","); got != "syncing,completed" {
		t.Fatalf("sync statuses = %q, want syncing,completed", got)
	}
	if store.insertedUserID != 42 || len(store.insertedSongs) != 1 || store.insertedSongs[0].SongName != "Synced Song" {
		t.Fatalf("inserted songs = %#v for user %d", store.insertedSongs, store.insertedUserID)
	}
}

func TestManualSyncRecordsFailed(t *testing.T) {
	for _, tt := range []struct {
		name      string
		clientErr error
		insertErr error
		wantCode  int
	}{
		{name: "Spotify failure", clientErr: errors.New("Spotify unavailable"), wantCode: http.StatusBadGateway},
		{name: "persistence failure", insertErr: errors.New("database unavailable"), wantCode: http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeManualSyncStore{token: testStoredSpotifyToken(), insertErr: tt.insertErr}
			client := &fakeSpotifySongClient{songs: []models.Song{{SongName: "Song"}}, err: tt.clientErr}
			handler := &Handler{manualSyncStore: store, spotifySongClient: client}
			response := performHandlerRequest(http.MethodGet, "/api/v1/songs/recent", "", intPointer(42), handler.GetRecentSongs)

			if response.Code != tt.wantCode {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if got := strings.Join(store.statuses, ","); got != "syncing,failed" {
				t.Fatalf("sync statuses = %q, want syncing,failed", got)
			}
		})
	}
}

func TestManualSyncUnlinkedDoesNotChangeStatus(t *testing.T) {
	store := &fakeManualSyncStore{tokenErr: errors.New("not found")}
	handler := &Handler{manualSyncStore: store, spotifySongClient: &fakeSpotifySongClient{}}
	response := performHandlerRequest(http.MethodGet, "/api/v1/songs/recent", "", intPointer(42), handler.GetRecentSongs)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "connect your Spotify account") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if len(store.statuses) != 0 {
		t.Fatalf("unlinked sync statuses = %#v, want none", store.statuses)
	}
}

func TestManualSyncFailureIsSafe(t *testing.T) {
	const (
		privateDetail = "upstream private response"
		accessToken   = "private-access-token"
	)
	store := &fakeManualSyncStore{token: &models.SpotifyToken{AccessToken: accessToken, TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}}
	client := &fakeSpotifySongClient{err: errors.New(privateDetail)}
	handler := &Handler{manualSyncStore: store, spotifySongClient: client}

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })
	response := performHandlerRequest(http.MethodGet, "/api/v1/songs/recent", "", intPointer(42), handler.GetRecentSongs)

	combined := response.Body.String() + logs.String()
	for _, secret := range []string{privateDetail, accessToken} {
		if strings.Contains(combined, secret) {
			t.Fatalf("manual sync output exposed %q: %s", secret, combined)
		}
	}
}

type fakeSongStore struct {
	songs           []models.Song
	err             error
	requestedUserID int
}

func (f *fakeSongStore) GetAllSongs(_ context.Context, userID int) ([]models.Song, error) {
	f.requestedUserID = userID
	return f.songs, f.err
}

type fakeManualSyncStore struct {
	token          *models.SpotifyToken
	tokenErr       error
	insertErr      error
	statusErr      error
	statuses       []string
	insertedUserID int
	insertedSongs  []models.Song
}

func (f *fakeManualSyncStore) GetSpotifyToken(context.Context, int) (*models.SpotifyToken, error) {
	return f.token, f.tokenErr
}

func (f *fakeManualSyncStore) InsertSongs(_ context.Context, userID int, songs []models.Song) error {
	f.insertedUserID = userID
	f.insertedSongs = append([]models.Song(nil), songs...)
	return f.insertErr
}

func (f *fakeManualSyncStore) UpdateUserSyncStatus(_ context.Context, _ int, status string) error {
	f.statuses = append(f.statuses, status)
	return f.statusErr
}

type fakeSpotifySongClient struct {
	songs []models.Song
	err   error
}

func (f *fakeSpotifySongClient) GetRecentlyPlayed(context.Context, *oauth2.Token) ([]models.Song, error) {
	return f.songs, f.err
}

func testStoredSpotifyToken() *models.SpotifyToken {
	return &models.SpotifyToken{
		AccessToken:  "stored-access",
		RefreshToken: "stored-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
}
