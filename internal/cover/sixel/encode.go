// Package sixel encodes images as DEC Sixel graphics (DCS) sequences.
//
// A Sixel is a terminal graphics protocol, not text: the payload is a DCS
// sequence (ESC P 0;1;0q ... ESC \) that the terminal draws outside the
// character grid. It must therefore be sent on a side channel, never embedded
// in a Bubble Tea View() string (see internal/cover/sixel/layer.go).
package sixel

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/cover"
)

const (
	// DCSStart follows the DECSIXEL parameter form used by Chafa/libsixel:
	//   P1=0  rely on the raster attributes for aspect ratio
	//   P2=1  keep zero bits transparent (do not repaint the background)
	//   P3=0  horizontal grid size is ignored by modern terminals
	// The bare "ESC P q" shorthand defaults P2=0, so each additional colour
	// pass in a band erases the previous colours with the background colour;
	// Windows Terminal then shows only blank image rows.
	DCSStart = "\x1bP0;1;0q"
	DCSEnd   = "\x1b\\"

	// runThreshold is the shortest identical run worth spending "!n" on.
	runThreshold = 4
)

// EncodeOptions tunes the trade-off between fidelity and payload size.
type EncodeOptions struct {
	// Colors is the palette budget (2..256). 0 means DefaultColors.
	Colors int
	// Dither applies Floyd-Steinberg error diffusion. Without it, smooth
	// gradients band even at 256 colours; with it the same budget looks much
	// closer to the original artwork.
	Dither bool
}

// DefaultOptions is the fidelity-first configuration used by the UI.
func DefaultOptions() EncodeOptions {
	return EncodeOptions{Colors: DefaultColors, Dither: true}
}

func (o EncodeOptions) normalized() EncodeOptions {
	switch {
	case o.Colors <= 0:
		o.Colors = DefaultColors
	case o.Colors < MinColors:
		o.Colors = MinColors
	case o.Colors > MaxColors:
		o.Colors = MaxColors
	}
	return o
}

// Encode downscales img to widthPx x heightPx pixels and returns a complete
// Sixel DCS sequence with default options.
func Encode(img image.Image, widthPx, heightPx int) (string, error) {
	return EncodeWith(img, widthPx, heightPx, DefaultOptions())
}

// EncodeWith is Encode with an explicit palette budget and dithering choice.
//
// The palette is built from the image itself (median cut) instead of a fixed
// 4x4x4 colour cube: a cube clips mid-tones, which is why skin came out green
// and flat areas banded. Each six-pixel band then emits one pass per colour
// actually present in that band, run-length encoded, with trailing transparent
// cells dropped. The result is deterministic and safe to cache.
func EncodeWith(img image.Image, widthPx, heightPx int, opts EncodeOptions) (string, error) {
	if img == nil {
		return "", fmt.Errorf("sixel: 图片为空")
	}
	if widthPx < 1 || heightPx < 1 {
		return "", fmt.Errorf("sixel: 像素尺寸非法 %dx%d", widthPx, heightPx)
	}
	opts = opts.normalized()
	px := cover.Resize(img, widthPx, heightPx)
	palette := buildPalette(px, opts.Colors)
	indices := quantize(px, newQuantizer(palette), opts.Dither)

	var b strings.Builder
	// Per pixel a colour pass emits at most one byte plus RLE, so a generous
	// preallocation avoids repeated growth for the common 200x200 cover.
	b.Grow(widthPx*heightPx/3 + len(palette)*16 + 64)
	// DCS introducer + DECGRA raster attributes (aspect 1:1, pixel W/H).
	b.WriteString(DCSStart)
	fmt.Fprintf(&b, "\"1;1;%d;%d", widthPx, heightPx)
	for i, c := range palette {
		// Sixel colour components are percentages (0..100), not 0..255.
		// Windows Terminal silently renders out-of-range values as black.
		fmt.Fprintf(&b, "#%d;2;%d;%d;%d", i,
			int(c.r)*100/255, int(c.g)*100/255, int(c.b)*100/255)
	}

	present := make([]bool, len(palette))
	pass := make([]byte, widthPx)
	for band := 0; band < heightPx; band += 6 {
		for i := range present {
			present[i] = false
		}
		bandEnd := min(band+6, heightPx)
		for y := band; y < bandEnd; y++ {
			row := indices[y*widthPx : (y+1)*widthPx]
			for _, index := range row {
				present[index] = true
			}
		}
		for colour, used := range present {
			if !used {
				continue
			}
			last := -1
			for x := 0; x < widthPx; x++ {
				var bits byte
				for dy := 0; dy < 6 && band+dy < bandEnd; dy++ {
					if int(indices[(band+dy)*widthPx+x]) == colour {
						bits |= 1 << dy
					}
				}
				pass[x] = bits
				if bits != 0 {
					last = x
				}
			}
			if last < 0 {
				continue
			}
			fmt.Fprintf(&b, "#%d", colour)
			writeRuns(&b, pass[:last+1])
			b.WriteByte('$') // carriage return: overlay the next colour
		}
		b.WriteByte('-') // next six-pixel band
	}
	b.WriteString(DCSEnd)
	return b.String(), nil
}

// writeRuns emits one colour pass as sixel data, collapsing repeated bytes into
// "!count<byte>" RLE. Most of a band is either a long transparent gap or a flat
// colour area, so this is what keeps a 256-colour payload affordable.
func writeRuns(b *strings.Builder, cells []byte) {
	for i := 0; i < len(cells); {
		j := i + 1
		for j < len(cells) && cells[j] == cells[i] {
			j++
		}
		// Sixel data is '?' (0x3f) plus the six vertical pixel bits.
		value := 0x3f + cells[i]
		if run := j - i; run >= runThreshold {
			fmt.Fprintf(b, "!%d", run)
			b.WriteByte(value)
		} else {
			for k := 0; k < run; k++ {
				b.WriteByte(value)
			}
		}
		i = j
	}
}

// GradientImage builds a deterministic test image for --sixel-check, so the
// self-test does not depend on a server or a downloaded cover.
func GradientImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / max(width, 1)),
				G: uint8(y * 255 / max(height, 1)),
				B: 180,
				A: 255,
			})
		}
	}
	return img
}
