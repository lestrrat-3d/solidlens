package solidlens_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

// cubeMesh returns an axis-aligned cube centred on the origin, wound so that
// every face normal points outward.
func cubeMesh(t *testing.T, size float64) *solidlens.Mesh {
	t.Helper()
	h := size / 2
	vertices := []solidlens.Vec{
		{X: -h, Y: -h, Z: -h}, {X: h, Y: -h, Z: -h}, {X: h, Y: h, Z: -h}, {X: -h, Y: h, Z: -h},
		{X: -h, Y: -h, Z: h}, {X: h, Y: -h, Z: h}, {X: h, Y: h, Z: h}, {X: -h, Y: h, Z: h},
	}
	triangles := [][3]int{
		{0, 2, 1}, {0, 3, 2},
		{4, 5, 6}, {4, 6, 7},
		{0, 1, 5}, {0, 5, 4},
		{1, 2, 6}, {1, 6, 5},
		{2, 3, 7}, {2, 7, 6},
		{3, 0, 4}, {3, 4, 7},
	}
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

// cubeScene looks at a cube from an angle that keeps three faces visible.
func cubeScene(t *testing.T, edges solidlens.Edges) solidlens.Scene {
	t.Helper()
	return solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{X: 60, Y: -60, Z: 45},
			Target:   solidlens.Vec{},
		},
		Models: []solidlens.Model{{
			Mesh:     cubeMesh(t, 20),
			Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
			Edges:    edges,
		}},
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{X: -1, Y: 1, Z: -1},
			Color:     solidlens.RGB(1, 1, 1),
			Intensity: 1,
		}},
		Background: solidlens.RGB(0, 0, 0.5),
	}
}

// redPixel reports whether a pixel carries the red edge color. No surface or
// background in these scenes leaves red dominant, and line borders are
// coverage blended, so a tint test is used instead of an exact match.
func redPixel(p color.RGBA) bool { return int(p.R) > int(p.G)+30 && int(p.R) > int(p.B)+30 }

// countRed returns how many pixels the red edge lines painted.
func countRed(img *image.RGBA) int {
	count := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if redPixel(img.RGBAAt(x, y)) {
				count++
			}
		}
	}
	return count
}

func TestEdges(t *testing.T) {
	settings := solidlens.Settings{Width: 200, Height: 200}
	red := solidlens.RGB(1, 0, 0)

	t.Run("disabled by default", func(t *testing.T) {
		img, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Edges{}), settings)
		require.NoError(t, err)
		require.Zero(t, countRed(img))
	})

	t.Run("enabled draws lines in the requested color", func(t *testing.T) {
		img, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Outline(red)), settings)
		require.NoError(t, err)
		require.Greater(t, countRed(img), 100)
	})

	t.Run("width scales the amount of drawn line", func(t *testing.T) {
		thin, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Outline(red)), settings)
		require.NoError(t, err)
		thick, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Edges{
			Enabled: true,
			Color:   red,
			Width:   5,
		}), settings)
		require.NoError(t, err)
		require.Greater(t, countRed(thick), 2*countRed(thin))
	})

	t.Run("negative crease angle draws every triangle edge", func(t *testing.T) {
		creases, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Outline(red)), settings)
		require.NoError(t, err)
		wireframe, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Edges{
			Enabled:     true,
			Color:       red,
			CreaseAngle: -1,
		}), settings)
		require.NoError(t, err)
		// The wireframe adds the diagonal of each visible cube face.
		require.Greater(t, countRed(wireframe), countRed(creases))
	})

	t.Run("a crease angle above the cube corner draws nothing but the silhouette", func(t *testing.T) {
		img, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Edges{
			Enabled:     true,
			Color:       red,
			CreaseAngle: 120,
		}), settings)
		require.NoError(t, err)
		lines := countRed(img)
		require.Greater(t, lines, 0)

		all, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Outline(red)), settings)
		require.NoError(t, err)
		require.Less(t, lines, countRed(all))
	})

	t.Run("edges stay hidden behind a nearer body", func(t *testing.T) {
		scene := cubeScene(t, solidlens.Outline(red))
		visible, err := solidlens.Render(t.Context(), scene, settings)
		require.NoError(t, err)

		// A second cube halfway to the camera covers the first one exactly,
		// because it sits on the view axis at half the distance, so none of
		// the first cube's edges may survive.
		scene.Models = append(scene.Models, solidlens.Model{
			Mesh:     translated(t, cubeMesh(t, 20), solidlens.Vec{X: 30, Y: -30, Z: 22.5}),
			Material: solidlens.Matte(solidlens.RGB(0, 0.4, 0)),
		})
		hidden, err := solidlens.Render(t.Context(), scene, settings)
		require.NoError(t, err)
		require.Greater(t, countRed(visible), 0)
		require.Zero(t, countRed(hidden))
	})

	t.Run("STL meshes that repeat vertices still share edges", func(t *testing.T) {
		// Two triangles forming a coplanar square, each with its own copy of
		// the shared diagonal, as an STL import produces.
		mesh, err := solidlens.NewMesh(
			[]solidlens.Vec{
				{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1},
				{X: -1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1},
			},
			[][3]int{{0, 1, 2}, {3, 4, 5}},
		)
		require.NoError(t, err)
		scene := solidlens.Scene{
			Camera:     solidlens.Camera{Position: solidlens.Vec{Z: 4}, Up: solidlens.Vec{Y: 1}},
			Models:     []solidlens.Model{{Mesh: mesh, Material: solidlens.Matte(solidlens.RGB(1, 1, 1)), Edges: solidlens.Outline(red)}},
			Background: solidlens.RGB(0, 0, 0),
		}
		img, err := solidlens.Render(t.Context(), scene, settings)
		require.NoError(t, err)
		// The square's border is drawn; the welded diagonal is flat, so it is
		// not, which leaves the centre of the square unpainted.
		require.Greater(t, countRed(img), 0)
		require.False(t, redPixel(img.RGBAAt(100, 100)))
	})

	t.Run("edges crossing the near plane are clipped", func(t *testing.T) {
		// The camera sits inside the cube, so every edge behind it has to be
		// cut at the near plane instead of projecting to a mirrored line.
		scene := cubeScene(t, solidlens.Outline(red))
		scene.Camera.Position = solidlens.Vec{X: -9.5}
		scene.Camera.Target = solidlens.Vec{X: 1}
		scene.Camera.FOV = 120
		img, err := solidlens.Render(t.Context(), scene, settings)
		require.NoError(t, err)
		require.Greater(t, countRed(img), 0)
	})

	t.Run("invalid settings are rejected", func(t *testing.T) {
		for name, edges := range map[string]solidlens.Edges{
			"negative width":       {Enabled: true, Width: -1},
			"crease angle too big": {Enabled: true, CreaseAngle: 181},
		} {
			t.Run(name, func(t *testing.T) {
				_, err := solidlens.Render(t.Context(), cubeScene(t, edges), settings)
				require.Error(t, err)
			})
		}
	})
}

// translated returns a copy of a mesh moved by offset.
func translated(t *testing.T, source solidlens.TriangleSource, offset solidlens.Vec) *solidlens.Mesh {
	t.Helper()
	vertices := source.Vertices()
	for i, vertex := range vertices {
		vertices[i] = vertex.Add(offset)
	}
	mesh, err := solidlens.NewMesh(vertices, source.Triangles())
	require.NoError(t, err)
	return mesh
}
