package components

import (
	"fmt"
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/model"
	"github.com/anccnuer/go-navidrome-tui/internal/theme"
)

// NowPlayingInfo is a UI-friendly snapshot of the playback state.
type NowPlayingInfo struct {
	Status     model.PlayerStatus
	Title      string
	Artist     string
	Album      string
	Position   int
	Duration   int
	Volume     int
	Mode       model.PlayMode
	Starred    bool
	Available  bool
	Message    string
	Frame      int
	Animations bool
}

func (i NowPlayingInfo) ratio() float64 {
	if i.Duration <= 0 {
		return 0
	}
	return float64(i.Position) / float64(i.Duration)
}

// Ratio exposes the playback progress for other views.
func (i NowPlayingInfo) Ratio() float64 {
	r := i.ratio()
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// stateGlyphs mirrors go-musicfox's animated play/pause indicator.
func (i NowPlayingInfo) stateGlyphs() string {
	playing := []string{"♫ ♪ ♫ ♪", "♪ ♫ ♪ ♫"}
	paused := []string{"_ z Z Z", "z Z Z z"}
	switch i.Status {
	case model.StatusPlaying:
		if i.Animations {
			return playing[(i.Frame/2)%2]
		}
		return playing[0]
	case model.StatusPaused:
		return paused[0]
	default:
		return "· · · ·"
	}
}

// SongInfoLine is the first of the two bottom rows: mode, volume, animated
// state, favourite mark, title and artist.
func SongInfoLine(s *theme.Styles, info NowPlayingInfo, width int) string {
	title := info.Title
	if title == "" {
		title = "尚未播放任何曲目"
	}
	mode := s.Secondary.Render("[" + info.Mode.String() + "]")
	vol := s.Info.Render(fmt.Sprintf("%d%%", info.Volume))
	stateStyle := s.Subtle
	if info.Status == model.StatusPlaying {
		stateStyle = s.Warning
	}
	prefix := " " + mode + " " + vol + " " + stateStyle.Render(info.stateGlyphs())
	if info.Starred {
		prefix += " " + s.Starred.Render("♥")
	}
	prefixW := VisualWidth(prefix)
	remain := width - prefixW - 3
	if remain < 8 {
		remain = 8
	}
	titleText := Truncate(title, remain)
	out := prefix + " " + s.Primary.Bold(true).Render(titleText)
	if info.Artist != "" {
		if room := remain - VisualWidth(titleText); room > 6 {
			out += s.Muted.Render("  " + Truncate(info.Artist, room-2))
		}
	}
	return PadRight(out, width)
}

// ProgressBarGeometry returns the bar's start column and width so mouse
// hit-testing shares the exact render geometry.
func ProgressBarGeometry(s *theme.Styles, info NowPlayingInfo, width int) (startX, barWidth int, ok bool) {
	if width < 20 || info.Duration <= 0 {
		return 0, 0, false
	}
	barWidth = width - progressTimeWidth
	if barWidth < 8 {
		return 0, 0, false
	}
	return 0, barWidth, true
}

// progressTimeWidth reserves columns for "00:00/00:00" plus padding.
const progressTimeWidth = 14

// playbarTime formats a duration as go-musicfox does: MM:SS with zero padding.
func playbarTime(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return fmt.Sprintf("%02d:%02d", sec/60, sec%60)
}

// RenderNowPlaying draws the two fixed bottom rows (plus an optional warning
// row). Row 1: song info; row 2: hash progress bar + elapsed/total.
func RenderNowPlaying(s *theme.Styles, info NowPlayingInfo, width int, compact bool) string {
	if width < 10 {
		return strings.Repeat(" ", width)
	}
	if compact {
		line := SongInfoLine(s, info, width)
		return s.NowBar.Padding(0).Width(width).Render(line)
	}

	barW := width - progressTimeWidth
	if barW < 8 {
		barW = 8
	}
	timeText := playbarTime(info.Position) + "/" + playbarTime(info.Duration)
	progress := s.ProgressBarHash(barW, info.Ratio()) + " " + s.Muted.Render(timeText)
	progress = PadRight(progress, width)

	lines := []string{SongInfoLine(s, info, width), progress}
	if info.Message != "" {
		lines = append(lines, s.ToastErr.Render(" ⚠ "+Truncate(info.Message, width-4)))
	}
	return s.NowBar.Padding(0).Width(width).Render(strings.Join(lines, "\n"))
}
