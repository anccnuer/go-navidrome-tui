package subsonic

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc) (Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := New(Options{
		BaseURL:    srv.URL,
		Username:   "alice",
		Password:   "secret",
		CacheTTL:   time.Minute,
		CacheSize:  64,
		HTTPClient: srv.Client(),
	})
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return c, srv
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func TestAuthParamsAndPing(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/ping.view" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("u") != "alice" || q.Get("v") != APIVersion || q.Get("c") != ClientName || q.Get("f") != "json" {
			t.Errorf("bad base params: %v", q)
		}
		sum := md5.Sum([]byte("secret" + q.Get("s")))
		if q.Get("t") != hex.EncodeToString(sum[:]) {
			t.Errorf("token does not match md5(password+salt): %v", q)
		}
		if q.Get("p") != "" {
			t.Errorf("password must never be sent in clear text")
		}
		writeJSON(w, `{"subsonic-response":{"status":"ok","version":"1.16.1","type":"navidrome","serverVersion":"0.55.0","openSubsonic":true}}`)
	})
	if err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	info := c.ServerInfo()
	if info.Type != "navidrome" || !info.OpenSubsonic || info.ServerVersion != "0.55.0" {
		t.Fatalf("info = %+v", info)
	}
}

func TestGetArtistsParsesIndexAndTolerantTypes(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/getArtists.view" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, `{"subsonic-response":{"status":"ok","artists":{"index":[
			{"name":"A","artist":[{"id":"a1","name":"Alpha","albumCount":"3","coverArt":"ar-1","starred":"2024-01-01T00:00:00Z"}]},
			{"name":"B","artist":[{"id":"a2","name":"Beta","albumCount":2}]}
		]}}}`)
	})
	artists, err := c.GetArtists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 2 || artists[0].AlbumCount != 3 || !artists[0].Starred || artists[1].Starred {
		t.Fatalf("artists = %+v", artists)
	}
}

func TestSearchParamsAndCache(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if r.URL.Path == "/rest/search3.view" {
			if q.Get("query") != "muse*" || q.Get("songCount") != "20" || q.Get("songOffset") != "5" {
				t.Errorf("search params = %v", q)
			}
			writeJSON(w, `{"subsonic-response":{"status":"ok","searchResult3":{"song":[{"id":"s1","title":"Hysteria","artist":"Muse","duration":"220"}],"album":[],"artist":[]}}}`)
			return
		}
		writeJSON(w, `{"subsonic-response":{"status":"ok"}}`)
	})
	for i := 0; i < 2; i++ {
		res, err := c.Search(context.Background(), "muse", Page{Offset: 5, Size: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Songs) != 1 || res.Songs[0].Duration != 220 {
			t.Fatalf("result = %+v", res)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected cache hit, got %d requests", calls.Load())
	}
	c.InvalidateCache("search:")
	if _, err := c.Search(context.Background(), "muse", Page{Offset: 5, Size: 20}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("invalidation did not force a refetch: %d", calls.Load())
	}
}

func TestStructuredLyricsAndOffset(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/getLyricsBySongId.view" {
			t.Errorf("unexpected fallback to %s", r.URL.Path)
		}
		writeJSON(w, `{"subsonic-response":{"status":"ok","lyricsList":{"structuredLyrics":[
			{"lang":"eng","synced":true,"offset":-100,"line":[{"start":1000,"value":"one"},{"start":2500,"value":"two"}]},
			{"lang":"und","synced":false,"line":[{"value":"plain"}]}
		]}}}`)
	})
	l, err := c.GetLyrics(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if !l.Synced || len(l.Lines) != 2 || l.Lines[0].Time != 900 || l.Lines[1].Text != "two" {
		t.Fatalf("lyrics = %+v", l)
	}
	if l.LineAt(1000) != 0 || l.LineAt(2500) != 1 {
		t.Fatalf("LineAt failed: %d %d", l.LineAt(1000), l.LineAt(2500))
	}
}

func TestLyricsFallsBackToLegacy(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/getLyricsBySongId.view":
			http.Error(w, "not found", http.StatusNotFound)
		case "/rest/getSong.view":
			writeJSON(w, `{"subsonic-response":{"status":"ok","song":{"id":"s1","title":"Song","artist":"Artist"}}}`)
		case "/rest/getLyrics.view":
			writeJSON(w, `{"subsonic-response":{"status":"ok","lyrics":{"artist":"Artist","title":"Song","value":"[00:02.50]hello\n[00:05.00]world"}}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	l, err := c.GetLyrics(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if !l.Synced || len(l.Lines) != 2 || l.Lines[0].Time != 2500 {
		t.Fatalf("lyrics = %+v", l)
	}
}

func TestAPIErrorMapping(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"subsonic-response":{"status":"failed","version":"1.16.1","error":{"code":40,"message":"Wrong username or password"}}}`)
	})
	err := c.Ping(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T %v", err, err)
	}
	if apiErr.Code != 40 {
		t.Fatalf("code = %d", apiErr.Code)
	}
}

func TestStarUsesCorrectParameterAndInvalidates(t *testing.T) {
	var starredCalls atomic.Int32
	var starParam atomic.Value
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/getStarred2.view":
			starredCalls.Add(1)
			writeJSON(w, `{"subsonic-response":{"status":"ok","starred2":{"song":[]}}}`)
		case "/rest/star.view":
			starParam.Store(r.URL.Query().Get("albumId"))
			writeJSON(w, `{"subsonic-response":{"status":"ok"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	if _, err := c.GetStarred(context.Background()); err != nil {
		t.Fatal(err)
	}
	if starredCalls.Load() != 1 {
		t.Fatal("starred not requested")
	}
	if err := c.Star(context.Background(), StarAlbum, "al-1", true); err != nil {
		t.Fatal(err)
	}
	if v, _ := starParam.Load().(string); v != "al-1" {
		t.Fatalf("albumId = %q", v)
	}
	if _, err := c.GetStarred(context.Background()); err != nil {
		t.Fatal(err)
	}
	if starredCalls.Load() != 2 {
		t.Fatalf("starred cache was not invalidated: %d calls", starredCalls.Load())
	}
}

func TestScrobbleSubmissionFlag(t *testing.T) {
	var submission, reportedTime string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		submission = r.URL.Query().Get("submission")
		reportedTime = r.URL.Query().Get("time")
		writeJSON(w, `{"subsonic-response":{"status":"ok"}}`)
	})
	if err := c.Scrobble(context.Background(), "s1", true, time.Unix(1700000000, 0)); err != nil {
		t.Fatal(err)
	}
	if submission != "true" {
		t.Fatalf("submission = %q", submission)
	}
	if reportedTime != "1700000000000" {
		t.Fatalf("time = %q, want the track start timestamp", reportedTime)
	}
}
