package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

type springSpec struct{ Frequency, Ratio float64 }

func durationSpring(seconds, bounce float64) springSpec {
	ratio := 1 / (1 + bounce)
	if bounce > 0 {
		ratio = 1 - bounce
	}
	return springSpec{2 * math.Pi / seconds, ratio}
}

type springValue struct {
	Value, Velocity, Target float64
	Spec                    springSpec
	At                      time.Time
}

func (s *springValue) step(now time.Time) {
	if s.At.IsZero() {
		s.At = now
		return
	}
	dt := now.Sub(s.At).Seconds()
	s.At = now
	if dt <= 0 || s.Spec.Frequency == 0 {
		return
	}
	x, v, w, z := s.Value-s.Target, s.Velocity, s.Spec.Frequency, s.Spec.Ratio
	switch {
	case z < 1:
		a, d := z*w, w*math.Sqrt(1-z*z)
		b := (v + a*x) / d
		e, sin, cos := math.Exp(-a*dt), math.Sin(d*dt), math.Cos(d*dt)
		y := x*cos + b*sin
		s.Value, s.Velocity = s.Target+e*y, e*(-a*y-x*d*sin+b*d*cos)
	case z == 1:
		b, e := v+w*x, math.Exp(-w*dt)
		s.Value, s.Velocity = s.Target+e*(x+b*dt), e*(b-w*(x+b*dt))
	default:
		r1, r2 := -w*(z-math.Sqrt(z*z-1)), -w*(z+math.Sqrt(z*z-1))
		b := (v - r1*x) / (r2 - r1)
		a := x - b
		e1, e2 := math.Exp(r1*dt), math.Exp(r2*dt)
		s.Value, s.Velocity = s.Target+a*e1+b*e2, a*r1*e1+b*r2*e2
	}
	if math.Abs(s.Value-s.Target) < .001 && math.Abs(s.Velocity) < .001*w {
		s.Value, s.Velocity = s.Target, 0
	}
}

func (s *springValue) to(now time.Time, target float64, spec springSpec) {
	s.step(now)
	s.Target, s.Spec = target, spec
}

func (s *springValue) jump(now time.Time, value float64) {
	s.Value, s.Target, s.Velocity, s.At = value, value, 0, now
}

func (s *springValue) frame(c *ui.Context) float32 {
	if c.Preferences().ReduceMotion {
		s.jump(c.Now(), s.Target)
	} else {
		s.step(c.Now())
	}
	if s.Value != s.Target || s.Velocity != 0 {
		c.AnimationFrame()
	}
	return float32(s.Value)
}
