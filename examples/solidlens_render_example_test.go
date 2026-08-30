package examples_test

import (
	"bytes"
	"context"
	"fmt"

	"github.com/lestrrat-3d/solidlens"
)

func Example_solidlens_render() {
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {Y: 1}},
		[][3]int{{0, 1, 2}},
	)
	if err != nil {
		fmt.Printf("failed to create mesh: %s\n", err)
		return
	}

	scene := solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{Z: 2},
			Target:   solidlens.Vec{},
			Up:       solidlens.Vec{Y: 1},
		},
		Models: []solidlens.Model{{
			Mesh:     mesh,
			Material: solidlens.Matte(solidlens.RGB(0.1, 0.5, 0.9)),
		}},
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{Z: -1},
			Color:     solidlens.RGB(1, 1, 1),
			Intensity: 1,
		}},
		Background: solidlens.RGB(0.02, 0.02, 0.03),
	}
	var output bytes.Buffer
	if err := solidlens.RenderPNG(context.Background(), &output, scene, solidlens.Settings{Width: 320, Height: 240}); err != nil {
		fmt.Printf("failed to render PNG: %s\n", err)
		return
	}
	fmt.Printf("PNG bytes generated: %t\n", output.Len() > 0)

	// Output:
	// PNG bytes generated: true
}
