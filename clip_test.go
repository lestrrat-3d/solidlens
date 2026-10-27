package solidlens_test

import (
	"math"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

// cube is the closed cube [-1, 1]^3 with outward-facing triangles.
func cube(t *testing.T) *solidlens.Mesh {
	t.Helper()
	vertices := []solidlens.Vec{
		{X: -1, Y: -1, Z: -1}, {X: 1, Y: -1, Z: -1}, {X: 1, Y: 1, Z: -1}, {X: -1, Y: 1, Z: -1},
		{X: -1, Y: -1, Z: 1}, {X: 1, Y: -1, Z: 1}, {X: 1, Y: 1, Z: 1}, {X: -1, Y: 1, Z: 1},
	}
	triangles := [][3]int{
		{0, 3, 2}, {0, 2, 1}, // -Z
		{4, 5, 6}, {4, 6, 7}, // +Z
		{0, 1, 5}, {0, 5, 4}, // -Y
		{3, 7, 6}, {3, 6, 2}, // +Y
		{0, 4, 7}, {0, 7, 3}, // -X
		{1, 2, 6}, {1, 6, 5}, // +X
	}
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

func meshArea(mesh *solidlens.Mesh) float64 {
	vertices := mesh.Vertices()
	total := 0.0
	for _, tri := range mesh.Triangles() {
		a, b, c := vertices[tri[0]], vertices[tri[1]], vertices[tri[2]]
		total += b.Sub(a).Cross(c.Sub(a)).Len() / 2
	}
	return total
}

func TestClipMeshKeepsTheBehindSide(t *testing.T) {
	clipped, err := solidlens.ClipMesh(cube(t), solidlens.ClipPlane{Normal: solidlens.Vec{X: 1}})
	require.NoError(t, err)
	for _, v := range clipped.Vertices() {
		require.LessOrEqual(t, v.X, 1e-12)
	}
	// The kept surface is the -X face (4) and the half faces on the other four sides (4 x 2).
	require.InDelta(t, 12, meshArea(clipped), 1e-9)
}

func TestClipMeshLeavesOneOpenLoopOnThePlane(t *testing.T) {
	clipped, err := solidlens.ClipMesh(cube(t), solidlens.ClipPlane{
		Origin: solidlens.Vec{X: 0.25}, Normal: solidlens.Vec{X: 3},
	})
	require.NoError(t, err)
	// Edges used by one triangle are the cut's boundary. They must all lie on the plane and add up to
	// its square outline, 8 mm, which holds only when the triangles share the vertices on the cut.
	uses := map[[2]int]int{}
	for _, tri := range clipped.Triangles() {
		for i := range 3 {
			a, b := tri[i], tri[(i+1)%3]
			if a > b {
				a, b = b, a
			}
			uses[[2]int{a, b}]++
		}
	}
	vertices := clipped.Vertices()
	length := 0.0
	for edge, n := range uses {
		require.LessOrEqual(t, n, 2)
		if n != 1 {
			continue
		}
		a, b := vertices[edge[0]], vertices[edge[1]]
		require.InDelta(t, 0.25, a.X, 1e-12)
		require.InDelta(t, 0.25, b.X, 1e-12)
		length += a.Sub(b).Len()
	}
	require.InDelta(t, 8, length, 1e-9)
}

func TestClipMeshRejectsBadPlanes(t *testing.T) {
	mesh := cube(t)
	_, err := solidlens.ClipMesh(mesh, solidlens.ClipPlane{})
	require.ErrorContains(t, err, "non-zero")
	_, err = solidlens.ClipMesh(mesh, solidlens.ClipPlane{Origin: solidlens.Vec{X: math.NaN()}, Normal: solidlens.Vec{X: 1}})
	require.ErrorContains(t, err, "finite")
	// The plane x = -5 with its removed side toward +X takes the whole cube away.
	_, err = solidlens.ClipMesh(mesh, solidlens.ClipPlane{Origin: solidlens.Vec{X: -5}, Normal: solidlens.Vec{X: 1}})
	require.ErrorContains(t, err, "whole mesh")
	_, err = solidlens.ClipMesh(nil, solidlens.ClipPlane{Normal: solidlens.Vec{X: 1}})
	require.Error(t, err)
}

// A camera on +X looks into the cube whose +X half was cut away. Its Cap material colours what it sees
// through the cut, and without a Cap it sees the inside shaded as the model's own material.
func TestRenderClipShowsTheCapThroughTheCut(t *testing.T) {
	green := solidlens.Material{Color: solidlens.RGB(0, 1, 0), Unlit: true}
	render := func(t *testing.T, capMaterial *solidlens.Material) (uint8, uint8) {
		t.Helper()
		image, err := solidlens.Render(t.Context(), solidlens.Scene{
			Camera: solidlens.Camera{Position: solidlens.Vec{X: 8}, Target: solidlens.Vec{}, Up: solidlens.Vec{Z: 1}},
			Models: []solidlens.Model{{
				Mesh:     cube(t),
				Material: solidlens.Matte(solidlens.RGB(1, 0, 0)),
				Clip:     &solidlens.ClipPlane{Normal: solidlens.Vec{X: 1}, Cap: capMaterial},
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{X: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0),
		}, solidlens.Settings{Width: 64, Height: 64})
		require.NoError(t, err)
		pixel := image.RGBAAt(32, 32)
		return pixel.R, pixel.G
	}
	r, g := render(t, &green)
	require.Greater(t, int(g), int(r)+150, "the cap colour shows through the cut")
	r, g = render(t, nil)
	require.Greater(t, int(r), int(g)+100, "without a cap the inside is shaded as the model's material")
}

func TestRenderClipErrors(t *testing.T) {
	_, err := solidlens.Render(t.Context(), solidlens.Scene{
		Camera: solidlens.Camera{Position: solidlens.Vec{X: 8}},
		Models: []solidlens.Model{{Mesh: cube(t), Clip: &solidlens.ClipPlane{}}},
	}, solidlens.Settings{Width: 8, Height: 8})
	require.ErrorContains(t, err, "model 0")
}
