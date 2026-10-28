package app

import (
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/desktop"
	"github.com/egoist/mygo/ui"
)

func TestDashboardSuspendedKeepsSourceWiFiOffGlyph(t *testing.T) {
	a := populatedApp(t)
	a.safe, a.wantRunning = false, true
	a.cfg.ExcludeSSIDs = []string{"Home"}
	a.onDemand.Known = true
	a.onDemand.State = desktop.WiFiState{Connected: true, SSID: "Home"}
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		ui.Box(c).Fill().Center().Children(func() { a.dashboardStartControl(c, 88) })
	}, 240, 240)
	if !tt.HasText(a.t("suspended")) || tt.HasText(a.t("stop")) {
		t.Fatal("suspension was lost from control semantics")
	}
	// Compare with the existing exported filled wifiOff, not a look-alike
	// pause glyph. No core, Wi-Fi backend or engine loop is started here.
	expected := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		ui.Box(c).Fill().Center().Children(func() {
			ui.Box(c).Size(88, 88).Background(a.colors.accent).Center().Children(func() {
				glyphVariant(c, "wifiOff", true).FontSize(88 * .34).TextColor(a.colors.onAccent)
			})
		})
	}, 240, 240)
	gotImage, wantImage := tt.Image(), expected.Image()
	for y := 105; y < 135; y++ {
		for x := 105; x < 135; x++ {
			if gotImage.RGBAAt(x, y) != wantImage.RGBAAt(x, y) {
				t.Fatalf("suspended glyph differs from filled wifiOff at %d,%d", x, y)
			}
		}
	}
	a.onDemand.State = desktop.WiFiState{}
	tt.Frame()
	if !tt.HasText(a.t("stop")) || !a.wantRunning {
		t.Fatal("resume changed the user's run intent")
	}
	// The source glyph tween is remounted at pause, not replayed from an old
	// hidden play value. Check its numerical state separately from raster AA.
	var state dashboardMorphState
	running, suspended := false, false
	var glyph float32
	motion := ui.NewTester(func(c *ui.Context) { _, glyph = state.frame(c, running, suspended) }, 1, 1)
	running = true
	motion.Frame()
	motion.Advance(90 * time.Millisecond)
	suspended = true
	motion.Frame()
	motion.Advance(90 * time.Millisecond)
	suspended = false
	motion.Frame()
	if glyph != 1 {
		t.Fatal("resuming replayed the removed glyph tween", glyph)
	}
}
