package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/theme"
)

// RenderLyrics shows a centred, scrolling lyric viewport. For synced lyrics
// the active line is highlighted; for plain lyrics `offset` controls scroll.
// marker controls the "▎" prefix on the active line (used by the lyric page,
// omitted by the immersive screen where colour alone marks the current line).
func RenderLyrics(s *theme.Styles, lyrics model.Lyrics, active, offset, width, height int, marker bool) string {
	if width < 10 || height < 3 {
		return FitLines("", width, max(1, height))
	}
	if lyrics.IsEmpty() {
		return RenderEmptyState(s, EmptyState{
			Glyph:  "╭───────╮\n│ ♪   ♫ │\n╰───────╯",
			Title:  "暂无歌词",
			Detail: "该曲目没有可用的同步歌词或歌词文件",
			Hint:   "y 返回 · Space 播放/暂停",
		}, width, height)
	}

	start := offset
	if lyrics.Synced && active >= 0 {
		start = active - height/2
	}
	maxStart := len(lyrics.Lines) - 1
	if maxStart < 0 {
		maxStart = 0
	}
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		idx := start + i
		if idx >= len(lyrics.Lines) {
			lines = append(lines, strings.Repeat(" ", width))
			continue
		}
		text := Truncate(strings.TrimSpace(lyrics.Lines[idx].Text), width-4)
		var centered string
		switch {
		case idx == active:
			if marker {
				highlight := s.Gradient(text)
				pad := (width - VisualWidth(text)) / 2
				if pad > 0 {
					centered = strings.Repeat(" ", pad-1) + "▎" + highlight
				} else {
					centered = "▎" + highlight
				}
			} else {
				// Colour-only highlight, no leading bar.
				centered = lipgloss.PlaceHorizontal(width, lipgloss.Center,
					s.Primary.Bold(true).Render(text))
			}
		case idx == active-1 || idx == active+1:
			centered = lipgloss.PlaceHorizontal(width, lipgloss.Center, s.Muted.Render(text))
		case idx == active-2 || idx == active+2:
			centered = lipgloss.PlaceHorizontal(width, lipgloss.Center, s.Subtle.Render(text))
		default:
			// Distance-based fade keeps the active line readable without any
			// per-frame animation.
			centered = lipgloss.PlaceHorizontal(width, lipgloss.Center, s.Divider.Render(text))
		}
		lines = append(lines, centered)
	}
	return FitLines(strings.Join(lines, "\n"), width, height)
}

// LyricsOffsetFor computes an unsynced-lyrics scroll offset from the active
// line so the current line stays roughly centred.
func LyricsOffsetFor(active, height int) int {
	off := active - height/2
	if off < 0 {
		return 0
	}
	return off
}
