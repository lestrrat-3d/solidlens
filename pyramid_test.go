package solidlens_test

import (
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

// pyramidMesh is a small closed solid inside the box (-1,-1,0) to (1,1,2).
func pyramidMesh(t *testing.T) *solidlens.Mesh {
	t.Helper()
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 0, Y: 1}, {X: 0, Y: 0, Z: 2}},
		[][3]int{{0, 2, 1}, {0, 1, 3}, {1, 2, 3}, {2, 0, 3}},
	)
	require.NoError(t, err)
	return mesh
}

// pyramidScene is a blue pyramid on a dark background, with no camera or
// lights.
func pyramidScene(t *testing.T) solidlens.Scene {
	t.Helper()
	return solidlens.Scene{
		Models:     []solidlens.Model{{Mesh: pyramidMesh(t), Material: solidlens.Matte(solidlens.RGB(0.1, 0.5, 0.9))}},
		Background: solidlens.RGB(0.02, 0.02, 0.03),
	}
}

// emptySource is a TriangleSource with no geometry, which NewMesh would refuse.
type emptySource struct{}

func (emptySource) Vertices() []solidlens.Vec { return nil }
func (emptySource) Triangles() [][3]int       { return nil }
