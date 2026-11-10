package scrobbler

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anccnuer/go-navidrome-tui/internal/model"
)

const lastfmEndpoint = "https://ws.audioscrobbler.com/2.0/"

type lastfmError struct {
	code      int
	message   string
	retryable bool
}

func (e *lastfmError) Error() string {
	return fmt.Sprintf("Last.fm 错误 %d: %s", e.code, e.message)
}

func isRetryable(err error) bool {
	var lfm *lastfmError
	if errors.As(err, &lfm) {
		return lfm.retryable
	}
	return true // network/transport failures are worth retrying
}

// apiSignature is the md5 of the alphabetically sorted name+value pairs
// followed by the API secret (Last.fm auth spec).
func apiSignature(params url.Values, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "api_sig" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		values := params[k]
		for _, v := range values {
			b.WriteString(k)
			b.WriteString(v)
		}
	}
	b.WriteString(secret)
	sum := md5.Sum([]byte(b.String())) //nolint:gosec // required by the Last.fm API
	return hex.EncodeToString(sum[:])
}

func (s *Service) post(ctx context.Context, params url.Values) error {
	params.Set("api_key", s.cfg.APIKey)
	params.Set("sk", s.cfg.SessionKey)
	params.Set("format", "json")
	params.Set("api_sig", apiSignature(params, s.cfg.APISecret))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.lastfmEP, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("Last.fm 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		Status  string `json:"status"`
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("解析 Last.fm 响应失败 (HTTP %d): %w", resp.StatusCode, err)
	}
	if out.Error != 0 {
		return &lastfmError{
			code:      out.Error,
			message:   out.Message,
			retryable: out.Error == 11 || out.Error == 16,
		}
	}
	if out.Status != "" && out.Status != "ok" {
		return &lastfmError{code: resp.StatusCode, message: out.Status}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Last.fm 返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *Service) updateNowPlaying(ctx context.Context, song model.Song) error {
	params := url.Values{}
	params.Set("method", "track.updateNowPlaying")
	params.Set("artist", song.DisplayArtist())
	params.Set("track", song.DisplayTitle())
	if song.Album != "" {
		params.Set("album", song.Album)
	}
	if song.Duration > 0 {
		params.Set("duration", strconv.Itoa(song.Duration))
	}
	return s.post(ctx, params)
}

func (s *Service) scrobble(ctx context.Context, song model.Song, started time.Time) error {
	params := url.Values{}
	params.Set("method", "track.scrobble")
	params.Set("artist[0]", song.DisplayArtist())
	params.Set("track[0]", song.DisplayTitle())
	params.Set("timestamp[0]", strconv.FormatInt(started.Unix(), 10))
	if song.Album != "" {
		params.Set("album[0]", song.Album)
	}
	if song.Duration > 0 {
		params.Set("duration[0]", strconv.Itoa(song.Duration))
	}
	return s.post(ctx, params)
}
