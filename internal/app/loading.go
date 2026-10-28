package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// CommonCircleLoading's one-second comet ring, with its original sweep,
// twelve-percent track, minimum stroke, gradient tail, and round head.
func loading(c *ui.Context, side float32, color ui.Color, label string) ui.Element {
	e := ui.Spinner(c).Size(side, side).TextColor(color).Label(label)
	turns := e.Loop("turn", time.Second, ui.Linear)
	e.Draw(func(p *ui.Painter, bounds ui.Rect) {
		side := min(bounds.W, bounds.H)
		stroke := max(side/12, float32(1.5))
		radius := (side*(20.0/24) - stroke) / 2
		if radius <= 0 {
			return
		}
		x, y := bounds.X+bounds.W/2, bounds.Y+bounds.H/2
		var ring ui.Path
		ring.Circle(x, y, radius).Cap(ui.ButtCap)
		p.StrokePath(&ring, stroke, color.Alpha(0.12))
		p.StrokePathConicGradient(&ring, stroke, ui.ConicGradient{Angle: 90 + turns*360, Stops: []ui.GradientStop{{Color: color.Alpha(0), At: 0}, {Color: color.Alpha(0.3), At: 0.36}, {Color: color, At: 0.72}, {Color: color.Alpha(0), At: 0.72}}})
		angle := float64((turns + 0.72) * 2 * math.Pi)
		var head ui.Path
		head.Circle(x+float32(math.Cos(angle))*radius, y+float32(math.Sin(angle))*radius, stroke/2)
		p.FillPath(&head, color)
	})
	return e
}
