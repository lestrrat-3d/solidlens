package examples_test

import (
	"context"
	"fmt"

	"github.com/lestrrat-3d/solidlens"
)

func Example_solidlens_edges() {
	// A square made of two triangles. Its four borders are drawn, and so is
	// its silhouette, but the shared diagonal is flat and stays invisible.
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}},
		[][3]int{{0, 1, 2}, {0, 2, 3}},
	)
	if err != nil {
		fmt.Printf("failed to create mesh: %s\n", err)
		return
	}

	scene := solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{Z: 4},
			Target:   solidlens.Vec{},
			Up:       solidlens.Vec{Y: 1},
		},
		Models: []solidlens.Model{{
			Mesh:     mesh,
			Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
			// Outline enables edges with the default width of one pixel and
			// the default crease angle of thirty degrees. Set Width and
			// CreaseAngle on solidlens.Edges directly to change either.
			Edges: solidlens.Outline(solidlens.RGB(1, 0, 0)),
		}},
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{Z: -1},
			Color:     solidlens.RGB(1, 1, 1),
			Intensity: 1,
		}},
		Background: solidlens.RGB(0, 0, 0),
	}
	image, err := solidlens.Render(context.Background(), scene, solidlens.Settings{Width: 64, Height: 64})
	if err != nil {
		fmt.Printf("failed to render: %s\n", err)
		return
	}

	// The border shows up as red pixels around the square, while its centre
	// keeps the lit material color.
	red := 0
	for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
		for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
			if pixel := image.RGBAAt(x, y); int(pixel.R) > int(pixel.G)+30 {
				red++
			}
		}
	}
	centre := image.RGBAAt(32, 32)
	fmt.Printf("edge pixels drawn: %t\n", red > 0)
	fmt.Printf("centre is red: %t\n", int(centre.R) > int(centre.G)+30)

	// Output:
	// edge pixels drawn: true
	// centre is red: false
}
