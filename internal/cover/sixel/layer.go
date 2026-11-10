package sixel

import (
	"fmt"
	"io"
	"sync"
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
	return fmt.Sprintf("\x1b[%d;%dH%s", l.row, l.col, l.payload), false, true
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
// It returns the number of bytes consumed from p, as io.Writer requires.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.Out.Write(p)
	if err != nil {
		return n, err
	}
	w.lastFrame = append(w.lastFrame[:0], p[:n]...)
	frame, clear, ok := w.Layer.Take()
	if !ok {
		return n, nil
	}
	if clear {
		// Erase the graphics layer and immediately replay the text frame so
		// the user does not see a blank screen until the next tick.
		_, _ = io.WriteString(w.Out, "\x1b[2J\x1b[H")
		_, _ = w.Out.Write(w.lastFrame)
		return n, nil
	}
	_, _ = io.WriteString(w.Out, frame)
	return n, nil
}
