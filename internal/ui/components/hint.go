package components

import (
	"strings"

	"github.com/subsonic-tui/subsonic-tui/internal/theme"
)

// Hint is one key/description pair shown under the menu, mirroring
// go-musicfox's centred hint line.
type Hint struct {
	Key  string
	Desc string
}

// RenderHintLine centres "key description" pairs and drops overflow from the
// tail, so narrow terminals keep the most important hints.
func RenderHintLine(s *theme.Styles, hints []Hint, width int) string {
	if width < 12 || len(hints) == 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for i, h := range hints {
		if h.Key == "" {
			continue
		}
		seg := s.HintKey.Render(h.Key) + " " + s.HintDesc.Render(h.Desc)
		w := VisualWidth(seg)
		if i > 0 {
			w += 2
		}
		if used+w > width-2 {
			break
		}
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(seg)
		used += w
	}
	if used == 0 {
		return ""
	}
	line := b.String()
	pad := max(0, (width-VisualWidth(line))/2)
	return PadRight(strings.Repeat(" ", pad)+line, width)
}
