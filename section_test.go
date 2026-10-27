package solidlens_test

import (
	"context"
	"image/color"
	"math"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

func xyPlane() solidlens.SectionPlane {
	return solidlens.SectionPlane{U: solidlens.Vec{X: 1}, V: solidlens.Vec{Y: 1}}
}

func rect(x0, y0, x1, y1 float64) []solidlens.Vec {
	return []solidlens.Vec{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

func sectionPixel(t *testing.T, want solidlens.Color) color.NRGBA {
	t.Helper()
	return want.NRGBA()
}

// twoRectangles returns a 400x200 view at 20 px/mm: A covers u in [0, 10],
// B covers u in [5, 15], and both span v in [0, 10].
func twoRectangles() solidlens.SectionView {
	return solidlens.SectionView{
		Plane: xyPlane(),
		Layers: []solidlens.SectionLayer{
			{Name: "A", Loops: [][]solidlens.Vec{rect(0, 0, 10, 10)}, Fill: solidlens.RGB(0, 0, 1)},
			{Name: "B", Loops: [][]solidlens.Vec{rect(5, 0, 15, 10)}, Fill: solidlens.RGB(0, 1, 0)},
		},
		Window:       &solidlens.SectionWindow{MinU: -2.5, MinV: 0, MaxU: 17.5, MaxV: 10},
		Width:        400,
		Height:       200,
		Background:   solidlens.RGB(1, 1, 1),
		OverlapColor: solidlens.RGB(1, 1, 0),
		HideGrid:     true,
		HideScaleBar: true,
		HideLegend:   true,
	}
}

func TestRenderSectionOverlap(t *testing.T) {
	view := twoRectangles()
	img, report, err := solidlens.RenderSection(t.Context(), view)
	require.NoError(t, err)
	require.Equal(t, 400, img.Bounds().Dx())

	// u = -2.5 + x/20; every probe sits well inside its region.
	pixelAt := func(u float64) color.NRGBA { return img.NRGBAAt(int((u+2.5)*20), 100) }
	require.Equal(t, sectionPixel(t, solidlens.RGB(1, 1, 1)), pixelAt(-1), "background")
	require.Equal(t, sectionPixel(t, solidlens.RGB(0, 0, 1)), pixelAt(2), "A only")
	require.Equal(t, sectionPixel(t, solidlens.RGB(1, 1, 0)), pixelAt(7), "overlap")
	require.Equal(t, sectionPixel(t, solidlens.RGB(0, 1, 0)), pixelAt(12), "B only")
	require.Equal(t, sectionPixel(t, solidlens.RGB(1, 1, 1)), pixelAt(16), "background right")

	require.Len(t, report.Layers, 2)
	require.InDelta(t, 100, report.Layers[0].Area, 1e-9)
	require.InDelta(t, 100, report.Layers[1].Area, 1e-9)
	require.InDelta(t, 100, report.Layers[0].RasterArea, 1)
	require.InDelta(t, 50, report.OverlapArea, 1)
	require.InDelta(t, 0.05, report.PixelSize, 1e-12)
	require.InDelta(t, 0.0125, report.SampleSize, 1e-12)
}

func TestRenderSectionHole(t *testing.T) {
	// The hole runs clockwise, so it subtracts from the outer loop.
	hole := []solidlens.Vec{{X: 3, Y: 3}, {X: 3, Y: 7}, {X: 7, Y: 7}, {X: 7, Y: 3}}
	view := solidlens.SectionView{
		Plane: xyPlane(),
		Layers: []solidlens.SectionLayer{{
			Name:  "ring",
			Loops: [][]solidlens.Vec{rect(0, 0, 10, 10), hole},
			Fill:  solidlens.RGB(1, 0, 0),
		}},
		Window:       &solidlens.SectionWindow{MinU: 0, MinV: 0, MaxU: 10, MaxV: 10},
		Width:        200,
		Height:       200,
		Background:   solidlens.RGB(1, 1, 1),
		HideGrid:     true,
		HideScaleBar: true,
		HideLegend:   true,
	}
	img, report, err := solidlens.RenderSection(t.Context(), view)
	require.NoError(t, err)
	require.Equal(t, sectionPixel(t, solidlens.RGB(1, 1, 1)), img.NRGBAAt(100, 100), "hole shows the background")
	require.Equal(t, sectionPixel(t, solidlens.RGB(1, 0, 0)), img.NRGBAAt(20, 100), "ring is filled")
	require.InDelta(t, 84, report.Layers[0].Area, 1e-9)
	require.InDelta(t, 84, report.Layers[0].RasterArea, 0.5)
	require.Zero(t, report.OverlapArea)
}

func TestRenderSectionAutoFitAndAnnotations(t *testing.T) {
	view := twoRectangles()
	view.Window = nil
	view.Margin = 1
	view.HideGrid, view.HideScaleBar, view.HideLegend = false, false, false
	view.Caption = "test section"
	img, report, err := solidlens.RenderSection(t.Context(), view)
	require.NoError(t, err)
	require.LessOrEqual(t, report.Window.MinU, -1.0)
	require.GreaterOrEqual(t, report.Window.MaxU, 16.0)
	require.LessOrEqual(t, report.Window.MinV, -1.0)
	require.GreaterOrEqual(t, report.Window.MaxV, 11.0)
	require.InDelta(t, 7.5, (report.Window.MinU+report.Window.MaxU)/2, 1e-9)
	require.InDelta(t, 5, (report.Window.MinV+report.Window.MaxV)/2, 1e-9)

	// The annotations add pixels that are neither background nor layer colors.
	other := 0
	known := map[color.NRGBA]bool{
		sectionPixel(t, solidlens.RGB(1, 1, 1)): true,
		sectionPixel(t, solidlens.RGB(0, 0, 1)): true,
		sectionPixel(t, solidlens.RGB(0, 1, 0)): true,
		sectionPixel(t, solidlens.RGB(1, 1, 0)): true,
	}
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			if !known[img.NRGBAAt(x, y)] {
				other++
			}
		}
	}
	require.Positive(t, other)
	require.InDelta(t, report.Layers[0].Area, report.Layers[0].RasterArea, 1)
}

func TestRenderSectionOutlineAndFillAlpha(t *testing.T) {
	view := twoRectangles()
	view.Layers = view.Layers[:1]
	view.Layers[0].Fill = solidlens.RGBA(0, 0, 0, 0.5)
	view.Layers[0].Outline = solidlens.RGB(1, 0, 0)
	view.Layers[0].OutlineWidth = 4
	img, _, err := solidlens.RenderSection(t.Context(), view)
	require.NoError(t, err)
	// The left edge u = 0 is at x = 50.
	require.Equal(t, sectionPixel(t, solidlens.RGB(1, 0, 0)), img.NRGBAAt(50, 100))
	inside := img.NRGBAAt(100, 100)
	require.InDelta(t, 128, int(inside.R), 2, "half transparent black over white")
	require.Equal(t, uint8(255), inside.A)
}

func TestRenderSectionOffOriginPlane(t *testing.T) {
	// A plane at z = 5 whose U axis is Y and V axis is Z flipped: the
	// coordinates follow (p-origin).U and (p-origin).V.
	plane := solidlens.SectionPlane{
		Origin: solidlens.Vec{X: 5},
		U:      solidlens.Vec{Y: 1},
		V:      solidlens.Vec{Z: 1},
	}
	loop := []solidlens.Vec{{X: 5, Y: 0, Z: 0}, {X: 5, Y: 4, Z: 0}, {X: 5, Y: 4, Z: 2}, {X: 5, Y: 0, Z: 2}}
	_, report, err := solidlens.RenderSection(t.Context(), solidlens.SectionView{
		Plane:  plane,
		Layers: []solidlens.SectionLayer{{Name: "wall", Loops: [][]solidlens.Vec{loop}, Fill: solidlens.RGB(1, 0, 0)}},
	})
	require.NoError(t, err)
	require.InDelta(t, 8, report.Layers[0].Area, 1e-9)
}

func TestRenderSectionErrors(t *testing.T) {
	nan := math.NaN()
	tests := []struct {
		name   string
		mutate func(*solidlens.SectionView)
	}{
		{"no layers", func(v *solidlens.SectionView) { v.Layers = nil }},
		{"empty window", func(v *solidlens.SectionView) {
			v.Window = &solidlens.SectionWindow{MinU: 1, MaxU: 1, MinV: 0, MaxV: 1}
		}},
		{"short loop", func(v *solidlens.SectionView) {
			v.Layers[0].Loops = [][]solidlens.Vec{{{X: 0}, {X: 1}}}
		}},
		{"non-finite point", func(v *solidlens.SectionView) { v.Layers[0].Loops[0][1].X = nan }},
		{"off plane", func(v *solidlens.SectionView) { v.Layers[0].Loops[0][1].Z = 0.5 }},
		{"non-unit axis", func(v *solidlens.SectionView) { v.Plane.U = solidlens.Vec{X: 2} }},
		{"skewed axes", func(v *solidlens.SectionView) { v.Plane.V = solidlens.Vec{X: 0.6, Y: 0.8} }},
		{"width too large", func(v *solidlens.SectionView) { v.Width = 20000 }},
		{"negative height", func(v *solidlens.SectionView) { v.Height = -1 }},
		{"bad supersample", func(v *solidlens.SectionView) { v.Supersample = 9 }},
		{"non-finite color", func(v *solidlens.SectionView) { v.Layers[0].Fill = solidlens.RGB(nan, 0, 0) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := twoRectangles()
			view.Layers = []solidlens.SectionLayer{{
				Name: "A", Loops: [][]solidlens.Vec{rect(0, 0, 10, 10)}, Fill: solidlens.RGB(0, 0, 1),
			}}
			tt.mutate(&view)
			img, _, err := solidlens.RenderSection(t.Context(), view)
			require.Error(t, err)
			require.Nil(t, img)
		})
	}
}

func TestRenderSectionContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, err := solidlens.RenderSection(ctx, twoRectangles())
	require.ErrorIs(t, err, context.Canceled)
}
