package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"spotify-backend/internal/auth"
	"spotify-backend/internal/models"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginUsesNormalizedEmail(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("long-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	store := &fakeAuthStore{
		wantLookupEmail: "listener@example.test",
		emailUser: &models.User{
			ID:           7,
			Username:     "listener",
			Email:        "listener@example.test",
			PasswordHash: string(passwordHash),
		},
	}
	handler := &Handler{authStore: store, jwtManager: newTestJWTManager(t)}

	response := performHandlerRequest(http.MethodPost, "/login", `{"email":" Listener@Example.Test ","password":"long-password"}`, nil, handler.Login)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := decodeJSONBody(t, response)
	if token, ok := body["token"].(string); !ok || token == "" {
		t.Fatalf("token = %#v, want a signed JWT", body["token"])
	}
	if store.lookupMismatch {
		t.Fatalf("email lookup did not use normalized value %q", store.wantLookupEmail)
	}
}

func TestLoginHidesUnknownEmailAndWrongPassword(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("different-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}

	tests := []struct {
		name  string
		store *fakeAuthStore
	}{
		{name: "unknown email", store: &fakeAuthStore{}},
		{name: "wrong password", store: &fakeAuthStore{emailUser: &models.User{PasswordHash: string(passwordHash)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{authStore: tt.store, jwtManager: newTestJWTManager(t)}
			response := performHandlerRequest(http.MethodPost, "/login", `{"email":"listener@example.test","password":"long-password"}`, nil, handler.Login)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			body := decodeJSONBody(t, response)
			if body["error"] != "Invalid email or password" {
				t.Fatalf("error = %#v, want generic authentication error", body["error"])
			}
		})
	}
}

func TestRegisterNormalizesEmailBeforePersistence(t *testing.T) {
	store := &fakeAuthStore{}
	handler := &Handler{authStore: store, jwtManager: newTestJWTManager(t)}

	response := performHandlerRequest(http.MethodPost, "/register", `{"username":"listener","email":" Listener@Example.Test ","password":"1234567890"}`, nil, handler.Register)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if store.createdUser == nil {
		t.Fatal("CreateUser was not called")
	}
	if store.createdUser.Email != "listener@example.test" {
		t.Fatalf("created email = %q, want normalized email", store.createdUser.Email)
	}
}

func TestRegisterRejectsPasswordShorterThanTenCharacters(t *testing.T) {
	handler := &Handler{}
	response := performHandlerRequest(http.MethodPost, "/register", `{"username":"listener","email":"listener@example.test","password":"123456789"}`, nil, handler.Register)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := decodeJSONBody(t, response)
	if _, exists := body["details"]; exists {
		t.Fatal("validation response exposed internal details")
	}
}

func TestRegisterDoesNotExposeDatabaseErrors(t *testing.T) {
	handler := &Handler{authStore: &fakeAuthStore{createErr: errors.New("sensitive SQL detail")}}
	response := performHandlerRequest(http.MethodPost, "/register", `{"username":"listener","email":"listener@example.test","password":"1234567890"}`, nil, handler.Register)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "sensitive SQL detail") {
		t.Fatal("response exposed database details")
	}
}

func TestGetCurrentUserProfileLinked(t *testing.T) {
	lastSynced := time.Date(2026, time.October, 12, 9, 0, 0, 0, time.UTC)
	store := &fakeProfileStore{profile: &models.UserProfile{
		UserID:             17,
		Username:           "listener",
		UpdateStatus:       "completed",
		UpdateDate:         &lastSynced,
		SpotifyLinked:      true,
		SpotifyDisplayName: "Listener",
		SpotifyAvatarURL:   "https://i.scdn.co/image/example",
		SpotifyProfileURL:  "https://open.spotify.com/user/example",
	}}
	handler := &Handler{profileStore: store}
	response := performHandlerRequest(http.MethodGet, "/api/v1/user/me", "", intPointer(17), handler.GetCurrentUser)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		User struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
		} `json:"user"`
		Spotify struct {
			Linked      bool   `json:"linked"`
			DisplayName string `json:"display_name"`
			AvatarURL   string `json:"avatar_url"`
			ProfileURL  string `json:"profile_url"`
		} `json:"spotify"`
		Sync struct {
			Status       *string    `json:"status"`
			LastSyncedAt *time.Time `json:"last_synced_at"`
		} `json:"sync"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if body.User.ID != 17 || body.User.Username != "listener" || !body.Spotify.Linked ||
		body.Spotify.DisplayName != "Listener" || body.Spotify.AvatarURL != "https://i.scdn.co/image/example" ||
		body.Spotify.ProfileURL != "https://open.spotify.com/user/example" || body.Sync.Status == nil ||
		*body.Sync.Status != "completed" || body.Sync.LastSyncedAt == nil || !body.Sync.LastSyncedAt.Equal(lastSynced) {
		t.Fatalf("unexpected linked profile response: %s", response.Body.String())
	}
	for _, forbidden := range []string{"account_id", "spotify_user_id", "email", "access_token", "refresh_token"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("profile response exposed forbidden field %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestGetCurrentUserProfileUnlinked(t *testing.T) {
	store := &fakeProfileStore{profile: &models.UserProfile{UserID: 18, Username: "local-listener"}}
	handler := &Handler{profileStore: store}
	response := performHandlerRequest(http.MethodGet, "/api/v1/user/me", "", intPointer(18), handler.GetCurrentUser)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	spotifyBody := body["spotify"].(map[string]any)
	if spotifyBody["linked"] != false || len(spotifyBody) != 1 {
		t.Fatalf("spotify response = %#v, want only linked:false", spotifyBody)
	}
	syncBody := body["sync"].(map[string]any)
	if status, exists := syncBody["status"]; !exists || status != nil {
		t.Fatalf("sync status = %#v, want null", syncBody)
	}
	if _, exists := syncBody["last_synced_at"]; exists {
		t.Fatalf("last_synced_at should be omitted: %#v", syncBody)
	}
}

func TestGetCurrentUserProfileRequiresJWT(t *testing.T) {
	handler := &Handler{profileStore: &fakeProfileStore{}}
	response := performHandlerRequest(http.MethodGet, "/api/v1/user/me", "", nil, handler.GetCurrentUser)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestGetCurrentUserProfileStoreFailureIsGeneric(t *testing.T) {
	handler := &Handler{profileStore: &fakeProfileStore{err: errors.New("sensitive database detail")}}
	response := performHandlerRequest(http.MethodGet, "/api/v1/user/me", "", intPointer(17), handler.GetCurrentUser)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "sensitive database detail") {
		t.Fatalf("response exposed store failure: %s", response.Body.String())
	}
}

type fakeProfileStore struct {
	profile         *models.UserProfile
	err             error
	requestedUserID int
}

func (f *fakeProfileStore) GetUserProfile(_ context.Context, userID int) (*models.UserProfile, error) {
	f.requestedUserID = userID
	return f.profile, f.err
}

type fakeAuthStore struct {
	createErr       error
	emailUser       *models.User
	wantLookupEmail string
	lookupMismatch  bool
	createdUser     *models.User
	idUser          *models.User
}

func (f *fakeAuthStore) GetUserByID(context.Context, int) (*models.User, error) {
	if f.idUser == nil {
		return nil, errors.New("not found")
	}
	return f.idUser, nil
}

func (f *fakeAuthStore) GetUserByUsername(context.Context, string) (*models.User, error) {
	return nil, errors.New("not found")
}

func (f *fakeAuthStore) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	if f.wantLookupEmail != "" && email != f.wantLookupEmail {
		f.lookupMismatch = true
		return nil, errors.New("not found")
	}
	if f.emailUser == nil {
		return nil, errors.New("not found")
	}
	return f.emailUser, nil
}

func (f *fakeAuthStore) CreateUser(_ context.Context, user *models.User) error {
	user.ID = 7
	copy := *user
	f.createdUser = &copy
	return f.createErr
}

func newTestJWTManager(t *testing.T) *auth.Manager {
	t.Helper()
	manager, err := auth.NewManager([]byte(strings.Repeat("s", 32)), "test-issuer", "test-audience", time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}
