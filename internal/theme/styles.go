package theme

import (
	"math"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
)

const gradientSteps = 32

// Styles holds prebuilt styles. Components may derive cheap value copies
// (Width/Bold) per row, but palettes and gradient tables are built only once
// per theme switch; View() never performs IO, decoding or regex work.
type Styles struct {
	T     Theme
	Depth ColorDepth

	App         lipgloss.Style
	Panel       lipgloss.Style
	PanelFocus  lipgloss.Style
	PanelDim    lipgloss.Style
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Text        lipgloss.Style
	Muted       lipgloss.Style
	Subtle      lipgloss.Style
	Primary     lipgloss.Style
	Secondary   lipgloss.Style
	Accent      lipgloss.Style
	Success     lipgloss.Style
	Warning     lipgloss.Style
	Danger      lipgloss.Style
	Info        lipgloss.Style
	Row         lipgloss.Style
	AltRow      lipgloss.Style
	Selected    lipgloss.Style
	Playing     lipgloss.Style
	Starred     lipgloss.Style
	StatusBar   lipgloss.Style
	NowBar      lipgloss.Style
	KeyCap      lipgloss.Style
	Badge       lipgloss.Style
	BadgeActive lipgloss.Style
	Input       lipgloss.Style
	InputFocus  lipgloss.Style
	Divider     lipgloss.Style
	BorderColor lipgloss.Color
	FocusColor  lipgloss.Color

	// Prebuilt polish styles. All of them are constructed once in New().
	PanelTitle    lipgloss.Style
	PanelTitleDim lipgloss.Style
	GroupHeader   lipgloss.Style
	Zebra         lipgloss.Style
	SelectedEdge  lipgloss.Style
	Chip          lipgloss.Style
	ChipActive    lipgloss.Style
	SoftRule      lipgloss.Style
	EmptyGlyph    lipgloss.Style
	ToastOK       lipgloss.Style
	ToastWarn     lipgloss.Style
	ToastErr      lipgloss.Style
	ToastInfo     lipgloss.Style

	// Hint line key/description pairs.
	HintKey  lipgloss.Style
	HintDesc lipgloss.Style

	barGradient   [gradientSteps]lipgloss.Style
	cacheMu       sync.RWMutex
	gradientCache map[string]string
	swatchMu      sync.RWMutex
	swatchCache   map[string]lipgloss.Style
}

// New precomputes all styles for a palette.
func New(t Theme, depth ColorDepth) *Styles {
	s := &Styles{
		T:             t,
		Depth:         depth,
		BorderColor:   lipgloss.Color(t.Border),
		FocusColor:    lipgloss.Color(t.BorderFocus),
		gradientCache: make(map[string]string, 64),
		swatchCache:   make(map[string]lipgloss.Style, 32),
	}
	// The UI is deliberately background-free: only foreground colours are
	// used, so no coloured slab sits behind labels, menu rows or status text.
	s.App = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text))
	s.Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.BorderColor).
		Foreground(lipgloss.Color(t.Text))
	s.PanelFocus = s.Panel.BorderForeground(s.FocusColor)
	s.PanelDim = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))

	s.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Primary))
	s.Subtitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Secondary))
	s.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text))
	s.Muted = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))
	s.Subtle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Subtle))
	s.Primary = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Primary))
	s.Secondary = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Secondary))
	s.Accent = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent))
	s.Success = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Success))
	s.Warning = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Warning))
	s.Danger = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Danger))
	s.Info = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Info))

	// Rows differ only by foreground tone; the selected row is highlighted by
	// colour plus the "=> " marker and half-block caps, never a filled slab.
	s.Row = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text))
	s.AltRow = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))
	s.Selected = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Bold(true)
	s.Playing = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Secondary)).Bold(true)
	s.Starred = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent))

	s.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Muted)).
		Padding(0, 1)
	s.NowBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Padding(0, 1)
	s.KeyCap = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Padding(0, 1)
	s.Badge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Muted)).
		Padding(0, 1)
	s.BadgeActive = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Bold(true).
		Padding(0, 1)
	s.Input = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Padding(0, 1)
	s.InputFocus = s.Input.Foreground(lipgloss.Color(t.Primary)).Bold(true)
	s.Divider = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border))

	s.PanelTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Bold(true).Padding(0, 1)
	s.PanelTitleDim = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Padding(0, 1)
	s.GroupHeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Bold(true)
	s.Zebra = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text))
	s.SelectedEdge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Bold(true)
	s.Chip = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Muted)).
		Padding(0, 1)
	s.ChipActive = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Bold(true).Padding(0, 1)
	s.SoftRule = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border))
	s.EmptyGlyph = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Subtle))
	s.ToastOK = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Success)).Padding(0, 1)
	s.ToastWarn = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Warning)).Padding(0, 1)
	s.ToastErr = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Danger)).Padding(0, 1)
	s.ToastInfo = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Info)).Padding(0, 1)
	s.HintKey = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Subtle)).Bold(true)
	s.HintDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))

	for i := 0; i < gradientSteps; i++ {
		f := float64(i) / float64(gradientSteps-1)
		s.barGradient[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(interpolate(t.Gradient, f)))
	}
	return s
}

// interpolate returns a hex colour along the theme gradient.
func interpolate(colors []string, f float64) string {
	if len(colors) == 0 {
		return "#FFFFFF"
	}
	if len(colors) == 1 || f <= 0 {
		return colors[0]
	}
	if f >= 1 {
		return colors[len(colors)-1]
	}
	scaled := f * float64(len(colors)-1)
	idx := int(scaled)
	if idx >= len(colors)-1 {
		return colors[len(colors)-1]
	}
	local := scaled - float64(idx)
	c1, err1 := colorful.Hex(colors[idx])
	c2, err2 := colorful.Hex(colors[idx+1])
	if err1 != nil || err2 != nil {
		return colors[idx]
	}
	return c1.BlendHcl(c2, local).Hex()
}

// Gradient colours a short string across the theme gradient, with a cache so
// static titles cost nothing on subsequent frames.
func (s *Styles) Gradient(text string) string {
	key := s.T.Name + ":" + text
	s.cacheMu.RLock()
	if v, ok := s.gradientCache[key]; ok {
		s.cacheMu.RUnlock()
		return v
	}
	s.cacheMu.RUnlock()

	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	var b strings.Builder
	for i, r := range runes {
		f := float64(i) / float64(maxInt(1, len(runes)-1))
		style := s.barGradient[int(math.Round(f*float64(gradientSteps-1)))]
		b.WriteString(style.Render(string(r)))
	}
	out := b.String()
	s.cacheMu.Lock()
	if len(s.gradientCache) > 256 {
		s.gradientCache = make(map[string]string, 64)
	}
	s.gradientCache[key] = out
	s.cacheMu.Unlock()
	return out
}

// ProgressBar draws a gradient-filled progress line. It is O(width) and uses
// only prebuilt styles.
func (s *Styles) ProgressBar(width int, ratio float64) string {
	if width < 4 {
		width = 4
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(math.Round(ratio * float64(width)))
	if filled > width {
		filled = width
	}
	var b strings.Builder
	empty := s.Subtle
	for i := 0; i < width; i++ {
		styleIndex := i * gradientSteps / width
		if styleIndex >= gradientSteps {
			styleIndex = gradientSteps - 1
		}
		switch {
		case i < filled:
			b.WriteString(s.barGradient[styleIndex].Render("━"))
		case i == filled && filled > 0:
			b.WriteString(s.barGradient[styleIndex].Render("╸"))
		default:
			b.WriteString(empty.Render("─"))
		}
	}
	return b.String()
}

// ProgressBarHash draws go-musicfox's "#"/"." bar with a per-cell gradient.
func (s *Styles) ProgressBarHash(width int, ratio float64) string {
	if width < 1 {
		return ""
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	var b strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			idx := i * gradientSteps / width
			if idx >= gradientSteps {
				idx = gradientSteps - 1
			}
			b.WriteString(s.barGradient[idx].Render("#"))
		} else {
			b.WriteString(s.Subtle.Render("."))
		}
	}
	return b.String()
}

// VolumeBar draws a compact volume meter.
func (s *Styles) VolumeBar(width, volume int) string {
	if width < 3 {
		width = 3
	}
	return s.ProgressBar(width, float64(clampInt(volume, 0, 100))/100)
}

// DividerLine renders a subtle horizontal rule.
func (s *Styles) DividerLine(width int) string {
	if width <= 0 {
		return ""
	}
	return s.Divider.Render(strings.Repeat("─", width))
}

// Swatch returns a cached style used to paint a two-cell colour block. The
// cache is bounded so custom themes cannot grow it without limit.
func (s *Styles) Swatch(color string) lipgloss.Style {
	if color == "" {
		return s.EmptyGlyph
	}
	s.swatchMu.RLock()
	if st, ok := s.swatchCache[color]; ok {
		s.swatchMu.RUnlock()
		return st
	}
	s.swatchMu.RUnlock()
	st := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	s.swatchMu.Lock()
	if len(s.swatchCache) > 64 {
		s.swatchCache = make(map[string]lipgloss.Style, 32)
	}
	s.swatchCache[color] = st
	s.swatchMu.Unlock()
	return st
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
