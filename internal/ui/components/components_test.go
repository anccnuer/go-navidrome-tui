package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/subsonic-tui/subsonic-tui/internal/keymap"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/theme"
)

func testStyles() *theme.Styles { return theme.New(theme.Builtin("aurora"), theme.DepthTrueColor) }

func TestLayoutHelpers(t *testing.T) {
	if got := Truncate("hello world", 6); VisualWidth(got) != 6 {
		t.Fatalf("Truncate = %q width %d", got, VisualWidth(got))
	}
	if got := PadRight("ab", 5); VisualWidth(got) != 5 {
		t.Fatalf("PadRight width = %d", VisualWidth(got))
	}
	block := FitLines("a\nb", 10, 4)
	if lines := strings.Split(block, "\n"); len(lines) != 4 || VisualWidth(lines[0]) != 10 {
		t.Fatalf("FitLines = %q", block)
	}
	if got := FitHeight("a\nb\nc", 2); len(strings.Split(got, "\n")) != 2 {
		t.Fatalf("FitHeight did not clip: %q", got)
	}
}

func TestRenderListViewport(t *testing.T) {
	s := testStyles()
	items := make([]Item, 100)
	for i := range items {
		items[i] = Item{Kind: ItemSong, ID: "s" + string(rune('a'+i%26)), Title: "Track"}
	}
	cfg := ListConfig{Items: items, Selected: 3, Width: 80, Height: 6, Double: true, ShowNumber: true}
	if cols := ColumnsFor(cfg); cols != 2 {
		t.Fatalf("columns = %d", cols)
	}
	if VisibleItems(cfg) != 12 {
		t.Fatalf("visible = %d", VisibleItems(cfg))
	}
	out := RenderList(s, cfg)
	lines := strings.Split(out, "\n")
	if len(lines) != 6 {
		t.Fatalf("rows = %d", len(lines))
	}
	for _, line := range lines {
		if VisualWidth(line) != 80 {
			t.Fatalf("row width = %d", VisualWidth(line))
		}
	}
	if ClampOffset(ListConfig{Items: items, Selected: 99, Width: 80, Height: 6, Double: true}) == 0 {
		t.Fatal("offset should advance for a late selection")
	}
}

func TestFilterItems(t *testing.T) {
	items := []Item{
		{Kind: ItemSection, Title: "歌曲"},
		{Kind: ItemSong, Title: "Hysteria", Subtitle: "Muse"},
		{Kind: ItemSong, Title: "Absolution", Subtitle: "Muse"},
	}
	if got := FilterItems(items, "muse"); len(got) != 2 {
		t.Fatalf("case-insensitive filter returned %d", len(got))
	}
	if got := FilterItems(items, "nope"); len(got) != 0 {
		t.Fatalf("filter returned %d", len(got))
	}
	if got := FilterItems(items, ""); len(got) != 3 {
		t.Fatal("empty query should keep sections")
	}
}

func TestRenderNowPlayingAndStatus(t *testing.T) {
	s := testStyles()
	for _, width := range []int{40, 80, 140} {
		bar := RenderNowPlaying(s, NowPlayingInfo{
			Status: model.StatusPlaying, Title: "Hysteria", Artist: "Muse", Album: "Absolution",
			Position: 42, Duration: 225, Volume: 80, Mode: model.ModeShuffle, Available: true,
		}, width, width < 60)
		for i, line := range strings.Split(bar, "\n") {
			if w := lipgloss.Width(line); w > width+2 {
				t.Fatalf("width %d: now-playing line %d overflows (%d)", width, i, w)
			}
		}
		status := RenderStatusBar(s, StatusInfo{Server: "home", ThemeName: "aurora", ViewName: "艺术家", Connection: "已连接", Hint: "/ 过滤"}, width)
		if w := lipgloss.Width(status); w > width+2 {
			t.Fatalf("width %d: status overflows (%d)", width, w)
		}
	}
	unavailable := RenderNowPlaying(s, NowPlayingInfo{Available: false, Message: "未找到 mpv"}, 80, false)
	if !strings.Contains(unavailable, "mpv") {
		t.Fatal("missing mpv degradation hint")
	}
}

func TestPanelAndLyrics(t *testing.T) {
	s := testStyles()
	panel := Panel(s, "标题", "第一行\n第二行", 30, 8, true)
	if lines := strings.Split(panel, "\n"); len(lines) != 8 {
		t.Fatalf("panel height = %d", len(lines))
	}
	for _, line := range strings.Split(panel, "\n") {
		if w := VisualWidth(line); w != 30 {
			t.Fatalf("panel width = %d", w)
		}
	}
	lyrics := model.Lyrics{
		Synced: true,
		Lines: []model.LyricsLine{
			{Time: 0, Text: "first"}, {Time: 2000, Text: "second"}, {Time: 4000, Text: "third"},
		},
	}
	view := RenderLyrics(s, lyrics, 1, 0, 40, 6)
	if lines := strings.Split(view, "\n"); len(lines) != 6 {
		t.Fatalf("lyrics height = %d", len(lines))
	}
	if !strings.Contains(view, "second") {
		t.Fatal("active lyric line missing")
	}
	empty := RenderLyrics(s, model.Lyrics{}, -1, 0, 40, 4)
	if !strings.Contains(empty, "暂无歌词") {
		t.Fatal("missing empty-lyrics hint")
	}
}

func TestHelpAndSidebar(t *testing.T) {
	s := testStyles()
	keys, _ := keymap.Build(true, nil)
	groups := keys.FilterHelp("播放")
	view := RenderHelp(s, groups, 60, 8, 0)
	if lines := strings.Split(view, "\n"); len(lines) != 8 {
		t.Fatalf("help height = %d", len(lines))
	}
	if HelpLineCount(groups) <= 0 {
		t.Fatal("help line count should be positive")
	}
	side := RenderSidebar(s, DefaultSidebar, 1, 20, 10, "home")
	if lines := strings.Split(side, "\n"); len(lines) != 10 {
		t.Fatalf("sidebar height = %d", len(lines))
	}
	tabs := RenderTabs(s, DefaultSidebar, 0, 40)
	if VisualWidth(tabs) != 40 {
		t.Fatalf("tabs width = %d", VisualWidth(tabs))
	}
}

func TestRenderEmptyStateGeometry(t *testing.T) {
	s := testStyles()
	for _, size := range []struct{ w, h int }{{40, 9}, {24, 5}, {80, 14}} {
		for _, loading := range []bool{false, true} {
			out := RenderEmptyState(s, EmptyState{
				Title: "没有内容", Detail: "试试切换到别的分类", Hint: "按 / 过滤", Loading: loading,
			}, size.w, size.h)
			lines := strings.Split(out, "\n")
			if len(lines) != size.h {
				t.Fatalf("w=%d h=%d loading=%v: rows=%d", size.w, size.h, loading, len(lines))
			}
			for i, line := range lines {
				if w := VisualWidth(line); w != size.w {
					t.Fatalf("w=%d h=%d row %d width=%d", size.w, size.h, i, w)
				}
			}
			if !strings.Contains(out, "没有内容") {
				t.Fatalf("missing title: %q", out)
			}
		}
	}
}

func TestRenderThemeSwatches(t *testing.T) {
	s := testStyles()
	themes := []theme.Theme{
		theme.Builtin("aurora"), theme.Builtin("nord"), theme.Builtin("mono"),
	}
	for _, width := range []int{20, 40, 80} {
		out := RenderThemeSwatches(s, themes, "aurora", width)
		lines := strings.Split(out, "\n")
		if len(lines) != len(themes) {
			t.Fatalf("width %d: rows=%d", width, len(lines))
		}
		for i, line := range lines {
			if w := VisualWidth(line); w != width {
				t.Fatalf("width %d: row %d width=%d", width, i, w)
			}
		}
		if !strings.Contains(lines[0], "aurora") {
			t.Fatalf("active theme name missing: %q", lines[0])
		}
	}
	if RenderThemeSwatches(s, themes, "aurora", 10) != "" {
		t.Fatal("swatches should be hidden on very narrow terminals")
	}
}

func TestStatusBarGracefulDegradation(t *testing.T) {
	s := testStyles()
	info := StatusInfo{
		Server: "demo", ThemeName: "aurora", ViewName: "艺术家",
		Connection: "已连接", Hint: "/ 过滤  ? 帮助",
	}
	for _, width := range []int{20, 30, 40, 60, 100, 140} {
		out := RenderStatusBar(s, info, width)
		if w := VisualWidth(out); w != width {
			t.Fatalf("width %d: got %d", width, w)
		}
		if width >= 40 && !strings.Contains(out, "艺术家") {
			t.Fatalf("width %d lost the view label: %q", width, out)
		}
		if width >= 30 && !strings.Contains(out, "●") {
			t.Fatalf("width %d lost the connection indicator: %q", width, out)
		}
	}
}

func TestCentredRootMenuRows(t *testing.T) {
	s := testStyles()
	items := []Item{
		{Kind: ItemNav, Title: "全部歌曲"},
		{Kind: ItemNav, Title: "歌单"},
	}
	view := RenderList(s, ListConfig{
		Items: items, Width: 60, Height: 3, Center: true, Selected: 0,
	})
	lines := strings.Split(view, "\n")
	if len(lines) != 3 {
		t.Fatalf("rows = %d", len(lines))
	}
	first := lines[0]
	if w := VisualWidth(first); w != 60 {
		t.Fatalf("row width = %d", w)
	}
	leading := VisualWidth(first) - VisualWidth(strings.TrimLeft(first, " "))
	if leading < 8 {
		t.Fatalf("selected root row is not centred (leading %d): %q", leading, first)
	}
	second := lines[1]
	leading2 := VisualWidth(second) - VisualWidth(strings.TrimLeft(second, " "))
	if leading2 < 8 {
		t.Fatalf("plain root row is not centred (leading %d): %q", leading2, second)
	}
}

func TestDefaultSidebarIsSongsAndPlaylists(t *testing.T) {
	want := []model.NodeKind{model.NodeAllSongs, model.NodePlaylists}
	if len(DefaultSidebar) != len(want) {
		t.Fatalf("sidebar has %d entries, want %d", len(DefaultSidebar), len(want))
	}
	for i, kind := range want {
		if DefaultSidebar[i].Kind != kind {
			t.Fatalf("entry %d = %v, want %v", i, DefaultSidebar[i].Kind, kind)
		}
	}
}

func TestHelpRowsStayWithinWidth(t *testing.T) {
	s := testStyles()
	keys, _ := keymap.Build(true, nil)
	for _, width := range []int{20, 32, 48, 80} {
		out := RenderHelpWithQuery(s, keys.HelpGroups(), "播放", width, 8, 0)
		lines := strings.Split(out, "\n")
		if len(lines) != 8 {
			t.Fatalf("width %d: rows = %d", width, len(lines))
		}
		for i, line := range lines {
			if w := VisualWidth(line); w > width {
				t.Fatalf("width %d: row %d overflows (%d)", width, i, w)
			}
		}
	}
}

func TestUnsyncedLyricsHonoursOffset(t *testing.T) {
	s := testStyles()
	lines := make([]model.LyricsLine, 40)
	for i := range lines {
		lines[i] = model.LyricsLine{Text: fmt.Sprintf("line-%02d", i)}
	}
	plain := model.Lyrics{Lines: lines, Synced: false}
	first := RenderLyrics(s, plain, 0, 0, 40, 5)
	later := RenderLyrics(s, plain, 30, 28, 40, 5)
	if first == later {
		t.Fatal("offset was ignored for plain lyrics")
	}
	if !strings.Contains(later, "line-30") {
		t.Fatalf("active line missing: %q", later)
	}
	far := RenderLyrics(s, plain, 39, 1000, 40, 5)
	if !strings.Contains(far, "line-39") {
		t.Fatalf("start offset was not clamped: %q", far)
	}
}
