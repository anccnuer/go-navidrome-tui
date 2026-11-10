// Package components contains pure rendering helpers. They never perform IO
// or hold state: everything they need is passed in so View() stays cheap.
package components

import (
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// VisualWidth returns the number of terminal cells a string occupies.
func VisualWidth(s string) int { return lipgloss.Width(s) }

// Truncate clips a string to w cells, appending an ellipsis when possible.
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	target := w - 1
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > target {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + "…"
}

// PadRight pads a string with spaces up to w visible cells.
func PadRight(s string, w int) string {
	pad := w - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

// FitWidth clips and pads every line to exactly w cells.
func FitWidth(s string, w int) string {
	lines := strings.Split(s, "\n")
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = PadRight(Truncate(line, w), w)
	}
	return strings.Join(out, "\n")
}

// FitHeight clips or pads a block to exactly h lines.
func FitHeight(s string, h int) string {
	if h <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// JoinColumns joins two blocks horizontally, aligning line by line.
func JoinColumns(left, right string, leftWidth, gap int) string {
	ll := strings.Split(left, "\n")
	rl := strings.Split(right, "\n")
	n := len(ll)
	if len(rl) > n {
		n = len(rl)
	}
	spacer := strings.Repeat(" ", gap)
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		l, r := "", ""
		if i < len(ll) {
			l = PadRight(ll[i], leftWidth)
		} else {
			l = strings.Repeat(" ", leftWidth)
		}
		if i < len(rl) {
			r = rl[i]
		}
		b.WriteString(l + spacer + r)
	}
	return b.String()
}

// Panel renders a titled rounded box with a fixed outer width and height.
func Panel(s *theme.Styles, title, content string, width, height int, focused bool) string {
	if width < 4 || height < 2 {
		return FitWidth(FitHeight(content, height), width)
	}
	style := s.Panel
	if focused {
		style = s.PanelFocus
	}
	innerW := width - 2 // rounded border occupies one cell on each side
	innerH := height - 2
	if title != "" {
		chip := s.PanelTitleDim.Render(" " + title + " ")
		if focused {
			chip = s.PanelTitle.Render(" " + title + " ")
		}
		content = chip + "\n" + content
	}
	return style.Width(innerW).Height(innerH).Render(FitLines(content, innerW, innerH))
}

// FitLines pads every line to w cells and clips/pads the block to h lines.
// Unlike Truncate it never cuts into an ANSI escape sequence, which makes it
// safe for pre-styled content.
func FitLines(s string, w, h int) string {
	if h <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i := range lines {
		lines[i] = PadRight(lines[i], w)
	}
	blank := strings.Repeat(" ", w)
	for len(lines) < h {
		lines = append(lines, blank)
	}
	return strings.Join(lines, "\n")
}
