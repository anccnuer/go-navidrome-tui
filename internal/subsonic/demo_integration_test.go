package subsonic

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anccnuer/go-navidrome-tui/internal/model"
)

// TestPublicDemoReadOnly exercises the real public Navidrome demo instance.
// It is opt-in so the normal unit-test suite stays offline:
//
//	NAVIDROME_TUI_DEMO_TEST=1 go test ./internal/subsonic -run TestPublicDemoReadOnly -v
//
// Only read-only endpoints are used; no star/rating/playlist writes are sent
// to the shared demo server. The public instance rate-limits bursts, so a
// timeout is reported as a skip rather than a failure: the assertions here
// verify auth, DTO mapping and URL building, not third-party availability.
func TestPublicDemoReadOnly(t *testing.T) {
	if os.Getenv("NAVIDROME_TUI_DEMO_TEST") != "1" {
		t.Skip("set NAVIDROME_TUI_DEMO_TEST=1 to run against demo.navidrome.org")
	}
	client, err := New(Options{
		BaseURL:       "https://demo.navidrome.org",
		Username:      "demo",
		Password:      "demo",
		MaxConcurrent: 2,
		CacheTTL:      time.Minute,
		CacheSize:     64,
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	busy := func(err error) bool {
		return errors.Is(err, context.DeadlineExceeded) ||
			strings.Contains(err.Error(), "Client.Timeout") ||
			strings.Contains(err.Error(), "deadline exceeded")
	}
	// Every call gets its own short timeout; the shared demo server will drop
	// or stall some requests in a burst.
	call := func(name string, required bool, fn func(context.Context) error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := fn(ctx)
		if err == nil {
			time.Sleep(350 * time.Millisecond)
			return
		}
		if busy(err) {
			t.Logf("%s: public instance busy, skipped (%v)", name, err)
			return
		}
		if required {
			t.Errorf("%s: %v", name, err)
		} else {
			t.Logf("%s: unavailable (%v)", name, err)
		}
	}

	call("ping", true, client.Ping)
	info := client.ServerInfo()
	t.Logf("server: type=%s version=%s openSubsonic=%v", info.Type, info.ServerVersion, info.OpenSubsonic)

	call("getArtists", false, func(ctx context.Context) error {
		artists, err := client.GetArtists(ctx)
		if err == nil {
			t.Logf("artists: %d", len(artists))
		}
		return err
	})

	var coverID string
	call("getAlbumList", false, func(ctx context.Context) error {
		albums, err := client.GetAlbumList(ctx, AlbumQuery{Type: "newest", Size: 5})
		if err == nil {
			t.Logf("albums: %d", len(albums))
			if len(albums) > 0 {
				coverID = albums[0].CoverID
			}
		}
		return err
	})
	if coverID != "" {
		call("getCoverArt", false, func(ctx context.Context) error {
			cover, err := client.GetCover(ctx, coverID, 64)
			if err == nil {
				t.Logf("cover bytes: %d", len(cover))
			}
			return err
		})
	}

	call("search3", false, func(ctx context.Context) error {
		found, err := client.Search(ctx, "love", Page{Size: 5})
		if err == nil {
			t.Logf("search: songs=%d albums=%d artists=%d", len(found.Songs), len(found.Albums), len(found.Artists))
			if len(found.Songs)+len(found.Albums)+len(found.Artists) == 0 {
				t.Error("search3 returned no results for a term that exists in the demo library")
			}
		}
		return err
	})

	call("getPlaylists", false, func(ctx context.Context) error {
		playlists, err := client.GetPlaylists(ctx)
		if err == nil {
			t.Logf("playlists: %d", len(playlists))
		}
		return err
	})
	call("getGenres", false, func(ctx context.Context) error {
		genres, err := client.GetGenres(ctx)
		if err == nil {
			t.Logf("genres: %d", len(genres))
		}
		return err
	})

	var firstSong model.Song
	call("getRandomSongs", false, func(ctx context.Context) error {
		songs, err := client.GetRandomSongs(ctx, 3)
		if err == nil {
			t.Logf("random songs: %d", len(songs))
			if len(songs) > 0 {
				firstSong = songs[0]
			}
		}
		return err
	})
	if firstSong.ID != "" {
		call("getLyrics", false, func(ctx context.Context) error {
			_, err := client.GetLyrics(ctx, firstSong.ID)
			return err
		})
		if url := client.StreamURL(firstSong.ID); url == "" {
			t.Error("stream URL is empty")
		}
	}
	call("getStarred", false, func(ctx context.Context) error {
		_, err := client.GetStarred(ctx)
		return err
	})
}
