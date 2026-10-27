package solidlens

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var sectionFont = sync.OnceValues(func() (*opentype.Font, error) {
	return opentype.Parse(goregular.TTF)
})

// sectionFace returns a text face sized for the image, or nil when the
// bundled font cannot be loaded, in which case annotations are skipped.
func sectionFace(height int) font.Face {
	parsed, err := sectionFont()
	if err != nil {
		return nil
	}
	size := clamp(float64(height)/55, 11, 22)
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil
	}
	return face
}

// niceStep returns the largest 1, 2, or 5 times a power of ten that does not
// exceed target.
func niceStep(target float64) float64 {
	if !(target > 0) || math.IsInf(target, 0) {
		return 1
	}
	exp := math.Pow(10, math.Floor(math.Log10(target)))
	for _, m := range []float64{5, 2, 1} {
		if m*exp <= target {
			return m * exp
		}
	}
	return exp
}

func formatMM(v, step float64) string {
	decimals := 0
	if step < 1 {
		decimals = int(math.Ceil(-math.Log10(step) - 1e-9))
	}
	if v == 0 {
		v = 0 // avoid "-0"
	}
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

func withAlpha(c color.NRGBA, a float64) color.NRGBA {
	c.A = uint8(clamp(a, 0, 1)*255 + 0.5)
	return c
}

// gridStep picks a grid spacing giving roughly eight lines across the longer
// image side.
func gridStep(l *sectionLayout) float64 {
	extent := math.Max(float64(l.width), float64(l.height)) / l.scale
	return niceStep(extent / 8)
}

func drawGrid(img *image.NRGBA, l *sectionLayout, ink color.NRGBA) {
	step := gridStep(l)
	w, h := img.Rect.Dx(), img.Rect.Dy()
	faint := withAlpha(ink, 0.14)
	axis := withAlpha(ink, 0.4)
	for u := math.Ceil(l.window.MinU/step) * step; u <= l.window.MaxU; u += step {
		x := int(math.Floor(l.toPixel(point2{x: u}).x))
		c := faint
		if math.Abs(u) < step/2 {
			c = axis
		}
		for y := range h {
			blendPixel(img, x, y, c, 1)
		}
	}
	for v := math.Ceil(l.window.MinV/step) * step; v <= l.window.MaxV; v += step {
		y := int(math.Floor(l.toPixel(point2{y: v}).y))
		c := faint
		if math.Abs(v) < step/2 {
			c = axis
		}
		for x := range w {
			blendPixel(img, x, y, c, 1)
		}
	}
}

// textBox draws s with its top-left corner at (x, y) and returns its width.
func drawText(img *image.NRGBA, face font.Face, c color.NRGBA, x, y int, s string) int {
	metrics := face.Metrics()
	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y+metrics.Ascent.Ceil()),
	}
	d.DrawString(s)
	return (d.Dot.X - fixed.I(x)).Ceil()
}

func textWidth(face font.Face, s string) int {
	return font.MeasureString(face, s).Ceil()
}

func fillRect(img *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	r = r.Intersect(img.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			blendPixel(img, x, y, c, 1)
		}
	}
}

// drawAnnotations adds the tick labels, scale bar, caption, and legend.
func drawAnnotations(img *image.NRGBA, l *sectionLayout, ink color.NRGBA) {
	face := sectionFace(l.height)
	if face == nil {
		return
	}
	defer face.Close()
	lineHeight := face.Metrics().Height.Ceil()
	if !l.view.HideGrid {
		step := gridStep(l)
		for u := math.Ceil(l.window.MinU/step) * step; u <= l.window.MaxU; u += step {
			x := int(math.Floor(l.toPixel(point2{x: u}).x)) + 3
			drawText(img, face, ink, x, l.height-lineHeight-2, formatMM(u, step))
		}
		for v := math.Ceil(l.window.MinV/step) * step; v <= l.window.MaxV; v += step {
			y := int(math.Floor(l.toPixel(point2{y: v}).y)) - lineHeight - 1
			if y >= 0 {
				drawText(img, face, ink, 4, y, formatMM(v, step))
			}
		}
	}
	if !l.view.HideScaleBar {
		drawScaleBar(img, l, face, ink)
	}
	y := 8
	panel := withAlpha(inkContrast(ink), 0.8)
	if l.view.Caption != "" {
		fillRect(img, image.Rect(4, y-3, 12+textWidth(face, l.view.Caption), y+lineHeight+3), panel)
		drawText(img, face, ink, 8, y, l.view.Caption)
		y += lineHeight + 8
	}
	if !l.view.HideLegend {
		drawLegend(img, l, face, ink, panel, y)
	}
}

// inkContrast returns the opposite of ink, used as the legend panel color.
func inkContrast(ink color.NRGBA) color.NRGBA {
	return color.NRGBA{R: 255 - ink.R, G: 255 - ink.G, B: 255 - ink.B, A: 255}
}

func drawScaleBar(img *image.NRGBA, l *sectionLayout, face font.Face, ink color.NRGBA) {
	length := niceStep(float64(l.width) / l.scale / 5)
	pixels := int(math.Round(length * l.scale))
	label := formatMM(length, length) + " mm"
	lineHeight := face.Metrics().Height.Ceil()
	right := l.width - 16
	left := right - pixels
	y := l.height - 2*lineHeight - 18
	fillRect(img, image.Rect(left-6, y-lineHeight-8, right+6, y+8), withAlpha(inkContrast(ink), 0.8))
	fillRect(img, image.Rect(left, y-1, right, y+2), ink)
	fillRect(img, image.Rect(left, y-5, left+2, y+6), ink)
	fillRect(img, image.Rect(right-2, y-5, right, y+6), ink)
	drawText(img, face, ink, left+(pixels-textWidth(face, label))/2, y-lineHeight-3, label)
}

func drawLegend(img *image.NRGBA, l *sectionLayout, face font.Face, ink, panel color.NRGBA, top int) {
	type entry struct {
		name          string
		fill, outline color.NRGBA
	}
	var entries []entry
	for _, layer := range l.view.Layers {
		entries = append(entries, entry{name: layer.Name, fill: layer.Fill.NRGBA(), outline: layer.Outline.NRGBA()})
	}
	if len(entries) > 1 {
		entries = append(entries, entry{name: "overlap", fill: l.overlap.NRGBA()})
	}
	lineHeight := face.Metrics().Height.Ceil()
	swatch := lineHeight
	rowHeight := lineHeight + 6
	widest := 0
	for _, e := range entries {
		widest = max(widest, textWidth(face, e.name))
	}
	box := image.Rect(4, top-3, 4+swatch+widest+20, top+len(entries)*rowHeight)
	fillRect(img, box, panel)
	for i, e := range entries {
		y := top + i*rowHeight
		sw := image.Rect(8, y, 8+swatch, y+swatch)
		fillRect(img, sw, e.fill)
		if e.outline.A != 0 {
			strokeRect(img, sw, e.outline)
		}
		drawText(img, face, ink, 8+swatch+6, y, e.name)
	}
}

func strokeRect(img *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	fillRect(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), c)
	fillRect(img, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), c)
	fillRect(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), c)
	fillRect(img, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), c)
}
