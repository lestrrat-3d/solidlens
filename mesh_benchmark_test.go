package solidlens_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/lestrrat-3d/solidlens"
)

// benchmarkGrid returns a curved, indexed surface with two triangles per cell.
// The same geometry feeds mesh creation, imports, and rendering benchmarks.
func benchmarkGrid(divisions int) ([]solidlens.Vec, [][3]int) {
	stride := divisions + 1
	vertices := make([]solidlens.Vec, stride*stride)
	for y := range stride {
		for x := range stride {
			px := (float64(x)/float64(divisions) - 0.5) * 100
			py := (float64(y)/float64(divisions) - 0.5) * 100
			vertices[y*stride+x] = solidlens.Vec{
				X: px, Y: py, Z: 3 * math.Sin(px*0.1) * math.Cos(py*0.1),
			}
		}
	}
	triangles := make([][3]int, 0, 2*divisions*divisions)
	for y := range divisions {
		for x := range divisions {
			a := y*stride + x
			b, c := a+1, a+stride
			triangles = append(triangles, [3]int{a, b, c}, [3]int{b, c + 1, c})
		}
	}
	return vertices, triangles
}

func BenchmarkNewMesh(b *testing.B) {
	for _, divisions := range []int{64, 128} {
		b.Run(strconv.Itoa(2*divisions*divisions)+"_triangles", func(b *testing.B) {
			vertices, triangles := benchmarkGrid(divisions)
			b.ReportAllocs()
			b.ResetTimer()
			var mesh *solidlens.Mesh
			for range b.N {
				var err error
				mesh, err = solidlens.NewMesh(vertices, triangles)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			if got := len(mesh.Triangles()); got != len(triangles) {
				b.Fatalf("mesh has %d triangles, want %d", got, len(triangles))
			}
		})
	}
}
