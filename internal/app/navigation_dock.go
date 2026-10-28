package app

import (
	"math"
	"slices"

	"github.com/egoist/mygo/ui"
)

type dockLayout struct{ Pitch, Lens, Inset float32 }

func navigationDockLayout(span float32, count int) dockLayout {
	if count <= 1 {
		return dockLayout{span - 8, span, 4}
	}
	pitch := (span - 8) / float32(count)
	if pitch+8 >= 84 {
		return dockLayout{pitch, pitch + 8, 4}
	}
	lens := min(float32(84), span)
	pitch = (span - lens) / float32(count-1)
	return dockLayout{pitch, lens, (lens - pitch) / 2}
}

func rubberBand(value, limit float64) float64 {
	return limit * (1 - 1/(math.Abs(value)*.55/limit+1)) * math.Copysign(1, value)
}

type navigationDockState struct {
	Samples                                                   []pointerSample
	Initialized, Pressed, Dragging, Dropping, Hover           bool
	Selected, PressedIndex                                    int
	PressX                                                    float32
	Lens, Lift, Swell, Jelly, Pull, Highlight, HighlightAlpha springValue
}

var dockTravel = durationSpring(.4, .15)
var dockHover = durationSpring(.26, .18)
var dockFade = durationSpring(.2, 0)
var dockLift = durationSpring(.25, 0)
var dockPress = springSpec{math.Sqrt(311), .593}
var dockFollow = springSpec{32, .9}
var dockSettle = durationSpring(.5, .32)

func dockPosition(x, width float32, count int) float64 {
	l := navigationDockLayout(width-8, count)
	position := float64((x-4-l.Inset)/l.Pitch - .5)
	last := float64(count - 1)
	if position < 0 {
		return rubberBand(position, .35)
	}
	if position > last {
		return last + rubberBand(position-last, .35)
	}
	return position
}

func dockIndex(position float64, count int) int {
	return min(max(int(math.Round(position)), 0), count-1)
}

func (a *App) navigationDockInput(e ui.InputEvent, width float32, pages []string) bool {
	s := &a.navigationDock
	position := dockPosition(e.X, width, len(pages))
	index := dockIndex(position, len(pages))
	switch e.Kind {
	case ui.InputPointerDown:
		if e.Button != 0 || s.Pressed {
			return false
		}
		s.Pressed, s.Dragging, s.Dropping = true, false, false
		s.PressX = e.X
		s.Samples = []pointerSample{{X: float64(e.X), At: e.Time}}
		s.PressedIndex = index
		s.HighlightAlpha.to(e.Time, 0, dockFade)
		s.Swell.to(e.Time, 1, dockPress)
		s.Lift.to(e.Time, 1, dockLift)
		s.Lens.to(e.Time, float64(index), dockTravel)
		return true
	case ui.InputPointerMove:
		if !s.Pressed {
			s.Hover = true
			lean := float64(index) + (position-float64(index))*.2
			if s.HighlightAlpha.Value == 0 {
				s.Highlight.jump(e.Time, lean)
			} else {
				s.Highlight.to(e.Time, lean, dockHover)
			}
			s.HighlightAlpha.to(e.Time, 1, dockFade)
			s.Swell.to(e.Time, .4, dockSettle)
			if index == s.Selected {
				s.Lens.to(e.Time, lean, dockHover)
			} else {
				s.Lens.to(e.Time, float64(s.Selected), dockHover)
			}
			return false
		}
		s.Samples = append(s.Samples, pointerSample{X: float64(e.X), At: e.Time})
		if len(s.Samples) > 20 {
			s.Samples = s.Samples[len(s.Samples)-20:]
		}
		if !s.Dragging && float32(math.Abs(float64(e.X-s.PressX))) < 18 {
			return true
		}
		s.Dragging = true
		s.PressedIndex = index
		s.Lens.to(e.Time, position, dockFollow)
		over := float64(e.X - min(max(e.X, 0), width))
		s.Pull.to(e.Time, rubberBand(over, float64(62+10*a.cfg.Scale-10)*7/32), durationSpring(.12, 0))
		return true
	case ui.InputPointerUp:
		if !s.Pressed {
			return false
		}
		commit := a.window == nil || a.window.IsFocused()
		velocity := float32(pointerVelocity(s.Samples, e.Time))
		if s.Dragging && commit {
			layout := navigationDockLayout(width-8, len(pages))
			lead := float64(velocity/layout.Pitch) * .1
			s.PressedIndex = min(max(dockIndex(s.Lens.Target+lead, len(pages)), s.PressedIndex-1), s.PressedIndex+1)
		}
		s.Pressed, s.Dragging, s.Dropping = false, false, true
		target := s.Selected
		if commit {
			target = s.PressedIndex
			a.selectNavigation(pages[target])
		}
		s.Lens.to(e.Time, float64(target), dockTravel)
		s.Pull.to(e.Time, 0, dockPress)
		s.Swell.to(e.Time, 0, dockPress)
		return true
	}
	return false
}

func (a *App) bottomNavigationInset(c *ui.Context) float32 {
	if !a.hasBottomNavigation(c) {
		return 0
	}
	if !a.cfg.FloatingNavigation {
		return 0
	}
	return 83 + fontSize(c, 10) - 10
}

func (a *App) navigationReservedHeight(c *ui.Context) float32 {
	if a.hasBottomNavigation(c) && !a.cfg.FloatingNavigation {
		return 80
	}
	return 0
}

func (a *App) bottomNavigation(c *ui.Context) {
	if !a.hasBottomNavigation(c) {
		a.navigationDock.Initialized = false
		return
	}
	pages := a.navigationPages(true)
	selected := slices.Index(pages, a.navigationParent())
	if selected < 0 {
		return
	}
	if !a.cfg.FloatingNavigation {
		a.dockedNavigation(c, pages, selected)
		return
	}
	width, _ := c.Size()
	barWidth := width - 42
	if len(pages) < 5 {
		barWidth = min(barWidth, float32(len(pages))*86+16)
	}
	height := 62 + fontSize(c, 10) - 10
	left := (width - barWidth) / 2
	layout := navigationDockLayout(barWidth-8, len(pages))
	s := &a.navigationDock
	if !s.Initialized {
		*s = navigationDockState{Initialized: true, Selected: selected, PressedIndex: -1}
		s.Lens.jump(c.Now(), float64(selected))
	}
	if s.Selected != selected {
		s.Selected = selected
		if !s.Pressed {
			s.Lens.to(c.Now(), float64(selected), dockTravel)
		}
	}
	position, lift, swell := s.Lens.frame(c), s.Lift.frame(c), s.Swell.frame(c)
	pull, highlight, highlightAlpha := s.Pull.frame(c), s.Highlight.frame(c), s.HighlightAlpha.frame(c)
	if s.Dropping && !s.Pressed && (lift >= .975 || c.Preferences().ReduceMotion) && math.Abs(s.Lens.Target-s.Lens.Value) <= .04 {
		s.Dropping = false
		s.Lift.to(c.Now(), 0, dockLift)
		c.AnimationFrame()
	}
	if s.Lens.Spec.Frequency != 0 {
		acc := s.Lens.Spec.Frequency*s.Lens.Spec.Frequency*(s.Lens.Target-s.Lens.Value) - 2*s.Lens.Spec.Ratio*s.Lens.Spec.Frequency*s.Lens.Velocity
		braking := max(0, -acc*math.Copysign(1, s.Lens.Velocity))
		target := .2*math.Tanh(math.Abs(s.Lens.Velocity)/3) - .15*math.Tanh(braking/25)
		s.Jelly.to(c.Now(), target, springSpec{10, .4})
	}
	jelly := s.Jelly.frame(c)
	lensWidth := (layout.Lens + 16*lift) * (1 + jelly)
	lensHeight := (height - 8 + 16*lift) * (1 - 1.4*jelly)
	lens := ui.Rect{X: 4 + layout.Inset + (position+.5)*layout.Pitch - lensWidth/2, Y: 4 + (height-8-lensHeight)/2, W: lensWidth, H: lensHeight}
	background, lensColor := a.colors.card, a.colors.block
	if a.colors.dark {
		background, lensColor = a.colors.block, a.colors.inset
	}
	strength := float32(1)
	if a.colors.dark {
		strength = 3
	}
	input := ui.Box(c.Key("navigation-dock-input")).Absolute().Left(left - 8).Bottom(13).Width(barWidth + 16).Height(height + 16)
	input.HandleInput(func(e ui.InputEvent) bool { e.X -= 8; e.Y -= 8; return a.navigationDockInput(e, barWidth, pages) })
	_, _, over := input.PointerPosition()
	if !over && s.Hover && !s.Pressed {
		s.Hover = false
		s.HighlightAlpha.to(c.Now(), 0, dockFade)
		s.Swell.to(c.Now(), 0, dockSettle)
		s.Lens.to(c.Now(), float64(selected), dockHover)
		c.AnimationFrame()
	}
	labelScale := float32(1)
	for _, page := range pages {
		measured, _ := c.MeasureText(0, ui.Span{Text: a.navigationLabel(page), Size: fontSize(c, 10), Weight: 500})
		if measured > layout.Pitch-4 {
			labelScale = min(labelScale, max((layout.Pitch-4)/measured, .9))
		}
	}
	input.Children(func() {
		bar := ui.Box(c.Key("navigation-dock")).Absolute().Left(8).Top(8).Width(barWidth).Height(height).Radius(999).Background(background).PassThrough().
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .08*strength)).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, .04*strength)).Role(ui.RoleTabList).Label(a.t("navigationBarStyle"))
		growth := 1 + swell*min(float32(.25), 16/barWidth)
		bar.Scale(growth*(1+float32(math.Abs(float64(pull)))/barWidth*.5), growth).Translate(pull, 0)
		bar.Draw(func(p *ui.Painter, r ui.Rect) {
			if highlightAlpha > 0 {
				p.Fill(ui.Rect{X: r.X + 4 + layout.Inset + (highlight+.5)*layout.Pitch - layout.Lens/2, Y: r.Y + 4, W: layout.Lens, H: height - 8}, a.colors.text.Alpha(.08*min(max(highlightAlpha, 0), 1)), (height-8)/2)
			}
			p.Fill(ui.Rect{X: r.X + lens.X, Y: r.Y + lens.Y, W: lens.W, H: lens.H}, lensColor.Mix(a.colors.text, .08*min(max(lift, 0), 1)), lens.H/2)
		})
		bar.Children(func() {
			a.dockItems(c, pages, selected, layout, height, a.colors.muted, labelScale, true)
			ui.Box(c).Absolute().Left(lens.X).Top(lens.Y).Width(lens.W).Height(lens.H).Radius(lens.H / 2).Clip().PassThrough().Children(func() {
				ui.Box(c).Absolute().Left(-lens.X).Top(-lens.Y).Width(barWidth).Height(height).Scale(1+.16*lift, 1+.16*lift).TransformOrigin((lens.X+lens.W/2)/barWidth, .5).PassThrough().Children(func() {
					a.dockItems(c, pages, selected, layout, height, a.colors.accent, labelScale, false)
				})
			})
		})
	})
}

func (a *App) dockItems(c *ui.Context, pages []string, selected int, layout dockLayout, height float32, color ui.Color, labelScale float32, interactive bool) {
	ui.Row(c).Absolute().Left(4 + layout.Inset).Right(4 + layout.Inset).Top(4).Bottom(4).Gap(0).Children(func() {
		for i, page := range pages {
			item := ui.Box(c.Key(page)).Width(layout.Pitch).FillHeight().Center().Radius(999)
			if interactive {
				item.Role(ui.RoleTab).Label(a.navigationLabel(page)).Selected(i == selected).Background(ui.Transparent).Focusable().OnClick(func() { a.selectNavigation(page) })
				if item.Focused() {
					item.Background(a.colors.text.Alpha(.1)).Border(2, a.colors.accent)
				}
			} else {
				item.PassThrough()
			}
			item.Children(func() {
				ui.Column(c).Gap(2).AlignItems(ui.Center).Children(func() {
					glyphVariant(c, page, true).FontSize(28).TextColor(color)
					ui.Text(c, a.navigationLabel(page)).FontSize(fontSize(c, 10) * labelScale).FixedLineHeight(fontSize(c, 14.5)).FontWeight(500).TextColor(color).SingleLine()
				})
			})
		}
	})
}

func (a *App) dockedNavigation(c *ui.Context, pages []string, selected int) {
	width, _ := c.Size()
	background, lensColor := a.colors.card, a.colors.block
	if a.colors.dark {
		background, lensColor = a.colors.block, a.colors.inset
	}
	ui.Row(c.Key("docked-navigation")).Absolute().Left(0).Right(0).Bottom(0).Height(80).Gap(0).Background(background).Role(ui.RoleTabList).Label(a.t("navigationBarStyle")).Children(func() {
		for i, page := range pages {
			ui.ButtonBase(c.Key(page)).Width(width / float32(len(pages))).Height(80).Label(a.navigationLabel(page)).Role(ui.RoleTab).Selected(i == selected).Background(ui.Transparent).OnClick(func() { a.selectNavigation(page) }).Center().Children(func() {
				ui.Column(c).Gap(4).AlignItems(ui.Center).Children(func() {
					fill, color := ui.Transparent, a.colors.muted
					if i == selected {
						fill, color = lensColor, a.colors.accent
					}
					ui.Box(c).Size(64, 32).Radius(999).Background(fill).Center().Children(func() { glyphVariant(c, page, i == selected).FontSize(24).TextColor(color) })
					weight, labelColor := 500, a.colors.muted
					if i == selected {
						weight, labelColor = 600, a.colors.text
					}
					ui.Text(c, a.navigationLabel(page)).FontSize(fontSize(c, 12)).FixedLineHeight(fontSize(c, 16)).FontWeight(weight).TextColor(labelColor).SingleLine()
				})
			})
		}
	})
}
