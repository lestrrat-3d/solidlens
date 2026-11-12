package solidlens_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

// lineCamera looks along +Y from 100 units away with a 90 degree field of
// view. On the plane Y=0 one world unit then covers exactly one pixel of a
// 200x200 image, and world (0, 0, 0) is the pixel corner at (100, 100).
func lineCamera() solidlens.Camera {
	return solidlens.Camera{Position: solidlens.Vec{Y: -100}, FOV: 90}
}

func renderLines(t *testing.T, models []solidlens.Model, lines ...solidlens.Line) *image.RGBA {
	t.Helper()
	img, err := solidlens.Render(t.Context(), solidlens.Scene{
		Camera:     lineCamera(),
		Models:     models,
		Lines:      lines,
		Background: solidlens.RGB(1, 1, 1),
	}, solidlens.Settings{Width: 200, Height: 200})
	require.NoError(t, err)
	return img
}

// rowRuns describes row y as runs of drawn pixels, each as [start, end), where
// a pixel counts as drawn when it is not the background.
func rowRuns(img *image.RGBA, y int) [][2]int {
	var runs [][2]int
	start := -1
	for x := range img.Bounds().Dx() {
		drawn := img.RGBAAt(x, y) != (color.RGBA{R: 255, G: 255, B: 255, A: 255})
		switch {
		case drawn && start < 0:
			start = x
		case !drawn && start >= 0:
			runs = append(runs, [2]int{start, x})
			start = -1
		}
	}
	if start >= 0 {
		runs = append(runs, [2]int{start, img.Bounds().Dx()})
	}
	return runs
}

func TestLineDotted(t *testing.T) {
	// The line spans pixels 20 to 180 on the rows 99 and 100, which a width of
	// two covers exactly. Dashes are 10 pixels long and 5 apart.
	red := color.RGBA{R: 255, A: 255}
	img := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: -80}, {X: 80}},
		Color:  solidlens.RGB(1, 0, 0),
		Width:  2,
		Dash:   []float64{10, 5},
	})
	var want [][2]int
	for start := 20; start < 180; start += 15 {
		want = append(want, [2]int{start, start + 10})
	}
	for _, y := range []int{99, 100} {
		require.Equal(t, want, rowRuns(img, y), "row %d", y)
		// The round dash ends make the corner pixels of a dash partly covered,
		// so only the pixels between them are checked for the exact colour.
		for _, run := range want {
			for x := run[0] + 1; x < run[1]-1; x++ {
				require.Equal(t, red, img.RGBAAt(x, y), "pixel %d,%d", x, y)
			}
		}
	}
	// Nothing is drawn outside the line's two rows.
	require.Empty(t, rowRuns(img, 98))
	require.Empty(t, rowRuns(img, 101))
}

func TestLineSolid(t *testing.T) {
	img := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: -80}, {X: 80}},
		Color:  solidlens.RGB(0, 0, 1),
		Width:  2,
	})
	require.Equal(t, [][2]int{{19, 181}}, rowRuns(img, 99), "round end caps reach half a width past the ends")
	require.Equal(t, color.RGBA{B: 255, A: 255}, img.RGBAAt(100, 99))
}

func TestLineDefaultWidth(t *testing.T) {
	img := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: -80, Z: 0.5}, {X: 80, Z: 0.5}},
		Color:  solidlens.RGB(0, 0, 0),
	})
	// Row 99 is centred on 99.5, the height of the line, and a one pixel wide
	// line covers it alone.
	require.NotEmpty(t, rowRuns(img, 99))
	require.Empty(t, rowRuns(img, 98))
	require.Empty(t, rowRuns(img, 100))
}

func TestLineDashOddPatternRepeats(t *testing.T) {
	img := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: -80}, {X: 80}},
		Color:  solidlens.RGB(0, 0, 0),
		Width:  2,
		Dash:   []float64{8},
	})
	runs := rowRuns(img, 99)
	require.Equal(t, [2]int{20, 28}, runs[0])
	require.Equal(t, [2]int{36, 44}, runs[1])
}

func TestLineDashShorterThanWidthIsDot(t *testing.T) {
	// Each dash is two pixels long, centred 21, 41, ..., and is drawn as a
	// round dot of the four pixel width. No dot appears at the line's end.
	img := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: -80}, {X: 80}},
		Color:  solidlens.RGB(0, 0, 0),
		Width:  4,
		Dash:   []float64{2, 18},
	})
	var want [][2]int
	for centre := 21; centre < 180; centre += 20 {
		want = append(want, [2]int{centre - 2, centre + 2})
	}
	require.Equal(t, want, rowRuns(img, 99))
}

func TestLineDashContinuesAroundCorner(t *testing.T) {
	// The pattern is measured along the polyline. Leg one is 85 pixels long,
	// so the dash that starts 80 pixels in runs 5 pixels along each leg.
	img := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: -80, Z: 0}, {X: 5, Z: 0}, {X: 5, Z: 80}},
		Color:  solidlens.RGB(0, 0, 0),
		Width:  2,
		Dash:   []float64{10, 10},
	})
	// Leg one covers x pixels 20 to 105 on rows 99 and 100. Its last dash
	// starts at pixel 100 and reaches the corner at 105.
	row := rowRuns(img, 99)
	last := row[len(row)-1]
	require.Equal(t, 100, last[0])
	require.GreaterOrEqual(t, last[1], 105)
	// Leg two climbs from the corner. The same dash carries on for five more
	// pixels, up to y pixel 95, and the gap that follows is ten pixels.
	column := func(x, y int) bool { return img.RGBAAt(x, y) != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) }
	for y := 96; y < 100; y++ {
		require.True(t, column(105, y), "dash on leg two at y=%d", y)
	}
	for y := 85; y < 95; y++ {
		require.False(t, column(105, y), "gap on leg two at y=%d", y)
	}
}

func TestLineDrawnOverModels(t *testing.T) {
	// A closed cube sits between the camera and the line's far half. The
	// line is not depth tested, so it shows over the cube's face.
	img := renderLines(t, []solidlens.Model{{
		Mesh:     cube(t),
		Material: solidlens.Material{Color: solidlens.RGB(0, 1, 0), Unlit: true},
	}}, solidlens.Line{
		Points: []solidlens.Vec{{X: -80, Y: 50}, {X: 80, Y: 50}},
		Color:  solidlens.RGB(1, 0, 0),
		Width:  2,
	})
	// The cube is about two pixels wide at this distance and covers the
	// image centre, which the line also crosses.
	require.Equal(t, color.RGBA{R: 255, A: 255}, img.RGBAAt(100, 100))
}

func TestLineOpacity(t *testing.T) {
	img := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: -80}, {X: 80}},
		Color:  solidlens.RGBA(0, 0, 0, 0.5),
		Width:  2,
	})
	pixel := img.RGBAAt(100, 99)
	require.InDelta(t, 128, int(pixel.R), 1)
	require.InDelta(t, 128, int(pixel.G), 1)
}

func TestLineOutsideClippingRange(t *testing.T) {
	// A line behind the camera draws nothing. A line crossing the camera
	// plane draws only its visible half.
	behind := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: -80, Y: -200}, {X: 80, Y: -200}},
		Color:  solidlens.RGB(0, 0, 0),
		Width:  2,
	})
	require.Empty(t, rowRuns(behind, 99))
	crossing := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: 20, Y: -200}, {X: 20, Y: 0}},
		Color:  solidlens.RGB(0, 0, 0),
		Width:  2,
	})
	require.NotEmpty(t, rowRuns(crossing, 99))
}

func TestLineFarOffScreenIsCheap(t *testing.T) {
	// A segment millions of pixels long with a fine dash pattern must not
	// cost a dash per period.
	img := renderLines(t, nil, solidlens.Line{
		Points: []solidlens.Vec{{X: -1e9}, {X: 1e9}},
		Color:  solidlens.RGB(0, 0, 0),
		Width:  2,
		Dash:   []float64{0.25, 0.25},
	})
	require.NotEmpty(t, rowRuns(img, 99))
}

func TestLineInvalid(t *testing.T) {
	good := solidlens.Line{Points: []solidlens.Vec{{X: -1}, {X: 1}}, Color: solidlens.RGB(0, 0, 0)}
	tests := map[string]func(*solidlens.Line){
		"one point":        func(l *solidlens.Line) { l.Points = l.Points[:1] },
		"nan point":        func(l *solidlens.Line) { l.Points[0].X = math.NaN() },
		"negative width":   func(l *solidlens.Line) { l.Width = -1 },
		"zero dash":        func(l *solidlens.Line) { l.Dash = []float64{2, 0} },
		"negative dash":    func(l *solidlens.Line) { l.Dash = []float64{-2, 2} },
		"tiny dash period": func(l *solidlens.Line) { l.Dash = []float64{0.1, 0.1} },
		"nan color":        func(l *solidlens.Line) { l.Color.R = math.NaN() },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			line := good
			line.Points = append([]solidlens.Vec(nil), good.Points...)
			mutate(&line)
			_, err := solidlens.Render(t.Context(), solidlens.Scene{Camera: lineCamera(), Lines: []solidlens.Line{line}},
				solidlens.Settings{Width: 20, Height: 20})
			require.Error(t, err)
		})
	}
}
