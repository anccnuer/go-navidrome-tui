package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/subsonic-tui/subsonic-tui/internal/config"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/theme"
	"github.com/subsonic-tui/subsonic-tui/internal/ui/components"
)

// View renders one complete frame. No IO or image decoding happens here.
func (m *Model) View() string {
	if m.width < 24 || m.height < 8 {
		return m.styles.Warning.Render("终端窗口太小，请至少调整为 24×8")
	}
	if m.view == model.ViewWizard {
		return m.wizard.render(m.styles, m.width, m.height)
	}

	bodyH := m.bodyHeight()
	compactBar := m.height < 14
	status := components.RenderStatusBar(m.styles, components.StatusInfo{
		Server:     m.activeServerName(),
		ViewName:   m.viewLabel(),
		Connection: m.connection,
		Warning:    m.connWarn || m.statusWarn,
		Breadcrumb: m.breadcrumbTrail(),
		Center:     m.statusCenter(),
	}, m.width)

	body := m.renderBody(bodyH)
	toastLine := m.toastLine(time.Now(), m.width)
	now := components.RenderNowPlaying(m.styles, m.nowPlayingInfo(), m.width, compactBar)

	parts := []string{status, body}
	if toastLine != "" {
		parts = append(parts, toastLine)
	}
	parts = append(parts, now)
	return components.FitHeight(strings.Join(parts, "\n"), m.height)
}

// statusCenter builds the middle status-bar segment (go-musicfox injects a
// similar "app · page · quality" block).
func (m *Model) statusCenter() string {
	node := m.currentNode()
	if node == nil {
		return "subsonic-tui"
	}
	total := len(node.Rows())
	pos := 0
	if total > 0 {
		pos = min(node.Selected+1, total)
	}
	return fmt.Sprintf("subsonic-tui · [%d/%d]", pos, total)
}

// breadcrumbTrail returns the navigation stack titles for the status bar.
func (m *Model) breadcrumbTrail() []string {
	parts := make([]string, 0, len(m.stack))
	for _, node := range m.stack {
		if node.Title != "" {
			parts = append(parts, node.Title)
		}
	}
	return parts
}

// nowPlayingInfo is the single source of playback display data, shared by the
// bottom bar, the immersive view and mouse hit-testing.
func (m *Model) nowPlayingInfo() components.NowPlayingInfo {
	info := components.NowPlayingInfo{
		Status:     m.playerState.Status,
		Title:      m.playerState.Song.DisplayTitle(),
		Artist:     m.playerState.Song.DisplayArtist(),
		Album:      m.playerState.Song.Album,
		Position:   m.playerState.Position,
		Duration:   m.playerState.Duration,
		Volume:     m.playerState.Volume,
		Mode:       m.playerState.Mode,
		Starred:    m.playerState.Song.Starred,
		Available:  m.deps.Player == nil || m.deps.Player.Available(),
		Message:    m.mpvMessage,
		Frame:      m.spinnerFrame,
		Animations: m.cfg.UI.Animations,
	}
	if m.playerState.Song.ID == "" {
		// Let the bar show its own placeholder instead of "未知曲目".
		info.Title, info.Artist, info.Album = "", "", ""
	}
	return info
}

// immersiveLayout is the shared geometry of the "now playing" screen:
// album art in a left column with a generous left margin (track title below
// it), and a full-height scrolling lyric column on the right. Both the
// renderer and refreshCover use it, so the artwork always fits its column.
func (m *Model) immersiveLayout() (leftPad, artW, lyricW, artH, gap int) {
	innerW := max(20, m.width-2) // rounded border takes one cell per side
	gap = 2
	leftPad = max(4, min(10, innerW/16))
	artW = min(44, max(16, innerW*2/5))
	lyricW = innerW - leftPad - artW - gap
	if lyricW < 20 {
		artW = max(10, innerW-leftPad-20-gap)
		lyricW = innerW - leftPad - artW - gap
	}
	// Panel: 2 border rows + 1 embedded title row. Reserve rows for a blank
	// line plus the track title underneath the artwork.
	rows := max(4, m.bodyHeight()-3)
	// Terminal cells are roughly twice as tall as they are wide.
	artH = max(4, min(artW/2, rows-4))
	return leftPad, artW, lyricW, artH, gap
}

// renderNowPlaying is the immersive, two-column playback screen (z).
func (m *Model) renderNowPlaying(width, height int) string {
	info := m.nowPlayingInfo()
	leftPad, artW, lyricW, _, gap := m.immersiveLayout()
	innerW := width - 2
	rows := max(4, height-3) // Panel border (2) + embedded title (1)

	// ---- left: artwork, then the track title (artist is on the bottom bar) ----
	title := info.Title
	if title == "" {
		title = "尚未播放任何曲目"
	}
	artLines := []string{m.styles.EmptyGlyph.Render("♪   ♫   ♪")}
	if m.coverArt != "" {
		artLines = strings.Split(m.coverArt, "\n")
	}
	titleLine := centerLine(m.styles.Primary.Bold(true).Render(
		components.Truncate(title, artW)), artW)
	// Artwork + one blank row + title form a block centred vertically.
	blockH := len(artLines) + 2
	top := max(0, (rows-blockH)/2)
	leftLines := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		idx := i - top
		var line string
		switch {
		case idx >= 0 && idx < len(artLines):
			line = artLines[idx]
		case idx == len(artLines)+1:
			line = titleLine
		}
		leftLines = append(leftLines,
			strings.Repeat(" ", leftPad)+components.PadRight(line, artW))
	}
	left := components.FitLines(strings.Join(leftLines, "\n"), leftPad+artW, rows)

	// ---- right: full-height scrolling lyrics, colour-only active line ----
	active := -1
	offset := 0
	if m.lyrics.Synced {
		active = m.lyrics.LineAt(info.Position * 1000)
		offset = components.LyricsOffsetFor(active, rows)
	} else {
		active = m.lyricsOffset
		offset = max(0, m.lyricsOffset-rows/2)
	}
	right := components.RenderLyrics(m.styles, m.lyrics, active, offset, lyricW, rows, false)

	content := components.JoinColumns(left, right, leftPad+artW, gap)
	content = components.FitLines(content, innerW, rows)
	return components.Panel(m.styles, "正在播放", content, width, height, true)
}

// menuLayout gives the menu block a bounded width and centres it horizontally.
// The old version ran the block all the way to the right edge, which pushed
// the items and the hint line off-centre.
func (m *Model) menuLayout() (start, width int) {
	w := m.width
	dual := m.doubleColumnActive()
	var menuW int
	switch {
	case dual && w >= 150:
		menuW = w * 2 / 3
	case dual && w >= 100:
		menuW = w * 3 / 4
	case w >= 100:
		menuW = min(w-8, max(56, w*3/5))
	default:
		menuW = min(w-4, max(28, w*2/3))
	}
	menuW = max(20, min(menuW, w))
	return (w - menuW) / 2, menuW
}

func (m *Model) doubleColumnActive() bool {
	if !m.cfg.UI.DoubleColumn || m.width < 100 {
		return false
	}
	if m.view == model.ViewLibrary {
		if node := m.currentNode(); node != nil && node.Kind == model.NodeRoot {
			return false // the short root menu is rendered as a centred column
		}
	}
	return m.view == model.ViewLibrary || m.view == model.ViewSearch || m.view == model.ViewQueue
}

// nowBarHeight is the single source of truth for the playback bar row budget.
// RenderNowPlaying appends its warning row whenever Message is non-empty.
func (m *Model) nowBarHeight() int {
	if m.height < 14 {
		return 1
	}
	if m.mpvMessage != "" {
		return 3
	}
	return 2
}

func (m *Model) bodyHeight() int {
	toastH := 0
	if len(m.toasts) > 0 {
		toastH = 1
	}
	return max(4, m.height-1-m.nowBarHeight()-toastH)
}

// inlineLyricLines reserves 3 or 5 rows for a lyric preview under the menu,
// following go-musicfox's FullLyricLines/CompactLyricLines.
func (m *Model) inlineLyricLines() int {
	if m.view != model.ViewLibrary || m.lyrics.IsEmpty() || m.height < 20 {
		return 0
	}
	if m.height >= 26 {
		return 5
	}
	return 3
}

// topPadding reserves a small blank band above the menu title, echoing
// go-musicfox's menuStartRow (kept small so the list stays dense).
func (m *Model) topPadding() int {
	if m.height < 20 {
		return 0
	}
	return min(2, m.height/12)
}

// listHeight is the single source of truth for list rows in every view:
// body = top padding + title + blank + list + hint.
func (m *Model) listHeight() int {
	return max(1, m.bodyHeight()-3-m.inlineLyricLines()-m.topPadding())
}

func (m *Model) activeServerName() string {
	if s, ok := m.cfg.ActiveServer(); ok {
		return s.Name
	}
	return "未配置"
}

func (m *Model) spinner() string {
	if !m.cfg.UI.Animations {
		return "◌"
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[m.spinnerFrame%len(frames)]
}

func (m *Model) renderBody(height int) string {
	// Full-width pages draw their own centred layout and must not be indented.
	switch m.view {
	case model.ViewHelp, model.ViewSettings, model.ViewNowPlaying, model.ViewLyrics:
		return m.renderMain(m.width, height)
	}
	start, width := m.menuLayout()
	return indentLines(m.renderMain(width, height), start)
}

func indentLines(block string, n int) string {
	if n <= 0 || block == "" {
		return block
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(block, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}

// centerLine centres a styled line inside the menu block. Because the menu
// block itself is centred, this also centres it on the whole screen.
func centerLine(text string, width int) string {
	w := components.VisualWidth(text)
	pad := max(0, (width-w)/2)
	return components.PadRight(strings.Repeat(" ", pad)+text, width)
}

func (m *Model) renderMain(width, height int) string {
	switch m.view {
	case model.ViewSearch:
		return m.renderSearch(width, height)
	case model.ViewQueue:
		return m.renderQueue(width, height)
	case model.ViewLyrics:
		return m.renderLyrics(width, height)
	case model.ViewHelp:
		return m.renderHelp(width, height)
	case model.ViewSettings:
		return m.renderSettings(width, height)
	case model.ViewNowPlaying:
		return m.renderNowPlaying(width, height)
	default:
		return m.renderLibrary(width, height)
	}
}

func (m *Model) listConfig(node *Node, width, height int) components.ListConfig {
	rows := []Item{}
	selected := 0
	if node != nil {
		rows, selected = node.Rows(), node.Selected
	}
	return components.ListConfig{
		Items: rows, Selected: selected, Width: width, Height: height,
		Double: m.doubleColumnActive(), PlayingID: m.playerState.Song.ID,
		ShowNumber: true,
	}
}

func (m *Model) renderLibrary(width, height int) string {
	node := m.currentNode()
	title := "主页"
	if node != nil {
		title = node.Title
		if node.Filter != "" {
			title += "  /" + node.Filter
		}
		if node.Loading {
			title += "  " + m.spinner()
		}
		if node.HasMore {
			title += "  ↓"
		}
	}
	labelText := m.styles.Subtitle.Render(components.Truncate(title, width-2))
	if len(m.stack) > 1 {
		labelText = m.styles.Muted.Render("←") + " " +
			m.styles.Subtitle.Render(components.Truncate(title, width-3))
	}
	label := centerLine(labelText, width)

	listH := m.listHeight()
	cfg := m.listConfig(node, width, listH)
	if node != nil {
		cfg.Loading = node.Loading
		cfg.Center = node.Kind == model.NodeRoot
	}
	list := components.RenderList(m.styles, cfg)
	// Replace the first row instead of appending: FitLines clips anything
	// beyond the viewport height, so an appended banner was never visible.
	if node != nil && node.Err != nil && listH >= 3 {
		lines := strings.Split(list, "\n")
		lines[0] = components.RenderErrorState(m.styles, node.Err.Error(), "按 r 重试", width)
		list = strings.Join(lines, "\n")
	}
	hint := components.RenderHintLine(m.styles, m.menuHints(), width)
	lyrics := m.renderInlineLyrics(width, m.inlineLyricLines())

	parts := make([]string, 0, 5+m.topPadding())
	for i := 0; i < m.topPadding(); i++ {
		parts = append(parts, "")
	}
	parts = append(parts, label, "", list, hint)
	if lyrics != "" {
		parts = append(parts, lyrics)
	}
	return components.FitLines(strings.Join(parts, "\n"), width, height)
}

// menuHints returns the go-musicfox-style shortcut pairs for the hint line.
func (m *Model) menuHints() []components.Hint {
	if m.filterActive {
		return []components.Hint{{Key: "Esc", Desc: "取消过滤"}, {Key: "Enter", Desc: "确认"}}
	}
	switch m.view {
	case model.ViewLibrary:
		if len(m.stack) > 1 {
			return []components.Hint{
				{Key: "↑↓/jk", Desc: "导航"}, {Key: "Enter", Desc: "进入 / 播放"},
				{Key: "b/Esc", Desc: "返回"}, {Key: "Space", Desc: "播放 / 暂停"},
				{Key: "/", Desc: "过滤"}, {Key: "?", Desc: "帮助"},
			}
		}
		return []components.Hint{
			{Key: "↑↓/jk", Desc: "导航"}, {Key: "Enter", Desc: "进入"},
			{Key: "z", Desc: "正在播放"}, {Key: "Ctrl+F", Desc: "搜索"},
			{Key: "?", Desc: "帮助"}, {Key: "q", Desc: "退出"},
		}
	case model.ViewSearch:
		return []components.Hint{{Key: "Enter", Desc: "搜索"}, {Key: "Esc", Desc: "返回"}, {Key: "↑↓", Desc: "选择"}}
	case model.ViewQueue:
		return []components.Hint{
			{Key: "Enter", Desc: "播放"}, {Key: "e", Desc: "下一曲"},
			{Key: "\\", Desc: "删除"}, {Key: "c", Desc: "返回"},
		}
	default:
		return []components.Hint{{Key: "b/Esc", Desc: "返回"}, {Key: "?", Desc: "帮助"}}
	}
}

// renderInlineLyrics shows the current lyric line centred with its neighbours,
// mirroring go-musicfox's 3/5-line lyric block above the player.
func (m *Model) renderInlineLyrics(width, lines int) string {
	if lines <= 0 || m.lyrics.IsEmpty() {
		return ""
	}
	active := -1
	if m.lyrics.Synced {
		active = m.lyrics.LineAt(m.playerState.Position * 1000)
	} else if m.lyricsOffset >= 0 && m.lyricsOffset < len(m.lyrics.Lines) {
		active = m.lyricsOffset
	}
	start := 0
	if active >= 0 {
		start = max(0, active-lines/2)
	}
	if limit := len(m.lyrics.Lines) - lines; start > limit && limit >= 0 {
		start = limit
	}
	out := make([]string, 0, lines)
	for i := 0; i < lines; i++ {
		idx := start + i
		if idx >= len(m.lyrics.Lines) {
			out = append(out, strings.Repeat(" ", width))
			continue
		}
		text := components.Truncate(strings.TrimSpace(m.lyrics.Lines[idx].Text), width-4)
		style := m.styles.Subtle
		if idx == active {
			style = m.styles.Primary.Bold(true)
		}
		pad := max(0, (width-components.VisualWidth(text))/2)
		out = append(out, components.PadRight(strings.Repeat(" ", pad)+style.Render(text), width))
	}
	return strings.Join(out, "\n")
}

func (m *Model) renderSearch(width, height int) string {
	inputW := width - 6
	inputStyle := m.styles.Input
	if m.searchFocus {
		inputStyle = m.styles.InputFocus
	}
	cursor := ""
	if m.searchFocus {
		cursor = "▏"
	}
	input := inputStyle.Width(inputW).Render(" / " + components.Truncate(m.searchInput+cursor, inputW-4))
	state := ""
	if m.searchLoading {
		state = " " + m.spinner() + " 搜索中"
	} else if len(m.searchNode.Items) > 0 {
		state = fmt.Sprintf("  %d 项结果", len(m.searchNode.Items))
	}
	header := input + state

	cfg := m.listConfig(m.searchNode, width, m.listHeight())
	cfg.Loading = m.searchLoading
	if m.searchNode.Loading {
		cfg.EmptyText = "正在搜索…"
	} else if m.searchQuery == "" {
		cfg.EmptyText = "输入关键字后按 Enter 搜索全部歌曲（Ctrl+F 唤起）"
	} else {
		cfg.EmptyText = "没有找到匹配的歌曲"
	}
	list := components.RenderList(m.styles, cfg)
	hint := components.RenderHintLine(m.styles, m.menuHints(), width)
	parts := make([]string, 0, 4+m.topPadding())
	for i := 0; i < m.topPadding(); i++ {
		parts = append(parts, "")
	}
	parts = append(parts, header, "", list, hint)
	return components.FitLines(strings.Join(parts, "\n"), width, height)
}

// queueItems is shared by the renderer and mouse hit-testing so both see the
// same row indices.
func (m *Model) queueItems() []Item {
	items := make([]Item, 0)
	if m.deps.Player != nil {
		for _, song := range m.deps.Player.Queue() {
			items = append(items, songItem(song))
		}
	}
	return items
}

func (m *Model) renderQueue(width, height int) string {
	items := m.queueItems()
	playingIndex := -1
	for i, it := range items {
		if it.ID != "" && it.ID == m.playerState.Song.ID {
			playingIndex = i
			break
		}
	}
	cfg := components.ListConfig{
		Items: items, Selected: m.queueSelected, Width: width, Height: m.listHeight(),
		Double:    m.doubleColumnActive(),
		PlayingID: m.playerState.Song.ID, ShowNumber: true,
		EmptyText: "播放队列为空，去全部歌曲里选一首吧",
	}
	list := components.RenderList(m.styles, cfg)
	position := ""
	if playingIndex >= 0 {
		position = fmt.Sprintf(" · 第 %d/%d 首", playingIndex+1, len(items))
	}
	title := centerLine(m.styles.Subtitle.Render("播放队列")+
		m.styles.Muted.Render(position+" · "+m.playerState.Mode.String()), width)
	hint := components.RenderHintLine(m.styles, m.menuHints(), width)
	parts := make([]string, 0, 4+m.topPadding())
	for i := 0; i < m.topPadding(); i++ {
		parts = append(parts, "")
	}
	parts = append(parts, title, "", list, hint)
	return components.FitLines(strings.Join(parts, "\n"), width, height)
}

func (m *Model) renderLyrics(width, height int) string {
	active := -1
	offset := 0
	lyricHeight := max(1, height-3)
	if m.lyrics.Synced {
		active = m.lyrics.LineAt(m.playerState.Position * 1000)
		offset = components.LyricsOffsetFor(active, lyricHeight)
	} else {
		// Plain lyrics have no timestamps: j/k moves the current line and the
		// viewport scrolls to keep it centred.
		active = m.lyricsOffset
		offset = max(0, m.lyricsOffset-lyricHeight/2)
	}
	source := m.lyrics.Source
	if m.lyricsLoading {
		source = "加载中 " + m.spinner()
	} else if m.lyrics.IsEmpty() {
		source = "无歌词"
	}
	header := "  " + m.styles.Subtitle.Render(components.Truncate(m.playerState.Song.DisplayTitle(), width-8)) +
		"   " + m.styles.Subtle.Render(source)
	lyricBlock := components.RenderLyrics(m.styles, m.lyrics, active, offset, width-2, lyricHeight, true)
	content := header + "\n" + lyricBlock
	if height < 10 {
		content = lyricBlock
	}
	return components.Panel(m.styles, "", components.FitLines(content, width-2, height-2), width, height, true)
}

// renderHelp follows go-musicfox's help popup: a centred card at 70% width
// (clamped 40..120) and 80% height, with the title embedded in the frame.
func (m *Model) renderHelp(width, height int) string {
	pw := min(120, max(40, width*7/10))
	if pw > width {
		pw = width
	}
	ph := min(height-2, max(10, height*8/10))
	if ph < 6 {
		ph = height
	}
	innerW := max(20, pw-4)
	cursor := ""
	if m.helpFocus {
		cursor = "▏"
	}
	queryLine := " " + m.styles.Muted.Render("过滤：") +
		m.styles.Input.Width(innerW-8).Render(" "+components.Truncate(m.helpQuery+cursor, innerW-10))
	groups := m.keys.FilterHelp(m.helpQuery)
	blockH := max(1, ph-4)
	block := components.RenderHelpWithQuery(m.styles, groups, m.helpQuery, innerW, blockH, m.helpOffset)
	content := components.FitLines(queryLine+"\n"+block, innerW, max(1, ph-2))
	box := components.Panel(m.styles, "帮助 · Esc 关闭 · "+fmt.Sprint(len(groups))+" 组", content, pw, ph, true)
	return m.styles.App.Render(lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box))
}

// builtinPalettes is computed once at startup; the settings screen must not
// rebuild theme values on every frame.
func builtinPalettes() []theme.Theme {
	names := theme.Names()
	out := make([]theme.Theme, 0, len(names))
	for _, name := range names {
		out = append(out, theme.Builtin(name))
	}
	return out
}

func (m *Model) renderSettings(width, height int) string {
	server, _ := m.cfg.ActiveServer()
	mpv := "未就绪"
	if m.deps.Player != nil && m.deps.Player.Available() {
		mpv = "已连接"
	}
	lastfm := "关闭"
	if m.cfg.LastFM.Enabled {
		lastfm = "启用"
	}
	hits, misses := m.coverCache.Stats()
	label := func(name string) string {
		return "    " + m.styles.Muted.Render(components.PadRight(name, 10))
	}
	value := func(v string) string {
		return m.styles.Text.Render(components.Truncate(v, max(8, width-20)))
	}
	lines := []string{
		"  " + m.styles.GroupHeader.Render("▎ 外观与连接"),
		label("配置文件") + value(m.deps.ConfigPath),
		label("服务器") + value(server.Name+"  "+server.URL),
		label("mpv") + value(mpv),
		label("Last.fm") + value(lastfm),
		"",
		"  " + m.styles.GroupHeader.Render("▎ 主题预览 (Ctrl+T 切换)"),
	}
	if swatches := components.RenderThemeSwatches(m.styles, m.themePalettes, m.styles.T.Name, width-4); swatches != "" {
		for _, line := range strings.Split(swatches, "\n") {
			lines = append(lines, "  "+line)
		}
	}
	lines = append(lines, "",
		"  "+m.styles.GroupHeader.Render("▎ 运行状态"),
		label("封面缓存")+value(fmt.Sprintf("%d 命中 / %d 未命中", hits, misses)),
		label("快捷键")+value(fmt.Sprintf("%d 组 · %d 条绑定 · %d 冲突", len(m.keys.HelpGroups()), len(m.keys.Bindings()), len(m.keys.Conflicts()))),
		label("播放")+value(fmt.Sprintf("%s · 音量 %d%%", m.playerState.Mode.String(), m.playerState.Volume)),
		"",
		"  "+m.styles.GroupHeader.Render("▎ 提示"),
		"    "+m.styles.Chip.Render("Ctrl+T")+" 主题  "+m.styles.Chip.Render("w")+" 服务器  "+m.styles.Chip.Render("F2")+" 返回",
		"    所有配置保存在 "+config.AppName+" 用户配置目录，密码请用环境变量传递以更安全",
	)
	if height < 14 {
		lines = []string{
			"  " + m.styles.GroupHeader.Render("▎ 外观与连接"),
			label("服务器") + value(server.Name),
			label("主题") + value(m.styles.T.Name),
			label("mpv") + value(mpv+" · Last.fm "+lastfm),
			"    Ctrl+T 切换主题 · F2 返回",
		}
	}
	return components.Panel(m.styles, "设置", components.FitLines(strings.Join(lines, "\n"), width-2, height-3), width, height, true)
}
