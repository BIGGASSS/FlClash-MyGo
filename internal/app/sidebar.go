package app

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// These timings, curves and dimensions follow NavigationSidebar in FlClash.
var sidebarExpansion = ui.CubicBezier(0.2, 0, 0, 1)
var sidebarLead = ui.CubicBezier(0.215, 0.61, 0.355, 1)
var sidebarTrail = ui.CubicBezier(0.645, 0.045, 0.355, 1)

type sidebarSelection struct {
	initialized bool
	from, index int
	started     time.Time
}

func (a *App) sidebarSize() float32 {
	if a.sidebarReady || a.sidebarWidth != 0 {
		return a.sidebarWidth
	}
	if a.cfg.Sidebar {
		return 220
	}
	return 48
}

func (a *App) sidebarCompactWidth(c *ui.Context) float32 {
	if a.frameEnabled && a.captionPlatform() == "darwin" && !a.smallDesktop(c) {
		return max(48, c.TitleBar().Left)
	}
	return 48
}

func (a *App) sidebar(c *ui.Context) {
	compact := a.sidebarCompactWidth(c)
	width, _ := c.Size()
	canExpand := !a.frameEnabled || width > 840
	expanded := canExpand && a.cfg.Sidebar
	target := compact
	if expanded {
		target = 220
	}
	if a.smallDesktop(c) {
		target = 0
	}
	pane := ui.Column(c).FillHeight().Background(a.colors.chrome).Gap(0).Clip()
	a.sidebarWidth = pane.AnimateWith("width", target, 250*time.Millisecond, sidebarExpansion)
	a.sidebarReady = true
	sideInset := min(float32(12), (compact-40)/2)
	pane.Width(a.sidebarWidth).Padding(4, min(sideInset, a.sidebarWidth/2))
	if a.sidebarWidth < .01 {
		return
	}
	progress := min(max((a.sidebarWidth-compact)/(220-compact), 0), 1)
	toggle := func() { a.cfg.Sidebar = !a.cfg.Sidebar; a.save() }
	if !canExpand {
		toggle = func() { a.openSidebarOverlay(c.Now()) }
	}
	a.sidebarPane(c, pane, compact, progress, expanded, toggle, nil, &a.sidebarSelection)
}

func (a *App) sidebarPane(c *ui.Context, pane ui.Element, compact, progress float32, expanded bool, toggleAction, onSelected func(), selection *sidebarSelection) {
	sideInset := min(float32(12), (compact-40)/2)
	iconInset := (compact - 2*sideInset - 24) / 2
	itemHeight := 40 + (a.cfg.Scale-1)*15
	labelOpacity := min(max((progress-.4)/.6, 0), 1)
	pane.Children(func() {
		if a.captionPlatform() == "darwin" && a.frameHeight == 0 {
			ui.Box(c).Height(max(c.TitleBar().Height, 28)).DragWindow()
		}
		toggleLabel := a.t("showMore")
		if expanded {
			toggleLabel = a.t("showLess")
		}
		toggle := ui.ButtonBase(c.Key("sidebar-toggle")).Label(toggleLabel).Tooltip(toggleLabel).
			Size(compact-2*sideInset, itemHeight).Margin(2, 0, 10, 0).AlignSelf(ui.Start).Radius(10).Cursor(ui.CursorDefault).
			OnClick(toggleAction)
		a.sidebarButton(c, toggle, false)
		toggle.Center().Children(func() { sidebarGlyph(c, progress, a.colors.muted) })
		pages := a.navigationPages(false)
		parent := a.navigationParent()
		selected := -1
		for index, page := range pages {
			if page == parent {
				selected = index
			}
		}
		top, height := a.sidebarIndicatorFor(c, selection, selected, itemHeight+4)
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(0).DrawOver(func(p *ui.Painter, r ui.Rect) {
				if selected >= 0 {
					p.Fill(ui.Rect{X: r.X, Y: r.Y + top, W: 3, H: height}, a.colors.accent, 1.5)
				}
			}).Children(func() {
				for _, page := range pages {
					key := page
					if page == "connections" {
						key = "liveConnections"
					}
					label := a.t(key)
					button := ui.ButtonBase(c.Key(page)).Label(label).Selected(page == parent).FillWidth().Height(itemHeight).
						Margin(2, 0).Radius(10).Clip().Cursor(ui.CursorDefault).
						OnClick(func() {
							a.selectNavigation(page)
							if onSelected != nil {
								onSelected()
							}
						})
					a.sidebarButton(c, button, page == parent)
					textWidth, _ := c.MeasureText(0, ui.Span{Text: label, Size: fontSize(c, 15)})
					if !expanded || progress < 1 || textWidth > 220-2*sideInset-iconInset-24-12-12 {
						button.Tooltip(label)
					}
					button.Children(func() {
						ui.Row(c).Width(220-2*sideInset).Shrink(0).FillHeight().Padding(0, 12, 0, iconInset).Gap(12).Children(func() {
							glyphVariant(c, page, parent == page).FontSize(24).TextColor(a.colors.muted)
							ui.Text(c, label).Grow(1).MinWidth(0).SingleLine().FixedLineHeight(a.textHeight(20)).
								FontSize(fontSize(c, 15)).TextColor(a.colors.text).Opacity(labelOpacity)
						})
					})
				}
			})
		})
		if a.safe {
			ui.Text(c, "SAFE MODE").FontSize(fontSize(c, 10)).TextColor(a.colors.warning).SingleLine()
		}
	})
}

func (a *App) sidebarButton(_ *ui.Context, button ui.Element, selected bool) {
	fill := ui.Transparent
	if selected {
		fill = a.colors.text.Alpha(0.08)
	}
	overlay := float32(0)
	switch {
	case button.Pressed():
		overlay = 0.06
	case button.Focused():
		overlay = 0.1
	case button.Hovered():
		overlay = 0.04
	}
	button.Background(fill).DrawOver(func(p *ui.Painter, r ui.Rect) {
		if overlay > 0 {
			p.Fill(r, a.colors.text.Alpha(overlay), 10)
		}
	})
}

func (a *App) sidebarIndicator(c *ui.Context, index int, stride float32) (float32, float32) {
	return a.sidebarIndicatorFor(c, &a.sidebarSelection, index, stride)
}

func (a *App) sidebarIndicatorFor(c *ui.Context, s *sidebarSelection, index int, stride float32) (float32, float32) {
	if index < 0 {
		s.initialized = false
		return 0, 0
	}
	if !s.initialized {
		*s = sidebarSelection{initialized: true, from: index, index: index}
	} else if index != s.index {
		s.from, s.index, s.started = s.index, index, c.Now()
	}
	t := float32(1)
	if !s.started.IsZero() && !c.Preferences().ReduceMotion {
		t = min(float32(c.Now().Sub(s.started))/float32(320*time.Millisecond), 1)
		if t < 1 {
			c.AnimationFrame()
		}
	}
	from := float32(s.from)*stride + (stride-16)/2
	to := float32(index)*stride + (stride-16)/2
	lead := sidebarLead(min(1, t/0.6))
	trail := sidebarTrail(max(0, (t-0.35)/0.65))
	top, bottom := from+(to-from)*trail, from+(to-from)*lead
	if to < from {
		top, bottom = bottom, top
	}
	return top, bottom - top + 16
}

func sidebarGlyph(c *ui.Context, progress float32, color ui.Color) {
	// Keep the exported superellipse frame exact and draw the original glyph's
	// four moving strokes directly, avoiding reparsing an SVG every frame.
	ensureGlyphs()
	frame := glyphCache["sidebar-frame"]
	if frame == nil {
		data := glyphVectors["sidebarCompact"].Outline
		frame = ui.MustParseSVG([]byte(data[:strings.Index(data, "</g>")+4] + "</svg>"))
		glyphCache["sidebar-frame"] = frame
	}
	ui.Box(c).Size(24, 24).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Icon(frame, r, color)
		x, y := r.W/24, r.H/24
		divider := 8.5 + 3*progress
		path := new(ui.Path).MoveTo(r.X+divider*x, r.Y+4.5*y).LineTo(r.X+divider*x, r.Y+19.5*y)
		for _, line := range []float32{8.5, 11, 13.5} {
			path.MoveTo(r.X+5.5*x, r.Y+line*y).LineTo(r.X+(divider-2.5)*x, r.Y+line*y)
		}
		p.StrokePath(path, 1.8*min(x, y), color)
	})
}
