package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/subsonic-tui/subsonic-tui/internal/keymap"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/ui/components"
)

const doubleClickInterval = 400 * time.Millisecond

// handleMouse consumes every mouse event. Without this branch a terminal that
// reports mouse activity would leak SGR escape sequences into text input.
func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if m.view == model.ViewWizard {
		return m.handleWizardMouse(msg)
	}
	if tea.MouseEvent(msg).IsWheel() {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			return m.move(-3)
		case tea.MouseButtonWheelDown:
			return m.move(3)
		}
		return nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return nil
	}
	if cmd, handled := m.handleProgressBarClick(msg); handled {
		return cmd
	}
	now := time.Now()
	double := now.Sub(m.lastClickAt) <= doubleClickInterval &&
		absInt(msg.X-m.lastClickX) <= 1 && absInt(msg.Y-m.lastClickY) <= 1
	m.lastClickAt, m.lastClickX, m.lastClickY = now, msg.X, msg.Y
	return m.handleMainMouse(msg, double)
}

func (m *Model) handleWizardMouse(msg tea.MouseMsg) tea.Cmd {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return nil
	}
	submit, handled := m.wizard.click(msg, m.width, m.height)
	if !handled {
		return nil
	}
	if submit {
		return m.wizardSubmit()
	}
	return nil
}

func (m *Model) handleMainMouse(msg tea.MouseMsg, double bool) tea.Cmd {
	// Only the three list screens have clickable rows in the menu layout.
	if m.view != model.ViewLibrary && m.view != model.ViewSearch && m.view != model.ViewQueue {
		return nil
	}
	start, mainW := m.menuLayout()
	bodyStart := 1
	if msg.Y < bodyStart || msg.Y >= bodyStart+m.bodyHeight() {
		return nil
	}
	if msg.X < start || msg.X >= start+mainW {
		return nil
	}
	listH := m.listHeight()
	var cfg components.ListConfig
	switch m.view {
	case model.ViewSearch:
		cfg = m.listConfig(m.searchNode, mainW, listH)
	case model.ViewQueue:
		cfg = components.ListConfig{
			Items: m.queueItems(), Selected: m.queueSelected, Width: mainW, Height: listH,
			Double: m.doubleColumnActive(), PlayingID: m.playerState.Song.ID, ShowNumber: true,
		}
	default:
		cfg = m.listConfig(m.currentNode(), mainW, listH)
	}
	rows := components.RowsFor(cfg)
	relRow := msg.Y - bodyStart - 2 // title + blank line
	if relRow < 0 || relRow >= rows {
		return nil
	}
	cols := components.ColumnsFor(cfg)
	col := 0
	if cols == 2 {
		colW := (cfg.Width - 2) / 2
		if msg.X >= start+colW+2 {
			col = 1
		}
	}
	idx := components.ClampOffset(cfg) + relRow*cols + col
	if idx < 0 || idx >= len(cfg.Items) || cfg.Items[idx].Kind == components.ItemSection {
		return nil
	}
	m.setSelection(idx)
	if double {
		return m.activateSelection()
	}
	return nil
}

// handleProgressBarClick maps a click on the bottom progress bar to an
// absolute seek. The row and column geometry is shared with the renderer via
// ProgressBarGeometry, so resizing cannot desynchronise the hit area.
func (m *Model) handleProgressBarClick(msg tea.MouseMsg) (tea.Cmd, bool) {
	nowH := m.nowBarHeight()
	if nowH < 2 {
		return nil, false // compact single-line bar has different geometry
	}
	if msg.Y != m.height-nowH+1 {
		return nil, false
	}
	info := m.nowPlayingInfo()
	startX, barW, ok := components.ProgressBarGeometry(m.styles, info, m.width)
	if !ok || msg.X < startX || msg.X >= startX+barW {
		return nil, false
	}
	sec := int(float64(msg.X-startX) / float64(barW) * float64(info.Duration))
	if sec < 0 {
		sec = 0
	}
	if sec > info.Duration {
		sec = info.Duration
	}
	if m.deps.Player == nil {
		m.setStatus("播放器未初始化", true)
		return nil, true
	}
	// Report success only after the async seek actually succeeds.
	action := keymap.Action("seekTo")
	cmd := func() tea.Msg {
		if err := m.deps.Player.Seek(sec); err != nil {
			return actionResultMsg{action: action, err: err}
		}
		return actionResultMsg{action: action, text: "已跳转到 " + model.FormatDuration(sec)}
	}
	return cmd, true
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
