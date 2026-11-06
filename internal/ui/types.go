package ui

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/subsonic-tui/subsonic-tui/internal/config"
	"github.com/subsonic-tui/subsonic-tui/internal/keymap"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/player"
	"github.com/subsonic-tui/subsonic-tui/internal/subsonic"
	"github.com/subsonic-tui/subsonic-tui/internal/ui/components"
)

// ScrobbleService is implemented by the scrobbler package.
type ScrobbleService interface {
	NowPlaying(song model.Song)
	Submitted(song model.Song, started time.Time)
	ShouldScrobble(song model.Song, positionSeconds int) bool
	Close()
}

// VerifyFunc validates a server profile on a background goroutine. It must
// never mutate the shared Config, which the UI reads on every frame.
type VerifyFunc func(config.Server) (subsonic.Client, error)

// CommitFunc applies a verified profile on the UI goroutine.
type CommitFunc func(config.Server, subsonic.Client) error

// Deps is the dependency container shared by the UI and the app wiring.
// The client is guarded because tea.Cmd goroutines read it concurrently with
// the login wizard replacing it.
type Deps struct {
	mu         sync.RWMutex
	Config     *config.Config
	ConfigPath string
	Client     subsonic.Client
	Player     player.Player
	Scrobbler  ScrobbleService
	Keys       *keymap.Resolver
	Logger     *slog.Logger
	Verify     VerifyFunc
	Commit     CommitFunc
	Save       func() error
}

// API returns the current API client (nil before the first login).
func (d *Deps) API() subsonic.Client {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Client
}

// SetClient swaps the active client.
func (d *Deps) SetClient(c subsonic.Client) {
	d.mu.Lock()
	d.Client = c
	d.mu.Unlock()
}

// StreamURL is injected into the player so a server switch does not require
// rebuilding the mpv process.
func (d *Deps) StreamURL(song model.Song) string {
	c := d.API()
	if c == nil {
		return ""
	}
	return c.StreamURL(song.ID)
}

// Item is the uniform browser row model.
type Item = components.Item

const (
	ItemSection  = components.ItemSection
	ItemArtist   = components.ItemArtist
	ItemAlbum    = components.ItemAlbum
	ItemPlaylist = components.ItemPlaylist
	ItemGenre    = components.ItemGenre
	ItemSong     = components.ItemSong
	ItemNav      = components.ItemNav
)

func songItem(s model.Song) Item {
	return Item{
		Kind: ItemSong, ID: s.ID, Title: s.DisplayTitle(), Subtitle: s.DisplayArtist(),
		Extra: s.DurationText(), CoverID: s.CoverID, Duration: s.Duration, Starred: s.Starred, Song: s,
	}
}

func albumItem(a model.Album) Item {
	count := a.SongCount
	if count == 0 {
		count = len(a.Songs)
	}
	return Item{
		Kind: ItemAlbum, ID: a.ID, Title: a.Name, Subtitle: a.Artist,
		Extra: fmt.Sprintf("%d 首", count), CoverID: a.CoverID, Duration: a.Duration,
		Starred: a.Starred, Album: a,
	}
}

func artistItem(a model.Artist) Item {
	return Item{
		Kind: ItemArtist, ID: a.ID, Title: a.Name, CoverID: a.CoverID,
		Extra: fmt.Sprintf("%d 专辑", a.AlbumCount), Starred: a.Starred, Artist: a,
	}
}

func playlistItem(p model.Playlist) Item {
	return Item{
		Kind: ItemPlaylist, ID: p.ID, Title: p.Name, Subtitle: p.Owner,
		Extra: fmt.Sprintf("%d 首", p.SongCount), CoverID: p.CoverID, Duration: p.Duration, Playlist: p,
	}
}

func genreItem(g model.Genre) Item {
	return Item{
		Kind: ItemGenre, ID: g.Name, Title: g.Name, GenreName: g.Name,
		Extra: fmt.Sprintf("%d 首", g.SongCount),
	}
}

func sectionItem(title string) Item {
	return Item{Kind: ItemSection, ID: "section:" + title, Title: title}
}

// Node is one level of the navigation stack.
type Node struct {
	Kind     model.NodeKind
	ID       string
	Title    string
	Items    []Item
	Filtered []Item
	Filter   string
	Selected int
	ReqID    int
	Page     int
	HasMore  bool
	Loading  bool
	Err      error
}

// Rows returns the list to render, applying the optional inline filter.
func (n *Node) Rows() []Item {
	if n.Filter == "" {
		return n.Items
	}
	return n.Filtered
}

func (n *Node) refreshFilter() {
	if n.Filter == "" {
		n.Filtered = nil
		return
	}
	n.Filtered = components.FilterItems(n.Items, n.Filter)
	if n.Selected >= len(n.Filtered) {
		n.Selected = max(0, len(n.Filtered)-1)
	}
}

func (n *Node) currentItem() (Item, bool) {
	rows := n.Rows()
	if len(rows) == 0 || n.Selected < 0 || n.Selected >= len(rows) {
		return Item{}, false
	}
	return rows[n.Selected], true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
