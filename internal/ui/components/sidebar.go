package components

import (
	"fmt"
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/model"
	"github.com/anccnuer/go-navidrome-tui/internal/theme"
)

// SidebarEntry is a navigation node.
type SidebarEntry struct {
	Kind  model.NodeKind
	Title string
	Icon  string
}

// DefaultSidebar is the top-level navigation. This build intentionally keeps
// only the flat song list and playlists: albums, artists and genres are not
// part of the browsing model.
var DefaultSidebar = []SidebarEntry{
	{Kind: model.NodeAllSongs, Title: "全部歌曲", Icon: "♪"},
	{Kind: model.NodePlaylists, Title: "歌单", Icon: "≡"},
}

// RenderSidebar draws the vertical navigation rail.
func RenderSidebar(s *theme.Styles, entries []SidebarEntry, selected, width, height int, serverName string) string {
	if width < 8 || height < 3 {
		return ""
	}
	lines := make([]string, 0, height)
	lines = append(lines, s.Gradient("  ◈  NAVIDROME"))
	lines = append(lines, s.Divider.Render(strings.Repeat("─", width-2)))
	lines = append(lines, s.Subtle.Render("  音乐库"))
	for i, entry := range entries {
		label := "  " + entry.Icon + " " + entry.Title
		line := PadRight(label, width)
		if i == selected {
			line = s.Selected.Width(width).Render("▎" + entry.Icon + " " + entry.Title)
		} else {
			line = s.Muted.Width(width).Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, s.Divider.Render(strings.Repeat("─", width-2)))
	if serverName != "" {
		lines = append(lines, s.Subtle.Render("  "+Truncate("服务器 · "+serverName, width-2)))
	}
	lines = append(lines, s.Subtle.Render("  ? 帮助   q 退出"))
	return FitLines(strings.Join(lines, "\n"), width, height)
}

// RenderTabs is the compact horizontal navigation used on narrow terminals.
func RenderTabs(s *theme.Styles, entries []SidebarEntry, selected, width int) string {
	var b strings.Builder
	truncated := false
	for i, entry := range entries {
		label := fmt.Sprintf(" %s %s ", entry.Icon, entry.Title)
		rendered := s.Badge.Render(label)
		if i == selected {
			rendered = s.BadgeActive.Render(label)
		}
		candidate := b.String() + rendered + " "
		// Reserve two cells for the overflow marker instead of silently
		// dropping the remaining categories.
		if VisualWidth(candidate) > width-2 {
			truncated = true
			break
		}
		b.WriteString(rendered)
		b.WriteString(" ")
	}
	if truncated {
		more := s.Subtle.Render(" …")
		if VisualWidth(b.String()+more) <= width {
			b.WriteString(more)
		}
	}
	return PadRight(b.String(), width)
}
