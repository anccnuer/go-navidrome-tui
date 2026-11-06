package cover

import (
	"fmt"
	"hash/fnv"
	"image"
	"strings"

	"github.com/subsonic-tui/subsonic-tui/internal/theme"
)

// UpperHalfBlock renders two vertical pixels in one character cell: the
// foreground colour is the upper pixel, the background the lower one.
const UpperHalfBlock = "▀"

// CacheKey identifies a rendered cover string. The UI caches these strings so
// View() never decodes an image on the hot path.
func CacheKey(id string, width, height int, depth theme.ColorDepth) string {
	return fmt.Sprintf("%s:%d:%d:%d", id, width, height, depth)
}

// Render downscales artwork to width x (height*2) pixels and emits `height`
// lines of half-block characters.
func Render(img image.Image, width, height int, depth theme.ColorDepth) string {
	if img == nil || width < 1 || height < 1 {
		return ""
	}
	pixels := Resize(img, width, height*2)
	lines := make([]string, 0, height)
	for y := 0; y < height; y++ {
		top := make([]RGB, width)
		bottom := make([]RGB, width)
		for x := 0; x < width; x++ {
			c1 := pixels.RGBAAt(x, y*2)
			c2 := pixels.RGBAAt(x, y*2+1)
			top[x] = RGB{int(c1.R), int(c1.G), int(c1.B)}
			bottom[x] = RGB{int(c2.R), int(c2.G), int(c2.B)}
		}
		lines = append(lines, renderLine(top, bottom, depth))
	}
	return strings.Join(lines, "\n")
}

func renderLine(top, bottom []RGB, depth theme.ColorDepth) string {
	var b strings.Builder
	b.Grow(len(top) * 24)
	for i := range top {
		b.WriteString(fgANSI(top[i], depth))
		b.WriteString(bgANSI(bottom[i], depth))
		b.WriteString(UpperHalfBlock)
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

func fgANSI(c RGB, depth theme.ColorDepth) string {
	switch depth {
	case theme.DepthTrueColor:
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
	case theme.DepthANSI256:
		return fmt.Sprintf("\x1b[38;5;%dm", to256(c))
	default:
		return fmt.Sprintf("\x1b[%dm", 30+to16(c))
	}
}

func bgANSI(c RGB, depth theme.ColorDepth) string {
	switch depth {
	case theme.DepthTrueColor:
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
	case theme.DepthANSI256:
		return fmt.Sprintf("\x1b[48;5;%dm", to256(c))
	default:
		return fmt.Sprintf("\x1b[%dm", 40+to16(c))
	}
}

func to256(c RGB) int {
	r := clamp(c.R*6/256, 0, 5)
	g := clamp(c.G*6/256, 0, 5)
	b := clamp(c.B*6/256, 0, 5)
	// Use the grayscale ramp when the colour is nearly neutral.
	max := c.R
	if c.G > max {
		max = c.G
	}
	if c.B > max {
		max = c.B
	}
	min := c.R
	if c.G < min {
		min = c.G
	}
	if c.B < min {
		min = c.B
	}
	if max-min < 10 {
		gray := clamp((c.R+c.G+c.B)/3, 0, 255)
		if gray < 8 {
			return 16
		}
		if gray > 248 {
			return 231
		}
		return 232 + (gray-8)*24/248
	}
	return 16 + 36*r + 6*g + b
}

var ansi16 = []RGB{
	{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
	{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
}

func to16(c RGB) int {
	best, bestDist := 0, int(^uint(0)>>1)
	for i, candidate := range ansi16 {
		dr, dg, db := c.R-candidate.R, c.G-candidate.G, c.B-candidate.B
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			best, bestDist = i, dist
		}
	}
	return best
}

func blend(a, b RGB, f float64) RGB {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return RGB{
		R: clamp(int(float64(a.R)+(float64(b.R)-float64(a.R))*f), 0, 255),
		G: clamp(int(float64(a.G)+(float64(b.G)-float64(a.G))*f), 0, 255),
		B: clamp(int(float64(a.B)+(float64(b.B)-float64(a.B))*f), 0, 255),
	}
}

// RenderPlaceholder draws a deterministic gradient tile when a track has no
// artwork, keeping the layout stable instead of showing an empty box.
func RenderPlaceholder(seed string, width, height int, depth theme.ColorDepth, colors []string) string {
	if width < 1 || height < 1 {
		return ""
	}
	if len(colors) == 0 {
		colors = []string{"#7C5CFF", "#22D3EE"}
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	v := int(h.Sum32())
	start := ParseHex(colors[v%len(colors)])
	end := ParseHex(colors[(v/7+1)%len(colors)])
	lines := make([]string, 0, height)
	for y := 0; y < height; y++ {
		top := make([]RGB, width)
		bottom := make([]RGB, width)
		for x := 0; x < width; x++ {
			f := float64(x)/float64(width)*0.65 + float64(y)/float64(height)*0.35
			top[x] = blend(start, end, f)
			bottom[x] = blend(end, start, f*0.8+0.1)
		}
		lines = append(lines, renderLine(top, bottom, depth))
	}
	return strings.Join(lines, "\n")
}
