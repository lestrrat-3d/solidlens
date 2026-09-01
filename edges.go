package solidlens

import (
	"fmt"
	"math"
)

// Edges controls the lines drawn along a model's edges. The zero value draws
// nothing, so a model without edges renders exactly as it did before.
//
// An edge is drawn when it is a border of the mesh, when it separates two
// faces that meet at CreaseAngle or more, or when it lies on the model's
// silhouette from the camera position.
type Edges struct {
	// Enabled draws the model's edges when true.
	Enabled bool
	// Color is the line color. The zero value means opaque black.
	Color Color
	// Width is the line width in pixels and defaults to 1.
	Width float64
	// CreaseAngle is the smallest angle in degrees between two neighbouring
	// face normals whose shared edge is still drawn. It defaults to 30. A
	// negative value draws every shared edge, which is a full wireframe.
	CreaseAngle float64
}

// Outline returns enabled edges of color c with the default width and crease
// angle.
func Outline(c Color) Edges { return Edges{Enabled: true, Color: c} }

const defaultCreaseAngle = 30

func (e Edges) normalized() (Edges, error) {
	if !finiteColor(e.Color) || !finite(e.Width) || !finite(e.CreaseAngle) {
		return Edges{}, fmt.Errorf("edges hold a non-finite value")
	}
	if e.Color == (Color{}) {
		e.Color = RGB(0, 0, 0)
	}
	if e.Width == 0 {
		e.Width = 1
	}
	if e.CreaseAngle == 0 {
		e.CreaseAngle = defaultCreaseAngle
	}
	if e.Width < 0 {
		return Edges{}, fmt.Errorf("edge width must not be negative")
	}
	if e.CreaseAngle > 180 {
		return Edges{}, fmt.Errorf("edge crease angle must not exceed 180 degrees")
	}
	return e, nil
}

// segment is a single edge line in world space.
type segment struct {
	a, b  Vec
	style Edges
}

// weldScale quantizes vertex positions to ten nanometres so that meshes which
// repeat a position per triangle, as STL files do, still share edges.
const weldScale = 1e5

type weldedVec struct {
	x, y, z float64
}

func weld(v Vec) weldedVec {
	return weldedVec{
		x: math.Round(v.X * weldScale),
		y: math.Round(v.Y * weldScale),
		z: math.Round(v.Z * weldScale),
	}
}

type edgeKey struct {
	low, high weldedVec
}

func newEdgeKey(a, b Vec) edgeKey {
	low, high := weld(a), weld(b)
	if high.x < low.x ||
		(high.x == low.x && high.y < low.y) ||
		(high.x == low.x && high.y == low.y && high.z < low.z) {
		low, high = high, low
	}
	return edgeKey{low: low, high: high}
}

type edgeRecord struct {
	a, b    Vec
	normals [2]Vec
	// point is a position on one of the faces, used for the silhouette test.
	point Vec
	faces int
}

// collectEdges returns the world-space lines to draw for one model. Triangles
// whose normal is undefined are ignored, and so is any edge they contribute.
func collectEdges(vertices []Vec, triangles [][3]int, eye Vec, style Edges) []segment {
	records := make(map[edgeKey]*edgeRecord)
	for _, triangle := range triangles {
		corners := [3]Vec{vertices[triangle[0]], vertices[triangle[1]], vertices[triangle[2]]}
		normal, ok := corners[1].Sub(corners[0]).Cross(corners[2].Sub(corners[0])).Normalize()
		if !ok {
			continue
		}
		for i, a := range corners {
			b := corners[(i+1)%3]
			key := newEdgeKey(a, b)
			record, exists := records[key]
			if !exists {
				record = &edgeRecord{a: a, b: b, point: corners[0]}
				records[key] = record
			}
			if record.faces < len(record.normals) {
				record.normals[record.faces] = normal
			}
			record.faces++
		}
	}
	creaseCos := math.Cos(style.CreaseAngle * math.Pi / 180)
	segments := make([]segment, 0, len(records))
	for _, record := range records {
		if !drawEdge(record, eye, style.CreaseAngle, creaseCos) {
			continue
		}
		segments = append(segments, segment{a: record.a, b: record.b, style: style})
	}
	return segments
}

func drawEdge(record *edgeRecord, eye Vec, creaseAngle, creaseCos float64) bool {
	// A border edge has one face and a non-manifold edge has more than two.
	// Both mark the shape of the body, so both are drawn.
	if record.faces != 2 {
		return true
	}
	if creaseAngle < 0 {
		return true
	}
	if record.normals[0].Dot(record.normals[1]) <= creaseCos {
		return true
	}
	// A silhouette edge has one face turned toward the camera and one away.
	toEye := eye.Sub(record.point)
	front := record.normals[0].Dot(toEye)
	back := record.normals[1].Dot(toEye)
	return (front > 0) != (back > 0)
}
