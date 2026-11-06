package components

import (
	"fmt"
	"strings"

	"github.com/subsonic-tui/subsonic-tui/internal/model"
	"github.com/subsonic-tui/subsonic-tui/internal/theme"
)

// ItemKind classifies a list row.
type ItemKind int

const (
	ItemSection ItemKind = iota
	ItemArtist
	ItemAlbum
	ItemPlaylist
	ItemGenre
	ItemSong
	ItemNav // top-level menu entry that pushes another node
)

// Item is the uniform row model used by the browser, search and queue views.
type Item struct {
	Kind      ItemKind
	ID        string
	Title     string
	Subtitle  string
	Extra     string
	CoverID   string
	Duration  int
	Starred   bool
	Song      model.Song
	Album     model.Album
	Artist    model.Artist
	Playlist  model.Playlist
	GenreName string
	Target    model.NodeKind // ItemNav destination
}

// IsPlayable reports whether Enter should start playback.
func (i Item) IsPlayable() bool { return i.Kind == ItemSong }

// IconFor returns a compact, monochrome glyph for a row kind.
func IconFor(k ItemKind) string {
	switch k {
	case ItemArtist:
		return "◉"
	case ItemAlbum:
		return "◆"
	case ItemPlaylist:
		return "≡"
	case ItemGenre:
		return "◇"
	case ItemSong:
		return "♪"
	case ItemNav:
		return "›"
	default:
		return "·"
	}
}

// ListConfig is everything the virtualised list needs to draw one frame.
type ListConfig struct {
	Items      []Item
	Selected   int
	Width      int
	Height     int
	Double     bool
	PlayingID  string
	EmptyText  string
	ShowNumber bool
	Loading    bool
	// Center centres each row's content; used by the short root menu so its
	// entries line up with the centred title and hint line.
	Center bool
}

// ColumnsFor returns 1 or 2 depending on available width.
func ColumnsFor(c ListConfig) int {
	if c.Double && c.Width >= 72 {
		return 2
	}
	return 1
}

// RowsFor returns the number of rows per column.
func RowsFor(c ListConfig) int {
	if c.Height < 1 {
		return 1
	}
	return c.Height
}

// VisibleItems returns how many items fit on one screen.
func VisibleItems(c ListConfig) int { return RowsFor(c) * ColumnsFor(c) }

// ClampOffset keeps the selected row inside the current page.
func ClampOffset(c ListConfig) int {
	pageSize := VisibleItems(c)
	if pageSize <= 0 {
		return 0
	}
	page := c.Selected / pageSize
	offset := page * pageSize
	if offset >= len(c.Items) {
		offset = ((len(c.Items) - 1) / pageSize) * pageSize
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

// FilterItems narrows a list to rows matching query (case-insensitive).
func FilterItems(items []Item, query string) []Item {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return items
	}
	out := make([]Item, 0, len(items))
	for _, item := range items {
		if item.Kind == ItemSection {
			continue
		}
		haystack := strings.ToLower(item.Title + " " + item.Subtitle + " " + item.Extra)
		if strings.Contains(haystack, query) {
			out = append(out, item)
		}
	}
	return out
}

// RenderList draws only the visible viewport rows (O(rows*cols)).
func RenderList(s *theme.Styles, c ListConfig) string {
	if c.Width < 8 || c.Height < 1 {
		return ""
	}
	if len(c.Items) == 0 {
		msg := c.EmptyText
		if msg == "" {
			msg = "这里还没有内容"
		}
		return RenderEmptyState(s, EmptyState{
			Title:   msg,
			Detail:  "按 Ctrl+F 搜索全部歌曲，或检查服务器上是否已有曲目",
			Hint:    "? 查看快捷键",
			Loading: c.Loading,
		}, c.Width, c.Height)
	}

	cols := ColumnsFor(c)
	rows := RowsFor(c)
	colWidth := c.Width / cols
	if cols == 2 {
		// Two columns separated by two blank cells, like go-musicfox's menu.
		colWidth = (c.Width - 2) / 2
	}
	centerWidth := 0
	if c.Center {
		for _, it := range c.Items {
			w := 3 // "   " / "=> " marker
			if c.ShowNumber && it.Kind != ItemNav {
				w += 4 // "%2d. "
			}
			w += VisualWidth(it.Title)
			if it.Subtitle != "" {
				w += 2 + VisualWidth(it.Subtitle)
			}
			centerWidth = max(centerWidth, w)
		}
		centerWidth = min(centerWidth, max(1, colWidth-3))
	}
	offset := ClampOffset(c)
	lines := make([]string, 0, rows)
	for row := 0; row < rows; row++ {
		cells := make([]string, 0, cols)
		for col := 0; col < cols; col++ {
			// Fill row by row (left 1, right 2, left 3, …) like go-musicfox.
			idx := offset + row*cols + col
			if idx >= len(c.Items) {
				cells = append(cells, strings.Repeat(" ", colWidth))
				continue
			}
			cells = append(cells, renderRow(s, c.Items[idx], idx, colWidth, idx == c.Selected, c.Items[idx].ID == c.PlayingID, c.ShowNumber, centerWidth))
		}
		sep := ""
		if cols == 2 {
			sep = "  "
		}
		lines = append(lines, strings.Join(cells, sep))
	}
	return FitLines(strings.Join(lines, "\n"), c.Width, c.Height)
}

// renderRow mirrors go-musicfox's menu rows: a four-cell marker ("=> " or
// "   "), a right-aligned index, the title, then a muted subtitle. The
// selected row is indicated by colour alone — no caps, bars or background.
// centerWidth is the shared content width for a centred menu; 0 disables it.
// Using one width for every row keeps the entries left-aligned inside a
// block that is itself centred on screen.
func renderRow(s *theme.Styles, item Item, index, width int, selected, playing, showNumber bool, centerWidth int) string {
	if width < 4 {
		return strings.Repeat(" ", width)
	}
	if item.Kind == ItemSection {
		label := " " + s.GroupHeader.Render("── "+item.Title+" ──")
		return s.Zebra.Width(width).Render(PadRight(label, width))
	}

	marker := "   "
	if selected {
		marker = "=> "
	}
	num := ""
	if showNumber && item.Kind != ItemNav {
		num = fmt.Sprintf("%2d. ", min(index+1, 999))
	}
	left := marker + num
	room := width - VisualWidth(left) - 1
	if room < 4 {
		room = 4
	}
	title := Truncate(item.Title, room)
	sub := ""
	if item.Subtitle != "" {
		if remain := room - VisualWidth(title); remain > 4 {
			sub = "  " + Truncate(item.Subtitle, remain-2)
		}
	}

	content := left + title + sub
	if centerWidth > 0 {
		inner := min(width, max(1, centerWidth))
		pad := max(0, (width-inner)/2)
		if selected {
			plain := PadRight(Truncate(content, inner), inner)
			return PadRight(strings.Repeat(" ", pad)+s.Selected.Width(inner).Render(plain), width)
		}
		out := s.Muted.Render(marker) + s.Subtle.Render(num)
		if playing {
			out += s.Playing.Render(title)
		} else {
			out += s.Text.Render(title)
		}
		if sub != "" {
			out += s.Muted.Render(sub)
		}
		return s.Row.Width(width).Render(PadRight(strings.Repeat(" ", pad)+out, width))
	}

	if selected {
		return s.Selected.Width(width).Render(PadRight(Truncate(content, width), width))
	}

	out := s.Muted.Render(marker) + s.Subtle.Render(num)
	if playing {
		out += s.Playing.Render(title)
	} else {
		out += s.Text.Render(title)
	}
	if sub != "" {
		out += s.Muted.Render(sub)
	}
	wrap := s.Row
	if index%2 == 1 {
		wrap = s.AltRow
	}
	return wrap.Width(width).Render(PadRight(out, width))
}
