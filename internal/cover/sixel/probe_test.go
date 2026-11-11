package sixel

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

// scriptReader replays pre-recorded chunks, optionally byte by byte, so a
// reply that arrives in several reads is handled like a real terminal.
type scriptReader struct {
	chunks [][]byte
	delay  time.Duration
	index  int
}

func newScriptReader(reply string, split bool) *scriptReader {
	if !split {
		return &scriptReader{chunks: [][]byte{[]byte(reply)}}
	}
	chunks := make([][]byte, 0, len(reply))
	for i := 0; i < len(reply); i++ {
		chunks = append(chunks, []byte{reply[i]})
	}
	return &scriptReader{chunks: chunks, delay: time.Millisecond}
}

func (r *scriptReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	n := copy(p, r.chunks[r.index])
	r.index++
	return n, nil
}

// blockingReader never produces data until released, which is what a terminal
// without Sixel support looks like.
type blockingReader struct{ release chan struct{} }

func (r *blockingReader) Read([]byte) (int, error) {
	<-r.release
	return 0, io.EOF
}

func TestQueryParsesDA1AndCellSize(t *testing.T) {
	// Windows Terminal flavoured reply: Sixel attribute 4, no cell size yet.
	var out bytes.Buffer
	in := newScriptReader("\x1b[?62;4;6;22c", false)
	probe := query(&out, in, 300*time.Millisecond)
	if !probe.Sixel {
		t.Fatal("DA1 attribute 4 should mark Sixel as supported")
	}
	if probe.NoReply {
		t.Fatal("a terminal that answered must not be reported as silent")
	}
	if !strings.Contains(probe.DA1Raw, "?62;4;6;22c") || strings.ContainsRune(probe.DA1Raw, 0x1b) {
		t.Fatalf("raw reply should be escaped and complete: %q", probe.DA1Raw)
	}
	if out.String() != QueryCellSize+QueryDA1 {
		t.Fatalf("queries were not written: %q", out.String())
	}
}

func TestQueryParsesCellSize(t *testing.T) {
	in := newScriptReader("\x1b[?62;4;6;22c\x1b[6;20;10t", false)
	probe := query(&bytes.Buffer{}, in, 300*time.Millisecond)
	if probe.CellWidth != 10 || probe.CellHeight != 20 {
		t.Fatalf("cell size not parsed: %dx%d", probe.CellWidth, probe.CellHeight)
	}
}

func TestQueryWithoutSixelAttribute(t *testing.T) {
	in := newScriptReader("\x1b[?1;2c", false)
	probe := query(&bytes.Buffer{}, in, 300*time.Millisecond)
	if probe.Sixel {
		t.Fatal("DA1 without attribute 4 must not report Sixel support")
	}
	if probe.NoReply || probe.DA1Raw == "" {
		t.Fatal("an answered DA1 should be reported even without Sixel")
	}
}

func TestQueryToleratesNoiseAndSplitReplies(t *testing.T) {
	in := newScriptReader("abc\x1b]0;title\x07\x1b[?62;4c\x1b[6;16;8t", true)
	probe := query(&bytes.Buffer{}, in, 2*time.Second)
	if !probe.Sixel {
		t.Fatalf("Sixel bit lost in a split reply: %q", probe.DA1Raw)
	}
	if probe.CellWidth != 8 || probe.CellHeight != 16 {
		t.Fatalf("cell size lost in a split reply: %dx%d", probe.CellWidth, probe.CellHeight)
	}
}

func TestQueryTimesOutWithoutReply(t *testing.T) {
	reader := &blockingReader{release: make(chan struct{})}
	defer close(reader.release)

	start := time.Now()
	probe := query(&bytes.Buffer{}, reader, 20*time.Millisecond)
	elapsed := time.Since(start)
	if !probe.NoReply {
		t.Fatal("a silent terminal must be reported as NoReply")
	}
	if probe.Sixel {
		t.Fatal("silence must not be mistaken for support")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("probe blocked for %v", elapsed)
	}
}

// Piped output cannot answer queries, so the probe must not even try.
func TestQueryTerminalSkipsNonTerminal(t *testing.T) {
	var out bytes.Buffer
	probe := QueryTerminal(&out, strings.NewReader("\x1b[?62;4c"), 10*time.Millisecond)
	if probe.IsTerminal {
		t.Fatal("buffers are not terminals")
	}
	if !probe.NoReply || probe.Sixel {
		t.Fatalf("unexpected probe result: %+v", probe)
	}
	if out.Len() != 0 {
		t.Fatalf("queries must not be written to a non-terminal: %q", out.String())
	}
}

func TestParseRepliesIgnoresTruncatedSequence(t *testing.T) {
	if parsed := parseReplies([]byte("\x1b[?62;4")); parsed.da1 || parsed.sixel {
		t.Fatalf("a truncated sequence must be ignored: %+v", parsed)
	}
}

func TestProbeDescribeExplainsVerdicts(t *testing.T) {
	cases := []struct {
		name  string
		probe Probe
		want  string
	}{
		{"silent", Probe{IsTerminal: true, NoReply: true}, "没有应答"},
		{"not a terminal", Probe{NoReply: true}, "不是终端"},
		{"supported", Probe{IsTerminal: true, Sixel: true}, "通告了 Sixel"},
		{"answered but unsupported", Probe{IsTerminal: true, DA1Raw: `\x1b[?1;2c`}, "没有通告 Sixel"},
	}
	for _, tc := range cases {
		if got := tc.probe.describe(); !strings.Contains(got, tc.want) {
			t.Fatalf("%s: describe = %q, want it to mention %q", tc.name, got, tc.want)
		}
	}
}
