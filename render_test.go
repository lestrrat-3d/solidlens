package solidlens_test

import (
	"bytes"
	"context"
	"image/color"
	"math"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

func TestRenderDrawsLitTriangle(t *testing.T) {
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {Y: 1}},
		[][3]int{{0, 1, 2}},
	)
	require.NoError(t, err)
	image, err := solidlens.Render(t.Context(), solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{Z: 2},
			Target:   solidlens.Vec{},
			Up:       solidlens.Vec{Y: 1},
		},
		Models: []solidlens.Model{{Mesh: mesh, Material: solidlens.Matte(solidlens.RGB(1, 0, 0))}},
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{Z: -1},
			Color:     solidlens.RGB(1, 1, 1),
			Intensity: 1,
		}},
		Background: solidlens.RGB(0, 0, 0),
	}, solidlens.Settings{Width: 64, Height: 64})
	require.NoError(t, err)
	require.Greater(t, image.RGBAAt(32, 32).R, uint8(200))
	require.Equal(t, color.RGBA{A: 255}, image.RGBAAt(0, 0))

	var png bytes.Buffer
	err = solidlens.RenderPNG(t.Context(), &png, solidlens.Scene{
		Camera: solidlens.Camera{Position: solidlens.Vec{Z: 2}, Target: solidlens.Vec{}, Up: solidlens.Vec{Y: 1}},
		Models: []solidlens.Model{{Mesh: mesh, Material: solidlens.Matte(solidlens.RGB(1, 0, 0))}},
	}, solidlens.Settings{Width: 8, Height: 8})
	require.NoError(t, err)
	require.NotEmpty(t, png.Bytes())
}

func TestRenderShadesBackSide(t *testing.T) {
	// The triangle winds counter-clockwise seen from +Z, so a camera and a
	// light on -Z see and light its back side.
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {Y: 1}},
		[][3]int{{0, 1, 2}},
	)
	require.NoError(t, err)
	blue := solidlens.Matte(solidlens.RGB(0, 0, 1))
	render := func(t *testing.T, cameraZ float64, back *solidlens.Material) color.RGBA {
		t.Helper()
		image, err := solidlens.Render(t.Context(), solidlens.Scene{
			Camera: solidlens.Camera{
				Position: solidlens.Vec{Z: cameraZ},
				Target:   solidlens.Vec{},
				Up:       solidlens.Vec{Y: 1},
			},
			Models: []solidlens.Model{{
				Mesh:         mesh,
				Material:     solidlens.Matte(solidlens.RGB(1, 0, 0)),
				BackMaterial: back,
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{Z: -cameraZ},
				Color:     solidlens.RGB(1, 1, 1),
				Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0),
		}, solidlens.Settings{Width: 64, Height: 64})
		require.NoError(t, err)
		// The triangle's centroid sits a little below the image centre.
		return image.RGBAAt(32, 36)
	}

	t.Run("lit with the front material", func(t *testing.T) {
		pixel := render(t, -2, nil)
		require.Greater(t, pixel.R, uint8(200))
		require.Zero(t, pixel.B)
	})
	t.Run("lit with the back material", func(t *testing.T) {
		pixel := render(t, -2, &blue)
		require.Greater(t, pixel.B, uint8(200))
		require.Zero(t, pixel.R)
	})
	t.Run("front side ignores the back material", func(t *testing.T) {
		pixel := render(t, 2, &blue)
		require.Greater(t, pixel.R, uint8(200))
		require.Zero(t, pixel.B)
	})
	t.Run("invalid back material", func(t *testing.T) {
		_, err := solidlens.Render(t.Context(), solidlens.Scene{
			Camera: solidlens.Camera{Position: solidlens.Vec{Z: 2}, Target: solidlens.Vec{}, Up: solidlens.Vec{Y: 1}},
			Models: []solidlens.Model{{
				Mesh:         mesh,
				Material:     solidlens.Matte(solidlens.RGB(1, 0, 0)),
				BackMaterial: &solidlens.Material{Color: solidlens.RGB(math.NaN(), 0, 0)},
			}},
		}, solidlens.Settings{Width: 8, Height: 8})
		require.ErrorContains(t, err, "back material")
	})
}

func TestRenderHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := solidlens.Render(ctx, solidlens.Scene{}, solidlens.Settings{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestRenderPreservesAlpha(t *testing.T) {
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {Y: 1}},
		[][3]int{{0, 1, 2}},
	)
	require.NoError(t, err)
	background := solidlens.RGBA(0.7, 0.2, 0.1, 0.4)
	surface := solidlens.RGBA(0.2, 0.7, 0.4, 0.6)
	image, err := solidlens.Render(t.Context(), solidlens.Scene{
		Camera: solidlens.Camera{Position: solidlens.Vec{Z: 2}, Up: solidlens.Vec{Y: 1}},
		Models: []solidlens.Model{{
			Mesh:     mesh,
			Material: solidlens.Material{Color: surface, Ambient: 1},
		}},
		Background: background,
	}, solidlens.Settings{Width: 64, Height: 64})
	require.NoError(t, err)
	require.Equal(t, color.RGBAModel.Convert(background.NRGBA()), image.RGBAAt(0, 0))
	require.Equal(t, color.RGBAModel.Convert(surface.NRGBA()), image.RGBAAt(32, 32))
}

func TestRenderPointLight(t *testing.T) {
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {Y: 1}},
		[][3]int{{0, 1, 2}},
	)
	require.NoError(t, err)
	scene := solidlens.Scene{
		Camera: solidlens.Camera{Position: solidlens.Vec{Z: 2}, Up: solidlens.Vec{Y: 1}},
		Models: []solidlens.Model{{
			Mesh:     mesh,
			Material: solidlens.Matte(solidlens.RGB(1, 1, 1)),
		}},
		PointLights: []solidlens.PointLight{{
			Position:  solidlens.Vec{Z: 2},
			Color:     solidlens.RGB(1, 1, 1),
			Intensity: 4,
		}},
	}
	settings := solidlens.Settings{Width: 64, Height: 64}
	lit, err := solidlens.Render(t.Context(), scene, settings)
	require.NoError(t, err)
	scene.PointLights = nil
	unlit, err := solidlens.Render(t.Context(), scene, settings)
	require.NoError(t, err)
	require.Greater(t, lit.RGBAAt(32, 32).R, unlit.RGBAAt(32, 32).R)

	scene.PointLights = []solidlens.PointLight{
		{Position: solidlens.Vec{Z: 2}, Color: solidlens.RGB(1, 1, 1), Intensity: 4},
		{Position: solidlens.Vec{X: math.NaN()}, Color: solidlens.RGB(1, 1, 1), Intensity: 4},
	}
	withInvalid, err := solidlens.Render(t.Context(), scene, settings)
	require.NoError(t, err)
	require.Equal(t, lit.Pix, withInvalid.Pix)
}

func TestRenderSharedTriangleEdgeHasNoGaps(t *testing.T) {
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{
			{X: -1, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: -1},
		},
		[][3]int{{0, 1, 2}, {0, 2, 3}},
	)
	require.NoError(t, err)
	image, err := solidlens.Render(t.Context(), solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{Z: 2}, Up: solidlens.Vec{Y: 1}, FOV: 90,
		},
		Models: []solidlens.Model{{
			Mesh: mesh, Material: solidlens.Material{Color: solidlens.RGB(1, 0, 0), Ambient: 1},
		}},
	}, solidlens.Settings{Width: 32, Height: 32})
	require.NoError(t, err)
	for pixel := 8; pixel < 24; pixel++ {
		require.Equal(t, uint8(255), image.RGBAAt(pixel, pixel).R, "shared edge pixel %d", pixel)
	}
}

// TestRenderDepthIsPerspectiveCorrect lays a red plate of small triangles 0.2 mm
// under a green plate of two large triangles and views them at a slant from
// close by. The green plate is nearer along every ray, so no red may show. A
// depth interpolated linearly across the screen would put the middle of each
// large triangle several millimetres too far away.
func TestRenderDepthIsPerspectiveCorrect(t *testing.T) {
	t.Parallel()

	plate := func(t *testing.T, half float64, cells int, z float64) *solidlens.Mesh {
		t.Helper()
		var vertices []solidlens.Vec
		var triangles [][3]int
		step := 2 * half / float64(cells)
		for j := 0; j <= cells; j++ {
			for i := 0; i <= cells; i++ {
				vertices = append(vertices, solidlens.Vec{X: -half + float64(i)*step, Y: -half + float64(j)*step, Z: z})
			}
		}
		for j := range cells {
			for i := range cells {
				a := j*(cells+1) + i
				b, c, d := a+1, a+cells+1, a+cells+2
				triangles = append(triangles, [3]int{a, b, d}, [3]int{a, d, c})
			}
		}
		mesh, err := solidlens.NewMesh(vertices, triangles)
		require.NoError(t, err)
		return mesh
	}

	scene := solidlens.Scene{
		Camera: solidlens.Camera{Position: solidlens.Vec{Y: -25, Z: 12}, Up: solidlens.Vec{Z: 1}},
		Models: []solidlens.Model{
			{Mesh: plate(t, 10, 1, 0), Material: solidlens.Matte(solidlens.RGB(0, 0.9, 0))},
			{Mesh: plate(t, 8, 16, -0.2), Material: solidlens.Matte(solidlens.RGB(0.9, 0, 0))},
		},
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
		}},
		Background: solidlens.RGB(0, 0, 0.5),
	}
	img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 400, Height: 400})
	require.NoError(t, err)
	require.Zero(t, countRed(img))
}
