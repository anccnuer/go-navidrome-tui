// Package ui implements the Bubble Tea root model and all screens.
package ui

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/subsonic-tui/subsonic-tui/internal/cache"
	"github.com/subsonic-tui/subsonic-tui/internal/config"
	"github.com/subsonic-tui/subsonic-tui/internal/cover"
	"github.com/subsonic-tui/subsonic-tui/internal/keymap"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/player"
	"github.com/subsonic-tui/subsonic-tui/internal/theme"
)

// Model is the root tea.Model.
type Model struct {
	deps   *Deps
	cfg    *config.Config
	styles *theme.Styles
	keys   *keymap.Resolver
	depth  theme.ColorDepth
	log    *slog.Logger

	coverCache *cache.Cache[string]

	width, height int
	view          model.ViewKind
	prevView      model.ViewKind
	stack         []*Node
	reqSeq        int

	wizard       wizardModel
	wizardForced bool

	searchInput   string
	searchQuery   string
	searchNode    *Node
	searchReq     int
	searchLoading bool
	searchFocus   bool

	filterInput  string
	filterActive bool

	helpQuery  string
	helpOffset int
	helpFocus  bool

	queueSelected int

	playlistAddTarget string

	lyrics        model.Lyrics
	lyricsSongID  string
	lyricsLoading bool
	lyricsOffset  int

	playerState player.PlayerState

	coverArt string
	coverKey string

	statusWarn bool
	toasts     []toast
	mpvMessage string
	connection string
	connWarn   bool

	scrobbleStarted time.Time
	scrobbleSent    bool

	lastClickAt time.Time
	lastClickX  int
	lastClickY  int

	themePalettes []theme.Theme
	spinnerFrame  int
}

// New wires the root model. The player may be nil in unit tests.
func New(deps *Deps) *Model {
	if deps == nil {
		panic("ui.New requires dependencies")
	}
	if deps.Config == nil {
		deps.Config = config.Default()
	}
	if deps.Logger == nil {
		deps.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	if deps.Keys == nil {
		deps.Keys, _ = keymap.Build(true, nil)
	}
	palette, err := theme.Resolve(deps.Config.Theme.Name, deps.Config.Theme.CustomPath)
	if err != nil {
		deps.Logger.Warn("自定义主题加载失败，使用内置主题", "err", err)
	}
	depth := theme.DetectDepth(deps.Config.UI.TrueColor)
	m := &Model{
		deps:          deps,
		cfg:           deps.Config,
		styles:        theme.New(palette, depth),
		keys:          deps.Keys,
		depth:         depth,
		log:           deps.Logger,
		coverCache:    cache.New[string](max(8, deps.Config.Cache.CoverCacheSize), deps.Config.Cache.TTLDuration()),
		searchNode:    &Node{Kind: model.NodeSearch, Title: "搜索结果"},
		connection:    "连接中",
		themePalettes: builtinPalettes(),
	}
	server, hasServer := deps.Config.ActiveServer()
	m.wizard = newWizard(server)
	if !hasServer {
		m.view = model.ViewWizard
	} else {
		m.view = model.ViewLibrary
	}
	if deps.Player != nil {
		m.playerState = deps.Player.State()
	}
	return m
}

// Init starts the background event loops and the first data load.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	if m.deps.Player != nil {
		cmds = append(cmds, playerEventCmd(m.deps.Player.Events()))
	}
	if m.view == model.ViewWizard {
		return tea.Batch(cmds...)
	}
	// go-musicfox style navigation starts from a root menu instead of a
	// persistent sidebar.
	m.stack = nil
	cmds = append(cmds, m.pingCmd(), m.pushNode(model.NodeRoot, "", "主页"))
	return tea.Batch(cmds...)
}

func (m *Model) nextReq() int {
	m.reqSeq++
	return m.reqSeq
}

// Update is the single entry point for all state transitions.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, m.refreshCover()

	case tea.KeyMsg:
		return m, m.handleKey(msg)

	case tea.MouseMsg:
		return m, m.handleMouse(msg)

	case nodeLoadedMsg:
		return m, m.onNodeLoaded(msg)

	case searchLoadedMsg:
		return m, m.onSearchLoaded(msg)

	case coverLoadedMsg:
		if msg.key != m.coverKey {
			return m, nil
		}
		m.coverArt = msg.art
		if msg.err != nil {
			m.log.Debug("封面加载失败，已显示占位图", "err", msg.err)
		}
		return m, nil

	case lyricsLoadedMsg:
		if msg.songID != m.playerState.Song.ID {
			return m, nil
		}
		m.lyricsLoading = false
		if msg.err != nil {
			m.lyrics = model.Lyrics{}
			m.log.Debug("歌词加载失败", "err", msg.err)
		} else {
			m.lyrics = msg.lyrics
		}
		return m, nil

	case playerEventMsg:
		if msg.closed {
			m.setStatus("mpv 事件通道已关闭", true)
			return m, nil
		}
		cmd := m.onPlayerEvent(msg.event)
		cmds := []tea.Cmd{cmd}
		if m.deps.Player != nil {
			cmds = append(cmds, playerEventCmd(m.deps.Player.Events()))
		}
		return m, tea.Batch(cmds...)

	case tickMsg:
		m.spinnerFrame++
		return m, m.onTick()

	case actionResultMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("%s 失败：%v", msg.action, msg.err), true)
		} else if msg.text != "" {
			m.setStatusOK(msg.text)
		}
		return m, nil

	case wizardResultMsg:
		return m, m.onWizardResult(msg)

	case pingResultMsg:
		if msg.err != nil {
			m.connection = "连接失败"
			m.connWarn = true
			m.setStatus("无法连接 Navidrome："+msg.err.Error(), true)
		} else {
			m.connection = "已连接"
			m.connWarn = false
			if info := m.deps.API(); info != nil {
				if si := info.ServerInfo(); si.Type != "" {
					m.connection = si.Type + " · 已连接"
				}
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) onNodeLoaded(msg nodeLoadedMsg) tea.Cmd {
	node := m.currentNode()
	if node == nil || node.ReqID != msg.req {
		return nil // stale response from an older navigation
	}
	node.Loading = false
	if msg.err != nil {
		node.Err = msg.err
		m.setStatus(msg.err.Error(), true)
		return nil
	}
	if msg.append {
		node.Items = append(node.Items, msg.items...)
	} else {
		node.Items = msg.items
		node.Selected = 0
	}
	node.HasMore = msg.hasMore
	node.refreshFilter()
	m.log.Debug("节点加载完成", "kind", node.Kind, "count", len(node.Items))
	return nil
}

func (m *Model) onSearchLoaded(msg searchLoadedMsg) tea.Cmd {
	if msg.req != m.searchReq {
		return nil
	}
	m.searchLoading = false
	if msg.err != nil {
		m.setStatus("搜索失败："+msg.err.Error(), true)
		return nil
	}
	m.searchNode.Items = msg.items
	m.searchNode.Selected = 0
	m.searchNode.refreshFilter()
	return nil
}

func (m *Model) onWizardResult(msg wizardResultMsg) tea.Cmd {
	m.wizard.busy = false
	if msg.err != nil {
		m.wizard.err = msg.err.Error()
		m.wizard.note = "连接失败，请检查地址、账号与网络后重试"
		return nil
	}
	// Verify already pinged the server on the background goroutine; applying
	// the profile here keeps all Config writes on the UI goroutine.
	if m.deps.Commit != nil {
		if err := m.deps.Commit(msg.server, msg.client); err != nil {
			m.wizard.err = err.Error()
			m.wizard.note = "保存配置失败，请检查配置目录权限后重试"
			return nil
		}
	} else if msg.client != nil {
		m.deps.SetClient(msg.client)
	}
	m.wizard.err = ""
	m.connection = "已连接"
	m.connWarn = false
	m.wizardForced = false
	m.view = model.ViewLibrary
	m.stack = nil
	return m.pushNode(model.NodeRoot, "", "主页")
}

func (m *Model) onPlayerEvent(event player.Event) tea.Cmd {
	if m.deps.Player != nil {
		m.playerState = m.deps.Player.State()
	}
	switch event.Kind {
	case player.EventReady:
		m.mpvMessage = ""
		m.setStatusOK("mpv 已就绪")
	case player.EventUnavailable:
		m.mpvMessage = event.Text
		m.connWarn = true
		m.setStatus("mpv 不可用，播放功能被禁用", true)
	case player.EventError:
		if event.Text != "" {
			m.setStatus(event.Text, true)
		}
	case player.EventFileLoaded, player.EventQueueChanged:
		return m.onSongMaybeChanged()
	case player.EventEndFile:
		// Last.fm allows submitting when a track finishes; this is also the
		// configured "scrobbleAt = 0" behaviour.
		m.submitScrobble()
	}
	return nil
}

func (m *Model) onSongMaybeChanged() tea.Cmd {
	song := m.playerState.Song
	if song.ID == "" || song.ID == m.lyricsSongID {
		return nil
	}
	m.lyricsSongID = song.ID
	m.lyrics = model.Lyrics{}
	m.lyricsLoading = true
	m.scrobbleStarted = time.Now()
	m.scrobbleSent = false
	cmds := []tea.Cmd{m.lyricsCmd(song.ID), m.refreshCover()}
	if m.deps.Scrobbler != nil {
		m.deps.Scrobbler.NowPlaying(song)
	}
	return tea.Batch(cmds...)
}

func (m *Model) onTick() tea.Cmd {
	m.pruneToasts(time.Now())
	if !m.scrobbleSent && m.deps.Scrobbler != nil && m.playerState.Song.ID != "" &&
		m.deps.Scrobbler.ShouldScrobble(m.playerState.Song, m.playerState.Position) {
		m.submitScrobble()
	}
	return tickCmd()
}

// submitScrobble reports a play at most once per track.
func (m *Model) submitScrobble() {
	if m.scrobbleSent || m.deps.Scrobbler == nil || m.playerState.Song.ID == "" {
		return
	}
	m.scrobbleSent = true
	m.deps.Scrobbler.Submitted(m.playerState.Song, m.scrobbleStarted)
}

func (m *Model) refreshCover() tea.Cmd {
	// Artwork is only shown on the immersive screen now that the browser uses
	// go-musicfox's borderless menu layout.
	if !m.cfg.UI.ShowCover || m.view != model.ViewNowPlaying {
		m.coverArt = ""
		return nil
	}
	w := min(40, max(16, m.width/4))
	h := max(5, min(w/2, max(6, m.bodyHeight()-9)))
	if w < 6 || h < 3 {
		m.coverArt = ""
		return nil
	}
	song := m.playerState.Song
	if song.ID == "" {
		m.coverArt = cover.RenderPlaceholder("idle", w, h, m.depth, m.styles.T.Gradient)
		m.coverKey = ""
		return nil
	}
	return m.coverCmd(song, w, h)
}

// ---- Key handling ---------------------------------------------------------

// wizardSubmit is the shared submit path for the Enter key and the on-screen
// button. Validation runs on the UI goroutine before any async command starts.
func (m *Model) wizardSubmit() tea.Cmd {
	if !m.wizard.submit() {
		return nil
	}
	return m.wizardSaveCmd(m.wizard.server())
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if key == "ctrl+c" {
		return tea.Quit
	}
	if m.view == model.ViewWizard {
		// In a form every printable character belongs to the field; Ctrl+C is
		// the only way to leave the wizard on first run.
		if msg.Type == tea.KeyEsc && m.wizardForced && !m.wizard.busy {
			m.view = model.ViewLibrary
			m.wizardForced = false
			return nil
		}
		if m.wizard.update(msg) {
			return m.wizardSaveCmd(m.wizard.server())
		}
		return nil
	}
	if m.filterActive {
		return m.handleFilterKey(msg)
	}
	if m.view == model.ViewSearch && m.searchFocus {
		return m.handleSearchKey(msg)
	}
	if m.view == model.ViewHelp && m.helpFocus {
		return m.handleHelpKey(msg)
	}
	action, ok := m.keys.Resolve(keymap.KeyCandidates(key)...)
	if !ok {
		return nil
	}
	return m.runAction(action)
}

func (m *Model) handleFilterKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.filterActive = false
		if msg.Type == tea.KeyEsc {
			m.filterInput = ""
			if node := m.currentNode(); node != nil {
				node.Filter = ""
				node.refreshFilter()
			}
		}
		return nil
	case tea.KeyBackspace, tea.KeyDelete:
		runes := []rune(m.filterInput)
		if len(runes) > 0 {
			m.filterInput = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		m.filterInput += cleanInput(string(msg.Runes))
	}
	if node := m.currentNode(); node != nil {
		node.Filter = m.filterInput
		node.refreshFilter()
	}
	return nil
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.searchFocus = false
		m.view = m.prevView
		return nil
	case tea.KeyEnter:
		m.searchFocus = false
		m.searchQuery = m.searchInput
		if m.searchQuery == "" {
			return nil
		}
		return m.searchCmd(m.searchQuery)
	case tea.KeyBackspace, tea.KeyDelete:
		runes := []rune(m.searchInput)
		if len(runes) > 0 {
			m.searchInput = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		m.searchInput += cleanInput(string(msg.Runes))
	default:
		return nil
	}
	return nil
}

func (m *Model) handleHelpKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.helpFocus = false
		m.view = m.prevView
		if m.view == model.ViewWizard {
			m.view = model.ViewLibrary
		}
		return nil
	case tea.KeyEnter:
		m.helpFocus = false
		return nil
	case tea.KeyBackspace, tea.KeyDelete:
		runes := []rune(m.helpQuery)
		if len(runes) > 0 {
			m.helpQuery = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		m.helpQuery += cleanInput(string(msg.Runes))
	}
	m.helpOffset = 0
	return nil
}
