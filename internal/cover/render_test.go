package cover

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/anccnuer/go-navidrome-tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if y < 4 {
				img.Set(x, y, color.RGBA{255, 0, 0, 255})
			} else {
				img.Set(x, y, color.RGBA{0, 0, 255, 255})
			}
		}
	}
	return img
}

func TestRenderTrueColor(t *testing.T) {
	out := Render(testImage(), 4, 2, theme.DepthTrueColor)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got != 4 {
			t.Fatalf("visible width = %d, want 4: %q", got, line)
		}
		if !strings.Contains(line, "▀") || !strings.Contains(line, "38;2;") || !strings.Contains(line, "48;2;") {
			t.Fatalf("missing half block or truecolor codes: %q", line)
		}
	}
	// top half is red, bottom half is blue
	if !strings.Contains(lines[0], "38;2;255;0;0") {
		t.Fatalf("top row should be red: %q", lines[0])
	}
}

func TestRenderDowngradedDepths(t *testing.T) {
	for _, tc := range []struct {
		depth theme.ColorDepth
		want  string
		avoid string
	}{
		{theme.DepthANSI256, "38;5;", "38;2;"},
		{theme.DepthANSI16, "\x1b[31m", "38;"},
	} {
		out := Render(testImage(), 3, 1, tc.depth)
		if !strings.Contains(out, tc.want) || strings.Contains(out, tc.avoid) {
			t.Fatalf("depth %d produced %q", tc.depth, out)
		}
	}
}

func TestDecodeAndResize(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		t.Fatal(err)
	}
	img, err := Decode(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	resized := Resize(img, 4, 4)
	if resized.Bounds().Dx() != 4 || resized.Bounds().Dy() != 4 {
		t.Fatalf("bounds = %v", resized.Bounds())
	}
	if _, err := Decode([]byte("not an image")); err == nil {
		t.Fatal("expected a decode error")
	}
	if d := Dominant(img); d.R == 0 && d.G == 0 && d.B == 0 {
		t.Fatal("dominant colour should not be black for a red/blue image")
	}
}

func TestPlaceholderAndCacheKey(t *testing.T) {
	colors := []string{"#7C5CFF", "#22D3EE", "#F472B6"}
	a := RenderPlaceholder("album-1", 6, 3, theme.DepthTrueColor, colors)
	b := RenderPlaceholder("album-1", 6, 3, theme.DepthTrueColor, colors)
	if a != b {
		t.Fatal("placeholder must be deterministic for the same seed")
	}
	if lines := strings.Split(a, "\n"); len(lines) != 3 {
		t.Fatalf("placeholder lines = %d", len(lines))
	}
	if RenderPlaceholder("album-1", 6, 3, theme.DepthTrueColor, colors) == RenderPlaceholder("album-2", 6, 3, theme.DepthTrueColor, colors) {
		t.Fatal("different seeds should produce different placeholders")
	}
	k1 := CacheKey("ar-1", 20, 10, theme.DepthTrueColor)
	k2 := CacheKey("ar-1", 20, 10, theme.DepthANSI256)
	k3 := CacheKey("ar-1", 21, 10, theme.DepthTrueColor)
	if k1 == k2 || k1 == k3 {
		t.Fatal("cache key must include size and colour depth")
	}
}

func TestParseHex(t *testing.T) {
	if got := ParseHex("#7C5CFF"); got.Hex() != "#7C5CFF" {
		t.Fatalf("parse = %s", got.Hex())
	}
	if got := ParseHex("invalid"); got.Hex() == "#7C5CFF" {
		t.Fatal("invalid hex should fall back to a neutral colour")
	}
}
