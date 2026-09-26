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
	lights := prepareLighting(scene)
	var edgeGroups []edgeGroup
	for modelIndex, model := range scene.Models {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if model.Mesh == nil {
			return nil, fmt.Errorf("solidlens: model %d has no mesh", modelIndex)
		}
		if !model.Material.valid() {
			return nil, fmt.Errorf("solidlens: model %d has an invalid material", modelIndex)
		}
		back := model.Material
		if model.BackMaterial != nil {
			if !model.BackMaterial.valid() {
				return nil, fmt.Errorf("solidlens: model %d has an invalid back material", modelIndex)
			}
			back = *model.BackMaterial
		}
		var vertices []Vec
		var triangles [][3]int
		if mesh, ok := model.Mesh.(*Mesh); ok && mesh != nil {
			// Mesh owns immutable slices, so rendering can read them without copies.
			vertices, triangles = mesh.vertices, mesh.triangles
		} else {
			vertices, triangles = model.Mesh.Vertices(), model.Mesh.Triangles()
		}
		var projectedVertices []screenVertex
		var projectedReady []bool
		if len(triangles) >= len(vertices) && len(vertices) > 0 {
			projectedVertices = make([]screenVertex, len(vertices))
			projectedReady = make([]bool, len(vertices))
		}
		var edgeNormals []Vec
		if model.Edges.Enabled {
			edgeNormals = make([]Vec, len(triangles))
		}
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
			if edgeNormals != nil {
				edgeNormals[triangleIndex] = normal
			}
			// A triangle seen from behind is shaded as its back side, with the
			// normal turned toward the camera. On a closed mesh these faces
			// are hidden by nearer ones, and on an open mesh they are the
			// visible inner side.
			material := model.Material
			if normal.Dot(camera.Position.Sub(world[0])) < 0 {
				normal = normal.Scale(-1)
				material = back
			}
			shade := shadeTriangle(lights, material, normal, world[0])
			var projected [3]screenVertex
			var visible bool
			if projectedVertices == nil {
				projected, visible = view.projectTriangle(world)
			} else {
				visible = true
				for corner, index := range triangle {
					if !projectedReady[index] {
						projectedVertices[index] = view.projectClippedPoint(vertices[index])
						projectedReady[index] = true
					}
					projected[corner] = projectedVertices[index]
					if projected[corner].z < view.near || projected[corner].z > view.far {
						visible = false
					}
				}
			}
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
				style:     edges,
				segments:  collectEdges(vertices, triangles, edgeNormals, camera.Position, edges),
				projected: projectedVertices,
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
				accumulateSegment(canvas, depth, view, segment, group.projected, group.style.Width)
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

func (v view) projectClippedPoint(point Vec) screenVertex {
	relative := point.Sub(v.position)
	z := relative.Dot(v.forward)
	if z < v.near || z > v.far {
		return screenVertex{z: z}
	}
	x := relative.Dot(v.right) * v.focal / (z * v.aspect)
	y := relative.Dot(v.up) * v.focal / z
	return screenVertex{
		x: (x + 1) * float64(v.width) / 2,
		y: (1 - y) * float64(v.height) / 2,
		z: z,
	}
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
	style     Edges
	segments  []segment
	projected []screenVertex
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
	if x < c.minX {
		c.minX = x
	}
	if x > c.maxX {
		c.maxX = x
	}
	if y < c.minY {
		c.minY = y
	}
	if y > c.maxY {
		c.maxY = y
	}
}

// composite draws the accumulated coverage in lineColor, one blend per pixel.
func (c *edgeCanvas) composite(image *image.RGBA, lineColor Color) {
	source := lineColor.NRGBA()
	opacity := clamp(lineColor.A, 0, 1)
	for y := c.minY; y <= c.maxY; y++ {
		for x := c.minX; x <= c.maxX; x++ {
			if coverage := c.coverage[y*c.stride+x]; coverage > 0 {
				blend(image, x, y, source, opacity*coverage)
			}
		}
	}
}

func accumulateSegment(canvas *edgeCanvas, depth []float64, v view, s segment, projected []screenVertex, width float64) {
	if projected != nil {
		aProjected, bProjected := projected[s.aIndex], projected[s.bIndex]
		if aProjected.z >= v.near && aProjected.z <= v.far && bProjected.z >= v.near && bProjected.z <= v.far {
			accumulateLine(canvas, depth, aProjected, bProjected, width)
			return
		}
	}
	a, b, ok := v.clipSegment(s.a, s.b)
	if !ok {
		return
	}
	accumulateLine(canvas, depth, v.projectPoint(a), v.projectPoint(b), width)
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
	minX, maxX := a.x, b.x
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := a.y, b.y
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	x0 := clampPixel(math.Floor(minX-extent), stride-1)
	x1 := clampPixel(math.Ceil(maxX+extent), stride-1)
	y0 := clampPixel(math.Floor(minY-extent), canvas.height-1)
	y1 := clampPixel(math.Ceil(maxY+extent), canvas.height-1)
	deltaX, deltaY := b.x-a.x, b.y-a.y
	lengthSquared := deltaX*deltaX + deltaY*deltaY
	inverseLengthSquared := 0.0
	if lengthSquared > 0 {
		inverseLengthSquared = 1 / lengthSquared
	}
	extentSquared := extent * extent
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			relativeX, relativeY := px-a.x, py-a.y
			position := 0.0
			if lengthSquared > 0 {
				position = (relativeX*deltaX + relativeY*deltaY) * inverseLengthSquared
			}
			var distanceSquared float64
			if position < 0 {
				position = 0
				distanceSquared = relativeX*relativeX + relativeY*relativeY
			} else if position > 1 {
				position = 1
				distanceX, distanceY := px-b.x, py-b.y
				distanceSquared = distanceX*distanceX + distanceY*distanceY
			} else {
				cross := relativeX*deltaY - relativeY*deltaX
				distanceSquared = cross * cross * inverseLengthSquared
			}
			if lengthSquared == 0 {
				distanceSquared = relativeX*relativeX + relativeY*relativeY
			}
			if distanceSquared >= extentSquared {
				continue
			}
			coverage := extent - math.Sqrt(distanceSquared)
			if coverage > 1 {
				coverage = 1
			}
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

func clampPixel(value float64, maximum int) int {
	if value <= 0 {
		return 0
	}
	if value >= float64(maximum) {
		return maximum
	}
	return int(value)
}

// blend composites a color over one pixel without touching the depth buffer.
func blend(image *image.RGBA, x, y int, source color.NRGBA, alpha float64) {
	if alpha <= 0 {
		return
	}
	offset := image.PixOffset(x, y)
	inverse := 1 - alpha
	image.Pix[offset] = uint8(float64(source.R)*alpha + float64(image.Pix[offset])*inverse + 0.5)
	image.Pix[offset+1] = uint8(float64(source.G)*alpha + float64(image.Pix[offset+1])*inverse + 0.5)
	image.Pix[offset+2] = uint8(float64(source.B)*alpha + float64(image.Pix[offset+2])*inverse + 0.5)
	image.Pix[offset+3] = uint8(255*alpha + float64(image.Pix[offset+3])*inverse + 0.5)
}

type preparedDirectionalLight struct {
	direction Vec
	intensity float64
	luminance float64
}

type preparedPointLight struct {
	position  Vec
	intensity float64
	luminance float64
}

type lighting struct {
	directional []preparedDirectionalLight
	points      []preparedPointLight
}

func prepareLighting(scene Scene) lighting {
	prepared := lighting{
		directional: make([]preparedDirectionalLight, 0, len(scene.DirectionalLights)),
		points:      make([]preparedPointLight, 0, len(scene.PointLights)),
	}
	for _, light := range scene.DirectionalLights {
		if !finiteVec(light.Direction) || !finiteColor(light.Color) || !finite(light.Intensity) {
			continue
		}
		direction, ok := light.Direction.Normalize()
		if !ok {
			continue
		}
		prepared.directional = append(prepared.directional, preparedDirectionalLight{
			direction: direction.Scale(-1),
			intensity: light.Intensity,
			luminance: luminance(light.Color),
		})
	}
	for _, light := range scene.PointLights {
		if !finiteVec(light.Position) || !finiteColor(light.Color) || !finite(light.Intensity) {
			continue
		}
		prepared.points = append(prepared.points, preparedPointLight{
			position:  light.Position,
			intensity: light.Intensity,
			luminance: luminance(light.Color),
		})
	}
	return prepared
}

func shadeTriangle(lights lighting, material Material, normal, position Vec) Color {
	intensity := math.Max(0, material.Ambient)
	for _, light := range lights.directional {
		intensity += math.Max(0, normal.Dot(light.direction)*light.intensity) * light.luminance
	}
	for _, light := range lights.points {
		delta := light.position.Sub(position)
		distanceSquared := delta.Dot(delta)
		direction, ok := delta.Normalize()
		if !ok {
			continue
		}
		intensity += math.Max(0, normal.Dot(direction)) * light.intensity * light.luminance /
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
	inverseArea := 1 / area
	pixel := premultiply(fillColor.NRGBA())
	width := image.Bounds().Dx()
	startX, startY := float64(minX)+0.5, float64(minY)+0.5
	rowW0 := edge(triangle[1], triangle[2], startX, startY) * inverseArea
	rowW1 := edge(triangle[2], triangle[0], startX, startY) * inverseArea
	rowW2 := edge(triangle[0], triangle[1], startX, startY) * inverseArea
	dx0 := (triangle[2].y - triangle[1].y) * inverseArea
	dx1 := (triangle[0].y - triangle[2].y) * inverseArea
	dx2 := (triangle[1].y - triangle[0].y) * inverseArea
	dy0 := (triangle[1].x - triangle[2].x) * inverseArea
	dy1 := (triangle[2].x - triangle[0].x) * inverseArea
	dy2 := (triangle[0].x - triangle[1].x) * inverseArea
	for y := minY; y <= maxY; y++ {
		w0, w1, w2 := rowW0, rowW1, rowW2
		for x := minX; x <= maxX; x++ {
			// Incremental weights can drift across an exact shared edge.
			// Recheck those pixels with the original edge equations.
			const edgeTolerance = 1e-9
			if w0 >= -edgeTolerance && w1 >= -edgeTolerance && w2 >= -edgeTolerance {
				pixelW0, pixelW1, pixelW2 := w0, w1, w2
				if w0 < edgeTolerance || w1 < edgeTolerance || w2 < edgeTolerance {
					px, py := float64(x)+0.5, float64(y)+0.5
					pixelW0 = edge(triangle[1], triangle[2], px, py) * inverseArea
					pixelW1 = edge(triangle[2], triangle[0], px, py) * inverseArea
					pixelW2 = edge(triangle[0], triangle[1], px, py) * inverseArea
				}
				if pixelW0 < 0 || pixelW1 < 0 || pixelW2 < 0 {
					w0, w1, w2 = w0+dx0, w1+dx1, w2+dx2
					continue
				}
				index := y*width + x
				z := pixelW0*triangle[0].z + pixelW1*triangle[1].z + pixelW2*triangle[2].z
				if z < depth[index] {
					depth[index] = z
					offset := y*image.Stride + x*4
					image.Pix[offset] = pixel.R
					image.Pix[offset+1] = pixel.G
					image.Pix[offset+2] = pixel.B
					image.Pix[offset+3] = pixel.A
				}
			}
			w0, w1, w2 = w0+dx0, w1+dx1, w2+dx2
		}
		rowW0, rowW1, rowW2 = rowW0+dy0, rowW1+dy1, rowW2+dy2
	}
}

func edge(a, b screenVertex, x, y float64) float64 {
	return (x-a.x)*(b.y-a.y) - (y-a.y)*(b.x-a.x)
}

func premultiply(c color.NRGBA) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func fill(image *image.RGBA, fillColor color.NRGBA) {
	converted := premultiply(fillColor)
	width := image.Bounds().Dx() * 4
	for y := range image.Bounds().Dy() {
		row := image.Pix[y*image.Stride : y*image.Stride+width]
		for x := 0; x < width; x += 4 {
			row[x] = converted.R
			row[x+1] = converted.G
			row[x+2] = converted.B
			row[x+3] = converted.A
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
