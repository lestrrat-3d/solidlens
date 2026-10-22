package examples_test

import (
	"context"
	"fmt"

	"github.com/lestrrat-3d/solidlens"
)

func Example_solidlens_edges_tolerance() {
	// A flat plate with a strip a ten-thousandth of a millimetre wide across
	// its middle. The strip rises a thousandth, so it is steep, but it is far
	// too narrow to see.
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{
			{X: -10, Y: -10}, {X: 0, Y: -10}, {X: 0, Y: 10}, {X: -10, Y: 10},
			{X: 1e-4, Y: -10, Z: 1e-3}, {X: 1e-4, Y: 10, Z: 1e-3},
			{X: 10, Y: -10, Z: 1e-3}, {X: 10, Y: 10, Z: 1e-3},
		},
		[][3]int{{0, 1, 2}, {0, 2, 3}, {1, 4, 5}, {1, 5, 2}, {4, 6, 7}, {4, 7, 5}},
	)
	if err != nil {
		fmt.Printf("failed to create mesh: %s\n", err)
		return
	}

	// stripLines renders the plate and counts edge pixels in a window that
	// holds the strip and stays clear of the plate's border.
	stripLines := func(edges solidlens.Edges) (int, error) {
		scene := solidlens.Scene{
			Camera: solidlens.Camera{Position: solidlens.Vec{Z: 60}, Up: solidlens.Vec{Y: 1}},
			Models: []solidlens.Model{{
				Mesh:     mesh,
				Material: solidlens.Matte(solidlens.RGB(0.9, 0.9, 0.9)),
				Edges:    edges,
			}},
			DirectionalLights: []solidlens.DirectionalLight{{
				Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1,
			}},
			Background: solidlens.RGB(0, 0, 0),
		}
		image, err := solidlens.Render(context.Background(), scene, solidlens.Settings{Width: 600, Height: 600})
		if err != nil {
			return 0, err
		}
		red := 0
		for y := 240; y < 360; y++ {
			for x := 240; x < 360; x++ {
				if pixel := image.RGBAAt(x, y); int(pixel.R) > int(pixel.G)+30 {
					red++
				}
			}
		}
		return red, nil
	}

	// By default a facet narrower than half a pixel does not make a line.
	byDefault, err := stripLines(solidlens.Outline(solidlens.RGB(1, 0, 0)))
	if err != nil {
		fmt.Printf("failed to render: %s\n", err)
		return
	}

	// Exact draws the mesh as it is, so the strip shows as two lines.
	exact := solidlens.Outline(solidlens.RGB(1, 0, 0))
	exact.Tolerance = solidlens.Exact
	asIs, err := stripLines(exact)
	if err != nil {
		fmt.Printf("failed to render: %s\n", err)
		return
	}

	fmt.Printf("lines by default: %t\n", byDefault > 0)
	fmt.Printf("lines when exact: %t\n", asIs > 0)

	// Output:
	// lines by default: false
	// lines when exact: true
}
