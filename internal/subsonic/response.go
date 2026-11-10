package subsonic

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/lyrics"
	"github.com/anccnuer/go-navidrome-tui/internal/model"
)

// The wire format is the Subsonic "f=json" envelope. Navidrome sometimes
// encodes booleans/numbers as strings, hence the tolerant scalar types below.

type envelope struct {
	Response responseBody `json:"subsonic-response"`
}

type responseBody struct {
	Status        string  `json:"status"`
	Version       string  `json:"version"`
	Type          string  `json:"type"`
	ServerVersion string  `json:"serverVersion"`
	OpenSubsonic  bool    `json:"openSubsonic"`
	Error         *apiErr `json:"error"`

	Artists       *artistsDTO       `json:"artists"`
	Artist        *artistAlbumsDTO  `json:"artist"`
	Album         *albumSongsDTO    `json:"album"`
	AlbumList2    *albumList2DTO    `json:"albumList2"`
	SearchResult3 *searchResult3DTO `json:"searchResult3"`
	Playlists     *playlistsDTO     `json:"playlists"`
	Playlist      *playlistSongsDTO `json:"playlist"`
	Starred2      *starred2DTO      `json:"starred2"`
	Genres        *genresDTO        `json:"genres"`
	RandomSongs   *songsDTO         `json:"randomSongs"`
	SongsByGenre  *songsDTO         `json:"songsByGenre"`
	SimilarSongs2 *songsDTO         `json:"similarSongs2"`
	Lyrics        *lyricsDTO        `json:"lyrics"`
	LyricsList    *lyricsListDTO    `json:"lyricsList"`
	Song          *songDTO          `json:"song"`
}

type apiErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// APIError is a non-ok subsonic-response.
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("Navidrome 错误 %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("Navidrome 错误 %d", e.Code)
}

// IsUnsupported reports an unknown endpoint / unsupported extension, used to
// fall back from OpenSubsonic structured lyrics to the legacy endpoint.
func (e *APIError) IsUnsupported() bool { return e.Code == 0 || e.Code == 70 || e.Code == 10 }

type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(strings.ToLower(string(data)))
	s = strings.Trim(s, `"`)
	switch s {
	case "", "null", "false", "0", "no":
		*b = false
	default:
		// Subsonic uses an ISO timestamp string for "starred".
		*b = true
	}
	return nil
}

type flexInt int

func (i *flexInt) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(strings.Trim(string(data), `"`))
	if s == "" || s == "null" {
		*i = 0
		return nil
	}
	if v, err := strconv.Atoi(s); err == nil {
		*i = flexInt(v)
		return nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		*i = flexInt(int(f))
		return nil
	}
	*i = 0
	return nil
}

type flexInt64 int64

func (i *flexInt64) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(strings.Trim(string(data), `"`))
	if s == "" || s == "null" {
		*i = 0
		return nil
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		*i = flexInt64(v)
		return nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		*i = flexInt64(int64(f))
		return nil
	}
	*i = 0
	return nil
}

type songDTO struct {
	ID          string    `json:"id"`
	Parent      string    `json:"parent"`
	Title       string    `json:"title"`
	Album       string    `json:"album"`
	AlbumID     string    `json:"albumId"`
	Artist      string    `json:"artist"`
	ArtistID    string    `json:"artistId"`
	CoverArt    string    `json:"coverArt"`
	Genre       string    `json:"genre"`
	Suffix      string    `json:"suffix"`
	ContentType string    `json:"contentType"`
	Path        string    `json:"path"`
	Created     string    `json:"created"`
	Duration    flexInt   `json:"duration"`
	BitRate     flexInt   `json:"bitRate"`
	Track       flexInt   `json:"track"`
	DiscNumber  flexInt   `json:"discNumber"`
	Year        flexInt   `json:"year"`
	Size        flexInt64 `json:"size"`
	PlayCount   flexInt   `json:"playCount"`
	Rating      flexInt   `json:"userRating"`
	Starred     flexBool  `json:"starred"`
}

func (s songDTO) toModel() model.Song {
	return model.Song{
		ID:          s.ID,
		Title:       s.Title,
		Artist:      s.Artist,
		ArtistID:    s.ArtistID,
		Album:       s.Album,
		AlbumID:     s.AlbumID,
		CoverID:     s.CoverArt,
		Genre:       s.Genre,
		Suffix:      s.Suffix,
		ContentType: s.ContentType,
		Path:        s.Path,
		Created:     s.Created,
		Duration:    int(s.Duration),
		BitRate:     int(s.BitRate),
		Track:       int(s.Track),
		DiscNumber:  int(s.DiscNumber),
		Year:        int(s.Year),
		Size:        int64(s.Size),
		PlayCount:   int(s.PlayCount),
		Rating:      int(s.Rating),
		Starred:     bool(s.Starred),
	}
}

type albumDTO struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Artist    string   `json:"artist"`
	ArtistID  string   `json:"artistId"`
	CoverArt  string   `json:"coverArt"`
	Genre     string   `json:"genre"`
	Created   string   `json:"created"`
	SongCount flexInt  `json:"songCount"`
	Duration  flexInt  `json:"duration"`
	Year      flexInt  `json:"year"`
	Starred   flexBool `json:"starred"`
}

func (a albumDTO) toModel() model.Album {
	return model.Album{
		ID:        a.ID,
		Name:      a.Name,
		Artist:    a.Artist,
		ArtistID:  a.ArtistID,
		CoverID:   a.CoverArt,
		Genre:     a.Genre,
		Created:   a.Created,
		SongCount: int(a.SongCount),
		Duration:  int(a.Duration),
		Year:      int(a.Year),
		Starred:   bool(a.Starred),
	}
}

type artistDTO struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	CoverArt   string   `json:"coverArt"`
	AlbumCount flexInt  `json:"albumCount"`
	Starred    flexBool `json:"starred"`
}

func (a artistDTO) toModel() model.Artist {
	return model.Artist{
		ID:         a.ID,
		Name:       a.Name,
		CoverID:    a.CoverArt,
		AlbumCount: int(a.AlbumCount),
		Starred:    bool(a.Starred),
	}
}

type albumSongsDTO struct {
	albumDTO
	Songs []songDTO `json:"song"`
}

func (a albumSongsDTO) toModel() model.Album {
	out := a.albumDTO.toModel()
	out.Songs = songsToModel(a.Songs)
	if out.SongCount == 0 {
		out.SongCount = len(out.Songs)
	}
	return out
}

type artistAlbumsDTO struct {
	artistDTO
	Albums []albumDTO `json:"album"`
}

func (a artistAlbumsDTO) toModel() model.Artist {
	out := a.artistDTO.toModel()
	out.Albums = albumsToModel(a.Albums)
	if out.AlbumCount == 0 {
		out.AlbumCount = len(out.Albums)
	}
	return out
}

type artistsIndexDTO struct {
	Name    string      `json:"name"`
	Artists []artistDTO `json:"artist"`
}

type artistsDTO struct {
	Index []artistsIndexDTO `json:"index"`
	// Some servers omit the index wrapper.
	Artist []artistDTO `json:"artist"`
}

func (a artistsDTO) toModel() []model.Artist {
	var out []model.Artist
	for _, idx := range a.Index {
		for _, ar := range idx.Artists {
			out = append(out, ar.toModel())
		}
	}
	for _, ar := range a.Artist {
		out = append(out, ar.toModel())
	}
	return out
}

type albumList2DTO struct {
	Album []albumDTO `json:"album"`
}

type searchResult3DTO struct {
	Song   []songDTO   `json:"song"`
	Album  []albumDTO  `json:"album"`
	Artist []artistDTO `json:"artist"`
}

func (s searchResult3DTO) toModel() model.SearchResult {
	return model.SearchResult{
		Songs:   songsToModel(s.Song),
		Albums:  albumsToModel(s.Album),
		Artists: artistsToModel(s.Artist),
	}
}

type starred2DTO struct {
	Song   []songDTO   `json:"song"`
	Album  []albumDTO  `json:"album"`
	Artist []artistDTO `json:"artist"`
}

func (s starred2DTO) toModel() model.SearchResult {
	return model.SearchResult{
		Songs:   songsToModel(s.Song),
		Albums:  albumsToModel(s.Album),
		Artists: artistsToModel(s.Artist),
	}
}

type genreDTO struct {
	Value      string  `json:"value"`
	Name       string  `json:"name"`
	SongCount  flexInt `json:"songCount"`
	AlbumCount flexInt `json:"albumCount"`
}

type genresDTO struct {
	Genre []genreDTO `json:"genre"`
}

func (g genresDTO) toModel() []model.Genre {
	out := make([]model.Genre, 0, len(g.Genre))
	for _, v := range g.Genre {
		name := v.Value
		if name == "" {
			name = v.Name
		}
		if name == "" {
			continue
		}
		out = append(out, model.Genre{
			Name:       name,
			SongCount:  int(v.SongCount),
			AlbumCount: int(v.AlbumCount),
		})
	}
	return out
}

type songsDTO struct {
	Song []songDTO `json:"song"`
}

type playlistSummaryDTO struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Comment   string   `json:"comment"`
	Owner     string   `json:"owner"`
	CoverArt  string   `json:"coverArt"`
	SongCount flexInt  `json:"songCount"`
	Duration  flexInt  `json:"duration"`
	Public    flexBool `json:"public"`
	Created   string   `json:"created"`
}

func (p playlistSummaryDTO) toModel() model.Playlist {
	return model.Playlist{
		ID:        p.ID,
		Name:      p.Name,
		Comment:   p.Comment,
		Owner:     p.Owner,
		CoverID:   p.CoverArt,
		SongCount: int(p.SongCount),
		Duration:  int(p.Duration),
		Public:    bool(p.Public),
	}
}

type playlistsDTO struct {
	Playlist []playlistSummaryDTO `json:"playlist"`
}

type playlistSongsDTO struct {
	playlistSummaryDTO
	Entry []songDTO `json:"entry"`
}

func (p playlistSongsDTO) toModel() model.Playlist {
	out := p.playlistSummaryDTO.toModel()
	out.Songs = songsToModel(p.Entry)
	if out.SongCount == 0 {
		out.SongCount = len(out.Songs)
	}
	return out
}

type lyricsDTO struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
	Value  string `json:"value"`
}

type structuredLyricsDTO struct {
	DisplayArtist string `json:"displayArtist"`
	DisplayTitle  string `json:"displayTitle"`
	Lang          string `json:"lang"`
	Kind          string `json:"kind"`
	Offset        int    `json:"offset"`
	Synced        bool   `json:"synced"`
	Line          []struct {
		Start int    `json:"start"`
		Value string `json:"value"`
	} `json:"line"`
}

type lyricsListDTO struct {
	StructuredLyrics []structuredLyricsDTO `json:"structuredLyrics"`
}

// bestLyrics picks the richest available lyric track: synced main lyrics
// first, then any synced track, then unsynced text.
func (l lyricsListDTO) bestLyrics() model.Lyrics {
	var fallback model.Lyrics
	for _, item := range l.StructuredLyrics {
		if len(item.Line) == 0 {
			continue
		}
		lines := make([]model.LyricsLine, 0, len(item.Line))
		for _, line := range item.Line {
			lines = append(lines, model.LyricsLine{Time: line.Start, Text: line.Value})
		}
		out := lyrics.FromStructured(lines, item.Synced, "OpenSubsonic")
		if item.Offset != 0 && item.Synced {
			// OpenSubsonic: offset is added to the playback position.
			out = out.Offset(item.Offset)
		}
		if item.Synced {
			main := item.Kind == "" || item.Kind == "main"
			if main {
				return out
			}
			if fallback.IsEmpty() {
				fallback = out
			}
		} else if fallback.IsEmpty() {
			fallback = out
		}
	}
	return fallback
}

func songsToModel(in []songDTO) []model.Song {
	out := make([]model.Song, 0, len(in))
	for _, s := range in {
		out = append(out, s.toModel())
	}
	return out
}

func albumsToModel(in []albumDTO) []model.Album {
	out := make([]model.Album, 0, len(in))
	for _, a := range in {
		out = append(out, a.toModel())
	}
	return out
}

func artistsToModel(in []artistDTO) []model.Artist {
	out := make([]model.Artist, 0, len(in))
	for _, a := range in {
		out = append(out, a.toModel())
	}
	return out
}

// decodeEnvelope is shared by the client and its tests.
func decodeEnvelope(data []byte) (responseBody, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return responseBody{}, fmt.Errorf("解析响应 JSON: %w", err)
	}
	if env.Response.Status != "ok" {
		e := &APIError{Message: "服务器返回非 ok 状态"}
		if env.Response.Error != nil {
			e.Code = env.Response.Error.Code
			e.Message = env.Response.Error.Message
		}
		return responseBody{}, e
	}
	return env.Response, nil
}
