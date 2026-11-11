package sixel

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

const (
	// DECSET 2026 = synchronized output. Everything between the begin and end
	// sequences is drawn as one frame, so the terminal never shows the state
	// where the text rewrite has erased the artwork cells but the new DCS
	// payload has not been painted yet (that intermediate frame is the flicker
	// seen when lyrics scroll). Windows Terminal 1.24 implements it; terminals
	// that do not simply ignore the unknown private mode.
	syncBegin = "\x1b[?2026h"
	syncEnd   = "\x1b[?2026l"
)

// Layer is the shared state between the TUI model (which knows where the cover
// should be drawn) and the output writer (which can only send bytes after a
// Bubble Tea frame has been flushed). It is safe for concurrent use: View()
// runs on the model goroutine while the renderer flushes on its own goroutine.
type Layer struct {
	mu      sync.Mutex
	enabled bool
	pending bool
	clear   bool
	row     int
	col     int
	payload string
	key     string
}

// NewLayer creates a disabled-by-default layer. Sixel output is opt-in.
func NewLayer(enabled bool) *Layer { return &Layer{enabled: enabled} }

// Enabled reports whether the Sixel path is active.
func (l *Layer) Enabled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}

// Place schedules a Sixel image at a 1-based terminal row/column. A repeated
// key is ignored, so a 500ms tick never re-sends the same image.
func (l *Layer) Place(row, col int, payload, key string) {
	if payload == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.enabled {
		return
	}
	if l.key == key && l.payload == payload && !l.pending {
		return
	}
	l.row, l.col, l.payload, l.key = row, col, payload, key
	l.pending = true
	l.clear = false
}

// Invalidate requests removal of the current image. Sixel has no "delete one
// image" command, so the writer clears the screen and replays the last frame.
func (l *Layer) Invalidate() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.enabled || (l.key == "" && l.payload == "") {
		return
	}
	l.key, l.payload = "", ""
	l.pending = true
	l.clear = true
}

// Take consumes one pending action. frame contains the bytes to append after
// the current text frame; clear tells the writer to wipe the screen first.
func (l *Layer) Take() (frame string, clear, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.pending {
		return "", false, false
	}
	l.pending = false
	if l.clear {
		return "", true, true
	}
	if l.payload == "" {
		return "", false, false
	}
	// CUP positions the cursor at the artwork's top-left cell; Sixel then
	// paints into that area without becoming part of the character grid.
	// DECSC/DECRC (ESC 7 / ESC 8) bracket the image because Windows Terminal
	// 1.22+ moves the cursor to the line below a finished Sixel image, while
	// xterm/foot/WezTerm leave it on the first image row. Without restoring
	// it, Bubble Tea's relative cursor moves desync and the next text frame
	// overwrites the artwork.
	return fmt.Sprintf("\x1b7\x1b[%d;%dH%s\x1b8", l.row, l.col, l.payload), false, true
}

// Writer wraps Bubble Tea's output. The standard renderer calls Write once per
// flushed frame, so appending the Sixel payload here guarantees it lands after
// the text frame instead of being truncated/diffed as part of View().
type Writer struct {
	Out   io.Writer
	Layer *Layer

	mu        sync.Mutex
	lastFrame []byte
}

// Fd keeps the wrapper compatible with Bubble Tea's term.File check. Without
// it the program would skip VT/raw-mode setup and window-size queries even in
// the default text mode, because WithOutput replaces os.Stdout.
func (w *Writer) Fd() uintptr {
	if f, ok := w.Out.(interface{ Fd() uintptr }); ok {
		return f.Fd()
	}
	return ^uintptr(0)
}

// Write forwards the text frame first, then appends the pending Sixel action.
// Bubble Tea also sends short control sequences (alt-screen switches, cursor
// visibility, clear-screen) through this same writer. Those must not consume a
// pending image before the real text frame reaches the terminal.
// It returns the number of bytes consumed from p, as io.Writer requires.
//
// When a graphics action is pending, the text frame and the DCS payload leave
// in a single write wrapped in DECSET 2026 (synchronized output). Bubble Tea
// repaints a whole line whenever any cell changes, so a scrolling lyric wipes
// the cells the artwork covers and the image has to be re-sent; without
// synchronized output the terminal shows that intermediate blank state as a
// flicker on every scroll. Windows Terminal 1.24 honours 2026 and renders the
// pair atomically; terminals that do not know the mode ignore it.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.Layer == nil || !isTextFrame(p) {
		return w.Out.Write(p)
	}
	// Remember the frame even when no image is pending: the clear path needs
	// something to replay.
	w.lastFrame = append(w.lastFrame[:0], p...)
	frame, clear, ok := w.Layer.Take()
	if !ok {
		return w.Out.Write(p)
	}

	var buf bytes.Buffer
	buf.Grow(len(p) + len(frame) + len(w.lastFrame) + len(syncBegin) + len(syncEnd))
	buf.WriteString(syncBegin)
	if clear {
		// Erase the graphics layer and immediately replay the text frame so
		// the user does not see a blank screen until the next tick.
		buf.WriteString("\x1b[2J\x1b[H")
		buf.Write(w.lastFrame)
	} else {
		buf.Write(p)
		buf.WriteString(frame)
	}
	buf.WriteString(syncEnd)
	if _, err := w.Out.Write(buf.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}

// isTextFrame identifies the standard renderer's one-write-per-flush frame.
// It separates rows with CRLF; control sequences are separate short writes.
func isTextFrame(p []byte) bool {
	return bytes.Contains(p, []byte("\r\n"))
}
