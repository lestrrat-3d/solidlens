package main

import (
	"github.com/lestrrat-3d/solidlens"
)

// cutawayScene is a torus and a cylinder, each cut open along the y = 0 plane. The studio camera sits at
// negative Y, on the removed side, so it looks into both solids; each flat cap is the colour of its model.
func cutawayScene() (solidlens.Scene, error) {
	cut := func(c solidlens.Color) *solidlens.ClipPlane {
		return &solidlens.ClipPlane{
			Normal: solidlens.Vec{Y: -1},
			Cap:    &solidlens.Material{Color: lighten(c, 0.25), Unlit: true},
		}
	}
	coral := solidlens.RGB(0.96, 0.4, 0.34)
	cyan := solidlens.RGB(0.2, 0.75, 0.85)
	ring := outlined(solidlens.Model{
		Mesh:     torus(solidlens.Vec{Z: 1.2}, 2.4, 0.9, 72, 36),
		Material: solidlens.Matte(coral),
		Clip:     cut(coral),
	})
	post := outlined(solidlens.Model{
		Mesh:     cylinder(solidlens.Vec{Z: 0}, 1, 2.6, 48),
		Material: solidlens.Matte(cyan),
		Clip:     cut(cyan),
	})
	return studioScene(ring, post), nil
}
