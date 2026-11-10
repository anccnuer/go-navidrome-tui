package sixel

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Environment holds the cheap, non-blocking hints we can read without a
// terminal query/response handshake (which Bubble Tea's input loop would
// consume). It is advisory only: the real proof is --sixel-check.
type Environment struct {
	WTSession bool
	Term      string
	ColorTerm string
	TMUX      bool
	SSH       bool
}

// ReadEnvironment never blocks and never writes to the terminal.
func ReadEnvironment() Environment {
	return Environment{
		WTSession: os.Getenv("WT_SESSION") != "",
		Term:      strings.ToLower(os.Getenv("TERM")),
		ColorTerm: strings.ToLower(os.Getenv("COLORTERM")),
		TMUX:      os.Getenv("TMUX") != "" || strings.Contains(strings.ToLower(os.Getenv("TERM")), "screen"),
		SSH:       os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "",
	}
}

// LikelySupported is a conservative guess used for the self-check hint. It is
// deliberately not used to enable Sixel automatically: older Windows Terminal
// builds also set WT_SESSION but cannot draw graphics.
func (e Environment) LikelySupported() bool {
	if e.Term == "dumb" || e.TMUX || e.SSH {
		return false
	}
	if e.WTSession {
		return true // Windows Terminal >= 1.22 is expected to support Sixel
	}
	for _, hint := range []string{"xterm", "mlterm", "foot", "wezterm", "kitty", "contour"} {
		if strings.Contains(e.Term, hint) || strings.Contains(e.ColorTerm, hint) {
			return true
		}
	}
	return false
}

// Describe explains what was detected, in plain language.
func (e Environment) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "  WT_SESSION : %v\n", e.WTSession)
	fmt.Fprintf(&b, "  TERM       : %q\n", e.Term)
	fmt.Fprintf(&b, "  COLORTERM  : %q\n", e.ColorTerm)
	fmt.Fprintf(&b, "  tmux/screen: %v   SSH: %v\n", e.TMUX, e.SSH)
	if e.LikelySupported() {
		b.WriteString("  结论：环境看起来支持 Sixel（仍需看下方图片是否出现）。\n")
	} else {
		b.WriteString("  结论：未检测到图形终端，或处于 tmux / SSH / dumb 环境，建议保持纯文本封面。\n")
	}
	return b.String()
}

// SelfCheck prints the environment and a generated test image. Run it in a
// real terminal: a visible gradient proves DCS passthrough actually works
// (including ConPTY on Windows), which cannot be verified from a TUI test.
func SelfCheck(out io.Writer) error {
	env := ReadEnvironment()
	fmt.Fprintln(out, "ndtui Sixel 自检")
	fmt.Fprint(out, env.Describe())
	fmt.Fprintln(out, "下面应该出现一张彩色渐变图（240x120 像素）：")
	payload, err := Encode(GradientImage(240, 120), 240, 120)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(out, payload); err != nil {
		return err
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "看到图片 → 可在 config.toml 设 [ui] coverProtocol = \"sixel\"。")
	fmt.Fprintln(out, "没看到或排版错乱 → 保持默认的 \"text\"，纯文本半块字符封面仍可用。")
	return nil
}
