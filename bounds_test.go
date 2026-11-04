package solidlens_test

import (
	"math"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

func TestBoundsOf(t *testing.T) {
	other, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: 5, Y: 0, Z: -3}, {X: 6, Y: 1, Z: -2}, {X: 5, Y: 1, Z: -2}},
		[][3]int{{0, 1, 2}},
	)
	require.NoError(t, err)

	b, err := solidlens.BoundsOf(pyramidMesh(t), other)
	require.NoError(t, err)
	require.Equal(t, solidlens.Vec{X: -1, Y: -1, Z: -3}, b.Min)
	require.Equal(t, solidlens.Vec{X: 6, Y: 1, Z: 2}, b.Max)
	require.Equal(t, solidlens.Vec{X: 2.5, Y: 0, Z: -0.5}, b.Center())
	require.InDelta(t, math.Sqrt(7*7+2*2+5*5)/2, b.Radius(), 1e-12)
}

func TestBoundsOfSinglePointHasNoRadius(t *testing.T) {
	b, err := solidlens.BoundsOf(pointSource{solidlens.Vec{X: 1, Y: 2, Z: 3}})
	require.NoError(t, err)
	require.Equal(t, solidlens.Vec{X: 1, Y: 2, Z: 3}, b.Center())
	require.Zero(t, b.Radius())
}

func TestBoundsOfRejectsBadInput(t *testing.T) {
	for name, sources := range map[string][]solidlens.TriangleSource{
		"none":         nil,
		"nil source":   {nil},
		"no vertices":  {emptySource{}},
		"NaN vertex":   {pointSource{solidlens.Vec{X: math.NaN()}}},
		"Inf vertex":   {pointSource{solidlens.Vec{Z: math.Inf(1)}}},
		"one bad of 2": {pyramidMesh(t), pointSource{solidlens.Vec{Y: math.NaN()}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := solidlens.BoundsOf(sources...)
			require.Error(t, err)
		})
	}
}

// pointSource is a TriangleSource of one vertex.
type pointSource struct{ p solidlens.Vec }

func (s pointSource) Vertices() []solidlens.Vec { return []solidlens.Vec{s.p} }
func (pointSource) Triangles() [][3]int         { return nil }
