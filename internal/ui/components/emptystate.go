package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/subsonic-tui/subsonic-tui/internal/theme"
)

// EmptyState describes a centred, illustrated empty/loading state.
type EmptyState struct {
	Glyph   string
	Title   string
	Detail  string
	Hint    string
	Loading bool
}

const defaultEmptyArt = "╭─────────╮\n│  ♪  ♫  │\n╰─────────╯"

func blankLine(width int) string { return strings.Repeat(" ", width) }

func centered(text string, width int) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, text)
}

// RenderEmptyState always returns exactly height lines of width cells.
func RenderEmptyState(s *theme.Styles, e EmptyState, width, height int) string {
	if width < 8 || height < 2 {
		return FitLines("", width, max(1, height))
	}
	lines := make([]string, 0, height)
	titleLine, detailLine := "", ""
	if e.Title != "" {
		titleLine = centered(s.Subtitle.Render(Truncate(e.Title, width-4)), width)
	}
	if e.Detail != "" {
		detailLine = centered(s.Muted.Render(Truncate(e.Detail, width-4)), width)
	}
	hintLine := ""
	if e.Hint != "" {
		hintLine = centered(s.Chip.Render(Truncate(e.Hint, width-6)), width)
	}

	if height < 7 {
		// Short viewports prioritise the message over the illustration.
		if titleLine != "" {
			lines = append(lines, titleLine)
		}
		if detailLine != "" && len(lines) < height {
			lines = append(lines, detailLine)
		}
		if hintLine != "" && len(lines)+1 <= height {
			lines = append(lines, hintLine)
		}
	} else {
		if e.Loading {
			for _, w := range []int{min(width-6, 28), min(width-10, 22), min(width-8, 26)} {
				if w < 4 {
					w = 4
				}
				lines = append(lines, centered(s.EmptyGlyph.Render(strings.Repeat("░", w)), width))
			}
		} else {
			art := e.Glyph
			if art == "" {
				art = defaultEmptyArt
			}
			for _, line := range strings.Split(art, "\n") {
				lines = append(lines, centered(s.EmptyGlyph.Render(Truncate(line, width-2)), width))
			}
		}
		if titleLine != "" {
			lines = append(lines, blankLine(width))
			lines = append(lines, titleLine)
		}
		if detailLine != "" {
			lines = append(lines, detailLine)
		}
		if hintLine != "" && len(lines)+2 <= height {
			lines = append(lines, blankLine(width))
			lines = append(lines, hintLine)
		}
	}

	padTop := max(0, (height-len(lines))/2)
	out := make([]string, 0, height)
	for i := 0; i < padTop; i++ {
		out = append(out, blankLine(width))
	}
	out = append(out, lines...)
	for len(out) < height {
		out = append(out, blankLine(width))
	}
	if len(out) > height {
		out = out[:height]
	}
	return strings.Join(out, "\n")
}

// RenderErrorState draws an inline, actionable error banner.
func RenderErrorState(s *theme.Styles, err, hint string, width int) string {
	if width < 10 {
		return ""
	}
	line := " " + s.ToastErr.Render("⚠ "+Truncate(err, width-8))
	if hint != "" {
		line += " " + s.Subtle.Render(Truncate(hint, width-VisualWidth(line)-2))
	}
	return PadRight(line, width)
}
