package app

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestCompactPickerBoundsAndSnapDrag(t *testing.T) {
	a := populatedApp(t)
	a.openProfilePicker()
	tt := ui.NewTester(func(c *ui.Context) { a.theme(c); ui.Box(c).Fill().Background(a.colors.page); a.dashboardPickerSheet(c) }, 560, 800)
	r, found := tt.Find("Example")
	if !found || r.X != 16 || r.Y != 418 || r.W != 528 || r.H != 64 {
		t.Fatal("compact profile bounds differ from source", r)
	}
	if err := tt.Click(a.t("close")); err != nil || a.picker.Open {
		t.Fatal("compact close was intercepted by drag", err)
	}
	a.openProxyPicker("")
	tt.Frame()
	query, found := tt.Find(a.t("search"))
	if !found || query.X != 16 || query.Y != 736 || query.W != 528 || query.H != 48 {
		t.Fatal("compact search is not docked", query)
	}
	tt.Press(280, 357)
	tt.Advance(16 * time.Millisecond)
	tt.Move(280, 257)
	tt.Advance(16 * time.Millisecond)
	tt.Move(280, 157)
	tt.Advance(16 * time.Millisecond)
	tt.Move(280, 87)
	tt.Release(280, 87)
	tt.Advance(2 * time.Second)
	s := a.snapSheet("dashboard-picker")
	if !a.picker.Open || s.Stop != 1 || math.Abs(s.Height.Value-720) > .01 {
		t.Fatal("drag did not reach upper detent", s.Stop, s.Height.Value)
	}
	tt.Press(280, 87)
	tt.Advance(16 * time.Millisecond)
	tt.Move(280, 287)
	tt.Advance(16 * time.Millisecond)
	tt.Move(280, 587)
	tt.Advance(16 * time.Millisecond)
	tt.Move(280, 787)
	tt.Release(280, 787)
	if a.picker.Open {
		t.Fatal("downward throw did not dismiss sheet")
	}
}

func TestSheetLaunchDropsReverseThrowsAndLimitsForwardThrows(t *testing.T) {
	for _, row := range []struct{ distance, velocity float64 }{{0, 100}, {100, 0}, {100, -100}, {-100, 100}} {
		if v, ratio := sheetLaunch(row.distance, row.velocity); v != 0 || ratio != 1 {
			t.Fatal("reverse throw retained", row, v, ratio)
		}
	}
	w := math.Sqrt(1000.0 / 3)
	if v, ratio := sheetLaunch(100, 1e6); math.Abs(v-.75*w*100) > 1e-9 || math.Abs(ratio-.8) > 1e-9 {
		t.Fatal("throw cap differs", v, ratio)
	}
	if sheetRubberBand(-1, 100) != 0 || sheetRubberBand(100, 0) != 0 || math.Abs(sheetRubberBand(100, 100)-100*(1-1/1.55)) > 1e-9 {
		t.Fatal("rubber band differs")
	}
}

func TestDashboardProfilePickerRetainsPageAndSelection(t *testing.T) {
	a := populatedApp(t)
	a.cfg.Profiles = append(a.cfg.Profiles, store.Profile{ID: "second", Name: "Second profile"})
	a.openProfilePicker()
	tt := ui.NewTester(a.View, 1200, 800)
	if err := tt.Click("Second profile"); err != nil {
		t.Fatal(err)
	}
	if !a.picker.Open || a.cfg.Active != "second" || a.page != "dashboard" || len(a.history) != 0 {
		t.Fatal("picker lost the dashboard or dismissed on selection", a.page, a.cfg.Active)
	}
	if !tt.HasText(a.t("totalTraffic")) {
		t.Fatal("dashboard was unmounted")
	}
	tt.Key(0, ui.KeyEscape)
	if a.picker.Open || a.page != "dashboard" {
		t.Fatal("Escape did not dismiss picker")
	}
	a.openProfilePicker()
	tt.Frame()
	tt.ClickAt(100, 400)
	if a.picker.Open {
		t.Fatal("backdrop did not dismiss picker")
	}
}

func TestDashboardProxyPickerRestoresPageSearchAndScroll(t *testing.T) {
	for _, style := range []string{"list", "tab"} {
		t.Run(style, func(t *testing.T) {
			a := populatedApp(t)
			a.cfg.ProxyStyle = style
			a.proxyQuery, a.proxySearch = "old query", true
			a.proxyScroll.Y = 137
			a.openProxyPicker("Streaming")
			tt := ui.NewTester(a.View, 1200, 800)
			if a.page != "dashboard" || a.group != "Streaming" || a.picker.Proxy.proxyQuery != "" || a.picker.Proxy.proxySearch {
				t.Fatal("picker did not isolate search")
			}
			if !tt.HasText("Singapore 01") {
				t.Fatal("picker hid proxy content")
			}
			if err := tt.Click(a.t("search")); err != nil {
				t.Fatal(err)
			}
			a.picker.Proxy.proxyQuery = "Japan"
			tt.Frame()
			if !tt.HasText("Japan 01") {
				t.Fatal("picker search did not use proxy query")
			}
			tt.Key(0, ui.KeyEscape)
			if !a.picker.Open || a.picker.Proxy.proxyQuery != "" {
				t.Fatal("search Escape should clear its query first")
			}
			tt.Key(0, ui.KeyEscape)
			if a.picker.Open || a.proxyQuery != "old query" || !a.proxySearch || a.proxyScroll.Y != 137 || a.page != "dashboard" {
				t.Fatal("picker did not restore page state", a.proxyQuery, a.proxyScroll.Y)
			}
		})
	}
}

func TestProfileGridVirtualizesAndFollowsSourceColumns(t *testing.T) {
	for _, c := range []struct {
		width, spacing, scale float32
		want                  int
	}{{300, 16, 1, 1}, {615, 16, 1, 1}, {616, 16, 1, 2}, {1000, 16, 1, 3}, {1000, 16, 1.4, 2}, {600, 8, .8, 2}} {
		if got := profileColumns(c.width, c.spacing, c.scale); got != c.want {
			t.Fatal(c, got)
		}
	}
	a := testApp(t)
	for i := range 10000 {
		a.cfg.Profiles = append(a.cfg.Profiles, store.Profile{ID: number(i), Name: "Profile " + number(i)})
	}
	a.openProfilePicker()
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("Profile 0") || tt.HasText("Profile 9999") || len(tt.Texts()) > 400 {
		t.Fatal("profile picker is not virtualized", len(tt.Texts()))
	}
}

func TestNativePickerReferenceScreens(t *testing.T) {
	for _, name := range []string{"profile-picker", "profile-picker-many", "profile-grid", "proxy-picker", "proxy-picker-tab", "profile-picker-small", "profile-picker-small-many", "proxy-picker-small", "profile-details-small"} {
		for _, dark := range []bool{false, true} {
			t.Run(name+map[bool]string{true: "-dark", false: "-light"}[dark], func(t *testing.T) {
				a := populatedApp(t)
				a.cfg.Profiles[0].Total, a.cfg.Profiles[0].Expire = 0, 0
				a.cfg.Profiles[0].Updated = time.Time{}
				if dark {
					a.cfg.Theme = "dark"
				}
				if name == "profile-picker-many" || name == "profile-picker-small-many" || name == "profile-grid" {
					a.cfg.Profiles = append(a.cfg.Profiles, store.Profile{ID: "second", Name: "Local configuration"}, store.Profile{ID: "third", Name: "Second subscription", URL: "https://example.org/subscription", Upload: 1 << 30, Download: 41 << 30, Total: 100 << 30, Expire: 1893456000})
				}
				if name == "profile-grid" {
					a.page = "profiles"
				} else if name == "profile-picker" || name == "profile-picker-many" || name == "profile-picker-small" || name == "profile-picker-small-many" {
					a.openProfilePicker()
				} else if name == "profile-details-small" {
					a.openProfileDetails()
				} else {
					if name == "proxy-picker-tab" {
						a.cfg.ProxyStyle = "tab"
					}
					a.openProxyPicker("")
				}
				width := 1200
				view := a.View
				if strings.Contains(name, "small") {
					width = 560
					view = func(c *ui.Context) {
						a.flushUpdates()
						a.theme(c)
						ui.Box(c).Fill().Background(a.colors.page)
						a.dashboardPickerSheet(c)
						a.profileDetailSheet(c)
					}
				}
				tt := ui.NewTester(view, width, 800)
				if dir := os.Getenv("FLCLASH_SNAPSHOT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					file, err := os.Create(filepath.Join(dir, name+map[bool]string{true: "-dark", false: "-light"}[dark]+".png"))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(file, tt.Image())
					closeErr := file.Close()
					if err != nil || closeErr != nil {
						t.Fatal(err, closeErr)
					}
				}
			})
		}
	}
}
