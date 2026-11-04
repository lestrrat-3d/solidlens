package solidlens

import (
	"fmt"
	"math"
)

// Bounds is the axis-aligned box that holds a set of points.
type Bounds struct {
	Min, Max Vec
}

// BoundsOf returns the box that holds every vertex of the sources. It fails
// when a source is nil, a vertex is not finite, or there are no vertices at
// all.
func BoundsOf(sources ...TriangleSource) (Bounds, error) {
	b := Bounds{
		Min: Vec{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)},
		Max: Vec{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)},
	}
	for i, source := range sources {
		if source == nil {
			return Bounds{}, fmt.Errorf("solidlens: bounds source %d is nil", i)
		}
		for _, p := range source.Vertices() {
			if !finiteVec(p) {
				return Bounds{}, fmt.Errorf("solidlens: bounds source %d has a non-finite vertex", i)
			}
			b.Min = Vec{X: math.Min(b.Min.X, p.X), Y: math.Min(b.Min.Y, p.Y), Z: math.Min(b.Min.Z, p.Z)}
			b.Max = Vec{X: math.Max(b.Max.X, p.X), Y: math.Max(b.Max.Y, p.Y), Z: math.Max(b.Max.Z, p.Z)}
		}
	}
	if math.IsInf(b.Min.X, 0) {
		return Bounds{}, fmt.Errorf("solidlens: bounds sources have no vertices")
	}
	return b, nil
}

// Center returns the middle of the box.
func (b Bounds) Center() Vec { return b.Min.Add(b.Max).Scale(0.5) }

// Radius returns the radius of the smallest sphere around Center that holds
// the box. It is zero for a single point.
func (b Bounds) Radius() float64 { return b.Max.Sub(b.Min).Len() / 2 }
