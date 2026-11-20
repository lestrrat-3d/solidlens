package solidlens_test

import (
	"bytes"
	"context"
	"image"
	"math"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

// depthOfFieldScene returns a square facing the camera, which sits distance
// away from it.
func depthOfFieldScene(t *testing.T, distance float64, lens *solidlens.DepthOfField) solidlens.Scene {
	t.Helper()
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}},
		[][3]int{{0, 1, 2}, {0, 2, 3}},
	)
	require.NoError(t, err)
	return solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{Z: distance},
			Up:       solidlens.Vec{Y: 1},
		},
		Models:       []solidlens.Model{{Mesh: mesh, Material: solidlens.Matte(solidlens.RGB(0.9, 0.2, 0.2))}},
		Background:   solidlens.RGB(0, 0, 0),
		DepthOfField: lens,
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
		}},
	}
}

// litPixels counts the pixels that are not the black background.
func litPixels(img *image.RGBA) int {
	count := 0
	for offset := 0; offset < len(img.Pix); offset += 4 {
		if img.Pix[offset] != 0 || img.Pix[offset+1] != 0 || img.Pix[offset+2] != 0 {
			count++
		}
	}
	return count
}

// redTotal sums the red channel over the image.
func redTotal(img *image.RGBA) float64 {
	var total float64
	for offset := 0; offset < len(img.Pix); offset += 4 {
		total += float64(img.Pix[offset])
	}
	return total
}

func TestDepthOfField(t *testing.T) {
	settings := solidlens.Settings{Width: 160, Height: 120}

	t.Run("surface on the focal plane is unchanged", func(t *testing.T) {
		sharp, err := solidlens.Render(t.Context(), depthOfFieldScene(t, 5, nil), settings)
		require.NoError(t, err)
		// The background is drawn at infinity, so it blurs, but it is black and
		// stays black. Only the square can differ.
		focused, err := solidlens.Render(t.Context(),
			depthOfFieldScene(t, 5, &solidlens.DepthOfField{FocusDistance: 5, Aperture: 0.2}), settings)
		require.NoError(t, err)
		require.Equal(t, litPixels(sharp), litPixels(focused))
	})

	t.Run("surface off the focal plane spreads out", func(t *testing.T) {
		sharp, err := solidlens.Render(t.Context(), depthOfFieldScene(t, 5, nil), settings)
		require.NoError(t, err)
		blurred, err := solidlens.Render(t.Context(),
			depthOfFieldScene(t, 5, &solidlens.DepthOfField{FocusDistance: 12, Aperture: 1}), settings)
		require.NoError(t, err)
		require.Greater(t, litPixels(blurred), litPixels(sharp))
		// The blur redistributes light without adding or removing much of it.
		require.InEpsilon(t, redTotal(sharp), redTotal(blurred), 0.05)
	})

	t.Run("polygon aperture differs from circle", func(t *testing.T) {
		circle, err := solidlens.Render(t.Context(),
			depthOfFieldScene(t, 5, &solidlens.DepthOfField{FocusDistance: 12, Aperture: 1}), settings)
		require.NoError(t, err)
		hexagon, err := solidlens.Render(t.Context(),
			depthOfFieldScene(t, 5, &solidlens.DepthOfField{FocusDistance: 12, Aperture: 1, Blades: 6}), settings)
		require.NoError(t, err)
		require.False(t, bytes.Equal(circle.Pix, hexagon.Pix))
	})

	t.Run("max radius limits the spread", func(t *testing.T) {
		wide, err := solidlens.Render(t.Context(),
			depthOfFieldScene(t, 5, &solidlens.DepthOfField{FocusDistance: 12, Aperture: 1, MaxRadius: 20}), settings)
		require.NoError(t, err)
		narrow, err := solidlens.Render(t.Context(),
			depthOfFieldScene(t, 5, &solidlens.DepthOfField{FocusDistance: 12, Aperture: 1, MaxRadius: 2}), settings)
		require.NoError(t, err)
		require.Greater(t, litPixels(wide), litPixels(narrow))
	})

	t.Run("cancelled context stops the blur", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := solidlens.Render(ctx,
			depthOfFieldScene(t, 5, &solidlens.DepthOfField{FocusDistance: 12, Aperture: 1}), settings)
		require.Error(t, err)
	})
}

func TestDepthOfFieldRejectsInvalidLens(t *testing.T) {
	tests := map[string]solidlens.DepthOfField{
		"zero focus distance":     {Aperture: 1},
		"negative focus distance": {FocusDistance: -1, Aperture: 1},
		"NaN focus distance":      {FocusDistance: math.NaN(), Aperture: 1},
		"zero aperture":           {FocusDistance: 5},
		"infinite aperture":       {FocusDistance: 5, Aperture: math.Inf(1)},
		"two blades":              {FocusDistance: 5, Aperture: 1, Blades: 2},
		"too many blades":         {FocusDistance: 5, Aperture: 1, Blades: 17},
		"negative blades":         {FocusDistance: 5, Aperture: 1, Blades: -3},
		"negative radius":         {FocusDistance: 5, Aperture: 1, MaxRadius: -1},
		"radius over limit":       {FocusDistance: 5, Aperture: 1, MaxRadius: 65},
	}
	for name, lens := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := solidlens.Render(t.Context(), depthOfFieldScene(t, 5, &lens),
				solidlens.Settings{Width: 32, Height: 24})
			require.Error(t, err)
		})
	}
}
