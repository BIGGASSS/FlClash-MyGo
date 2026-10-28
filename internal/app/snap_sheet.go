package app

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// The source uses a 9/16 initial detent, a 90% upper detent and UIKit's
// projected release. These coordinates exclude the desktop caption.
type snapSheetState struct {
	Stop                            int
	Height                          springValue
	Pressed                         bool
	PressY, PressHeight, Top, Width float32
	Samples                         []pointerSample
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

func (a *App) resetSheet(key string) { delete(a.snapSheets, key) }

func (s *snapSheetState) height(c *ui.Context, available, maximum float32) float32 {
	target := available * 9 / 16
	if s.Stop == 1 {
		target = maximum
	}
	if s.Height.At.IsZero() {
		s.Height.jump(c.Now(), float64(target))
	}
	if !s.Pressed && s.Height.Target != float64(target) {
		s.Height.to(c.Now(), float64(target), springSpec{math.Sqrt(1000.0 / 3), 1})
	}
	height := s.Height.frame(c)
	s.Top = available - height
	return height
}

func (s *snapSheetState) input(e ui.InputEvent, available, maximum float32, close func()) bool {
	y := e.Y + s.Top
	switch e.Kind {
	case ui.InputPointerDown:
		if e.Button != 0 || e.Y > 68 || e.Y > 10 && (e.X < 72 || e.X > s.Width-72) {
			return false
		}
		s.Pressed = true
		s.PressY, s.PressHeight = y, float32(s.Height.Value)
		s.Height.Velocity = 0
		s.Samples = []pointerSample{{X: float64(y), At: e.Time}}
		return true
	case ui.InputPointerMove:
		if !s.Pressed {
			return false
		}
		height := s.PressHeight - (y - s.PressY)
		if height > maximum {
			height = maximum + float32(sheetRubberBand(float64(height-maximum), float64(min(100, max(0, available-14-maximum)))))
		}
		s.Height.jump(e.Time, float64(height))
		s.Samples = append(s.Samples, pointerSample{X: float64(y), At: e.Time})
		if len(s.Samples) > 20 {
			s.Samples = s.Samples[len(s.Samples)-20:]
		}
		return true
	case ui.InputPointerUp:
		if !s.Pressed {
			return false
		}
		s.Pressed = false
		velocity := -pointerVelocity(s.Samples, e.Time)
		projected := s.Height.Value + velocity*.099
		stops := []float64{0, float64(available * 9 / 16), float64(maximum)}
		target := 0
		for i := range stops {
			if math.Abs(stops[i]-projected) < math.Abs(stops[target]-projected) {
				target = i
			}
		}
		if target == 0 {
			close()
			return true
		}
		s.Stop = target - 1
		launched, ratio := sheetLaunch(stops[target]-s.Height.Value, velocity)
		s.Height.Velocity = launched
		s.Height.to(e.Time, stops[target], springSpec{math.Sqrt(1000.0 / 3), ratio})
		return true
	}
	return false
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
	reach := math.Sqrt(1000.0/3) * math.Abs(distance)
	strength := math.Abs(velocity) / reach
	return math.Copysign(min(strength, .75)*reach, velocity), 1 - .2*min(strength/.3, 1)
}
