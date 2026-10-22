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

// stepped returns two flat faces at the same angle joined by a step three
// micrometres high. The left face is a pair of large triangles. The right face
// is a fan of 250 needle-shaped strips, each 0.02 mm wide and 20 mm long,
// that ends in a second pair of large triangles. The step is a sliver that
// touches the first needle of the fan.
func stepped(t *testing.T) *solidlens.Mesh {
	t.Helper()
	const strips = 250
	const width = 0.02
	const height = 3e-3
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
	foot := addColumn(0, 0)
	addQuad(plate, foot)
	previous := addColumn(1e-3, height)
	addQuad(foot, previous)
	for i := 1; i <= strips; i++ {
		next := addColumn(1e-3+float64(i)*width, height)
		addQuad(previous, next)
		previous = next
	}
	far := addColumn(10, height)
	addQuad(previous, far)
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

func TestEdgesToleranceHidesStepBesideFanOfNarrowFacets(t *testing.T) {
	t.Parallel()

	scene := solidlens.Scene{
		Camera: solidlens.Camera{Position: solidlens.Vec{Z: 60}, Up: solidlens.Vec{Y: 1}},
		Models: []solidlens.Model{{
			Mesh:     stepped(t),
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

	// The image is 12 pixels to the millimetre, so the step sits at column
	// 300. The window covers it and the first four millimetres of the fan.
	count := 0
	for y := 240; y < 360; y++ {
		for x := 280; x < 348; x++ {
			if redPixel(img.RGBAAt(x, y)) {
				count++
			}
		}
	}
	require.Zero(t, count)
}

// curved returns a sheet that leaves a flat plate and bends upward in three
// strips, each 0.25 mm wide and 20 mm long, turning 20 degrees at every joint. It
// then runs on as a second flat plate turned 20 degrees further. No joint
// reaches the default crease angle of 30 degrees, so the sheet is one smooth
// face.
func curved(t *testing.T) *solidlens.Mesh {
	t.Helper()
	var vertices []solidlens.Vec
	var triangles [][3]int
	addColumn := func(x, z float64) int {
		vertices = append(vertices, solidlens.Vec{X: x, Y: -10, Z: z}, solidlens.Vec{X: x, Y: 10, Z: z})
		return len(vertices) - 2
	}
	addQuad := func(left, right int) {
		triangles = append(triangles, [3]int{left, right, right + 1}, [3]int{left, right + 1, left + 1})
	}
	x, z := 0.0, 0.0
	previous := addColumn(-10, 0)
	current := addColumn(x, z)
	addQuad(previous, current)
	for turn := 1; turn <= 3; turn++ {
		angle := float64(turn) * 20 * math.Pi / 180
		x, z = x+0.25*math.Cos(angle), z+0.25*math.Sin(angle)
		next := addColumn(x, z)
		addQuad(current, next)
		current = next
	}
	angle := 80 * math.Pi / 180
	far := addColumn(x+10*math.Cos(angle), z+10*math.Sin(angle))
	addQuad(current, far)
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

func TestEdgesToleranceKeepsSmoothFaceOfNarrowFacets(t *testing.T) {
	t.Parallel()

	// A tolerance wider than every strip makes slivers of them all. The face
	// is smooth, so the lines it draws are the same with and without it.
	lines := func(t *testing.T, tolerance float64) int {
		t.Helper()
		scene := solidlens.Scene{
			Camera: solidlens.Camera{Position: solidlens.Vec{X: -3, Y: -5, Z: 40}, Target: solidlens.Vec{X: 1}, Up: solidlens.Vec{Y: 1}},
			Models: []solidlens.Model{{
				Mesh:     curved(t),
				Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
				Edges:    solidlens.Edges{Enabled: true, Color: solidlens.RGB(1, 0, 0), Tolerance: tolerance},
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0.5),
		}
		img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 600, Height: 600})
		require.NoError(t, err)
		return countRed(img)
	}

	require.Equal(t, lines(t, solidlens.Exact), lines(t, 0.6))
}

// wedged returns a floor of 40 strips, each 0.5 mm wide and 4 mm long, with a
// ramp rising at 60 degrees along its far edge. The ramp is a fan of triangles
// that share one distant apex, so each is a sliver whose short side lies on a
// floor strip's far edge, as a bore's end face touches the strips of its wall.
// The floor strips are taller than the slivers. The ramp is a face, not a gap,
// since the fan covers a triangle 20 mm wide.
func wedged(t *testing.T) *solidlens.Mesh {
	t.Helper()
	const strips = 40
	const width = 0.5
	var vertices []solidlens.Vec
	var triangles [][3]int
	for j := range strips + 1 {
		y := -10 + float64(j)*width
		vertices = append(vertices, solidlens.Vec{X: 0, Y: y}, solidlens.Vec{X: 4, Y: y})
	}
	for j := range strips {
		near, far := 2*j, 2*(j+1)
		triangles = append(triangles, [3]int{near, near + 1, far + 1}, [3]int{near, far + 1, far})
	}
	apex := len(vertices)
	vertices = append(vertices, solidlens.Vec{X: 7, Y: 80, Z: 5})
	for j := range strips {
		triangles = append(triangles, [3]int{2*j + 1, apex, 2*(j+1) + 1})
	}
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

func TestEdgesToleranceKeepsWideFanBesideTallFacets(t *testing.T) {
	t.Parallel()

	lines := func(t *testing.T, tolerance float64) int {
		t.Helper()
		scene := solidlens.Scene{
			Camera: solidlens.Camera{
				Position: solidlens.Vec{X: 3, Y: 30, Z: 100}, Target: solidlens.Vec{X: 3, Y: 30}, Up: solidlens.Vec{Y: 1},
			},
			Models: []solidlens.Model{{
				Mesh:     wedged(t),
				Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
				Edges:    solidlens.Edges{Enabled: true, Color: solidlens.RGB(1, 0, 0), Tolerance: tolerance},
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0.5),
		}
		img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 600, Height: 600})
		require.NoError(t, err)
		return countRed(img)
	}

	// Every sliver of the fan touches a floor strip, yet the fan is a face, so
	// the crease along the floor's far edge stays whatever the tolerance is.
	require.Equal(t, lines(t, solidlens.Exact), lines(t, 0.1))
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

// chamfered returns a flat plate in the z = 0 plane that ends at x = 0 in a 45
// degree slope one millimetre long. Along the end of the plate, ten triangles
// 0.1 mm wide fan out from one point on the plate's side edge, 0.5 mm from the
// corner. Each is 10 mm long and only 0.005 mm high, so they are slivers, but
// the slope's triangles that share their 0.1 mm ends are 0.1 mm high. The plate
// is one flat face whose triangles all have the normal (0, 0, 1), and its only
// crease is the line at x = 0 where the slope begins.
func chamfered(t *testing.T) *solidlens.Mesh {
	t.Helper()
	const needles = 10
	const width = 0.1
	var vertices []solidlens.Vec
	var triangles [][3]int
	add := func(v solidlens.Vec) int {
		vertices = append(vertices, v)
		return len(vertices) - 1
	}
	// addUp adds a triangle wound so that it faces up.
	addUp := func(a, b, c int) {
		cross := vertices[b].Sub(vertices[a]).Cross(vertices[c].Sub(vertices[a]))
		if cross.Z < 0 {
			b, c = c, b
		}
		triangles = append(triangles, [3]int{a, b, c})
	}
	apex := add(solidlens.Vec{X: -0.5, Y: -10})
	cornerLow := add(solidlens.Vec{Y: -10})
	cornerHigh := add(solidlens.Vec{Y: 10})
	farLow := add(solidlens.Vec{X: -20, Y: -10})
	farHigh := add(solidlens.Vec{X: -20, Y: 10})
	slopeLow := add(solidlens.Vec{X: 1, Y: -10, Z: -1})
	slopeHigh := add(solidlens.Vec{X: 1, Y: 10, Z: -1})
	// ends and feet are the needles' ends on the plate and on the slope.
	var ends, feet []int
	for k := 0; k <= needles; k++ {
		y := (float64(k) - needles/2) * width
		ends = append(ends, add(solidlens.Vec{Y: y}))
		feet = append(feet, add(solidlens.Vec{X: 1, Y: y, Z: -1}))
	}
	last := needles
	// The plate: the needles and the large triangles around them.
	for k := range needles {
		addUp(apex, ends[k], ends[k+1])
	}
	addUp(apex, cornerLow, ends[0])
	addUp(apex, ends[last], cornerHigh)
	addUp(apex, cornerHigh, farHigh)
	addUp(apex, farHigh, farLow)
	// The slope: a pair of triangles on every needle's end, and the large
	// triangles at either side.
	for k := range needles {
		addUp(ends[k], ends[k+1], feet[k+1])
		addUp(ends[k], feet[k+1], feet[k])
	}
	addUp(cornerLow, ends[0], feet[0])
	addUp(cornerLow, feet[0], slopeLow)
	addUp(cornerHigh, slopeHigh, feet[last])
	addUp(cornerHigh, feet[last], ends[last])
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

func TestEdgesToleranceKeepsFlatFaceOfNarrowFacetsBesideSlope(t *testing.T) {
	t.Parallel()

	// The image is 12 pixels to the millimetre, so the plate's end is at
	// column 300 and y = 0 is row 300. The window lies on the plate, between
	// the fan's first edges and the crease, and spans most of the fan's length.
	inside := func(t *testing.T, tolerance float64) int {
		t.Helper()
		scene := solidlens.Scene{
			Camera: solidlens.Camera{Position: solidlens.Vec{Z: 60}, Up: solidlens.Vec{Y: 1}},
			Models: []solidlens.Model{{
				Mesh:     chamfered(t),
				Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
				Edges:    solidlens.Edges{Enabled: true, Color: solidlens.RGB(1, 0, 0), Tolerance: tolerance},
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0.5),
		}
		img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 600, Height: 600})
		require.NoError(t, err)
		count := 0
		for y := 305; y < 415; y++ {
			for x := 292; x < 298; x++ {
				if redPixel(img.RGBAAt(x, y)) {
					count++
				}
			}
		}
		return count
	}

	// Every triangle of the plate faces the same way, so no line crosses it
	// whatever the tolerance is. The fan's triangles are slivers with a
	// tolerance of 0.05 or more, and they must not take the slope's normal.
	for _, tolerance := range []float64{solidlens.Exact, 0.01, 0.05, 0.2} {
		require.Zero(t, inside(t, tolerance), "tolerance %v", tolerance)
	}
}

// bent returns a strip along the y axis made of a flat plate in the z = 0 plane
// that ends at x = 0, three narrow facets of a rounding that turn 20 degrees
// each, and a plate that continues at 60 degrees below the first. Every edge
// turns less than the crease angle, but the two plates meet at 60 degrees. The
// facets are 0.01 mm wide and 20 mm long, so they are slivers.
func bent(t *testing.T) *solidlens.Mesh {
	t.Helper()
	const length = 20.0
	const width = 0.01
	var vertices []solidlens.Vec
	var triangles [][3]int
	// slopes holds the angle of every strip below the horizontal, in degrees.
	slopes := []float64{0, 20, 40, 60, 60}
	lengths := []float64{20, width, width, width, 20}
	x, z := -20.0, 0.0
	add := func(x, z float64) (int, int) {
		vertices = append(vertices, solidlens.Vec{X: x, Y: -length / 2, Z: z}, solidlens.Vec{X: x, Y: length / 2, Z: z})
		return len(vertices) - 2, len(vertices) - 1
	}
	lowPrev, highPrev := add(x, z)
	for i, slope := range slopes {
		radians := slope * math.Pi / 180
		x += lengths[i] * math.Cos(radians)
		z -= lengths[i] * math.Sin(radians)
		low, high := add(x, z)
		// Both triangles of the strip face up and toward the camera.
		triangles = append(triangles, [3]int{lowPrev, low, high}, [3]int{lowPrev, high, highPrev})
		lowPrev, highPrev = low, high
	}
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

func TestEdgesToleranceKeepsCreaseAcrossBentSliverRounding(t *testing.T) {
	t.Parallel()

	// The image is 12 pixels to the millimetre, so the rounding is at column
	// 300 and y = 0 is row 300. The window spans the rounding and most of its
	// length.
	inside := func(t *testing.T, tolerance float64) int {
		t.Helper()
		scene := solidlens.Scene{
			Camera: solidlens.Camera{Position: solidlens.Vec{Z: 60}, Up: solidlens.Vec{Y: 1}},
			Models: []solidlens.Model{{
				Mesh:     bent(t),
				Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
				Edges:    solidlens.Edges{Enabled: true, Color: solidlens.RGB(1, 0, 0), Tolerance: tolerance},
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0.5),
		}
		img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 600, Height: 600})
		require.NoError(t, err)
		count := 0
		for y := 200; y < 400; y++ {
			for x := 294; x < 306; x++ {
				if redPixel(img.RGBAAt(x, y)) {
					count++
				}
			}
		}
		return count
	}

	// The rounding bends through more than the crease angle, so it belongs to
	// neither plate. With a tolerance wider than it, the plates' crease is
	// still drawn once. With none, no edge turns far enough to be drawn.
	require.Zero(t, inside(t, solidlens.Exact))
	for _, tolerance := range []float64{0.05, 0.2} {
		require.Positive(t, inside(t, tolerance), "tolerance %v", tolerance)
	}
}

// turnedStrips returns a plate in the z = 0 plane that ends at x = -0.01, a 0.01 mm
// wide strip beside it in the same plane, a second strip of the same width that
// turns by the given angle in degrees, and a plate that continues at that
// angle. Both strips are 20 mm long, so they are slivers, and each lies flat
// against the plate beside it.
func turnedStrips(t *testing.T, turn float64) *solidlens.Mesh {
	t.Helper()
	const length = 20.0
	const width = 0.01
	var vertices []solidlens.Vec
	var triangles [][3]int
	slopes := []float64{0, 0, turn, turn}
	lengths := []float64{20, width, width, 20}
	x, z := -20.0-width, 0.0
	add := func(x, z float64) (int, int) {
		vertices = append(vertices, solidlens.Vec{X: x, Y: -length / 2, Z: z}, solidlens.Vec{X: x, Y: length / 2, Z: z})
		return len(vertices) - 2, len(vertices) - 1
	}
	lowPrev, highPrev := add(x, z)
	for i, slope := range slopes {
		radians := slope * math.Pi / 180
		x += lengths[i] * math.Cos(radians)
		z -= lengths[i] * math.Sin(radians)
		low, high := add(x, z)
		triangles = append(triangles, [3]int{lowPrev, low, high}, [3]int{lowPrev, high, highPrev})
		lowPrev, highPrev = low, high
	}
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	return mesh
}

func TestEdgesToleranceDropsFacetStepBetweenSlivers(t *testing.T) {
	t.Parallel()

	// The image is 12 pixels to the millimetre, so the step is at column 300
	// and y = 0 is row 300. The window spans the step and most of its length.
	inside := func(t *testing.T, turn, tolerance float64) int {
		t.Helper()
		scene := solidlens.Scene{
			Camera: solidlens.Camera{Position: solidlens.Vec{Z: 60}, Up: solidlens.Vec{Y: 1}},
			Models: []solidlens.Model{{
				Mesh:     turnedStrips(t, turn),
				Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
				Edges:    solidlens.Edges{Enabled: true, Color: solidlens.RGB(1, 0, 0), Tolerance: tolerance},
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0.5),
		}
		img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 600, Height: 600})
		require.NoError(t, err)
		count := 0
		for y := 200; y < 400; y++ {
			for x := 294; x < 306; x++ {
				if redPixel(img.RGBAAt(x, y)) {
					count++
				}
			}
		}
		return count
	}

	// A turn of 35 degrees is a little beyond the crease angle, so with a
	// tolerance wider than the strips it is a facet of a curve. Without a
	// tolerance, and for a turn of 50 degrees, it is drawn.
	require.Zero(t, inside(t, 35, 0.05))
	require.Positive(t, inside(t, 35, solidlens.Exact))
	require.Positive(t, inside(t, 50, 0.05))
}

// plainSource is a TriangleSource that is not a *solidlens.Mesh.
type plainSource struct {
	vertices  []solidlens.Vec
	triangles [][3]int
}

func (s plainSource) Vertices() []solidlens.Vec { return s.vertices }
func (s plainSource) Triangles() [][3]int       { return s.triangles }
