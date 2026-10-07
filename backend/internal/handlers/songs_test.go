package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"spotify-backend/internal/models"
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

type fakeSongStore struct {
	songs           []models.Song
	err             error
	requestedUserID int
}

func (f *fakeSongStore) GetAllSongs(_ context.Context, userID int) ([]models.Song, error) {
	f.requestedUserID = userID
	return f.songs, f.err
}
