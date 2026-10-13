package spotify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"spotify-backend/internal/models"

	spotifyapi "github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
)

func TestGetCurrentUserProfileMapsApprovedFields(t *testing.T) {
	const accessToken = "test-access-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+accessToken {
			t.Errorf("Authorization header = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"account_id":" stable-account ",
			"id":" legacy-user ",
			"display_name":"Listener",
			"images":[{"url":"https://i.scdn.co/image/first"},{"url":"https://i.scdn.co/image/second"}],
			"external_urls":{"spotify":"https://open.spotify.com/user/legacy-user"},
			"email":"must-not-be-mapped",
			"country":"TW"
		}`))
	}))
	defer server.Close()

	client := newProfileTestClient(server.URL)
	got, err := client.GetCurrentUserProfile(context.Background(), validTestToken(accessToken))
	if err != nil {
		t.Fatalf("GetCurrentUserProfile() error = %v", err)
	}

	want := models.SpotifyProfile{
		AccountID:    "stable-account",
		LegacyUserID: "legacy-user",
		DisplayName:  "Listener",
		AvatarURL:    "https://i.scdn.co/image/first",
		ProfileURL:   "https://open.spotify.com/user/legacy-user",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCurrentUserProfile() = %#v, want %#v", got, want)
	}
}

func TestGetCurrentUserProfileSelectsFirstNonEmptyImage(t *testing.T) {
	client, closeServer := profileClientForJSON(t, http.StatusOK, `{
		"account_id":"account-1",
		"id":"legacy-1",
		"images":[{"url":"  "},{"url":" https://i.scdn.co/image/fallback "},{"url":"https://i.scdn.co/image/later"}]
	}`)
	defer closeServer()

	got, err := client.GetCurrentUserProfile(context.Background(), validTestToken("token"))
	if err != nil {
		t.Fatalf("GetCurrentUserProfile() error = %v", err)
	}
	if got.AvatarURL != "https://i.scdn.co/image/fallback" {
		t.Fatalf("AvatarURL = %q, want first non-empty image", got.AvatarURL)
	}
}

func TestGetCurrentUserProfileAllowsMissingImage(t *testing.T) {
	client, closeServer := profileClientForJSON(t, http.StatusOK, `{"account_id":"account-1","id":"legacy-1"}`)
	defer closeServer()

	got, err := client.GetCurrentUserProfile(context.Background(), validTestToken("token"))
	if err != nil {
		t.Fatalf("GetCurrentUserProfile() error = %v", err)
	}
	if got.AvatarURL != "" {
		t.Fatalf("AvatarURL = %q, want empty", got.AvatarURL)
	}
}

func TestGetCurrentUserProfileRejectsMissingAccountID(t *testing.T) {
	for _, accountID := range []string{"", "   "} {
		t.Run("account_id="+accountID, func(t *testing.T) {
			client, closeServer := profileClientForJSON(t, http.StatusOK, `{"account_id":"`+accountID+`","id":"legacy-1"}`)
			defer closeServer()

			got, err := client.GetCurrentUserProfile(context.Background(), validTestToken("token"))
			if err == nil {
				t.Fatal("GetCurrentUserProfile() error = nil, want missing account ID error")
			}
			if got != (models.SpotifyProfile{}) {
				t.Fatalf("GetCurrentUserProfile() profile = %#v, want zero value", got)
			}
		})
	}
}

func TestGetCurrentUserProfileHidesUpstreamDetails(t *testing.T) {
	const (
		accessToken    = "secret-access-token"
		responseSecret = "upstream-private-detail"
	)
	client, closeServer := profileClientForJSON(t, http.StatusUnauthorized, responseSecret)
	defer closeServer()

	got, err := client.GetCurrentUserProfile(context.Background(), validTestToken(accessToken))
	if err == nil {
		t.Fatal("GetCurrentUserProfile() error = nil, want upstream error")
	}
	if got != (models.SpotifyProfile{}) {
		t.Fatalf("GetCurrentUserProfile() profile = %#v, want zero value", got)
	}
	for _, secret := range []string{accessToken, responseSecret} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error %q contains sensitive value %q", err, secret)
		}
	}
}

func TestSelectImageURL(t *testing.T) {
	t.Run("uses Spotify artist image", func(t *testing.T) {
		images := []spotifyapi.Image{{URL: "https://images.example/artist.jpg"}}

		if got := selectImageURL(images, "https://images.example/album.jpg"); got != images[0].URL {
			t.Fatalf("selectImageURL() = %q, want artist image", got)
		}
	})

	t.Run("falls back to album image", func(t *testing.T) {
		const fallback = "https://images.example/album.jpg"

		if got := selectImageURL(nil, fallback); got != fallback {
			t.Fatalf("selectImageURL() = %q, want %q", got, fallback)
		}
	})
}

func newProfileTestClient(profileURL string) *Client {
	client := New("client-id", "client-secret", "http://localhost/callback")
	client.profileRequest = func(ctx context.Context, token *oauth2.Token) (*http.Response, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
		if err != nil {
			return nil, err
		}
		return client.auth.Client(ctx, token).Do(request) //nolint:gosec // profileURL is an httptest URL controlled by this test.
	}
	return client
}

func profileClientForJSON(t *testing.T, status int, body string) (*Client, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	return newProfileTestClient(server.URL), server.Close
}

func validTestToken(accessToken string) *oauth2.Token {
	return &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}
}
