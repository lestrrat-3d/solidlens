package examples_test

import (
	"context"
	"fmt"

	"github.com/lestrrat-3d/solidlens"
)

func Example_solidlens_section() {
	// Two 10 mm squares overlap by a 5 mm wide strip on the plane z = 0.
	square := func(x float64) []solidlens.Vec {
		return []solidlens.Vec{{X: x}, {X: x + 10}, {X: x + 10, Y: 10}, {X: x, Y: 10}}
	}
	view := solidlens.SectionView{
		Plane: solidlens.SectionPlane{U: solidlens.Vec{X: 1}, V: solidlens.Vec{Y: 1}},
		Layers: []solidlens.SectionLayer{
			{Name: "gear", Loops: [][]solidlens.Vec{square(0)}, Fill: solidlens.RGBA(0.1, 0.4, 1, 0.6)},
			{Name: "rack", Loops: [][]solidlens.Vec{square(5)}, Fill: solidlens.RGBA(0.1, 0.8, 0.3, 0.6)},
		},
		Caption: "overlap of two squares",
	}
	img, report, err := solidlens.RenderSection(context.Background(), view)
	if err != nil {
		fmt.Printf("failed to render section: %s\n", err)
		return
	}
	fmt.Printf("image: %dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())
	fmt.Printf("gear area: %.1f mm2\n", report.Layers[0].Area)
	fmt.Printf("overlap area: about %.0f mm2\n", report.OverlapArea)

	// Output:
	// image: 1000x800
	// gear area: 100.0 mm2
	// overlap area: about 50 mm2
}
