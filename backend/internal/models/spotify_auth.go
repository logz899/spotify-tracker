package models

// SpotifyProfile contains the stable identity and public display metadata
// approved for account linking.
type SpotifyProfile struct {
	AccountID    string
	LegacyUserID string
	DisplayName  string
	AvatarURL    string
	ProfileURL   string
}

// SpotifyAuthorization binds an OAuth token to Spotify's stable user ID.
type SpotifyAuthorization struct {
	Profile SpotifyProfile
	Token   SpotifyToken
}
