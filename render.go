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
		relative := vertex.Sub(v.position)
		z := relative.Dot(v.forward)
		if z < v.near || z > v.far {
			return projected, false
		}
		x := relative.Dot(v.right) * v.focal / (z * v.aspect)
		y := relative.Dot(v.up) * v.focal / z
		projected[i] = screenVertex{
			x: (x + 1) * float64(v.width) / 2,
			y: (1 - y) * float64(v.height) / 2,
			z: z,
		}
	}
	return projected, true
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
