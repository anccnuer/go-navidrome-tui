package components

import (
	"strings"

	"github.com/subsonic-tui/subsonic-tui/internal/keymap"
	"github.com/subsonic-tui/subsonic-tui/internal/theme"
)

// RenderHelp keeps the original signature for callers that do not filter.
func RenderHelp(s *theme.Styles, groups []keymap.HelpGroup, width, height, offset int) string {
	return RenderHelpWithQuery(s, groups, "", width, height, offset)
}

// RenderHelpWithQuery builds the shortcut sheet with a coloured group header,
// aligned key caps and highlighted query matches.
func RenderHelpWithQuery(s *theme.Styles, groups []keymap.HelpGroup, query string, width, height, offset int) string {
	if width < 12 || height < 2 {
		return FitLines("", width, max(1, height))
	}
	lines := make([]string, 0, 64)
	for _, group := range groups {
		// Truncate the plain title before styling so PadRight never has to
		// deal with a string that is already wider than the viewport.
		title := Truncate(group.Title, max(4, width-6))
		head := "  " + s.GroupHeader.Render("▎ "+title) + " "
		lines = append(lines, PadRight(head, width))
		for _, entry := range group.Entries {
			keys := make([]string, 0, len(entry.Keys))
			// Cap the key column so leftW never has to be clamped below its
			// minimum; otherwise the assembled row would exceed `width`.
			budget := max(4, width-13)
			used := 0
			for _, key := range entry.Keys {
				cap := s.KeyCap.Render(key)
				w := VisualWidth(cap) + 1
				if used+w > budget {
					break
				}
				keys = append(keys, cap)
				used += w
			}
			right := strings.Join(keys, " ")
			rightW := VisualWidth(right)
			leftW := width - rightW - 3
			if leftW < 10 {
				leftW = 10
			}
			left := "   " + highlightMatch(s, entry.Label, query, leftW-3)
			lines = append(lines, PadRight(left, leftW)+" "+right)
		}
		lines = append(lines, "")
	}
	if len(lines) == 0 {
		return RenderEmptyState(s, EmptyState{
			Title: "没有匹配的快捷键", Detail: "试试别的关键词", Hint: "Esc 返回",
		}, width, height)
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		offset = len(lines) - 1
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return FitLines(strings.Join(lines[offset:end], "\n"), width, height)
}

func highlightMatch(s *theme.Styles, text, query string, width int) string {
	label := Truncate(text, width)
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return label
	}
	idx := strings.Index(strings.ToLower(label), q)
	if idx < 0 {
		return label
	}
	end := idx + len(q)
	if end > len(label) {
		end = len(label)
	}
	return label[:idx] + s.ChipActive.Render(label[idx:end]) + label[end:]
}

// HelpLineCount returns the total number of rows produced by RenderHelp.
func HelpLineCount(groups []keymap.HelpGroup) int {
	n := 0
	for _, group := range groups {
		n += 2 + len(group.Entries) // header + entries + blank
	}
	return n
}
