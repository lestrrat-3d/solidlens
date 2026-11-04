package solidlens

import (
	"context"
	"fmt"
	"image"
	"image/gif"
	"io"
	"runtime"
	"sync"
	"time"
)

const (
	gifDefaultWidth  = 640
	gifDefaultHeight = 480
	gifDefaultDelay  = 40 * time.Millisecond
	gifMaxEdge       = 4096
	gifMaxFrames     = 720
	gifMaxPixels     = 1 << 29 // frames * width * height, one byte each
	gifPaletteFrames = 12      // frames sampled to choose the palette
)

// GIFOptions sets the size and speed of RenderGIF's output. Every field is
// optional.
type GIFOptions struct {
	// Width and Height are the size of each frame in pixels. They default to
	// 640 by 480 and are limited to 4096.
	Width, Height int
	// Delay is how long each frame shows. It defaults to 40 milliseconds. A
	// GIF keeps time in hundredths of a second, so it is rounded to that and
	// cannot be shorter than 10 milliseconds.
	Delay time.Duration
}

func (o GIFOptions) normalized(frames int) (GIFOptions, error) {
	if o.Width == 0 {
		o.Width = gifDefaultWidth
	}
	if o.Height == 0 {
		o.Height = gifDefaultHeight
	}
	if o.Delay == 0 {
		o.Delay = gifDefaultDelay
	}
	switch {
	case o.Width < 1 || o.Height < 1 || o.Width > gifMaxEdge || o.Height > gifMaxEdge:
		return GIFOptions{}, fmt.Errorf("solidlens: GIF size must be between 1 and %d pixels", gifMaxEdge)
	case frames < 1 || frames > gifMaxFrames:
		return GIFOptions{}, fmt.Errorf("solidlens: GIF needs between 1 and %d frames, got %d", gifMaxFrames, frames)
	case o.Width*o.Height*frames > gifMaxPixels:
		return GIFOptions{}, fmt.Errorf("solidlens: GIF is too large: %d pixels in all", o.Width*o.Height*frames)
	case o.Delay < 0:
		return GIFOptions{}, fmt.Errorf("solidlens: GIF delay must not be negative")
	}
	return o, nil
}

// hundredths returns the delay in the hundredths of a second a GIF stores.
func (o GIFOptions) hundredths() int {
	return max(1, int((o.Delay+5*time.Millisecond)/(10*time.Millisecond)))
}

// RenderGIF renders every scene of src and writes them as a GIF that loops
// forever. A GIF is the animated format a GitHub README shows inline.
//
// Frames are drawn on all CPUs. A GIF holds 256 colours, so the palette is
// chosen from a sample of the frames and used for all of them. Flat-shaded
// models keep their look, but a smooth gradient may show bands.
//
// Rendering stops with ctx's error when ctx is cancelled, and a scene that
// fails to render stops it with that error. Nothing is written to w unless
// every frame is drawn.
func RenderGIF(ctx context.Context, w io.Writer, src SceneSource, opts GIFOptions) error {
	frames := src.Len()
	opts, err := opts.normalized(frames)
	if err != nil {
		return err
	}
	settings := Settings{Width: opts.Width, Height: opts.Height}

	// Pass one draws a few evenly spaced frames to choose the palette. Pass
	// two draws every frame again and maps it to that palette as soon as it
	// is done, so only one byte per pixel is kept for each frame.
	sample := min(frames, gifPaletteFrames)
	samples := make([]*image.RGBA, sample)
	err = forEachFrame(ctx, sample, func(i int) error {
		img, err := Render(ctx, src.Scene(i*frames/sample), settings)
		samples[i] = img
		return err
	})
	if err != nil {
		return err
	}
	palette := buildPalette(samples)
	samples = nil
	table := newPaletteTable(palette)

	anim := &gif.GIF{
		Image:     make([]*image.Paletted, frames),
		Delay:     make([]int, frames),
		LoopCount: 0, // forever
	}
	delay := opts.hundredths()
	err = forEachFrame(ctx, frames, func(i int) error {
		img, err := Render(ctx, src.Scene(i), settings)
		if err != nil {
			return err
		}
		frame := image.NewPaletted(img.Bounds(), palette)
		for p := range frame.Pix {
			frame.Pix[p] = table.index(img.Pix[4*p], img.Pix[4*p+1], img.Pix[4*p+2])
		}
		anim.Image[i] = frame
		anim.Delay[i] = delay
		return nil
	})
	if err != nil {
		return err
	}
	return gif.EncodeAll(w, anim)
}

// forEachFrame calls fn for 0 through n-1 on up to one goroutine per CPU. It
// stops handing out frames after the first error and returns that error, or
// ctx's error when ctx ends first.
func forEachFrame(ctx context.Context, n int, fn func(i int) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		once     sync.Once
		firstErr error
		wg       sync.WaitGroup
		next     = make(chan int)
	)
	fail := func(err error) {
		once.Do(func() { firstErr = err; cancel() })
	}
	for range min(n, runtime.NumCPU()) {
		wg.Go(func() {
			for i := range next {
				if err := fn(i); err != nil {
					fail(err)
				}
			}
		})
	}
feed:
	for i := range n {
		select {
		case next <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(next)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}
