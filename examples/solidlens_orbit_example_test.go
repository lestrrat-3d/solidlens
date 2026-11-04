package examples_test

import (
	"bytes"
	"context"
	"fmt"

	"github.com/lestrrat-3d/solidlens"
)

func Example_solidlens_orbit() {
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {Y: 1}, {Z: 2}},
		[][3]int{{0, 2, 1}, {0, 1, 3}, {1, 2, 3}, {2, 0, 3}},
	)
	if err != nil {
		fmt.Printf("failed to create mesh: %s\n", err)
		return
	}
	bounds, err := solidlens.BoundsOf(mesh)
	if err != nil {
		fmt.Printf("failed to measure mesh: %s\n", err)
		return
	}

	// An Orbit is a plain value. A viewer that rotates on drag changes Yaw and
	// Pitch, and one that zooms scales Distance. Each change is rendered as a
	// new scene.
	orbit := solidlens.FitOrbit(bounds, 0)
	orbit.Yaw += 15
	orbit.Pitch -= 5
	orbit.Distance *= 0.8

	camera := orbit.Camera()
	scene := solidlens.Scene{
		Camera: camera,
		Models: []solidlens.Model{{
			Mesh:     mesh,
			Material: solidlens.Matte(solidlens.RGB(0.05, 0.45, 0.85)),
		}},
		// Lights placed from the camera keep the visible side lit as it moves.
		DirectionalLights: solidlens.CameraLights(camera),
	}
	var frame bytes.Buffer
	if err := solidlens.RenderPNG(context.Background(), &frame, scene, solidlens.Settings{Width: 160, Height: 120}); err != nil {
		fmt.Printf("failed to render view: %s\n", err)
		return
	}
	fmt.Printf("PNG bytes generated: %t\n", frame.Len() > 0)

	// Output:
	// PNG bytes generated: true
}
