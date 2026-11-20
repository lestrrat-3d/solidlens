package solidlens

import (
	"context"
	"fmt"
	"image"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
)

const (
	defaultBlurRadius = 24
	largestBlurRadius = 64
	smallestBlades    = 3
	largestBlades     = 16
	largestHighlight  = 100
	// highlightDepthMargin is the fraction of a pixel's inverse depth below
	// which a gathered pixel counts as behind it and gets no highlight boost.
	highlightDepthMargin = 0.9
	// defaultHighlightThreshold is the brightness above which a pixel counts
	// as a highlight when HighlightThreshold is zero.
	defaultHighlightThreshold = 0.8
	// sharpRadius is the blur radius, in pixels, below which a pixel is
	// treated as in focus and spreads to no neighbour.
	sharpRadius = 0.5
)

// DepthOfField blurs the parts of a scene that lie away from a focal plane.
// Each pixel spreads into a disc, or a regular polygon when Blades is set, so
// bright shapes out of focus take the shape of the aperture, which is what
// the photographic term bokeh describes.
//
// The blur works from the depth buffer of the finished render. A surface
// hidden behind a nearer one is not in that buffer, so a blurred foreground
// edge does not reveal what is behind it. Nothing in a scene glows, so
// without Highlight a blurred bright spot fades to the average of its disc.
type DepthOfField struct {
	// FocusDistance is the distance from the camera, in the units of the
	// scene, at which surfaces are sharp. It must be positive.
	FocusDistance float64
	// Aperture is the diameter of the lens in the units of the scene. A larger
	// aperture blurs more. It must be positive.
	Aperture float64
	// Blades is the number of straight sides of the blur shape, between 3 and
	// 16. Zero gives a circle.
	Blades int
	// MaxRadius caps the blur radius in pixels, and bounds the render time,
	// which grows with its square. It defaults to 24 and may not exceed 64.
	MaxRadius float64
	// Highlight makes bright pixels count for more when the blur averages them
	// with their neighbours, so a small bright spot out of focus stays visible
	// as a disc or polygon instead of fading. A bright pixel counts for
	// 1+Highlight times a dark one. Zero turns this off, and it may not exceed
	// 100.
	Highlight float64
	// HighlightThreshold is the brightness, from 0 to 1 and measured on the
	// final pixel color, above which a pixel is boosted. The boost reaches its
	// full size at 1. It defaults to 0.8 and must be below 1.
	HighlightThreshold float64
}

func (d DepthOfField) normalized() (DepthOfField, error) {
	if !finite(d.FocusDistance) || d.FocusDistance <= 0 {
		return DepthOfField{}, fmt.Errorf("solidlens: depth of field focus distance must be positive")
	}
	if !finite(d.Aperture) || d.Aperture <= 0 {
		return DepthOfField{}, fmt.Errorf("solidlens: depth of field aperture must be positive")
	}
	if d.Blades != 0 && (d.Blades < smallestBlades || d.Blades > largestBlades) {
		return DepthOfField{}, fmt.Errorf("solidlens: depth of field blades must be 0 or between %d and %d",
			smallestBlades, largestBlades)
	}
	if d.MaxRadius == 0 {
		d.MaxRadius = defaultBlurRadius
	}
	if !finite(d.MaxRadius) || d.MaxRadius < 0 || d.MaxRadius > largestBlurRadius {
		return DepthOfField{}, fmt.Errorf("solidlens: depth of field max radius must be between 0 and %d",
			largestBlurRadius)
	}
	if !finite(d.Highlight) || d.Highlight < 0 || d.Highlight > largestHighlight {
		return DepthOfField{}, fmt.Errorf("solidlens: depth of field highlight must be between 0 and %d",
			largestHighlight)
	}
	if d.HighlightThreshold == 0 {
		d.HighlightThreshold = defaultHighlightThreshold
	}
	if !finite(d.HighlightThreshold) || d.HighlightThreshold < 0 || d.HighlightThreshold >= 1 {
		return DepthOfField{}, fmt.Errorf("solidlens: depth of field highlight threshold must be at least 0 and below 1")
	}
	return d, nil
}

// apertureShape tests whether an offset from the center of a blur shape lies
// inside it, and reports the area of the shape for a radius of one.
type apertureShape struct {
	normals [][2]float64
	inset   float64
	area    float64
}

func newApertureShape(blades int) apertureShape {
	if blades == 0 {
		return apertureShape{area: math.Pi}
	}
	count := float64(blades)
	shape := apertureShape{
		normals: make([][2]float64, blades),
		inset:   math.Cos(math.Pi / count),
		area:    count / 2 * math.Sin(2*math.Pi/count),
	}
	for side := range shape.normals {
		angle := 2 * math.Pi * float64(side) / count
		shape.normals[side] = [2]float64{math.Sin(angle), -math.Cos(angle)}
	}
	return shape
}

// contains reports whether the offset lies inside the shape scaled to radius.
// The caller has already checked that it lies inside the circle of that radius.
func (s apertureShape) contains(dx, dy, radius float64) bool {
	limit := radius * s.inset
	for _, normal := range s.normals {
		if dx*normal[0]+dy*normal[1] > limit {
			return false
		}
	}
	return true
}

// applyDepthOfField replaces the pixels of img with the blurred result. depth
// holds the inverse camera-space depth of each pixel, zero where nothing is
// drawn. Each pixel gathers from every pixel whose blur shape covers it,
// weighted by the inverse of that shape's area so brightness is preserved. A
// pixel farther away than the one gathering contributes only as far as the
// gatherer is itself blurred, so a sharp object keeps its edge against a
// blurred background.
func applyDepthOfField(ctx context.Context, img *image.RGBA, depth []float64, v view, lens DepthOfField) error {
	width, height := v.width, v.height
	// With the image plane one focal length from the lens, a point at depth z
	// blurs to a disc of diameter aperture*focal*|1/focus - 1/z|.
	pixelsPerUnit := v.focal * float64(height) / 2
	scale := lens.Aperture * pixelsPerUnit / 2
	inverseFocus := 1 / lens.FocusDistance
	radii := make([]float64, len(depth))
	var widest float64
	for index, inverse := range depth {
		radius := math.Min(scale*math.Abs(inverseFocus-inverse), lens.MaxRadius)
		radii[index] = radius
		widest = math.Max(widest, radius)
	}
	if widest < sharpRadius {
		return nil
	}
	reach := int(math.Floor(widest))
	plan := blurPlan{
		source: append([]uint8(nil), img.Pix...),
		depth:  depth,
		radii:  radii,
		shape:  newApertureShape(lens.Blades),
		width:  width,
		height: height,
		reach:  reach,
	}
	if lens.Highlight > 0 {
		plan.boosts = highlightBoosts(plan.source, lens.Highlight, lens.HighlightThreshold)
	}

	var failed atomic.Bool
	var next atomic.Int64
	var group sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		group.Go(func() {
			for {
				y := int(next.Add(1)) - 1
				if y >= height || failed.Load() {
					return
				}
				if ctx.Err() != nil {
					failed.Store(true)
					return
				}
				for x := range width {
					plan.gather(img, x, y)
				}
			}
		})
	}
	group.Wait()
	return ctx.Err()
}

// blurPlan is the read-only input that every pixel's gather shares.
type blurPlan struct {
	source []uint8
	depth  []float64
	radii  []float64
	// boosts is the weight multiplier of each pixel, or nil when highlights
	// are not boosted.
	boosts        []float64
	shape         apertureShape
	width, height int
	reach         int
}

// highlightBoosts returns, for each pixel of the premultiplied source, one
// plus highlight scaled by how far its brightness is above threshold. The
// scale rises smoothly from 0 at threshold to 1 at full brightness.
func highlightBoosts(source []uint8, highlight, threshold float64) []float64 {
	boosts := make([]float64, len(source)/4)
	for index := range boosts {
		offset := index * 4
		brightness := (0.2126*float64(source[offset]) + 0.7152*float64(source[offset+1]) +
			0.0722*float64(source[offset+2])) / 255
		t := min(max((brightness-threshold)/(1-threshold), 0), 1)
		boosts[index] = 1 + highlight*t*t*(3-2*t)
	}
	return boosts
}

func (p *blurPlan) gather(img *image.RGBA, x, y int) {
	width, height, reach := p.width, p.height, p.reach
	source, depth, radii, shape := p.source, p.depth, p.radii, p.shape
	here := y*width + x
	hereDepth := depth[here]
	// A sharp pixel keeps only a little of the blurred background behind it.
	behind := math.Min(1, radii[here]/sharpRadius)
	var red, green, blue, alpha, total float64
	for sy := max(0, y-reach); sy <= min(height-1, y+reach); sy++ {
		dy := float64(sy - y)
		for sx := max(0, x-reach); sx <= min(width-1, x+reach); sx++ {
			index := sy*width + sx
			radius := radii[index]
			dx := float64(sx - x)
			if dx*dx+dy*dy > radius*radius {
				continue
			}
			if index != here && !shape.contains(dx, dy, radius) {
				continue
			}
			radius = math.Max(radius, sharpRadius)
			weight := 1 / (shape.area * radius * radius)
			if depth[index] < hereDepth {
				weight *= behind
			}
			// A highlight much farther than the pixel gathering it is not
			// boosted, so it cannot glow over a nearer surface. The margin keeps
			// the far side of a curved light boosted for its own near side.
			if p.boosts != nil && depth[index] >= hereDepth*highlightDepthMargin {
				weight *= p.boosts[index]
			}
			offset := index * 4
			red += weight * float64(source[offset])
			green += weight * float64(source[offset+1])
			blue += weight * float64(source[offset+2])
			alpha += weight * float64(source[offset+3])
			total += weight
		}
	}
	// The pixel covers itself, so total is positive.
	out := y*img.Stride + x*4
	img.Pix[out] = uint8(red/total + 0.5)
	img.Pix[out+1] = uint8(green/total + 0.5)
	img.Pix[out+2] = uint8(blue/total + 0.5)
	img.Pix[out+3] = uint8(alpha/total + 0.5)
}
