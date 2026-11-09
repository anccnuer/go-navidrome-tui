package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/subsonic-tui/subsonic-tui/internal/keymap"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/player"
	"github.com/subsonic-tui/subsonic-tui/internal/subsonic"
	"github.com/subsonic-tui/subsonic-tui/internal/theme"
	"github.com/subsonic-tui/subsonic-tui/internal/ui/components"
)

// runAction dispatches a resolved keymap action. Every call that touches the
// network or mpv is wrapped in a tea.Cmd so the UI stays responsive.
func (m *Model) runAction(a keymap.Action) tea.Cmd {
	// The immersive screen has no list: navigation keys control playback
	// instead of being routed into a no-op selection change.
	if m.view == model.ViewNowPlaying {
		switch a {
		case keymap.ActionMoveLeft, keymap.ActionSeekBack5:
			return m.playerAction(a, func() error { return m.deps.Player.SeekBy(-5) })
		case keymap.ActionMoveRight, keymap.ActionSeekForward5:
			return m.playerAction(a, func() error { return m.deps.Player.SeekBy(5) })
		case keymap.ActionMoveUp, keymap.ActionVolumeUp:
			return m.playerAction(a, func() error { return m.deps.Player.AdjustVolume(5) })
		case keymap.ActionMoveDown, keymap.ActionVolumeDown:
			return m.playerAction(a, func() error { return m.deps.Player.AdjustVolume(-5) })
		}
	}
	switch a {
	case keymap.ActionQuit:
		return tea.Quit
	case keymap.ActionRerender:
		// A state change forces Bubble Tea to rebuild the frame; also refresh
		// the artwork in case it was rendered before the palette/size changed.
		m.spinnerFrame++
		m.setStatusOK("界面已重新渲染")
		return m.refreshCover()
	case keymap.ActionMoveUp:
		// Rows are filled column-wise on screen (1 2 / 3 4 / 5 6), so up/down
		// must skip a whole row to stay in the same column.
		return m.move(-m.gridColumns())
	case keymap.ActionMoveDown:
		return m.move(m.gridColumns())
	case keymap.ActionMoveTop:
		m.setSelection(0)
	case keymap.ActionMoveBottom:
		m.setSelection(m.listLen() - 1)
	case keymap.ActionPageUp:
		m.move(-m.pageSize())
	case keymap.ActionPageDown:
		m.move(m.pageSize())
	case keymap.ActionMoveLeft:
		switch {
		case m.view == model.ViewHelp || m.view == model.ViewSettings || m.view == model.ViewLyrics:
			return m.move(-1)
		case m.gridColumns() == 2:
			// Even indices are the left column, odd ones the right column
			// (ClampOffset's page size is always even). Right -> left is -1.
			if m.selection()%2 == 1 {
				return m.move(-1)
			}
			return nil
		case !m.popNode():
			m.setStatus("已在顶层，按 b 或 Esc 返回上一级视图", false)
		}
	case keymap.ActionMoveRight:
		switch {
		case m.view == model.ViewHelp || m.view == model.ViewSettings || m.view == model.ViewLyrics:
			return m.move(1)
		case m.gridColumns() == 2:
			if m.selection()%2 == 0 && m.selection()+1 < m.listLen() {
				return m.move(1)
			}
			return nil
		default:
			return m.activateSelection()
		}
	case keymap.ActionEnter:
		return m.activateSelection()
	case keymap.ActionBack:
		return m.goBack()
	case keymap.ActionSearch:
		switch m.view {
		case model.ViewLibrary:
			m.filterActive = true
			m.filterInput = ""
			m.setStatus("输入关键字过滤当前列表，Esc 取消", false)
		case model.ViewSearch:
			m.searchFocus = true
			m.setStatus("输入新的搜索关键字，Enter 搜索", false)
		}
	case keymap.ActionGlobalSearch:
		return m.openSearch()

	case keymap.ActionHelp:
		return m.openView(model.ViewHelp)
	case keymap.ActionPlaylistView:
		return m.openView(model.ViewQueue)
	case keymap.ActionLyricsView:
		return m.openView(model.ViewLyrics)
	case keymap.ActionSettingsView:
		return m.openView(model.ViewSettings)
	case keymap.ActionNowPlayingView:
		return m.openView(model.ViewNowPlaying)

	case keymap.ActionPlayToggle:
		return m.playerAction(a, func() error { return m.deps.Player.Toggle() })
	case keymap.ActionPrev:
		return m.playerAction(a, func() error { return m.deps.Player.Prev() })
	case keymap.ActionNext:
		return m.playerAction(a, func() error { return m.deps.Player.Next() })
	case keymap.ActionSeekBack1:
		return m.playerAction(a, func() error { return m.deps.Player.SeekBy(-1) })
	case keymap.ActionSeekBack5:
		return m.playerAction(a, func() error { return m.deps.Player.SeekBy(-5) })
	case keymap.ActionSeekForward5:
		return m.playerAction(a, func() error { return m.deps.Player.SeekBy(5) })
	case keymap.ActionSeekForward10:
		return m.playerAction(a, func() error { return m.deps.Player.SeekBy(10) })
	case keymap.ActionVolumeDown:
		return m.playerAction(a, func() error { return m.deps.Player.AdjustVolume(-5) })
	case keymap.ActionVolumeUp:
		return m.playerAction(a, func() error { return m.deps.Player.AdjustVolume(5) })
	case keymap.ActionSwitchMode:
		return m.switchMode()
	case keymap.ActionSwitchTheme:
		return m.switchTheme()

	case keymap.ActionAppendNext:
		return m.appendSelected(true)
	case keymap.ActionAppendEnd:
		return m.appendSelected(false)
	case keymap.ActionDeleteFromQueue:
		return m.deleteFromQueue()
	case keymap.ActionClearQueue:
		return m.playerAction(a, func() error { return m.deps.Player.ClearQueue() })

	case keymap.ActionLikePlaying:
		return m.starPlaying(true)
	case keymap.ActionDislikePlaying:
		return m.starPlaying(false)
	case keymap.ActionTrashPlaying:
		return m.ratePlaying(1)
	case keymap.ActionLikeSelected:
		return m.starSelected(true, keymap.ActionLikeSelected)
	case keymap.ActionDislikeSelected:
		return m.starSelected(false, keymap.ActionDislikeSelected)
	case keymap.ActionTrashSelected:
		return m.rateSelected(1)

	case keymap.ActionOpenAlbumPlaying:
		return m.openRelated(model.NodeAlbum, m.playerState.Song.AlbumID, m.playerState.Song.Album, "专辑")
	case keymap.ActionOpenArtistPlaying:
		return m.openRelated(model.NodeArtist, m.playerState.Song.ArtistID, m.playerState.Song.Artist, "艺术家")
	case keymap.ActionOpenBrowserPlaying:
		return m.openBrowser("song", m.playerState.Song.ID, keymap.ActionOpenBrowserPlaying)
	case keymap.ActionSimilarPlaying:
		return m.openSimilar(m.playerState.Song.ID)
	case keymap.ActionOpenAlbumSelected:
		return m.openSelectedRelated(model.NodeAlbum)
	case keymap.ActionOpenArtistSelected:
		return m.openSelectedRelated(model.NodeArtist)
	case keymap.ActionOpenBrowserSelected:
		return m.openSelectedBrowser()
	case keymap.ActionSimilarSelected:
		return m.openSimilarSelected()
	case keymap.ActionAddToPlaylist:
		return m.choosePlaylistForSelected()
	case keymap.ActionRemoveFromPlaylist:
		return m.removeFromPlaylist()
	case keymap.ActionCollectPlaylist, keymap.ActionDiscollectPlaylist:
		// Subsonic's star/unstar has no playlist parameter; sending a playlist
		// id as a song id would be wrong. Keep the keys but explain the limit.
		m.setStatusWarn("Subsonic API 不支持歌单收藏，请在 Navidrome 网页端操作")
	case keymap.ActionActionSelected, keymap.ActionActionPlaying:
		m.setStatus("可用操作：, 喜欢 · e 下一曲播放 · E 加入队列 · Tab 加入歌单 · A 专辑 · S 艺术家", false)

	case keymap.ActionClearCache:
		m.coverCache.Clear()
		if c := m.deps.API(); c != nil {
			c.InvalidateCache("")
		}
		m.setStatusOK("缓存已清除")
		return tea.Batch(m.reloadCurrent(), m.refreshCover())
	case keymap.ActionRelogin:
		return m.openWizard()
	case keymap.ActionToggleSort:
		return m.toggleSort()
	}
	return nil
}

func (m *Model) playerAction(a keymap.Action, fn func() error) tea.Cmd {
	if m.deps.Player == nil {
		m.setStatus("播放器未初始化", true)
		return nil
	}
	return asyncAction(a, fn)
}

// ---- navigation helpers ---------------------------------------------------

func (m *Model) listLen() int {
	switch m.view {
	case model.ViewSearch:
		if m.searchNode != nil {
			return len(m.searchNode.Rows())
		}
	case model.ViewQueue:
		if m.deps.Player != nil {
			return len(m.deps.Player.Queue())
		}
	case model.ViewLibrary:
		if node := m.currentNode(); node != nil {
			return len(node.Rows())
		}
	case model.ViewHelp:
		return components.HelpLineCount(m.keys.FilterHelp(m.helpQuery))
	case model.ViewLyrics:
		return len(m.lyrics.Lines)
	}
	return 0
}

func (m *Model) selection() int {
	switch m.view {
	case model.ViewSearch:
		return m.searchNode.Selected
	case model.ViewQueue:
		return m.queueSelected
	case model.ViewHelp:
		return m.helpOffset
	case model.ViewLyrics:
		return m.lyricsOffset
	case model.ViewLibrary:
		if node := m.currentNode(); node != nil {
			return node.Selected
		}
	}
	return 0
}

func (m *Model) setSelection(v int) {
	n := m.listLen()
	if n == 0 {
		v = 0
	} else if v < 0 {
		v = 0
	} else if v >= n {
		v = n - 1
	}
	switch m.view {
	case model.ViewSearch:
		m.searchNode.Selected = v
	case model.ViewQueue:
		m.queueSelected = v
	case model.ViewHelp:
		m.helpOffset = v
	case model.ViewLyrics:
		m.lyricsOffset = v
	case model.ViewLibrary:
		if node := m.currentNode(); node != nil {
			node.Selected = v
		}
	}
}

func (m *Model) move(delta int) tea.Cmd {
	m.setSelection(m.selection() + delta)
	if m.view == model.ViewLibrary && delta > 0 {
		node := m.currentNode()
		// Paging is driven by the full dataset, not the filtered view.
		if node != nil && node.HasMore && node.Selected >= len(node.Items)-3 {
			return m.loadNextPage()
		}
	}
	return nil
}

// listRows/pageSize must always equal ListConfig.Height, which listHeight()
// now owns for every view.
func (m *Model) listRows() int { return m.listHeight() }
func (m *Model) pageSize() int { return m.listHeight() }

// gridColumns returns the number of columns that are actually drawn. It
// mirrors components.ColumnsFor so navigation cannot disagree with the
// rendered grid (e.g. a narrow menu width falling back to one column).
func (m *Model) gridColumns() int {
	if !m.doubleColumnActive() {
		return 1
	}
	_, width := m.menuLayout()
	return components.ColumnsFor(components.ListConfig{
		Width: width, Height: m.listHeight(), Double: true,
	})
}

func (m *Model) selectedItem() (Item, bool) {
	switch m.view {
	case model.ViewLibrary:
		if node := m.currentNode(); node != nil {
			return node.currentItem()
		}
	case model.ViewSearch:
		if m.searchNode != nil {
			return m.searchNode.currentItem()
		}
	case model.ViewQueue:
		if m.deps.Player == nil {
			return Item{}, false
		}
		queue := m.deps.Player.Queue()
		if m.queueSelected >= 0 && m.queueSelected < len(queue) {
			return songItem(queue[m.queueSelected]), true
		}
	}
	return Item{}, false
}

func (m *Model) activateSelection() tea.Cmd {
	if m.view == model.ViewQueue {
		if m.deps.Player == nil {
			return nil
		}
		queue := m.deps.Player.Queue()
		if len(queue) == 0 {
			return nil
		}
		idx := min(max(0, m.queueSelected), len(queue)-1)
		return m.playerAction(keymap.ActionEnter, func() error { return m.deps.Player.SetQueue(queue, idx) })
	}
	item, ok := m.selectedItem()
	if !ok {
		return nil
	}
	if m.playlistAddTarget != "" {
		if item.Kind == ItemPlaylist {
			songID := m.playlistAddTarget
			playlistID := item.ID
			m.playlistAddTarget = ""
			m.popNode()
			client := m.deps.API()
			return asyncAction(keymap.ActionAddToPlaylist, func() error {
				if client == nil {
					return errNoServer
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				return client.UpdatePlaylist(ctx, playlistID, "", []string{songID}, nil)
			})
		}
		return nil
	}
	switch item.Kind {
	case ItemSection:
		return m.move(1)
	case ItemNav:
		if item.Target == model.NodeRoot {
			return nil
		}
		return m.pushNode(item.Target, "", item.Title)
	case ItemSong:
		return m.playFromList()
	case ItemAlbum:
		return m.pushNode(model.NodeAlbum, item.ID, item.Title)
	case ItemArtist:
		return m.pushNode(model.NodeArtist, item.ID, item.Title)
	case ItemPlaylist:
		return m.pushNode(model.NodePlaylist, item.ID, item.Title)
	case ItemGenre:
		return m.pushNode(model.NodeGenreAlbums, item.GenreName, item.Title+" · 专辑")
	}
	return nil
}

func (m *Model) playFromList() tea.Cmd {
	if m.deps.Player == nil {
		m.setStatus("播放器未初始化", true)
		return nil
	}
	var rows []Item
	var selected int
	if m.view == model.ViewSearch && m.searchNode != nil {
		rows, selected = m.searchNode.Rows(), m.searchNode.Selected
	} else if node := m.currentNode(); node != nil {
		rows, selected = node.Rows(), node.Selected
	}
	songs := make([]model.Song, 0, len(rows))
	start := 0
	for i, item := range rows {
		if item.Kind != ItemSong {
			continue
		}
		if i == selected {
			start = len(songs)
		}
		songs = append(songs, item.Song)
	}
	if len(songs) == 0 {
		return nil
	}
	return m.playerAction(keymap.ActionEnter, func() error { return m.deps.Player.SetQueue(songs, start) })
}

func (m *Model) goBack() tea.Cmd {
	if m.playlistAddTarget != "" {
		m.playlistAddTarget = ""
		if m.popNode() {
			m.setStatus("已取消加入歌单", false)
			return nil
		}
	}
	if m.filterActive {
		m.filterActive = false
		m.filterInput = ""
		if node := m.currentNode(); node != nil {
			node.Filter = ""
			node.refreshFilter()
		}
		return nil
	}
	if m.view != model.ViewLibrary {
		m.view = m.prevView
		if m.view == model.ViewWizard {
			m.view = model.ViewLibrary
		}
		m.searchFocus = false
		m.helpFocus = false
		return nil
	}
	if !m.popNode() {
		m.setStatus("已在顶层，b / Esc 返回上一视图，q 退出", false)
	}
	return nil
}

func (m *Model) openView(v model.ViewKind) tea.Cmd {
	wasImmersive := m.view == model.ViewNowPlaying
	var cmd tea.Cmd
	if m.view == v {
		// Pressing the same view key again returns to the previous screen.
		m.view = m.prevView
		if m.view == model.ViewWizard {
			m.view = model.ViewLibrary
		}
		m.helpFocus = false
	} else {
		m.prevView = m.view
		m.view = v
		m.filterActive = false
		if v == model.ViewHelp {
			m.helpOffset = 0
			m.helpQuery = ""
			m.helpFocus = true
		}
		if v == model.ViewLyrics {
			m.lyricsOffset = 0
			if m.playerState.Song.ID != "" && m.lyricsSongID != m.playerState.Song.ID {
				cmd = m.onSongMaybeChanged()
			}
		}
	}
	// The immersive screen uses a different artwork size than the right rail,
	// so regenerate it when entering or leaving that view.
	if wasImmersive || m.view == model.ViewNowPlaying {
		if refresh := m.refreshCover(); refresh != nil {
			if cmd != nil {
				return tea.Batch(cmd, refresh)
			}
			return refresh
		}
	}
	return cmd
}

func (m *Model) openSearch() tea.Cmd {
	if m.view != model.ViewSearch {
		m.prevView = m.view
	}
	m.view = model.ViewSearch
	m.searchFocus = true
	m.searchInput = ""
	return nil
}

func (m *Model) openWizard() tea.Cmd {
	server, _ := m.cfg.ActiveServer()
	m.wizard = newWizard(server)
	m.wizardForced = true
	m.wizard.busy = false
	m.wizard.err = ""
	m.view = model.ViewWizard
	return nil
}

func (m *Model) toggleSort() tea.Cmd {
	var rows *[]Item
	if node := m.currentNode(); node != nil {
		rows = &node.Items
	} else if m.view == model.ViewSearch && m.searchNode != nil {
		rows = &m.searchNode.Items
	}
	if rows == nil {
		return nil
	}
	selected := m.selection()
	for i, j := 0, len(*rows)-1; i < j; i, j = i+1, j-1 {
		(*rows)[i], (*rows)[j] = (*rows)[j], (*rows)[i]
	}
	if len(*rows) > 0 {
		m.setSelection(len(*rows) - 1 - selected)
	}
	if node := m.currentNode(); node != nil {
		node.refreshFilter()
	} else {
		m.searchNode.refreshFilter()
	}
	m.setStatus("已反转当前排序", false)
	return nil
}

// ---- playback / annotations ----------------------------------------------

func (m *Model) switchMode() tea.Cmd {
	if m.deps.Player == nil {
		return nil
	}
	next := m.playerState.Mode.Next()
	m.cfg.Player.Mode = player.ModeString(next)
	m.persist()
	m.setStatusOK("播放模式：" + next.String())
	return m.playerAction(keymap.ActionSwitchMode, func() error {
		m.deps.Player.SetMode(next)
		return nil
	})
}

func (m *Model) switchTheme() tea.Cmd {
	name := theme.Next(m.cfg.Theme.Name)
	palette, err := theme.Resolve(name, "")
	if err != nil {
		m.setStatus("主题加载失败："+err.Error(), true)
		return nil
	}
	m.cfg.Theme.Name = name
	m.cfg.Theme.CustomPath = ""
	m.styles = theme.New(palette, m.depth)
	m.persist()
	m.setStatusOK("主题已切换为 " + name)
	return m.refreshCover()
}

func (m *Model) persist() {
	if m.deps.Save != nil {
		if err := m.deps.Save(); err != nil {
			m.log.Warn("保存配置失败", "err", err)
		}
	}
}

func (m *Model) appendSelected(next bool) tea.Cmd {
	item, ok := m.selectedItem()
	if !ok || item.Kind != ItemSong || m.deps.Player == nil {
		m.setStatus("请先选中一首歌曲", false)
		return nil
	}
	songs := []model.Song{item.Song}
	return m.playerAction(keymap.ActionAppendNext, func() error {
		if next {
			return m.deps.Player.InsertNext(songs)
		}
		return m.deps.Player.Append(songs)
	})
}

func (m *Model) deleteFromQueue() tea.Cmd {
	if m.deps.Player == nil {
		return nil
	}
	idx := m.queueSelected
	if m.view != model.ViewQueue {
		idx = m.deps.Player.CurrentIndex()
	}
	return m.playerAction(keymap.ActionDeleteFromQueue, func() error {
		return m.deps.Player.RemoveAt(idx)
	})
}

func (m *Model) starPlaying(on bool) tea.Cmd {
	song := m.playerState.Song
	if song.ID == "" {
		return nil
	}
	return m.star(song.ID, subsonic.StarSong, on, keymap.ActionLikePlaying)
}

func (m *Model) starSelected(on bool, action keymap.Action) tea.Cmd {
	item, ok := m.selectedItem()
	if !ok || item.Kind == ItemSection || item.Kind == ItemPlaylist {
		return nil
	}
	kind := subsonic.StarSong
	switch item.Kind {
	case ItemAlbum:
		kind = subsonic.StarAlbum
	case ItemArtist:
		kind = subsonic.StarArtist
	}
	return m.star(item.ID, kind, on, action)
}

func (m *Model) star(id string, kind subsonic.StarKind, on bool, action keymap.Action) tea.Cmd {
	client := m.deps.API()
	if client == nil {
		m.setStatus(errNoServer.Error(), true)
		return nil
	}
	return asyncAction(action, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return client.Star(ctx, kind, id, on)
	})
}

func (m *Model) ratePlaying(rating int) tea.Cmd {
	song := m.playerState.Song
	if song.ID == "" {
		return nil
	}
	return m.rate(song.ID, rating, keymap.ActionTrashPlaying)
}

func (m *Model) rateSelected(rating int) tea.Cmd {
	item, ok := m.selectedItem()
	if !ok || item.Kind != ItemSong {
		return nil
	}
	return m.rate(item.ID, rating, keymap.ActionTrashSelected)
}

func (m *Model) rate(id string, rating int, action keymap.Action) tea.Cmd {
	client := m.deps.API()
	if client == nil {
		return nil
	}
	return asyncAction(action, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return client.SetRating(ctx, id, rating)
	})
}

func (m *Model) openRelated(kind model.NodeKind, id, title, label string) tea.Cmd {
	if id == "" {
		m.setStatus("当前曲目没有关联的"+label+"信息", false)
		return nil
	}
	if title == "" {
		title = label
	}
	return m.pushNode(kind, id, title)
}

func (m *Model) openSelectedRelated(kind model.NodeKind) tea.Cmd {
	item, ok := m.selectedItem()
	if !ok {
		return nil
	}
	if kind == model.NodeAlbum {
		id := item.Song.AlbumID
		if id == "" && item.Kind == ItemAlbum {
			return m.pushNode(model.NodeAlbum, item.ID, item.Title)
		}
		return m.openRelated(kind, id, item.Song.Album, "专辑")
	}
	id := item.Song.ArtistID
	if id == "" && item.Kind == ItemArtist {
		return m.pushNode(model.NodeArtist, item.ID, item.Title)
	}
	return m.openRelated(kind, id, item.Song.Artist, "艺术家")
}

func (m *Model) openBrowser(kind, id string, action keymap.Action) tea.Cmd {
	client := m.deps.API()
	if client == nil {
		return nil
	}
	return openBrowserCmd(client.WebURL(kind, id), action)
}

func (m *Model) openSelectedBrowser() tea.Cmd {
	item, ok := m.selectedItem()
	if !ok {
		return nil
	}
	kind := "song"
	switch item.Kind {
	case ItemAlbum:
		kind = "album"
	case ItemArtist:
		kind = "artist"
	case ItemPlaylist:
		kind = "playlist"
	}
	return m.openBrowser(kind, item.ID, keymap.ActionOpenBrowserSelected)
}

func (m *Model) openSimilar(id string) tea.Cmd {
	if id == "" {
		return nil
	}
	return m.pushNode(model.NodeRandom, id, "相似歌曲")
}

func (m *Model) openSimilarSelected() tea.Cmd {
	item, ok := m.selectedItem()
	if !ok || item.Kind != ItemSong {
		m.setStatus("请先选中一首歌曲", false)
		return nil
	}
	return m.openSimilar(item.ID)
}

func (m *Model) choosePlaylistForSelected() tea.Cmd {
	item, ok := m.selectedItem()
	if !ok || item.Kind != ItemSong {
		m.setStatus("请先选中一首歌曲", false)
		return nil
	}
	m.playlistAddTarget = item.ID
	m.setStatus("请选择要加入的歌单，Esc 取消", false)
	return m.pushNode(model.NodePlaylists, "", "选择目标歌单")
}

func (m *Model) removeFromPlaylist() tea.Cmd {
	node := m.currentNode()
	if node == nil || node.Kind != model.NodePlaylist {
		m.setStatus("请先在歌单曲目列表中选中歌曲", false)
		return nil
	}
	item, ok := m.selectedItem()
	if !ok || item.Kind != ItemSong {
		return nil
	}
	client := m.deps.API()
	if client == nil {
		return nil
	}
	// songIndexToRemove refers to the playlist's original order, not the
	// filtered view's selection index.
	originalIndex := node.Selected
	for i, it := range node.Items {
		if it.ID == item.ID && it.Kind == ItemSong {
			originalIndex = i
			break
		}
	}
	return asyncAction(keymap.ActionRemoveFromPlaylist, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return client.UpdatePlaylist(ctx, node.ID, "", nil, []int{originalIndex})
	})
}

func (m *Model) viewLabel() string {
	switch m.view {
	case model.ViewLibrary:
		if node := m.currentNode(); node != nil {
			return fmt.Sprintf("%s · %d 项", node.Title, len(node.Rows()))
		}
	case model.ViewSearch:
		return "搜索 · " + m.searchQuery
	case model.ViewQueue:
		return "播放队列"
	case model.ViewLyrics:
		return "同步歌词"
	case model.ViewHelp:
		return "快捷键帮助"
	case model.ViewSettings:
		return "设置"
	case model.ViewNowPlaying:
		return "正在播放"
	case model.ViewWizard:
		return "连接向导"
	}
	return "音乐库"
}

func (m *Model) hint() string {
	// Toasts have their own reserved row; the status bar keeps showing the
	// contextual shortcut hints instead of being taken over for 4 seconds.
	if m.filterActive {
		return "/ " + m.filterInput + "▏  Esc 取消"
	}
	switch m.view {
	case model.ViewSearch:
		return "Enter 搜索 · Esc 返回"
	case model.ViewQueue:
		return "Enter 播放 · \\ 删除 · c 返回"
	case model.ViewLyrics:
		return "Space 播放/暂停 · y 返回"
	case model.ViewHelp:
		return "输入过滤 · Esc 返回"
	case model.ViewNowPlaying:
		return "h/l 快退快进 · j/k 音量 · Space 暂停 · z 返回"
	default:
		return strings.TrimSpace("/ 过滤  ? 帮助  Space 播放/暂停")
	}
}
