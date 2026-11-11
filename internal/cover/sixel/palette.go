package sixel

import (
	"image"
	"sort"
)

// Colour budget. 256 registers is what libsixel and chafa use, and Windows
// Terminal keeps one register per defined colour, so staying at 256 is safe.
const (
	MinColors     = 2
	MaxColors     = 256
	DefaultColors = 256
)

// rgb is one palette register. Sixel sends components as percentages, so eight
// bits per channel are more than enough.
type rgb struct{ r, g, b uint8 }

type colourCount struct {
	c rgb
	n int
}

func keyOf(c rgb) uint32 { return uint32(c.r)<<16 | uint32(c.g)<<8 | uint32(c.b) }

func lessRGB(a, b rgb) bool { return keyOf(a) < keyOf(b) }

// buildPalette picks at most maxColours representative colours with median cut.
// A photo quantised to the old fixed 4x4x4 cube lost most of its skin tones;
// sampling the artwork itself keeps gradients and faces recognisable.
//
// The result is deterministic — identical pixels always yield the same palette
// — so encoded payloads stay cacheable.
func buildPalette(px *image.RGBA, maxColours int) []rgb {
	if maxColours < MinColors {
		maxColours = MinColors
	}
	if maxColours > MaxColors {
		maxColours = MaxColors
	}
	width, height := px.Rect.Dx(), px.Rect.Dy()
	counts := make(map[uint32]int, 1024)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := px.RGBAAt(x, y)
			counts[keyOf(rgb{c.R, c.G, c.B})]++
		}
	}
	if len(counts) == 0 {
		return []rgb{{0, 0, 0}}
	}

	// Fewer distinct colours than registers: use them all, no approximation.
	if len(counts) <= maxColours {
		palette := make([]rgb, 0, len(counts))
		for key := range counts {
			palette = append(palette, rgb{uint8(key >> 16), uint8(key >> 8), uint8(key)})
		}
		sort.Slice(palette, func(i, j int) bool { return lessRGB(palette[i], palette[j]) })
		return palette
	}

	entries := make([]colourCount, 0, len(counts))
	for key, n := range counts {
		entries = append(entries, colourCount{c: rgb{uint8(key >> 16), uint8(key >> 8), uint8(key)}, n: n})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].c == entries[j].c {
			return entries[i].n > entries[j].n
		}
		return lessRGB(entries[i].c, entries[j].c)
	})

	boxes := []paletteBox{{lo: 0, hi: len(entries)}}
	for len(boxes) < maxColours {
		best, bestScore := -1, 0
		for i, box := range boxes {
			if score := box.score(entries); score > bestScore {
				best, bestScore = i, score
			}
		}
		if best < 0 {
			break
		}
		left, right, ok := boxes[best].split(entries)
		if !ok {
			break
		}
		boxes[best] = left
		boxes = append(boxes, right)
	}

	palette := make([]rgb, 0, len(boxes))
	for _, box := range boxes {
		palette = append(palette, box.average(entries))
	}
	return palette
}

// paletteBox is a contiguous range of the sorted colour list.
type paletteBox struct{ lo, hi int }

func (b paletteBox) count(entries []colourCount) int {
	total := 0
	for i := b.lo; i < b.hi; i++ {
		total += entries[i].n
	}
	return total
}

// score is the box's importance: colour spread multiplied by how many pixels
// it covers. Splitting the most important box first is what keeps small but
// saturated details (lips, eye shadow) from being swallowed by large flat
// areas.
func (b paletteBox) score(entries []colourCount) int {
	if b.hi-b.lo < 2 {
		return 0
	}
	_, spread := b.ranges(entries)
	if spread <= 0 {
		return 0
	}
	return spread * b.count(entries)
}

// ranges returns the widest channel index and its extent.
func (b paletteBox) ranges(entries []colourCount) (channel, extent int) {
	minR, minG, minB := 255, 255, 255
	maxR, maxG, maxB := 0, 0, 0
	for i := b.lo; i < b.hi; i++ {
		c := entries[i].c
		minR, maxR = min(minR, int(c.r)), max(maxR, int(c.r))
		minG, maxG = min(minG, int(c.g)), max(maxG, int(c.g))
		minB, maxB = min(minB, int(c.b)), max(maxB, int(c.b))
	}
	extent, channel = maxR-minR, 0
	if d := maxG - minG; d > extent {
		extent, channel = d, 1
	}
	if d := maxB - minB; d > extent {
		extent, channel = d, 2
	}
	return channel, extent
}

// split divides the box at the pixel-weighted median of its widest channel.
func (b paletteBox) split(entries []colourCount) (paletteBox, paletteBox, bool) {
	if b.hi-b.lo < 2 {
		return b, paletteBox{}, false
	}
	channel, extent := b.ranges(entries)
	if extent == 0 {
		return b, paletteBox{}, false
	}
	segment := entries[b.lo:b.hi]
	sort.SliceStable(segment, func(i, j int) bool {
		return component(segment[i].c, channel) < component(segment[j].c, channel)
	})
	total := 0
	for _, e := range segment {
		total += e.n
	}
	half, running := total/2, 0
	mid := b.lo + 1
	for i, e := range segment {
		running += e.n
		if running > half && i > 0 {
			mid = b.lo + i
			break
		}
	}
	if mid <= b.lo || mid >= b.hi {
		mid = b.lo + (b.hi-b.lo)/2
	}
	return paletteBox{lo: b.lo, hi: mid}, paletteBox{lo: mid, hi: b.hi}, true
}

func (b paletteBox) average(entries []colourCount) rgb {
	var sr, sg, sb, total int
	for i := b.lo; i < b.hi; i++ {
		e := entries[i]
		sr += int(e.c.r) * e.n
		sg += int(e.c.g) * e.n
		sb += int(e.c.b) * e.n
		total += e.n
	}
	if total == 0 {
		return rgb{}
	}
	return rgb{uint8(sr / total), uint8(sg / total), uint8(sb / total)}
}

func component(c rgb, channel int) int {
	switch channel {
	case 0:
		return int(c.r)
	case 1:
		return int(c.g)
	default:
		return int(c.b)
	}
}

// quantizer maps pixels to palette indices.
type quantizer struct{ palette []rgb }

func newQuantizer(palette []rgb) *quantizer { return &quantizer{palette: palette} }

// nearest finds the perceptually closest register. Green is weighted highest
// and blue lowest, which matches how the eye judges skin tones and gradients.
func (q *quantizer) nearest(r, g, b int) int {
	best, bestDistance := 0, 1<<30
	for i, c := range q.palette {
		dr, dg, db := r-int(c.r), g-int(c.g), b-int(c.b)
		distance := 3*dr*dr + 4*dg*dg + 2*db*db
		if distance < bestDistance {
			best, bestDistance = i, distance
			if distance == 0 {
				return i
			}
		}
	}
	return best
}

// quantize maps every pixel to a palette index, optionally with Floyd-Steinberg
// error diffusion. Without dithering a 256-colour palette still bands on soft
// gradients; with it the same budget looks smooth.
func quantize(px *image.RGBA, q *quantizer, dither bool) []uint8 {
	width, height := px.Rect.Dx(), px.Rect.Dy()
	indices := make([]uint8, width*height)
	if !dither {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				c := px.RGBAAt(x, y)
				indices[y*width+x] = uint8(q.nearest(int(c.R), int(c.G), int(c.B)))
			}
		}
		return indices
	}

	// Errors are kept scaled by 16 so the diffusion stays integer-exact and
	// deterministic across platforms.
	current := make([]int32, width*3)
	line := make([]int32, width*3)
	for y := 0; y < height; y++ {
		for i := range line {
			line[i] = 0
		}
		leftToRight := y%2 == 0
		direction := 1
		if !leftToRight {
			direction = -1
		}
		for step := 0; step < width; step++ {
			x := step
			if !leftToRight {
				x = width - 1 - step
			}
			c := px.RGBAAt(x, y)
			r := clamp8(int(c.R) + int(current[x*3])/16)
			g := clamp8(int(c.G) + int(current[x*3+1])/16)
			b := clamp8(int(c.B) + int(current[x*3+2])/16)
			index := q.nearest(r, g, b)
			indices[y*width+x] = uint8(index)
			target := q.palette[index]
			diffuse(current, line, width, x, direction,
				r-int(target.r), g-int(target.g), b-int(target.b))
		}
		current, line = line, current
	}
	return indices
}

// diffuse spreads the quantisation error to the neighbouring pixels using the
// classic Floyd-Steinberg weights (7/16 right, 3/16 and 5/16 below, 1/16
// below-right), mirrored on right-to-left rows.
func diffuse(current, line []int32, width, x, direction, er, eg, eb int) {
	add := func(buf []int32, at, weight int) {
		buf[at*3] += int32(er * weight)
		buf[at*3+1] += int32(eg * weight)
		buf[at*3+2] += int32(eb * weight)
	}
	if next := x + direction; next >= 0 && next < width {
		add(current, next, 7)
		add(line, next, 1)
	}
	add(line, x, 5)
	if previous := x - direction; previous >= 0 && previous < width {
		add(line, previous, 3)
	}
}

func clamp8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
