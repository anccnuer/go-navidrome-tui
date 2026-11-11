package sixel

import (
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

// Probe is what the terminal told us about itself. Environment variables are
// only hints: nushell and IDE terminals frequently drop WT_SESSION/TERM, so a
// missing hint proves nothing. The only authoritative answers are the device
// queries below.
type Probe struct {
	// IsTerminal reports whether stdin and stdout were both terminals, i.e.
	// whether asking was even possible.
	IsTerminal bool
	// NoReply reports that the queries went unanswered (or could not be sent).
	NoReply bool
	// Sixel reports that DA1 announced attribute 4 (Sixel graphics). Windows
	// Terminal only includes it when its Sixel renderer is active.
	Sixel bool
	// DA1Raw is the raw reply with control bytes escaped, for pasting in a
	// bug report.
	DA1Raw string
	// CellWidth/CellHeight are the character cell size in pixels, or 0 when
	// the terminal does not answer CSI 16 t.
	CellWidth  int
	CellHeight int
}

const (
	// QueryCellSize asks for the character cell size in pixels (CSI 16 t).
	// Windows Terminal, xterm, WezTerm and foot answer CSI 6 ; height ; width t.
	QueryCellSize = "\x1b[16t"
	// QueryDA1 asks for the primary device attributes (CSI c). Parameter 4 is
	// the traditional DEC "Sixel graphics" bit.
	QueryDA1 = "\x1b[c"

	// DefaultProbeTimeout is a compromise: long enough for a terminal that
	// answers after a redraw, short enough that a silent one is not felt.
	DefaultProbeTimeout = 300 * time.Millisecond
)

// QueryTerminal asks the terminal about Sixel support and its cell size. It
// never blocks longer than timeout and never runs when stdin/stdout are piped,
// so it is safe to call from a standalone command. Bubble Tea's input loop must
// not run at the same time: the replies arrive on stdin and would otherwise be
// interpreted as keystrokes.
func QueryTerminal(out io.Writer, in io.Reader, timeout time.Duration) Probe {
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	outFile, outOK := out.(interface{ Fd() uintptr })
	inFile, inOK := in.(interface{ Fd() uintptr })
	if !outOK || !inOK || !term.IsTerminal(outFile.Fd()) || !term.IsTerminal(inFile.Fd()) {
		return Probe{NoReply: true}
	}
	probe := Probe{IsTerminal: true}
	state, err := term.MakeRaw(inFile.Fd())
	if err != nil {
		// Without raw mode the reply would sit in the line buffer until the
		// user presses Enter, so do not pretend to have asked.
		probe.NoReply = true
		return probe
	}
	defer func() { _ = term.Restore(inFile.Fd(), state) }()
	return query(out, in, timeout)
}

// query is the testable core: it writes both queries and collects whatever
// arrives within timeout. Callers must already hold the terminal in raw mode.
func query(out io.Writer, in io.Reader, timeout time.Duration) Probe {
	probe := Probe{IsTerminal: true}
	if _, err := io.WriteString(out, QueryCellSize+QueryDA1); err != nil {
		probe.NoReply = true
		return probe
	}
	replies := readReplies(in, timeout)
	parsed := parseReplies(replies)
	probe.DA1Raw = escapeReply(replies)
	probe.Sixel = parsed.sixel
	probe.CellWidth, probe.CellHeight = parsed.cellWidth, parsed.cellHeight
	probe.NoReply = len(replies) == 0
	return probe
}

// readReplies accumulates input until both answers have arrived or the timeout
// expires. A read goroutine may outlive the call when the terminal stays
// silent: the console read cannot be cancelled, and the only caller is a
// command that exits right afterwards.
func readReplies(in io.Reader, timeout time.Duration) []byte {
	// Buffered so a reader that keeps producing bytes after we gave up can
	// still finish instead of blocking forever on the send.
	chunks := make(chan []byte, 16)
	go func() {
		defer close(chunks)
		buf := make([]byte, 256)
		for {
			n, err := in.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				chunks <- chunk
			}
			if err != nil {
				return
			}
		}
	}()

	var replies []byte
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				return replies
			}
			replies = append(replies, chunk...)
			if parsed := parseReplies(replies); parsed.da1 && parsed.cellSize {
				return replies
			}
		case <-timer.C:
			return replies
		}
	}
}

// terminalReplies is the subset of answers we care about.
type terminalReplies struct {
	sixel      bool
	da1        bool // a DA1 reply was received at all
	cellSize   bool // a CSI 16 t reply was received
	cellWidth  int
	cellHeight int
}

// parseReplies extracts the DA1 Sixel bit and the cell size from a reply
// stream. Anything that is not a complete CSI sequence is ignored, so stray
// keystrokes or SGR noise from the shell do not confuse it.
func parseReplies(replies []byte) terminalReplies {
	var parsed terminalReplies
	for i := 0; i < len(replies); i++ {
		if replies[i] != 0x1b || i+1 >= len(replies) || replies[i+1] != '[' {
			continue
		}
		start := i + 2
		j := start
		for j < len(replies) && replies[j] >= 0x20 && replies[j] <= 0x3f {
			j++
		}
		if j >= len(replies) {
			return parsed // truncated reply
		}
		final := replies[j]
		params := string(replies[start:j])
		i = j
		switch final {
		case 'c':
			// DA1: CSI ? P1 ; P2 ; ... c
			if !strings.HasPrefix(params, "?") {
				continue
			}
			parsed.da1 = true
			for _, field := range strings.Split(params[1:], ";") {
				if field == "4" {
					parsed.sixel = true
				}
			}
		case 't':
			// CSI 16 t reply: CSI 6 ; height ; width t
			fields := strings.Split(params, ";")
			if len(fields) != 3 || fields[0] != "6" {
				continue
			}
			h, okH := atoi(fields[1])
			w, okW := atoi(fields[2])
			if okH && okW && h > 0 && w > 0 {
				parsed.cellSize = true
				parsed.cellHeight, parsed.cellWidth = h, w
			}
		}
	}
	return parsed
}

// atoi avoids importing strconv for two tiny fields.
func atoi(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	value := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		value = value*10 + int(s[i]-'0')
	}
	return value, true
}

// escapeReply makes a raw terminal reply safe to print: control bytes, which
// would otherwise be replayed to the terminal, are shown as \x1b / \x07.
func escapeReply(replies []byte) string {
	if len(replies) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range replies {
		switch {
		case c == 0x1b:
			b.WriteString(`\x1b`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\n':
			b.WriteString(`\n`)
		case c < 0x20 || c > 0x7e:
			b.WriteString(`\x`)
			b.WriteByte("0123456789abcdef"[c>>4])
			b.WriteByte("0123456789abcdef"[c&0x0f])
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func (p Probe) describe() string {
	switch {
	case p.IsTerminal && p.NoReply:
		return "终端没有应答 DA1 / CSI 16 t（可能有程序在中间转发，也可能是旧 conhost）"
	case !p.IsTerminal:
		return "stdin/stdout 不是终端（管道或重定向），无法查询"
	case p.Sixel:
		return "终端在 DA1 中通告了 Sixel（属性 4），可以渲染图形"
	default:
		return "终端有应答，但没有通告 Sixel（属性 4）：当前窗口不渲染 Sixel 图形"
	}
}
