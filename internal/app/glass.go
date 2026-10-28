package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// The two source quality choices are preserved, but both use MyGo's material.
// Its macOS optics are not the source's iOS shader or its ultra content warp.
func glassAllowed(mode string, preferences mygo.ThemePreferences) bool {
	return (mode == "standard" || mode == "ultra") && !preferences.ReduceTransparency && !preferences.HighContrast
}

func (a *App) glassEnabled(c *ui.Context) bool {
	preferences := mygo.ThemePreferences{HighContrast: c.Preferences().HighContrast}
	// ui.Preferences in the pinned MyGo does not expose ReduceTransparency.
	// Only an actual window has a running app; headless testers never call it.
	if a.window != nil {
		native := mygo.Theme.Preferences()
		preferences.ReduceTransparency = native.ReduceTransparency
		preferences.HighContrast = preferences.HighContrast || native.HighContrast
	}
	return glassAllowed(a.cfg.LiquidGlass, preferences)
}

// glassButton is applied after a toolbar control's final Background call.
// It never retains the build-scoped element or installs a second press animation.
func (a *App) glassButton(c *ui.Context, e ui.Element) ui.Element {
	if a.glassEnabled(c) {
		e.Material(glass.Glass{Tint: a.colors.glassRegularTint})
	}
	return e
}

func (a *App) glassTintedButton(c *ui.Context, e ui.Element, tint ui.Color) ui.Element {
	if a.glassEnabled(c) {
		tint.A = 255
		e.Material(glass.Glass{Tint: tint.Alpha(.82)})
	}
	return e
}

func (a *App) glassDockSurface(c *ui.Context, e ui.Element) bool {
	if !a.glassEnabled(c) {
		return false
	}
	e.Material(glass.Glass{Tint: a.colors.glassPillTint})
	return true
}

func (a *App) dockSelectionColor(fallback ui.Color, lift float32, enabled bool) ui.Color {
	lift = min(max(lift, 0), 1)
	if enabled {
		return a.colors.glassPlatter.Alpha(1 - lift)
	}
	return fallback.Mix(a.colors.text, .08*lift)
}

func (a *App) dockGlassLens(c *ui.Context, lens ui.Rect, lift float32, enabled bool) {
	if enabled && lift > 0 {
		ui.Box(c.Key("navigation-glass-lens")).Absolute().Left(lens.X).Top(lens.Y).Width(lens.W).Height(lens.H).
			Radius(lens.H / 2).PassThrough().Opacity(min(lift, 1)).Material(glass.Glass{Style: glass.Clear})
	}
}
