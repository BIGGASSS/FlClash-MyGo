package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

const dashboardMorphDuration = 450 * time.Millisecond
const dashboardStateDuration = 150 * time.Millisecond

// dashboardCurve follows Flutter Cubic's 0.001 input tolerance as well as the
// source control points. A higher precision solver subtly changes the exported
// intermediate glyphs. See dashboard_morph.md for provenance and the BSD notice.
func dashboardCurve(t float64, reverse bool) float64 {
	if t <= 0 || t >= 1 {
		return min(max(t, 0), 1)
	}
	x1, y1, x2, y2 := .175, .885, .32, 1.275 // easeOutBack
	if reverse {
		x1, y1, x2, y2 = .55, .055, .675, .19 // easeInCubic
	}
	cubic := func(a, b, u float64) float64 {
		return 3*a*(1-u)*(1-u)*u + 3*b*(1-u)*u*u + u*u*u
	}
	lo, hi := 0.0, 1.0
	for {
		u := (lo + hi) / 2
		x := cubic(x1, x2, u)
		if math.Abs(t-x) < .001 {
			return cubic(y1, y2, u)
		}
		if x < t {
			lo = u
		} else {
			hi = u
		}
	}
}

type dashboardMorphState struct {
	initialized, running, suspended   bool
	controller, controllerFrom        float64
	glyph, glyphFrom                  float64
	at, glyphAt                       time.Time
	controllerDuration, glyphDuration time.Duration
	// CurvedAnimation retains the selected curve through mid-flight reversals.
	moving, reverseCurve bool
	press                dashboardPressState
}

func (s *dashboardMorphState) step(now time.Time) {
	if s.moving {
		delta := float64(now.Sub(s.at)) / float64(s.controllerDuration)
		if !s.running {
			delta = -delta
		}
		value := s.controllerFrom + delta
		s.controller = min(max(value, 0), 1)
		// Flutter's simulation completes strictly AFTER its end time. A
		// reversal at exactly 450 ms still retains the previous curve.
		s.moving = value >= 0 && value <= 1
	}
	if !s.suspended {
		target := float64(0)
		if s.running {
			target = 1
		}
		t := min(max(float64(now.Sub(s.glyphAt))/float64(s.glyphDuration), 0), 1)
		s.glyph = s.glyphFrom + (target-s.glyphFrom)*dashboardCurve(t, false)
	}
}

func (s *dashboardMorphState) frame(c *ui.Context, running, suspended bool) (float32, float32) {
	now := c.Now()
	target := float64(0)
	if running {
		target = 1
	}
	duration := dashboardMorphDuration
	if c.Preferences().ReduceMotion {
		// AnimationController's normal accessibility behavior compresses the
		// duration by 0.05. Neither source morph tween uses motionDuration.
		duration /= 20
	}
	if !s.initialized {
		s.initialized, s.running, s.suspended = true, running, suspended
		s.controller, s.controllerFrom, s.glyph, s.glyphFrom = target, target, target, target
		s.at, s.glyphAt = now, now
		s.controllerDuration, s.glyphDuration = duration, duration
	}
	s.step(now)
	if s.running != running {
		if !s.moving {
			s.reverseCurve = !running
		}
		s.running, s.moving = running, true
		s.controllerFrom, s.at, s.controllerDuration = s.controller, now, duration
		s.glyphFrom, s.glyphAt, s.glyphDuration = s.glyph, now, duration
	}
	if s.suspended && !suspended {
		// The source removes the glyph tween while suspended. Reintroducing it
		// mounts at the current target, not the old hidden animation value.
		s.glyph, s.glyphFrom, s.glyphAt = target, target, now
	}
	s.suspended = suspended
	if s.moving || (!suspended && now.Sub(s.glyphAt) < s.glyphDuration && s.glyphFrom != target) {
		c.AnimationFrame()
	}
	return float32(dashboardCurve(s.controller, s.reverseCurve)), float32(s.glyph)
}

// The source ElasticPress has no hover swell here (no ElasticHover ancestor).
// Pointer transforms paint only; the button and tooltip keep their resting box.
type dashboardPressState struct {
	lift, x, y       springValue
	pressed, release bool
	originX, originY float32
}

func (s *dashboardPressState) input(e ui.InputEvent, side float32) {
	switch e.Kind {
	case ui.InputPointerDown:
		if e.Button == 0 && !s.pressed {
			s.pressed, s.release = true, false
			s.originX, s.originY = e.X, e.Y
			s.lift.to(e.Time, 1, dockPress)
		}
	case ui.InputPointerMove:
		if s.pressed {
			limit := float64(side) * 7 / 32
			s.x.to(e.Time, rubberBand(float64(e.X-s.originX), limit), durationSpring(.12, 0))
			s.y.to(e.Time, rubberBand(float64(e.Y-s.originY), limit), durationSpring(.12, 0))
		}
	case ui.InputPointerUp:
		if e.Button == 0 && s.pressed {
			s.pressed, s.release = false, true
			s.x.to(e.Time, 0, dockPress)
			s.y.to(e.Time, 0, dockPress)
		}
	}
}

func (s *dashboardPressState) frame(c *ui.Context, side float32, enabled bool) (sx, sy, x, y float32) {
	if !enabled {
		s.lift.jump(c.Now(), 0)
		s.x.jump(c.Now(), 0)
		s.y.jump(c.Now(), 0)
		s.release = false
		s.pressed = false
	}
	// Source ElasticPress uses a ticker and SpringSimulation directly, not
	// AnimationController: accessibility duration scaling does not affect it.
	frame := func(spring *springValue) float32 {
		spring.step(c.Now())
		if spring.Value != spring.Target || spring.Velocity != 0 {
			c.AnimationFrame()
		}
		return float32(spring.Value)
	}
	lift := frame(&s.lift)
	if s.release && lift >= .975 {
		s.release = false
		s.lift.to(c.Now(), 0, dockPress)
		c.AnimationFrame()
	}
	x, y = frame(&s.x), frame(&s.y)
	swell := 1 + lift*min(float32(.25), 16/side)
	return swell * (1 + float32(math.Abs(float64(x)))/side*.5), swell * (1 + float32(math.Abs(float64(y)))/side*.5), x, y
}

// Rounded GlyphPolyline from AppGlyphs.playPause, on its original 24-unit grid.
// Fold is clamped, but turn is not: overshoot continues rotating at both ends.
func dashboardPlayPauseVertices(progress float64) [][][2]float64 {
	fold := min(max(progress, 0), 1)
	halves := [][][2]float64{{{8.3, 5.6}, {18.8, 12}, {18.8, 12}, {8.3, 12}}, {{8.3, 12}, {18.8, 12}, {18.8, 12}, {8.3, 18.4}}}
	bars := [][][2]float64{{{5.9, 6.9}, {18.1, 6.9}, {18.1, 9.1}, {5.9, 9.1}}, {{5.9, 14.9}, {18.1, 14.9}, {18.1, 17.1}, {5.9, 17.1}}}
	if fold == 0 {
		halves = [][][2]float64{{{8.3, 5.6}, {18.8, 12}, {8.3, 18.4}}}
	}
	sin, cos := math.Sincos(math.Pi / 2 * progress)
	for h := range halves {
		for i, point := range halves[h] {
			x, y := point[0], point[1]
			if fold > 0 {
				x += (bars[h][i][0] - x) * fold
				y += (bars[h][i][1] - y) * fold
			}
			x, y = x-12, y-12
			halves[h][i] = [2]float64{12 + x*cos - y*sin, 12 + x*sin + y*cos}
		}
	}
	return halves
}

func dashboardPlayPausePaths(r ui.Rect, progress float32) []*ui.Path {
	vertices := dashboardPlayPauseVertices(float64(progress))
	paths := make([]*ui.Path, 0, len(vertices))
	for _, points := range vertices {
		corner := func(i int) (enter, exit [2]float64) {
			p := points[i]
			near := [2][2]float64{points[(i+len(points)-1)%len(points)], points[(i+1)%len(points)]}
			out := [2][2]float64{}
			for j, q := range near {
				dx, dy := q[0]-p[0], q[1]-p[1]
				d := math.Hypot(dx, dy)
				out[j] = p
				if d > 0 {
					f := min(1.2, d/2) / d
					out[j] = [2]float64{p[0] + dx*f, p[1] + dy*f}
				}
			}
			return out[0], out[1]
		}
		point := func(p [2]float64) (float32, float32) {
			return r.X + float32(p[0])*r.W/24, r.Y + float32(p[1])*r.H/24
		}
		path := new(ui.Path).Join(ui.RoundJoin).Cap(ui.RoundCap)
		_, first := corner(0)
		x, y := point(first)
		path.MoveTo(x, y)
		for i := 1; i <= len(points); i++ {
			index := i % len(points)
			enter, exit := corner(index)
			x, y := point(enter)
			path.LineTo(x, y)
			cx, cy := point(points[index])
			x, y = point(exit)
			path.QuadTo(cx, cy, x, y)
		}
		paths = append(paths, path.Close())
	}
	return paths
}

func paintDashboardPlayPause(p *ui.Painter, r ui.Rect, progress float32, color ui.Color) {
	for _, path := range dashboardPlayPausePaths(r, progress) {
		p.FillPath(path, color)
		p.StrokePath(path, 1.8*r.W/24, color)
	}
}

func dashboardOverlay(enabled, focused, hovered bool) float32 {
	if enabled {
		if focused {
			return .1
		}
		if hovered {
			return .08
		}
	}
	return 0
}

func dashboardFocusPath(r ui.Rect, progress float32) *ui.Path {
	// BorderSide(width:2, strokeAlign:2), strokeUnitShape scales about center.
	r.X, r.Y, r.W, r.H = r.X-2, r.Y-2, r.W+4, r.H+4
	return dashboardShape("start", r, progress, progress*40)
}
