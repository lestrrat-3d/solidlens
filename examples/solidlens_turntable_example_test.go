package examples_test

import (
	"bytes"
	"context"
	"fmt"
	"image/gif"

	"github.com/lestrrat-3d/solidlens"
)

func Example_solidlens_turntable() {
	mesh, err := solidlens.NewMesh(
		[]solidlens.Vec{{X: -1, Y: -1}, {X: 1, Y: -1}, {Y: 1}, {Z: 2}},
		[][3]int{{0, 2, 1}, {0, 1, 3}, {1, 2, 3}, {2, 0, 3}},
	)
	if err != nil {
		fmt.Printf("failed to create mesh: %s\n", err)
		return
	}

	// Each step is a separate piece, so any of them can be swapped for your own.
	// Bounds measure the model, FitOrbit places a camera that frames it, and a
	// Turntable turns that camera once around the scene you describe.
	bounds, err := solidlens.BoundsOf(mesh)
	if err != nil {
		fmt.Printf("failed to measure mesh: %s\n", err)
		return
	}
	turntable := solidlens.Turntable{
		Base: solidlens.Scene{
			Models: []solidlens.Model{{
				Mesh:     mesh,
				Material: solidlens.Matte(solidlens.RGB(0.05, 0.45, 0.85)),
				Edges:    solidlens.Outline(solidlens.RGB(0.01, 0.01, 0.02)),
			}},
			Background: solidlens.RGB(0.02, 0.02, 0.03),
		},
		Orbit:  solidlens.FitOrbit(bounds, 0),
		Frames: 12,
	}

	// RenderGIF takes any SceneSource, and a GIF is what a GitHub README shows
	// inline. Write to a file instead of a buffer to commit it with the README.
	var out bytes.Buffer
	if err := solidlens.RenderGIF(context.Background(), &out, turntable,
		solidlens.GIFOptions{Width: 160, Height: 120}); err != nil {
		fmt.Printf("failed to render GIF: %s\n", err)
		return
	}
	anim, err := gif.DecodeAll(&out)
	if err != nil {
		fmt.Printf("failed to decode GIF: %s\n", err)
		return
	}
	fmt.Printf("frames: %d\n", len(anim.Image))

	// Output:
	// frames: 12
}
