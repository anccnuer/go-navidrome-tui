package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/player"
	"github.com/subsonic-tui/subsonic-tui/internal/ui/components"
)

type stubSeekPlayer struct {
	player.Player
	seekTo int
}

func (s *stubSeekPlayer) Available() bool    { return true }
func (s *stubSeekPlayer) Seek(sec int) error { s.seekTo = sec; return nil }

func TestProgressBarClickSeek(t *testing.T) {
	m := testModel(t, true)
	stub := &stubSeekPlayer{}
	m.deps.Player = stub
	m.playerState = player.PlayerState{
		Status:   model.StatusPlaying,
		Song:     model.Song{ID: "s1", Title: "Track", Duration: 200},
		Position: 40, Duration: 200, Volume: 80, Mode: model.ModeSequential,
	}
	startX, barW, ok := components.ProgressBarGeometry(m.styles, m.nowPlayingInfo(), m.width)
	if !ok || barW < 8 {
		t.Fatalf("geometry: start=%d width=%d ok=%v", startX, barW, ok)
	}
	progressRow := m.height - 2 + 1
	cmd, handled := m.handleProgressBarClick(tea.MouseMsg{
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
		X: startX + barW/2, Y: progressRow,
	})
	if !handled || cmd == nil {
		t.Fatal("clicking the middle of the progress bar should dispatch a seek")
	}
	m.Update(cmd())
	if absInt(stub.seekTo-100) > 12 {
		t.Fatalf("seek = %d, expected roughly the midpoint (100s)", stub.seekTo)
	}
	if !strings.Contains(m.statusLine(), "已跳转到") {
		t.Fatalf("missing seek feedback: %q", m.statusLine())
	}
	for _, miss := range []tea.MouseMsg{
		// The bar starts at column 0 now, so a hit is only invalid past its
		// right edge or on a different row.
		{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: startX, Y: progressRow - 1},
		{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: startX + barW, Y: progressRow},
	} {
		if _, hit := m.handleProgressBarClick(miss); hit {
			t.Fatalf("unexpected progress-bar hit at x=%d y=%d", miss.X, miss.Y)
		}
	}
}

func TestImmersiveCoverUnaffectedByRightRail(t *testing.T) {
	m := testModel(t, true)
	m.cfg.UI.ShowCover = true
	// 80 columns: layoutSizes hides the right cover rail entirely.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m.Update(keyMsg("z"))
	if m.view != model.ViewNowPlaying {
		t.Fatalf("view = %v", m.view)
	}
	if m.coverArt == "" {
		t.Fatal("immersive view must still render artwork when the right rail is hidden")
	}
}

func TestNowPlayingViewToggle(t *testing.T) {
	m := testModel(t, true)
	m.stack = []*Node{sampleNode()}
	m.Update(keyMsg("z"))
	if m.view != model.ViewNowPlaying {
		t.Fatalf("z should open the immersive view, got %v", m.view)
	}
	if view := m.View(); !strings.Contains(view, "正在播放") {
		t.Fatal("immersive view is missing its title")
	}
	m.Update(keyMsg("z"))
	if m.view != model.ViewLibrary {
		t.Fatalf("pressing z again should return, got %v", m.view)
	}
}
