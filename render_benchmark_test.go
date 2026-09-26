package solidlens_test

import (
	"bytes"
	"image"
	"testing"

	"github.com/lestrrat-3d/solidlens"
)

type benchmarkTriangleSource struct {
	vertices  []solidlens.Vec
	triangles [][3]int
}

func (s benchmarkTriangleSource) Vertices() []solidlens.Vec { return s.vertices }
func (s benchmarkTriangleSource) Triangles() [][3]int       { return s.triangles }

func benchmarkScene(b *testing.B, divisions int, edges solidlens.Edges) solidlens.Scene {
	b.Helper()
	vertices, triangles := benchmarkGrid(divisions)
	mesh, err := solidlens.NewMesh(vertices, triangles)
	if err != nil {
		b.Fatal(err)
	}
	model := solidlens.Model{
		Mesh:     mesh,
		Material: solidlens.Matte(solidlens.RGB(0.8, 0.5, 0.3)),
		Edges:    edges,
	}
	return solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{Z: 160},
			Target:   solidlens.Vec{},
			Up:       solidlens.Vec{Y: 1},
		},
		Models: []solidlens.Model{model},
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{X: 0.3, Y: 0.4, Z: -1},
			Color:     solidlens.RGB(1, 1, 1),
			Intensity: 1,
		}},
		Background: solidlens.RGB(0, 0, 0),
	}
}

func BenchmarkRender(b *testing.B) {
	outline := solidlens.Outline(solidlens.RGB(0, 0, 0))
	wireframe := outline
	wireframe.CreaseAngle = -1
	cases := []struct {
		name      string
		divisions int
		pixels    int
		edges     solidlens.Edges
		points    bool
		source    bool
	}{
		{"surface_8192_triangles_512px", 64, 512, solidlens.Edges{}, false, false},
		{"outline_8192_triangles_512px", 64, 512, outline, false, false},
		{"wireframe_8192_triangles_512px", 64, 512, wireframe, false, false},
		{"surface_32768_triangles_512px", 128, 512, solidlens.Edges{}, false, false},
		{"wireframe_32768_triangles_512px", 128, 512, wireframe, false, false},
		{"surface_8192_triangles_1024px", 64, 1024, solidlens.Edges{}, false, false},
		{"point_lights_8192_triangles_512px", 64, 512, solidlens.Edges{}, true, false},
		{"triangle_source_8192_triangles_512px", 64, 512, solidlens.Edges{}, false, true},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			scene := benchmarkScene(b, tc.divisions, tc.edges)
			if tc.source {
				mesh := scene.Models[0].Mesh
				scene.Models[0].Mesh = benchmarkTriangleSource{mesh.Vertices(), mesh.Triangles()}
			}
			if tc.points {
				scene.DirectionalLights = nil
				for _, position := range []solidlens.Vec{
					{X: -40, Y: -40, Z: 60}, {X: 40, Y: -40, Z: 60},
					{X: -40, Y: 40, Z: 60}, {X: 40, Y: 40, Z: 60},
					{X: 0, Y: -50, Z: 70}, {X: 50, Y: 0, Z: 70},
					{X: 0, Y: 50, Z: 70}, {X: -50, Y: 0, Z: 70},
				} {
					scene.PointLights = append(scene.PointLights, solidlens.PointLight{
						Position:  position,
						Color:     solidlens.RGB(1, 1, 1),
						Intensity: 100,
					})
				}
			}
			settings := solidlens.Settings{Width: tc.pixels, Height: tc.pixels}
			b.ReportAllocs()
			b.ResetTimer()
			var output *image.RGBA
			for range b.N {
				var err error
				output, err = solidlens.Render(b.Context(), scene, settings)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			for i := 0; i < len(output.Pix); i += 4 {
				if output.Pix[i] != 0 || output.Pix[i+1] != 0 || output.Pix[i+2] != 0 {
					return
				}
			}
			b.Fatal("rendered image contains no model pixels")
		})
	}
}

func BenchmarkRenderPNG(b *testing.B) {
	scene := benchmarkScene(b, 64, solidlens.Edges{})
	settings := solidlens.Settings{Width: 512, Height: 512}
	var output bytes.Buffer
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		output.Reset()
		if err := solidlens.RenderPNG(b.Context(), &output, scene, settings); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if output.Len() == 0 {
		b.Fatal("rendered PNG is empty")
	}
}
