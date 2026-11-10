package sixel

import (
	"bytes"
	"strings"
	"testing"
)

func TestSelfCheckWritesEnvironmentAndImage(t *testing.T) {
	var buf bytes.Buffer
	if err := SelfCheck(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Sixel 自检", "WT_SESSION", "TERM", DCSStart, DCSEnd, `"1;1;240;120`} {
		if !strings.Contains(out, want) {
			t.Fatalf("self-check output is missing %q", want)
		}
	}
}

func TestLikelySupportedRejectsDumbAndRemoteSessions(t *testing.T) {
	cases := []struct {
		name string
		env  Environment
		want bool
	}{
		{"windows terminal", Environment{WTSession: true, Term: "xterm-256color"}, true},
		{"xterm", Environment{Term: "xterm-256color"}, true},
		{"dumb", Environment{WTSession: true, Term: "dumb"}, false},
		{"tmux", Environment{WTSession: true, Term: "screen-256color", TMUX: true}, false},
		{"ssh", Environment{WTSession: true, Term: "xterm", SSH: true}, false},
	}
	for _, tc := range cases {
		if got := tc.env.LikelySupported(); got != tc.want {
			t.Fatalf("%s: LikelySupported = %v, want %v", tc.name, got, tc.want)
		}
	}
	if !strings.Contains(Environment{Term: "dumb"}.Describe(), "纯文本") {
		t.Fatal("description should recommend the text fallback")
	}
}
