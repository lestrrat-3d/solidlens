package solidlens

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
)

// Render draws scene into a new RGBA image. It has no mutable package or
// receiver state, so concurrent calls are independent.
func Render(ctx context.Context, scene Scene, settings Settings) (*image.RGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	settings, err := settings.normalized()
	if err != nil {
		return nil, err
	}
	camera, err := scene.Camera.normalized()
	if err != nil {
		return nil, err
	}
	if !finiteColor(scene.Background) {
		return nil, fmt.Errorf("solidlens: background color is not finite")
	}
	view, err := newView(camera, settings)
	if err != nil {
		return nil, err
	}
	image := image.NewRGBA(image.Rect(0, 0, settings.Width, settings.Height))
	fill(image, scene.Background.NRGBA())
	depth := make([]float64, settings.Width*settings.Height)
	for index := range depth {
		depth[index] = math.Inf(1)
	}
	var edgeGroups []edgeGroup
	for modelIndex, model := range scene.Models {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if model.Mesh == nil {
			return nil, fmt.Errorf("solidlens: model %d has no mesh", modelIndex)
		}
		if !finiteColor(model.Material.Color) || !finite(model.Material.Ambient) {
			return nil, fmt.Errorf("solidlens: model %d has an invalid material", modelIndex)
		}
		vertices := model.Mesh.Vertices()
		triangles := model.Mesh.Triangles()
		for triangleIndex, triangle := range triangles {
			if triangle[0] < 0 || triangle[1] < 0 || triangle[2] < 0 ||
				triangle[0] >= len(vertices) || triangle[1] >= len(vertices) || triangle[2] >= len(vertices) {
				return nil, fmt.Errorf("solidlens: model %d triangle %d has an invalid index", modelIndex, triangleIndex)
			}
			world := [3]Vec{vertices[triangle[0]], vertices[triangle[1]], vertices[triangle[2]]}
			if !finiteVec(world[0]) || !finiteVec(world[1]) || !finiteVec(world[2]) {
				return nil, fmt.Errorf("solidlens: model %d triangle %d has a non-finite vertex", modelIndex, triangleIndex)
			}
			normal, ok := world[1].Sub(world[0]).Cross(world[2].Sub(world[0])).Normalize()
			if !ok {
				continue
			}
			shade := shadeTriangle(scene, model.Material, normal, world[0])
			projected, visible := view.projectTriangle(world)
			if !visible {
				continue
			}
			rasterize(image, depth, projected, shade)
		}
		if model.Edges.Enabled {
			edges, err := model.Edges.normalized()
			if err != nil {
				return nil, fmt.Errorf("solidlens: model %d: %w", modelIndex, err)
			}
			edgeGroups = append(edgeGroups, edgeGroup{
				style:    edges,
				segments: collectEdges(vertices, triangles, camera.Position, edges),
			})
		}
	}
	// Edges are drawn last so that they test against the finished depth
	// buffer and stay hidden behind the surfaces of every model. Each model's
	// lines are gathered into a coverage buffer and composited in one pass, so
	// a pixel that several of its lines cross is drawn once at the strongest
	// coverage rather than blended once per line.
	if len(edgeGroups) > 0 {
		canvas := newEdgeCanvas(settings.Width, settings.Height)
		for _, group := range edgeGroups {
			canvas.reset()
			for index, segment := range group.segments {
				if index%1024 == 0 {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
				}
				accumulateSegment(canvas, depth, view, segment)
			}
			canvas.composite(image, group.style.Color)
		}
	}
	return image, nil
}

// RenderPNG renders scene and writes it as a PNG image.
func RenderPNG(ctx context.Context, w io.Writer, scene Scene, settings Settings) error {
	image, err := Render(ctx, scene, settings)
	if err != nil {
		return err
	}
	if err := png.Encode(w, image); err != nil {
		return fmt.Errorf("solidlens: write PNG: %w", err)
	}
	return nil
}

type view struct {
	position Vec
	right    Vec
	up       Vec
	forward  Vec
	focal    float64
	aspect   float64
	near     float64
	far      float64
	width    int
	height   int
}

type screenVertex struct {
	x, y, z float64
}

func newView(camera Camera, settings Settings) (view, error) {
	forward, ok := camera.Target.Sub(camera.Position).Normalize()
	if !ok {
		return view{}, fmt.Errorf("solidlens: camera direction is zero")
	}
	right, ok := forward.Cross(camera.Up).Normalize()
	if !ok {
		return view{}, fmt.Errorf("solidlens: camera up vector is parallel to its direction")
	}
	up, ok := right.Cross(forward).Normalize()
	if !ok {
		return view{}, fmt.Errorf("solidlens: camera basis is invalid")
	}
	return view{
		position: camera.Position,
		right:    right,
		up:       up,
		forward:  forward,
		focal:    1 / math.Tan(camera.FOV*math.Pi/360),
		aspect:   float64(settings.Width) / float64(settings.Height),
		near:     camera.Near,
		far:      camera.Far,
		width:    settings.Width,
		height:   settings.Height,
	}, nil
}

func (v view) projectTriangle(vertices [3]Vec) ([3]screenVertex, bool) {
	var projected [3]screenVertex
	for i, vertex := range vertices {
		if z := v.depthOf(vertex); z < v.near || z > v.far {
			return projected, false
		}
		projected[i] = v.projectPoint(vertex)
	}
	return projected, true
}

// depthOf returns the camera-space distance of point along the view direction.
func (v view) depthOf(point Vec) float64 { return point.Sub(v.position).Dot(v.forward) }

// projectPoint maps a world point to pixel coordinates. The caller is
// responsible for keeping the point inside the clipping range.
func (v view) projectPoint(point Vec) screenVertex {
	relative := point.Sub(v.position)
	z := relative.Dot(v.forward)
	x := relative.Dot(v.right) * v.focal / (z * v.aspect)
	y := relative.Dot(v.up) * v.focal / z
	return screenVertex{
		x: (x + 1) * float64(v.width) / 2,
		y: (1 - y) * float64(v.height) / 2,
		z: z,
	}
}

// clipSegment shortens a world-space segment to the camera clipping range. It
// reports false when the segment lies entirely outside that range.
func (v view) clipSegment(a, b Vec) (Vec, Vec, bool) {
	depthA, depthB := v.depthOf(a), v.depthOf(b)
	if (depthA < v.near && depthB < v.near) || (depthA > v.far && depthB > v.far) {
		return a, b, false
	}
	if depthA < v.near {
		a = lerpVec(a, b, (v.near-depthA)/(depthB-depthA))
	} else if depthB < v.near {
		b = lerpVec(b, a, (v.near-depthB)/(depthA-depthB))
	}
	if math.IsInf(v.far, 1) {
		return a, b, true
	}
	depthA, depthB = v.depthOf(a), v.depthOf(b)
	if depthA > v.far {
		a = lerpVec(a, b, (v.far-depthA)/(depthB-depthA))
	} else if depthB > v.far {
		b = lerpVec(b, a, (v.far-depthB)/(depthA-depthB))
	}
	return a, b, true
}

func lerpVec(a, b Vec, t float64) Vec { return a.Add(b.Sub(a).Scale(t)) }

// edgeDepthBias lets an edge line win the depth test against the surfaces it
// borders, which share its depth to within rounding error.
const edgeDepthBias = 1e-3

// edgeGroup is one model's edge lines. They share a style, so they can be
// accumulated together and composited in a single pass.
type edgeGroup struct {
	style    Edges
	segments []segment
}

// edgeCanvas accumulates the coverage of one model's edge lines. A pixel that
// several lines cross keeps the strongest coverage rather than being blended
// once per line, which is what used to darken the seams where lines meet.
// Keeping the strongest also makes the result independent of the order the
// lines arrive in.
type edgeCanvas struct {
	coverage       []float64
	stride, height int
	// Bounds of the pixels touched since the last reset, inclusive. The
	// range is empty while maxX is below minX.
	minX, minY, maxX, maxY int
}

func newEdgeCanvas(width, height int) *edgeCanvas {
	canvas := &edgeCanvas{coverage: make([]float64, width*height), stride: width, height: height}
	canvas.reset()
	return canvas
}

// reset clears the coverage left by the previous model. Only the pixels that
// model touched are cleared, so a second model costs its own area rather than
// the whole frame.
func (c *edgeCanvas) reset() {
	for y := c.minY; y <= c.maxY; y++ {
		clear(c.coverage[y*c.stride+c.minX : y*c.stride+c.maxX+1])
	}
	c.minX, c.minY = c.stride, c.height
	c.maxX, c.maxY = -1, -1
}

func (c *edgeCanvas) add(x, y int, coverage float64) {
	index := y*c.stride + x
	if coverage <= c.coverage[index] {
		return
	}
	c.coverage[index] = coverage
	c.minX, c.maxX = min(c.minX, x), max(c.maxX, x)
	c.minY, c.maxY = min(c.minY, y), max(c.maxY, y)
}

// composite draws the accumulated coverage in lineColor, one blend per pixel.
func (c *edgeCanvas) composite(image *image.RGBA, lineColor Color) {
	for y := c.minY; y <= c.maxY; y++ {
		for x := c.minX; x <= c.maxX; x++ {
			if coverage := c.coverage[y*c.stride+x]; coverage > 0 {
				blend(image, x, y, lineColor, coverage)
			}
		}
	}
}

func accumulateSegment(canvas *edgeCanvas, depth []float64, v view, s segment) {
	a, b, ok := v.clipSegment(s.a, s.b)
	if !ok {
		return
	}
	accumulateLine(canvas, depth, v.projectPoint(a), v.projectPoint(b), s.style.Width)
}

// accumulateLine records the coverage of a depth-tested line of the given
// pixel width. Coverage of the half pixel at the line border is fractional, so
// diagonals stay readable.
func accumulateLine(canvas *edgeCanvas, depth []float64, a, b screenVertex, width float64) {
	if !finite(a.x) || !finite(a.y) || !finite(a.z) || !finite(b.x) || !finite(b.y) || !finite(b.z) {
		return
	}
	stride := canvas.stride
	extent := width/2 + 0.5
	minX := int(clamp(math.Floor(math.Min(a.x, b.x)-extent), 0, float64(stride-1)))
	maxX := int(clamp(math.Ceil(math.Max(a.x, b.x)+extent), 0, float64(stride-1)))
	minY := int(clamp(math.Floor(math.Min(a.y, b.y)-extent), 0, float64(canvas.height-1)))
	maxY := int(clamp(math.Ceil(math.Max(a.y, b.y)+extent), 0, float64(canvas.height-1)))
	deltaX, deltaY := b.x-a.x, b.y-a.y
	lengthSquared := deltaX*deltaX + deltaY*deltaY
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			position := 0.0
			if lengthSquared > 0 {
				position = clamp(((px-a.x)*deltaX+(py-a.y)*deltaY)/lengthSquared, 0, 1)
			}
			coverage := clamp(extent-math.Hypot(px-(a.x+position*deltaX), py-(a.y+position*deltaY)), 0, 1)
			if coverage <= 0 {
				continue
			}
			z := a.z + position*(b.z-a.z)
			if z > depth[y*stride+x]*(1+edgeDepthBias) {
				continue
			}
			canvas.add(x, y, coverage)
		}
	}
}

// blend composites a color over one pixel without touching the depth buffer.
func blend(image *image.RGBA, x, y int, over Color, coverage float64) {
	alpha := clamp(over.A, 0, 1) * coverage
	if alpha <= 0 {
		return
	}
	source := over.NRGBA()
	destination := image.RGBAAt(x, y)
	inverse := 1 - alpha
	image.SetRGBA(x, y, color.RGBA{
		R: uint8(float64(source.R)*alpha + float64(destination.R)*inverse + 0.5),
		G: uint8(float64(source.G)*alpha + float64(destination.G)*inverse + 0.5),
		B: uint8(float64(source.B)*alpha + float64(destination.B)*inverse + 0.5),
		A: uint8(255*alpha + float64(destination.A)*inverse + 0.5),
	})
}

func shadeTriangle(scene Scene, material Material, normal, position Vec) Color {
	intensity := math.Max(0, material.Ambient)
	for _, light := range scene.DirectionalLights {
		if !finiteVec(light.Direction) || !finiteColor(light.Color) || !finite(light.Intensity) {
			continue
		}
		direction, ok := light.Direction.Normalize()
		if !ok {
			continue
		}
		intensity += math.Max(0, normal.Dot(direction.Scale(-1))*light.Intensity) * luminance(light.Color)
	}
	for _, light := range scene.PointLights {
		if !finiteVec(light.Position) || !finiteColor(light.Color) || !finite(light.Intensity) {
			continue
		}
		delta := light.Position.Sub(position)
		distanceSquared := delta.Dot(delta)
		direction, ok := delta.Normalize()
		if !ok {
			continue
		}
		intensity += math.Max(0, normal.Dot(direction)) * light.Intensity * luminance(light.Color) /
			math.Max(1, distanceSquared)
	}
	return Color{
		R: material.Color.R * intensity,
		G: material.Color.G * intensity,
		B: material.Color.B * intensity,
		A: material.Color.A,
	}
}

func rasterize(image *image.RGBA, depth []float64, triangle [3]screenVertex, fillColor Color) {
	area := edge(triangle[0], triangle[1], triangle[2].x, triangle[2].y)
	if area == 0 || math.IsNaN(area) || math.IsInf(area, 0) {
		return
	}
	minX := maxInt(0, int(math.Floor(min3(triangle[0].x, triangle[1].x, triangle[2].x))))
	maxX := minInt(image.Bounds().Dx()-1, int(math.Ceil(max3(triangle[0].x, triangle[1].x, triangle[2].x))))
	minY := maxInt(0, int(math.Floor(min3(triangle[0].y, triangle[1].y, triangle[2].y))))
	maxY := minInt(image.Bounds().Dy()-1, int(math.Ceil(max3(triangle[0].y, triangle[1].y, triangle[2].y))))
	if minX > maxX || minY > maxY {
		return
	}
	color := fillColor.NRGBA()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			w0 := edge(triangle[1], triangle[2], px, py) / area
			w1 := edge(triangle[2], triangle[0], px, py) / area
			w2 := edge(triangle[0], triangle[1], px, py) / area
			if w0 < 0 || w1 < 0 || w2 < 0 {
				continue
			}
			index := y*image.Bounds().Dx() + x
			z := w0*triangle[0].z + w1*triangle[1].z + w2*triangle[2].z
			if z >= depth[index] {
				continue
			}
			depth[index] = z
			image.Set(x, y, color)
		}
	}
}

func edge(a, b screenVertex, x, y float64) float64 {
	return (x-a.x)*(b.y-a.y) - (y-a.y)*(b.x-a.x)
}

func fill(image *image.RGBA, color color.NRGBA) {
	for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
		for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
			image.Set(x, y, color)
		}
	}
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func finiteColor(c Color) bool { return finite(c.R) && finite(c.G) && finite(c.B) && finite(c.A) }

func luminance(c Color) float64 { return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B }

func min3(a, b, c float64) float64 { return math.Min(a, math.Min(b, c)) }

func max3(a, b, c float64) float64 { return math.Max(a, math.Max(b, c)) }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
