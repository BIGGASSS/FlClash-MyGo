package app

import (
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/desktop"
	"github.com/egoist/mygo/ui"
)

func dashboardGeometryRect(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing dashboard element %q", label)
	}
	return r
}

func dashboardGeometryNear(t *testing.T, name string, got, want float32) {
	t.Helper()
	if math.IsNaN(float64(got)) || math.Abs(float64(got-want)) > 0.02 {
		t.Errorf("%s: got %.3f, want %.3f", name, got, want)
	}
}

// The outer rectangles were measured by port_reference_test.dart from the
// pinned Flutter widgets. Check all the cards whose interiors this port changes,
// not only the status card used by the earlier density regression test.
func TestDashboardGeometryKeepsOriginalGrid(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-layout-metrics.json")
	if err != nil {
		t.Fatal(err)
	}
	var references map[string]struct {
		Cards [][4]float32 `json:"dashboardCards"`
	}
	if err := json.Unmarshal(data, &references); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		w, h int
	}{{"dashboard", 1200, 800}, {"dashboard-medium", 1440, 860}, {"dashboard-standard", 1440, 1000}} {
		t.Run(item.name, func(t *testing.T) {
			a := populatedApp(t)
			tt := ui.NewTester(a.View, item.w, item.h)
			for _, card := range []struct {
				index int
				key   string
			}{{0, "status"}, {1, "upload"}, {2, "download"}, {6, "intranetIP"}, {7, "profile"}, {9, "memoryInfo"}} {
				got := dashboardGeometryRect(t, tt, a.t(card.key))
				want := references[item.name].Cards[card.index]
				if got != (ui.Rect{X: want[0], Y: want[1], W: want[2], H: want[3]}) {
					t.Errorf("%s moved out of the original grid: %v, want %v", card.key, got, want)
				}
			}
		})
	}
}

// Expected positions below are derived from current_outbound.dart's
// _OutboundMetrics, card.dart's header/padding and typography.dart's line
// heights, rather than fitted to screenshots. Resizing the same tester also
// guards against using stale element bounds for width-dependent decisions.
func TestDashboardGeometrySourceCardFormulas(t *testing.T) {
	for _, scale := range []float32{0.8, 1, 1.4} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			a := populatedApp(t)
			a.cfg.Scale = scale
			tt := ui.NewTester(a.View, 1440, 1000)
			for _, size := range [][2]int{{1440, 1000}, {1440, 860}, {1200, 640}, {1000, 800}, {1440, 1200}, {2800, 1400}} {
				t.Run(fmt.Sprint(size), func(t *testing.T) {
					tt.SetSize(size[0], size[1])
					layoutScale := max(scale, 1)
					room := float32(size[0]-220) / layoutScale
					if room < 640 {
						t.Skip("source selects the narrow dashboard at this text scale; covered by dashboard_narrow_test.go")
					}
					compact := float32(size[1])/layoutScale < 824 || room >= 860 && room < 1040
					py, px := float32(20), float32(22)
					ipTop, badge, ipLeading, button, slot := float32(10), float32(48), float32(34), float32(104), float32(120)
					trafficTop, trafficLeading := float32(10), float32(44)
					nameLeading := float32(22)
					if compact {
						py, px = 16, 20
						ipTop, badge, ipLeading, button, slot = 8, 40, 28, 84, 100
						trafficTop, trafficLeading, nameLeading = 6, 41, 21
					}
					line := func(leading float32) float32 { return float32(math.Round(float64(leading * scale))) }
					status := dashboardGeometryRect(t, tt, a.t("status"))
					start := dashboardGeometryRect(t, tt, a.t("stop"))
					dashboardGeometryNear(t, "start width", start.W, button)
					dashboardGeometryNear(t, "start height", start.H, button)
					dashboardGeometryNear(t, "start slot", start.X, status.X+status.W-px-slot+(slot-button)/2)
					dashboardGeometryNear(t, "start centered on entire card", start.Y, status.Y+(status.H-button)/2)
					exitTop := status.Y + py + 24*layoutScale + ipTop
					exitHeight := max(badge, ipLeading*layoutScale+4+24)
					refresh := dashboardGeometryRect(t, tt, a.t("refresh"))
					dashboardGeometryNear(t, "country size", refresh.W, badge)
					dashboardGeometryNear(t, "country x", refresh.X, status.X+px)
					dashboardGeometryNear(t, "country vertically centered on exit row", refresh.Y, exitTop+(exitHeight-badge)/2)
					ip := dashboardGeometryRect(t, tt, a.ip)
					dashboardGeometryNear(t, "IP top", ip.Y, exitTop)
					dashboardGeometryNear(t, "IP x", ip.X, status.X+px+badge+12)
					dashboardGeometryNear(t, "IP allocated line", ip.H, ipLeading*layoutScale)
					runtime := dashboardGeometryRect(t, tt, a.t("runTime"))
					dashboardGeometryNear(t, "status cells pinned to foot", runtime.Y, status.Y+status.H-py-line(16)-2-line(20))
					dashboardGeometryNear(t, "status cell label leading", runtime.H, line(16))

					// traffic.dart: a fixed-height chart behind a column of text;
					// the large figure has the source's 41/44-point line.
					traffic := dashboardGeometryRect(t, tt, a.t("upload"))
					figure := dashboardGeometryRect(t, tt, bytesText(a.sample.Traffic.Up)+"/s")
					dashboardGeometryNear(t, "traffic figure top", figure.Y, traffic.Y+py+24*layoutScale+trafficTop)
					dashboardGeometryNear(t, "traffic figure line", figure.H, line(trafficLeading))

					// profiles.dart: compact metadata shares the name row;
					// standard has an additional caption line. The usage meter
					// always ends in a 12-point wave with a six-point top gap.
					// In a short two-column window these cards are below the
					// viewport. Mount them separately at the source column size:
					// Tester.Find intentionally clips text bounds to the window.
					columns := float32(4)
					if room < 860 {
						columns = 2
					}
					gap := 16 * min(float32(1), 1+(scale-1)/2)
					column := min((float32(size[0]-220)-40-gap*(columns-1))/columns, 440*layoutScale)
					infoHeight := float32(176) * layoutScale
					if compact {
						infoHeight = 152 * layoutScale
					}
					infoTT := ui.NewTester(func(c *ui.Context) {
						a.theme(c)
						ui.Row(c).FillWidth().Gap(gap).Children(func() {
							a.intranetCard(c).Width(column).Height(infoHeight).Shrink(0)
							a.profileCard(c).Width(column).Height(infoHeight).Shrink(0)
						})
					}, size[0], size[1])
					profile := dashboardGeometryRect(t, infoTT, a.t("profile"))
					name := dashboardGeometryRect(t, infoTT, a.cfg.Profiles[0].Name)
					dashboardGeometryNear(t, "profile name top", name.Y, profile.Y+py+24*layoutScale+10)
					dashboardGeometryNear(t, "profile name leading", name.H, line(nameLeading))
					foundUsage := false
					for _, text := range infoTT.Texts() {
						if strings.HasPrefix(text, "42%") {
							foundUsage = true
							percent := dashboardGeometryRect(t, infoTT, text)
							dashboardGeometryNear(t, "profile meter pinned to foot", percent.Y, profile.Y+profile.H-py-12-6-line(nameLeading))
						}
					}
					if !foundUsage {
						t.Fatal("profile usage disappeared")
					}
					if infoTT.HasText(a.messageText("profileUpdatedAt", map[string]any{"time": a.updatedAgo(a.cfg.Profiles[0].Updated)})) && compact {
						t.Error("compact profile retained the standard update-time row")
					}
					lan := dashboardGeometryRect(t, infoTT, a.t("intranetIP"))
					lanRow := dashboardGeometryRect(t, infoTT, a.t("allowLan"))
					vertical, icon := float32(10), float32(38)
					if compact {
						vertical, icon = 6, 32
					}
					// In these fixtures text is at least as tall as a hidden
					// icon, or the row has enough width to show its icon.
					rowHeight := max(float32(32), icon, line(20)+line(16)) + vertical*2
					dashboardGeometryNear(t, "LAN row height", lanRow.H, rowHeight)
					dashboardGeometryNear(t, "LAN row pinned to foot", lanRow.Y+lanRow.H, lan.Y+lan.H-py)

					if a.engine != nil || a.window != nil {
						t.Fatal("headless geometry test reached an integration host")
					}
				})
			}
		})
	}
}

func TestDashboardGeometryProfileCountsAndSuspendedState(t *testing.T) {
	for _, size := range [][2]int{{1200, 640}, {1440, 1000}} {
		a := populatedApp(t)
		a.cfg.Profiles[0].Total = 0
		a.sample.Counts = &ConfigCounts{Groups: 31, Proxies: 547, Rules: 8642}
		tt := ui.NewTester(a.View, size[0], size[1])
		card := dashboardGeometryRect(t, tt, a.t("profile"))
		py, leading := float32(20), float32(22)
		if size[0] == 1200 {
			py, leading = 16, 21
		}
		for _, count := range []string{"31", "547", "8,642"} {
			r := dashboardGeometryRect(t, tt, count)
			dashboardGeometryNear(t, "config counts bottom block", r.Y, card.Y+card.H-py-16-leading)
		}
	}

	// Build only the status card; no watchers, engine, Wi-Fi reader, or core.
	a := populatedApp(t)
	a.safe = false
	a.cfg.ExcludeSSIDs = []string{"Home"}
	a.onDemand.Known = true
	a.onDemand.State = desktop.WiFiState{Connected: true, SSID: "Home"}
	tt := ui.NewTester(func(c *ui.Context) { a.theme(c); a.statusCard(c).Size(462, 176) }, 1200, 800)
	if !tt.HasText(a.t("suspended")) || tt.HasText(a.t("running")) {
		t.Fatal("wide status lost wantRunning && onDemandSuspended priority")
	}
	labels := 0
	for _, label := range tt.Texts() {
		if label == a.t("suspended") {
			labels++
		}
	}
	if labels < 2 {
		t.Fatal("suspended state lost either the status badge or start-control label")
	}
	a.wantRunning = false
	a.status = "stopped"
	tt.Frame()
	if tt.HasText(a.t("suspended")) || !tt.HasText(a.t("start")) {
		t.Fatal("stale excluded network overrode the latest stop")
	}
}

func TestDashboardGeometryConfigStatsScaleAsOneRow(t *testing.T) {
	a := populatedApp(t)
	a.cfg.Locale = "ru"
	a.cfg.Profiles[0].Total = 0
	a.sample.Counts = &ConfigCounts{Groups: 31, Proxies: 547, Rules: 8642}
	var factor float32
	// 860 is the first four-column width. Source compact columns are 193
	// points there, leaving 153 after padding; Russian stats need scaleDown.
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		natural := float32(32)
		for i, key := range []string{"proxyGroup", "proxies", "rules"} {
			countWidth, _ := c.MeasureText(0, ui.Span{Text: []string{"31", "547", "8,642"}[i], Size: 16, Weight: 600})
			labelWidth, _ := c.MeasureText(0, ui.Span{Text: a.t(key), Size: 12})
			natural += max(countWidth, labelWidth)
		}
		factor = min(float32(1), 153/natural)
		a.profileCard(c).Width(193).Height(152)
	}, 1080, 800)
	if factor >= 1 {
		t.Fatal("fixture no longer exercises source FittedBox.scaleDown")
	}
	for _, count := range []string{"31", "547", "8,642"} {
		r := dashboardGeometryRect(t, tt, count)
		dashboardGeometryNear(t, "stats uniformly scaled", r.H, 21*factor)
		dashboardGeometryNear(t, "scaled stats pinned to foot", r.Y, 152-16-(21+16)*factor)
	}
}

func TestDashboardGeometryDensityExcludesCaptionButIncludesToolbar(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled, a.framePlatform = true, "linux"
	tt := ui.NewTester(a.View, 1440, 855)
	r := dashboardGeometryRect(t, tt, a.t("status"))
	if r.H != 176 {
		t.Fatal("823-point body must remain compact", r)
	}
	tt.SetSize(1440, 856)
	r = dashboardGeometryRect(t, tt, a.t("status"))
	if r.H != 208 {
		t.Fatal("824-point body must select standard density, without subtracting its toolbar", r)
	}
}

// card.dart's DashboardFitText uses scaleDown only if the whole string fits
// at 75% or larger; longer IPv6 addresses retain normal-size ellipsis instead.
func TestDashboardGeometryFitTextThreshold(t *testing.T) {
	for _, ratio := range []float32{1, 0.9, 0.75, 0.74} {
		t.Run(fmt.Sprint(ratio), func(t *testing.T) {
			a := testApp(t)
			text := "2001:db8:ffff:1234::1"
			tt := ui.NewTester(func(c *ui.Context) {
				a.theme(c)
				width, _ := c.MeasureText(0, ui.Span{Text: text, Size: 22, Weight: 700})
				ui.Box(c).Width(width * ratio).Height(28).Justify(ui.Center).Children(func() {
					a.dashboardFitText(c, text, 22, 28, width*ratio, "")
				})
			}, 1200, 800)
			wantHeight := float32(28)
			if ratio >= 0.75 {
				wantHeight *= ratio
			}
			r := dashboardGeometryRect(t, tt, text)
			dashboardGeometryNear(t, "fit-text line scale", r.H, wantHeight)
		})
	}
}

// Optional, reproducible native captures. Synthetic providers and temporary
// stores are the same safe fixtures used by the geometry assertions, with no
// system-proxy/TUN actions. Nothing is written into docs/reference.
func TestDashboardGeometrySafeSnapshots(t *testing.T) {
	dir := os.Getenv("FLCLASH_DASHBOARD_GEOMETRY_DIR")
	if dir == "" {
		t.Skip("set FLCLASH_DASHBOARD_GEOMETRY_DIR to save headless card captures")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{1200, 640}, {1000, 800}, {1440, 860}, {1440, 1000}} {
		for _, dark := range []bool{false, true} {
			a := populatedApp(t)
			a.cfg.Theme = map[bool]string{false: "light", true: "dark"}[dark]
			tt := ui.NewTester(a.View, size[0], size[1])
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Advance(time.Second)
			name := fmt.Sprintf("dashboard-%dx%d-%s.png", size[0], size[1], a.cfg.Theme)
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
			}
			if a.engine != nil {
				t.Fatal("snapshot created an engine")
			}
		}
	}
}
