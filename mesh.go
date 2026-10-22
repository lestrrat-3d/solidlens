package solidlens

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"

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
	cache     *meshCache
}

type meshCache struct {
	edgesOnce sync.Once
	edges     []edgeRecord
	// tolerant holds the edge records of the last tolerance they were asked for.
	tolerant atomic.Pointer[tolerantEdges]
	// Each pointer holds one immutable result and is replaced when its key changes.
	render   atomic.Pointer[meshRenderCache]
	coverage atomic.Pointer[edgeCoverageCache]
}

type tolerantEdges struct {
	tolerance float64
	smoothCos float64
	records   []edgeRecord
}

// cachedEdgeRecords returns the mesh's edge records for an Edges.Tolerance
// already resolved to a length, which is zero for none, and the cosine of the
// angle below which two faces count as smooth.
func (m *Mesh) cachedEdgeRecords(tolerance, smoothCos float64) []edgeRecord {
	if m.cache == nil {
		return m.buildEdgeRecords(tolerance, smoothCos)
	}
	if tolerance > 0 {
		cached := m.cache.tolerant.Load()
		if cached != nil && cached.tolerance == tolerance && cached.smoothCos == smoothCos {
			return cached.records
		}
		records := m.buildEdgeRecords(tolerance, smoothCos)
		m.cache.tolerant.Store(&tolerantEdges{tolerance: tolerance, smoothCos: smoothCos, records: records})
		return records
	}
	m.cache.edgesOnce.Do(func() {
		m.cache.edges = m.buildEdgeRecords(0, smoothCos)
	})
	return m.cache.edges
}

func (m *Mesh) buildEdgeRecords(tolerance, smoothCos float64) []edgeRecord {
	normals := make([]Vec, len(m.triangles))
	for index, triangle := range m.triangles {
		corners := [3]Vec{
			m.vertices[triangle[0]], m.vertices[triangle[1]], m.vertices[triangle[2]],
		}
		normal, ok := corners[1].Sub(corners[0]).Cross(corners[2].Sub(corners[0])).Normalize()
		if ok {
			normals[index] = normal
		}
	}
	return buildEdgeRecords(m.vertices, m.triangles, normals, tolerance, smoothCos)
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
		cache:     &meshCache{},
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
