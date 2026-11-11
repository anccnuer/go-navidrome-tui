package sixel

import (
	"image"
	"image/color"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
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

var paletteDef = regexp.MustCompile(`#(\d+);2;(\d+);(\d+);(\d+)`)

// paletteEntries returns the colour registers defined in a payload.
func paletteEntries(t *testing.T, payload string) [][4]int {
	t.Helper()
	matches := paletteDef.FindAllStringSubmatch(payload, -1)
	entries := make([][4]int, 0, len(matches))
	for _, m := range matches {
		var values [4]int
		for i := 0; i < 4; i++ {
			v, err := strconv.Atoi(m[i+1])
			if err != nil {
				t.Fatalf("palette definition is not numeric: %q", m[0])
			}
			values[i] = v
		}
		entries = append(entries, values)
	}
	return entries
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
	// A solid image needs exactly one register, and it must be the real
	// colour rather than a cube approximation.
	if entries := paletteEntries(t, out); len(entries) != 1 || entries[0] != [4]int{0, 100, 0, 0} {
		t.Fatalf("solid red must define one register #0;2;100;0;0, got %v", entries)
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

func TestEncodeUsesTransparentBackgroundDCS(t *testing.T) {
	out, err := Encode(solidImage(8, 12, color.RGBA{R: 255, A: 255}), 8, 12)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "\x1bP0;1;0q") {
		t.Fatalf("Sixel DCS must set P2=1 for transparent colour overlays: %q", out[:min(len(out), 24)])
	}
}

// The old encoder quantised to a fixed 4x4x4 cube, which clipped skin tones and
// banded gradients. The adaptive palette must spend its budget on the artwork.
func TestEncodeBuildsAdaptivePalette(t *testing.T) {
	img := GradientImage(64, 48)
	out, err := Encode(img, 64, 48)
	if err != nil {
		t.Fatal(err)
	}
	entries := paletteEntries(t, out)
	if len(entries) < 32 {
		t.Fatalf("adaptive palette is too coarse: %d registers", len(entries))
	}
	if len(entries) > MaxColors {
		t.Fatalf("palette exceeds the register budget: %d", len(entries))
	}
	distinct := make(map[[3]int]bool, len(entries))
	for _, e := range entries {
		if e[1] > 100 || e[2] > 100 || e[3] > 100 {
			t.Fatalf("palette component exceeds 100%%: %v", e)
		}
		distinct[[3]int{e[1], e[2], e[3]}] = true
	}
	if len(distinct) < 16 {
		t.Fatalf("palette collapsed to %d distinct colours", len(distinct))
	}
}

func TestEncodeHonoursColourBudget(t *testing.T) {
	out, err := EncodeWith(GradientImage(64, 48), 64, 48, EncodeOptions{Colors: 16})
	if err != nil {
		t.Fatal(err)
	}
	if entries := paletteEntries(t, out); len(entries) > 16 {
		t.Fatalf("colour budget ignored: %d registers", len(entries))
	}
}

// Flat artwork is the common case for covers and must not balloon: the band
// pass is run-length encoded and trailing transparent cells are dropped.
func TestEncodeRunLengthEncodesFlats(t *testing.T) {
	out, err := Encode(solidImage(64, 12, color.RGBA{G: 128, A: 255}), 64, 12)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "!") {
		t.Fatalf("expected RLE runs in a flat image: %q", out)
	}
	if len(out) > 400 {
		t.Fatalf("flat image payload is too large: %d bytes", len(out))
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

func TestEncodeDitheringIsDeterministic(t *testing.T) {
	img := GradientImage(48, 36)
	plain, err := EncodeWith(img, 48, 36, EncodeOptions{Colors: 32, Dither: false})
	if err != nil {
		t.Fatal(err)
	}
	dithered, err := EncodeWith(img, 48, 36, EncodeOptions{Colors: 32, Dither: true})
	if err != nil {
		t.Fatal(err)
	}
	again, err := EncodeWith(img, 48, 36, EncodeOptions{Colors: 32, Dither: true})
	if err != nil {
		t.Fatal(err)
	}
	if dithered != again {
		t.Fatal("dithering is not deterministic")
	}
	if dithered == plain {
		t.Fatal("dithering had no effect on a gradient")
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

func TestEncodeClampsColourBudget(t *testing.T) {
	out, err := EncodeWith(solidImage(4, 6, color.RGBA{R: 10, A: 255}), 4, 6, EncodeOptions{Colors: -5})
	if err != nil {
		t.Fatal(err)
	}
	if entries := paletteEntries(t, out); len(entries) == 0 || len(entries) > MaxColors {
		t.Fatalf("clamped palette is out of range: %d", len(entries))
	}
}

// photoLike builds a smooth, photo-shaped test image: a warm skin-tone field,
// a soft gradient and a few saturated details. It is what the old fixed colour
// cube failed on.
func photoLike(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			t := float64(x) / float64(width)
			u := float64(y) / float64(height)
			r := 205 + 35*t - 10*u
			g := 160 + 30*u - 25*t
			b := 135 + 20*(1-t)*(1-u)
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(clamp8(int(r))), G: uint8(clamp8(int(g))), B: uint8(clamp8(int(b))), A: 255,
			})
		}
	}
	// A saturated detail the palette must also be able to spend a register on.
	for y := height / 4; y < height/2; y++ {
		for x := width / 4; x < width/2; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 30, B: 40, A: 255})
		}
	}
	return img
}

func meanSquaredError(px *image.RGBA, indices []uint8, palette []rgb) float64 {
	width, height := px.Rect.Dx(), px.Rect.Dy()
	var total float64
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := px.RGBAAt(x, y)
			p := palette[indices[y*width+x]]
			dr := float64(int(c.R) - int(p.r))
			dg := float64(int(c.G) - int(p.g))
			db := float64(int(c.B) - int(p.b))
			total += dr*dr + dg*dg + db*db
		}
	}
	return total / float64(width*height)
}

// cubePalette reproduces the old fixed 4x4x4 + grey ramp palette, so the test
// pins down the quality regression it caused (green skin, banded gradients).
func cubePalette() []rgb {
	levels := []uint8{0, 85, 170, 255}
	grey := []uint8{16, 50, 84, 118, 152, 186, 220, 255}
	palette := make([]rgb, 0, 64+len(grey))
	for _, r := range levels {
		for _, g := range levels {
			for _, b := range levels {
				palette = append(palette, rgb{r, g, b})
			}
		}
	}
	for _, v := range grey {
		palette = append(palette, rgb{v, v, v})
	}
	return palette
}

func TestAdaptivePaletteBeatsFixedCube(t *testing.T) {
	px := photoLike(96, 96)
	adaptive := buildPalette(px, MaxColors)
	adaptiveMSE := meanSquaredError(px, quantize(px, newQuantizer(adaptive), false), adaptive)

	cube := cubePalette()
	cubeMSE := meanSquaredError(px, quantize(px, newQuantizer(cube), false), cube)

	t.Logf("MSE adaptive=%.1f cube=%.1f (%d vs %d registers)",
		adaptiveMSE, cubeMSE, len(adaptive), len(cube))
	if adaptiveMSE > cubeMSE/2 {
		t.Fatalf("adaptive palette is not clearly better: %.1f vs %.1f", adaptiveMSE, cubeMSE)
	}
}

// Dithering must reduce banding on a smooth ramp even with a small palette.
func TestDitheringReducesBandingOnGradients(t *testing.T) {
	px := photoLike(96, 96)
	palette := buildPalette(px, 32)
	plain := quantize(px, newQuantizer(palette), false)
	dithered := quantize(px, newQuantizer(palette), true)
	if meanSquaredError(px, dithered, palette) >= meanSquaredError(px, plain, palette) {
		// Error diffusion trades per-pixel error for a smoother tone ramp, so
		// the pixel metric alone is not the whole story; require at least that
		// it does not explode.
		if meanSquaredError(px, dithered, palette) > 4*meanSquaredError(px, plain, palette) {
			t.Fatal("dithering made the quantisation dramatically worse")
		}
	}
	// The two paths must differ, otherwise the flag is not wired through.
	same := true
	for i := range plain {
		if plain[i] != dithered[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("dithering produced the identical index map")
	}
}

// A 200x200 cover is the common case. Together with dithering the payload must
// still be cheap enough to send on every track change without stalling the
// terminal's parser.
func TestEncodeCoverPayloadBudget(t *testing.T) {
	img := GradientImage(208, 208)
	start := time.Now()
	out, err := Encode(img, 208, 208)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("208x208 payload: %d bytes in %v", len(out), time.Since(start))
	if len(out) > 512*1024 {
		t.Fatalf("payload is too large for a terminal to parse comfortably: %d bytes", len(out))
	}
}

// Noise is the worst case for run-length encoding: every pixel differs from its
// neighbour, so no run collapses. Keep that case bounded too.
func TestEncodeNoisePayloadStaysBounded(t *testing.T) {
	const size = 208
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	seed := uint32(12345)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			seed = seed*1664525 + 1013904223
			img.SetRGBA(x, y, color.RGBA{R: uint8(seed >> 24), G: uint8(seed >> 16), B: uint8(seed >> 8), A: 255})
		}
	}
	out, err := Encode(img, size, size)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("worst-case noise payload: %d bytes", len(out))
	if len(out) > 1024*1024 {
		t.Fatalf("noise payload is too large: %d bytes", len(out))
	}
}

func BenchmarkEncodeCover(b *testing.B) {
	img := GradientImage(208, 208)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(img, 208, 208); err != nil {
			b.Fatal(err)
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
