// Package subsonic is a small, cached (Open)Subsonic REST client tailored to
// Navidrome. It only uses the standard library so that auth, paging, errors
// and response shapes stay fully under our control.
package subsonic

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anccnuer/go-navidrome-tui/internal/cache"
	"github.com/anccnuer/go-navidrome-tui/internal/model"
)

const (
	// APIVersion is the Subsonic version Navidrome is compatible with.
	APIVersion = "1.16.1"
	// ClientName is sent as the "c" parameter and shown in Navidrome's clients.
	ClientName = "navidrome-tui"
	maxBody    = 16 << 20 // 16 MiB guard for JSON responses
)

// Page is an offset based page descriptor.
type Page struct {
	Offset int
	Size   int
}

// AlbumQuery describes a getAlbumList2 request.
type AlbumQuery struct {
	Type     string // newest | recent | random | byYear | byGenre | alphabeticalByName | starred | frequent
	Size     int
	Offset   int
	Genre    string
	FromYear int
	ToYear   int
}

// ServerInfo is populated after a successful Ping.
type ServerInfo struct {
	Type          string
	Version       string
	ServerVersion string
	OpenSubsonic  bool
}

// Client is the only dependency the UI layer has on the data source.
type Client interface {
	Ping(ctx context.Context) error
	ServerInfo() ServerInfo

	GetArtists(ctx context.Context) ([]model.Artist, error)
	GetArtist(ctx context.Context, id string) (model.Artist, error)
	GetAlbum(ctx context.Context, id string) (model.Album, error)
	GetAlbumList(ctx context.Context, q AlbumQuery) ([]model.Album, error)
	GetPlaylists(ctx context.Context) ([]model.Playlist, error)
	GetPlaylist(ctx context.Context, id string) (model.Playlist, error)
	GetStarred(ctx context.Context) (model.SearchResult, error)
	GetGenres(ctx context.Context) ([]model.Genre, error)
	GetGenreSongs(ctx context.Context, genre string, p Page) ([]model.Song, error)
	GetRandomSongs(ctx context.Context, size int) ([]model.Song, error)
	GetSimilarSongs(ctx context.Context, id string, count int) ([]model.Song, error)
	GetSong(ctx context.Context, id string) (model.Song, error)
	Search(ctx context.Context, query string, p Page) (model.SearchResult, error)
	GetSongs(ctx context.Context, p Page) ([]model.Song, error)

	GetLyrics(ctx context.Context, songID string) (model.Lyrics, error)
	GetCover(ctx context.Context, coverID string, size int) ([]byte, error)

	Star(ctx context.Context, kind StarKind, id string, on bool) error
	SetRating(ctx context.Context, id string, rating int) error
	Scrobble(ctx context.Context, songID string, submission bool, started time.Time) error
	CreatePlaylist(ctx context.Context, name string) (model.Playlist, error)
	UpdatePlaylist(ctx context.Context, playlistID, name string, add []string, removeIndexes []int) error

	StreamURL(songID string) string
	CoverURL(coverID string, size int) string
	WebURL(kind, id string) string
	InvalidateCache(prefix string)
}

// Options configures a client instance.
type Options struct {
	BaseURL       string
	Username      string
	Password      string // plain password; token auth is derived per request
	Insecure      bool
	CacheTTL      time.Duration
	CacheSize     int
	MaxConcurrent int
	Logger        *slog.Logger
	HTTPClient    *http.Client
}

type client struct {
	base     string
	username string
	password string
	http     *http.Client
	sem      chan struct{}
	cache    *cache.Cache[any]
	log      *slog.Logger
	info     ServerInfo
}

// New validates the options and builds a client. No network call is made.
func New(opts Options) (Client, error) {
	base := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("无效的服务器地址: %q", opts.BaseURL)
	}
	if opts.Username == "" {
		return nil, fmt.Errorf("用户名不能为空")
	}
	hc := opts.HTTPClient
	if hc == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns = 32
		transport.MaxIdleConnsPerHost = 8
		transport.IdleConnTimeout = 90 * time.Second
		if opts.Insecure {
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit opt-in
		}
		hc = &http.Client{Transport: transport, Timeout: 20 * time.Second}
	}
	if opts.CacheSize <= 0 {
		opts.CacheSize = 512
	}
	if opts.MaxConcurrent <= 0 {
		opts.MaxConcurrent = 8
	}
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &client{
		base:     base,
		username: opts.Username,
		password: opts.Password,
		http:     hc,
		sem:      make(chan struct{}, opts.MaxConcurrent),
		cache:    cache.New[any](opts.CacheSize, opts.CacheTTL),
		log:      log,
	}, nil
}

// ServerInfo returns the values observed on the last successful Ping.
func (c *client) ServerInfo() ServerInfo { return c.info }

func (c *client) InvalidateCache(prefix string) {
	if prefix == "" {
		c.cache.Clear()
		return
	}
	c.cache.InvalidatePrefix(prefix)
}

// authParams builds the query with a fresh random salt for every request.
// The password is never sent in clear text.
func (c *client) authParams() url.Values {
	v := url.Values{}
	v.Set("u", c.username)
	salt := randomHex(8)
	sum := md5.Sum([]byte(c.password + salt)) //nolint:gosec // required by Subsonic token auth
	v.Set("t", hex.EncodeToString(sum[:]))
	v.Set("s", salt)
	v.Set("v", APIVersion)
	v.Set("c", ClientName)
	v.Set("f", "json")
	return v
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (c *client) endpointURL(endpoint string, params url.Values) string {
	q := c.authParams()
	for k, values := range params {
		for _, value := range values {
			q.Set(k, value)
		}
	}
	return c.base + "/rest/" + endpoint + ".view?" + q.Encode()
}

// do performs a JSON API request. It never logs credentials.
func (c *client) do(ctx context.Context, endpoint string, params url.Values) (responseBody, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return responseBody{}, ctx.Err()
	}

	target := c.endpointURL(endpoint, params)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return responseBody{}, err
	}
	req.Header.Set("Accept", "application/json")
	c.log.Debug("API 请求", "url", safeURL(target))
	resp, err := c.http.Do(req)
	if err != nil {
		c.log.Warn("请求失败", "endpoint", endpoint, "err", err)
		return responseBody{}, fmt.Errorf("连接 Navidrome 失败（%s）: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return responseBody{}, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		c.log.Warn("HTTP 错误", "endpoint", endpoint, "status", resp.StatusCode)
		return responseBody{}, fmt.Errorf("Navidrome 返回 HTTP %d（%s），请检查地址与账号", resp.StatusCode, endpoint)
	}
	result, err := decodeEnvelope(body)
	if err != nil {
		c.log.Warn("响应错误", "endpoint", endpoint, "err", err)
		return responseBody{}, err
	}
	return result, nil
}

func (c *client) doRaw(ctx context.Context, endpoint string, params url.Values) ([]byte, string, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	target := c.endpointURL(endpoint, params)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("连接 Navidrome 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Navidrome 返回 HTTP %d（%s）", resp.StatusCode, endpoint)
	}
	ct := resp.Header.Get("Content-Type")
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, "", err
	}
	if strings.Contains(ct, "application/json") {
		// getCoverArt may answer with a JSON error envelope instead of an image.
		if _, err := decodeEnvelope(data); err != nil {
			return nil, "", err
		}
		return nil, "", fmt.Errorf("封面请求返回了非图片响应")
	}
	if err != nil {
		return nil, "", err
	}
	return data, ct, nil
}

// cached loads a typed value from the shared cache. Keys are namespaced by
// logical resource so mutations can invalidate a whole group cheaply.
func cached[T any](c *client, key string, fetch func() (T, error)) (T, error) {
	if v, ok := c.cache.Get(key); ok {
		if typed, ok := v.(T); ok {
			return typed, nil
		}
	}
	v, err := fetch()
	if err != nil {
		return v, err
	}
	c.cache.Set(key, v)
	return v, nil
}

// safeURL strips credentials from a URL before it is ever logged.
func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid-url>"
	}
	q := u.Query()
	for _, k := range []string{"u", "t", "s", "p", "password", "api_key", "sk", "api_sig"} {
		if q.Has(k) {
			q.Set(k, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
