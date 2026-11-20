package examples_test

import (
	"context"
	"fmt"

	"github.com/lestrrat-3d/solidlens"
)

func Example_solidlens_depth_of_field() {
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
			Position: solidlens.Vec{Z: 4},
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
		// Focus well behind the triangle so it renders as a soft hexagonal
		// blur. A focus distance of 4 would keep it sharp.
		DepthOfField: &solidlens.DepthOfField{FocusDistance: 10, Aperture: 0.5, Blades: 6},
	}
	img, err := solidlens.Render(context.Background(), scene, solidlens.Settings{Width: 160, Height: 120})
	if err != nil {
		fmt.Printf("failed to render: %s\n", err)
		return
	}
	fmt.Printf("rendered %dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())

	// Output:
	// rendered 160x120
}
