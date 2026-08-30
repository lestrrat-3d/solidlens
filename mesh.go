package solidlens

import (
	"fmt"
	"math"

	"github.com/lestrrat-3d/r3"
)

// Vec is a three-dimensional position or direction in millimetres.
type Vec = r3.Vec

// TriangleSource supplies indexed triangle geometry. *decad.Mesh satisfies
// this interface directly.
type TriangleSource interface {
	Vertices() []r3.Vec
	Triangles() [][3]int
}

// Mesh is an immutable indexed triangle mesh.
type Mesh struct {
	vertices  []Vec
	triangles [][3]int
}

// NewMesh copies and validates indexed triangle geometry.
func NewMesh(vertices []Vec, triangles [][3]int) (*Mesh, error) {
	if len(vertices) == 0 {
		return nil, fmt.Errorf("solidlens: mesh has no vertices")
	}
	if len(triangles) == 0 {
		return nil, fmt.Errorf("solidlens: mesh has no triangles")
	}
	for i, vertex := range vertices {
		if !finiteVec(vertex) {
			return nil, fmt.Errorf("solidlens: vertex %d is not finite", i)
		}
	}
	for i, triangle := range triangles {
		for _, index := range triangle {
			if index < 0 || index >= len(vertices) {
				return nil, fmt.Errorf("solidlens: triangle %d references vertex %d", i, index)
			}
		}
		if triangle[0] == triangle[1] || triangle[1] == triangle[2] || triangle[2] == triangle[0] {
			return nil, fmt.Errorf("solidlens: triangle %d repeats a vertex", i)
		}
	}
	return &Mesh{
		vertices:  append([]Vec(nil), vertices...),
		triangles: append([][3]int(nil), triangles...),
	}, nil
}

// MeshFrom copies a mesh from any TriangleSource, including *decad.Mesh.
func MeshFrom(source TriangleSource) (*Mesh, error) {
	if source == nil {
		return nil, fmt.Errorf("solidlens: mesh source is nil")
	}
	return NewMesh(source.Vertices(), source.Triangles())
}

// Vertices returns a copy of the mesh vertex positions.
func (m *Mesh) Vertices() []Vec {
	if m == nil {
		return nil
	}
	return append([]Vec(nil), m.vertices...)
}

// Triangles returns a copy of the mesh triangle indices.
func (m *Mesh) Triangles() [][3]int {
	if m == nil {
		return nil
	}
	return append([][3]int(nil), m.triangles...)
}

func finiteVec(v Vec) bool {
	return !math.IsNaN(v.X) && !math.IsInf(v.X, 0) &&
		!math.IsNaN(v.Y) && !math.IsInf(v.Y, 0) &&
		!math.IsNaN(v.Z) && !math.IsInf(v.Z, 0)
}
