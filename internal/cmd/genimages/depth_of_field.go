package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/lestrrat-3d/solidlens"
)

// depthOfFieldScene is a row of spheres receding from the camera, which looks
// along +Y. The sphere at y = 8 is 16 units away and sits on the focal plane.
func depthOfFieldScene(lens *solidlens.DepthOfField) solidlens.Scene {
	colors := []solidlens.Color{
		solidlens.RGB(0.96, 0.4, 0.34),
		solidlens.RGB(0.95, 0.75, 0.25),
		solidlens.RGB(0.2, 0.75, 0.85),
		solidlens.RGB(0.55, 0.4, 0.95),
		solidlens.RGB(0.96, 0.23, 0.54),
		solidlens.RGB(0.17, 0.76, 0.54),
	}
	xs := []float64{-2.2, 2.2, -0.4, 4.2, -5.5, 6}
	ys := []float64{-2, 3, 8, 13, 19, 26}
	var models []solidlens.Model
	for i := range colors {
		models = append(models, solidlens.Model{
			Mesh:     sphere(solidlens.Vec{X: xs[i], Y: ys[i], Z: 1.2}, 1, 48, 24),
			Material: solidlens.Matte(colors[i]),
		})
	}
	return solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{Y: -8, Z: 1.2},
			Target:   solidlens.Vec{Y: 10, Z: 1.2},
			Up:       solidlens.Vec{Z: 1},
			FOV:      38,
		},
		Models: models,
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{X: -0.5, Y: 0.6, Z: -0.7},
			Color:     solidlens.RGB(1, 1, 1),
			Intensity: 1.2,
		}},
		PointLights: []solidlens.PointLight{{
			Position:  solidlens.Vec{X: -3, Y: -6, Z: 6},
			Color:     solidlens.RGB(0.5, 0.7, 1),
			Intensity: 13,
		}},
		Background:   solidlens.RGB(0.004, 0.01, 0.035),
		DepthOfField: lens,
	}
}

// writeDepthOfField renders the same scene sharp and with a hexagonal blur.
func writeDepthOfField() error {
	for _, render := range []struct {
		name string
		lens *solidlens.DepthOfField
	}{
		{name: "depth-of-field-sharp.png"},
		{name: "depth-of-field-bokeh.png", lens: &solidlens.DepthOfField{FocusDistance: 16, Aperture: 0.7, Blades: 6}},
	} {
		file, err := os.Create(filepath.Join(outputDir, render.name)) //nolint:gosec
		if err != nil {
			return err
		}
		err = solidlens.RenderPNG(context.Background(), file, depthOfFieldScene(render.lens),
			solidlens.Settings{Width: 1200, Height: 675})
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
