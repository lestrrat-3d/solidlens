package main

import (
	"context"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/lestrrat-3d/solidlens"
)

// sectionPlane is the z = 0 plane with U along X and V along Y.
func sectionPlane() solidlens.SectionPlane {
	return solidlens.SectionPlane{U: solidlens.Vec{X: 1}, V: solidlens.Vec{Y: 1}}
}

// toothedWheel returns a closed outline of a wheel with square-ish teeth,
// counter-clockwise, centred at (cx, cy).
func toothedWheel(cx, cy, root, tip float64, teeth int, phase float64) []solidlens.Vec {
	const perTooth = 8
	loop := make([]solidlens.Vec, 0, teeth*perTooth)
	for i := range teeth * perTooth {
		angle := phase + 2*math.Pi*float64(i)/float64(teeth*perTooth)
		radius := root
		if i%perTooth < perTooth/2 {
			radius = tip
		}
		loop = append(loop, solidlens.Vec{X: cx + radius*math.Cos(angle), Y: cy + radius*math.Sin(angle)})
	}
	return loop
}

func circleLoop(cx, cy, radius float64, clockwise bool) []solidlens.Vec {
	const n = 64
	loop := make([]solidlens.Vec, n)
	for i := range n {
		angle := 2 * math.Pi * float64(i) / n
		if clockwise {
			angle = -angle
		}
		loop[i] = solidlens.Vec{X: cx + radius*math.Cos(angle), Y: cy + radius*math.Sin(angle)}
	}
	return loop
}

func writeSection() error {
	view := solidlens.SectionView{
		Plane: sectionPlane(),
		Layers: []solidlens.SectionLayer{
			{
				Name: "gear", Fill: solidlens.RGBA(0.05, 0.4, 0.95, 0.55), Outline: solidlens.RGB(0.02, 0.2, 0.7),
				Loops: [][]solidlens.Vec{toothedWheel(0, 0, 10, 12, 10, 0), circleLoop(0, 0, 3, true)},
			},
			{
				Name: "mate", Fill: solidlens.RGBA(0.95, 0.6, 0.05, 0.55), Outline: solidlens.RGB(0.7, 0.35, 0.02),
				Loops: [][]solidlens.Vec{toothedWheel(21.5, 0, 10, 12, 10, math.Pi/10)},
			},
		},
		Width:   1200,
		Height:  800,
		Caption: "Two toothed wheels meeting on the z = 0 plane",
	}
	img, _, err := solidlens.RenderSection(context.Background(), view)
	if err != nil {
		return err
	}
	return writePNG(filepath.Join(outputDir, "section.png"), img)
}

func writePNG(path string, img image.Image) (err error) {
	file, err := os.Create(path) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	return png.Encode(file, img)
}
