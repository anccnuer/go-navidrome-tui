package sixel

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

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
	n, err := w.Write([]byte("FRAME"))
	if err != nil || n != len("FRAME") {
		t.Fatalf("write = %d, %v", n, err)
	}
	if got := buf.String(); got != "FRAME\x1b[2;4HIMG" {
		t.Fatalf("unexpected byte order: %q", got)
	}
}

func TestWriterClearReplaysFrame(t *testing.T) {
	var buf bytes.Buffer
	l := NewLayer(true)
	w := &Writer{Out: &buf, Layer: l}
	l.Place(2, 4, "IMG", "k")
	_, _ = w.Write([]byte("FRAME1"))
	buf.Reset()
	l.Invalidate()
	_, _ = w.Write([]byte("FRAME2"))
	if got := buf.String(); got != "FRAME2\x1b[2J\x1b[HFRAME2" {
		t.Fatalf("clear did not wipe and replay: %q", got)
	}
	l.Invalidate()
	if _, clear, ok := l.Take(); ok || clear {
		t.Fatal("invalidate should be idempotent")
	}
}
