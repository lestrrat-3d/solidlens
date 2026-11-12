package solidlens

import (
	"context"
	"fmt"
	"image"
	"math"
)

// minDashPeriod is the smallest total length of a dash pattern in pixels. It
// bounds the number of dashes a line can be split into.
const minDashPeriod = 0.25

// dashTolerance absorbs rounding when deciding whether a dash starts or stops
// exactly at a segment's end.
const dashTolerance = 1e-9

// lineStyle is a Line's validated width and dash pattern.
type lineStyle struct {
	width float64
	// pattern alternates dash and gap lengths, starting with a dash, and always
	// has an even length. It is empty for a solid line.
	pattern []float64
	// period is the sum of pattern.
	period float64
}

func (l Line) style() (lineStyle, error) {
	if len(l.Points) < 2 {
		return lineStyle{}, fmt.Errorf("a line needs at least two points")
	}
	for index, point := range l.Points {
		if !finiteVec(point) {
			return lineStyle{}, fmt.Errorf("point %d is not finite", index)
		}
	}
	if !finiteColor(l.Color) {
		return lineStyle{}, fmt.Errorf("color is not finite")
	}
	style := lineStyle{width: l.Width}
	if style.width == 0 {
		style.width = 1
	}
	if !finite(style.width) || style.width < 0 {
		return lineStyle{}, fmt.Errorf("width must be positive")
	}
	if len(l.Dash) == 0 {
		return style, nil
	}
	style.pattern = append(style.pattern, l.Dash...)
	if len(style.pattern)%2 == 1 {
		style.pattern = append(style.pattern, l.Dash...)
	}
	for _, length := range style.pattern {
		if !finite(length) || length <= 0 {
			return lineStyle{}, fmt.Errorf("dash lengths must be positive")
		}
		style.period += length
	}
	if !finite(style.period) || style.period < minDashPeriod {
		return lineStyle{}, fmt.Errorf("dash pattern must add up to at least %g pixels", minDashPeriod)
	}
	return style, nil
}

// drawLines draws every line over image. A line is gathered into a coverage
// buffer and composited in one pass, so a pixel that several of its dashes or
// segments touch is blended once, at the strongest coverage.
func drawLines(ctx context.Context, image *image.RGBA, v view, lines []Line) error {
	if len(lines) == 0 {
		return nil
	}
	styles := make([]lineStyle, len(lines))
	for index, line := range lines {
		style, err := line.style()
		if err != nil {
			return fmt.Errorf("solidlens: line %d: %w", index, err)
		}
		styles[index] = style
	}
	canvas := newEdgeCanvas(v.width, v.height)
	for index, line := range lines {
		if err := ctx.Err(); err != nil {
			return err
		}
		canvas.reset()
		drawLine(canvas, v, line.Points, styles[index])
		canvas.composite(image, line.Color)
	}
	return nil
}

// drawLine accumulates the coverage of one polyline. arc is the distance in
// screen pixels already walked along the polyline, which is what the dash
// pattern is measured against.
func drawLine(canvas *edgeCanvas, v view, points []Vec, style lineStyle) {
	arc := 0.0
	for index := 0; index+1 < len(points); index++ {
		a, b, ok := v.clipSegment(points[index], points[index+1])
		if !ok {
			continue
		}
		start, end := v.projectPoint(a), v.projectPoint(b)
		length := math.Hypot(end.x-start.x, end.y-start.y)
		if !finite(length) || length == 0 {
			continue
		}
		// Only the part of the segment near the screen is walked, so a
		// segment that runs far off screen does not cost a dash per period.
		t0, t1, visible := visibleRange(v, start, end, style.width)
		if visible {
			segment := lineSegment{start: start, end: end, arc: arc, length: length}
			first, last := arc+t0*length, arc+t1*length
			if len(style.pattern) == 0 {
				accumulateLine(canvas, nil, segment.point(first), segment.point(last), style.width)
			} else {
				segment.dashes(canvas, style, first, last)
			}
		}
		arc += length
	}
}

// lineSegment is one projected polyline segment and where it sits along the
// polyline, measured in pixels.
type lineSegment struct {
	start, end screenVertex
	arc        float64
	length     float64
}

// point returns the point at distance along from the segment's start, in the
// same units as arc.
func (s lineSegment) point(distance float64) screenVertex {
	t := (distance - s.arc) / s.length
	return screenVertex{
		x: s.start.x + (s.end.x-s.start.x)*t,
		y: s.start.y + (s.end.y-s.start.y)*t,
		z: s.start.z + (s.end.z-s.start.z)*t,
	}
}

// dashes accumulates the dashes of style that fall between the arc positions
// first and last, which lie within the segment.
func (s lineSegment) dashes(canvas *edgeCanvas, style lineStyle, first, last float64) {
	half := style.width / 2
	segmentEnd := s.arc + s.length
	for cycle := math.Floor(first / style.period); cycle*style.period <= last; cycle++ {
		offset := cycle * style.period
		for index := 0; index < len(style.pattern); index += 2 {
			dashStart := offset
			dashEnd := dashStart + style.pattern[index]
			offset = dashEnd + style.pattern[index+1]
			if dashEnd < first || dashStart > last {
				continue
			}
			// A dash that stops where this segment starts, or starts where it
			// stops, belongs to the neighbouring segment.
			if dashEnd <= s.arc+dashTolerance || dashStart >= segmentEnd-dashTolerance {
				continue
			}
			from, to := math.Max(dashStart, first), math.Min(dashEnd, last)
			// A round cap reaches half the width past its end. Pull in the ends
			// where the dash really starts or stops, so that it covers its
			// stated length. A dash that carries on into the next segment keeps
			// its end, and the cap there serves as the corner join.
			if dashStart >= s.arc-dashTolerance {
				from += half
			}
			if dashEnd <= segmentEnd+dashTolerance {
				to -= half
			}
			if from > to {
				from = (from + to) / 2
				to = from
			}
			accumulateLine(canvas, nil, s.point(from), s.point(to), style.width)
		}
	}
}

// visibleRange returns the range of the segment from start to end, as
// fractions from zero to one of its length, that lies within a margin around
// the screen. It reports false when no part of the segment does.
func visibleRange(v view, start, end screenVertex, width float64) (float64, float64, bool) {
	margin := width + 2
	t0, t1 := 0.0, 1.0
	clip := func(delta, distance float64) bool {
		// The segment keeps the part where distance + t*delta is not negative.
		if delta == 0 {
			return distance >= 0
		}
		t := -distance / delta
		if delta > 0 {
			t0 = math.Max(t0, t)
		} else {
			t1 = math.Min(t1, t)
		}
		return t0 <= t1
	}
	deltaX, deltaY := end.x-start.x, end.y-start.y
	if !clip(deltaX, start.x+margin) ||
		!clip(-deltaX, float64(v.width)+margin-start.x) ||
		!clip(deltaY, start.y+margin) ||
		!clip(-deltaY, float64(v.height)+margin-start.y) {
		return 0, 0, false
	}
	return t0, t1, true
}
