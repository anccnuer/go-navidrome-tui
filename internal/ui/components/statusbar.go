package components

import (
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/theme"
)

// StatusInfo is the data shown by the always-on status bar.
type StatusInfo struct {
	Server     string // active [[servers]] name, shown at the top right
	ViewName   string
	Connection string
	Hint       string
	Warning    bool
	// Breadcrumb is the navigation trail, mirroring go-musicfox's status bar.
	Breadcrumb []string
	// Center is the injected middle segment ("navidrome-tui · [3/120] · …").
	Center string
}

// RenderStatusBar follows go-musicfox: a " » " nugget, the breadcrumb trail
// (last level emphasised), a centred context segment and the right-hand
// connection status plus the active server instance name. Degradation drops
// the right-hand block before the centre, never the navigation context.
func RenderStatusBar(s *theme.Styles, info StatusInfo, width int) string {
	if width < 10 {
		return strings.Repeat(" ", width)
	}

	crumb := " » "
	switch {
	case len(info.Breadcrumb) > 0:
		start := max(0, len(info.Breadcrumb)-3)
		parts := make([]string, 0, 3)
		for i, seg := range info.Breadcrumb[start:] {
			seg = Truncate(seg, 32)
			if start+i == len(info.Breadcrumb)-1 {
				parts = append(parts, s.Primary.Bold(true).Render(seg))
			} else {
				parts = append(parts, s.Muted.Render(seg))
			}
		}
		crumb += strings.Join(parts, s.Divider.Render(" / ")) + " "
	case info.ViewName != "":
		crumb += s.Primary.Bold(true).Render(Truncate(info.ViewName, 32)) + " "
	}

	connStyle := s.Success
	if info.Warning {
		connStyle = s.Warning
	}
	conn := ""
	if info.Connection != "" {
		conn = connStyle.Render("● " + info.Connection)
	}
	rightFull := ""
	if conn != "" {
		rightFull = conn + "  "
	}
	if info.Server != "" {
		rightFull += s.Badge.Render(Truncate(info.Server, 24)) + " "
	}
	rightDot := ""
	if info.Connection != "" {
		rightDot = connStyle.Render("●") + " "
	}
	center := ""
	if info.Center != "" {
		center = " " + s.Muted.Render(info.Center) + " "
	}

	render := func(c, r string) (string, bool) {
		lw, cw, rw := VisualWidth(crumb), VisualWidth(center), VisualWidth(r)
		if lw+cw+rw > width {
			return "", false
		}
		avail := width - lw - rw
		padLeft := max(0, (avail-cw)/2)
		padRight := max(0, avail-cw-padLeft)
		return crumb + strings.Repeat(" ", padLeft) + c + strings.Repeat(" ", padRight) + r, true
	}
	for _, r := range []string{rightFull, rightDot, ""} {
		if out, ok := render(center, r); ok {
			return s.StatusBar.Padding(0).Width(width).Render(out)
		}
	}
	if out, ok := render("", ""); ok {
		return s.StatusBar.Padding(0).Width(width).Render(out)
	}
	return s.StatusBar.Padding(0).Width(width).Render(Truncate(crumb, width))
}
