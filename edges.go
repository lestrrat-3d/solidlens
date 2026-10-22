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
	// Tolerance is the length in millimetres below which a feature of the
	// mesh is not worth a line. A facet narrower than it takes the normal of
	// the facet it borders, so a sliver between two smooth faces does not
	// turn their shared edge into a crease. Facets that touch one another
	// take a normal only when together they are narrower than Tolerance, so a
	// face built from many long thin facets keeps its own creases. The zero
	// value picks half the size of one pixel at the model's distance from the
	// camera, so what cannot be seen does not show. A negative value, such as
	// Exact, draws the mesh as it is.
	Tolerance float64
}

// Exact is an Edges.Tolerance that draws every facet and edge of the mesh as
// it is, however small.
const Exact = -1.0

// Outline returns enabled edges of color c with the default width and crease
// angle.
func Outline(c Color) Edges { return Edges{Enabled: true, Color: c} }

const defaultCreaseAngle = 30

func (e Edges) normalized() (Edges, error) {
	if !finiteColor(e.Color) || !finite(e.Width) || !finite(e.CreaseAngle) || !finite(e.Tolerance) {
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
	low, high int
}

func newEdgeKey(a, b int) edgeKey {
	low, high := a, b
	if high < low {
		low, high = high, low
	}
	return edgeKey{low: low, high: high}
}

type edgeRecord struct {
	a, b           Vec
	aIndex, bIndex int
	normals        [2]Vec
	// tris holds the first two triangles that share the edge.
	tris [2]int
	// point is a position on one of the faces, used for the silhouette test.
	point Vec
	faces int
}

// edgeTolerance resolves an Edges.Tolerance to a length. Zero in the result
// means no tolerance. An automatic tolerance is half a pixel at the depth of
// the middle of the model's bounding box.
func edgeTolerance(setting float64, v view, vertices []Vec) float64 {
	if setting < 0 {
		return 0
	}
	if setting > 0 {
		return setting
	}
	if len(vertices) == 0 {
		return 0
	}
	low, high := vertices[0], vertices[0]
	for _, vertex := range vertices[1:] {
		low = Vec{X: math.Min(low.X, vertex.X), Y: math.Min(low.Y, vertex.Y), Z: math.Min(low.Z, vertex.Z)}
		high = Vec{X: math.Max(high.X, vertex.X), Y: math.Max(high.Y, vertex.Y), Z: math.Max(high.Z, vertex.Z)}
	}
	depth := math.Max(v.depthOf(lerpVec(low, high, 0.5)), v.near)
	return depth / (v.focal * float64(v.height))
}

// sliverNormals returns the normal each triangle takes for the crease test.
// A triangle whose smallest height, its area over its longest side, is below
// tolerance is a sliver. It adopts the normal of the tallest solid triangle it
// shares an edge with, so a gap between two faces does not add a crease of its
// own. A solid triangle is a tall one or a sliver that has already adopted.
//
// A chain of slivers adopts from the end that reaches a tall triangle, one
// step a pass, but only while the widths of the slivers in the chain add up to
// less than tolerance. Slivers that share edges form a group, and a group
// adopts as a whole or not at all. A flat face drawn as a fan of long thin
// triangles is wider than tolerance, so the reach stops short of its far side
// and the whole fan keeps its own normals, which leaves its creases where they
// are.
func sliverNormals(vertices []Vec, triangles [][3]int, normals []Vec, edgesOf [][3]int, ordered []edgeRecord,
	tolerance float64) []Vec {
	heights := make([]float64, len(triangles))
	// reach is the combined width of the slivers between a solid triangle and
	// the tall triangle its normal came from. It is zero for a tall triangle.
	reach := make([]float64, len(triangles))
	solid := make([]bool, len(triangles))
	pending := make([]int, 0)
	for index, triangle := range triangles {
		if normals[index] == (Vec{}) {
			continue
		}
		a, b, c := vertices[triangle[0]], vertices[triangle[1]], vertices[triangle[2]]
		longest := math.Max(b.Sub(a).Len(), math.Max(c.Sub(b).Len(), a.Sub(c).Len()))
		heights[index] = b.Sub(a).Cross(c.Sub(a)).Len() / longest
		if heights[index] >= tolerance {
			solid[index] = true
			continue
		}
		pending = append(pending, index)
	}
	groupOf, groupCount := sliverGroups(pending, heights, tolerance, edgesOf, ordered)
	adopted := append([]Vec(nil), normals...)
	for len(pending) > 0 {
		type adoption struct {
			triangle, from int
		}
		var chosen []adoption
		var rest []int
		for _, index := range pending {
			best, bestReach := -1, 0.0
			for _, recordIndex := range edgesOf[index] {
				record := &ordered[recordIndex]
				if record.faces != 2 {
					continue
				}
				other := record.tris[0]
				if other == index {
					other = record.tris[1]
				}
				if !solid[other] {
					continue
				}
				total := reach[other] + heights[index]
				if total >= tolerance {
					continue
				}
				if best < 0 || total < bestReach || (total == bestReach && heights[other] > heights[best]) {
					best, bestReach = other, total
				}
			}
			if best < 0 {
				rest = append(rest, index)
				continue
			}
			chosen = append(chosen, adoption{triangle: index, from: best})
		}
		if len(chosen) == 0 {
			break
		}
		for _, a := range chosen {
			adopted[a.triangle] = adopted[a.from]
			solid[a.triangle] = true
			reach[a.triangle] = reach[a.from] + heights[a.triangle]
		}
		pending = rest
	}
	// A group the reach did not cover is too wide to be a gap, so none of its
	// slivers keeps an adopted normal.
	if len(pending) == 0 {
		return adopted
	}
	uncovered := make([]bool, groupCount)
	for _, index := range pending {
		uncovered[groupOf[index]] = true
	}
	for index, group := range groupOf {
		if group >= 0 && uncovered[group] {
			adopted[index] = normals[index]
		}
	}
	return adopted
}

// sliverGroups numbers the groups of slivers that are joined to one another by
// shared edges. It returns the group of every triangle, -1 for a triangle that
// is not a sliver, and the number of groups.
func sliverGroups(slivers []int, heights []float64, tolerance float64, edgesOf [][3]int,
	ordered []edgeRecord) ([]int, int) {
	groupOf := make([]int, len(heights))
	for index := range groupOf {
		groupOf[index] = -1
	}
	count := 0
	var stack []int
	for _, start := range slivers {
		if groupOf[start] >= 0 {
			continue
		}
		groupOf[start] = count
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			index := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, recordIndex := range edgesOf[index] {
				record := &ordered[recordIndex]
				if record.faces != 2 {
					continue
				}
				other := record.tris[0]
				if other == index {
					other = record.tris[1]
				}
				if groupOf[other] >= 0 || heights[other] >= tolerance || heights[other] == 0 {
					continue
				}
				groupOf[other] = count
				stack = append(stack, other)
			}
		}
		count++
	}
	return groupOf, count
}

// buildEdgeRecords collects the edges of a mesh. A positive tolerance applies
// Edges.Tolerance.
func buildEdgeRecords(vertices []Vec, triangles [][3]int, normals []Vec, tolerance float64) []edgeRecord {
	// Resolve welded positions once per vertex. Edge lookups then use small
	// integer keys instead of hashing six coordinates for every triangle edge.
	weldedIDs := make(map[weldedVec]int, len(vertices))
	vertexIDs := make([]int, len(vertices))
	for index, vertex := range vertices {
		key := weld(vertex)
		id, exists := weldedIDs[key]
		if !exists {
			id = len(weldedIDs) + 1
			weldedIDs[key] = id
		}
		vertexIDs[index] = id
	}
	records := make(map[edgeKey]int, len(triangles)*3/2)
	// ordered holds the same records in first-encounter order. Ranging over
	// records instead would take Go's randomised map order, and overlapping
	// edge lines blend, so the drawn image would differ between calls. A
	// closed mesh shares every edge between two faces, so it has half of the
	// three edges per triangle; an open one appends past that.
	ordered := make([]edgeRecord, 0, len(triangles)*3/2)
	var edgesOf [][3]int
	if tolerance > 0 {
		edgesOf = make([][3]int, len(triangles))
	}
	for triangleIndex, triangle := range triangles {
		corners := [3]Vec{vertices[triangle[0]], vertices[triangle[1]], vertices[triangle[2]]}
		normal := normals[triangleIndex]
		if normal == (Vec{}) {
			continue
		}
		for i, a := range corners {
			b := corners[(i+1)%3]
			key := newEdgeKey(vertexIDs[triangle[i]], vertexIDs[triangle[(i+1)%3]])
			recordIndex, exists := records[key]
			if !exists {
				recordIndex = len(ordered)
				records[key] = recordIndex
				ordered = append(ordered, edgeRecord{
					a: a, b: b, aIndex: triangle[i], bIndex: triangle[(i+1)%3], point: corners[0],
				})
			}
			record := &ordered[recordIndex]
			if record.faces < len(record.normals) {
				record.normals[record.faces] = normal
				record.tris[record.faces] = triangleIndex
			}
			record.faces++
			if edgesOf != nil {
				edgesOf[triangleIndex][i] = recordIndex
			}
		}
	}
	if tolerance > 0 {
		adopted := sliverNormals(vertices, triangles, normals, edgesOf, ordered, tolerance)
		for index := range ordered {
			record := &ordered[index]
			for side := range min(record.faces, len(record.normals)) {
				record.normals[side] = adopted[record.tris[side]]
			}
		}
	}
	return ordered
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
