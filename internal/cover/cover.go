// Package cover decodes Navidrome artwork and renders it as Unicode half
// blocks with ANSI colours, so it works in every terminal (no image protocol
// needed). Results are cached by the UI layer using CacheKey.
package cover

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strconv"
	"strings"
)

// RGB is a 24-bit colour.
type RGB struct{ R, G, B int }

// Hex returns the #RRGGBB form.
func (c RGB) Hex() string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ParseHex accepts #RRGGBB and RRGGBB, falling back to a neutral grey.
func ParseHex(s string) RGB {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return RGB{90, 99, 128}
	}
	r, errR := strconv.ParseUint(s[0:2], 16, 8)
	g, errG := strconv.ParseUint(s[2:4], 16, 8)
	b, errB := strconv.ParseUint(s[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return RGB{90, 99, 128}
	}
	return RGB{int(r), int(g), int(b)}
}

// Decode reads JPEG/PNG/GIF artwork bytes.
func Decode(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("封面数据为空")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("无法解码封面图片: %w", err)
	}
	return img, nil
}

// Resize performs a bounded area-average downsample. Bounding the sample grid
// keeps the cost linear in the destination size even for huge artwork.
func Resize(img image.Image, width, height int) *image.RGBA {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	bounds := img.Bounds()
	if bounds.Empty() {
		return out
	}
	sw, sh := bounds.Dx(), bounds.Dy()
	for y := 0; y < height; y++ {
		y0 := bounds.Min.Y + y*sh/height
		y1 := bounds.Min.Y + (y+1)*sh/height
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < width; x++ {
			x0 := bounds.Min.X + x*sw/width
			x1 := bounds.Min.X + (x+1)*sw/width
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sr, sg, sb uint64
			count := 0
			stepX := (x1 - x0) / 6
			stepY := (y1 - y0) / 6
			if stepX < 1 {
				stepX = 1
			}
			if stepY < 1 {
				stepY = 1
			}
			for sy := y0; sy < y1 && sy < bounds.Max.Y; sy += stepY {
				for sx := x0; sx < x1 && sx < bounds.Max.X; sx += stepX {
					r, g, b, _ := img.At(sx, sy).RGBA()
					sr += uint64(r >> 8)
					sg += uint64(g >> 8)
					sb += uint64(b >> 8)
					count++
				}
			}
			if count == 0 {
				continue
			}
			out.SetRGBA(x, y, color.RGBA{
				R: uint8(clamp(int(sr/uint64(count)), 0, 255)),
				G: uint8(clamp(int(sg/uint64(count)), 0, 255)),
				B: uint8(clamp(int(sb/uint64(count)), 0, 255)),
				A: 255,
			})
		}
	}
	return out
}

// Dominant returns the average colour of a small downsample; used for accent
// colours when no artwork is available.
func Dominant(img image.Image) RGB {
	small := Resize(img, 8, 8)
	var r, g, b int
	n := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			c := small.RGBAAt(x, y)
			r += int(c.R)
			g += int(c.G)
			b += int(c.B)
			n++
		}
	}
	if n == 0 {
		return RGB{90, 99, 128}
	}
	return RGB{r / n, g / n, b / n}
}
