package app

import (
	"encoding/json"
	"math"
	"sync"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"github.com/egoist/mygo/ui"
)

var dashboardShapes map[string][][8]float32
var dashboardShapesOnce sync.Once

func loadDashboardShapes() {
	_ = json.Unmarshal(assets.Read("dashboard-shapes.json"), &dashboardShapes)
}

func (a *App) dashboardStartControl(c *ui.Context, side float32) {
	enabled := len(a.cfg.Profiles) > 0
	label, icon := a.t("start"), "play"
	target := float32(0)
	if a.wantRunning {
		label, icon, target = a.t("stop"), "pause", 1
	}
	ui.Box(c.Key("start-stop")).Size(side, side).Shrink(0).Children(func() {
		button := ui.ButtonBase(c).Label(label).Tooltip(label).Fill().Disabled(!enabled).OnClick(func() {
			if !a.safe {
				a.toggle()
			}
		})
		ease := ui.CubicBezier(0.175, 0.885, 0.32, 1.275)
		if !a.wantRunning {
			ease = ui.CubicBezier(0.55, 0.055, 0.675, 0.19)
		}
		progress := button.AnimateWith("running", target, 450*time.Millisecond, ease)
		fill, ink := a.colors.accent, a.colors.onAccent
		if !enabled {
			fill, ink = a.colors.inset, a.colors.muted
		}
		overlay := float32(0)
		if button.Focused() {
			overlay = 0.1
		} else if button.Hovered() {
			overlay = 0.08
		}
		overlay = button.Animate("overlay", overlay, 150*time.Millisecond)
		fill = fill.Mix(ink, overlay)
		ui.Box(c).Absolute().Left(0).Top(0).Size(side, side).PassThrough().Center().Draw(func(p *ui.Painter, r ui.Rect) {
			if enabled {
				p.Image(startShadow(r.W, progress, a.colors.accent), ui.Rect{X: r.X - 28, Y: r.Y - 28, W: r.W + 56, H: r.H + 56}, ui.Contain)
			}
			p.FillPath(dashboardShape("start", r, progress, progress*40), fill)
		}).Children(func() { glyphVariant(c, icon, true).FontSize(side * 0.34).TextColor(ink) })
	})
}

func dashboardShape(name string, r ui.Rect, progress, degrees float32) *ui.Path {
	dashboardShapesOnce.Do(loadDashboardShapes)
	curves := dashboardShapes[name]
	if name == "start" {
		curves = dashboardShapes["start0"]
	}
	angle := float64(degrees) * math.Pi / 180
	sin, cos := float32(math.Sin(angle)), float32(math.Cos(angle))
	point := func(x, y float32) (float32, float32) {
		x, y = x-0.5, y-0.5
		return r.X + r.W/2 + (x*cos-y*sin)*r.W, r.Y + r.H/2 + (x*sin+y*cos)*r.H
	}
	path := new(ui.Path)
	for index, curve := range curves {
		if name == "start" {
			end := dashboardShapes["start1"][index]
			for i := range curve {
				curve[i] += (end[i] - curve[i]) * progress
			}
		}
		if index == 0 {
			x, y := point(curve[0], curve[1])
			path.MoveTo(x, y)
		}
		x1, y1 := point(curve[2], curve[3])
		x2, y2 := point(curve[4], curve[5])
		x3, y3 := point(curve[6], curve[7])
		path.CubeTo(x1, y1, x2, y2, x3, y3)
	}
	return path.Close()
}
