package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

var sheetOmega = math.Sqrt(1000.0 / 3)

// Flutter precisionErrorTolerance and precise-pointer recognizer defaults.
// MyGo's desktop InputEvent does not expose a touch/stylus device kind.
const (
	sheetPrecision        = 1e-10
	sheetPointerSlop      = 1
	sheetMinFlingVelocity = 50
	sheetMaxFlingVelocity = 8000
)

// Coordinates exclude the desktop caption. Detents, spring tolerances, launch
// limits and the wheel-turn gap follow the pinned lib/ui/snap_sheet.dart.
type snapSheetState struct {
	Stop                            int
	Height                          springValue
	Pressed                         bool
	PressY, PressHeight, Top, Width float32
	Minimum, Maximum                float32
	Samples                         []pointerSample
	Focus                           ui.Handle

	Mounted, Compact, Reduced, Dismissing bool
	DismissRequested, Committing          bool
	Close                                 func()
	Presentation                          springValue
	Controller                            float64
	Progress, Barrier                     float32
	SideFrom, SideTarget, ClosingVelocity float64
	SideStarted                           time.Time

	Scroll    *ui.ScrollState
	Scrolls   map[ui.Handle]*ui.ScrollState
	LastWheel time.Time
	WheelHeld bool

	// Content drags latch the scroller and its starting edge, not a native
	// element. Controls/selection keep their own pointer sequence in MyGo.
	DragScroll                              *ui.ScrollState
	ContentDrag, DragStarted, StartedAtEdge bool
	SheetHeld, SheetTouched                 bool
	LastY                                   float32
	LiftFollows                             bool
	Lift                                    float32
}

func (a *App) snapSheet(key string) *snapSheetState {
	if a.snapSheets == nil {
		a.snapSheets = map[string]*snapSheetState{}
	}
	s := a.snapSheets[key]
	if s == nil {
		s = &snapSheetState{}
		a.snapSheets[key] = s
	}
	return s
}

func (a *App) resetSheet(key string) {
	if s := a.snapSheets[key]; s != nil {
		s.clearInput()
		s.Close = nil
	}
	delete(a.snapSheets, key)
}

func sheetSpringFrame(c *ui.Context, spring *springValue, distance, velocity float64) float32 {
	if c.Preferences().ReduceMotion {
		spring.jump(c.Now(), spring.Target)
	} else {
		spring.step(c.Now())
		if math.Abs(spring.Value-spring.Target) < distance && math.Abs(spring.Velocity) < velocity {
			spring.jump(c.Now(), spring.Target)
		}
	}
	if spring.Value != spring.Target || spring.Velocity != 0 {
		c.AnimationFrame()
	}
	return float32(spring.Value)
}

func (s *snapSheetState) height(c *ui.Context, available, maximum float32) float32 {
	minimum := available * 9 / 16
	refit := s.Minimum != minimum || s.Maximum != maximum
	s.Minimum, s.Maximum = minimum, maximum
	target := s.Minimum
	if s.Stop == 1 {
		target = maximum
	}
	if s.Height.At.IsZero() {
		s.Height.jump(c.Now(), float64(target))
	}
	if !s.Pressed && !s.Dismissing && s.Height.Target != float64(target) {
		s.Height.step(c.Now())
		if refit && s.Height.Value == s.Height.Target && s.Height.Velocity == 0 {
			// _Resting resolves its detent on every layout, including content
			// changes at the upper stop. Do not leave a blank old-size viewport.
			s.Height.jump(c.Now(), float64(target))
		} else {
			velocity, ratio := sheetLaunch(float64(target)-s.Height.Value, s.Height.Velocity)
			s.Height.Velocity = velocity
			s.Height.to(c.Now(), float64(target), springSpec{sheetOmega, ratio})
		}
	}
	height := sheetSpringFrame(c, &s.Height, .5, 10)
	// The input's local Y already includes the presentation translation;
	// add it back to obtain viewport coordinates stable across drag builds.
	s.Top = available - height
	if s.Mounted {
		s.Top += (1 - s.Progress) * max(available*9/16, height)
	}
	return height
}

func (s *snapSheetState) clearInput() {
	s.Pressed = false
	s.Samples = nil
	s.WheelHeld = false
	s.DragScroll = nil
	s.ContentDrag, s.DragStarted, s.StartedAtEdge = false, false, false
	s.SheetHeld, s.SheetTouched = false, false
}

func (s *snapSheetState) input(e ui.InputEvent, available, maximum float32, close func()) bool {
	y := e.Y + s.Top
	switch e.Kind {
	case ui.InputScroll:
		return s.wheel(e, available, maximum, e.Y > 68 && s.Scroll != nil)
	case ui.InputPointerDown:
		if e.Button != 0 || e.Y > 68 || s.Pressed {
			return false
		}
		s.hold(e.Time, available)
		s.Pressed = true
		s.PressY, s.PressHeight = y, float32(s.Height.Value)
		s.Samples = []pointerSample{{X: float64(y), At: e.Time}}
		return true
	case ui.InputPointerMove:
		if !s.Pressed {
			return false
		}
		if s.ContentDrag {
			return s.moveContent(e.Time, y, available, maximum)
		}
		height := max(0, s.PressHeight-(y-s.PressY))
		if height > maximum {
			height = maximum + float32(sheetRubberBand(float64(height-maximum), float64(min(100, max(0, available-14-maximum)))))
		}
		s.Height.jump(e.Time, float64(height))
		s.LiftFollows = s.LiftFollows || height < available*9/16
		s.sample(e.Time, y)
		return true
	case ui.InputPointerUp:
		if !s.Pressed || e.Button != 0 {
			return false
		}
		velocity := -pointerVelocity(s.Samples, e.Time)
		if s.ContentDrag {
			if !s.SheetTouched {
				s.clearInput()
				return true
			}
			if !s.SheetHeld || math.Abs(velocity) < sheetMinFlingVelocity {
				velocity = 0
			}
			velocity = min(max(velocity, -sheetMaxFlingVelocity), sheetMaxFlingVelocity)
		}
		s.release(e.Time, available, maximum, velocity, close)
		return true
	}
	return false
}

func (s *snapSheetState) hold(now time.Time, available float32) {
	s.Height.step(now)
	if s.Mounted && s.Compact && s.Presentation.Value < 1 {
		// Start at the painted position if presentation was interrupted.
		s.Height.jump(now, max(0, s.Height.Value-(1-s.Presentation.Value)*max(float64(available*9/16), s.Height.Value)))
		s.Presentation.jump(now, 1)
	}
	s.Height.jump(now, s.Height.Value)
}

func (s *snapSheetState) sample(now time.Time, y float32) {
	s.Samples = append(s.Samples, pointerSample{X: float64(y), At: now})
	if len(s.Samples) > 20 {
		s.Samples = s.Samples[len(s.Samples)-20:]
	}
}

// Handle bounds are committed geometry, safe to query between builds. In
// particular, a header/tab wheel must not borrow the list's scrolled offset.
func (s *snapSheetState) inScroll(c *ui.Context, e ui.InputEvent) bool {
	if s.Scroll == nil {
		return false
	}
	pane, scroll := s.Focus.Bounds(c), s.Scroll.Bounds(c)
	x, y := pane.X+e.X, pane.Y+e.Y
	return scroll.W > 0 && scroll.H > 0 && x >= scroll.X && x < scroll.X+scroll.W && y >= scroll.Y && y < scroll.Y+scroll.H
}

func (s *snapSheetState) startContent(e ui.InputEvent) bool {
	if e.Button != 0 || s.Pressed || s.Scroll == nil {
		return false
	}
	s.Pressed, s.ContentDrag = true, true
	s.DragScroll = s.Scroll
	s.StartedAtEdge = s.Scroll.Y <= sheetPrecision
	s.PressY, s.LastY = e.Y+s.Top, e.Y+s.Top
	s.Samples = []pointerSample{{X: float64(s.PressY), At: e.Time}}
	return true
}

// _SheetContentDrag: up first grows the sheet and sends the remainder to
// content. Down may take the sheet only when the gesture STARTED at the top;
// a content-owned drag reaching zero does not suddenly pull the route down.
func (s *snapSheetState) moveContent(now time.Time, y, available, maximum float32) bool {
	s.sample(now, y)
	if !s.DragStarted {
		// Flutter's VerticalDragGestureRecognizer uses the precise-pointer
		// hit slop (1 DIP) on desktop, and DragStartBehavior.start drops the
		// accepting move. MyGo does not expose pointer-device kind here.
		if math.Abs(float64(y-s.PressY)) <= sheetPointerSlop {
			return true
		}
		s.DragStarted, s.LastY = true, y
		return true
	}
	grow := s.LastY - y
	s.LastY = y
	if grow == 0 {
		return true
	}
	scroll := s.DragScroll
	scrolled := scroll.Y > sheetPrecision
	open := s.Height.Value >= float64(maximum)-sheetPrecision || !s.SheetTouched && s.Stop == 1
	if !s.SheetHeld && (grow > 0 && (scrolled || open) || grow < 0 && (!s.StartedAtEdge || scrolled)) {
		sheetScrollBy(scroll, grow)
		return true
	}
	if !s.SheetTouched {
		s.hold(now, available)
		s.SheetTouched = true
	}
	s.SheetHeld = true
	height := float32(s.Height.Value)
	rest := float32(0)
	if grow > 0 {
		taken := min(grow, max(0, maximum-height))
		height += taken
		rest = grow - taken
	} else {
		height = max(0, height+grow)
	}
	s.Height.jump(now, float64(height))
	s.LiftFollows = s.LiftFollows || height < available*9/16
	if rest > sheetPrecision {
		s.SheetHeld = false
		sheetScrollBy(scroll, rest)
	}
	return true
}

func sheetScrollBy(scroll *ui.ScrollState, delta float32) {
	// A pointer follows directly, not ScrollTo's default spring. Replacing
	// numeric state also drops an outstanding programmatic ScrollTo while
	// preserving the scroller's stable native handle and measured extents.
	*scroll = ui.ScrollState{
		Handle: scroll.Handle, X: scroll.X, Y: min(max(scroll.Y+delta, 0), scroll.MaxY),
		MaxX: scroll.MaxX, MaxY: scroll.MaxY,
	}
}

// Flutter's onVerticalDragCancel calls dragEnd(0). In particular, a blur must
// not turn the last few fast moves into an unintended dismissal fling.
func (s *snapSheetState) cancel(now time.Time, available, maximum float32, close func()) bool {
	if !s.Pressed {
		return false
	}
	if s.ContentDrag && !s.SheetTouched {
		s.clearInput()
		return true
	}
	s.release(now, available, maximum, 0, close)
	return true
}

func (s *snapSheetState) release(now time.Time, available, maximum float32, velocity float64, close func()) {
	s.clearInput()
	projected := s.Height.Value + velocity*.099
	minimum := float64(available * 9 / 16)
	stops := []float64{0, minimum, float64(maximum)}
	target := 0
	for i := range stops {
		if math.Abs(stops[i]-projected) < math.Abs(stops[target]-projected) {
			target = i
		}
	}
	below := s.Height.Value < minimum && minimum > 0 && s.Mounted
	if below {
		// Below the first detent the route, not the content height, moves.
		// This keeps both the barrier and the exit continuous on release.
		s.Presentation.jump(now, max(0, s.Height.Value/minimum))
		s.Controller = s.Presentation.Value
		s.Height.jump(now, minimum)
	}
	if target == 0 {
		if minimum > 0 {
			s.ClosingVelocity = velocity / minimum
		}
		// _close restores presentation while the owner's dismissal guard
		// decides. An accepted close reverses it at the following build.
		if below {
			s.present(now, 1)
		} else {
			s.Stop = 0
			s.Height.to(now, minimum, springSpec{sheetOmega, 1})
		}
		close()
		return
	}
	if below {
		s.Stop = 0
		s.Presentation.Velocity = velocity / minimum
		s.present(now, 1)
		return
	}
	s.Stop = target - 1
	launched, ratio := sheetLaunch(stops[target]-s.Height.Value, velocity)
	s.Height.Velocity = launched
	s.Height.to(now, stops[target], springSpec{sheetOmega, ratio})
}

// One turn may choose only one adjacent detent, and a turn that starts in
// scrolled content stays there even if it subsequently reaches the top. Positive
// DY grows the sheet, just as _SheetScrollPosition.pointerScroll in the source.
func (s *snapSheetState) wheel(e ui.InputEvent, available, maximum float32, content bool) bool {
	if e.DY == 0 || e.Mods&(ui.Ctrl|ui.Cmd|ui.Shift) != 0 || s.Pressed {
		return false
	}
	continued := !s.LastWheel.IsZero() && e.Time.Sub(s.LastWheel) < 500*time.Millisecond
	s.LastWheel = e.Time
	if continued {
		return s.WheelHeld
	}
	s.WheelHeld = false
	if content && s.Scroll != nil && s.Scroll.Y > sheetPrecision {
		return false
	}
	// At the upper detent a forward turn belongs to the list, including
	// while that detent is still settling. Wheel input never dismisses.
	if content && e.DY > 0 && s.Stop == 1 {
		return false
	}
	target := available * 9 / 16
	if e.DY > 0 {
		s.Stop, target = 1, maximum
	} else {
		s.Stop = 0
	}
	s.Height.step(e.Time)
	s.Height.Velocity = 0 // settleTowards starts a new, unthrown spring
	s.Height.to(e.Time, float64(target), springSpec{sheetOmega, 1})
	s.WheelHeld = true
	return true
}

func sheetRubberBand(distance, extent float64) float64 {
	if distance <= 0 || extent <= 0 {
		return 0
	}
	return extent * (1 - 1/(distance*.55/extent+1))
}

// Translated from the pinned SnapSheet launch limiter: discard throws away
// from the chosen stop and cap throws toward it before applying damping.
func sheetLaunch(distance, velocity float64) (float64, float64) {
	if distance == 0 || velocity == 0 || math.Signbit(distance) != math.Signbit(velocity) {
		return 0, 1
	}
	reach := sheetOmega * math.Abs(distance)
	strength := math.Abs(velocity) / reach
	return math.Copysign(min(strength, .75)*reach, velocity), 1 - .2*min(strength/.3, 1)
}
