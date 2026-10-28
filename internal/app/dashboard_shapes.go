package app

import (
	"encoding/json"
	"math"
	"sync"

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
	label := a.t("start")
	target := float32(0)
	if a.wantRunning {
		label, target = a.t("stop"), 1
		if a.onDemandSuspended() {
			label = a.t("suspended")
		}
	}
	ui.Box(c.Key("start-stop")).Size(side, side).Shrink(0).Children(func() {
		button := ui.ButtonBase(c).Label(label).Tooltip(label).Fill().Disabled(!enabled).FocusRing(false).OnClick(func() {
			if !a.safe {
				a.toggle()
			}
		})
		state := ui.Local(button, "dashboard-morph", func() dashboardMorphState { return dashboardMorphState{} })
		suspended := target == 1 && a.onDemandSuspended()
		progress, glyphProgress := state.frame(c, target == 1, suspended)
		button.HandleInput(func(e ui.InputEvent) bool {
			if enabled {
				state.press.input(e, side)
			}
			return false // Keep ButtonBase's pointer/keyboard activation and capture.
		})
		sx, sy, x, y := state.press.frame(c, side, enabled)
		fill, ink := a.colors.accent, a.colors.onAccent
		if !enabled {
			fill, ink = a.colors.inset, a.colors.muted
		}
		focused := enabled && button.Focused()
		overlay := button.AnimateWith("overlay", dashboardOverlay(enabled, focused, button.Hovered()), dashboardStateDuration, ui.Linear)
		fill = fill.Mix(ink, overlay)
		button.Children(func() {
			ui.Box(c).Absolute().Left(0).Top(0).Size(side, side).Scale(sx, sy).Translate(x, y).PassThrough().Center().Draw(func(p *ui.Painter, r ui.Rect) {
				if enabled {
					p.Image(startShadow(r.W, progress, a.colors.accent), ui.Rect{X: r.X - startShadowMargin, Y: r.Y - startShadowMargin, W: r.W + 2*startShadowMargin, H: r.H + 2*startShadowMargin}, ui.Contain)
				}
				p.FillPath(dashboardShape("start", r, progress, progress*40), fill)
				if focused {
					p.StrokePath(dashboardFocusPath(r, progress), 2, a.colors.accent)
				}
			}).Children(func() {
				if suspended {
					glyphVariant(c, "wifiOff", true).FontSize(side * 0.34).TextColor(ink)
				} else {
					ui.Box(c).Size(side*0.34, side*0.34).Draw(func(p *ui.Painter, r ui.Rect) {
						paintDashboardPlayPause(p, r, glyphProgress, ink)
					})
				}
			})
		})
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
