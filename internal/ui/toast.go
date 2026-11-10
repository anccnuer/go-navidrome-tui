package ui

import (
	"strings"
	"time"

	"github.com/anccnuer/go-navidrome-tui/internal/ui/components"
)

type toastLevel int

const (
	ToastInfo toastLevel = iota
	ToastOK
	ToastWarn
	ToastErr
)

type toast struct {
	text  string
	level toastLevel
	born  time.Time
	until time.Time
}

const (
	toastLifetime = 4 * time.Second
	toastFade     = time.Second
	maxToasts     = 3
)

func toastIcon(level toastLevel) string {
	switch level {
	case ToastOK:
		return "✓"
	case ToastWarn:
		return "!"
	case ToastErr:
		return "✕"
	default:
		return "◈"
	}
}

// pushToast keeps a short, deduplicated queue. The legacy statusWarn flag is
// still maintained because the status bar uses it for its warning colour.
func (m *Model) pushToast(text string, level toastLevel) {
	// Error strings may contain newlines; a toast always occupies one row.
	text = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(strings.TrimSpace(text))
	if text == "" {
		return
	}
	now := time.Now()
	warn := level == ToastErr || level == ToastWarn
	if n := len(m.toasts); n > 0 && m.toasts[n-1].text == text && now.Before(m.toasts[n-1].until) {
		m.toasts[n-1].until = now.Add(toastLifetime)
		m.statusWarn = warn
		return
	}
	m.toasts = append(m.toasts, toast{text: text, level: level, born: now, until: now.Add(toastLifetime)})
	if len(m.toasts) > maxToasts {
		m.toasts = m.toasts[len(m.toasts)-maxToasts:]
	}
	m.statusWarn = warn
}

// setStatus is the compatibility entry point for all existing call sites.
func (m *Model) setStatus(text string, warn bool) {
	level := ToastInfo
	if warn {
		level = ToastErr
	}
	m.pushToast(text, level)
}

func (m *Model) setStatusOK(text string)   { m.pushToast(text, ToastOK) }
func (m *Model) setStatusWarn(text string) { m.pushToast(text, ToastWarn) }

// pruneToasts runs on the existing 500ms tick: no extra timers are created.
func (m *Model) pruneToasts(now time.Time) {
	kept := m.toasts[:0]
	for _, t := range m.toasts {
		if now.Before(t.until) {
			kept = append(kept, t)
		}
	}
	m.toasts = kept
	if len(m.toasts) == 0 {
		m.statusWarn = false
	}
}

func (m *Model) latestToast(now time.Time) (toast, bool) {
	for i := len(m.toasts) - 1; i >= 0; i-- {
		if now.Before(m.toasts[i].until) {
			return m.toasts[i], true
		}
	}
	return toast{}, false
}

// statusLine feeds the top status bar hint, keeping the existing hint() flow.
func (m *Model) statusLine() string {
	if t, ok := m.latestToast(time.Now()); ok {
		return toastIcon(t.level) + " " + t.text
	}
	return ""
}

// toastLine renders the bottom toast overlay. Fading only changes a prebuilt
// style on the last second, so it costs no additional repaint frequency.
func (m *Model) toastLine(now time.Time, width int) string {
	t, ok := m.latestToast(now)
	if !ok || width < 16 {
		return ""
	}
	style := m.styles.ToastInfo
	switch t.level {
	case ToastOK:
		style = m.styles.ToastOK
	case ToastWarn:
		style = m.styles.ToastWarn
	case ToastErr:
		style = m.styles.ToastErr
	}
	fading := m.cfg.UI.Animations && now.After(t.until.Add(-toastFade))
	icon := toastIcon(t.level)
	if fading {
		style = m.styles.Chip
		icon = "·" // visible even on terminals without colour support
	}
	body := icon + " " + components.Truncate(t.text, width-10)
	return components.PadRight("  "+style.Render(body), width)
}
