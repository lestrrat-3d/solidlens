package solidlens

import (
	"context"
	"fmt"
	"image"
	"math"
)

// SectionPlane is a cutting plane described by an origin and two in-plane
// axes. The plane coordinates of a point p are (p-Origin).U and (p-Origin).V,
// and the plane normal is U x V. U and V must be unit length and
// perpendicular. In the rendered image U points right and V points up.
type SectionPlane struct {
	Origin Vec
	U      Vec
	V      Vec
}

// SectionWindow is a rectangle in plane coordinates, in millimetres.
type SectionWindow struct {
	MinU, MinV, MaxU, MaxV float64
}

// SectionLayer is one solid's cross-section: a set of closed loops that lie
// in the cutting plane.
//
// Loops use the non-zero winding rule with the orientation that decad's
// SectionOutline returns, seen from the tip of the plane normal: outer loops
// run counter-clockwise and holes run clockwise. A section made of several
// separate loops fills correctly. The last point of a loop may repeat the
// first. Fill is the area color; an alpha of zero leaves the area unfilled.
// Outline is the boundary line color, and an alpha of zero draws no outline.
// OutlineWidth is in pixels and defaults to 1.5 when an outline is drawn.
type SectionLayer struct {
	Name         string
	Loops        [][]Vec
	Fill         Color
	Outline      Color
	OutlineWidth float64
}

// SectionView describes a plane section image of one or more layers.
//
// Window selects the visible rectangle. When nil the window fits every layer
// with Margin millimetres around it, or 5% of the larger extent when Margin is
// zero. The window keeps its aspect ratio and is centred in the image, so the
// visible area can be larger than Window.
//
// Width and Height default to 1000 and 800. Supersample is the number of
// samples per pixel along each axis, from 1 to 8, and defaults to 4.
// Background defaults to white when its alpha is zero, and its alpha is
// otherwise ignored because the image is opaque. OverlapColor paints over
// every area covered by two or more layers and defaults to a strong red when
// its alpha is zero. PlaneTolerance is how far in millimetres a loop point may
// sit from the plane, and defaults to 1e-4. Caption is drawn above the legend.
type SectionView struct {
	Plane          SectionPlane
	Layers         []SectionLayer
	Window         *SectionWindow
	Margin         float64
	Width          int
	Height         int
	Supersample    int
	Background     Color
	OverlapColor   Color
	PlaneTolerance float64
	Caption        string
	HideGrid       bool
	HideScaleBar   bool
	HideLegend     bool
}

// SectionLayerReport holds the areas of one layer.
//
// Area is exact: the absolute value of the shoelace sum over all loops, with
// holes subtracting, so it is not limited to the window. RasterArea counts the
// samples the layer covers inside the image, so it is an estimate whose
// precision is the sample size and which excludes anything outside the image.
type SectionLayerReport struct {
	Name       string
	Area       float64
	RasterArea float64
}

// SectionReport carries the numbers behind a section image. All areas are in
// square millimetres.
//
// OverlapArea is a raster estimate of the area covered by two or more layers.
// PixelSize and SampleSize are the millimetres per output pixel and per
// supersample, so the error of a raster area is on the order of the boundary
// length times SampleSize. Window is the rectangle the image actually shows.
type SectionReport struct {
	Layers      []SectionLayerReport
	OverlapArea float64
	PixelSize   float64
	SampleSize  float64
	Window      SectionWindow
}

const (
	sectionDefaultWidth       = 1000
	sectionDefaultHeight      = 800
	sectionDefaultSupersample = 4
	sectionMaxSupersample     = 8
	sectionAxisTolerance      = 1e-6
	sectionDefaultPlaneTol    = 1e-4
	sectionDefaultMargin      = 0.05
)

// sectionLayout is a validated view: loops projected to plane coordinates and
// the mapping between plane coordinates and image pixels.
type sectionLayout struct {
	view       SectionView
	loops      [][][]point2 // layer, loop, point in plane coordinates
	width      int
	height     int
	supersamp  int
	background Color
	overlap    Color
	scale      float64 // pixels per millimetre
	window     SectionWindow
}

type point2 struct{ x, y float64 }

// toPixel maps plane coordinates to image coordinates, with y pointing down.
func (l *sectionLayout) toPixel(p point2) point2 {
	return point2{
		x: (p.x - l.window.MinU) * l.scale,
		y: (l.window.MaxV - p.y) * l.scale,
	}
}

// RenderSection draws a plane section of the view's layers: each layer filled
// and outlined, the overlap of layers in the overlap color, a millimetre grid,
// a scale bar, a legend, and the caption. It also returns the layer and
// overlap areas. The result is opaque. Concurrent calls are safe.
func RenderSection(ctx context.Context, view SectionView) (*image.NRGBA, SectionReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, SectionReport{}, err
	}
	layout, err := newSectionLayout(view)
	if err != nil {
		return nil, SectionReport{}, err
	}
	raster, err := rasterizeSection(ctx, layout)
	if err != nil {
		return nil, SectionReport{}, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, layout.width, layout.height))
	paintSection(img, layout, raster)
	return img, buildSectionReport(layout, raster), nil
}

func newSectionLayout(view SectionView) (*sectionLayout, error) {
	if len(view.Layers) == 0 {
		return nil, fmt.Errorf("solidlens: section view has no layers")
	}
	layout := &sectionLayout{view: view, width: view.Width, height: view.Height, supersamp: view.Supersample}
	if layout.width == 0 {
		layout.width = sectionDefaultWidth
	}
	if layout.height == 0 {
		layout.height = sectionDefaultHeight
	}
	if layout.width < 1 || layout.height < 1 || layout.width > 16384 || layout.height > 16384 {
		return nil, fmt.Errorf("solidlens: section image size %dx%d is outside 1 to 16384 pixels",
			view.Width, view.Height)
	}
	if layout.supersamp == 0 {
		layout.supersamp = sectionDefaultSupersample
	}
	if layout.supersamp < 1 || layout.supersamp > sectionMaxSupersample {
		return nil, fmt.Errorf("solidlens: section supersample %d is outside 1 to %d",
			view.Supersample, sectionMaxSupersample)
	}
	layout.background = view.Background
	if layout.background.A == 0 {
		layout.background = RGB(1, 1, 1)
	}
	layout.overlap = view.OverlapColor
	if layout.overlap.A == 0 {
		layout.overlap = RGBA(1, 0, 0, 0.85)
	}
	for _, color := range []Color{layout.background, layout.overlap} {
		if !finiteColor(color) {
			return nil, fmt.Errorf("solidlens: section view contains a non-finite color")
		}
	}
	if !finite(view.Margin) || view.Margin < 0 {
		return nil, fmt.Errorf("solidlens: section margin must be finite and not negative")
	}
	if !finite(view.PlaneTolerance) || view.PlaneTolerance < 0 {
		return nil, fmt.Errorf("solidlens: section plane tolerance must be finite and not negative")
	}
	normal, err := view.Plane.normal()
	if err != nil {
		return nil, err
	}
	tolerance := view.PlaneTolerance
	if tolerance == 0 {
		tolerance = sectionDefaultPlaneTol
	}
	if err := layout.projectLayers(normal, tolerance); err != nil {
		return nil, err
	}
	if err := layout.fitWindow(); err != nil {
		return nil, err
	}
	return layout, nil
}

// normal validates the plane axes and returns the unit normal U x V.
func (p SectionPlane) normal() (Vec, error) {
	if !finiteVec(p.Origin) || !finiteVec(p.U) || !finiteVec(p.V) {
		return Vec{}, fmt.Errorf("solidlens: section plane contains a non-finite vector")
	}
	if math.Abs(p.U.Len()-1) > sectionAxisTolerance || math.Abs(p.V.Len()-1) > sectionAxisTolerance ||
		math.Abs(p.U.Dot(p.V)) > sectionAxisTolerance {
		return Vec{}, fmt.Errorf("solidlens: section plane axes U and V must be orthonormal")
	}
	return p.U.Cross(p.V), nil
}

func (l *sectionLayout) projectLayers(normal Vec, tolerance float64) error {
	plane := l.view.Plane
	l.loops = make([][][]point2, len(l.view.Layers))
	for layerIndex, layer := range l.view.Layers {
		if !finiteColor(layer.Fill) || !finiteColor(layer.Outline) ||
			!finite(layer.OutlineWidth) || layer.OutlineWidth < 0 {
			return fmt.Errorf("solidlens: section layer %d (%q) has an invalid color or outline width",
				layerIndex, layer.Name)
		}
		l.loops[layerIndex] = make([][]point2, len(layer.Loops))
		for loopIndex, loop := range layer.Loops {
			// A loop may repeat its first point to close itself.
			if n := len(loop); n > 1 && loop[0] == loop[n-1] {
				loop = loop[:n-1]
			}
			if len(loop) < 3 {
				return fmt.Errorf("solidlens: section layer %d (%q) loop %d has fewer than 3 points",
					layerIndex, layer.Name, loopIndex)
			}
			points := make([]point2, len(loop))
			for i, p := range loop {
				if !finiteVec(p) {
					return fmt.Errorf("solidlens: section layer %d (%q) loop %d point %d is not finite",
						layerIndex, layer.Name, loopIndex, i)
				}
				d := p.Sub(plane.Origin)
				if off := math.Abs(d.Dot(normal)); off > tolerance {
					return fmt.Errorf(
						"solidlens: section layer %d (%q) loop %d point %d is %g mm off the plane (tolerance %g)",
						layerIndex, layer.Name, loopIndex, i, off, tolerance)
				}
				points[i] = point2{x: d.Dot(plane.U), y: d.Dot(plane.V)}
			}
			l.loops[layerIndex][loopIndex] = points
		}
	}
	return nil
}

// fitWindow resolves the visible window and the pixel scale. The requested
// window is grown on one axis to match the image aspect ratio.
func (l *sectionLayout) fitWindow() error {
	var win SectionWindow
	if l.view.Window != nil {
		win = *l.view.Window
		for _, v := range []float64{win.MinU, win.MinV, win.MaxU, win.MaxV} {
			if !finite(v) {
				return fmt.Errorf("solidlens: section window is not finite")
			}
		}
	} else {
		win = SectionWindow{MinU: math.Inf(1), MinV: math.Inf(1), MaxU: math.Inf(-1), MaxV: math.Inf(-1)}
		for _, layer := range l.loops {
			for _, loop := range layer {
				for _, p := range loop {
					win.MinU, win.MaxU = math.Min(win.MinU, p.x), math.Max(win.MaxU, p.x)
					win.MinV, win.MaxV = math.Min(win.MinV, p.y), math.Max(win.MaxV, p.y)
				}
			}
		}
		margin := l.view.Margin
		if margin == 0 {
			margin = sectionDefaultMargin * math.Max(win.MaxU-win.MinU, win.MaxV-win.MinV)
		}
		win = SectionWindow{MinU: win.MinU - margin, MinV: win.MinV - margin, MaxU: win.MaxU + margin, MaxV: win.MaxV + margin}
	}
	width, height := win.MaxU-win.MinU, win.MaxV-win.MinV
	if !(width > 0) || !(height > 0) || math.IsInf(width, 0) || math.IsInf(height, 0) {
		return fmt.Errorf("solidlens: section window is empty")
	}
	l.scale = math.Min(float64(l.width)/width, float64(l.height)/height)
	// Centre the window by growing it to the full image extent.
	fullU, fullV := float64(l.width)/l.scale, float64(l.height)/l.scale
	centreU, centreV := (win.MinU+win.MaxU)/2, (win.MinV+win.MaxV)/2
	l.window = SectionWindow{
		MinU: centreU - fullU/2, MaxU: centreU + fullU/2,
		MinV: centreV - fullV/2, MaxV: centreV + fullV/2,
	}
	return nil
}

// loopArea is the signed shoelace area of a loop, positive when it runs
// counter-clockwise.
func loopArea(loop []point2) float64 {
	sum := 0.0
	for i, a := range loop {
		b := loop[(i+1)%len(loop)]
		sum += a.x*b.y - b.x*a.y
	}
	return sum / 2
}

func buildSectionReport(l *sectionLayout, r *sectionRaster) SectionReport {
	sample := 1 / (l.scale * float64(l.supersamp))
	report := SectionReport{
		Layers:      make([]SectionLayerReport, len(l.view.Layers)),
		OverlapArea: float64(r.overlapSamples) * sample * sample,
		PixelSize:   1 / l.scale,
		SampleSize:  sample,
		Window:      l.window,
	}
	for i, layer := range l.view.Layers {
		exact := 0.0
		for _, loop := range l.loops[i] {
			exact += loopArea(loop)
		}
		report.Layers[i] = SectionLayerReport{
			Name:       layer.Name,
			Area:       math.Abs(exact),
			RasterArea: float64(r.layerSamples[i]) * sample * sample,
		}
	}
	return report
}
