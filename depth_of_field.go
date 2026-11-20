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
// edge does not reveal what is behind it. Nothing in a scene glows, so a
// blurred highlight is no brighter than the surface it came from.
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
	source := append([]uint8(nil), img.Pix...)
	shape := newApertureShape(lens.Blades)

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
					gatherPixel(img, source, depth, radii, shape, width, height, reach, x, y)
				}
			}
		})
	}
	group.Wait()
	return ctx.Err()
}

func gatherPixel(
	img *image.RGBA, source []uint8, depth, radii []float64, shape apertureShape, width, height, reach, x, y int,
) {
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
