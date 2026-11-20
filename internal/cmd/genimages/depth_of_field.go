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

// writeDepthOfField renders the depth of field samples for the README.
func writeDepthOfField() error {
	for _, render := range []struct {
		name  string
		scene solidlens.Scene
	}{
		{name: "depth-of-field-sharp.png", scene: depthOfFieldScene(nil)},
		{name: "depth-of-field-bokeh.png", scene: depthOfFieldScene(
			&solidlens.DepthOfField{FocusDistance: 16, Aperture: 0.7, Blades: 6})},
		{name: "depth-of-field-lights.png", scene: bokehLightsScene(80)},
	} {
		file, err := os.Create(filepath.Join(outputDir, render.name)) //nolint:gosec
		if err != nil {
			return err
		}
		err = solidlens.RenderPNG(context.Background(), file, render.scene,
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

// bokehLightsScene is a lit sphere in focus in front of a field of small
// glowing spheres far behind it. Each one blurs into a hexagon.
func bokehLightsScene(highlight float64) solidlens.Scene {
	scene := depthOfFieldScene(nil)
	scene.Models = []solidlens.Model{{
		Mesh:     sphere(solidlens.Vec{X: -0.6, Y: 4, Z: 1.0}, 1.2, 64, 32),
		Material: solidlens.Matte(solidlens.RGB(0.2, 0.75, 0.85)),
	}}
	warm := []solidlens.Color{
		solidlens.RGB(1, 0.85, 0.55), solidlens.RGB(1, 0.6, 0.45), solidlens.RGB(0.85, 0.95, 1),
	}
	for i := range 28 {
		// A scattered but repeatable layout.
		x := -24 + 48*float64((i*37)%28)/27
		z := -9 + 17*float64((i*11)%28)/27
		depth := 30 + float64((i*5)%7)*4
		scene.Models = append(scene.Models, solidlens.Model{
			Mesh:     sphere(solidlens.Vec{X: x, Y: depth, Z: z}, 0.4, 12, 8),
			Material: solidlens.Material{Color: warm[i%len(warm)], Unlit: true},
		})
	}
	scene.DepthOfField = &solidlens.DepthOfField{FocusDistance: 12, Aperture: 0.8, Blades: 6, Highlight: highlight}
	return scene
}
