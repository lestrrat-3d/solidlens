package solidlens

import "math"

const (
	orbitMaxPitch   = 89.0 // degrees; keeps the camera off the pole
	orbitFramingPad = 1.15
	defaultFOV      = 45.0 // matches Camera
)

// Orbit is a camera on a sphere around a point, looking at that point with Z
// up. It is a plain value: a viewer that rotates on drag changes Yaw and
// Pitch, one that zooms scales Distance, and either calls Camera to get
// the camera to put in a Scene.
//
// Yaw turns about the Z axis and Pitch lifts the camera above the XY plane,
// both in degrees. Yaw 0 and Pitch 0 look from the positive X axis. Pitch is
// held within ±89 degrees so the view never flips over the pole.
type Orbit struct {
	// Target is the point the camera looks at.
	Target Vec
	// Distance is how far the camera sits from Target.
	Distance float64
	// Yaw and Pitch place the camera on the sphere, in degrees.
	Yaw, Pitch float64
	// FOV is the vertical field of view in degrees. It defaults to 45, like
	// Camera.
	FOV float64
}

// FitOrbit returns an Orbit that looks at the middle of b from far enough away
// that all of b fits in the frame, from the DefaultOrbitYaw and
// DefaultOrbitPitch angles. A zero fov means 45 degrees.
func FitOrbit(b Bounds, fov float64) Orbit {
	if fov == 0 {
		fov = defaultFOV
	}
	radius := b.Radius()
	if radius == 0 || !finite(radius) {
		radius = 1
	}
	half := clamp(fov, 1, 178) * math.Pi / 360
	return Orbit{
		Target: b.Center(),
		// A sphere of this radius fits the frame in both directions when
		// the narrower one is the vertical field of view.
		Distance: orbitFramingPad * radius / math.Sin(half),
		Yaw:      DefaultOrbitYaw,
		Pitch:    DefaultOrbitPitch,
		FOV:      fov,
	}
}

// DefaultOrbitYaw and DefaultOrbitPitch, in degrees, give a three-quarter view
// from above, a good first look at a model.
const (
	DefaultOrbitYaw   = -35.0
	DefaultOrbitPitch = 25.0
)

// Camera returns the camera at the orbit's position. A Distance that is not
// positive gives a camera with no direction, which Render rejects.
func (o Orbit) Camera() Camera {
	pitch := clamp(o.Pitch, -orbitMaxPitch, orbitMaxPitch) * math.Pi / 180
	yaw := o.Yaw * math.Pi / 180
	offset := Vec{
		X: math.Cos(pitch) * math.Cos(yaw),
		Y: math.Cos(pitch) * math.Sin(yaw),
		Z: math.Sin(pitch),
	}
	return Camera{
		Position: o.Target.Add(offset.Scale(o.Distance)),
		Target:   o.Target,
		Up:       Vec{Z: 1},
		FOV:      o.FOV,
		// Keeps the near plane in proportion, so zooming in on a small
		// part does not clip it.
		Near: o.Distance / 1000,
	}
}

// CameraLights returns a key light and a dimmer fill light placed relative to
// camera, so the side being looked at stays lit however the camera moves. Use
// it as Scene.DirectionalLights for a camera that changes, or with any other
// camera for a quick even lighting. The camera's Up must not point along its
// view.
func CameraLights(camera Camera) []DirectionalLight {
	forward := camera.Target.Sub(camera.Position)
	if forward.Len() == 0 {
		return nil
	}
	forward = forward.Scale(1 / forward.Len())
	up := camera.Up
	if up == (Vec{}) {
		up = Vec{Z: 1}
	}
	right := forward.Cross(up)
	if right.Len() == 0 {
		return nil
	}
	right = right.Scale(1 / right.Len())
	above := right.Cross(forward) // the camera's own up
	// A light's direction points away from the light. The key light sits
	// above and to the left of the camera, so it shines down and to the
	// right. The fill sits to the right and a little below.
	key := forward.Add(right.Scale(0.5)).Sub(above.Scale(0.6))
	fill := forward.Sub(right.Scale(0.8)).Add(above.Scale(0.3))
	return []DirectionalLight{
		{Direction: key, Color: RGB(1, 1, 1), Intensity: 1},
		{Direction: fill, Color: RGB(0.7, 0.8, 1), Intensity: 0.35},
	}
}
