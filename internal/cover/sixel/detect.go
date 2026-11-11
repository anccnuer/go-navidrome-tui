package sixel

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Environment holds the cheap, non-blocking hints we can read without a
// terminal query/response handshake (which Bubble Tea's input loop would
// consume). These hints are advisory only: nushell, IDE terminals and
// detached launchers routinely drop WT_SESSION and leave TERM empty, so the
// real proof is the DA1 probe in --sixel-check.
type Environment struct {
	WTSession   bool
	WTProfileID string
	Term        string
	TermProgram string
	ColorTerm   string
	TMUX        bool
	SSH         bool
}

// ReadEnvironment never blocks and never writes to the terminal.
func ReadEnvironment() Environment {
	return Environment{
		WTSession:   os.Getenv("WT_SESSION") != "",
		WTProfileID: os.Getenv("WT_PROFILE_ID"),
		Term:        strings.ToLower(os.Getenv("TERM")),
		TermProgram: strings.ToLower(os.Getenv("TERM_PROGRAM")),
		ColorTerm:   strings.ToLower(os.Getenv("COLORTERM")),
		TMUX:        os.Getenv("TMUX") != "" || strings.Contains(strings.ToLower(os.Getenv("TERM")), "screen"),
		SSH:         os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "",
	}
}

// LikelySupported is a conservative hint used by --sixel-check and startup
// logs. It must not gate an explicit coverProtocol="sixel": ConPTY often leaves
// TERM unset or "dumb", shells such as nushell do not always forward
// WT_SESSION, and IDE terminals may not forward it either.
func (e Environment) LikelySupported() bool {
	if e.TMUX || e.SSH {
		return false
	}
	// Windows Terminal identifies itself independently of TERM. A dumb TERM
	// here is a shell/IDE artefact, not proof that Sixel is unavailable.
	if e.WTSession || e.WTProfileID != "" || strings.EqualFold(e.TermProgram, "windows_terminal") {
		return true
	}
	if e.Term == "dumb" {
		return false
	}
	termProgram := strings.ToLower(e.TermProgram)
	for _, hint := range []string{"sixel", "xterm", "mlterm", "foot", "wezterm", "kitty", "contour"} {
		if strings.Contains(e.Term, hint) || strings.Contains(e.ColorTerm, hint) ||
			strings.Contains(termProgram, hint) {
			return true
		}
	}
	return false
}

// ConfigEnabled is the hard gate for opt-in Sixel output. An explicit
// "sixel" setting wins over every environment hint; only a redirected regular
// file on stdout falls back, so DCS payloads are never written into a log.
func ConfigEnabled(showCover bool, protocol string, stdoutRegularFile bool) bool {
	return showCover && strings.EqualFold(strings.TrimSpace(protocol), "sixel") && !stdoutRegularFile
}

// Describe explains what was detected, in plain language. It is deliberately
// not a verdict: only DescribeProbe and the reference images are.
func (e Environment) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "  WT_SESSION   : %v\n", e.WTSession)
	fmt.Fprintf(&b, "  WT_PROFILE_ID: %q\n", e.WTProfileID)
	fmt.Fprintf(&b, "  TERM         : %q\n", e.Term)
	fmt.Fprintf(&b, "  TERM_PROGRAM : %q\n", e.TermProgram)
	fmt.Fprintf(&b, "  COLORTERM    : %q\n", e.ColorTerm)
	fmt.Fprintf(&b, "  tmux/screen  : %v   SSH: %v\n", e.TMUX, e.SSH)
	if e.LikelySupported() {
		b.WriteString("  环境提示：看起来像图形终端，但仍以下面的探测与对照图为准。\n")
	} else {
		b.WriteString("  环境提示：没有识别出图形终端（可能是 conhost、IDE 内置终端，或 nushell 这类不转发 WT_SESSION 的 shell）；环境变量只作参考，显式配置仍会发送 Sixel。\n")
	}
	return b.String()
}

// referenceBlock is a hand-written, spec-example Sixel image: 12x6 magenta
// pixels, P2=1 like chafa/libsixel. It is a compile-time constant on purpose:
// if this one shows but the encoder output does not, the encoder is at fault,
// and if neither shows, the terminal cannot render Sixel at all.
var referenceBlock = DCSStart + `"1;1;12;6#0;2;100;0;100#0` + strings.Repeat("~", 12) + DCSEnd

// SelfCheck prints the environment, what the terminal itself answered, and two
// reference images. probe may be nil when the queries were not run (e.g. in
// tests or with piped output). Run it in a real terminal: a visible gradient
// proves DCS passthrough actually works, including through ConPTY on Windows.
func SelfCheck(out io.Writer, version string, probe *Probe) error {
	env := ReadEnvironment()
	fmt.Fprintf(out, "ndtui Sixel 自检（%s）\n", version)
	fmt.Fprint(out, env.Describe())

	fmt.Fprintln(out, "\n终端探测（DA1 / CSI 16 t）：")
	if probe == nil {
		fmt.Fprintln(out, "  未执行（缺少终端句柄）")
	} else {
		if probe.IsTerminal && probe.DA1Raw != "" {
			fmt.Fprintf(out, "  DA1 应答    : %s\n", probe.DA1Raw)
		} else {
			fmt.Fprintln(out, "  DA1 应答    : （无）")
		}
		fmt.Fprintf(out, "  Sixel 通告  : %v\n", probe.Sixel)
		if probe.CellWidth > 0 && probe.CellHeight > 0 {
			fmt.Fprintf(out, "  字符格像素  : %d x %d\n", probe.CellWidth, probe.CellHeight)
		} else {
			fmt.Fprintln(out, "  字符格像素  : （终端未应答）")
		}
		fmt.Fprintf(out, "  探测结论    : %s\n", probe.describe())
		if !probe.Sixel {
			fmt.Fprintln(out, "  说明        : DA1 里的属性 4 才是 Sixel 能力；Windows Terminal 只在它的 Sixel 渲染器生效时上报。")
			fmt.Fprintln(out, "                下面两张图才是最终判据，若都看不到，请先用 `chafa -f sixel <图片>` 自证终端。")
		}
	}

	fmt.Fprintln(out, "\n对照图 1/2：内联品红块（12x6 像素，手写常量，公认正确）")
	fmt.Fprint(out, referenceBlock)
	fmt.Fprintln(out)

	payload, err := Encode(GradientImage(240, 120), 240, 120)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "对照图 2/2：本程序编码器生成的渐变图（240x120 像素）")
	fmt.Fprint(out, payload)
	fmt.Fprintln(out)

	fmt.Fprintln(out, "判定方法：")
	fmt.Fprintln(out, "  两张都看到      → 终端与编码器都正常，可在 config.toml 设 [ui] coverProtocol = \"sixel\"。")
	fmt.Fprintln(out, "  只看到品红块    → 终端支持 Sixel，但本程序的载荷被拒；请运行 `ndtui --sixel-dump sixel.txt` 并与 `chafa -f sixel` 的输出对比反馈。")
	fmt.Fprintln(out, "  两张都没看到    → 当前窗口不渲染 Sixel（conhost / IDE 内置终端 / 远程会话）；注意 `chafa <图片>` 默认是半块字符，不代表支持 Sixel。")
	if probe != nil && probe.CellWidth > 0 && probe.CellHeight > 0 {
		fmt.Fprintf(out, "  尺寸建议        → 按实测字符格把 [ui] sixelCellWidth/sixelCellHeight 设为 %d / %d。\n",
			probe.CellWidth, probe.CellHeight)
	}
	return nil
}
