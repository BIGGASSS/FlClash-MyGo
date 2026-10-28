package app

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestGlassAccessibilityPolicy(t *testing.T) {
	for _, mode := range []string{"", "none", "standard", "ultra", "future"} {
		enabled := mode == "standard" || mode == "ultra"
		for _, reduceMotion := range []bool{false, true} {
			prefs := mygo.ThemePreferences{ReduceMotion: reduceMotion}
			if glassAllowed(mode, prefs) != enabled {
				t.Fatal("reduced motion changed the static glass preference", mode)
			}
			prefs.ReduceTransparency = true
			if glassAllowed(mode, prefs) {
				t.Fatal("reduced transparency left glass enabled", mode)
			}
			prefs.ReduceTransparency, prefs.HighContrast = false, true
			if glassAllowed(mode, prefs) {
				t.Fatal("high contrast left glass enabled", mode)
			}
		}
	}
}

func glassReferenceView(a *App) func(*ui.Context) {
	return func(c *ui.Context) {
		a.theme(c)
		ui.Box(c).Fill().Children(func() {
			ui.Row(c).Fill().Gap(0).Children(func() {
				for _, color := range []string{"#ffd5cf", "#cfe5ff", "#d6f4d3", "#ffe8b7", "#e4d4ff"} {
					ui.Box(c).Grow(1).FillHeight().Background(ui.Hex(color))
				}
			})
			a.glassButton(c, a.button(c, "refresh", "Glass reference control", func() {}).
				Absolute().Left(16).Top(16).Size(44, 44).Background(a.colors.card))
			a.bottomNavigation(c)
		})
	}
}

func glassImageRegion(im *image.RGBA, rect image.Rectangle) []byte {
	var out []byte
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		from := im.PixOffset(rect.Min.X, y)
		out = append(out, im.Pix[from:from+rect.Dx()*4]...)
	}
	return out
}

func TestNativeGlassSoftwareFallbackAndLivePreferences(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled = true
	a.cfg.Theme = "light"
	tt := ui.NewTester(glassReferenceView(a), 380, 400)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Advance(time.Second)
	control := image.Rect(16, 16, 60, 60)
	dock := image.Rect(21, 317, 359, 379)
	plainControl, plainDock := glassImageRegion(tt.Image(), control), glassImageRegion(tt.Image(), dock)
	a.cfg.LiquidGlass = "standard"
	tt.Frame()
	glassControl, glassDock := glassImageRegion(tt.Image(), control), glassImageRegion(tt.Image(), dock)
	if bytes.Equal(plainControl, glassControl) || bytes.Equal(plainDock, glassDock) {
		t.Fatal("headless software renderer did not paint the native glass")
	}
	a.cfg.LiquidGlass = "ultra"
	tt.Frame()
	if !bytes.Equal(glassDock, glassImageRegion(tt.Image(), dock)) {
		t.Fatal("preserved source quality choice unexpectedly changed MyGo optics")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, HighContrast: true})
	if a.cfg.LiquidGlass != "ultra" || !bytes.Equal(plainDock, glassImageRegion(tt.Image(), dock)) || !bytes.Equal(plainControl, glassImageRegion(tt.Image(), control)) {
		t.Fatal("accessibility fallback changed the preference or failed to restore opaque controls")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if !bytes.Equal(glassDock, glassImageRegion(tt.Image(), dock)) {
		t.Fatal("glass did not return after the desktop preference changed")
	}
	if err := tt.Click(a.t("proxies")); err != nil || a.page != "proxies" {
		t.Fatal("glass blocked dock interaction", err, a.page)
	}
	tt.Advance(time.Second)
	if a.navigationDock.Lens.Value != a.navigationDock.Lens.Target {
		t.Fatal("reduced motion did not settle the dock")
	}
	a.cfg.LiquidGlass = "none"
	tt.Frame()
	a.cfg.LiquidGlass = "standard"
	tt.Frame()
	if a.page != "proxies" {
		t.Fatal("appearance rebuild lost navigation state")
	}
}

func TestGlassDockPlatterFade(t *testing.T) {
	a := testApp(t)
	a.colors.glassPlatter = ui.Hex("#d8e2ffd9")
	a.colors.text = ui.RGB(0, 0, 0)
	fallback := ui.RGB(200, 200, 200)
	for _, item := range []struct {
		lift  float32
		alpha uint8
	}{{-1, 217}, {0, 217}, {.5, 109}, {1, 0}, {2, 0}} {
		got := a.dockSelectionColor(fallback, item.lift, true)
		if got.R != 216 || got.G != 226 || got.B != 255 || got.A != item.alpha {
			t.Fatal("source platter did not fade as the lens lifted", item, got)
		}
	}
	if got := a.dockSelectionColor(fallback, 0, false); got != fallback {
		t.Fatal("disabled glass changed the default selection", got)
	}
}

func TestNativeGlassReferenceScreens(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, held := range []bool{false, true} {
			name := "glass-dock-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + map[bool]string{false: "rest", true: "held"}[held]
			t.Run(name, func(t *testing.T) {
				a := populatedApp(t)
				a.frameEnabled = true
				a.cfg.Theme = map[bool]string{false: "light", true: "dark"}[dark]
				a.cfg.LiquidGlass = "standard"
				tt := ui.NewTester(glassReferenceView(a), 380, 400)
				tt.Advance(time.Second)
				if bounds, ok := tt.Find(a.t("navigationBarStyle")); !ok || bounds.X != 21 || bounds.Y != 317 || bounds.W != 338 || bounds.H != 62 {
					t.Fatal("glass changed source dock geometry", bounds)
				}
				if held {
					tt.Press(66, 348)
					tt.Advance(600 * time.Millisecond)
					if !a.navigationDock.Pressed || a.navigationDock.Lift.Value < .975 {
						t.Fatal("glass reference did not reach its lifted lens")
					}
				}
				if dir := os.Getenv("FLCLASH_SNAPSHOT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(dir, name+".png")
					f, err := os.Create(path)
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, tt.Image())
					_ = f.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
				if held {
					tt.Release(66, 348)
					tt.Advance(time.Second)
				}
			})
		}
	}
}
