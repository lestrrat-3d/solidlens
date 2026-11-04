package solidlens_test

import (
	"bytes"
	"context"
	"errors"
	"image/gif"
	"testing"
	"time"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

func renderGIF(t *testing.T, src solidlens.SceneSource, opts solidlens.GIFOptions) *gif.GIF {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, solidlens.RenderGIF(t.Context(), &out, src, opts))
	anim, err := gif.DecodeAll(&out)
	require.NoError(t, err)
	return anim
}

func TestRenderGIFShape(t *testing.T) {
	anim := renderGIF(t, newTurntable(t), solidlens.GIFOptions{Width: 48, Height: 32, Delay: 50 * time.Millisecond})
	require.Len(t, anim.Image, 8)
	require.Equal(t, 0, anim.LoopCount, "zero loops forever")
	for i, frame := range anim.Image {
		require.Equal(t, 48, frame.Bounds().Dx(), "frame %d width", i)
		require.Equal(t, 32, frame.Bounds().Dy(), "frame %d height", i)
		require.Equal(t, 5, anim.Delay[i], "frame %d delay in hundredths", i)
	}
	require.NotEqual(t, anim.Image[0].Pix, anim.Image[4].Pix, "the model turns, so frames differ")
}

func TestRenderGIFDefaults(t *testing.T) {
	turntable := newTurntable(t)
	turntable.Frames = 2
	anim := renderGIF(t, turntable, solidlens.GIFOptions{})
	require.Equal(t, 640, anim.Image[0].Bounds().Dx())
	require.Equal(t, 480, anim.Image[0].Bounds().Dy())
	require.Equal(t, 4, anim.Delay[0])
}

func TestRenderGIFShortDelayStaysVisible(t *testing.T) {
	anim := renderGIF(t, newTurntable(t), solidlens.GIFOptions{Width: 16, Height: 16, Delay: time.Millisecond})
	require.Equal(t, 1, anim.Delay[0], "a GIF cannot show a frame for less than one hundredth of a second")
}

func TestRenderGIFKeepsColours(t *testing.T) {
	anim := renderGIF(t, newTurntable(t), solidlens.GIFOptions{Width: 64, Height: 48})
	blue := false
	for _, c := range anim.Image[0].Palette {
		r, g, b, _ := c.RGBA()
		if b > 2*r && b > g {
			blue = true
		}
	}
	require.True(t, blue, "palette lost the model's colour")
}

func TestRenderGIFAcceptsAnySceneSource(t *testing.T) {
	// A hand-built list of scenes animates the same way, here a camera that
	// moves in close.
	base := newTurntable(t)
	var scenes solidlens.Scenes
	for _, distance := range []float64{12, 8, 5} {
		orbit := base.Orbit
		orbit.Distance = distance
		scene := base.Base
		scene.Camera = orbit.Camera()
		scene.DirectionalLights = solidlens.CameraLights(scene.Camera)
		scenes = append(scenes, scene)
	}
	anim := renderGIF(t, scenes, solidlens.GIFOptions{Width: 32, Height: 32})
	require.Len(t, anim.Image, 3)
}

func TestRenderGIFRejectsBadOptions(t *testing.T) {
	turntable := newTurntable(t)
	for name, tc := range map[string]struct {
		src  solidlens.SceneSource
		opts solidlens.GIFOptions
	}{
		"no frames":      {solidlens.Scenes{}, solidlens.GIFOptions{}},
		"too many":       {func() solidlens.SceneSource { tt := turntable; tt.Frames = 721; return tt }(), solidlens.GIFOptions{}},
		"negative size":  {turntable, solidlens.GIFOptions{Width: -1}},
		"huge edge":      {turntable, solidlens.GIFOptions{Width: 5000}},
		"too many pixel": {func() solidlens.SceneSource { tt := turntable; tt.Frames = 600; return tt }(), solidlens.GIFOptions{Width: 4096, Height: 4096}},
		"negative delay": {turntable, solidlens.GIFOptions{Delay: -time.Second}},
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			require.Error(t, solidlens.RenderGIF(t.Context(), &out, tc.src, tc.opts))
			require.Zero(t, out.Len(), "nothing should be written on failure")
		})
	}
}

func TestRenderGIFReportsSceneError(t *testing.T) {
	turntable := newTurntable(t)
	turntable.Orbit.Distance = 0 // a camera with no direction
	var out bytes.Buffer
	require.Error(t, solidlens.RenderGIF(t.Context(), &out, turntable, solidlens.GIFOptions{Width: 16, Height: 16}))
	require.Zero(t, out.Len())
}

func TestRenderGIFStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var out bytes.Buffer
	err := solidlens.RenderGIF(ctx, &out, newTurntable(t), solidlens.GIFOptions{Width: 32, Height: 32})
	require.True(t, errors.Is(err, context.Canceled), "got %v", err)
	require.Zero(t, out.Len())
}
