// Package sixel encodes images as DEC Sixel graphics (DCS) sequences.
//
// A Sixel is a terminal graphics protocol, not text: the payload is a DCS
// sequence (ESC P q ... ESC \) that the terminal draws outside the character
// grid. It must therefore be sent on a side channel, never embedded in a
// Bubble Tea View() string (see internal/app/output.go).
package sixel

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/anccnuer/go-navidrome-tui/internal/cover"
)

const (
	// DCSStart / DCSEnd wrap every Sixel image.
	DCSStart = "\x1bPq"
	DCSEnd   = "\x1b\\"

	cubeLevels  = 4
	cubeSize    = cubeLevels * cubeLevels * cubeLevels // 64
	graySize    = 8
	paletteSize = cubeSize + graySize
)

var cubeRGB = [cubeLevels]int{0, 85, 170, 255}
var grayRGB = [graySize]int{16, 50, 84, 118, 152, 186, 220, 255}

// paletteColor returns the 0..255 RGB triplet of a palette entry.
func paletteColor(i int) (int, int, int) {
	if i < cubeSize {
		return cubeRGB[i/(cubeLevels*cubeLevels)],
			cubeRGB[(i/cubeLevels)%cubeLevels],
			cubeRGB[i%cubeLevels]
	}
	v := grayRGB[(i-cubeSize)%graySize]
	return v, v, v
}

// colorIndex maps an RGB pixel to the small fixed palette with a cheap O(1)
// quantisation. Encoding runs in a background tea.Cmd, but artwork can be
// several hundred thousand pixels, so a nearest-colour search is too costly.
func colorIndex(r, g, b int) int {
	max, min := r, r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}
	if g < min {
		min = g
	}
	if b < min {
		min = b
	}
	if max-min < 12 {
		return cubeSize + (r+g+b)/3*(graySize-1)/255
	}
	ri := r * (cubeLevels - 1) / 255
	gi := g * (cubeLevels - 1) / 255
	bi := b * (cubeLevels - 1) / 255
	return ri*cubeLevels*cubeLevels + gi*cubeLevels + bi
}

// Encode downscales img to widthPx x heightPx pixels and returns a complete
// Sixel DCS sequence. Each six-pixel band emits one pass per colour actually
// present in that band, which keeps the payload far smaller than a full
// palette sweep. The result is deterministic and safe to cache.
func Encode(img image.Image, widthPx, heightPx int) (string, error) {
	if img == nil {
		return "", fmt.Errorf("sixel: 图片为空")
	}
	if widthPx < 1 || heightPx < 1 {
		return "", fmt.Errorf("sixel: 像素尺寸非法 %dx%d", widthPx, heightPx)
	}
	px := cover.Resize(img, widthPx, heightPx)
	idx := make([]uint8, widthPx*heightPx)
	for y := 0; y < heightPx; y++ {
		for x := 0; x < widthPx; x++ {
			c := px.RGBAAt(x, y)
			idx[y*widthPx+x] = uint8(colorIndex(int(c.R), int(c.G), int(c.B)))
		}
	}

	var b strings.Builder
	// DCS introducer + DECGRA raster attributes (aspect 1:1, pixel W/H).
	b.WriteString(DCSStart)
	fmt.Fprintf(&b, "\"1;1;%d;%d", widthPx, heightPx)
	for i := 0; i < paletteSize; i++ {
		r, g, bl := paletteColor(i)
		// Sixel colour components are percentages (0..100), not 0..255.
		fmt.Fprintf(&b, "#%d;2;%d;%d;%d", i, r*100/255, g*100/255, bl*100/255)
	}

	seen := make([]bool, paletteSize)
	for band := 0; band < heightPx; band += 6 {
		for i := range seen {
			seen[i] = false
		}
		bandEnd := band + 6
		if bandEnd > heightPx {
			bandEnd = heightPx
		}
		for y := band; y < bandEnd; y++ {
			row := idx[y*widthPx : (y+1)*widthPx]
			for _, c := range row {
				seen[c] = true
			}
		}
		for colour, present := range seen {
			if !present {
				continue
			}
			fmt.Fprintf(&b, "#%d", colour)
			for x := 0; x < widthPx; x++ {
				var bits byte
				for dy := 0; dy < 6 && band+dy < heightPx; dy++ {
					if int(idx[(band+dy)*widthPx+x]) == colour {
						bits |= 1 << dy
					}
				}
				// Sixel data is '?' (0x3f) plus the six vertical pixel bits.
				b.WriteByte(0x3f + bits)
			}
			b.WriteByte('$') // carriage return: overlay the next colour
		}
		b.WriteByte('-') // next six-pixel band
	}
	b.WriteString(DCSEnd)
	return b.String(), nil
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
