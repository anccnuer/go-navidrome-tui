package components

import (
	"fmt"
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/model"
	"github.com/anccnuer/go-navidrome-tui/internal/theme"
)

// RenderCoverPanel combines a pre-rendered half-block artwork with track
// metadata. Art is produced by the cover package and cached by the UI.
func RenderCoverPanel(s *theme.Styles, art string, song model.Song, width, height int) string {
	if width < 8 || height < 4 {
		return FitLines("", width, max(1, height))
	}
	rule := s.SoftRule.Render(strings.Repeat("─", min(width-4, 16)))
	lines := make([]string, 0, height)
	framed := height >= 10
	if art != "" {
		if framed {
			lines = append(lines, " "+rule)
		}
		for _, line := range strings.Split(art, "\n") {
			lines = append(lines, " "+line)
		}
		if framed {
			lines = append(lines, " "+rule)
		}
	} else {
		space := min(8, max(3, height/2))
		placeholder := RenderEmptyState(s, EmptyState{
			Glyph:  "╭─────╮\n│ ♪ ♫ │\n╰─────╯",
			Title:  "暂无封面",
			Detail: "该专辑没有封面图",
		}, width-2, space)
		for _, line := range strings.Split(placeholder, "\n") {
			lines = append(lines, " "+line)
		}
	}
	lines = append(lines, "")
	lines = append(lines, " "+s.Subtitle.Render(Truncate(song.DisplayTitle(), width-2)))
	if song.DisplayArtist() != "" {
		lines = append(lines, " "+s.Muted.Render(Truncate(song.DisplayArtist(), width-2)))
	}
	if song.Album != "" {
		lines = append(lines, " "+s.Subtle.Render(Truncate(song.Album, width-2)))
	}
	lines = append(lines, " "+rule)

	meta := make([]string, 0, 4)
	if song.Year > 0 {
		meta = append(meta, fmt.Sprintf("%d", song.Year))
	}
	if song.BitRate > 0 {
		meta = append(meta, fmt.Sprintf("%dkbps", song.BitRate))
	}
	if strings.TrimSpace(song.Suffix) != "" {
		meta = append(meta, strings.ToUpper(song.Suffix))
	}
	if song.Duration > 0 {
		meta = append(meta, model.FormatDuration(song.Duration))
	}
	if len(meta) > 0 {
		var b strings.Builder
		used := 0
		for _, value := range meta {
			chip := s.Chip.Render(value) + " "
			w := VisualWidth(chip)
			if used+w > width-2 {
				break // never cut into an ANSI escape sequence
			}
			b.WriteString(chip)
			used += w
		}
		lines = append(lines, " "+PadRight(b.String(), width-2))
	}
	if song.Starred {
		lines = append(lines, " "+s.Starred.Render("♥ 已收藏"))
	}
	return FitLines(strings.Join(lines, "\n"), width, height)
}
