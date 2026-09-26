package solidlens_test

import (
	"image"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

type cacheTriangleSource struct {
	vertices  []solidlens.Vec
	triangles [][3]int
}

func (s cacheTriangleSource) Vertices() []solidlens.Vec { return s.vertices }
func (s cacheTriangleSource) Triangles() [][3]int       { return s.triangles }

func cacheTestScene(t *testing.T) (solidlens.Scene, *solidlens.Mesh, cacheTriangleSource) {
	t.Helper()
	vertices := []solidlens.Vec{
		{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1},
	}
	triangles := [][3]int{{0, 1, 2}, {0, 2, 3}}
	mesh, err := solidlens.NewMesh(vertices, triangles)
	require.NoError(t, err)
	edges := solidlens.Outline(solidlens.RGB(0, 0, 0))
	edges.CreaseAngle = -1
	scene := solidlens.Scene{
		Camera: solidlens.Camera{
			Position: solidlens.Vec{Z: 3}, Up: solidlens.Vec{Y: 1}, FOV: 60,
		},
		Models: []solidlens.Model{{
			Material: solidlens.Material{Color: solidlens.RGB(0.8, 0.3, 0.1), Ambient: 0.3},
			Edges:    edges,
		}},
		DirectionalLights: []solidlens.DirectionalLight{{
			Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 0.7,
		}},
		Background: solidlens.RGB(0.1, 0.2, 0.3),
	}
	return scene, mesh, cacheTriangleSource{vertices: vertices, triangles: triangles}
}

func TestRenderMeshCacheTracksSceneChanges(t *testing.T) {
	scene, mesh, source := cacheTestScene(t)
	settings := solidlens.Settings{Width: 64, Height: 64}
	check := func(scene solidlens.Scene, settings solidlens.Settings) {
		t.Helper()
		meshScene := scene
		meshScene.Models = append([]solidlens.Model(nil), scene.Models...)
		meshScene.Models[0].Mesh = mesh
		got, err := solidlens.Render(t.Context(), meshScene, settings)
		require.NoError(t, err)
		sourceScene := scene
		sourceScene.Models = append([]solidlens.Model(nil), scene.Models...)
		sourceScene.Models[0].Mesh = source
		want, err := solidlens.Render(t.Context(), sourceScene, settings)
		require.NoError(t, err)
		require.Equal(t, want.Pix, got.Pix)
	}

	check(scene, settings)
	check(scene, settings)
	withBackground := scene
	withBackground.Background = solidlens.RGB(0.7, 0.4, 0.2)
	check(withBackground, settings)
	withMaterial := scene
	withMaterial.Models = append([]solidlens.Model(nil), scene.Models...)
	withMaterial.Models[0].Material = solidlens.Matte(solidlens.RGB(0.2, 0.6, 0.9))
	check(withMaterial, settings)
	withLights := scene
	withLights.DirectionalLights = []solidlens.DirectionalLight{{
		Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 0.1,
	}}
	check(withLights, settings)
	withPoints := scene
	withPoints.PointLights = []solidlens.PointLight{{
		Position: solidlens.Vec{Z: 3}, Color: solidlens.RGB(1, 1, 1), Intensity: 5,
	}}
	check(withPoints, settings)
	withCamera := scene
	withCamera.Camera.Position.Z = 4
	check(withCamera, settings)
	withBack := scene
	withBack.Camera.Position.Z = -3
	withBack.Models = append([]solidlens.Model(nil), scene.Models...)
	back := solidlens.Matte(solidlens.RGB(0.1, 0.7, 0.9))
	withBack.Models[0].BackMaterial = &back
	check(withBack, settings)
	back = solidlens.Matte(solidlens.RGB(0.9, 0.1, 0.5))
	check(withBack, settings)
	withEdges := scene
	withEdges.Models = append([]solidlens.Model(nil), scene.Models...)
	withEdges.Models[0].Edges.Width = 2
	withEdges.Models[0].Edges.CreaseAngle = 30
	check(withEdges, settings)
	check(scene, solidlens.Settings{Width: 48, Height: 48})
	check(scene, settings)
}

func TestRenderMeshCacheConcurrent(t *testing.T) {
	scene, mesh, source := cacheTestScene(t)
	settings := solidlens.Settings{Width: 64, Height: 64}
	scene.Models[0].Mesh = source
	want, err := solidlens.Render(t.Context(), scene, settings)
	require.NoError(t, err)
	scene.Models[0].Mesh = mesh
	ctx := t.Context()
	type result struct {
		image *image.RGBA
		err   error
	}
	results := make(chan result, 8)
	for range 8 {
		go func() {
			image, err := solidlens.Render(ctx, scene, settings)
			results <- result{image: image, err: err}
		}()
	}
	for range 8 {
		got := <-results
		require.NoError(t, got.err)
		require.Equal(t, want.Pix, got.image.Pix)
	}
}
