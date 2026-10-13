package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"spotify-backend/internal/models"
	"strings"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

type Client struct {
	clientID       string
	clientSecret   string
	redirectURI    string
	profileRequest func(context.Context, *oauth2.Token) (*http.Response, error)
	auth           *spotifyauth.Authenticator
}

const spotifyProfileURL = "https://api.spotify.com/v1/me"

// New creates a new Spotify client
func New(clientID, clientSecret, redirectURI string) *Client {
	auth := spotifyauth.New(
		spotifyauth.WithClientID(clientID),
		spotifyauth.WithClientSecret(clientSecret),
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserLibraryRead,
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopeUserReadRecentlyPlayed,
			spotifyauth.ScopeUserReadCurrentlyPlaying,
		),
	)

	client := &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		auth:         auth,
	}
	client.profileRequest = client.requestCurrentUserProfile
	return client
}

func (c *Client) GetAuthURL(state string) string {
	return c.auth.AuthURL(state)
}

func (c *Client) GetToken(ctx context.Context, code string) (*oauth2.Token, error) {
	return c.auth.Exchange(ctx, code)
}

func (c *Client) GetCurrentUserProfile(ctx context.Context, token *oauth2.Token) (models.SpotifyProfile, error) {
	type image struct {
		URL string `json:"url"`
	}
	type profileResponse struct {
		AccountID    string            `json:"account_id"`
		LegacyUserID string            `json:"id"`
		DisplayName  string            `json:"display_name"`
		Images       []image           `json:"images"`
		ExternalURLs map[string]string `json:"external_urls"`
	}

	response, err := c.profileRequest(ctx, token)
	if err != nil {
		return models.SpotifyProfile{}, errors.New("request current Spotify profile")
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return models.SpotifyProfile{}, fmt.Errorf("request current Spotify profile: unexpected status %d", response.StatusCode)
	}

	var decoded profileResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return models.SpotifyProfile{}, errors.New("decode current Spotify profile response")
	}

	accountID := strings.TrimSpace(decoded.AccountID)
	if accountID == "" {
		return models.SpotifyProfile{}, errors.New("spotify profile has no account ID")
	}

	var avatarURL string
	for _, candidate := range decoded.Images {
		if trimmed := strings.TrimSpace(candidate.URL); trimmed != "" {
			avatarURL = trimmed
			break
		}
	}

	return models.SpotifyProfile{
		AccountID:    accountID,
		LegacyUserID: strings.TrimSpace(decoded.LegacyUserID),
		DisplayName:  strings.TrimSpace(decoded.DisplayName),
		AvatarURL:    avatarURL,
		ProfileURL:   strings.TrimSpace(decoded.ExternalURLs["spotify"]),
	}, nil
}

func (c *Client) requestCurrentUserProfile(ctx context.Context, token *oauth2.Token) (*http.Response, error) {
	return c.auth.Client(ctx, token).Get(spotifyProfileURL)
}

func (c *Client) GetRecentlyPlayed(ctx context.Context, token *oauth2.Token) ([]models.Song, error) {
	httpClient := c.auth.Client(ctx, token)
	client := spotify.New(httpClient)

	recentlyPlayed, err := client.PlayerRecentlyPlayed(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get recently played tracks: %w", err)
	}

	// Use a composite key of song name + artist name to handle duplicate song names by different artists
	songCount := make(map[string]*models.Song)

	for _, item := range recentlyPlayed {
		track := item.Track
		songName := track.Name

		// Get artist name (use first artist if multiple)
		var artistName string
		if len(track.Artists) > 0 {
			artistName = track.Artists[0].Name
		} else {
			artistName = "Unknown Artist"
		}

		// Get album name
		albumName := track.Album.Name

		var imageURL string
		if len(track.Album.Images) > 1 {
			imageURL = track.Album.Images[1].URL
		} else if len(track.Album.Images) > 0 {
			imageURL = track.Album.Images[0].URL
		}

		// Use composite key to differentiate songs with same name by different artists
		compositeKey := fmt.Sprintf("%s|%s", songName, artistName)

		if _, exists := songCount[compositeKey]; !exists {
			songCount[compositeKey] = &models.Song{
				SongName:      songName,
				AuthorName:    artistName,
				AlbumName:     albumName,
				ImageURL:      imageURL,
				SongCount:     0,
				ListeningDate: item.PlayedAt,
			}
		} else {
			songCount[compositeKey].SongCount++
			// Update to most recent listening date
			if item.PlayedAt.After(songCount[compositeKey].ListeningDate) {
				songCount[compositeKey].ListeningDate = item.PlayedAt
			}
		}

		songCount[compositeKey].SongCount++
	}

	songs := make([]models.Song, 0, len(songCount))
	for _, song := range songCount {
		songs = append(songs, *song)
	}

	sort.Sort(models.SongList(songs))

	log.Printf("Processed %d unique songs from %d recently played tracks", len(songs), len(recentlyPlayed))

	return songs, nil
}
