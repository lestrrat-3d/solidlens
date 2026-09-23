package examples_test

import (
	"context"
	"fmt"

	"github.com/lestrrat-3d/solidlens"
)

func Example_solidlens_sheet() {
	// An open square tube: four walls and no caps, like the sheet body a
	// decad surface extrude builds. Each wall winds counter-clockwise seen
	// from outside, so its outer side is the front.
	vertices := []solidlens.Vec{
		{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1},
		{X: -1, Y: -1, Z: 2}, {X: 1, Y: -1, Z: 2}, {X: 1, Y: 1, Z: 2}, {X: -1, Y: 1, Z: 2},
	}
	var triangles [][3]int
	for i := range 4 {
		next := (i + 1) % 4
		triangles = append(triangles, [3]int{i, next, next + 4}, [3]int{i, next + 4, i + 4})
	}
	mesh, err := solidlens.NewMesh(vertices, triangles)
	if err != nil {
		fmt.Printf("failed to create mesh: %s\n", err)
		return
	}

	// Looking down into the tube shows only the inner side of its walls.
	// BackMaterial paints that side blue, while Material stays orange.
	back := solidlens.Matte(solidlens.RGB(0.1, 0.3, 1))
	scene := solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{Z: 6},
			Target:   solidlens.Vec{},
			Up:       solidlens.Vec{Y: 1},
		},
		Models: []solidlens.Model{{
			Mesh:         mesh,
			Material:     solidlens.Matte(solidlens.RGB(1, 0.5, 0.1)),
			BackMaterial: &back,
		}},
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{X: 0.5, Y: 0.5, Z: -1},
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

	blue, orange := 0, 0
	for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
		for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
			pixel := image.RGBAAt(x, y)
			switch {
			case int(pixel.B) > int(pixel.R)+30:
				blue++
			case int(pixel.R) > int(pixel.B)+30:
				orange++
			}
		}
	}
	centre := image.RGBAAt(32, 32)
	fmt.Printf("inner walls drawn: %t\n", blue > 0)
	fmt.Printf("outer walls drawn: %t\n", orange > 0)
	fmt.Printf("centre shows the background: %t\n", centre.R == 0 && centre.G == 0 && centre.B == 0)

	// Output:
	// inner walls drawn: true
	// outer walls drawn: false
	// centre shows the background: true
}
