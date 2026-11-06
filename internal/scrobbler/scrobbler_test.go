package scrobbler

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/subsonic-tui/subsonic-tui/internal/config"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
)

func TestSignature(t *testing.T) {
	params := url.Values{"a": {"b"}, "c": {"d"}}
	want := md5.Sum([]byte("abcdsecret"))
	if got := apiSignature(params, "secret"); got != hex.EncodeToString(want[:]) {
		t.Fatalf("signature = %s, want %s", got, hex.EncodeToString(want[:]))
	}
	// api_sig itself must never be part of the signature input.
	params.Set("api_sig", "junk")
	if got := apiSignature(params, "secret"); got != hex.EncodeToString(want[:]) {
		t.Fatalf("api_sig leaked into the signature: %s", got)
	}
}

func TestShouldScrobble(t *testing.T) {
	song := model.Song{Duration: 200}
	if ShouldScrobble(song, 0) || ShouldScrobble(song, 99) {
		t.Fatal("should not scrobble before half the track")
	}
	if !ShouldScrobble(song, 100) {
		t.Fatal("should scrobble at half the track")
	}
	long := model.Song{Duration: 900}
	if !ShouldScrobble(long, 240) {
		t.Fatal("should scrobble at four minutes")
	}
	if ShouldScrobble(model.Song{Duration: 20}, 15) {
		t.Fatal("short tracks must not be scrobbled")
	}
}

func TestLastFMRequest(t *testing.T) {
	requests := make(chan url.Values, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		requests <- values
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	s := New(config.LastFMConfig{Enabled: true, APIKey: "key", APISecret: "secret", SessionKey: "sk"}, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.lastfmEP = srv.URL
	defer s.Close()

	song := model.Song{ID: "s1", Title: "Hysteria", Artist: "Muse", Album: "Absolution", Duration: 231}
	s.NowPlaying(song)
	s.Submitted(song, time.Unix(1700000000, 0))

	var captured url.Values
	for i := 0; i < 2; i++ {
		select {
		case captured = <-requests:
		case <-time.After(3 * time.Second):
			t.Fatal("worker did not send both requests")
		}
	}

	if captured.Get("api_key") != "key" || captured.Get("sk") != "sk" {
		t.Fatalf("credentials missing: %v", captured)
	}
	if captured.Get("method") != "track.scrobble" {
		t.Fatalf("method = %q", captured.Get("method"))
	}
	if captured.Get("artist[0]") != "Muse" || captured.Get("track[0]") != "Hysteria" || captured.Get("timestamp[0]") != "1700000000" {
		t.Fatalf("scrobble params = %v", captured)
	}
	if captured.Get("api_sig") == "" {
		t.Fatal("missing api_sig")
	}
	if got := apiSignature(captured, "secret"); got != captured.Get("api_sig") {
		t.Fatalf("signature mismatch: %s vs %s", got, captured.Get("api_sig"))
	}
}

func TestRetryableClassification(t *testing.T) {
	for code, want := range map[int]bool{9: false, 11: true, 16: true, 6: false} {
		err := &lastfmError{code: code, message: "x", retryable: code == 11 || code == 16}
		if got := isRetryable(err); got != want {
			t.Fatalf("code %d retryable = %v", code, got)
		}
	}
	if !isRetryable(io.EOF) {
		t.Fatal("transport errors should be retryable")
	}
}

func TestShouldScrobbleAtThreshold(t *testing.T) {
	song := model.Song{Duration: 200}
	if !ShouldScrobbleAt(song, 40, 20) {
		t.Fatal("20% of a 200s track should be reached at 40s")
	}
	if ShouldScrobbleAt(song, 39, 20) {
		t.Fatal("should not report before the configured threshold")
	}
	if ShouldScrobbleAt(song, 199, 0) {
		t.Fatal("threshold 0 must only report on end-of-file")
	}
	if !ShouldScrobbleAt(model.Song{Duration: 900}, 240, 80) {
		t.Fatal("the four-minute rule must still apply")
	}
	service := New(config.LastFMConfig{ScrobbleAt: 0}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer service.Close()
	if service.ShouldScrobble(song, 199) {
		t.Fatal("Service should honour ScrobbleAt=0")
	}
}
