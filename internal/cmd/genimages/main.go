// Command genimages renders the README gallery with solidlens itself.
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/lestrrat-3d/solidlens"
	tmf "github.com/lestrrat-go/3mf"
	"github.com/lestrrat-go/stl"
)

const outputDir = "docs/images"
const modelDir = "docs/models"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--models" {
		if err := writeModelAssets(); err != nil {
			fmt.Fprintf(os.Stderr, "build model assets: %s\n", err)
			os.Exit(1)
		}
		return
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create image directory: %s\n", err)
		os.Exit(1)
	}
	for _, render := range []galleryRender{
		{name: "hero.png", scene: heroScene},
		{name: "mechanical.png", scene: mechanicalScene},
		{name: "forms.png", scene: formsScene},
	} {
		if err := render.write(); err != nil {
			fmt.Fprintf(os.Stderr, "render %s: %s\n", render.name, err)
			os.Exit(1)
		}
	}
}

type galleryRender struct {
	name  string
	scene func() (solidlens.Scene, error)
}

func (r galleryRender) write() error {
	scene, err := r.scene()
	if err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outputDir, r.name)) //nolint:gosec
	if err != nil {
		return err
	}
	err = solidlens.RenderPNG(context.Background(), file, scene, solidlens.Settings{Width: 1440, Height: 810})
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func heroScene() (solidlens.Scene, error) {
	text, err := readModel("solidlens.stl")
	if err != nil {
		return solidlens.Scene{}, err
	}
	ball, err := readModel("hero-ball.stl")
	if err != nil {
		return solidlens.Scene{}, err
	}
	pyramid, err := readModel("hero-pyramid.stl")
	if err != nil {
		return solidlens.Scene{}, err
	}
	cube, err := readModel("hero-cube.stl")
	if err != nil {
		return solidlens.Scene{}, err
	}
	scene := studioScene(
		solidlens.Model{Mesh: text, Material: solidlens.Matte(solidlens.RGB(0.78, 0.9, 1))},
		outlined(solidlens.Model{Mesh: ball, Material: solidlens.Matte(solidlens.RGB(0.04, 0.78, 0.88))}),
		outlined(solidlens.Model{Mesh: pyramid, Material: solidlens.Matte(solidlens.RGB(0.47, 0.21, 0.93))}),
		outlined(solidlens.Model{Mesh: cube, Material: solidlens.Matte(solidlens.RGB(1, 0.31, 0.22))}),
	)
	scene.Camera.Position = solidlens.Vec{X: -0.24, Y: -5.5, Z: 2.5}
	scene.Camera.Target = solidlens.Vec{Z: 2.25}
	scene.Camera.FOV = 52
	return scene, nil
}

func mechanicalScene() (solidlens.Scene, error) {
	blue, err := readModel("mechanical-blue.stl")
	if err != nil {
		return solidlens.Scene{}, err
	}
	gold, err := readModel("mechanical-gold.3mf")
	if err != nil {
		return solidlens.Scene{}, err
	}
	return studioScene(
		outlined(solidlens.Model{Mesh: blue, Material: solidlens.Matte(solidlens.RGB(0.05, 0.68, 0.94))}),
		outlined(solidlens.Model{Mesh: gold, Material: solidlens.Matte(solidlens.RGB(0.95, 0.64, 0.06))}),
	), nil
}

func formsScene() (solidlens.Scene, error) {
	pink, err := readModel("forms-pink.stl")
	if err != nil {
		return solidlens.Scene{}, err
	}
	green, err := readModel("forms-green.stl")
	if err != nil {
		return solidlens.Scene{}, err
	}
	blue, err := readModel("forms-blue.stl")
	if err != nil {
		return solidlens.Scene{}, err
	}
	return studioScene(
		outlined(solidlens.Model{Mesh: pink, Material: solidlens.Matte(solidlens.RGB(0.96, 0.23, 0.54))}),
		outlined(solidlens.Model{Mesh: green, Material: solidlens.Matte(solidlens.RGB(0.17, 0.76, 0.54))}),
		outlined(solidlens.Model{Mesh: blue, Material: solidlens.Matte(solidlens.RGB(0.33, 0.44, 0.98))}),
	), nil
}

// outlined enables edge lines on a model. The line color is a lightened form
// of the material so that it reads against both the lit surface and the dark
// background.
func outlined(model solidlens.Model) solidlens.Model {
	model.Edges = solidlens.Edges{
		Enabled: true,
		Color:   lighten(model.Material.Color, 0.6),
		Width:   1.5,
	}
	return model
}

func lighten(c solidlens.Color, amount float64) solidlens.Color {
	return solidlens.Color{
		R: c.R + (1-c.R)*amount,
		G: c.G + (1-c.G)*amount,
		B: c.B + (1-c.B)*amount,
		A: 1,
	}
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
	for i := range segments {
		angle := 2 * math.Pi * float64(i) / float64(segments)
		x, y := radius*math.Cos(angle), radius*math.Sin(angle)
		vertices = append(vertices, center.Add(solidlensVec(x, y, 0)), center.Add(solidlensVec(x, y, height)))
	}
	vertices = append(vertices, center, center.Add(solidlensVec(0, 0, height)))
	bottom, top := segments*2, segments*2+1
	triangles := make([][3]int, 0, segments*4)
	for i := range segments {
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
	for i := range segments {
		angle := 2 * math.Pi * float64(i) / float64(segments)
		vertices = append(vertices, center.Add(solidlensVec(radius*math.Cos(angle), radius*math.Sin(angle), 0)))
	}
	vertices = append(vertices, center, center.Add(solidlensVec(0, 0, height)))
	bottom, peak := segments, segments+1
	triangles := make([][3]int, 0, segments*2)
	for i := range segments {
		next := (i + 1) % segments
		triangles = append(triangles, [3]int{bottom, next, i}, [3]int{i, next, peak})
	}
	b.add(vertices, triangles)
	return b.mesh()
}

func pyramid(center solidlens.Vec, width, height float64) *solidlens.Mesh {
	half := width / 2
	b := newBuilder()
	b.add([]solidlens.Vec{
		center.Add(solidlensVec(-half, -half, 0)),
		center.Add(solidlensVec(half, -half, 0)),
		center.Add(solidlensVec(half, half, 0)),
		center.Add(solidlensVec(-half, half, 0)),
		center.Add(solidlensVec(0, 0, height)),
	}, [][3]int{
		{0, 2, 1}, {0, 3, 2}, {0, 1, 4}, {1, 2, 4}, {2, 3, 4}, {3, 0, 4},
	})
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
	for stack := range stacks {
		for slice := range slices {
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
	for major := range majorSegments {
		a := 2 * math.Pi * float64(major) / float64(majorSegments)
		for minor := range minorSegments {
			c := 2 * math.Pi * float64(minor) / float64(minorSegments)
			vertices = append(vertices, center.Add(solidlensVec(
				(radius+tube*math.Cos(c))*math.Cos(a), (radius+tube*math.Cos(c))*math.Sin(a), tube*math.Sin(c),
			)))
		}
	}
	triangles := make([][3]int, 0, majorSegments*minorSegments*2)
	for major := range majorSegments {
		for minor := range minorSegments {
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

func readModel(name string) (*solidlens.Mesh, error) {
	file, err := os.Open(filepath.Join(modelDir, name)) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("open model %q: %w", name, err)
	}
	mesh, err := solidlens.ReadMesh(name, file)
	closeErr := file.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return mesh, nil
}

func writeModelAssets() error {
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		return err
	}
	if err := writeSTL("solidlens.stl", wordMesh("Solidlens")); err != nil {
		return err
	}
	if err := writeSTL("hero-ball.stl", sphere(solidlens.Vec{X: -1.8, Z: 1.55}, 0.8, 32, 18)); err != nil {
		return err
	}
	if err := writeSTL("hero-pyramid.stl", pyramid(solidlens.Vec{Z: 0.75}, 1.6, 1.6)); err != nil {
		return err
	}
	if err := writeSTL("hero-cube.stl", boxMesh(solidlens.Vec{X: 1.8, Z: 1.55}, solidlens.Vec{X: 1.6, Y: 1.6, Z: 1.6})); err != nil {
		return err
	}

	spokes := newBuilder()
	for i := range 8 {
		angle := 2 * math.Pi * float64(i) / 8
		spokes.box(
			solidlens.Vec{X: -1.55 + 0.64*math.Cos(angle), Y: 0.64 * math.Sin(angle), Z: 0.85},
			solidlens.Vec{X: 0.28, Y: 0.28, Z: 0.52},
		)
	}
	if err := writeSTL("mechanical-blue.stl", mergeMeshes(
		torus(solidlens.Vec{X: -1.55, Z: 0.85}, 0.95, 0.25, 24, 10),
		spokes.mesh(),
	)); err != nil {
		return err
	}
	if err := write3MF("mechanical-gold.3mf", mergeMeshes(
		cylinder(solidlens.Vec{X: 1.55}, 0.92, 2.25, 12),
		sphere(solidlens.Vec{X: 1.55, Z: 2.36}, 0.7, 24, 12),
	)); err != nil {
		return err
	}
	if err := writeSTL("forms-pink.stl", sphere(solidlens.Vec{X: -2, Z: 1.1}, 1.1, 32, 18)); err != nil {
		return err
	}
	if err := writeSTL("forms-green.stl", boxMesh(solidlens.Vec{Z: 0.95}, solidlens.Vec{X: 1.57, Y: 1.57, Z: 1.67})); err != nil {
		return err
	}
	return writeSTL("forms-blue.stl", cone(solidlens.Vec{X: 2.1}, 1.0, 2.4, 8))
}

func writeSTL(name string, mesh *solidlens.Mesh) error {
	file, err := os.Create(filepath.Join(modelDir, name)) //nolint:gosec
	if err != nil {
		return err
	}
	vertices := mesh.Vertices()
	triangles := make([]stl.Triangle, 0, len(mesh.Triangles()))
	for _, triangle := range mesh.Triangles() {
		facet := stl.Triangle{}
		for index, vertexIndex := range triangle {
			vertex := vertices[vertexIndex]
			facet.Vertices[index] = stl.Vec3{float32(vertex.X), float32(vertex.Y), float32(vertex.Z)}
		}
		triangles = append(triangles, facet)
	}
	err = stl.Encode(file, &stl.Solid{Name: "solidlens", Triangles: triangles}, stl.FormatASCII)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func write3MF(name string, mesh *solidlens.Mesh) error {
	vertices := mesh.Vertices()
	modelVertices := make([]tmf.Vertex, len(vertices))
	for i, vertex := range vertices {
		modelVertices[i] = tmf.Vertex{X: vertex.X, Y: vertex.Y, Z: vertex.Z}
	}
	triangles := mesh.Triangles()
	modelTriangles := make([]tmf.Triangle, len(triangles))
	for i, triangle := range triangles {
		modelTriangles[i] = tmf.Triangle{V1: uint32(triangle[0]), V2: uint32(triangle[1]), V3: uint32(triangle[2])}
	}
	geometry := tmf.NewMesh(tmf.WithVertices(modelVertices), tmf.WithTriangles(modelTriangles))
	object := tmf.NewObject(tmf.WithObjectID(1), tmf.WithObjectName("solidlens gallery model"), tmf.WithMesh(geometry))
	model := tmf.NewModel(tmf.WithUnit(tmf.UnitMillimeter), tmf.WithObject(object), tmf.WithBuildItem(tmf.NewBuildItem(tmf.WithObjectRef(object))))
	pkg := tmf.NewPackage(tmf.WithModel(model))
	file, err := os.Create(filepath.Join(modelDir, name)) //nolint:gosec
	if err != nil {
		return err
	}
	_, err = pkg.WriteTo(file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func mergeMeshes(meshes ...*solidlens.Mesh) *solidlens.Mesh {
	b := newBuilder()
	for _, mesh := range meshes {
		b.add(mesh.Vertices(), mesh.Triangles())
	}
	return b.mesh()
}

func wordMesh(text string) *solidlens.Mesh {
	patterns := map[rune][]string{
		'S': {"01110", "10000", "10000", "01110", "00001", "00001", "11110"},
		'o': {"00000", "00000", "01110", "10001", "10001", "10001", "01110"},
		'l': {"01000", "01000", "01000", "01000", "01000", "01000", "00110"},
		'i': {"00000", "00100", "00000", "01100", "00100", "00100", "01110"},
		'd': {"00001", "00001", "01101", "10011", "10001", "10011", "01101"},
		'e': {"00000", "00000", "01110", "10001", "11111", "10000", "01110"},
		'n': {"00000", "00000", "10110", "11001", "10001", "10001", "10001"},
		's': {"00000", "00000", "01111", "10000", "01110", "00001", "11110"},
	}
	const cell = 0.16
	const gap = 0.08
	width := 0.0
	for _, letter := range text {
		width += float64(len(patterns[letter][0]))*cell + gap
	}
	width -= gap
	b := newBuilder()
	x := -width / 2
	for _, letter := range text {
		glyph := patterns[letter]
		for row, pixels := range glyph {
			for column, pixel := range pixels {
				if pixel != '1' {
					continue
				}
				b.box(
					solidlens.Vec{X: x + (float64(column)+0.5)*cell, Y: 0.1, Z: 4.2 - (float64(row)+0.5)*cell},
					solidlens.Vec{X: cell * 1.03, Y: 0.26, Z: cell * 1.03},
				)
			}
		}
		x += float64(len(glyph[0]))*cell + gap
	}
	return b.mesh()
}
