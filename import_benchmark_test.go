package solidlens_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	tmf "github.com/lestrrat-go/3mf"
)

func BenchmarkReadMesh(b *testing.B) {
	vertices, triangles := benchmarkGrid(32)
	cases := []struct {
		name string
		data []byte
	}{
		{"grid.obj", benchmarkOBJ(vertices, triangles)},
		{"grid.stl", benchmarkSTL(vertices, triangles)},
		{"grid.3mf", benchmark3MF(b, vertices, triangles)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			// Parsing includes the format decoder and the final mesh validation.
			b.SetBytes(int64(len(tc.data)))
			b.ReportAllocs()
			b.ResetTimer()
			var mesh *solidlens.Mesh
			for range b.N {
				var err error
				mesh, err = solidlens.ReadMesh(tc.name, bytes.NewReader(tc.data))
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

func benchmarkOBJ(vertices []solidlens.Vec, triangles [][3]int) []byte {
	var output bytes.Buffer
	for _, vertex := range vertices {
		fmt.Fprintf(&output, "v %g %g %g\n", vertex.X, vertex.Y, vertex.Z)
	}
	for _, triangle := range triangles {
		fmt.Fprintf(&output, "f %d %d %d\n", triangle[0]+1, triangle[1]+1, triangle[2]+1)
	}
	return output.Bytes()
}

func benchmarkSTL(vertices []solidlens.Vec, triangles [][3]int) []byte {
	// Binary STL stores a normal, three vertices, and an attribute count per triangle.
	data := make([]byte, 84+50*len(triangles))
	binary.LittleEndian.PutUint32(data[80:84], uint32(len(triangles)))
	for i, triangle := range triangles {
		record := data[84+i*50 : 84+(i+1)*50]
		for corner, index := range triangle {
			vertex := vertices[index]
			for axis, value := range [3]float64{vertex.X, vertex.Y, vertex.Z} {
				offset := 12 + corner*12 + axis*4
				binary.LittleEndian.PutUint32(record[offset:offset+4], math.Float32bits(float32(value)))
			}
		}
	}
	return data
}

func benchmark3MF(b *testing.B, vertices []solidlens.Vec, triangles [][3]int) []byte {
	b.Helper()
	tmfVertices := make([]tmf.Vertex, len(vertices))
	for i, vertex := range vertices {
		tmfVertices[i] = tmf.Vertex{X: vertex.X, Y: vertex.Y, Z: vertex.Z}
	}
	tmfTriangles := make([]tmf.Triangle, len(triangles))
	for i, triangle := range triangles {
		tmfTriangles[i] = tmf.Triangle{
			V1: uint32(triangle[0]), V2: uint32(triangle[1]), V3: uint32(triangle[2]),
		}
	}
	mesh := tmf.NewMesh(tmf.WithVertices(tmfVertices), tmf.WithTriangles(tmfTriangles))
	object := tmf.NewObject(tmf.WithObjectID(1), tmf.WithMesh(mesh))
	model := tmf.NewModel(tmf.WithObject(object), tmf.WithBuildItem(tmf.NewBuildItem(tmf.WithObjectRef(object))))
	pkg := tmf.NewPackage(tmf.WithModel(model))
	var output bytes.Buffer
	if _, err := pkg.WriteTo(&output); err != nil {
		b.Fatal(err)
	}
	return output.Bytes()
}
