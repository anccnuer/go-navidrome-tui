package subsonic

import (
	"net/url"
	"strconv"
	"strings"
)

// StreamURL builds an authenticated direct-play URL for mpv.
func (c *client) StreamURL(songID string) string {
	p := url.Values{}
	p.Set("id", songID)
	return c.endpointURL("stream", p)
}

// CoverURL builds an authenticated cover art URL. The UI prefers GetCover
// (bytes) so it can downsample, but this is useful for browser actions.
func (c *client) CoverURL(coverID string, size int) string {
	p := url.Values{}
	p.Set("id", coverID)
	if size > 0 {
		p.Set("size", strconv.Itoa(size))
	}
	return c.endpointURL("getCoverArt", p)
}

// WebURL returns a Navidrome web UI link for the "open in browser" action.
// Navidrome's SPA routes are best-effort: unknown kinds fall back to the
// server root, which is always valid.
func (c *client) WebURL(kind, id string) string {
	escaped := url.PathEscape(id)
	switch strings.ToLower(kind) {
	case "album":
		return c.base + "/app/#/album/" + escaped
	case "artist":
		return c.base + "/app/#/artist/" + escaped
	case "playlist":
		return c.base + "/app/#/playlist/" + escaped
	default:
		return c.base
	}
}
