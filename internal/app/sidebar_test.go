package app

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSidebarGeometryAndExpansion(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-layout-metrics.json")
	if err != nil {
		t.Fatal(err)
	}
	var references map[string]struct {
		SidebarButtons [][4]float32 `json:"sidebarButtons"`
	}
	if err := json.Unmarshal(data, &references); err != nil {
		t.Fatal(err)
	}
	for _, scale := range []float32{0.8, 1, 1.4} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			a := testApp(t)
			a.cfg.Scale = scale
			tt := ui.NewTester(a.View, 1200, 800)
			row, ok := tt.Find(a.t("dashboard"))
			name := map[float32]string{0.8: "proxies-scaled-down", 1: "core-connected", 1.4: "proxies-scaled-up"}[scale]
			want := references[name].SidebarButtons[1]
			if !ok || row != (ui.Rect{X: want[0], Y: want[1], W: want[2], H: want[3]}) {
				t.Fatal("sidebar no longer follows the original row geometry", row)
			}
			if err := tt.Click(a.t("showLess")); err != nil {
				t.Fatal(err)
			}
			tt.Advance(125 * time.Millisecond)
			if a.sidebarSize() <= 48 || a.sidebarSize() >= 220 {
				t.Fatal("sidebar snapped instead of animating", a.sidebarSize())
			}
			tt.Advance(150 * time.Millisecond)
			if a.sidebarSize() != 48 {
				t.Fatal("sidebar did not finish collapsing", a.sidebarSize())
			}
			if row, ok := tt.Find(a.t("proxies")); !ok || row.W != 40 {
				t.Fatal("collapsed navigation lost its labelled hit target", row)
			}
			if err := tt.Click(a.t("proxies")); err != nil || a.page != "proxies" {
				t.Fatal("collapsed destination was not selectable", err)
			}
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			if err := tt.Click(a.t("showMore")); err != nil {
				t.Fatal(err)
			}
			if a.sidebarSize() != 220 {
				t.Fatal("reduced-motion expansion animated", a.sidebarSize())
			}
		})
	}
}

func TestSidebarSelectionStretchesAndReverses(t *testing.T) {
	a := testApp(t)
	index := 0
	var top, height float32
	tt := ui.NewTester(func(c *ui.Context) { top, height = a.sidebarIndicator(c, index, 44) }, 300, 300)
	if top != 14 || height != 16 {
		t.Fatal("initial selection geometry", top, height)
	}
	index = 2
	tt.Frame()
	tt.Advance(160 * time.Millisecond)
	if top < 14 || top >= 102 || height <= 16 || top+height > 118 {
		t.Fatal("selection did not stretch toward the destination", top, height)
	}
	tt.Advance(200 * time.Millisecond)
	if math.Abs(float64(top-102)) > 0.01 || math.Abs(float64(height-16)) > 0.01 {
		t.Fatal("selection did not settle", top, height)
	}
	index = 0
	tt.Frame()
	tt.Advance(160 * time.Millisecond)
	if top <= 14 || top >= 102 || height <= 16 {
		t.Fatal("reverse selection did not stretch", top, height)
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if top != 14 || height != 16 {
		t.Fatal("reduced-motion selection did not settle", top, height)
	}
}

func TestDashboardDensityAgainstOriginalCardBounds(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-layout-metrics.json")
	if err != nil {
		t.Fatal(err)
	}
	var references map[string]struct {
		DashboardCards [][4]float32 `json:"dashboardCards"`
	}
	if err := json.Unmarshal(data, &references); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name          string
		width, height int
	}{
		{"dashboard", 1200, 800},
		{"dashboard-medium", 1440, 860},
		{"dashboard-standard", 1440, 1000},
	} {
		t.Run(item.name, func(t *testing.T) {
			a := populatedApp(t)
			tt := ui.NewTester(a.View, item.width, item.height)
			r, ok := tt.Find(a.t("status"))
			want := references[item.name].DashboardCards[0]
			if !ok || r != (ui.Rect{X: want[0], Y: want[1], W: want[2], H: want[3]}) {
				t.Fatal("dashboard density differs from original card bounds", r, want)
			}
		})
	}
}
