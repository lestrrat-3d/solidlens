package solidlens_test

import (
	"bytes"
	"image"
	"image/color"
	"math"
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

	t.Run("repeated renders of one scene produce identical pixels", func(t *testing.T) {
		// A negative crease angle draws every triangle edge, so the cube's
		// lines cross at each corner. Overlapping lines blend, so an unstable
		// edge order shows up as differing pixels. Pixels are compared rather
		// than encoded PNGs because the encoder is a pure function of them.
		scene := cubeScene(t, solidlens.Edges{Enabled: true, Color: red, CreaseAngle: -1})
		first, err := solidlens.Render(t.Context(), scene, settings)
		require.NoError(t, err)
		for run := 1; run < 8; run++ {
			again, err := solidlens.Render(t.Context(), scene, settings)
			require.NoError(t, err)
			require.Truef(t, bytes.Equal(first.Pix, again.Pix), "render %d differs from the first", run)
		}
	})

	t.Run("crossing lines composite once per pixel", func(t *testing.T) {
		// A half transparent black line can darken a pixel to at most half of
		// what the surface under it was. Blending that pixel a second time
		// would take it to a quarter, so this catches lines that composite
		// once per line rather than once per pixel. A negative crease angle
		// draws the full wireframe, whose lines cross at every cube corner.
		plain, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Edges{}), settings)
		require.NoError(t, err)
		lined, err := solidlens.Render(t.Context(), cubeScene(t, solidlens.Edges{
			Enabled:     true,
			Color:       solidlens.RGBA(0, 0, 0, 0.5),
			CreaseAngle: -1,
		}), settings)
		require.NoError(t, err)
		for y := range settings.Height {
			for x := range settings.Width {
				under, over := plain.RGBAAt(x, y), lined.RGBAAt(x, y)
				require.GreaterOrEqualf(t, int(over.G), int(under.G)/2-1,
					"pixel (%d,%d) is %v over a surface of %v, darker than one composite allows", x, y, over, under)
			}
		}
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

// slivered returns a flat plate whose middle holds a strip a ten-thousandth of
// a millimetre wide that rises a thousandth, so its facets are slivers tilted
// 84 degrees from the plate.
func slivered(t *testing.T) *solidlens.Mesh {
	t.Helper()
	vertices := []solidlens.Vec{
		{X: -10, Y: -10}, {X: 0, Y: -10}, {X: 0, Y: 10}, {X: -10, Y: 10},
		{X: 1e-4, Y: -10, Z: 1e-3}, {X: 1e-4, Y: 10, Z: 1e-3},
		{X: 10, Y: -10, Z: 1e-3}, {X: 10, Y: 10, Z: 1e-3},
	}
	triangles := [][3]int{
		{0, 1, 2}, {0, 2, 3},
		{1, 4, 5}, {1, 5, 2},
		{4, 6, 7}, {4, 7, 5},
	}
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

// fanned returns a ramp made of three faces. A flat plate on the left meets a
// slope at x = 0 in a 45 degree crease. The slope's first half millimetre is a
// fan of 25 strips, each 0.02 mm wide and 20 mm long, and a pair of large
// triangles continues the same plane beyond it. The strips are slivers one by
// one, but together they are a face wider than a pixel, so the only crease is
// at x = 0. A sliver that took the normal of the large triangle on either side
// would leave a crease where the two meet in the middle of the fan.
func fanned(t *testing.T) *solidlens.Mesh {
	t.Helper()
	const strips = 25
	const step = 0.02
	var vertices []solidlens.Vec
	var triangles [][3]int
	addColumn := func(x, z float64) int {
		vertices = append(vertices, solidlens.Vec{X: x, Y: -10, Z: z}, solidlens.Vec{X: x, Y: 10, Z: z})
		return len(vertices) - 2
	}
	addQuad := func(left, right int) {
		triangles = append(triangles, [3]int{left, right, right + 1}, [3]int{left, right + 1, left + 1})
	}
	plate := addColumn(-10, 0)
	previous := addColumn(0, 0)
	addQuad(plate, previous)
	for i := 1; i <= strips; i++ {
		next := addColumn(float64(i)*step, float64(i)*step)
		addQuad(previous, next)
		previous = next
	}
	far := addColumn(10.5, 10.5)
	addQuad(previous, far)
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

func TestEdgesToleranceKeepsFaceOfNarrowFacets(t *testing.T) {
	t.Parallel()

	// redIn counts edge pixels in the columns from first up to, not including, last.
	redIn := func(t *testing.T, first, last int) int {
		t.Helper()
		scene := solidlens.Scene{
			Camera: solidlens.Camera{Position: solidlens.Vec{Z: 60}, Up: solidlens.Vec{Y: 1}},
			Models: []solidlens.Model{{
				Mesh:     fanned(t),
				Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
				Edges:    solidlens.Edges{Enabled: true, Color: solidlens.RGB(1, 0, 0)},
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0.5),
		}
		img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 600, Height: 600})
		require.NoError(t, err)
		count := 0
		for y := 240; y < 360; y++ {
			for x := first; x < last; x++ {
				if redPixel(img.RGBAAt(x, y)) {
					count++
				}
			}
		}
		return count
	}

	// The image is 12 pixels to the millimetre, so x = 0 is column 300 and the
	// fan covers columns 300 to 306.
	t.Run("the crease where the slope meets the plate is drawn", func(t *testing.T) {
		t.Parallel()
		require.Positive(t, redIn(t, 296, 304))
	})
	t.Run("the fan draws no crease inside its face", func(t *testing.T) {
		t.Parallel()
		require.Zero(t, redIn(t, 302, 312))
	})
}

func TestEdgesTolerance(t *testing.T) {
	t.Parallel()

	// interiorRed counts edge pixels in a window that holds the strip and
	// stays clear of the plate's border.
	interiorRed := func(t *testing.T, edges solidlens.Edges) int {
		t.Helper()
		scene := solidlens.Scene{
			Camera: solidlens.Camera{
				Position: solidlens.Vec{Z: 60},
				Up:       solidlens.Vec{Y: 1},
			},
			Models: []solidlens.Model{{
				Mesh:     slivered(t),
				Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
				Edges:    edges,
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0.5),
		}
		img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 600, Height: 600})
		require.NoError(t, err)
		count := 0
		for y := 240; y < 360; y++ {
			for x := 240; x < 360; x++ {
				if redPixel(img.RGBAAt(x, y)) {
					count++
				}
			}
		}
		return count
	}
	red := solidlens.Edges{Enabled: true, Color: solidlens.RGB(1, 0, 0)}

	t.Run("the default hides a sliver narrower than a pixel", func(t *testing.T) {
		t.Parallel()
		require.Zero(t, interiorRed(t, red))
	})
	t.Run("an explicit tolerance wider than the sliver hides it", func(t *testing.T) {
		t.Parallel()
		withTolerance := red
		withTolerance.Tolerance = 0.01
		require.Zero(t, interiorRed(t, withTolerance))
	})
	t.Run("an explicit tolerance narrower than the sliver draws it", func(t *testing.T) {
		t.Parallel()
		withTolerance := red
		withTolerance.Tolerance = 1e-6
		require.Positive(t, interiorRed(t, withTolerance))
	})
	t.Run("Exact draws the sliver", func(t *testing.T) {
		t.Parallel()
		exact := red
		exact.Tolerance = solidlens.Exact
		require.Positive(t, interiorRed(t, exact))
	})
	t.Run("a source that is not a Mesh gets the same tolerance", func(t *testing.T) {
		t.Parallel()
		plate := slivered(t)
		source := plainSource{vertices: plate.Vertices(), triangles: plate.Triangles()}
		render := func(edges solidlens.Edges) int {
			scene := solidlens.Scene{
				Camera: solidlens.Camera{Position: solidlens.Vec{Z: 60}, Up: solidlens.Vec{Y: 1}},
				Models: []solidlens.Model{{
					Mesh: source, Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)), Edges: edges,
				}},
				DirectionalLights: []solidlens.DirectionalLight{{
					Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
				}},
				Background: solidlens.RGB(0, 0, 0.5),
			}
			img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 600, Height: 600})
			require.NoError(t, err)
			count := 0
			for y := 240; y < 360; y++ {
				for x := 240; x < 360; x++ {
					if redPixel(img.RGBAAt(x, y)) {
						count++
					}
				}
			}
			return count
		}
		exact := red
		exact.Tolerance = solidlens.Exact
		require.Zero(t, render(red))
		require.Positive(t, render(exact))
	})
	t.Run("a tolerance is a finite number", func(t *testing.T) {
		t.Parallel()
		bad := red
		bad.Tolerance = math.NaN()
		_, err := solidlens.Render(t.Context(), solidlens.Scene{
			Camera: solidlens.Camera{Position: solidlens.Vec{Z: 60}, Up: solidlens.Vec{Y: 1}},
			Models: []solidlens.Model{{Mesh: slivered(t), Material: solidlens.Matte(solidlens.RGB(1, 1, 1)), Edges: bad}},
		}, solidlens.Settings{Width: 10, Height: 10})
		require.Error(t, err)
	})
}

// plainSource is a TriangleSource that is not a *solidlens.Mesh.
type plainSource struct {
	vertices  []solidlens.Vec
	triangles [][3]int
}

func (s plainSource) Vertices() []solidlens.Vec { return s.vertices }
func (s plainSource) Triangles() [][3]int       { return s.triangles }
