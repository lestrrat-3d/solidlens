package solidlens

import (
	"image"
	"image/color"
	"math"
)

// blendPixel composites c with the extra opacity alpha over the opaque pixel
// at (x, y). Components are blended as sRGB bytes.
func blendPixel(img *image.NRGBA, x, y int, c color.NRGBA, alpha float64) {
	if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		return
	}
	a := clamp(alpha*float64(c.A)/255, 0, 1)
	if a == 0 {
		return
	}
	i := img.PixOffset(x, y)
	pix := img.Pix[i : i+4 : i+4]
	pix[0] = uint8(float64(pix[0])*(1-a) + float64(c.R)*a + 0.5)
	pix[1] = uint8(float64(pix[1])*(1-a) + float64(c.G)*a + 0.5)
	pix[2] = uint8(float64(pix[2])*(1-a) + float64(c.B)*a + 0.5)
	pix[3] = 255
}

// paintSection draws every part of the section image in back-to-front order.
func paintSection(img *image.NRGBA, l *sectionLayout, r *sectionRaster) {
	bg := l.background.NRGBA()
	bg.A = 255
	fillImage(img, bg)
	ink := inkColor(bg)
	if !l.view.HideGrid {
		drawGrid(img, l, ink)
	}
	samples := float64(l.supersamp * l.supersamp)
	for i, layer := range l.view.Layers {
		fill := layer.Fill.NRGBA()
		if fill.A == 0 {
			continue
		}
		paintCoverage(img, r.cover[i], samples, fill)
	}
	paintCoverage(img, r.overlap, samples, l.overlap.NRGBA())
	for i, layer := range l.view.Layers {
		outline := layer.Outline.NRGBA()
		if outline.A == 0 {
			continue
		}
		width := layer.OutlineWidth
		if width == 0 {
			width = 1.5
		}
		drawOutline(img, l, i, outline, width)
	}
	drawAnnotations(img, l, ink)
}

func fillImage(img *image.NRGBA, c color.NRGBA) {
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
}

func paintCoverage(img *image.NRGBA, counts []uint8, samples float64, c color.NRGBA) {
	width := img.Rect.Dx()
	for p, n := range counts {
		if n != 0 {
			blendPixel(img, p%width, p/width, c, float64(n)/samples)
		}
	}
}

// inkColor picks black or white text and grid color for the background.
func inkColor(bg color.NRGBA) color.NRGBA {
	luma := 0.2126*float64(bg.R) + 0.7152*float64(bg.G) + 0.0722*float64(bg.B)
	if luma > 140 {
		return color.NRGBA{R: 20, G: 24, B: 32, A: 255}
	}
	return color.NRGBA{R: 235, G: 238, B: 245, A: 255}
}

// drawOutline strokes every loop of a layer with width pixels, anti-aliased by
// the distance from each pixel centre to the nearest segment.
func drawOutline(img *image.NRGBA, l *sectionLayout, layer int, c color.NRGBA, width float64) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	mask := make([]float32, w*h)
	half := width / 2
	reach := half + 1
	for _, loop := range l.loops[layer] {
		for i, from := range loop {
			a, b := l.toPixel(from), l.toPixel(loop[(i+1)%len(loop)])
			x0 := max(int(math.Floor(math.Min(a.x, b.x)-reach)), 0)
			x1 := min(int(math.Ceil(math.Max(a.x, b.x)+reach)), w-1)
			y0 := max(int(math.Floor(math.Min(a.y, b.y)-reach)), 0)
			y1 := min(int(math.Ceil(math.Max(a.y, b.y)+reach)), h-1)
			for y := y0; y <= y1; y++ {
				for x := x0; x <= x1; x++ {
					d := segmentDistance(point2{float64(x) + 0.5, float64(y) + 0.5}, a, b)
					cov := float32(clamp(half+0.5-d, 0, 1))
					if cov > mask[y*w+x] {
						mask[y*w+x] = cov
					}
				}
			}
		}
	}
	for p, cov := range mask {
		if cov > 0 {
			blendPixel(img, p%w, p/w, c, float64(cov))
		}
	}
}

func segmentDistance(p, a, b point2) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	length2 := dx*dx + dy*dy
	t := 0.0
	if length2 > 0 {
		t = clamp(((p.x-a.x)*dx+(p.y-a.y)*dy)/length2, 0, 1)
	}
	return math.Hypot(p.x-(a.x+t*dx), p.y-(a.y+t*dy))
}
