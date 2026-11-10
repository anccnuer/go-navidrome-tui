package sixel

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func solidImage(width, height int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestEncodeProducesSixelDCS(t *testing.T) {
	out, err := Encode(solidImage(8, 12, color.RGBA{R: 255, A: 255}), 8, 12)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, DCSStart) || !strings.HasSuffix(out, DCSEnd) {
		t.Fatalf("missing DCS wrapper: %q", out[:min(len(out), 40)])
	}
	if !strings.Contains(out, `"1;1;8;12`) {
		t.Fatal("raster attributes are missing or wrong")
	}
	if !strings.Contains(out, "#0;2;0;0;0") {
		t.Fatal("palette definition is missing")
	}
	if strings.ContainsAny(out, "\r\n") {
		t.Fatal("sixel payload must not contain line breaks")
	}
	for i, c := range out {
		if c == 0x1b {
			continue
		}
		if c < 0x20 || c > 0x7e {
			t.Fatalf("non-printable byte %#x at offset %d", c, i)
		}
	}
}

func TestEncodeIsDeterministicAndColourSensitive(t *testing.T) {
	red := solidImage(16, 18, color.RGBA{R: 255, A: 255})
	blue := solidImage(16, 18, color.RGBA{B: 255, A: 255})
	a, err := Encode(red, 16, 18)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Encode(red, 16, 18)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("encoding is not deterministic")
	}
	c, err := Encode(blue, 16, 18)
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("different colours produced identical payloads")
	}
}

func TestEncodeRejectsInvalidInput(t *testing.T) {
	if _, err := Encode(nil, 8, 8); err == nil {
		t.Fatal("nil image should be rejected")
	}
	if _, err := Encode(solidImage(4, 4, color.RGBA{}), 0, 4); err == nil {
		t.Fatal("zero width should be rejected")
	}
	if _, err := Encode(solidImage(4, 4, color.RGBA{}), 4, -1); err == nil {
		t.Fatal("negative height should be rejected")
	}
}

func TestColorIndexStaysWithinPalette(t *testing.T) {
	for r := 0; r < 256; r += 17 {
		for g := 0; g < 256; g += 17 {
			for b := 0; b < 256; b += 17 {
				if i := colorIndex(r, g, b); i < 0 || i >= paletteSize {
					t.Fatalf("colorIndex(%d,%d,%d) = %d, palette size %d", r, g, b, i, paletteSize)
				}
			}
		}
	}
}

func TestGradientImageEncodes(t *testing.T) {
	img := GradientImage(40, 24)
	out, err := Encode(img, 40, 24)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, DCSStart) || !strings.HasSuffix(out, DCSEnd) {
		t.Fatal("self-check image did not encode")
	}
}
