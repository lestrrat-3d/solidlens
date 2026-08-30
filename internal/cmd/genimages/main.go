// Command genimages renders the README gallery with solidlens itself.
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/lestrrat-3d/solidlens"
)

const outputDir = "docs/images"

func main() {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create image directory: %s\n", err)
		os.Exit(1)
	}
	for _, render := range []galleryRender{
		{name: "hero.png", scene: heroScene()},
		{name: "mechanical.png", scene: mechanicalScene()},
		{name: "forms.png", scene: formsScene()},
	} {
		if err := render.write(); err != nil {
			fmt.Fprintf(os.Stderr, "render %s: %s\n", render.name, err)
			os.Exit(1)
		}
	}
}

type galleryRender struct {
	name  string
	scene solidlens.Scene
}

func (r galleryRender) write() error {
	file, err := os.Create(filepath.Join(outputDir, r.name)) //nolint:gosec
	if err != nil {
		return err
	}
	err = solidlens.RenderPNG(context.Background(), file, r.scene, solidlens.Settings{Width: 1440, Height: 810})
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func heroScene() solidlens.Scene {
	return studioScene(
		solidlens.Model{Mesh: torus(solidlens.Vec{X: -2.25, Z: 1.3}, 0.95, 0.29, 36, 14), Material: solidlens.Matte(solidlens.RGB(0.04, 0.78, 0.88))},
		solidlens.Model{Mesh: boxMesh(solidlens.Vec{Z: 1.1}, solidlens.Vec{X: 1.44, Y: 1.44, Z: 2.04}), Material: solidlens.Matte(solidlens.RGB(0.47, 0.21, 0.93))},
		solidlens.Model{Mesh: cylinder(solidlens.Vec{X: 2.3}, 0.88, 2.5, 10), Material: solidlens.Matte(solidlens.RGB(1, 0.31, 0.22))},
	)
}

func mechanicalScene() solidlens.Scene {
	gear := torus(solidlens.Vec{X: -1.55, Z: 0.85}, 0.95, 0.25, 24, 10)
	spokes := newBuilder()
	for i := 0; i < 8; i++ {
		angle := 2 * math.Pi * float64(i) / 8
		spokes.box(solidlens.Vec{X: -1.55 + 0.64*math.Cos(angle), Y: 0.64 * math.Sin(angle), Z: 0.85}, solidlens.Vec{X: 0.28, Y: 0.28, Z: 0.52})
	}
	return studioScene(
		solidlens.Model{Mesh: gear, Material: solidlens.Matte(solidlens.RGB(0.05, 0.68, 0.94))},
		solidlens.Model{Mesh: spokes.mesh(), Material: solidlens.Matte(solidlens.RGB(0.05, 0.68, 0.94))},
		solidlens.Model{Mesh: cylinder(solidlens.Vec{X: 1.55}, 0.92, 2.25, 12), Material: solidlens.Matte(solidlens.RGB(0.95, 0.64, 0.06))},
		solidlens.Model{Mesh: sphere(solidlens.Vec{X: 1.55, Z: 2.36}, 0.7, 24, 12), Material: solidlens.Matte(solidlens.RGB(0.95, 0.64, 0.06))},
	)
}

func formsScene() solidlens.Scene {
	return studioScene(
		solidlens.Model{Mesh: sphere(solidlens.Vec{X: -2, Z: 1.1}, 1.1, 32, 18), Material: solidlens.Matte(solidlens.RGB(0.96, 0.23, 0.54))},
		solidlens.Model{Mesh: boxMesh(solidlens.Vec{Z: 0.95}, solidlens.Vec{X: 1.57, Y: 1.57, Z: 1.67}), Material: solidlens.Matte(solidlens.RGB(0.17, 0.76, 0.54))},
		solidlens.Model{Mesh: cone(solidlens.Vec{X: 2.1}, 1.0, 2.4, 8), Material: solidlens.Matte(solidlens.RGB(0.33, 0.44, 0.98))},
	)
}

func studioScene(models ...solidlens.Model) solidlens.Scene {
	return solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{X: 6.5, Y: -9, Z: 5.2},
			Target:   solidlens.Vec{Z: 1},
			Up:       solidlens.Vec{Z: 1},
			FOV:      38,
		},
		Models: models,
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{X: -0.8, Y: 0.45, Z: -1},
			Color:     solidlens.RGB(1, 1, 1),
			Intensity: 1.1,
		}},
		PointLights: []solidlens.PointLight{{
			Position:  solidlens.Vec{X: -3, Y: -4, Z: 6},
			Color:     solidlens.RGB(0.5, 0.7, 1),
			Intensity: 13,
		}},
		Background: solidlens.RGB(0.004, 0.01, 0.035),
	}
}

type builder struct {
	vertices  []solidlens.Vec
	triangles [][3]int
}

func newBuilder() *builder { return &builder{} }

func (b *builder) mesh() *solidlens.Mesh {
	mesh, err := solidlens.NewMesh(b.vertices, b.triangles)
	if err != nil {
		panic(err)
	}
	return mesh
}

func (b *builder) add(vertices []solidlens.Vec, triangles [][3]int) {
	base := len(b.vertices)
	b.vertices = append(b.vertices, vertices...)
	for _, triangle := range triangles {
		b.triangles = append(b.triangles, [3]int{base + triangle[0], base + triangle[1], base + triangle[2]})
	}
}

func (b *builder) box(center, size solidlens.Vec) {
	half := size.Scale(0.5)
	vertices := []solidlens.Vec{
		center.Add(solidlensVec(-half.X, -half.Y, -half.Z)),
		center.Add(solidlensVec(half.X, -half.Y, -half.Z)),
		center.Add(solidlensVec(half.X, half.Y, -half.Z)),
		center.Add(solidlensVec(-half.X, half.Y, -half.Z)),
		center.Add(solidlensVec(-half.X, -half.Y, half.Z)),
		center.Add(solidlensVec(half.X, -half.Y, half.Z)),
		center.Add(solidlensVec(half.X, half.Y, half.Z)),
		center.Add(solidlensVec(-half.X, half.Y, half.Z)),
	}
	b.add(vertices, [][3]int{
		{0, 2, 1}, {0, 3, 2}, {4, 5, 6}, {4, 6, 7},
		{0, 1, 5}, {0, 5, 4}, {1, 2, 6}, {1, 6, 5},
		{2, 3, 7}, {2, 7, 6}, {3, 0, 4}, {3, 4, 7},
	})
}

func cylinder(center solidlens.Vec, radius, height float64, segments int) *solidlens.Mesh {
	b := newBuilder()
	vertices := make([]solidlens.Vec, 0, segments*2+2)
	for i := 0; i < segments; i++ {
		angle := 2 * math.Pi * float64(i) / float64(segments)
		x, y := radius*math.Cos(angle), radius*math.Sin(angle)
		vertices = append(vertices, center.Add(solidlensVec(x, y, 0)), center.Add(solidlensVec(x, y, height)))
	}
	vertices = append(vertices, center, center.Add(solidlensVec(0, 0, height)))
	bottom, top := segments*2, segments*2+1
	triangles := make([][3]int, 0, segments*4)
	for i := 0; i < segments; i++ {
		next := (i + 1) % segments
		triangles = append(triangles,
			[3]int{i * 2, next * 2, i*2 + 1}, [3]int{i*2 + 1, next * 2, next*2 + 1},
			[3]int{bottom, next * 2, i * 2}, [3]int{top, i*2 + 1, next*2 + 1},
		)
	}
	b.add(vertices, triangles)
	return b.mesh()
}

func cone(center solidlens.Vec, radius, height float64, segments int) *solidlens.Mesh {
	b := newBuilder()
	vertices := make([]solidlens.Vec, 0, segments+2)
	for i := 0; i < segments; i++ {
		angle := 2 * math.Pi * float64(i) / float64(segments)
		vertices = append(vertices, center.Add(solidlensVec(radius*math.Cos(angle), radius*math.Sin(angle), 0)))
	}
	vertices = append(vertices, center, center.Add(solidlensVec(0, 0, height)))
	bottom, peak := segments, segments+1
	triangles := make([][3]int, 0, segments*2)
	for i := 0; i < segments; i++ {
		next := (i + 1) % segments
		triangles = append(triangles, [3]int{bottom, next, i}, [3]int{i, next, peak})
	}
	b.add(vertices, triangles)
	return b.mesh()
}

func sphere(center solidlens.Vec, radius float64, slices, stacks int) *solidlens.Mesh {
	b := newBuilder()
	vertices := make([]solidlens.Vec, 0, (slices+1)*(stacks+1))
	for stack := 0; stack <= stacks; stack++ {
		phi := math.Pi * float64(stack) / float64(stacks)
		for slice := 0; slice <= slices; slice++ {
			theta := 2 * math.Pi * float64(slice) / float64(slices)
			vertices = append(vertices, center.Add(solidlensVec(
				radius*math.Sin(phi)*math.Cos(theta), radius*math.Sin(phi)*math.Sin(theta), radius*math.Cos(phi),
			)))
		}
	}
	triangles := make([][3]int, 0, slices*stacks*2)
	for stack := 0; stack < stacks; stack++ {
		for slice := 0; slice < slices; slice++ {
			a := stack*(slices+1) + slice
			b := a + slices + 1
			triangles = append(triangles, [3]int{a, b, a + 1}, [3]int{a + 1, b, b + 1})
		}
	}
	b.add(vertices, triangles)
	return b.mesh()
}

func torus(center solidlens.Vec, radius, tube float64, majorSegments, minorSegments int) *solidlens.Mesh {
	b := newBuilder()
	vertices := make([]solidlens.Vec, 0, majorSegments*minorSegments)
	for major := 0; major < majorSegments; major++ {
		a := 2 * math.Pi * float64(major) / float64(majorSegments)
		for minor := 0; minor < minorSegments; minor++ {
			c := 2 * math.Pi * float64(minor) / float64(minorSegments)
			vertices = append(vertices, center.Add(solidlensVec(
				(radius+tube*math.Cos(c))*math.Cos(a), (radius+tube*math.Cos(c))*math.Sin(a), tube*math.Sin(c),
			)))
		}
	}
	triangles := make([][3]int, 0, majorSegments*minorSegments*2)
	for major := 0; major < majorSegments; major++ {
		for minor := 0; minor < minorSegments; minor++ {
			nextMajor := (major + 1) % majorSegments
			nextMinor := (minor + 1) % minorSegments
			a := major*minorSegments + minor
			b := nextMajor*minorSegments + minor
			c := major*minorSegments + nextMinor
			d := nextMajor*minorSegments + nextMinor
			triangles = append(triangles, [3]int{a, b, c}, [3]int{c, b, d})
		}
	}
	b.add(vertices, triangles)
	return b.mesh()
}

func boxMesh(center, size solidlens.Vec) *solidlens.Mesh {
	b := newBuilder()
	b.box(center, size)
	return b.mesh()
}

func solidlensVec(x, y, z float64) solidlens.Vec { return solidlens.Vec{X: x, Y: y, Z: z} }
