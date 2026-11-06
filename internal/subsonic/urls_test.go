package subsonic

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestStreamAndCoverURL(t *testing.T) {
	c, err := New(Options{BaseURL: "https://music.example.com:4533/", Username: "u", Password: "p", CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	stream := c.StreamURL("song-42")
	u, err := url.Parse(stream)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/rest/stream.view" || u.Query().Get("id") != "song-42" {
		t.Fatalf("stream url = %s", stream)
	}
	if u.Query().Get("u") != "u" || u.Query().Get("t") == "" || u.Query().Get("s") == "" {
		t.Fatalf("missing auth: %s", stream)
	}
	if strings.Contains(stream, "p=secret") {
		t.Fatal("password leaked into URL")
	}
	cover := c.CoverURL("al-1", 300)
	if !strings.Contains(cover, "/rest/getCoverArt.view") || !strings.Contains(cover, "size=300") {
		t.Fatalf("cover url = %s", cover)
	}
}

func TestWebURLFallsBack(t *testing.T) {
	c, _ := New(Options{BaseURL: "http://localhost:4533", Username: "u", Password: "p"})
	if got := c.WebURL("album", "al 1"); !strings.Contains(got, "/app/#/album/al%201") {
		t.Fatalf("web url = %s", got)
	}
	if got := c.WebURL("song", "x"); got != "http://localhost:4533" {
		t.Fatalf("web url fallback = %s", got)
	}
}

func TestSafeURLRedactsCredentials(t *testing.T) {
	raw := "https://host/rest/stream.view?u=alice&t=deadbeef&s=salt&p=enc:00&id=s1"
	got := safeURL(raw)
	for _, forbidden := range []string{"alice", "deadbeef", "salt", "enc%3A00"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("credential %q leaked in %s", forbidden, got)
		}
	}
	if !strings.Contains(got, "id=s1") || !strings.Contains(got, "REDACTED") {
		t.Fatalf("safeURL = %s", got)
	}
}
