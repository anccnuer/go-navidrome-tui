package sixel

import (
	"bytes"
	"strings"
	"testing"
)

func TestSelfCheckWritesProbeAndReferenceImages(t *testing.T) {
	probe := Probe{IsTerminal: true, Sixel: true, DA1Raw: `\x1b[?62;4;6;22c`, CellWidth: 10, CellHeight: 20}
	var buf bytes.Buffer
	if err := SelfCheck(&buf, "1.2.3", &probe); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"1.2.3", "WT_SESSION", "TERM_PROGRAM", "终端探测", "DA1 应答",
		`\x1b[?62;4;6;22c`, "10 x 20", "对照图 1/2", "对照图 2/2",
		referenceBlock, `"1;1;240;120`, "sixelCellWidth",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("self-check output is missing %q", want)
		}
	}
	// Both reference images must be complete DCS sequences.
	if n := strings.Count(out, DCSStart); n != 2 {
		t.Fatalf("expected two Sixel payloads, found %d", n)
	}
	if n := strings.Count(out, DCSEnd); n != 2 {
		t.Fatalf("expected two terminated payloads, found %d", n)
	}
}

// A missing probe (piped output, tests) must still produce a usable report.
func TestSelfCheckToleratesMissingProbe(t *testing.T) {
	var buf bytes.Buffer
	if err := SelfCheck(&buf, "dev", nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "未执行") {
		t.Fatal("missing probe should be stated explicitly")
	}
	if strings.Contains(out, "sixelCellWidth") {
		t.Fatal("cell size advice must not appear without a probe result")
	}
}

func TestReferenceBlockIsSixelDCS(t *testing.T) {
	if !strings.HasPrefix(referenceBlock, DCSStart) || !strings.HasSuffix(referenceBlock, DCSEnd) {
		t.Fatalf("reference block is not a DCS sequence: %q", referenceBlock)
	}
	if !strings.Contains(referenceBlock, `"1;1;12;6`) {
		t.Fatal("reference block must declare its raster size")
	}
	if strings.Count(referenceBlock, "~") != 12 {
		t.Fatal("reference block must paint exactly 12 columns")
	}
}

func TestLikelySupportedHints(t *testing.T) {
	cases := []struct {
		name string
		env  Environment
		want bool
	}{
		{"windows terminal", Environment{WTSession: true, Term: "xterm-256color"}, true},
		{"windows terminal with dumb term", Environment{WTSession: true, Term: "dumb"}, true},
		{"WT_PROFILE_ID only", Environment{Term: "dumb", WTProfileID: "profile"}, true},
		{"TERM_PROGRAM windows terminal", Environment{Term: "dumb", TermProgram: "Windows_Terminal"}, true},
		{"xterm", Environment{Term: "xterm-256color"}, true},
		{"sixel term", Environment{Term: "xterm-sixel"}, true},
		{"wezterm program", Environment{TermProgram: "WezTerm"}, true},
		{"dumb", Environment{Term: "dumb"}, false},
		{"unknown terminal", Environment{Term: "vt100"}, false},
		{"no hints at all", Environment{}, false},
		{"tmux", Environment{WTSession: true, Term: "screen-256color", TMUX: true}, false},
		{"ssh", Environment{WTSession: true, Term: "xterm", SSH: true}, false},
	}
	for _, tc := range cases {
		if got := tc.env.LikelySupported(); got != tc.want {
			t.Fatalf("%s: LikelySupported = %v, want %v", tc.name, got, tc.want)
		}
	}
	if !strings.Contains(Environment{Term: "dumb"}.Describe(), "显式配置") {
		t.Fatal("description should explain that an explicit setting still enables Sixel")
	}
}

func TestConfigEnabledIsExplicit(t *testing.T) {
	if !ConfigEnabled(true, " Sixel ", false) {
		t.Fatal("an explicit sixel setting must enable the layer even without environment hints")
	}
	for _, tc := range []struct {
		name     string
		show     bool
		protocol string
		file     bool
	}{
		{"text default", true, "text", false},
		{"unknown protocol", true, "kitty", false},
		{"cover disabled", false, "sixel", false},
		{"redirected file", true, "sixel", true},
	} {
		if ConfigEnabled(tc.show, tc.protocol, tc.file) {
			t.Fatalf("%s should not enable Sixel", tc.name)
		}
	}
}
