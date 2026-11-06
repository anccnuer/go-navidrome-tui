package subsonic

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/subsonic-tui/subsonic-tui/internal/lyrics"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
)

// StarKind selects the correct parameter for star/unstar: Subsonic uses a
// different parameter name for songs, albums and artists.
type StarKind string

const (
	StarSong   StarKind = "id"
	StarAlbum  StarKind = "albumId"
	StarArtist StarKind = "artistId"
)

// Ping checks connectivity and records the server capabilities.
func (c *client) Ping(ctx context.Context) error {
	body, err := c.do(ctx, "ping", url.Values{})
	if err != nil {
		return err
	}
	c.info = ServerInfo{
		Type:          body.Type,
		Version:       body.Version,
		ServerVersion: body.ServerVersion,
		OpenSubsonic:  body.OpenSubsonic,
	}
	return nil
}

func (c *client) GetArtists(ctx context.Context) ([]model.Artist, error) {
	return cached(c, "artists", func() ([]model.Artist, error) {
		body, err := c.do(ctx, "getArtists", url.Values{})
		if err != nil {
			return nil, err
		}
		if body.Artists == nil {
			return []model.Artist{}, nil
		}
		return body.Artists.toModel(), nil
	})
}

func (c *client) GetArtist(ctx context.Context, id string) (model.Artist, error) {
	return cached(c, "artist:"+id, func() (model.Artist, error) {
		p := url.Values{}
		p.Set("id", id)
		body, err := c.do(ctx, "getArtist", p)
		if err != nil {
			return model.Artist{}, err
		}
		if body.Artist == nil {
			return model.Artist{}, fmt.Errorf("找不到艺术家 %s", id)
		}
		return body.Artist.toModel(), nil
	})
}

func (c *client) GetAlbum(ctx context.Context, id string) (model.Album, error) {
	return cached(c, "album:"+id, func() (model.Album, error) {
		p := url.Values{}
		p.Set("id", id)
		body, err := c.do(ctx, "getAlbum", p)
		if err != nil {
			return model.Album{}, err
		}
		if body.Album == nil {
			return model.Album{}, fmt.Errorf("找不到专辑 %s", id)
		}
		return body.Album.toModel(), nil
	})
}

func (c *client) GetAlbumList(ctx context.Context, q AlbumQuery) ([]model.Album, error) {
	if q.Type == "" {
		q.Type = "newest"
	}
	if q.Size <= 0 {
		q.Size = 60
	}
	key := fmt.Sprintf("albumList:%s:%d:%d:%s:%d:%d", q.Type, q.Size, q.Offset, q.Genre, q.FromYear, q.ToYear)
	return cached(c, key, func() ([]model.Album, error) {
		p := url.Values{}
		p.Set("type", q.Type)
		p.Set("size", strconv.Itoa(q.Size))
		p.Set("offset", strconv.Itoa(q.Offset))
		if q.Genre != "" {
			p.Set("genre", q.Genre)
		}
		if q.FromYear > 0 {
			p.Set("fromYear", strconv.Itoa(q.FromYear))
			p.Set("toYear", strconv.Itoa(q.ToYear))
		}
		body, err := c.do(ctx, "getAlbumList2", p)
		if err != nil {
			return nil, err
		}
		if body.AlbumList2 == nil {
			return []model.Album{}, nil
		}
		return albumsToModel(body.AlbumList2.Album), nil
	})
}

func (c *client) GetPlaylists(ctx context.Context) ([]model.Playlist, error) {
	return cached(c, "playlists", func() ([]model.Playlist, error) {
		body, err := c.do(ctx, "getPlaylists", url.Values{})
		if err != nil {
			return nil, err
		}
		if body.Playlists == nil {
			return []model.Playlist{}, nil
		}
		out := make([]model.Playlist, 0, len(body.Playlists.Playlist))
		for _, p := range body.Playlists.Playlist {
			out = append(out, p.toModel())
		}
		return out, nil
	})
}

func (c *client) GetPlaylist(ctx context.Context, id string) (model.Playlist, error) {
	return cached(c, "playlist:"+id, func() (model.Playlist, error) {
		p := url.Values{}
		p.Set("id", id)
		body, err := c.do(ctx, "getPlaylist", p)
		if err != nil {
			return model.Playlist{}, err
		}
		if body.Playlist == nil {
			return model.Playlist{}, fmt.Errorf("找不到歌单 %s", id)
		}
		return body.Playlist.toModel(), nil
	})
}

func (c *client) GetStarred(ctx context.Context) (model.SearchResult, error) {
	return cached(c, "starred", func() (model.SearchResult, error) {
		body, err := c.do(ctx, "getStarred2", url.Values{})
		if err != nil {
			return model.SearchResult{}, err
		}
		if body.Starred2 == nil {
			return model.SearchResult{}, nil
		}
		return body.Starred2.toModel(), nil
	})
}

func (c *client) GetGenres(ctx context.Context) ([]model.Genre, error) {
	return cached(c, "genres", func() ([]model.Genre, error) {
		body, err := c.do(ctx, "getGenres", url.Values{})
		if err != nil {
			return nil, err
		}
		if body.Genres == nil {
			return []model.Genre{}, nil
		}
		return body.Genres.toModel(), nil
	})
}

func (c *client) GetGenreSongs(ctx context.Context, genre string, p Page) ([]model.Song, error) {
	if p.Size <= 0 {
		p.Size = 60
	}
	key := fmt.Sprintf("genreSongs:%s:%d:%d", genre, p.Offset, p.Size)
	return cached(c, key, func() ([]model.Song, error) {
		params := url.Values{}
		params.Set("genre", genre)
		params.Set("count", strconv.Itoa(p.Size))
		params.Set("offset", strconv.Itoa(p.Offset))
		body, err := c.do(ctx, "getSongsByGenre", params)
		if err != nil {
			return nil, err
		}
		if body.SongsByGenre == nil {
			return []model.Song{}, nil
		}
		return songsToModel(body.SongsByGenre.Song), nil
	})
}

// GetRandomSongs is intentionally never cached: each call should yield a
// genuinely new random selection.
func (c *client) GetRandomSongs(ctx context.Context, size int) ([]model.Song, error) {
	if size <= 0 {
		size = 50
	}
	p := url.Values{}
	p.Set("size", strconv.Itoa(size))
	body, err := c.do(ctx, "getRandomSongs", p)
	if err != nil {
		return nil, err
	}
	if body.RandomSongs == nil {
		return []model.Song{}, nil
	}
	return songsToModel(body.RandomSongs.Song), nil
}

func (c *client) GetSimilarSongs(ctx context.Context, id string, count int) ([]model.Song, error) {
	if count <= 0 {
		count = 30
	}
	return cached(c, fmt.Sprintf("similar:%s:%d", id, count), func() ([]model.Song, error) {
		p := url.Values{}
		p.Set("id", id)
		p.Set("count", strconv.Itoa(count))
		body, err := c.do(ctx, "getSimilarSongs2", p)
		if err != nil {
			return nil, err
		}
		if body.SimilarSongs2 == nil {
			return []model.Song{}, nil
		}
		return songsToModel(body.SimilarSongs2.Song), nil
	})
}

func (c *client) GetSong(ctx context.Context, id string) (model.Song, error) {
	return cached(c, "song:"+id, func() (model.Song, error) {
		p := url.Values{}
		p.Set("id", id)
		body, err := c.do(ctx, "getSong", p)
		if err != nil {
			return model.Song{}, err
		}
		if body.Song == nil {
			return model.Song{}, fmt.Errorf("找不到曲目 %s", id)
		}
		return body.Song.toModel(), nil
	})
}

// GetSongs returns the whole library in stable alphabetical order. Navidrome
// accepts an empty search3 query and treats it as "match everything", which
// avoids walking every album individually.
func (c *client) GetSongs(ctx context.Context, p Page) ([]model.Song, error) {
	if p.Size <= 0 {
		p.Size = 100
	}
	key := fmt.Sprintf("songs:all:%d:%d", p.Offset, p.Size)
	return cached(c, key, func() ([]model.Song, error) {
		params := url.Values{}
		params.Set("query", "")
		params.Set("songCount", strconv.Itoa(p.Size))
		params.Set("songOffset", strconv.Itoa(p.Offset))
		params.Set("albumCount", "0")
		params.Set("artistCount", "0")
		body, err := c.do(ctx, "search3", params)
		if err != nil {
			return nil, err
		}
		if body.SearchResult3 == nil {
			return []model.Song{}, nil
		}
		return body.SearchResult3.toModel().Songs, nil
	})
}

func (c *client) Search(ctx context.Context, query string, p Page) (model.SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return model.SearchResult{}, nil
	}
	if p.Size <= 0 {
		p.Size = 30
	}
	key := fmt.Sprintf("search:%s:%d:%d", query, p.Offset, p.Size)
	return cached(c, key, func() (model.SearchResult, error) {
		params := url.Values{}
		term := query
		if !strings.HasSuffix(term, "*") {
			term += "*" // Navidrome matches prefixes; avoid doubling "**"
		}
		params.Set("query", term)
		params.Set("songCount", strconv.Itoa(p.Size))
		params.Set("songOffset", strconv.Itoa(p.Offset))
		// This build only browses songs; skip album/artist results.
		params.Set("albumCount", "0")
		params.Set("artistCount", "0")
		body, err := c.do(ctx, "search3", params)
		if err != nil {
			return model.SearchResult{}, err
		}
		if body.SearchResult3 == nil {
			return model.SearchResult{}, nil
		}
		return body.SearchResult3.toModel(), nil
	})
}

// GetLyrics prefers OpenSubsonic structured lyrics and falls back to the
// legacy getLyrics endpoint (which requires artist + title).
func (c *client) GetLyrics(ctx context.Context, songID string) (model.Lyrics, error) {
	return cached(c, "lyrics:"+songID, func() (model.Lyrics, error) {
		params := url.Values{}
		params.Set("id", songID)
		body, err := c.do(ctx, "getLyricsBySongId", params)
		if err != nil {
			c.log.Debug("结构化歌词不可用，回退 getLyrics", "song", songID, "err", err)
		} else if body.LyricsList != nil {
			if out := body.LyricsList.bestLyrics(); !out.IsEmpty() {
				return out, nil
			}
		}

		song, err := c.GetSong(ctx, songID)
		if err != nil {
			// Missing metadata is a normal "no lyrics" condition, not an error
			// that should break the lyrics view.
			c.log.Debug("getSong 失败，返回空歌词", "song", songID, "err", err)
			return model.Lyrics{}, nil
		}
		legacy := url.Values{}
		legacy.Set("artist", song.Artist)
		legacy.Set("title", song.Title)
		legacyBody, err := c.do(ctx, "getLyrics", legacy)
		if err != nil {
			c.log.Debug("旧版歌词端点不可用", "song", songID, "err", err)
			return model.Lyrics{}, nil // no lyrics is a normal condition
		}
		if legacyBody.Lyrics == nil || strings.TrimSpace(legacyBody.Lyrics.Value) == "" {
			return model.Lyrics{}, nil
		}
		return lyrics.FromPlain(legacyBody.Lyrics.Value), nil
	})
}

func (c *client) GetCover(ctx context.Context, coverID string, size int) ([]byte, error) {
	if coverID == "" {
		return nil, fmt.Errorf("封面 ID 为空")
	}
	if size <= 0 {
		size = 300
	}
	key := fmt.Sprintf("cover:%s:%d", coverID, size)
	return cached(c, key, func() ([]byte, error) {
		p := url.Values{}
		p.Set("id", coverID)
		p.Set("size", strconv.Itoa(size))
		data, _, err := c.doRaw(ctx, "getCoverArt", p)
		if err != nil {
			return nil, err
		}
		return data, nil
	})
}

func (c *client) Star(ctx context.Context, kind StarKind, id string, on bool) error {
	endpoint := "star"
	if !on {
		endpoint = "unstar"
	}
	p := url.Values{}
	p.Set(string(kind), id)
	_, err := c.do(ctx, endpoint, p)
	if err != nil {
		return err
	}
	c.InvalidateCache("starred")
	c.InvalidateCache("albumList:")
	return nil
}

func (c *client) SetRating(ctx context.Context, id string, rating int) error {
	if rating < 0 {
		rating = 0
	}
	if rating > 5 {
		rating = 5
	}
	p := url.Values{}
	p.Set("id", id)
	p.Set("rating", strconv.Itoa(rating))
	_, err := c.do(ctx, "setRating", p)
	if err != nil {
		return err
	}
	c.InvalidateCache("starred")
	return nil
}

// Scrobble reports playback. submission=false is "now playing",
// submission=true increments the play count in Navidrome.
func (c *client) Scrobble(ctx context.Context, songID string, submission bool, started time.Time) error {
	p := url.Values{}
	p.Set("id", songID)
	p.Set("submission", strconv.FormatBool(submission))
	if started.IsZero() {
		started = time.Now()
	}
	p.Set("time", strconv.FormatInt(started.UnixMilli(), 10))
	_, err := c.do(ctx, "scrobble", p)
	return err
}

func (c *client) CreatePlaylist(ctx context.Context, name string) (model.Playlist, error) {
	p := url.Values{}
	p.Set("name", name)
	body, err := c.do(ctx, "createPlaylist", p)
	if err != nil {
		return model.Playlist{}, err
	}
	c.InvalidateCache("playlists")
	if body.Playlist == nil {
		return model.Playlist{Name: name}, nil
	}
	return body.Playlist.toModel(), nil
}

func (c *client) UpdatePlaylist(ctx context.Context, playlistID, name string, add []string, removeIndexes []int) error {
	p := url.Values{}
	p.Set("playlistId", playlistID)
	if name != "" {
		p.Set("name", name)
	}
	for _, id := range add {
		p.Add("songIdToAdd", id)
	}
	for _, idx := range removeIndexes {
		p.Add("songIndexToRemove", strconv.Itoa(idx))
	}
	_, err := c.do(ctx, "updatePlaylist", p)
	if err != nil {
		return err
	}
	c.InvalidateCache("playlists")
	c.InvalidateCache("playlist:" + playlistID)
	return nil
}
