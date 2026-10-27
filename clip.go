package solidlens

import (
	"fmt"
)

// ClipPlane cuts a model open along a plane. The half-space the Normal points
// into is removed, so a camera on the Normal's side of the plane looks at the
// side that was cut away and sees into the model.
//
// A closed solid cut this way shows the inside of its own surface through the
// cut. Cap, when set, shades that inside, and an unlit material such as
// Material{Color: c, Unlit: true} makes it read as the solid's flat cut face.
// Cap replaces the model's BackMaterial. Without a Cap the inside is shaded as
// Model.BackMaterial, or as Model.Material when that is nil.
type ClipPlane struct {
	// Origin is any point on the plane.
	Origin Vec
	// Normal points toward the removed side. It need not be a unit vector.
	Normal Vec
	// Cap shades the inside of a cut solid. Nil leaves the back material as it is.
	Cap *Material
}

// ClipMesh returns the part of source on the kept side of plane: the triangles
// whose vertices all lie on or behind the plane, and the pieces of the
// triangles the plane crosses. A vertex exactly on the plane is kept. The cut
// is open, so ClipMesh of a closed mesh is an open mesh. Vertices on the cut
// are shared by the triangles that meet there, so edge detection sees one
// surface. It returns an error when plane is not finite or its Normal is zero,
// and when nothing of source is kept.
func ClipMesh(source TriangleSource, plane ClipPlane) (*Mesh, error) {
	if source == nil {
		return nil, fmt.Errorf("solidlens: clip mesh source is nil")
	}
	normal, ok := plane.Normal.Normalize()
	if !ok || !finiteVec(plane.Origin) || !finiteVec(plane.Normal) {
		return nil, fmt.Errorf("solidlens: clip plane needs a finite origin and a non-zero finite normal")
	}
	vertices, triangles := source.Vertices(), source.Triangles()
	distance := make([]float64, len(vertices))
	for i, v := range vertices {
		distance[i] = v.Sub(plane.Origin).Dot(normal)
	}
	var out clipBuilder
	out.source = vertices
	out.distance = distance
	out.kept = make(map[int]int, len(vertices))
	out.crossing = make(map[[2]int]int)
	for i, triangle := range triangles {
		for _, index := range triangle {
			if index < 0 || index >= len(vertices) {
				return nil, fmt.Errorf("solidlens: clip mesh triangle %d references vertex %d", i, index)
			}
		}
		out.clip(triangle)
	}
	if len(out.triangles) == 0 {
		return nil, fmt.Errorf("solidlens: clip plane removes the whole mesh")
	}
	return NewMesh(out.vertices, out.triangles)
}

// clipBuilder accumulates the clipped mesh. kept maps a source vertex to its
// place in the result and crossing maps a source edge, by its two vertices in
// ascending order, to the vertex where the plane cuts it.
type clipBuilder struct {
	source    []Vec
	distance  []float64
	vertices  []Vec
	triangles [][3]int
	kept      map[int]int
	crossing  map[[2]int]int
}

func (b *clipBuilder) keep(index int) int {
	if out, ok := b.kept[index]; ok {
		return out
	}
	out := len(b.vertices)
	b.vertices = append(b.vertices, b.source[index])
	b.kept[index] = out
	return out
}

// cut returns the vertex where the plane crosses the edge between the source
// vertices a and b, one of which is kept and the other removed. The position is
// computed from the lower index so both triangles sharing the edge agree on it.
func (b *clipBuilder) cut(a, c int) int {
	if a > c {
		a, c = c, a
	}
	key := [2]int{a, c}
	if out, ok := b.crossing[key]; ok {
		return out
	}
	da, dc := b.distance[a], b.distance[c]
	t := da / (da - dc)
	out := len(b.vertices)
	b.vertices = append(b.vertices, lerpVec(b.source[a], b.source[c], t))
	b.crossing[key] = out
	return out
}

// clip adds the kept part of one triangle, keeping its winding.
func (b *clipBuilder) clip(triangle [3]int) {
	var polygon []int
	for corner := range 3 {
		current, next := triangle[corner], triangle[(corner+1)%3]
		currentKept, nextKept := b.distance[current] <= 0, b.distance[next] <= 0
		if currentKept {
			polygon = append(polygon, b.keep(current))
		}
		if currentKept != nextKept {
			polygon = append(polygon, b.cut(current, next))
		}
	}
	// A polygon of three or four corners fans into triangles. Corners that
	// coincide, as when the plane only touches a vertex, leave no area.
	for i := 1; i+1 < len(polygon); i++ {
		t := [3]int{polygon[0], polygon[i], polygon[i+1]}
		if t[0] == t[1] || t[1] == t[2] || t[2] == t[0] {
			continue
		}
		b.triangles = append(b.triangles, t)
	}
}
