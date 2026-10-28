package app

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestWindowCaptionReferencesAndHitTargets(t *testing.T) {
	for _, name := range []string{"caption-normal", "caption-maximized", "caption-pinned-fullscreen", "caption-close-hover"} {
		for _, dark := range []bool{false, true} {
			t.Run(name+map[bool]string{true: "-dark", false: "-light"}[dark], func(t *testing.T) {
				a := testApp(t)
				if dark {
					a.cfg.Theme = "dark"
				}
				a.caption = windowCaptionState{Maximized: name == "caption-maximized", Pinned: name == "caption-pinned-fullscreen", FullScreen: name == "caption-pinned-fullscreen"}
				tt := ui.NewTester(func(c *ui.Context) {
					a.theme(c)
					a.frameHeight = 32
					ui.Box(c).Fill().Children(func() { a.captionBar(c) })
				}, 1200, 32)
				labels := []string{a.t("pinWindow"), a.t("minimize"), a.t("maximize"), a.t("close")}
				if a.caption.Pinned {
					labels[0] = a.t("unpinWindow")
				}
				if a.caption.FullScreen {
					labels[2] = a.t("exitFullScreen")
				} else if a.caption.Maximized {
					labels[2] = a.t("unmaximize")
				}
				for i, label := range labels {
					r, ok := tt.Find(label)
					width := float32(46)
					x := float32(1062 + (i-1)*46)
					if i == 0 {
						width, x = 32, 1030
					}
					if !ok || r.X != x || r.Y != 0 || r.W != width || r.H != 32 {
						t.Fatal(label, "caption geometry differs from source", r)
					}
				}
				if name == "caption-close-hover" {
					tt.Move(1177, 16)
					tt.Advance(300 * time.Millisecond)
					red, green, blue, _ := tt.Image().At(1156, 2).RGBA()
					if uint8(red>>8) != a.colors.danger.R || uint8(green>>8) != a.colors.danger.G || uint8(blue>>8) != a.colors.danger.B {
						t.Fatal("close hover lost its error fill")
					}
				}
				if dir := os.Getenv("FLCLASH_SNAPSHOT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					f, err := os.Create(filepath.Join(dir, t.Name()[len("TestWindowCaptionReferencesAndHitTargets/"):]+".png"))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, tt.Image())
					_ = f.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestDesktopCaptionKeepsSheetsInBodyAndRespectsMacLayout(t *testing.T) {
	for _, platform := range []string{"linux", "windows", "darwin"} {
		for _, width := range []int{820, 1200} {
			a := populatedApp(t)
			a.frameEnabled, a.framePlatform = true, platform
			a.openProfileDetails()
			tt := ui.NewTester(a.View, width, 1000)
			want := float32(32)
			if platform == "darwin" {
				want = 0
			}
			if a.frameHeight != want {
				t.Fatal(platform, width, a.frameHeight, want)
			}
			r, ok := tt.Find(a.t("runtimeConfig"))
			if !ok || r.Y != want+14 {
				t.Fatal("sheet header escaped the body", platform, r, want)
			}
			if platform != "darwin" {
				caption, ok := tt.Find(a.t("minimize"))
				if !ok || caption.Y != 0 || caption.H != 32 {
					t.Fatal("sheet hid the caption bar", caption)
				}
			}
			tt.Key(0, ui.KeyEscape)
			if a.profileDetail.Open {
				t.Fatal("caption frame broke sheet Escape")
			}
		}
	}
}

func TestCaptionGeometryFixtureIsOriginal(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-caption-metrics.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference struct{ CaptionButtons [][4]float32 }
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	want := [][4]float32{{1030, 0, 32, 32}, {1062, 0, 46, 32}, {1108, 0, 46, 32}, {1154, 0, 46, 32}}
	if len(reference.CaptionButtons) != len(want) {
		t.Fatal(reference)
	}
	for i, r := range want {
		if reference.CaptionButtons[i] != r {
			t.Fatal("source caption fixture changed", reference)
		}
	}
}

type captionFixtureHost struct {
	pinned                                       bool
	minimized, closed, maximized, leftFullScreen int
}

func (h *captionFixtureHost) SetAlwaysOnTop(v bool) { h.pinned = v }
func (h *captionFixtureHost) IsAlwaysOnTop() bool   { return h.pinned }
func (h *captionFixtureHost) Minimize()             { h.minimized++ }
func (h *captionFixtureHost) Close()                { h.closed++ }
func (h *captionFixtureHost) ToggleMaximize()       { h.maximized++ }
func (h *captionFixtureHost) SetFullScreen(v bool) {
	if !v {
		h.leftFullScreen++
	}
}

func TestCaptionControlsStayUsableAboveSheetAndFollowNativeEvents(t *testing.T) {
	a := populatedApp(t)
	host := &captionFixtureHost{}
	a.captionHost = host
	a.frameEnabled = true
	a.openProfileDetails()
	tt := ui.NewTester(a.View, 1200, 800)
	if err := tt.Click(a.t("pinWindow")); err != nil || !host.pinned || !a.caption.Pinned || !a.profileDetail.Open {
		t.Fatal("pin unavailable above sheet", err)
	}
	if err := tt.Click(a.t("unpinWindow")); err != nil || host.pinned || a.caption.Pinned {
		t.Fatal("unpin did not follow native state", err)
	}
	if err := tt.Click(a.t("minimize")); err != nil || host.minimized != 1 {
		t.Fatal("minimize unavailable above sheet", err)
	}
	if err := tt.Click(a.t("maximize")); err != nil || host.maximized != 1 || a.caption.Maximized {
		t.Fatal("maximize became optimistic", err)
	}
	a.caption.Maximized = true
	tt.Frame()
	if err := tt.Click(a.t("unmaximize")); err != nil || host.maximized != 2 {
		t.Fatal("native maximize event did not change the action", err)
	}
	a.caption.FullScreen = true
	tt.Frame()
	if err := tt.Click(a.t("exitFullScreen")); err != nil || host.leftFullScreen != 1 {
		t.Fatal("restore did not leave full screen", err)
	}
	// The caption's Close and the sheet's Close intentionally have the same
	// accessible name. Click the original caption rectangle explicitly.
	tt.ClickAt(1177, 16)
	if host.closed != 1 || !a.profileDetail.Open {
		t.Fatal("caption close was intercepted by sheet")
	}
}
