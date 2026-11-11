package sixel

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Windows Terminal 1.22+ advances the cursor below a finished Sixel image, so
// the placement must be bracketed by DECSC/DECRC; otherwise Bubble Tea's next
// relative cursor move lands in the wrong row and the artwork is overwritten.
func TestLayerBracketsImageWithSavedCursor(t *testing.T) {
	l := NewLayer(true)
	l.Place(3, 5, "PAYLOAD", "song-1")
	frame, clear, ok := l.Take()
	if !ok || clear {
		t.Fatalf("take = %q clear=%v ok=%v", frame, clear, ok)
	}
	if !strings.HasPrefix(frame, "\x1b7") || !strings.HasSuffix(frame, "\x1b8") {
		t.Fatalf("image is not wrapped in ESC7/ESC8: %q", frame)
	}
	if !strings.Contains(frame, "\x1b[3;5H") {
		t.Fatalf("CUP missing between the cursor save and restore: %q", frame)
	}
}

func TestLayerDeduplicatesByKey(t *testing.T) {
	l := NewLayer(true)
	l.Place(3, 5, "PAYLOAD", "song-1")
	frame, clear, ok := l.Take()
	if !ok || clear {
		t.Fatalf("take = %q clear=%v ok=%v", frame, clear, ok)
	}
	if !strings.Contains(frame, "\x1b[3;5H") || !strings.Contains(frame, "PAYLOAD") {
		t.Fatalf("frame missing CUP or payload: %q", frame)
	}
	l.Place(3, 5, "PAYLOAD", "song-1")
	if _, _, ok := l.Take(); ok {
		t.Fatal("an unchanged key must not reschedule the image")
	}
	l.Place(4, 6, "PAYLOAD", "song-2")
	if _, _, ok := l.Take(); !ok {
		t.Fatal("a new key must reschedule the image")
	}
}

func TestDisabledLayerIgnoresPlace(t *testing.T) {
	l := NewLayer(false)
	l.Place(1, 1, "PAYLOAD", "k")
	if _, _, ok := l.Take(); ok {
		t.Fatal("disabled layer produced output")
	}
}

func TestWriterExposesUnderlyingFd(t *testing.T) {
	r, file, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer file.Close()
	w := &Writer{Out: file, Layer: NewLayer(false)}
	if w.Fd() != file.Fd() {
		t.Fatal("wrapper must expose the terminal fd for Bubble Tea")
	}
}

func TestWriterAppendsAfterTextFrame(t *testing.T) {
	var buf bytes.Buffer
	l := NewLayer(true)
	w := &Writer{Out: &buf, Layer: l}
	l.Place(2, 4, "IMG", "k")
	n, err := w.Write([]byte("FRAME\r\n"))
	if err != nil || n != len("FRAME\r\n") {
		t.Fatalf("write = %d, %v", n, err)
	}
	if got := buf.String(); got != syncBegin+"FRAME\r\n\x1b7\x1b[2;4HIMG\x1b8"+syncEnd {
		t.Fatalf("unexpected byte order: %q", got)
	}
}

// The text rewrite and the DCS payload must reach the terminal inside one
// synchronized-output block: Bubble Tea repaints whole lines, so an erase and a
// re-send split across two frames showed up as a flicker on every lyric scroll.
func TestWriterPresentsFrameAndImageAtomically(t *testing.T) {
	var buf bytes.Buffer
	l := NewLayer(true)
	w := &Writer{Out: &buf, Layer: l}
	l.Place(2, 4, "IMG", "k")
	if _, err := w.Write([]byte("ROW1\r\nROW2")); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.HasPrefix(got, syncBegin) || !strings.HasSuffix(got, syncEnd) {
		t.Fatalf("frame and image are not wrapped in synchronized output: %q", got)
	}
	if strings.Index(got, "ROW2") > strings.Index(got, "IMG") {
		t.Fatalf("image must follow the text frame inside the same block: %q", got)
	}
	if n := strings.Count(got, syncBegin); n != 1 {
		t.Fatalf("expected a single synchronized block, found %d", n)
	}
}

// Short control writes must stay unwrapped: wrapping them in 2026 would make
// the terminal hold alt-screen switches and cursor updates.
func TestWriterLeavesControlWritesAlone(t *testing.T) {
	var buf bytes.Buffer
	w := &Writer{Out: &buf, Layer: NewLayer(true)}
	if _, err := w.Write([]byte("\x1b[?1049h")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "\x1b[?1049h" {
		t.Fatalf("control sequence was wrapped: %q", got)
	}
}

// Bubble Tea sends cursor visibility, alt-screen and clear-screen sequences
// through the same writer. Those short writes must not consume a pending image
// (or clear) before the full text frame has been flushed.
func TestWriterIgnoresControlWrites(t *testing.T) {
	var buf bytes.Buffer
	l := NewLayer(true)
	w := &Writer{Out: &buf, Layer: l}
	l.Place(2, 4, "IMG", "k")
	if _, err := w.Write([]byte("\x1b[?25l")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "\x1b[?25l" {
		t.Fatalf("control write consumed the pending image: %q", got)
	}
	buf.Reset()
	if _, err := w.Write([]byte("ROW1\r\nROW2")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != syncBegin+"ROW1\r\nROW2\x1b7\x1b[2;4HIMG\x1b8"+syncEnd {
		t.Fatalf("image was not appended after the next frame: %q", got)
	}

	l.Invalidate()
	buf.Reset()
	if _, err := w.Write([]byte("\x1b[2J")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "\x1b[2J" {
		t.Fatalf("control write consumed the pending clear: %q", got)
	}
	buf.Reset()
	if _, err := w.Write([]byte("NEW\r\nFRAME")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != syncBegin+"\x1b[2J\x1b[HNEW\r\nFRAME"+syncEnd {
		t.Fatalf("clear did not wipe and replay the full frame: %q", got)
	}
}

func TestWriterClearReplaysFrame(t *testing.T) {
	var buf bytes.Buffer
	l := NewLayer(true)
	w := &Writer{Out: &buf, Layer: l}
	l.Place(2, 4, "IMG", "k")
	_, _ = w.Write([]byte("FRAME1\r\n"))
	buf.Reset()
	l.Invalidate()
	_, _ = w.Write([]byte("FRAME2\r\n"))
	if got := buf.String(); got != syncBegin+"\x1b[2J\x1b[HFRAME2\r\n"+syncEnd {
		t.Fatalf("clear did not wipe and replay: %q", got)
	}
	l.Invalidate()
	if _, clear, ok := l.Take(); ok || clear {
		t.Fatal("invalidate should be idempotent")
	}
}
