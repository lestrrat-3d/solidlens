package main

import (
	"math"

	"github.com/lestrrat-3d/solidlens"
)

const (
	heroStone = iota
	heroStoneLight
	heroSoil
	heroSoilLight
	heroGrass
	heroGrassLight
	heroGrassDark
	heroPath
	heroPathLight
	heroWater
	heroWood
	heroDarkWood
	heroLeaves
	heroRoof
	heroLights
	heroClouds
)

type heroLayer struct {
	name    string
	color   solidlens.Color
	builder *voxelBuilder
}

func heroLayers() []heroLayer {
	return []heroLayer{
		{"hero-stone.stl", solidlens.RGB(0.27, 0.35, 0.43), newVoxelBuilder()},
		{"hero-stone-light.stl", solidlens.RGB(0.37, 0.43, 0.46), newVoxelBuilder()},
		{"hero-soil.stl", solidlens.RGB(0.43, 0.25, 0.12), newVoxelBuilder()},
		{"hero-soil-light.stl", solidlens.RGB(0.52, 0.32, 0.16), newVoxelBuilder()},
		{"hero-grass.stl", solidlens.RGB(0.12, 0.42, 0.18), newVoxelBuilder()},
		{"hero-grass-light.stl", solidlens.RGB(0.24, 0.55, 0.23), newVoxelBuilder()},
		{"hero-grass-dark.stl", solidlens.RGB(0.07, 0.31, 0.15), newVoxelBuilder()},
		{"hero-path.stl", solidlens.RGB(0.68, 0.49, 0.28), newVoxelBuilder()},
		{"hero-path-light.stl", solidlens.RGB(0.83, 0.66, 0.41), newVoxelBuilder()},
		{"hero-water.stl", solidlens.RGB(0.02, 0.43, 0.82), newVoxelBuilder()},
		{"hero-wood.stl", solidlens.RGB(0.48, 0.28, 0.13), newVoxelBuilder()},
		{"hero-dark-wood.stl", solidlens.RGB(0.18, 0.1, 0.05), newVoxelBuilder()},
		{"hero-leaves.stl", solidlens.RGB(0.04, 0.29, 0.13), newVoxelBuilder()},
		{"hero-roof.stl", solidlens.RGB(0.67, 0.16, 0.1), newVoxelBuilder()},
		{"hero-lights.stl", solidlens.RGB(1, 0.75, 0.28), newVoxelBuilder()},
		{"hero-clouds.stl", solidlens.RGB(0.54, 0.68, 0.78), newVoxelBuilder()},
	}
}

type voxelBuilder struct{ *builder }

func newVoxelBuilder() *voxelBuilder { return &voxelBuilder{builder: newBuilder()} }

func (b *voxelBuilder) box(center, size solidlens.Vec) {
	const pitch = 0.125
	const offset = 0.004
	nx := max(1, int(math.Ceil(size.X/pitch)))
	ny := max(1, int(math.Ceil(size.Y/pitch)))
	nz := max(1, int(math.Ceil(size.Z/pitch)))
	cell := solidlens.Vec{X: size.X / float64(nx), Y: size.Y / float64(ny), Z: size.Z / float64(nz)}
	corner := center.Sub(size.Scale(0.5))
	b.builder.box(center, size)
	insetX := min(offset, cell.X*0.1)
	insetY := min(offset, cell.Y*0.1)
	insetZ := min(offset, cell.Z*0.1)
	for x := range nx {
		x0 := corner.X + float64(x)*cell.X + insetX
		x1 := corner.X + float64(x+1)*cell.X - insetX
		for y := range ny {
			y0 := corner.Y + float64(y)*cell.Y + insetY
			y1 := corner.Y + float64(y+1)*cell.Y - insetY
			b.tile(
				solidlens.Vec{X: x0, Y: y0, Z: corner.Z - offset},
				solidlens.Vec{X: x1, Y: y0, Z: corner.Z - offset},
				solidlens.Vec{X: x1, Y: y1, Z: corner.Z - offset},
				solidlens.Vec{X: x0, Y: y1, Z: corner.Z - offset},
			)
			b.tile(
				solidlens.Vec{X: x0, Y: y0, Z: corner.Z + size.Z + offset},
				solidlens.Vec{X: x1, Y: y0, Z: corner.Z + size.Z + offset},
				solidlens.Vec{X: x1, Y: y1, Z: corner.Z + size.Z + offset},
				solidlens.Vec{X: x0, Y: y1, Z: corner.Z + size.Z + offset},
			)
		}
	}
	for x := range nx {
		x0 := corner.X + float64(x)*cell.X + insetX
		x1 := corner.X + float64(x+1)*cell.X - insetX
		for z := range nz {
			z0 := corner.Z + float64(z)*cell.Z + insetZ
			z1 := corner.Z + float64(z+1)*cell.Z - insetZ
			b.tile(
				solidlens.Vec{X: x0, Y: corner.Y - offset, Z: z0},
				solidlens.Vec{X: x1, Y: corner.Y - offset, Z: z0},
				solidlens.Vec{X: x1, Y: corner.Y - offset, Z: z1},
				solidlens.Vec{X: x0, Y: corner.Y - offset, Z: z1},
			)
			b.tile(
				solidlens.Vec{X: x0, Y: corner.Y + size.Y + offset, Z: z0},
				solidlens.Vec{X: x1, Y: corner.Y + size.Y + offset, Z: z0},
				solidlens.Vec{X: x1, Y: corner.Y + size.Y + offset, Z: z1},
				solidlens.Vec{X: x0, Y: corner.Y + size.Y + offset, Z: z1},
			)
		}
	}
	for y := range ny {
		y0 := corner.Y + float64(y)*cell.Y + insetY
		y1 := corner.Y + float64(y+1)*cell.Y - insetY
		for z := range nz {
			z0 := corner.Z + float64(z)*cell.Z + insetZ
			z1 := corner.Z + float64(z+1)*cell.Z - insetZ
			b.tile(
				solidlens.Vec{X: corner.X - offset, Y: y0, Z: z0},
				solidlens.Vec{X: corner.X - offset, Y: y1, Z: z0},
				solidlens.Vec{X: corner.X - offset, Y: y1, Z: z1},
				solidlens.Vec{X: corner.X - offset, Y: y0, Z: z1},
			)
			b.tile(
				solidlens.Vec{X: corner.X + size.X + offset, Y: y0, Z: z0},
				solidlens.Vec{X: corner.X + size.X + offset, Y: y1, Z: z0},
				solidlens.Vec{X: corner.X + size.X + offset, Y: y1, Z: z1},
				solidlens.Vec{X: corner.X + size.X + offset, Y: y0, Z: z1},
			)
		}
	}
}

func (b *voxelBuilder) tile(p0, p1, p2, p3 solidlens.Vec) {
	b.add([]solidlens.Vec{p0, p1, p2, p3}, [][3]int{{0, 1, 2}, {0, 2, 3}})
}

func heroScene() (solidlens.Scene, error) {
	layers := heroLayers()
	models := make([]solidlens.Model, 0, len(layers))
	for _, layer := range layers {
		mesh, err := readModel(layer.name)
		if err != nil {
			return solidlens.Scene{}, err
		}
		material := solidlens.Matte(layer.color)
		if layer.name == "hero-lights.stl" {
			material.Ambient = 1.1
		}
		model := solidlens.Model{Mesh: mesh, Material: material}
		edgeColor := lighten(layer.color, 0.2)
		edgeColor.A = 0.24
		model.Edges = solidlens.Edges{
			Enabled: true,
			Color:   edgeColor,
			Width:   0.7,
		}
		models = append(models, model)
	}
	scene := studioScene(models...)
	scene.Camera = heroCamera()
	scene.DirectionalLights = []solidlens.DirectionalLight{{
		Direction: solidlens.Vec{X: -0.8, Y: 0.45, Z: -1},
		Color:     solidlens.RGB(1, 0.9, 0.8),
		Intensity: 0.68,
	}}
	scene.PointLights = []solidlens.PointLight{{
		Position:  solidlens.Vec{X: 0.5, Y: -1.5, Z: 3.8},
		Color:     solidlens.RGB(1, 0.82, 0.5),
		Intensity: 10,
	}}
	scene.Background = solidlens.RGB(0.006, 0.018, 0.052)
	scene.Models = append(scene.Models,
		solidlens.Model{
			Mesh:     heroTitleMesh(scene.Camera, 56, 31, 6.01),
			Material: solidlens.Material{Color: solidlens.RGB(0.2, 0.34, 0.42), Ambient: 1},
		},
		solidlens.Model{
			Mesh:     heroTitleMesh(scene.Camera, 46, 21, 6),
			Material: solidlens.Material{Color: solidlens.RGB(0.78, 0.88, 1), Ambient: 1},
		},
	)
	return scene, nil
}

func heroCamera() solidlens.Camera {
	return solidlens.Camera{
		Position: solidlens.Vec{X: 8, Y: -11, Z: 9},
		Target:   solidlens.Vec{Z: 2},
		Up:       solidlens.Vec{Z: 1},
		FOV:      59,
	}
}

type heroScreenPlane struct {
	center, right, up    solidlens.Vec
	horizontal, vertical float64
}

func newHeroScreenPlane(camera solidlens.Camera, depth float64) heroScreenPlane {
	forward, _ := camera.Target.Sub(camera.Position).Normalize()
	right, _ := forward.Cross(camera.Up).Normalize()
	up, _ := right.Cross(forward).Normalize()
	focal := 1 / math.Tan(camera.FOV*math.Pi/360)
	return heroScreenPlane{
		center:     camera.Position.Add(forward.Scale(depth)),
		right:      right,
		up:         up,
		horizontal: depth * (1440.0 / 810.0) / focal,
		vertical:   depth / focal,
	}
}

func (p heroScreenPlane) point(x, y float64) solidlens.Vec {
	return p.center.
		Add(p.right.Scale((x/720 - 1) * p.horizontal)).
		Add(p.up.Scale((1 - y/405) * p.vertical))
}

func heroTitleMesh(camera solidlens.Camera, left, top, depth float64) *solidlens.Mesh {
	patterns := map[rune][]string{
		's': {"00000", "01111", "10000", "01110", "00001", "11110", "00000"},
		'o': {"00000", "01110", "10001", "10001", "10001", "01110", "00000"},
		'l': {"01000", "01000", "01000", "01000", "01000", "00110", "00000"},
		'i': {"00100", "00000", "01100", "00100", "00100", "01110", "00000"},
		'd': {"00001", "00001", "01101", "10011", "10001", "01101", "00000"},
		'e': {"00000", "01110", "10001", "11111", "10000", "01110", "00000"},
		'n': {"00000", "10110", "11001", "10001", "10001", "10001", "00000"},
	}
	plane := newHeroScreenPlane(camera, depth)
	b := newBuilder()
	const cell = 21.0
	const square = 19.8
	x := left
	for _, letter := range "solidlens" {
		glyph := patterns[letter]
		first, last := len(glyph[0]), -1
		for _, pixels := range glyph {
			for column, pixel := range pixels {
				if pixel == '1' {
					first = min(first, column)
					last = max(last, column)
				}
			}
		}
		for row, pixels := range glyph {
			for column, pixel := range pixels {
				if pixel != '1' {
					continue
				}
				px, py := x+float64(column-first)*cell, top+float64(row)*cell
				b.add([]solidlens.Vec{
					plane.point(px, py), plane.point(px+square, py),
					plane.point(px+square, py+square), plane.point(px, py+square),
				}, [][3]int{{0, 1, 2}, {0, 2, 3}})
			}
		}
		x += float64(last-first+2) * cell
	}
	return b.mesh()
}

func writeHeroModels() error {
	layers := heroLayers()
	buildHeroIsland(layers)
	for _, layer := range layers {
		if err := writeBinarySTL(layer.name, layer.builder.mesh()); err != nil {
			return err
		}
	}
	return nil
}

func buildHeroIsland(layers []heroLayer) {
	for x := -6; x <= 6; x++ {
		for y := -4; y <= 4; y++ {
			if !heroOnIsland(x, y) {
				continue
			}
			height := heroTerrainHeight(x, y)
			for z := -2; z < 0; z++ {
				if heroHasUnderside(x, y, z) {
					layers[heroStone].builder.box(
						solidlens.Vec{X: float64(x), Y: float64(y), Z: float64(z) + 0.5},
						solidlens.Vec{X: 1, Y: 1, Z: 1},
					)
				}
			}
			stoneTop := height - 0.72
			stone := heroStone
			if heroTerrainHash(x, y, 0)%5 == 0 {
				stone = heroStoneLight
			}
			layers[stone].builder.box(
				solidlens.Vec{X: float64(x), Y: float64(y), Z: stoneTop / 2},
				solidlens.Vec{X: 1, Y: 1, Z: stoneTop},
			)
			soil := heroSoil
			if heroTerrainHash(x, y, 1)%4 == 0 {
				soil = heroSoilLight
			}
			layers[soil].builder.box(
				solidlens.Vec{X: float64(x), Y: float64(y), Z: height - 0.36},
				solidlens.Vec{X: 1, Y: 1, Z: 0.72},
			)
			for dx := range 2 {
				for dy := range 2 {
					top := heroGroundColor(x, y, dx, dy)
					layers[top].builder.box(
						solidlens.Vec{
							X: float64(x) - 0.25 + float64(dx)*0.5,
							Y: float64(y) - 0.25 + float64(dy)*0.5,
							Z: height + 0.055,
						},
						solidlens.Vec{X: 0.5, Y: 0.5, Z: 0.11},
					)
				}
			}
		}
	}
	buildHeroTerrainDetails(layers)
	buildHeroWaterfall(layers[heroWater].builder)
	buildHeroBridge(layers[heroWood].builder, layers[heroLights].builder)
	buildHeroApproaches(layers[heroPath].builder, layers[heroPathLight].builder)
	buildHeroVillage(layers)
	buildHeroTrees(layers[heroWood].builder, layers[heroLeaves].builder)
	buildHeroSky(layers[heroLights].builder, layers[heroClouds].builder, heroCamera())
}

func heroOnIsland(x, y int) bool {
	fx, fy := float64(x)/6.5, float64(y)/4.6
	return fx*fx+fy*fy <= 1
}

func heroHasUnderside(x, y, z int) bool {
	if z >= 0 {
		return true
	}
	fx, fy := float64(x)/6.5, float64(y)/4.6
	radius := math.Sqrt(fx*fx + fy*fy)
	if z == -1 {
		return radius < 0.72
	}
	return radius < 0.46
}

func heroTerrainHeight(x, y int) float64 {
	if heroRiver(x, y) {
		return 1.75
	}
	if heroOnPath(x, y) || x >= -5 && x <= -4 && y >= -2 && y <= 0 {
		return 2
	}
	if x >= -4 && x <= -1 && y >= 1 && y <= 2 ||
		x >= 3 && x <= 4 && y >= 1 && y <= 2 {
		return 3
	}
	height := 2.0
	if y >= 2 {
		height += 0.25 * float64(y-1)
	}
	if x <= -5 && y >= 1 || x >= 5 && y >= 1 {
		height += 0.5
	}
	height += 0.25 * math.Sin(float64(x)*0.85+float64(y)*0.65)
	height += 0.18 * math.Cos(float64(x)*0.55-float64(y)*0.9)
	return math.Round(height*4) / 4
}

func heroRiver(x, y int) bool {
	return (x == 0 || x == 1) && y <= 2
}

func heroOnPath(x, y int) bool {
	return y == -2 && x >= -5 && x <= -3 ||
		y == -1 && (x >= -3 && x <= -2 || x >= 3 && x <= 4) ||
		y == 0 && (x >= -3 && x <= -2 || x >= 3 && x <= 4)
}

func heroTerrainHash(x, y, salt int) int {
	n := int64(x)*73856093 ^ int64(y)*19349663 ^ int64(salt)*83492791
	n ^= n >> 13
	return int(n & 0x7fffffff)
}

func heroGroundColor(x, y, dx, dy int) int {
	if heroRiver(x, y) {
		return heroWater
	}
	variation := heroTerrainHash(x*2+dx, y*2+dy, 3) % 10
	if heroOnPath(x, y) {
		if variation < 3 {
			return heroPathLight
		}
		return heroPath
	}
	if variation < 3 {
		return heroGrassLight
	}
	if variation == 9 {
		return heroGrassDark
	}
	return heroGrass
}

func buildHeroTerrainDetails(layers []heroLayer) {
	for x := -6; x <= 6; x++ {
		for y := -4; y <= 4; y++ {
			if !heroOnIsland(x, y) || heroRiver(x, y) || heroOnPath(x, y) {
				continue
			}
			if x >= -4 && x <= -2 && y >= 0 && y <= 2 ||
				x >= 3 && x <= 4 && y >= 0 && y <= 2 ||
				x >= -5 && x <= -4 && y >= -2 && y <= 0 {
				continue
			}
			ground := heroTerrainHeight(x, y)
			if (x*17+y*29)%3 == 0 {
				layers[heroLeaves].builder.box(
					solidlens.Vec{X: float64(x) + 0.28, Y: float64(y) - 0.24, Z: ground + 0.14},
					solidlens.Vec{X: 0.2, Y: 0.2, Z: 0.17},
				)
			}
			if (x*13+y*19)%9 == 0 {
				layers[heroStone].builder.box(
					solidlens.Vec{X: float64(x) - 0.24, Y: float64(y) + 0.18, Z: ground + 0.16},
					solidlens.Vec{X: 0.32, Y: 0.28, Z: 0.2},
				)
			}
			if (x*31+y*7)%11 == 0 {
				layers[heroRoof].builder.box(
					solidlens.Vec{X: float64(x) - 0.18, Y: float64(y) - 0.1, Z: ground + 0.16},
					solidlens.Vec{X: 0.11, Y: 0.11, Z: 0.19},
				)
			}
		}
	}
}

func buildHeroWaterfall(water *voxelBuilder) {
	for _, x := range []float64{0, 1} {
		water.box(solidlens.Vec{X: x, Y: -4.57, Z: -0.32},
			solidlens.Vec{X: 0.92, Y: 0.2, Z: 4.25})
	}
}

func buildHeroApproaches(path, light *voxelBuilder) {
	for _, entrance := range []struct {
		x, startY, landingY float64
	}{
		{-2.08, -0.75, 0.48},
		{3.5, -0.55, 0.64},
	} {
		for step := range 5 {
			y := entrance.startY + float64(step)*0.25
			top := 2.24 + float64(step)*0.22
			path.box(solidlens.Vec{X: entrance.x, Y: y, Z: (2.1 + top) / 2},
				solidlens.Vec{X: 0.94, Y: 0.26, Z: top - 2.1})
			light.box(solidlens.Vec{X: entrance.x, Y: y, Z: top + 0.035},
				solidlens.Vec{X: 0.84, Y: 0.22, Z: 0.07})
		}
		light.box(solidlens.Vec{X: entrance.x, Y: entrance.landingY, Z: 3.15},
			solidlens.Vec{X: 1.02, Y: 0.34, Z: 0.12})
	}
	light.box(solidlens.Vec{X: -4.35, Y: -2.04, Z: 2.15},
		solidlens.Vec{X: 0.65, Y: 0.42, Z: 0.1})
}

func buildHeroBridge(wood, lights *voxelBuilder) {
	wood.box(solidlens.Vec{X: 0.5, Y: -1, Z: 2.3}, solidlens.Vec{X: 4.15, Y: 1.1, Z: 0.22})
	for _, y := range []float64{-1.49, -0.51} {
		wood.box(solidlens.Vec{X: 0.5, Y: y, Z: 2.67}, solidlens.Vec{X: 4.15, Y: 0.1, Z: 0.11})
		for _, x := range []float64{-1.45, 0.5, 2.45} {
			wood.box(solidlens.Vec{X: x, Y: y, Z: 2.52}, solidlens.Vec{X: 0.12, Y: 0.12, Z: 0.65})
		}
	}
	for _, x := range []float64{-1.45, 2.45} {
		lights.box(solidlens.Vec{X: x, Y: -1.49, Z: 2.94}, solidlens.Vec{X: 0.22, Y: 0.22, Z: 0.25})
	}
	wood.box(solidlens.Vec{X: 0.5, Y: -1.49, Z: 3.16}, solidlens.Vec{X: 0.14, Y: 0.14, Z: 1.15})
	lights.box(solidlens.Vec{X: 0.5, Y: -1.49, Z: 3.8}, solidlens.Vec{X: 0.38, Y: 0.38, Z: 0.42})
	wood.box(solidlens.Vec{X: 0.5, Y: -1.49, Z: 4.06}, solidlens.Vec{X: 0.48, Y: 0.48, Z: 0.1})
}

func buildHeroVillage(layers []heroLayer) {
	stone, wood := layers[heroStone].builder, layers[heroWood].builder
	roof, lights := layers[heroRoof].builder, layers[heroLights].builder
	dark := layers[heroDarkWood].builder

	wood.box(solidlens.Vec{X: -2.5, Y: 1.5, Z: 3.85}, solidlens.Vec{X: 2.2, Y: 2, Z: 1.7})
	for step := range 6 {
		roof.box(solidlens.Vec{X: -2.5, Y: 1.5, Z: 4.7 + float64(step)*0.16},
			solidlens.Vec{X: 2.8 - float64(step)*0.38, Y: 2.45, Z: 0.16})
	}
	dark.box(solidlens.Vec{X: -2.08, Y: 0.43, Z: 3.48}, solidlens.Vec{X: 0.52, Y: 0.08, Z: 0.94})
	for _, x := range []float64{-3.48, -1.52} {
		dark.box(solidlens.Vec{X: x, Y: 0.43, Z: 3.83}, solidlens.Vec{X: 0.13, Y: 0.08, Z: 1.75})
	}
	dark.box(solidlens.Vec{X: -2.5, Y: 0.42, Z: 4.55}, solidlens.Vec{X: 2.05, Y: 0.1, Z: 0.1})
	dark.box(solidlens.Vec{X: -3.13, Y: 0.4, Z: 4.08}, solidlens.Vec{X: 0.66, Y: 0.1, Z: 0.68})
	lights.box(solidlens.Vec{X: -3.13, Y: 0.33, Z: 4.08}, solidlens.Vec{X: 0.48, Y: 0.08, Z: 0.5})
	dark.box(solidlens.Vec{X: -3.13, Y: 0.27, Z: 4.08}, solidlens.Vec{X: 0.06, Y: 0.07, Z: 0.5})
	dark.box(solidlens.Vec{X: -1.36, Y: 1.5, Z: 4.05}, solidlens.Vec{X: 0.1, Y: 0.65, Z: 0.65})
	lights.box(solidlens.Vec{X: -1.29, Y: 1.5, Z: 4.05}, solidlens.Vec{X: 0.08, Y: 0.48, Z: 0.48})
	dark.box(solidlens.Vec{X: -1.23, Y: 1.5, Z: 4.05}, solidlens.Vec{X: 0.07, Y: 0.06, Z: 0.48})
	roof.box(solidlens.Vec{X: -2.08, Y: 0.15, Z: 4.18}, solidlens.Vec{X: 0.9, Y: 0.72, Z: 0.13})
	stone.box(solidlens.Vec{X: -3.15, Y: 1.95, Z: 5.58}, solidlens.Vec{X: 0.35, Y: 0.35, Z: 0.85})

	wood.box(solidlens.Vec{X: -4.35, Y: -1.15, Z: 2.62}, solidlens.Vec{X: 1.45, Y: 1.45, Z: 1.25})
	for step := range 5 {
		roof.box(solidlens.Vec{X: -4.35, Y: -1.15, Z: 3.3 + float64(step)*0.15},
			solidlens.Vec{X: 1.9 - float64(step)*0.33, Y: 1.85, Z: 0.15})
	}
	dark.box(solidlens.Vec{X: -4.35, Y: -1.89, Z: 2.42}, solidlens.Vec{X: 0.32, Y: 0.08, Z: 0.58})
	dark.box(solidlens.Vec{X: -3.59, Y: -1.15, Z: 2.72}, solidlens.Vec{X: 0.09, Y: 0.5, Z: 0.5})
	lights.box(solidlens.Vec{X: -3.52, Y: -1.15, Z: 2.72}, solidlens.Vec{X: 0.07, Y: 0.34, Z: 0.34})
	roof.box(solidlens.Vec{X: -4.35, Y: -2.02, Z: 3.03}, solidlens.Vec{X: 0.7, Y: 0.42, Z: 0.1})

	stone.box(solidlens.Vec{X: 3.5, Y: 1.5, Z: 4.15}, solidlens.Vec{X: 1.55, Y: 1.55, Z: 2.3})
	for _, x := range []float64{2.8, 4.2} {
		for _, y := range []float64{0.8, 2.2} {
			stone.box(solidlens.Vec{X: x, Y: y, Z: 5.48}, solidlens.Vec{X: 0.42, Y: 0.42, Z: 0.42})
		}
	}
	roof.box(solidlens.Vec{X: 3.5, Y: 1.5, Z: 5.48}, solidlens.Vec{X: 1.65, Y: 1.65, Z: 0.16})
	dark.box(solidlens.Vec{X: 3.5, Y: 0.68, Z: 3.55}, solidlens.Vec{X: 0.47, Y: 0.09, Z: 0.82})
	stone.box(solidlens.Vec{X: 3.5, Y: 0.65, Z: 4.12}, solidlens.Vec{X: 0.7, Y: 0.17, Z: 0.15})
	dark.box(solidlens.Vec{X: 3.05, Y: 0.68, Z: 4.68}, solidlens.Vec{X: 0.12, Y: 0.1, Z: 0.62})
	dark.box(solidlens.Vec{X: 4.29, Y: 1.5, Z: 4.55}, solidlens.Vec{X: 0.08, Y: 0.13, Z: 0.55})
	stone.box(solidlens.Vec{X: 3.5, Y: 0.7, Z: 5.48}, solidlens.Vec{X: 0.32, Y: 0.38, Z: 0.42})
	wood.box(solidlens.Vec{X: 3.5, Y: 1.5, Z: 5.9}, solidlens.Vec{X: 0.1, Y: 0.1, Z: 0.9})
	roof.box(solidlens.Vec{X: 3.78, Y: 1.5, Z: 6.13}, solidlens.Vec{X: 0.57, Y: 0.09, Z: 0.32})
}

func buildHeroTrees(wood, leaves *voxelBuilder) {
	for _, tree := range []struct {
		x, y float64
	}{
		{-5, 1}, {-6, -1}, {-1, 3}, {2, 3}, {5, 2},
	} {
		ground := heroTerrainHeight(int(tree.x), int(tree.y))
		wood.box(solidlens.Vec{X: tree.x, Y: tree.y, Z: ground + 0.7},
			solidlens.Vec{X: 0.28, Y: 0.28, Z: 1.8})
		for step := range 5 {
			width := 1.8 - float64(step)*0.34
			leaves.box(solidlens.Vec{X: tree.x, Y: tree.y, Z: ground + 1.25 + float64(step)*0.36},
				solidlens.Vec{X: width, Y: width, Z: 0.42})
		}
	}
}

func buildHeroSky(lights, clouds *voxelBuilder, camera solidlens.Camera) {
	lights.box(solidlens.Vec{X: 5.5, Y: 6, Z: 7.2}, solidlens.Vec{X: 0.95, Y: 0.14, Z: 0.95})
	for _, star := range []solidlens.Vec{
		{X: -6, Y: 3, Z: 6.2}, {X: -3.5, Y: 4, Z: 6.6},
		{X: -0.5, Y: 5, Z: 6.8}, {X: 2.5, Y: 4.5, Z: 6.5},
		{X: 5.8, Y: 4, Z: 6.1}, {X: 6.6, Y: -2, Z: 4.2},
	} {
		lights.box(star, solidlens.Vec{X: 0.15, Y: 0.15, Z: 0.15})
	}
	for _, star := range []solidlens.Vec{
		{X: -7.31, Y: -8.13, Z: 4.86}, {X: -4.54, Y: -6.54, Z: 4.19},
		{X: -2.67, Y: -7.26, Z: 0.93}, {X: -5.26, Y: -10.09, Z: -0.56},
		{X: 7.83, Y: 2.88, Z: 4.86}, {X: 10.24, Y: 3.83, Z: 3.6},
		{X: 8.66, Y: 0.6, Z: 0.33}, {X: 11.13, Y: 1.74, Z: -0.71},
	} {
		lights.box(star, solidlens.Vec{X: 0.11, Y: 0.11, Z: 0.11})
	}
	original := newVoxelBuilder()
	addHeroCloud(original, solidlens.Vec{X: 8, Y: 5, Z: 8}, 1)
	clouds.add(original.vertices, original.triangles)
	copyHeroCloud(clouds, original, camera, -1113, 421)
	copyHeroCloud(clouds, original, camera, -8, 451)
}

func copyHeroCloud(dst, original *voxelBuilder, camera solidlens.Camera, dx, dy float64) {
	forward, _ := camera.Target.Sub(camera.Position).Normalize()
	right, _ := forward.Cross(camera.Up).Normalize()
	up, _ := right.Cross(forward).Normalize()
	focal := 1 / math.Tan(camera.FOV*math.Pi/360)
	vertices := make([]solidlens.Vec, len(original.vertices))
	for i, vertex := range original.vertices {
		depth := vertex.Sub(camera.Position).Dot(forward)
		shift := depth / (405 * focal)
		vertices[i] = vertex.Add(right.Scale(dx * shift)).Add(up.Scale(-dy * shift))
	}
	dst.add(vertices, original.triangles)
}

func addHeroCloud(clouds *voxelBuilder, center solidlens.Vec, scale float64) {
	clouds.box(center, solidlens.Vec{X: 1.9 * scale, Y: 0.5 * scale, Z: 0.35 * scale})
	clouds.box(center.Add(solidlens.Vec{X: -0.7 * scale, Z: -0.25 * scale}),
		solidlens.Vec{X: 0.95 * scale, Y: 0.55 * scale, Z: 0.35 * scale})
	clouds.box(center.Add(solidlens.Vec{X: 0.55 * scale, Z: 0.22 * scale}),
		solidlens.Vec{X: 0.95 * scale, Y: 0.55 * scale, Z: 0.35 * scale})
}
