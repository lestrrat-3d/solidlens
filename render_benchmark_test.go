package solidlens_test

import (
	"bytes"
	"image"
	"testing"

	"github.com/lestrrat-3d/solidlens"
)

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
	}{
		{"surface_8192_triangles_512px", 64, 512, solidlens.Edges{}},
		{"outline_8192_triangles_512px", 64, 512, outline},
		{"wireframe_8192_triangles_512px", 64, 512, wireframe},
		{"surface_32768_triangles_512px", 128, 512, solidlens.Edges{}},
		{"wireframe_32768_triangles_512px", 128, 512, wireframe},
		{"surface_8192_triangles_1024px", 64, 1024, solidlens.Edges{}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			scene := benchmarkScene(b, tc.divisions, tc.edges)
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
