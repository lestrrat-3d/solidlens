package solidlens

import (
	"fmt"
	"image/color"
	"math"
)

// Color is a linear RGBA color. Components normally range from zero to one.
type Color struct {
	R, G, B, A float64
}

// RGB returns an opaque color.
func RGB(r, g, b float64) Color { return Color{R: r, G: g, B: b, A: 1} }

// RGBA returns a color with the supplied alpha value.
func RGBA(r, g, b, a float64) Color { return Color{R: r, G: g, B: b, A: a} }

// NRGBA converts the linear color to a standard-library color value.
func (c Color) NRGBA() color.NRGBA {
	return color.NRGBA{
		R: linearToSRGB(c.R),
		G: linearToSRGB(c.G),
		B: linearToSRGB(c.B),
		A: uint8(clamp(c.A, 0, 1)*255 + 0.5),
	}
}

// Material controls a model's diffuse surface color and ambient contribution.
type Material struct {
	Color   Color
	Ambient float64
}

// Matte returns a diffuse material with a small ambient contribution.
func Matte(c Color) Material { return Material{Color: c, Ambient: 0.12} }

// Model places a mesh in a scene. Mesh coordinates are in world space. Edges
// draws lines along the model's edges and is off by default.
type Model struct {
	Mesh     TriangleSource
	Material Material
	Edges    Edges
}

// Camera describes a perspective camera looking from Position toward Target.
// Up defaults to the positive Z axis. FOV is the vertical field of view in
// degrees and defaults to 45.
type Camera struct {
	Position Vec
	Target   Vec
	Up       Vec
	FOV      float64
	Near     float64
	Far      float64
}

// DirectionalLight shines in Direction, which points from the light toward
// the scene.
type DirectionalLight struct {
	Direction Vec
	Color     Color
	Intensity float64
}

// PointLight shines outward from Position with inverse-square falloff.
type PointLight struct {
	Position  Vec
	Color     Color
	Intensity float64
}

// Scene is a complete renderable scene layer.
type Scene struct {
	Camera            Camera
	Models            []Model
	DirectionalLights []DirectionalLight
	PointLights       []PointLight
	Background        Color
}

// Settings controls the output raster.
type Settings struct {
	Width  int
	Height int
}

func (s Settings) normalized() (Settings, error) {
	if s.Width == 0 {
		s.Width = 800
	}
	if s.Height == 0 {
		s.Height = 600
	}
	if s.Width < 1 || s.Height < 1 {
		return Settings{}, fmt.Errorf("solidlens: image dimensions must be positive")
	}
	if s.Width > 16384 || s.Height > 16384 {
		return Settings{}, fmt.Errorf("solidlens: image dimensions exceed 16384 pixels")
	}
	return s, nil
}

func (c Camera) normalized() (Camera, error) {
	if !finiteVec(c.Position) || !finiteVec(c.Target) || !finiteVec(c.Up) {
		return Camera{}, fmt.Errorf("solidlens: camera contains a non-finite vector")
	}
	if c.Up == (Vec{}) {
		c.Up = Vec{Z: 1}
	}
	if c.FOV == 0 {
		c.FOV = 45
	}
	if c.Near == 0 {
		c.Near = 0.001
	}
	if c.Far == 0 {
		c.Far = math.Inf(1)
	}
	if c.FOV <= 0 || c.FOV >= 179 {
		return Camera{}, fmt.Errorf("solidlens: camera FOV must be between 0 and 179 degrees")
	}
	if c.Near <= 0 || c.Far <= c.Near {
		return Camera{}, fmt.Errorf("solidlens: camera clipping range is invalid")
	}
	if c.Target.Sub(c.Position).Len() == 0 || c.Up.Len() == 0 {
		return Camera{}, fmt.Errorf("solidlens: camera direction or up vector is zero")
	}
	return c, nil
}

func linearToSRGB(v float64) uint8 {
	v = clamp(v, 0, 1)
	if v <= 0.0031308 {
		return uint8(v*12.92*255 + 0.5)
	}
	return uint8((1.055*math.Pow(v, 1/2.4)-0.055)*255 + 0.5)
}

func clamp(v, low, high float64) float64 {
	return math.Min(math.Max(v, low), high)
}
