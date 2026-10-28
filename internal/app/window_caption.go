package app

import (
	"encoding/json"
	"runtime"
	"sync"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"github.com/egoist/mygo/ui"
)

type windowCaptionState struct{ Pinned, Maximized, FullScreen bool }

type windowCaptionHost interface {
	Close()
	Minimize()
	ToggleMaximize()
	SetFullScreen(bool)
	SetAlwaysOnTop(bool)
	IsAlwaysOnTop() bool
}

func (a *App) captionPlatform() string {
	if a.framePlatform != "" {
		return a.framePlatform
	}
	return runtime.GOOS
}

func (a *App) captionHeight(c *ui.Context) float32 {
	if !a.frameEnabled {
		return 0
	}
	if a.captionPlatform() == "darwin" {
		if a.smallDesktop(c) {
			return 28
		}
		return 0
	}
	return 32
}

func (a *App) viewSize(c *ui.Context) (float32, float32) {
	w, h := c.Size()
	return w, max(0, h-a.frameHeight-a.navigationReservedHeight(c))
}

func (a *App) dialogBase(c *ui.Context, open *bool, content func(ui.Element, ui.Element)) ui.Element {
	return ui.DialogBase(c, open, func(back, panel ui.Element) { back.Top(a.frameHeight); content(back, panel) })
}

func (a *App) toggleWindowPin() {
	if a.captionHost == nil {
		return
	}
	a.captionHost.SetAlwaysOnTop(!a.captionHost.IsAlwaysOnTop())
	a.caption.Pinned = a.captionHost.IsAlwaysOnTop()
}

func (a *App) toggleWindowMaximized() {
	if a.captionHost == nil {
		return
	}
	if a.caption.FullScreen {
		a.captionHost.SetFullScreen(false)
	} else {
		a.captionHost.ToggleMaximize()
	}
}

func (a *App) captionBar(c *ui.Context) {
	if a.frameHeight == 0 {
		return
	}
	bar := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(a.frameHeight).Gap(0).Background(a.colors.chrome).DragWindow().Label("Window title bar")
	bar.Children(func() {
		ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Height(1).Background(a.colors.line).PassThrough()
		if a.captionPlatform() == "darwin" {
			ui.Box(c).Fill().Center().PassThrough().Children(func() { ui.Text(c, "FlClash").FontSize(13).FontWeight(500) })
			return
		}
		ui.Spacer(c)
		pinLabel := a.t("pinWindow")
		if a.caption.Pinned {
			pinLabel = a.t("unpinWindow")
		}
		ui.Box(c).Size(32, 32).Center().Children(func() {
			ui.ButtonBase(c).Size(32, 32).Radius(999).Label(pinLabel).Tooltip(pinLabel).OnClick(a.toggleWindowPin).Center().Children(func() { glyphVariant(c, "pin", a.caption.Pinned).FontSize(16) })
		})
		a.captionButton(c, "minimize", a.t("minimize"), func() {
			if a.captionHost != nil {
				a.captionHost.Minimize()
			}
		})
		glyph, label := "maximize", a.t("maximize")
		if a.caption.FullScreen {
			glyph, label = "restore", a.t("exitFullScreen")
		} else if a.caption.Maximized {
			glyph, label = "restore", a.t("unmaximize")
		}
		a.captionButton(c, glyph, label, a.toggleWindowMaximized).MaximizeButton()
		a.captionButton(c, "close", a.t("close"), func() {
			if a.captionHost != nil {
				a.captionHost.Close()
			}
		})
	})
}

func (a *App) captionButton(c *ui.Context, kind, label string, action func()) ui.Element {
	button := ui.ButtonBase(c).Size(46, 32).Radius(0).Label(label).Tooltip(label).Cursor(ui.CursorDefault).OnClick(action)
	foreground := a.colors.text
	if kind == "close" && (button.Hovered() || button.Pressed()) {
		button.Background(a.colors.danger)
		foreground = a.colors.onDanger
	}
	button.Draw(func(p *ui.Painter, r ui.Rect) { paintCaptionGlyph(p, kind, r.X+(r.W-10)/2, r.Y+(r.H-10)/2, foreground) })
	return button
}

func paintCaptionGlyph(p *ui.Painter, kind string, x, y float32, color ui.Color) {
	loadCaptions.Do(func() {
		var definitions map[string]string
		_ = json.Unmarshal(assets.Read("caption-glyphs.json"), &definitions)
		captionGlyphs = map[string]*ui.SVG{}
		for name, source := range definitions {
			captionGlyphs[name] = ui.MustParseSVG([]byte(source))
		}
	})
	if svg := captionGlyphs[kind]; svg != nil {
		p.Icon(svg, ui.Rect{X: x, Y: y, W: 10, H: 10}, color)
	}
}

var loadCaptions sync.Once
var captionGlyphs map[string]*ui.SVG
