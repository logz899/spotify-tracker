package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"spotify-backend/internal/config"
	"spotify-backend/internal/database"
	"spotify-backend/internal/models"
	"spotify-backend/internal/security"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

func TestSpotifyLoginPersistsGuestAndAuthenticatedState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(1_700_000_000, 0).UTC()
	for _, tt := range []struct {
		name       string
		contextID  *int
		wantUserID *int
	}{
		{name: "guest"},
		{name: "authenticated", contextID: intPointer(17), wantUserID: intPointer(17)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeOAuthStore{}
			client := &fakeSpotifyOAuthClient{}
			handler := newOAuthTestHandler(store, client, now)
			response := performHandlerRequest(http.MethodGet, "/login", "", tt.contextID, handler.SpotifyLogin)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if !equalOptionalInt(store.createdStateUserID, tt.wantUserID) {
				t.Fatalf("stored user = %v, want %v", store.createdStateUserID, tt.wantUserID)
			}
			if store.createdStateExpiry != now.Add(10*time.Minute) {
				t.Fatalf("state expiry = %s", store.createdStateExpiry)
			}
			if err := security.ValidateOpaqueToken(client.authState); err != nil {
				t.Fatalf("Spotify state is not a valid opaque token: %v", err)
			}
			hash := security.HashOpaqueToken(client.authState)
			if string(store.createdStateHash) != string(hash[:]) {
				t.Fatal("stored state hash does not match raw Spotify state")
			}
			if store.cleanupCalls != 1 {
				t.Fatalf("cleanup calls = %d, want 1", store.cleanupCalls)
			}
		})
	}
}

func TestSpotifyCallbackRejectsMissingInvalidAndExpiredState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(1_700_000_000, 0).UTC()
	validState, _, _ := security.NewOpaqueToken()
	tests := []struct {
		name       string
		target     string
		consumeErr error
	}{
		{name: "missing code", target: "/callback?state=" + validState},
		{name: "missing state", target: "/callback?code=code"},
		{name: "malformed state", target: "/callback?code=code&state=bad!"},
		{name: "expired or replayed state", target: "/callback?code=code&state=" + validState, consumeErr: database.ErrOAuthStateInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeOAuthStore{consumeStateErr: tt.consumeErr}
			client := &fakeSpotifyOAuthClient{}
			response := performHandlerRequest(http.MethodGet, tt.target, "", nil, newOAuthTestHandler(store, client, now).SpotifyCallback)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if client.exchangeCalls != 0 {
				t.Fatal("Spotify exchange ran before state validation")
			}
		})
	}
}

func TestSpotifyCallbackUsesStoredUserAndMapsSpotifyFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(1_700_000_000, 0).UTC()
	state, _, _ := security.NewOpaqueToken()

	t.Run("Spotify failure", func(t *testing.T) {
		store := &fakeOAuthStore{}
		client := &fakeSpotifyOAuthClient{exchangeErr: errors.New("upstream unavailable")}
		response := performHandlerRequest(http.MethodGet, "/callback?code=code&state="+state, "", nil, newOAuthTestHandler(store, client, now).SpotifyCallback)
		if response.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502", response.Code)
		}
	})

	t.Run("stored user cannot be reassigned by request context", func(t *testing.T) {
		store := &fakeOAuthStore{consumeStateUserID: intPointer(17)}
		client := &fakeSpotifyOAuthClient{token: testOAuthToken(now)}
		handler := newOAuthTestHandler(store, client, now)
		response := performHandlerRequest(http.MethodGet, "/callback?code=code&state="+state, "", intPointer(99), handler.SpotifyCallback)
		if response.Code != http.StatusTemporaryRedirect {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
		if store.savedToken == nil || store.savedToken.UserID != 17 {
			t.Fatalf("saved user = %v, want state-bound user 17", store.savedToken)
		}
		if got := response.Header().Get("Location"); got != "http://localhost:3000/#/home?spotify_linked=true" {
			t.Fatalf("Location = %q", got)
		}
	})
}

func TestSpotifyHandlersMapStoreAvailabilityFailureToServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(1_700_000_000, 0).UTC()
	state, _, _ := security.NewOpaqueToken()

	t.Run("state creation", func(t *testing.T) {
		store := &fakeOAuthStore{createStateErr: errors.New("database unavailable")}
		response := performHandlerRequest(http.MethodGet, "/login", "", nil, newOAuthTestHandler(store, &fakeSpotifyOAuthClient{}, now).SpotifyLogin)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", response.Code)
		}
	})

	t.Run("state consumption", func(t *testing.T) {
		store := &fakeOAuthStore{consumeStateErr: errors.New("database unavailable")}
		response := performHandlerRequest(http.MethodGet, "/callback?code=code&state="+state, "", nil, newOAuthTestHandler(store, &fakeSpotifyOAuthClient{}, now).SpotifyCallback)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", response.Code)
		}
	})
}

func TestSpotifyCallbackGuestCreatesPendingFragmentKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(1_700_000_000, 0).UTC()
	state, _, _ := security.NewOpaqueToken()
	store := &fakeOAuthStore{}
	client := &fakeSpotifyOAuthClient{token: testOAuthToken(now)}
	response := performHandlerRequest(http.MethodGet, "/callback?code=code&state="+state, "", nil, newOAuthTestHandler(store, client, now).SpotifyCallback)

	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	location := response.Header().Get("Location")
	const prefix = "http://localhost:3000/#/?spotify_connected=true&key="
	if !strings.HasPrefix(location, prefix) {
		t.Fatalf("Location = %q, want fragment prefix %q", location, prefix)
	}
	rawKey := strings.TrimPrefix(location, prefix)
	if err := security.ValidateOpaqueToken(rawKey); err != nil {
		t.Fatalf("pending key is invalid: %v", err)
	}
	hash := security.HashOpaqueToken(rawKey)
	if string(store.createdPendingHash) != string(hash[:]) {
		t.Fatal("pending hash does not match fragment key")
	}
	if store.createdPendingExpiry != now.Add(10*time.Minute) {
		t.Fatalf("pending expiry = %s", store.createdPendingExpiry)
	}
}

func TestLinkPendingSpotifyRejectsInvalidOrExpiredKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(1_700_000_000, 0).UTC()
	validKey, _, _ := security.NewOpaqueToken()
	for _, tt := range []struct {
		name  string
		body  string
		error error
	}{
		{name: "malformed", body: `{"pending_key":"bad!"}`},
		{name: "expired", body: `{"pending_key":"` + validKey + `"}`, error: database.ErrPendingSpotifyTokenInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeOAuthStore{consumePendingErr: tt.error}
			handler := newOAuthTestHandler(store, &fakeSpotifyOAuthClient{}, now)
			response := performHandlerRequest(http.MethodPost, "/link", tt.body, intPointer(23), handler.LinkPendingSpotify)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func newOAuthTestHandler(store OAuthStore, client SpotifyOAuthClient, now time.Time) *Handler {
	return &Handler{
		oauthStore:  store,
		oauthClient: client,
		cfg:         &config.Config{FrontendURL: "http://localhost:3000"},
		now:         func() time.Time { return now },
	}
}

func performHandlerRequest(method, target, body string, userID *int, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("Accept", "application/json")
	if userID != nil {
		context.Set("userID", *userID)
	}
	handler(context)
	return response
}

func testOAuthToken(now time.Time) *oauth2.Token {
	return &oauth2.Token{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", Expiry: now.Add(time.Hour)}
}

func intPointer(value int) *int { return &value }

func equalOptionalInt(first, second *int) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}

type fakeSpotifyOAuthClient struct {
	authState     string
	token         *oauth2.Token
	exchangeErr   error
	exchangeCalls int
}

func (f *fakeSpotifyOAuthClient) GetAuthURL(state string) string {
	f.authState = state
	return "https://accounts.example.test/authorize?state=" + state
}

func (f *fakeSpotifyOAuthClient) GetToken(_ context.Context, _ string) (*oauth2.Token, error) {
	f.exchangeCalls++
	return f.token, f.exchangeErr
}

type fakeOAuthStore struct {
	createdStateHash     []byte
	createdStateUserID   *int
	createdStateExpiry   time.Time
	createStateErr       error
	consumeStateUserID   *int
	consumeStateErr      error
	createdPendingHash   []byte
	createdPendingToken  models.SpotifyToken
	createdPendingExpiry time.Time
	consumePendingUserID int
	consumePendingErr    error
	savedToken           *models.SpotifyToken
	cleanupCalls         int
}

func (f *fakeOAuthStore) CreateOAuthState(_ context.Context, hash []byte, userID *int, expiresAt time.Time) error {
	f.createdStateHash = append([]byte(nil), hash...)
	if userID != nil {
		f.createdStateUserID = intPointer(*userID)
	}
	f.createdStateExpiry = expiresAt
	return f.createStateErr
}

func (f *fakeOAuthStore) ConsumeOAuthState(_ context.Context, _ []byte, _ time.Time) (*int, error) {
	return f.consumeStateUserID, f.consumeStateErr
}

func (f *fakeOAuthStore) CreatePendingSpotifyToken(_ context.Context, hash []byte, token models.SpotifyToken, expiresAt time.Time) error {
	f.createdPendingHash = append([]byte(nil), hash...)
	f.createdPendingToken = token
	f.createdPendingExpiry = expiresAt
	return nil
}

func (f *fakeOAuthStore) ConsumePendingSpotifyToken(_ context.Context, _ []byte, userID int, _ time.Time) error {
	f.consumePendingUserID = userID
	return f.consumePendingErr
}

func (f *fakeOAuthStore) SaveSpotifyToken(_ context.Context, token *models.SpotifyToken) error {
	copy := *token
	f.savedToken = &copy
	return nil
}

func (f *fakeOAuthStore) DeleteExpiredOAuthSessions(_ context.Context, _ time.Time) (int64, error) {
	f.cleanupCalls++
	return 0, nil
}

func decodeJSONBody(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}
