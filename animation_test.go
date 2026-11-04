package solidlens_test

import (
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

func TestScenesPlaysInOrder(t *testing.T) {
	a, b := pyramidScene(t), pyramidScene(t)
	a.Background, b.Background = solidlens.RGB(1, 0, 0), solidlens.RGB(0, 1, 0)
	var src solidlens.SceneSource = solidlens.Scenes{a, b}
	require.Equal(t, 2, src.Len())
	require.Equal(t, a.Background, src.Scene(0).Background)
	require.Equal(t, b.Background, src.Scene(1).Background)
}

func newTurntable(t *testing.T) solidlens.Turntable {
	t.Helper()
	base := pyramidScene(t)
	b, err := solidlens.BoundsOf(base.Models[0].Mesh)
	require.NoError(t, err)
	return solidlens.Turntable{Base: base, Orbit: solidlens.FitOrbit(b, 0), Frames: 8}
}

func TestTurntableTurnsOnceAndLoops(t *testing.T) {
	turntable := newTurntable(t)
	require.Equal(t, 8, turntable.Len())

	first := turntable.Scene(0).Camera
	require.Equal(t, turntable.Orbit.Camera(), first, "the first frame is the orbit's own view")

	// Every frame keeps its distance and height, and yaw moves by a constant
	// step, so the camera circles the target.
	radius := first.Position.Sub(first.Target).Len()
	for i := 1; i < 8; i++ {
		camera := turntable.Scene(i).Camera
		require.InDelta(t, radius, camera.Position.Sub(camera.Target).Len(), 1e-9, "frame %d", i)
		require.InDelta(t, first.Position.Z, camera.Position.Z, 1e-9, "frame %d", i)
		require.NotEqual(t, first.Position, camera.Position, "frame %d", i)
	}
	// One more step would land back on frame 0, so the loop has no stutter.
	wrapped := turntable.Orbit
	wrapped.Yaw += 360
	got, want := wrapped.Camera().Position, first.Position
	require.InDelta(t, want.X, got.X, 1e-9)
	require.InDelta(t, want.Y, got.Y, 1e-9)
}

func TestTurntableDefaultFrames(t *testing.T) {
	turntable := newTurntable(t)
	turntable.Frames = 0
	require.Equal(t, solidlens.DefaultTurntableFrames, turntable.Len())
}

func TestTurntableLighting(t *testing.T) {
	turntable := newTurntable(t)

	// With no lights in Base, lights follow the camera.
	require.NotEmpty(t, turntable.Scene(0).DirectionalLights)
	require.NotEqual(t, turntable.Scene(0).DirectionalLights[0].Direction, turntable.Scene(3).DirectionalLights[0].Direction)

	// With a light in Base, it stays where the caller put it.
	fixed := solidlens.DirectionalLight{Direction: solidlens.Vec{Z: -1}, Color: solidlens.RGB(1, 1, 1), Intensity: 1}
	turntable.Base.DirectionalLights = []solidlens.DirectionalLight{fixed}
	for i := range 8 {
		require.Equal(t, []solidlens.DirectionalLight{fixed}, turntable.Scene(i).DirectionalLights)
	}

	// A point light alone also counts, so the caller is not given the
	// camera lights on top of it.
	turntable.Base.DirectionalLights = nil
	turntable.Base.PointLights = []solidlens.PointLight{{Position: solidlens.Vec{Z: 5}, Color: solidlens.RGB(1, 1, 1), Intensity: 10}}
	require.Empty(t, turntable.Scene(0).DirectionalLights)
}

func TestTurntableDoesNotChangeBase(t *testing.T) {
	turntable := newTurntable(t)
	before := turntable.Base
	for i := range turntable.Len() {
		_ = turntable.Scene(i)
	}
	require.Empty(t, before.DirectionalLights)
	require.Empty(t, turntable.Base.DirectionalLights, "building frames must not light the caller's scene")
	require.Equal(t, before.Camera, turntable.Base.Camera)
}
