package solidlens

// SceneSource is a sequence of scenes, one per frame of an animation. Anything
// that can name its frames can be animated this way: a camera that circles a
// model, a cutting plane that sweeps through it, or a list of scenes built by
// hand.
//
// Scene may be called for several indexes at once, so it must not change the
// source. It is called with 0 through Len()-1.
type SceneSource interface {
	// Len returns the number of frames.
	Len() int
	// Scene returns the scene for frame i.
	Scene(i int) Scene
}

// Scenes is a SceneSource that plays a list of scenes in order.
type Scenes []Scene

// Len implements SceneSource.
func (s Scenes) Len() int { return len(s) }

// Scene implements SceneSource.
func (s Scenes) Scene(i int) Scene { return s[i] }

// DefaultTurntableFrames is the frame count of a Turntable that sets none.
const DefaultTurntableFrames = 72

// Turntable is a SceneSource in which the camera makes one full turn around
// its target, which is the animation a model is usually shown with. The
// camera is Orbit, with Yaw growing by 360/Frames degrees from frame to
// frame, so the first frame is the view Orbit gives and the animation loops
// seamlessly.
//
// Base supplies everything else: models, lights, and background. Its Camera
// is ignored. When Base has no lights at all, each frame is lit with
// CameraLights, so the lighting follows the camera. Set any light to keep the
// lights fixed in the world and let the shading change as the model turns.
type Turntable struct {
	Base   Scene
	Orbit  Orbit
	Frames int // defaults to DefaultTurntableFrames
}

// Len implements SceneSource.
func (t Turntable) Len() int {
	if t.Frames == 0 {
		return DefaultTurntableFrames
	}
	return t.Frames
}

// Scene implements SceneSource.
func (t Turntable) Scene(i int) Scene {
	orbit := t.Orbit
	orbit.Yaw += 360 * float64(i) / float64(t.Len())
	scene := t.Base
	scene.Camera = orbit.Camera()
	if len(scene.DirectionalLights) == 0 && len(scene.PointLights) == 0 {
		scene.DirectionalLights = CameraLights(scene.Camera)
	}
	return scene
}
