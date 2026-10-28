package app

import (
	"math"
	"runtime"
	"strings"
	"unicode"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	colortheme "github.com/BIGGASSS/FlClash-MyGo/internal/theme"
	"github.com/egoist/mygo/ui"
)

type palette struct {
	glassRegularTint, glassPillTint, glassPlatter                                                                           ui.Color
	secondaryContainer, onSecondaryContainer                                                                                ui.Color
	outline, successContainer, onSuccessContainer, warningContainer, onWarningContainer, dangerContainer, onDangerContainer ui.Color
	page, card, block, inset, chrome, text, muted, accent, onAccent, success, warning, danger, onDanger, line               ui.Color
	dark                                                                                                                    bool
}

func (a *App) theme(c *ui.Context) {
	dark := a.systemDark
	if a.cfg.Theme == "dark" {
		dark = true
	} else if a.cfg.Theme == "light" {
		dark = false
	}
	colors, err := colortheme.Generate(a.cfg.Accent, a.cfg.ColorStyle, dark, a.cfg.PureBlack)
	if err != nil {
		colors, _ = colortheme.Generate("#007aff", "vivid", dark, false)
	}
	p := palette{dark: dark}
	for key, dst := range map[string]*ui.Color{"page": &p.page, "card": &p.card, "block": &p.block, "inset": &p.inset, "chrome": &p.chrome, "text": &p.text, "muted": &p.muted, "accent": &p.accent, "onAccent": &p.onAccent, "secondaryContainer": &p.secondaryContainer, "onSecondaryContainer": &p.onSecondaryContainer, "success": &p.success, "warning": &p.warning, "danger": &p.danger, "onDanger": &p.onDanger, "line": &p.line, "outline": &p.outline, "successContainer": &p.successContainer, "onSuccessContainer": &p.onSuccessContainer, "warningContainer": &p.warningContainer, "onWarningContainer": &p.onWarningContainer, "dangerContainer": &p.dangerContainer, "onDangerContainer": &p.onDangerContainer} {
		*dst = ui.Hex(colors[key])
	}
	if a.cfg.LiquidGlass == "standard" || a.cfg.LiquidGlass == "ultra" {
		glassColors, err := colortheme.GlassColors(a.cfg.Accent, a.cfg.ColorStyle, dark, a.cfg.PureBlack)
		if err != nil {
			glassColors, _ = colortheme.GlassColors("#007aff", "vivid", dark, false)
		}
		p.glassRegularTint = ui.Hex(glassColors["regularTint"])
		p.glassPillTint = ui.Hex(glassColors["pillTint"])
		p.glassPlatter = ui.Hex(glassColors["platter"])
	}
	a.colors = p
	t := *ui.LightTheme()
	t.Dark = dark
	t.ContinuousCorners = true
	t.Background = p.page
	t.Surface = p.block
	t.SurfaceHover = p.inset
	t.SurfacePressed = p.inset
	t.Border = ui.Transparent
	t.Text = p.text
	t.TextMuted = p.muted
	t.Accent = p.accent
	t.AccentText = p.onAccent
	t.AccentHover = p.accent.Mix(p.text, 0.08)
	t.AccentPressed = p.accent.Mix(p.text, 0.15)
	t.Success = p.success
	t.Warning = p.warning
	t.Danger = p.danger
	t.Radius = 16
	t.FontSize = 15 * a.cfg.Scale
	if runtime.GOOS != "darwin" {
		t.Font = "Roboto"
	}
	if a.cfg.FontFamily != "" {
		t.Font = a.cfg.FontFamily
	}
	t.Focus = p.accent
	t.Selection = p.accent.Alpha(0.2)
	c.SetTheme(&t)
	c.SetZoom(1)
	locale := a.cfg.Locale
	if locale == "" {
		locale = c.Locale()
	}
	if a.locale != locale || a.catalog == nil {
		a.locale = locale
		a.catalog = assets.Catalog(locale)
	}
}

// fontSize applies the text preference without scaling hit targets or window geometry.
func fontSize(c *ui.Context, size float32) float32 {
	return size * c.Theme().FontSize / 15
}

// Flutter rounds each measured line to a logical pixel before arranging proxy rows.
func (a *App) textHeight(leading float32) float32 {
	return float32(math.Round(float64(leading) * float64(a.cfg.Scale)))
}

func (a *App) card(c *ui.Context) ui.Element {
	return ui.Column(c).Background(a.colors.card).Radius(28).Padding(20).Gap(12)
}
func (a *App) section(c *ui.Context, title string, fn func(), footer ...string) {
	ui.Column(c).Gap(8).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(fontSize(c, 13)).TextColor(a.colors.muted).Padding(4, 16)
		}
		ui.Column(c).Background(a.colors.card).Radius(20).Padding(4, 0).Children(fn)
		if len(footer) > 0 && footer[0] != "" {
			ui.Text(c, footer[0]).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).TextColor(a.colors.muted).Padding(0, 16)
		}
	})
}
func (a *App) row(c *ui.Context, icon, label string, tail func()) ui.Element {
	return ui.Row(c).FillWidth().MinHeight(58).Padding(10, 16).Gap(14).Children(func() {
		if icon != "" {
			glyph(c, icon).FontSize(22).TextColor(a.colors.muted)
		}
		ui.Text(c, label).Grow(1)
		if tail != nil {
			tail()
		}
	})
}
func (a *App) linkRow(c *ui.Context, icon, label, page string) {
	ui.ButtonBase(c.Key(page)).FillWidth().Padding(0).OnClick(func() { a.goPage(page) }).Children(func() { a.row(c, icon, label, func() { glyph(c, "chevron").FontSize(18).TextColor(a.colors.muted) }) })
}
func (a *App) separator(c *ui.Context) {
	ui.Box(c).Height(1).Margin(0, 16, 0, 52).Background(a.colors.line.Alpha(0.6))
}
func (a *App) chip(c *ui.Context, text string, color ui.Color) {
	background, foreground := color.Alpha(0.13), color
	if color == a.colors.success {
		background, foreground = a.colors.successContainer, a.colors.onSuccessContainer
	} else if color == a.colors.warning {
		background, foreground = a.colors.warningContainer, a.colors.onWarningContainer
	} else if color == a.colors.danger {
		background, foreground = a.colors.dangerContainer, a.colors.onDangerContainer
	}
	ui.Text(c, text).FontSize(fontSize(c, 12)).FontWeight(600).TextColor(foreground).Background(background).Radius(999).Padding(4, 9)
}
func (a *App) button(c *ui.Context, icon, label string, action func()) ui.Element {
	b := ui.ButtonBase(c).Label(label).Padding(10).Radius(999).Background(a.colors.block).TextColor(a.colors.text).OnClick(action)
	if b.Hovered() {
		b.Background(a.colors.inset)
	}
	b.Children(func() { glyph(c, icon).FontSize(21) })
	return a.glassButton(c, b)
}
func (a *App) header(c *ui.Context, icon, label string) {
	ui.Row(c).Gap(8).Children(func() {
		glyph(c, icon).FontSize(17).TextColor(a.colors.muted)
		ui.Text(c, label).FontSize(fontSize(c, 13)).FontWeight(600).TextColor(a.colors.muted)
	})
}
func (a *App) empty(c *ui.Context, icon, title, body string, action func()) {
	ui.Column(c).Fill().MinHeight(240).Center().Gap(16).Padding(32).Children(func() {
		glyph(c, icon).FontSize(52).TextColor(a.colors.muted)
		ui.Text(c, title).FontSize(fontSize(c, 20)).Bold()
		ui.Text(c, body).TextColor(a.colors.muted).MaxWidth(420).TextAlign(ui.Center)
		if action != nil {
			ui.PrimaryButton(c, a.t("addProfile")).Radius(999).Padding(12, 24).OnClick(action)
		}
	})
}
func seg(c *ui.Context, current *int, labels ...string) ui.Element {
	s := ui.SegmentedBase(c, current, len(labels))
	t := c.Theme()
	s.Track.Gap(3).Padding(3).Radius(999).Background(t.Surface).Children(func() {
		for i, label := range labels {
			e := s.Segment(i).Label(label).Padding(9, 14).Radius(999)
			if i == *current {
				e.Background(t.Accent).TextColor(t.AccentText)
			}
			e.Children(func() { ui.Text(c, label).SingleLine().FontSize(fontSize(c, 14)).FontWeight(600) })
		}
	})
	return s.Track
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
func humanize(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r == '-' || r == '_' {
			b.WriteByte(' ')
			continue
		}
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return titleCase(b.String())
}

func (a *App) groupedSection(c *ui.Context, title string, content func()) {
	ui.Column(c).Gap(12).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(fontSize(c, 17)).FontWeight(600).TextColor(a.colors.muted).Margin(0, 16)
		}
		ui.Column(c).Radius(24).Background(a.colors.card).Children(content)
	})
}
