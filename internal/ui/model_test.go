package ui

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/subsonic-tui/subsonic-tui/internal/config"
	"github.com/subsonic-tui/subsonic-tui/internal/keymap"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/subsonic"
)

func testModel(t *testing.T, withServer bool) *Model {
	t.Helper()
	cfg := config.Default()
	if withServer {
		cfg.Servers = []config.Server{{Name: "home", URL: "http://localhost:4533", Username: "u", Password: "p"}}
		cfg.Active = "home"
	}
	keys, _ := keymap.Build(true, nil)
	deps := &Deps{
		Config:     cfg,
		ConfigPath: "config.toml",
		Keys:       keys,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verify:     func(config.Server) (subsonic.Client, error) { return nil, nil },
		Commit:     func(config.Server, subsonic.Client) error { return nil },
	}
	m := New(deps)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func sampleNode() *Node {
	return &Node{
		Kind: model.NodeAllSongs, Title: "全部歌曲",
		Items: []Item{
			{Kind: ItemArtist, ID: "a1", Title: "Alpha"},
			{Kind: ItemArtist, ID: "a2", Title: "Beta"},
			{Kind: ItemArtist, ID: "a3", Title: "Gamma"},
			{Kind: ItemArtist, ID: "a4", Title: "Delta"},
		},
	}
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestInitialViewAndWizard(t *testing.T) {
	withServer := testModel(t, true)
	if withServer.view != model.ViewLibrary {
		t.Fatalf("with a server we should start in the library, got %v", withServer.view)
	}
	without := testModel(t, false)
	if without.view != model.ViewWizard {
		t.Fatalf("without a server the wizard should be shown, got %v", without.view)
	}
	if view := without.View(); !strings.Contains(view, "连接向导") {
		t.Fatalf("wizard view missing title: %q", view)
	}
}

func TestResponsiveLayout(t *testing.T) {
	for _, size := range []struct{ w, h int }{{140, 44}, {90, 30}, {50, 20}, {30, 10}} {
		m := testModel(t, true)
		m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		view := m.View()
		if got := lipgloss.Height(view); got != size.h {
			t.Errorf("%dx%d: height = %d", size.w, size.h, got)
		}
		for i, line := range strings.Split(view, "\n") {
			if w := lipgloss.Width(line); w > size.w+2 {
				t.Errorf("%dx%d: line %d overflows: %d cols", size.w, size.h, i, w)
			}
		}
	}
	wide := testModel(t, true)
	start, width := wide.menuLayout()
	if start <= 0 || width <= 0 || start+width > wide.width {
		t.Fatalf("wide menu layout = start %d width %d (window %d)", start, width, wide.width)
	}
	if start != (wide.width-width)/2 {
		t.Fatalf("menu block is not centred: start %d width %d (window %d)", start, width, wide.width)
	}
	if !wide.doubleColumnActive() {
		t.Fatal("wide layout should enable the dual column")
	}
	narrow := testModel(t, true)
	narrow.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	if narrow.doubleColumnActive() {
		t.Fatal("narrow layout should stay single column")
	}
	if _, width := narrow.menuLayout(); width >= narrow.width {
		t.Fatalf("menu should be indented: width %d of %d", width, narrow.width)
	}
}

func TestRootMenuIsCentred(t *testing.T) {
	m := testModel(t, true)
	m.stack = []*Node{{
		Kind: model.NodeRoot, Title: "主页",
		Items: []Item{
			{Kind: ItemNav, Title: "全部歌曲", Target: model.NodeAllSongs},
			{Kind: ItemNav, Title: "歌单", Target: model.NodePlaylists},
		},
	}}
	view := m.View()
	t.Log("\n" + view)
	if !strings.Contains(view, "歌单") || !strings.Contains(view, "全部歌曲") {
		t.Fatal("root menu entries are missing")
	}
	for i, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > m.width {
			t.Fatalf("row %d overflows: %d > %d", i, w, m.width)
		}
	}
}

func TestNowPlayingArtworkLeftLyricsRight(t *testing.T) {
	m := testModel(t, true)
	m.view = model.ViewNowPlaying
	m.playerState.Song = model.Song{ID: "s1", Title: "My Song", Artist: "Someone"}
	lines := make([]model.LyricsLine, 30)
	for i := range lines {
		lines[i] = model.LyricsLine{Text: fmt.Sprintf("lyric-%02d", i)}
	}
	m.lyrics = model.Lyrics{Lines: lines, Synced: false}
	m.lyricsOffset = 12
	m.coverArt = "ART-TOP\nART-MID\nART-END"

	view := m.View()
	t.Log("\n" + view)
	leftPad, artW, _, artH, gap := m.immersiveLayout()
	if artH < 4 {
		t.Fatalf("artwork height = %d, too small", artH)
	}
	if leftPad < 4 {
		t.Fatalf("left margin = %d, too small", leftPad)
	}
	lyricStart := 1 + leftPad + artW + gap // left border + art column + gap
	artRow, titleRow, lyricRows := -1, -1, 0
	for i, line := range strings.Split(view, "\n") {
		if idx := strings.Index(line, "ART-"); idx >= 0 {
			artRow = i
			col := lipgloss.Width(line[:idx])
			if col < 1+leftPad-1 {
				t.Fatalf("artwork too close to the left border: col %d", col)
			}
			if col >= lyricStart {
				t.Fatalf("artwork should be on the left: col %d", col)
			}
		}
		// The bottom bar repeats the title; only the first occurrence belongs
		// to the immersive panel (directly under the artwork).
		if titleRow < 0 && strings.Contains(line, "My Song") {
			titleRow = i
		}
		if idx := strings.Index(line, "lyric-"); idx >= 0 {
			lyricRows++
			col := lipgloss.Width(line[:idx])
			if col < lyricStart-2 {
				t.Fatalf("lyrics should be on the right: col %d", col)
			}
		}
		if strings.Contains(line, "▎") {
			t.Fatal("active lyric must be colour-only, without a leading bar")
		}
	}
	if artRow < 0 {
		t.Fatal("album art is missing from the immersive view")
	}
	if titleRow < 0 {
		t.Fatal("track title is missing")
	}
	if titleRow <= artRow {
		t.Fatal("track title must be displayed below the artwork")
	}
	if lyricRows < 8 {
		t.Fatalf("only %d lyric rows are visible; expected a tall scrolling viewport", lyricRows)
	}
}

func TestMenuLayoutRendersMusicfoxStyle(t *testing.T) {
	m := testModel(t, true)
	m.stack = []*Node{sampleNode()}
	view := m.View()
	t.Log("\n" + view)
	for _, want := range []string{"=>", "Alpha", "↑↓/jk", "subsonic-tui · ["} {
		if !strings.Contains(view, want) {
			t.Fatalf("view is missing %q\n%s", want, view)
		}
	}
	if strings.ContainsAny(view, "▐▌") {
		t.Fatal("selected rows must be colour-only, without half-block caps")
	}
}

// TestDualColumnNavigation checks the row-wise grid (1 2 / 3 4 / 5 6):
// up/down stays in one column, left/right switches column within a row.
func TestDualColumnNavigation(t *testing.T) {
	makeItems := func() []Item {
		items := make([]Item, 0, 12)
		for i := 0; i < 12; i++ {
			items = append(items, songItem(model.Song{
				ID: fmt.Sprintf("s%d", i), Title: fmt.Sprintf("Track %d", i),
			}))
		}
		return items
	}

	m := testModel(t, true)
	m.stack = []*Node{{Kind: model.NodeAllSongs, Title: "全部歌曲", Items: makeItems()}}
	if got := m.gridColumns(); got != 2 {
		t.Fatalf("grid columns = %d, want 2", got)
	}
	m.setSelection(0)
	for _, want := range []int{2, 4} { // 1 -> 3 -> 5
		m.Update(keyMsg("j"))
		if got := m.selection(); got != want {
			t.Fatalf("after j: %d, want %d", got, want)
		}
	}
	m.Update(keyMsg("l")) // 5 -> 6
	if got := m.selection(); got != 5 {
		t.Fatalf("after l: %d, want 5", got)
	}
	m.Update(keyMsg("h")) // 6 -> 5
	if got := m.selection(); got != 4 {
		t.Fatalf("after h: %d, want 4", got)
	}
	m.Update(keyMsg("k")) // 5 -> 3
	if got := m.selection(); got != 2 {
		t.Fatalf("after k: %d, want 2", got)
	}
	m.Update(keyMsg("l")) // 3 -> 4 (right column)
	m.Update(keyMsg("j")) // 4 -> 6
	if got := m.selection(); got != 5 {
		t.Fatalf("right-column down: %d, want 5", got)
	}

	// Column boundaries must not wrap into the previous/next row.
	m.setSelection(0)
	m.Update(keyMsg("h"))
	if got := m.selection(); got != 0 {
		t.Fatalf("left edge wrapped to %d", got)
	}
	m.setSelection(1)
	m.Update(keyMsg("l"))
	if got := m.selection(); got != 1 {
		t.Fatalf("right edge wrapped to %d", got)
	}

	// Single column keeps the plain ±1 vertical step.
	narrow := testModel(t, true)
	narrow.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	narrow.stack = []*Node{{Kind: model.NodeAllSongs, Title: "全部歌曲", Items: makeItems()}}
	if got := narrow.gridColumns(); got != 1 {
		t.Fatalf("narrow grid columns = %d", got)
	}
	narrow.setSelection(0)
	narrow.Update(keyMsg("j"))
	if got := narrow.selection(); got != 1 {
		t.Fatalf("single-column j: %d, want 1", got)
	}
}

func TestNavigationKeys(t *testing.T) {
	m := testModel(t, true)
	// Keep this test in single-column mode so it covers the plain ±1 step;
	// dual-column movement is covered by TestDualColumnNavigation.
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	m.stack = []*Node{sampleNode()}
	m.Update(keyMsg("j"))
	if got := m.currentNode().Selected; got != 1 {
		t.Fatalf("after j selected = %d", got)
	}
	m.Update(keyMsg("k"))
	if got := m.currentNode().Selected; got != 0 {
		t.Fatalf("after k selected = %d", got)
	}
	m.Update(keyMsg("G"))
	if got := m.currentNode().Selected; got != 3 {
		t.Fatalf("after G selected = %d", got)
	}
	m.Update(keyMsg("g"))
	if got := m.currentNode().Selected; got != 0 {
		t.Fatalf("after g selected = %d", got)
	}
}

func TestViewSwitchingAndBack(t *testing.T) {
	m := testModel(t, true)
	m.stack = []*Node{sampleNode()}
	for key, want := range map[string]model.ViewKind{
		"?": model.ViewHelp,
		"c": model.ViewQueue,
		"y": model.ViewLyrics,
	} {
		m.Update(keyMsg(key))
		if m.view != want {
			t.Fatalf("key %q -> view %v, want %v", key, m.view, want)
		}
		m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if m.view != model.ViewLibrary {
			t.Fatalf("Esc did not return to library: %v", m.view)
		}
	}
}

func TestFilterInput(t *testing.T) {
	m := testModel(t, true)
	m.stack = []*Node{sampleNode()}
	m.Update(keyMsg("/"))
	if !m.filterActive {
		t.Fatal("/ should activate inline filtering")
	}
	m.Update(keyMsg("b"))
	m.Update(keyMsg("e"))
	if node := m.currentNode(); node.Filter != "be" || len(node.Rows()) != 1 {
		t.Fatalf("filter = %q, rows = %d", node.Filter, len(node.Rows()))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.filterActive || m.currentNode().Filter != "" {
		t.Fatal("Esc should clear the filter")
	}
}

func TestStaleNodeResponseIsIgnored(t *testing.T) {
	m := testModel(t, true)
	node := sampleNode()
	node.ReqID = 7
	m.stack = []*Node{node}
	m.Update(nodeLoadedMsg{req: 6, items: []Item{{Kind: ItemArtist, Title: "stale"}}})
	if len(node.Items) != 4 || node.Loading {
		t.Fatal("stale response mutated the node")
	}
	m.Update(nodeLoadedMsg{req: 7, items: []Item{{Kind: ItemArtist, Title: "fresh"}}})
	if len(node.Items) != 1 || node.Items[0].Title != "fresh" {
		t.Fatalf("matching response was not applied: %+v", node.Items)
	}
}

func TestWizardSubmitFlow(t *testing.T) {
	m := testModel(t, false)
	typeText := func(text string) {
		for _, r := range text {
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	// On a fresh install the wizard focuses the first empty field (address),
	// so the URL can be pasted immediately.
	typeText("http://localhost:4533")
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	typeText("alice")
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	typeText("secret")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a save command")
	}
	msg := cmd()
	result, ok := msg.(wizardResultMsg)
	if !ok || result.err != nil {
		t.Fatalf("wizard result = %#v", msg)
	}
	m.Update(result)
	if m.view != model.ViewLibrary {
		t.Fatalf("after a successful login we should see the library, got %v", m.view)
	}
	if server := result.server; server.URL != "http://localhost:4533" || server.Username != "alice" || server.PlainPassword() != "secret" {
		t.Fatalf("server = %+v", server)
	}
}

func TestHelpFilterGeneratesGroups(t *testing.T) {
	m := testModel(t, true)
	groups := m.keys.FilterHelp("播放")
	if len(groups) == 0 {
		t.Fatal("expected at least one help group")
	}
	view := m.View()
	if view == "" {
		t.Fatal("empty view")
	}
}

func TestViewToggleAndHelpInput(t *testing.T) {
	m := testModel(t, true)
	m.stack = []*Node{sampleNode()}

	m.Update(keyMsg("c"))
	if m.view != model.ViewQueue {
		t.Fatalf("c should open the queue, got %v", m.view)
	}
	m.Update(keyMsg("c"))
	if m.view != model.ViewLibrary {
		t.Fatalf("pressing c again should return, got %v", m.view)
	}

	m.Update(keyMsg("?"))
	if m.view != model.ViewHelp || !m.helpFocus {
		t.Fatalf("help view should be focused for filtering: view=%v focus=%v", m.view, m.helpFocus)
	}
	for _, r := range "nex" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.helpQuery != "nex" {
		t.Fatalf("help query = %q", m.helpQuery)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.view != model.ViewLibrary || m.helpFocus {
		t.Fatalf("Esc should leave the help view: view=%v focus=%v", m.view, m.helpFocus)
	}
}

func TestPlainLyricsScrolling(t *testing.T) {
	m := testModel(t, true)
	m.view = model.ViewLyrics
	lines := make([]model.LyricsLine, 30)
	for i := range lines {
		lines[i] = model.LyricsLine{Text: fmt.Sprintf("line-%02d", i)}
	}
	m.lyrics = model.Lyrics{Lines: lines, Synced: false}

	m.Update(keyMsg("j"))
	if m.lyricsOffset != 1 {
		t.Fatalf("j should scroll plain lyrics, offset = %d", m.lyricsOffset)
	}
	m.Update(keyMsg("G"))
	if m.lyricsOffset != 29 {
		t.Fatalf("G should jump to the last line, offset = %d", m.lyricsOffset)
	}
	if view := m.View(); !strings.Contains(view, "line-29") {
		t.Fatal("the last lyric line should be visible after scrolling")
	}
}
