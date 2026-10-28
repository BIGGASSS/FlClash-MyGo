package app

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNavigationSpringCurvesMatchFlutter(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-navigation-springs.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frequency, Ratio float64
		Samples          []struct {
			MS              int
			Value, Velocity float64
		}
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		start := time.Unix(1700000000, 0)
		s := springValue{Value: -1.2, Velocity: .7, Target: 2.3, Spec: springSpec{row.Frequency, row.Ratio}, At: start}
		for _, sample := range row.Samples {
			s.step(start.Add(time.Duration(sample.MS) * time.Millisecond))
			if math.Abs(s.Value-sample.Value) > 1e-10 || math.Abs(s.Velocity-sample.Velocity) > 1e-10 {
				t.Fatalf("spring %v at %d: %.12f/%.12f, source %.12f/%.12f", row.Frequency, sample.MS, s.Value, s.Velocity, sample.Value, sample.Velocity)
			}
		}
	}
}

func TestNativeNavigationReferenceScreens(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-dock-bounds.json")
	if err != nil {
		t.Fatal(err)
	}
	var bounds map[string][4]float32
	if err := json.Unmarshal(data, &bounds); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dock-380", "dock-600", "dock-scaled-up", "dock-empty", "dock-docked", "sidebar-laptop", "sidebar-overlay"} {
		for _, dark := range []bool{false, true} {
			t.Run(name+map[bool]string{true: "-dark", false: "-light"}[dark], func(t *testing.T) {
				a := populatedApp(t)
				a.safe = false
				a.frameEnabled = true
				width, height := 380, 400
				if strings.HasPrefix(name, "sidebar") {
					width, height = 820, 800
				}
				if name == "dock-600" {
					width = 600
				}
				if name == "dock-scaled-up" {
					a.cfg.Scale = 1.4
				}
				if name == "dock-empty" {
					a.cfg.Profiles = nil
					a.sample.Proxies.All = nil
				}
				if name == "dock-docked" {
					a.cfg.FloatingNavigation = false
				}
				if dark {
					a.cfg.Theme = "dark"
				}
				tt := ui.NewTester(func(c *ui.Context) {
					a.theme(c)
					ui.Box(c).Fill().Background(a.colors.page).Children(func() {
						if strings.HasPrefix(name, "sidebar") {
							a.sidebar(c)
						} else {
							a.bottomNavigation(c)
						}
					})
					a.sidebarOverlayView(c)
				}, width, height)
				if name == "sidebar-overlay" {
					if err := tt.Click(a.t("showMore")); err != nil {
						t.Fatal(err)
					}
					tt.Advance(300 * time.Millisecond)
					tt.Move(600, 400)
					tt.Advance(300 * time.Millisecond)
				}
				if want, ok := bounds[name]; ok {
					r, found := tt.Find(a.t("navigationBarStyle"))
					if !found || [4]float32{r.X, r.Y, r.W, r.H} != want {
						t.Fatal("dock bounds differ from original", r, want)
					}
				}
				if dir := os.Getenv("FLCLASH_SNAPSHOT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(dir, t.Name()[len("TestNativeNavigationReferenceScreens/"):]+".png")
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
			})
		}
	}
}
