package subsonic

import (
	"context"
	"net/http"
	"testing"
)

func TestGetSongsUsesEmptySearchQuery(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/search3.view" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("query") != "" {
			t.Errorf("query = %q, want an empty query", q.Get("query"))
		}
		if q.Get("songCount") != "2" || q.Get("songOffset") != "0" {
			t.Errorf("paging = %s/%s", q.Get("songCount"), q.Get("songOffset"))
		}
		if q.Get("albumCount") != "0" || q.Get("artistCount") != "0" {
			t.Errorf("albums/artists should not be requested: %v", q)
		}
		writeJSON(w, `{"subsonic-response":{"status":"ok","version":"1.16.1","searchResult3":{
			"song":[{"id":"s1","title":"One","artist":"A","album":"Al","duration":100},
			        {"id":"s2","title":"Two","artist":"B","album":"Al2","duration":120}]}}}`)
	})

	songs, err := c.GetSongs(context.Background(), Page{Offset: 0, Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 2 || songs[0].ID != "s1" || songs[1].Title != "Two" {
		t.Fatalf("songs = %+v", songs)
	}
}

func TestGetSongsPaginates(t *testing.T) {
	var offsets []string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		offsets = append(offsets, r.URL.Query().Get("songOffset"))
		writeJSON(w, `{"subsonic-response":{"status":"ok","searchResult3":{"song":[{"id":"s","title":"T"}]}}}`)
	})
	ctx := context.Background()
	if _, err := c.GetSongs(ctx, Page{Offset: 0, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetSongs(ctx, Page{Offset: 1, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if len(offsets) != 2 || offsets[0] != "0" || offsets[1] != "1" {
		t.Fatalf("offsets = %v", offsets)
	}
}

func TestSearchRequestsSongsOnly(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("albumCount") != "0" || q.Get("artistCount") != "0" {
			t.Errorf("search should only request songs: %v", q)
		}
		if q.Get("query") != "love*" {
			t.Errorf("query = %q", q.Get("query"))
		}
		writeJSON(w, `{"subsonic-response":{"status":"ok","searchResult3":{"song":[{"id":"s1","title":"Love"}]}}}`)
	})
	res, err := c.Search(context.Background(), "love", Page{Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Songs) != 1 || len(res.Albums) != 0 || len(res.Artists) != 0 {
		t.Fatalf("search result = %+v", res)
	}
}
