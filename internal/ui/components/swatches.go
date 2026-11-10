package components

import (
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/theme"
)

// RenderThemeSwatches draws one row per palette with real colour blocks so
// the settings screen can preview themes without switching them.
func RenderThemeSwatches(s *theme.Styles, themes []theme.Theme, active string, width int) string {
	if width < 16 {
		return ""
	}
	colors := func(t theme.Theme) []string {
		return []string{t.Primary, t.Secondary, t.Accent, t.Success, t.BorderFocus}
	}
	lines := make([]string, 0, len(themes))
	for _, t := range themes {
		mark := "  "
		nameStyle := s.Muted
		if strings.EqualFold(t.Name, active) {
			mark = s.Success.Render("▸ ")
			nameStyle = s.Title
		}
		label := PadRight(Truncate(t.Name, 9), 9)
		fixed := VisualWidth(mark) + VisualWidth(label) + 1
		slots := max(0, min(5, (width-fixed)/2))
		var b strings.Builder
		b.WriteString(mark)
		b.WriteString(nameStyle.Render(label))
		b.WriteString(" ")
		palette := colors(t)
		for i := 0; i < slots; i++ {
			b.WriteString(s.Swatch(palette[i]).Render("██"))
		}
		lines = append(lines, PadRight(b.String(), width))
	}
	return strings.Join(lines, "\n")
}
