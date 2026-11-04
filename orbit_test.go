package solidlens_test

import (
	"math"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

func TestOrbitCameraPlacesCameraOnSphere(t *testing.T) {
	orbit := solidlens.Orbit{Target: solidlens.Vec{X: 1, Y: 2, Z: 3}, Distance: 10}

	// Yaw 0 and pitch 0 look from the positive X axis, level with the target.
	camera := orbit.Camera()
	require.Equal(t, orbit.Target, camera.Target)
	require.Equal(t, solidlens.Vec{Z: 1}, camera.Up)
	require.InDelta(t, 11, camera.Position.X, 1e-9)
	require.InDelta(t, 2, camera.Position.Y, 1e-9)
	require.InDelta(t, 3, camera.Position.Z, 1e-9)

	for _, view := range [][2]float64{{90, 0}, {200, 35}, {-35, -60}, {0, 89}} {
		orbit.Yaw, orbit.Pitch = view[0], view[1]
		camera = orbit.Camera()
		require.InDelta(t, 10, camera.Position.Sub(camera.Target).Len(), 1e-9, "yaw %g pitch %g", view[0], view[1])
	}

	orbit.Yaw, orbit.Pitch = 90, 0
	require.InDelta(t, 12, orbit.Camera().Position.Y, 1e-9, "yaw 90 looks from the positive Y axis")
	orbit.Yaw, orbit.Pitch = 0, 90
	require.Greater(t, orbit.Camera().Position.Z, 3.0, "positive pitch lifts the camera")
}

func TestOrbitCameraHoldsPitchShortOfThePole(t *testing.T) {
	orbit := solidlens.Orbit{Distance: 10, Pitch: 500}
	camera := orbit.Camera()
	// Straight above, the camera's direction would match its up vector.
	require.Greater(t, math.Hypot(camera.Position.X, camera.Position.Y), 1e-3)
	orbit.Pitch = -500
	camera = orbit.Camera()
	require.Greater(t, math.Hypot(camera.Position.X, camera.Position.Y), 1e-3)
}

func TestOrbitCameraRejectedWithoutDistance(t *testing.T) {
	scene := pyramidScene(t)
	scene.Camera = solidlens.Orbit{Target: solidlens.Vec{Z: 1}}.Camera()
	_, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: 8, Height: 8})
	require.Error(t, err)
}

func TestFitOrbitFramesBounds(t *testing.T) {
	b, err := solidlens.BoundsOf(pyramidMesh(t))
	require.NoError(t, err)

	orbit := solidlens.FitOrbit(b, 0)
	require.Equal(t, b.Center(), orbit.Target)
	require.Equal(t, solidlens.DefaultOrbitYaw, orbit.Yaw)
	require.Equal(t, solidlens.DefaultOrbitPitch, orbit.Pitch)
	require.InDelta(t, 45, orbit.FOV, 0, "zero FOV means the Camera default")
	// The bounding sphere must fit inside the view cone.
	require.Greater(t, orbit.Distance*math.Sin(orbit.FOV*math.Pi/360), b.Radius())

	narrow := solidlens.FitOrbit(b, 20)
	require.Greater(t, narrow.Distance, orbit.Distance, "a narrower view needs more distance")

	// A single point still gets a usable distance.
	point, err := solidlens.BoundsOf(pointSource{})
	require.NoError(t, err)
	require.Positive(t, solidlens.FitOrbit(point, 0).Distance)
}

func TestFitOrbitKeepsModelInFrame(t *testing.T) {
	scene := pyramidScene(t)
	b, err := solidlens.BoundsOf(scene.Models[0].Mesh)
	require.NoError(t, err)
	orbit := solidlens.FitOrbit(b, 0)
	scene.Camera = orbit.Camera()
	scene.DirectionalLights = solidlens.CameraLights(scene.Camera)
	const w, h = 64, 48
	img, err := solidlens.Render(t.Context(), scene, solidlens.Settings{Width: w, Height: h})
	require.NoError(t, err)

	bg := scene.Background.NRGBA()
	covered, edge := 0, 0
	for y := range h {
		for x := range w {
			p := img.Pix[(y*w+x)*4:]
			if p[0] == bg.R && p[1] == bg.G && p[2] == bg.B {
				continue
			}
			covered++
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				edge++
			}
		}
	}
	require.Positive(t, covered, "the model should cover some pixels")
	require.Zero(t, edge, "the model should not touch the frame edge")
}

func TestCameraLightsFollowCamera(t *testing.T) {
	for _, view := range [][2]float64{{0, 0}, {90, 20}, {200, -30}, {-35, 25}} {
		camera := solidlens.Orbit{Distance: 5, Yaw: view[0], Pitch: view[1]}.Camera()
		lights := solidlens.CameraLights(camera)
		require.Len(t, lights, 2)
		forward := camera.Target.Sub(camera.Position)
		for i, light := range lights {
			// Every light must shine onto the side the camera sees.
			require.Positive(t, light.Direction.Dot(forward), "view %v light %d", view, i)
		}
		// The key light comes from above the camera, so it shines downward
		// relative to the camera's view when the camera is level.
		if view[1] == 0 {
			require.Negative(t, lights[0].Direction.Z, "key light should shine down")
		}
	}
}

func TestCameraLightsNeedAUsableCamera(t *testing.T) {
	require.Nil(t, solidlens.CameraLights(solidlens.Camera{}), "no direction")
	straightDown := solidlens.Camera{Position: solidlens.Vec{Z: 5}, Up: solidlens.Vec{Z: 1}}
	require.Nil(t, solidlens.CameraLights(straightDown), "view along up")
}
