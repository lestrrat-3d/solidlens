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
