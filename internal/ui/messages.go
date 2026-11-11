package ui

import (
	"context"
	"fmt"
	"image"
	"os/exec"
	"runtime"
	"time"

	"github.com/anccnuer/go-navidrome-tui/internal/config"
	"github.com/anccnuer/go-navidrome-tui/internal/cover"
	"github.com/anccnuer/go-navidrome-tui/internal/cover/sixel"
	"github.com/anccnuer/go-navidrome-tui/internal/keymap"
	"github.com/anccnuer/go-navidrome-tui/internal/model"
	"github.com/anccnuer/go-navidrome-tui/internal/player"
	"github.com/anccnuer/go-navidrome-tui/internal/subsonic"
	tea "github.com/charmbracelet/bubbletea"
)

// ---- Messages -------------------------------------------------------------

type nodeLoadedMsg struct {
	req     int
	kind    model.NodeKind
	id      string
	items   []Item
	hasMore bool
	append  bool
	err     error
}

type searchLoadedMsg struct {
	req   int
	items []Item
	err   error
}

type coverLoadedMsg struct {
	key   string
	art   string // portable half-block artwork (always present as fallback)
	sixel string // optional DCS payload, empty when unavailable
	err   error
}

type lyricsLoadedMsg struct {
	songID string
	lyrics model.Lyrics
	err    error
}

type playerEventMsg struct {
	event  player.Event
	closed bool
}

type tickMsg time.Time

type actionResultMsg struct {
	action keymap.Action
	text   string
	err    error
}

type wizardResultMsg struct {
	server config.Server
	client subsonic.Client
	err    error
}

type pingResultMsg struct{ err error }

var errNoServer = fmt.Errorf("尚未配置 Navidrome 服务器，请按 w 打开登录向导")

// ---- Command factories ----------------------------------------------------
// Every network or process call runs inside a tea.Cmd so Update/View never
// block. Values needed by the closure are copied before the goroutine starts.

func (m *Model) loadNodeCmd(node *Node, page int) tea.Cmd {
	req := m.nextReq()
	node.ReqID = req
	node.Loading = true
	node.Err = nil
	kind, id := node.Kind, node.ID
	client := m.deps.API()
	pageSize := m.deps.Config.Cache.PageSize
	return func() tea.Msg {
		if client == nil {
			return nodeLoadedMsg{req: req, kind: kind, id: id, err: errNoServer}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		items, hasMore, err := fetchNode(ctx, client, kind, id, page, pageSize)
		return nodeLoadedMsg{
			req: req, kind: kind, id: id, items: items,
			hasMore: hasMore, append: page > 0, err: err,
		}
	}
}

// searchItems maps a search response to list rows. Only songs are returned:
// a section header would add a phantom row and make the result count, the row
// numbers and the paging arithmetic all off by one.
func searchItems(result model.SearchResult) []Item {
	items := make([]Item, 0, len(result.Songs))
	for _, s := range result.Songs {
		items = append(items, songItem(s))
	}
	return items
}

func (m *Model) searchCmd(query string) tea.Cmd {
	req := m.nextReq()
	m.searchReq = req
	m.searchLoading = true
	client := m.deps.API()
	size := m.deps.Config.Cache.PageSize
	return func() tea.Msg {
		if client == nil {
			return searchLoadedMsg{req: req, err: errNoServer}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		result, err := client.Search(ctx, query, subsonic.Page{Size: size})
		if err != nil {
			return searchLoadedMsg{req: req, err: err}
		}
		return searchLoadedMsg{req: req, items: searchItems(result)}
	}
}

func (m *Model) coverCmd(song model.Song, width, height int) tea.Cmd {
	key := cover.CacheKey(song.CoverID, width, height, m.depth)
	m.coverKey = key
	// Sixel works in pixels, while the text artwork works in character cells.
	// Convert with the configured cell size so both representations cover the
	// same rectangle of the screen.
	pxW, pxH := 0, 0
	if m.sixelLayer != nil && m.sixelLayer.Enabled() {
		cellW, cellH := m.cfg.UI.SixelCellWidth, m.cfg.UI.SixelCellHeight
		if cellW < 1 {
			cellW = 8
		}
		if cellH < 1 {
			cellH = 16
		}
		pxW, pxH = width*cellW, height*cellH
	}
	sixelKey := fmt.Sprintf("%s:sixel:%dx%d", key, pxW, pxH)
	if art, ok := m.coverCache.Get(key); ok {
		cachedSixel, hasSixel := m.coverCache.Get(sixelKey)
		// A cached text rendering without a DCS payload must not short-circuit
		// the Sixel path: re-fetch and encode the image so the next refresh can
		// actually switch from half-block artwork to the real picture.
		if pxW < 1 || hasSixel {
			return func() tea.Msg {
				return coverLoadedMsg{key: key, art: art, sixel: cachedSixel}
			}
		}
	}
	client := m.deps.API()
	colors := append([]string(nil), m.styles.T.Gradient...)
	seed := song.CoverID + song.ID
	depth := m.depth
	coverID := song.CoverID
	// A fixed upstream size keeps the API cache key stable when the window is
	// resized; the image is downsampled locally to the requested cell grid.
	const requestSize = 320
	return func() tea.Msg {
		placeholder := cover.RenderPlaceholder(seed, width, height, depth, colors)
		// A Sixel placeholder keeps the artwork area visually stable while the
		// real cover is loading; an encode failure simply falls back to text.
		encodeOpts := sixel.EncodeOptions{Colors: m.cfg.UI.SixelColors, Dither: true}
		encode := func(img image.Image) string {
			if pxW < 1 {
				return ""
			}
			out, err := sixel.EncodeWith(img, pxW, pxH, encodeOpts)
			if err != nil {
				return ""
			}
			m.coverCache.Set(sixelKey, out)
			return out
		}
		fallback := func(err error) coverLoadedMsg {
			return coverLoadedMsg{
				key: key, art: placeholder,
				sixel: encode(sixel.GradientImage(width, height)), err: err,
			}
		}
		if client == nil || coverID == "" {
			return fallback(nil)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		data, err := client.GetCover(ctx, coverID, requestSize)
		if err != nil {
			return fallback(err)
		}
		img, err := cover.Decode(data)
		if err != nil {
			return fallback(err)
		}
		art := cover.Render(img, width, height, depth)
		m.coverCache.Set(key, art)
		return coverLoadedMsg{key: key, art: art, sixel: encode(img)}
	}
}

func (m *Model) lyricsCmd(songID string) tea.Cmd {
	client := m.deps.API()
	return func() tea.Msg {
		if client == nil {
			return lyricsLoadedMsg{songID: songID, err: errNoServer}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		lines, err := client.GetLyrics(ctx, songID)
		return lyricsLoadedMsg{songID: songID, lyrics: lines, err: err}
	}
}

func playerEventCmd(events <-chan player.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return playerEventMsg{closed: true}
		}
		return playerEventMsg{event: ev}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func asyncAction(action keymap.Action, fn func() error) tea.Cmd {
	return func() tea.Msg {
		err := fn()
		return actionResultMsg{action: action, err: err}
	}
}

func openBrowserCmd(raw string, action keymap.Action) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", raw)
		case "darwin":
			cmd = exec.Command("open", raw)
		default:
			cmd = exec.Command("xdg-open", raw)
		}
		if err := cmd.Start(); err != nil {
			return actionResultMsg{action: action, err: fmt.Errorf("打开浏览器失败: %w", err)}
		}
		return actionResultMsg{action: action, text: "已在浏览器中打开"}
	}
}

func (m *Model) pingCmd() tea.Cmd {
	client := m.deps.API()
	return func() tea.Msg {
		if client == nil {
			return pingResultMsg{err: errNoServer}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return pingResultMsg{err: client.Ping(ctx)}
	}
}

func (m *Model) wizardSaveCmd(server config.Server) tea.Cmd {
	verify := m.deps.Verify
	return func() tea.Msg {
		if verify == nil {
			return wizardResultMsg{server: server, err: fmt.Errorf("应用未正确初始化")}
		}
		client, err := verify(server)
		return wizardResultMsg{server: server, client: client, err: err}
	}
}

// fetchNode is a pure translation from API results to browser rows.
func fetchNode(ctx context.Context, client subsonic.Client, kind model.NodeKind, id string, page, size int) ([]Item, bool, error) {
	offset := page * size
	switch kind {
	case model.NodeRoot:
		// Root menu entries show only their names; descriptions were noisy.
		return []Item{
			{Kind: ItemNav, Title: "全部歌曲", Target: model.NodeAllSongs},
			{Kind: ItemNav, Title: "歌单", Target: model.NodePlaylists},
		}, false, nil

	case model.NodeAllSongs:
		songs, err := client.GetSongs(ctx, subsonic.Page{Offset: offset, Size: size})
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(songs))
		for _, s := range songs {
			items = append(items, songItem(s))
		}
		// A full page means there is probably another one.
		return items, len(songs) == size, nil

	case model.NodeArtists:
		artists, err := client.GetArtists(ctx)
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(artists))
		for _, a := range artists {
			items = append(items, artistItem(a))
		}
		return items, false, nil

	case model.NodeArtist:
		artist, err := client.GetArtist(ctx, id)
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(artist.Albums))
		for _, a := range artist.Albums {
			items = append(items, albumItem(a))
		}
		return items, false, nil

	case model.NodeAlbums:
		albums, err := client.GetAlbumList(ctx, subsonic.AlbumQuery{Type: "newest", Size: size, Offset: offset})
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(albums))
		for _, a := range albums {
			items = append(items, albumItem(a))
		}
		return items, len(albums) == size, nil

	case model.NodeGenreAlbums:
		albums, err := client.GetAlbumList(ctx, subsonic.AlbumQuery{Type: "byGenre", Genre: id, Size: size, Offset: offset})
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(albums))
		for _, a := range albums {
			items = append(items, albumItem(a))
		}
		return items, len(albums) == size, nil

	case model.NodeAlbum:
		album, err := client.GetAlbum(ctx, id)
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(album.Songs))
		for _, s := range album.Songs {
			items = append(items, songItem(s))
		}
		return items, false, nil

	case model.NodePlaylists:
		playlists, err := client.GetPlaylists(ctx)
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(playlists))
		for _, p := range playlists {
			items = append(items, playlistItem(p))
		}
		return items, false, nil

	case model.NodePlaylist:
		playlist, err := client.GetPlaylist(ctx, id)
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(playlist.Songs))
		for _, s := range playlist.Songs {
			items = append(items, songItem(s))
		}
		return items, false, nil

	case model.NodeGenres:
		genres, err := client.GetGenres(ctx)
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(genres))
		for _, g := range genres {
			items = append(items, genreItem(g))
		}
		return items, false, nil

	case model.NodeStarred:
		starred, err := client.GetStarred(ctx)
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, starred.Total()+3)
		if len(starred.Songs) > 0 {
			items = append(items, sectionItem(fmt.Sprintf("歌曲 · %d", len(starred.Songs))))
			for _, s := range starred.Songs {
				items = append(items, songItem(s))
			}
		}
		if len(starred.Albums) > 0 {
			items = append(items, sectionItem(fmt.Sprintf("专辑 · %d", len(starred.Albums))))
			for _, a := range starred.Albums {
				items = append(items, albumItem(a))
			}
		}
		if len(starred.Artists) > 0 {
			items = append(items, sectionItem(fmt.Sprintf("艺术家 · %d", len(starred.Artists))))
			for _, a := range starred.Artists {
				items = append(items, artistItem(a))
			}
		}
		return items, false, nil

	case model.NodeRandom:
		var songs []model.Song
		var err error
		if id != "" {
			// A non-empty id means "similar songs for this track".
			songs, err = client.GetSimilarSongs(ctx, id, max(size, 50))
		} else {
			songs, err = client.GetRandomSongs(ctx, max(size, 50))
		}
		if err != nil {
			return nil, false, err
		}
		items := make([]Item, 0, len(songs))
		for _, s := range songs {
			items = append(items, songItem(s))
		}
		return items, false, nil

	default:
		return nil, false, fmt.Errorf("未知的浏览节点 %d", kind)
	}
}
