package solidlens_test

import (
	"bytes"
	"context"
	"image/color"
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

func TestRenderHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := solidlens.Render(ctx, solidlens.Scene{}, solidlens.Settings{})
	require.ErrorIs(t, err, context.Canceled)
}
