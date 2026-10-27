package solidlens

import (
	"context"
	"math"
	"slices"
)

// sectionRaster holds the supersampled coverage of every layer. cover[i][p]
// counts how many of the supersamples inside pixel p fall in layer i, and
// overlap[p] counts those covered by two or more layers.
type sectionRaster struct {
	cover          [][]uint8
	overlap        []uint8
	layerSamples   []int64
	overlapSamples int64
}

type crossing struct {
	x    float64
	wind int
}

// rasterizeSection fills the coverage counts by scanning the supersampled
// image row by row with the non-zero winding rule.
func rasterizeSection(ctx context.Context, l *sectionLayout) (*sectionRaster, error) {
	layers := len(l.loops)
	rows := l.height * l.supersamp
	cols := l.width * l.supersamp
	raster := &sectionRaster{
		cover:        make([][]uint8, layers),
		overlap:      make([]uint8, l.width*l.height),
		layerSamples: make([]int64, layers),
	}
	crossings := make([][][]crossing, layers)
	for i := range layers {
		raster.cover[i] = make([]uint8, l.width*l.height)
		crossings[i] = l.collectCrossings(i, rows)
	}
	spans := make([][]bool, layers)
	for i := range spans {
		spans[i] = make([]bool, cols)
	}
	for row := range rows {
		if row%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		for i := range layers {
			fillSpans(spans[i], crossings[i][row])
		}
		pixelRow := (row / l.supersamp) * l.width
		for col := range cols {
			covered := 0
			pixel := pixelRow + col/l.supersamp
			for i := range layers {
				if spans[i][col] {
					covered++
					raster.cover[i][pixel]++
					raster.layerSamples[i]++
				}
			}
			if covered >= 2 {
				raster.overlap[pixel]++
				raster.overlapSamples++
			}
		}
	}
	return raster, nil
}

// collectCrossings buckets the edge crossings of one layer by supersample row.
// A row's sample line sits at the row centre.
func (l *sectionLayout) collectCrossings(layer, rows int) [][]crossing {
	buckets := make([][]crossing, rows)
	ss := float64(l.supersamp)
	for _, loop := range l.loops[layer] {
		for i, from := range loop {
			to := loop[(i+1)%len(loop)]
			a, b := l.toPixel(from), l.toPixel(to)
			a, b = point2{a.x * ss, a.y * ss}, point2{b.x * ss, b.y * ss}
			if a.y == b.y {
				continue
			}
			wind := 1
			if a.y > b.y {
				a, b = b, a
				wind = -1
			}
			// Rows whose centre y+0.5 lies in [a.y, b.y).
			first := max(int(math.Ceil(a.y-0.5)), 0)
			last := min(int(math.Ceil(b.y-0.5))-1, rows-1)
			slope := (b.x - a.x) / (b.y - a.y)
			for row := first; row <= last; row++ {
				x := a.x + (float64(row)+0.5-a.y)*slope
				buckets[row] = append(buckets[row], crossing{x: x, wind: wind})
			}
		}
	}
	return buckets
}

// fillSpans sets span[c] for every sample column c whose centre is inside the
// non-zero winding region described by the crossings.
func fillSpans(span []bool, crossings []crossing) {
	clear(span)
	if len(crossings) == 0 {
		return
	}
	slices.SortFunc(crossings, func(a, b crossing) int {
		switch {
		case a.x < b.x:
			return -1
		case a.x > b.x:
			return 1
		}
		return 0
	})
	winding := 0
	for i, c := range crossings[:len(crossings)-1] {
		winding += c.wind
		if winding == 0 {
			continue
		}
		first := max(int(math.Ceil(c.x-0.5)), 0)
		last := min(int(math.Ceil(crossings[i+1].x-0.5))-1, len(span)-1)
		for col := first; col <= last; col++ {
			span[col] = true
		}
	}
}
