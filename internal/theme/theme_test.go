package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTextStylesHaveNoBackground(t *testing.T) {
	s := New(Builtin("musicfox"), DepthTrueColor)
	styles := map[string]lipgloss.Style{
		"App": s.App, "Panel": s.Panel, "Row": s.Row, "AltRow": s.AltRow,
		"Selected": s.Selected, "StatusBar": s.StatusBar, "NowBar": s.NowBar,
		"Badge": s.Badge, "BadgeActive": s.BadgeActive, "KeyCap": s.KeyCap,
		"Chip": s.Chip, "ChipActive": s.ChipActive, "Input": s.Input,
		"InputFocus": s.InputFocus, "PanelTitle": s.PanelTitle,
		"PanelTitleDim": s.PanelTitleDim, "Zebra": s.Zebra,
		"ToastOK":   s.ToastOK,
		"ToastWarn": s.ToastWarn, "ToastErr": s.ToastErr, "ToastInfo": s.ToastInfo,
	}
	for name, st := range styles {
		if _, ok := st.GetBackground().(lipgloss.NoColor); !ok {
			t.Fatalf("%s still paints a background slab: %v", name, st.GetBackground())
		}
	}
}

func TestBuiltinPalettes(t *testing.T) {
	for _, name := range Names() {
		p := Builtin(name)
		if p.Name != name || len(p.Gradient) < 2 || p.Background == "" || p.Primary == "" {
			t.Fatalf("palette %s incomplete: %+v", name, p)
		}
	}
	if got := Next("nord"); got != "gruvbox" {
		t.Fatalf("Next(nord) = %s", got)
	}
	if got := Next("mono"); got != Names()[0] {
		t.Fatalf("Next(mono) = %s, want %s", got, Names()[0])
	}
}

func TestLoadCustomMergesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.toml")
	content := "name = \"mine\"\nprimary = \"#FF0000\"\ngradient = [\"#FF0000\", \"#00FF00\"]\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadCustom(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "mine" || p.Primary != "#FF0000" {
		t.Fatalf("custom values lost: %+v", p)
	}
	if p.Background != Builtin("aurora").Background || len(p.Gradient) != 2 {
		t.Fatalf("defaults lost: %+v", p)
	}
}

func TestResolveFallsBackOnBrokenFile(t *testing.T) {
	p, err := Resolve("nord", filepath.Join(t.TempDir(), "missing.toml"))
	if err == nil {
		t.Fatal("expected an error for a missing custom theme")
	}
	if p.Name != "nord" {
		t.Fatalf("fallback = %s", p.Name)
	}
}

func TestGradientAndProgressBar(t *testing.T) {
	s := New(Builtin("aurora"), DepthTrueColor)
	styled := s.Gradient("SUBSONIC TUI")
	if lipgloss.Width(styled) != len("SUBSONIC TUI") {
		t.Fatalf("gradient width = %d", lipgloss.Width(styled))
	}
	if styled == "" {
		t.Fatal("gradient text is empty")
	}
	// Colour codes are only emitted when the detected terminal profile supports
	// colour; in CI it may be Ascii.
	bar := s.ProgressBar(20, 0.5)
	if lipgloss.Width(bar) != 20 {
		t.Fatalf("bar width = %d, want 20", lipgloss.Width(bar))
	}
	bar2 := s.Gradient("SUBSONIC TUI")
	if bar2 != styled {
		t.Fatal("gradient cache returned a different result")
	}
	if got := lipgloss.Width(s.ProgressBar(20, 0)); got != 20 {
		t.Fatalf("empty bar width = %d", got)
	}
}

func TestInterpolate(t *testing.T) {
	colors := []string{"#000000", "#FFFFFF"}
	if got := interpolate(colors, 0); got != "#000000" {
		t.Fatalf("start = %s", got)
	}
	if got := interpolate(colors, 1); got != "#FFFFFF" {
		t.Fatalf("end = %s", got)
	}
	mid := interpolate(colors, 0.5)
	if mid == "#000000" || mid == "#FFFFFF" {
		t.Fatalf("midpoint = %s", mid)
	}
}

func TestDetectDepth(t *testing.T) {
	// Just make sure detection never panics in a non-TTY test environment.
	if d := DetectDepth(false); d < DepthTrueColor || d > DepthANSI16 {
		t.Fatalf("unexpected depth %d", d)
	}
}
