package solidlens

import (
	"image"
	"image/color"
	"slices"
)

// Colours are counted and looked up with paletteBits bits per channel.
const (
	paletteBits  = 6
	paletteShift = 8 - paletteBits
	paletteCells = 1 << (3 * paletteBits)
	paletteSize  = 256
)

// paletteBin is one cell of a coarse colour histogram.
type paletteBin struct {
	rgb   [3]int // channel values at the cell's centre, 0-255
	count int
}

// paletteBox is a group of bins that median cut may split.
type paletteBox struct{ bins []paletteBin }

func (b paletteBox) total() int {
	n := 0
	for _, bin := range b.bins {
		n += bin.count
	}
	return n
}

// widestChannel returns the channel with the largest spread, and that spread.
func (b paletteBox) widestChannel() (int, int) {
	best, spread := 0, -1
	for c := range 3 {
		lo, hi := 255, 0
		for _, bin := range b.bins {
			lo, hi = min(lo, bin.rgb[c]), max(hi, bin.rgb[c])
		}
		if hi-lo > spread {
			best, spread = c, hi-lo
		}
	}
	return best, spread
}

// average returns the colour of the box, weighted by how often each bin
// occurs.
func (b paletteBox) average() color.RGBA {
	var sum [3]int
	for _, bin := range b.bins {
		for c := range 3 {
			sum[c] += bin.rgb[c] * bin.count
		}
	}
	n := b.total()
	return color.RGBA{R: uint8(sum[0] / n), G: uint8(sum[1] / n), B: uint8(sum[2] / n), A: 255}
}

// split cuts the box in two at the weighted median of its widest channel. It
// needs at least two bins and returns two non-empty boxes.
func (b paletteBox) split() (paletteBox, paletteBox) {
	channel, _ := b.widestChannel()
	bins := slices.Clone(b.bins)
	slices.SortStableFunc(bins, func(x, y paletteBin) int { return x.rgb[channel] - y.rgb[channel] })
	half, run, cut := b.total()/2, 0, 1
	for i, bin := range bins {
		run += bin.count
		if run >= half {
			cut = min(max(i+1, 1), len(bins)-1)
			break
		}
	}
	return paletteBox{bins: bins[:cut]}, paletteBox{bins: bins[cut:]}
}

// cellCenter returns the 8-bit channel value at the centre of a histogram cell.
func cellCenter(v int) int { return v<<paletteShift | (1<<paletteShift)/2 }

// buildPalette picks up to 256 colours for the images by median cut over a
// histogram of their pixels. The result is the same for the same input.
func buildPalette(images []*image.RGBA) color.Palette {
	counts := make([]int, paletteCells)
	for _, img := range images {
		for p := 0; p+3 < len(img.Pix); p += 4 {
			counts[cellKey(img.Pix[p], img.Pix[p+1], img.Pix[p+2])]++
		}
	}
	var bins []paletteBin
	for key, n := range counts {
		if n == 0 {
			continue
		}
		mask := 1<<paletteBits - 1
		bins = append(bins, paletteBin{
			rgb: [3]int{
				cellCenter(key >> (2 * paletteBits)),
				cellCenter(key >> paletteBits & mask),
				cellCenter(key & mask),
			},
			count: n,
		})
	}

	boxes := []paletteBox{{bins: bins}}
	for len(boxes) < paletteSize {
		pick, best := -1, 0
		for i, b := range boxes {
			if len(b.bins) < 2 {
				continue
			}
			_, spread := b.widestChannel()
			if score := spread * b.total(); score > best {
				pick, best = i, score
			}
		}
		if pick < 0 {
			break
		}
		low, high := boxes[pick].split()
		boxes[pick] = low
		boxes = append(boxes, high)
	}

	palette := make(color.Palette, len(boxes))
	for i, b := range boxes {
		palette[i] = b.average()
	}
	return palette
}

// cellKey returns the histogram cell of an 8-bit colour.
func cellKey(r, g, b uint8) int {
	return int(r>>paletteShift)<<(2*paletteBits) | int(g>>paletteShift)<<paletteBits | int(b>>paletteShift)
}

// paletteTable maps a colour to the index of the nearest palette entry. It is
// built once and read-only afterwards, so frames can use it concurrently.
type paletteTable [paletteCells]uint8

func newPaletteTable(palette color.Palette) *paletteTable {
	var table paletteTable
	mask := 1<<paletteBits - 1
	for key := range table {
		c := color.RGBA{
			R: uint8(cellCenter(key >> (2 * paletteBits))),
			G: uint8(cellCenter(key >> paletteBits & mask)),
			B: uint8(cellCenter(key & mask)),
			A: 255,
		}
		table[key] = uint8(palette.Index(c))
	}
	return &table
}

func (t *paletteTable) index(r, g, b uint8) uint8 { return t[cellKey(r, g, b)] }
