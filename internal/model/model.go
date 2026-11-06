// Package model holds the domain types shared by every layer of the player.
package model

import (
	"fmt"
	"sort"
	"strings"
)

// Song is a single track as exposed by the (Open)Subsonic API.
type Song struct {
	ID          string
	Title       string
	Artist      string
	ArtistID    string
	Album       string
	AlbumID     string
	CoverID     string
	Genre       string
	Suffix      string
	ContentType string
	Path        string
	Track       int
	DiscNumber  int
	Year        int
	Duration    int // seconds
	BitRate     int
	Size        int64
	Starred     bool
	Rating      int
	PlayCount   int
	Created     string
}

// IsZero reports whether the song is the empty value.
func (s Song) IsZero() bool { return s.ID == "" }

// DisplayTitle returns a usable title even for tracks with missing metadata.
func (s Song) DisplayTitle() string {
	if strings.TrimSpace(s.Title) != "" {
		return s.Title
	}
	if s.Path != "" {
		parts := strings.Split(strings.ReplaceAll(s.Path, "\\", "/"), "/")
		return parts[len(parts)-1]
	}
	return "未知曲目"
}

// DisplayArtist returns a fallback for missing artist metadata.
func (s Song) DisplayArtist() string {
	if strings.TrimSpace(s.Artist) != "" {
		return s.Artist
	}
	return "未知艺术家"
}

// DurationText formats the duration as m:ss or h:mm:ss.
func (s Song) DurationText() string { return FormatDuration(s.Duration) }

// Meta builds the compact "Artist · Album · Year" secondary line.
func (s Song) Meta() string {
	parts := make([]string, 0, 3)
	if a := s.DisplayArtist(); a != "" {
		parts = append(parts, a)
	}
	if strings.TrimSpace(s.Album) != "" {
		parts = append(parts, s.Album)
	}
	if s.Year > 0 {
		parts = append(parts, fmt.Sprintf("%d", s.Year))
	}
	return strings.Join(parts, " · ")
}

// FormatDuration formats a number of seconds into a human friendly string.
func FormatDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// Album is an album with its track list (filled by getAlbum).
type Album struct {
	ID        string
	Name      string
	Artist    string
	ArtistID  string
	CoverID   string
	Genre     string
	Created   string
	SongCount int
	Duration  int // seconds
	Year      int
	Starred   bool
	Songs     []Song
}

// DurationText returns the formatted album duration.
func (a Album) DurationText() string { return FormatDuration(a.Duration) }

// Artist is an ID3 artist. Albums is populated by getArtist.
type Artist struct {
	ID         string
	Name       string
	CoverID    string
	AlbumCount int
	Starred    bool
	Albums     []Album
}

// Playlist is a Navidrome playlist. Songs is populated by getPlaylist.
type Playlist struct {
	ID        string
	Name      string
	Comment   string
	Owner     string
	CoverID   string
	SongCount int
	Duration  int
	Public    bool
	Starred   bool
	Songs     []Song
}

// Genre is a music genre with library counters.
type Genre struct {
	Name       string
	SongCount  int
	AlbumCount int
}

// SearchResult groups the three result sets returned by search3.
type SearchResult struct {
	Songs   []Song
	Albums  []Album
	Artists []Artist
}

// IsEmpty reports whether the search produced no hits at all.
func (r SearchResult) IsEmpty() bool {
	return len(r.Songs) == 0 && len(r.Albums) == 0 && len(r.Artists) == 0
}

// Total returns the number of hits across all groups.
func (r SearchResult) Total() int { return len(r.Songs) + len(r.Albums) + len(r.Artists) }

// LyricsLine is one lyric line. Time is in milliseconds from the track start.
type LyricsLine struct {
	Time int
	Text string
}

// Lyrics is a parsed, time-indexed lyric sheet.
type Lyrics struct {
	Lines  []LyricsLine
	Synced bool
	Source string
}

// IsEmpty reports whether there is any lyric text to display.
func (l Lyrics) IsEmpty() bool { return len(l.Lines) == 0 }

// Normalize sorts lines by time and drops leading empty placeholder rows.
func (l Lyrics) Normalize() Lyrics {
	out := make([]LyricsLine, 0, len(l.Lines))
	for _, line := range l.Lines {
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		if line.Time < 0 {
			line.Time = 0
		}
		out = append(out, line)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	l.Lines = out
	return l
}

// Offset shifts every timestamp by delta milliseconds and returns a copy.
func (l Lyrics) Offset(delta int) Lyrics {
	lines := make([]LyricsLine, len(l.Lines))
	for i, line := range l.Lines {
		t := line.Time + delta
		if t < 0 {
			t = 0
		}
		lines[i] = LyricsLine{Time: t, Text: line.Text}
	}
	l.Lines = lines
	return l
}

// LineAt returns the index of the active line at the given position in
// milliseconds. It uses binary search so it is safe to call on every frame.
// It returns -1 when the position is before the first line.
func (l Lyrics) LineAt(positionMS int) int {
	if len(l.Lines) == 0 {
		return -1
	}
	idx := sort.Search(len(l.Lines), func(i int) bool { return l.Lines[i].Time > positionMS }) - 1
	if idx < 0 {
		return -1
	}
	if idx >= len(l.Lines) {
		idx = len(l.Lines) - 1
	}
	return idx
}
