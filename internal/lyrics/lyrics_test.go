package lyrics

import (
	"testing"

	"github.com/anccnuer/go-navidrome-tui/internal/model"
)

func TestParseLRC(t *testing.T) {
	text := `[ar:Test Artist]
[ti:Song]
[offset:500]
[00:12.34]First line
[00:45.5][01:02.00]Repeated line
`
	l := ParseLRC(text)
	if !l.Synced {
		t.Fatal("expected synced lyrics")
	}
	if len(l.Lines) != 3 {
		t.Fatalf("lines = %d, want 3: %+v", len(l.Lines), l.Lines)
	}
	// 12.34s shifted by -500ms
	if l.Lines[0].Time != 11840 || l.Lines[0].Text != "First line" {
		t.Fatalf("first line = %+v", l.Lines[0])
	}
	if l.Lines[1].Time != 45000 || l.Lines[2].Time != 61500 {
		t.Fatalf("repeated lines = %+v %+v", l.Lines[1], l.Lines[2])
	}
	if idx := l.LineAt(46000); idx != 1 {
		t.Fatalf("LineAt(46s) = %d, want 1", idx)
	}
	if idx := l.LineAt(0); idx != -1 {
		t.Fatalf("LineAt(0) = %d, want -1", idx)
	}
	if idx := l.LineAt(999000); idx != 2 {
		t.Fatalf("LineAt(end) = %d, want 2", idx)
	}
}

func TestParseLRCMillisecondFormats(t *testing.T) {
	l := ParseLRC("[01:02.1]a\n[01:03.05]b\n[01:04.123]c\n")
	want := []int{62100, 63050, 64123}
	if len(l.Lines) != 3 {
		t.Fatalf("lines = %d", len(l.Lines))
	}
	for i, w := range want {
		if l.Lines[i].Time != w {
			t.Fatalf("line %d time = %d, want %d", i, l.Lines[i].Time, w)
		}
	}
}

func TestParseLRCStripsWordTimestamps(t *testing.T) {
	l := ParseLRC("[00:01.00]<00:01.10>Hel<00:01.30>lo")
	if len(l.Lines) != 1 || l.Lines[0].Text != "Hello" {
		t.Fatalf("unexpected line: %+v", l.Lines)
	}
}

func TestFromStructured(t *testing.T) {
	lines := []model.LyricsLine{
		{Time: 2000, Text: "  later  "},
		{Time: 500, Text: "earlier"},
		{Time: 1000, Text: ""},
	}
	l := FromStructured(lines, true, "test")
	if len(l.Lines) != 2 || l.Lines[0].Text != "earlier" || l.Lines[1].Time != 2000 {
		t.Fatalf("unexpected: %+v", l.Lines)
	}
	if l.Source != "test" || !l.Synced {
		t.Fatalf("unexpected metadata: %+v", l)
	}
	unsynced := FromStructured(lines, false, "")
	if unsynced.Synced || unsynced.Lines[0].Time != 0 {
		t.Fatalf("unsynced should have zero timestamps: %+v", unsynced)
	}
}

func TestFromPlain(t *testing.T) {
	l := FromPlain("line one\nline two\n")
	if l.Synced || len(l.Lines) != 2 {
		t.Fatalf("unexpected: %+v", l)
	}
	// Servers returning LRC in the plain field are detected.
	embedded := FromPlain("[00:03.00]hello")
	if !embedded.Synced || embedded.Lines[0].Time != 3000 {
		t.Fatalf("embedded LRC not detected: %+v", embedded)
	}
}
