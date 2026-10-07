package models

import "time"

// Song represents a song with its play count and album cover
type Song struct {
	ID            int       `json:"id"`
	SongID        int       `json:"song_id"`
	SongName      string    `json:"song_name"`
	ImageURL      string    `json:"image_url"`
	SongCount     int       `json:"song_count"`
	AuthorID      int       `json:"author_id"`
	AuthorName    string    `json:"author_name,omitempty"`
	AlbumName     string    `json:"album_name,omitempty"`
	ListeningDate time.Time `json:"listening_date"`
}

// SongList is a slice of songs
type SongList []Song

// Len returns the length of the song list (for sorting)
func (s SongList) Len() int {
	return len(s)
}

// Less compares two songs by count in descending order (for sorting)
func (s SongList) Less(i, j int) bool {
	return s[i].SongCount > s[j].SongCount
}

// Swap swaps two songs in the list (for sorting)
func (s SongList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
