package app

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestNarrowCardBoundsMatchOriginalAtSupportedScales(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-layout-metrics.json")
	if err != nil {
		t.Fatal(err)
	}
	var references map[string]struct {
		Scale          float32
		DashboardCards [][4]float32 `json:"dashboardCards"`
	}
	if err := json.Unmarshal(data, &references); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dashboard-narrow", "dashboard-narrow-custom", "dashboard-narrow-scaled-up", "dashboard-narrow-scaled-down"} {
		t.Run(name, func(t *testing.T) {
			a := populatedApp(t)
			a.cfg.Scale = references[name].Scale
			a.engine = &Engine{}
			a.cfg.Profiles[0].Total, a.cfg.Profiles[0].Expire = 0, 0
			a.cfg.Profiles[0].Updated = time.Time{}
			if name == "dashboard-narrow-custom" {
				a.cfg.DashboardWidgets = []string{"currentOutbound", "memoryInfo", "profiles"}
			}
			// A taller viewport exposes the entire natural column to Find, which
			// reports clipped rectangles. Width/text scale still match the source.
			tt := ui.NewTester(a.View, 820, 2400)
			for index, widget := range a.dashboardWidgets() {
				// The profile statistics also contain this caption; the following
				// connection card still checks the group summary's occupied height.
				if widget == "proxyGroups" {
					continue
				}
				r, ok := tt.Find(a.t(dashboardLabels[widget]))
				want := references[name].DashboardCards[index]
				got := [4]float32{r.X, r.Y, r.W, r.H}
				if !ok {
					t.Fatalf("missing card %s", widget)
				}
				for i := range got {
					if math.Abs(float64(got[i]-want[i])) > 0.1 {
						t.Errorf("%s bounds %v, original %v", widget, got, want)
						break
					}
				}
			}
			button, ok := tt.Find(a.t("stop"))
			if !ok || button.W != 88 || button.H != 88 {
				t.Fatal("start control lost its hit target", button)
			}
			red, green, blue, _ := tt.Image().At(int(button.X+button.W/4), int(button.Y+button.H/2)).RGBA()
			if uint8(red>>8) != a.colors.accent.R || uint8(green>>8) != a.colors.accent.G || uint8(blue>>8) != a.colors.accent.B {
				t.Fatal("decoration covered the start control", red, green, blue)
			}
		})
	}
}

func TestNarrowDashboardEditingPersistsAndKeepsStartControl(t *testing.T) {
	a := populatedApp(t)
	a.cfg.DashboardWidgets = []string{"currentOutbound", "vpnButton", "memoryInfo"}
	tt := ui.NewTester(a.View, 820, 800)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if err := tt.Click(a.t("editDashboard")); err != nil || !a.dashboardEditing {
		t.Fatal("narrow dashboard has no editor", err)
	}
	if tt.HasText(a.t("remove") + " " + a.t("status")) {
		t.Fatal("editor can remove the start control")
	}
	if err := tt.Click(a.t("remove") + " " + a.t("memoryInfo")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(a.t("add") + " " + a.t("networkSpeed")); err != nil {
		t.Fatal("removed card did not leave the addable section accessible", err)
	}
	if want := []string{"currentOutbound", "vpnButton", "networkSpeed"}; !reflect.DeepEqual(a.cfg.DashboardWidgets, want) {
		t.Fatal("edit lost the unsupported platform's position", a.cfg.DashboardWidgets)
	}
	speed, ok := tt.Find(a.t("networkSpeed"))
	if !ok {
		t.Fatal("added card is not visible")
	}
	status, ok := tt.Find(a.t("status"))
	if !ok {
		t.Fatal("start card is not visible")
	}
	tt.Press(500, speed.Y+50)
	tt.Move(500, status.Y+5)
	tt.Release(500, status.Y+5)
	if want := []string{"networkSpeed", "currentOutbound", "vpnButton"}; !reflect.DeepEqual(a.cfg.DashboardWidgets, want) {
		t.Fatal("reorder lost a card or unsupported-platform position", a.cfg.DashboardWidgets)
	}
	tt.Key(0, ui.KeyEscape)
	if a.dashboardEditing {
		t.Fatal("Escape did not finish dashboard editing")
	}
	_, saved, err := store.Open(a.store.Dir)
	if err != nil || !reflect.DeepEqual(saved.DashboardWidgets, a.cfg.DashboardWidgets) {
		t.Fatal("editing did not save the dashboard", saved.DashboardWidgets, err)
	}
}

func TestNarrowDashboardResizeEndsEditingAndUsesDesktopLayout(t *testing.T) {
	a := populatedApp(t)
	tt := ui.NewTester(a.View, 820, 800)
	if err := tt.Click(a.t("editDashboard")); err != nil {
		t.Fatal(err)
	}
	tt.SetSize(1200, 800)
	if a.dashboardEditing {
		t.Fatal("wide desktop retained narrow editing")
	}
	if _, ok := tt.Find(a.t("editDashboard")); ok {
		t.Fatal("wide desktop exposes the narrow layout editor")
	}
}
