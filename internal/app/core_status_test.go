package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestCoreStatusUsesConnectionAndConfirmsRestart(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText(a.t("disconnected")) {
		t.Fatal("disconnected core was shown as healthy")
	}
	if err := tt.Click(a.t("coreStatus")); err != nil || !a.confirmOpen || a.confirmText != a.t("restartCoreTip") {
		t.Fatal("restart did not ask with the disconnected explanation", err, a.confirmText)
	}
	a.confirm()
	a.confirmOpen = false
	if a.engine != nil {
		t.Fatal("safe-mode restart launched a core")
	}
	a.status = "running"
	tt.Frame()
	if tt.HasText(a.t("disconnected")) || tt.HasText(a.t("connected")) {
		t.Fatal("connected desktop control is not icon-only")
	}
	if err := tt.Click(a.t("coreStatus")); err != nil || a.confirmText != a.t("forceRestartCoreTip") {
		t.Fatal("connected control did not explain force restart", err, a.confirmText)
	}
}

func TestCoreConnectingHoldAndFailureFollowOriginal(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1200, 800)
	a.engine = &Engine{}
	a.status = "connecting"
	tt.Frame()
	if !tt.HasText(a.t("connecting")) {
		t.Fatal("connecting state missing")
	}
	// The process connects promptly; the original keeps its progress indicator
	// for 600 ms rather than flashing it for a single frame.
	a.engine = nil
	a.status = "running"
	tt.Advance(200 * time.Millisecond)
	if !tt.HasText(a.t("connecting")) {
		t.Fatal("connecting indicator flashed away")
	}
	if err := tt.Click(a.t("coreStatus")); err != nil || a.confirmOpen {
		t.Fatal("held connecting action opened a restart", err)
	}
	tt.Advance(500 * time.Millisecond)
	if tt.HasText(a.t("connecting")) {
		t.Fatal("connecting hold did not finish")
	}
	a.engine = &Engine{}
	a.status = "connecting"
	tt.Frame()
	a.status = "error"
	tt.Frame()
	if !tt.HasText(a.t("disconnected")) {
		t.Fatal("failed connection remained in the progress hold")
	}
}

func TestCoreStatusVisualReferences(t *testing.T) {
	for _, phase := range []string{"connected", "connecting", "disconnected"} {
		for _, dark := range []bool{false, true} {
			t.Run(phase+map[bool]string{true: "-dark", false: "-light"}[dark], func(t *testing.T) {
				a := testApp(t)
				a.page = "dashboard"
				if dark {
					a.cfg.Theme = "dark"
				}
				switch phase {
				case "connected":
					a.status = "running"
				case "connecting":
					a.status = "connecting"
					a.engine = &Engine{}
				}
				view := func(c *ui.Context) {
					a.theme(c)
					ui.Row(c).Fill().Background(a.colors.page).Children(func() {
						a.sidebar(c)
						ui.Column(c).Grow(1).FillHeight().Children(func() { a.titlebar(c); ui.Box(c).Grow(1).FillWidth() })
					})
				}
				tt := ui.NewTester(view, 1200, 800)
				tt.Advance(2 * time.Second)
				if dir := os.Getenv("FLCLASH_SNAPSHOT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					f, err := os.Create(filepath.Join(dir, "core-"+phase+map[bool]string{true: "-dark", false: "-light"}[dark]+".png"))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, tt.Image())
					f.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestCoreStatusAlreadyConnectingAtMountDoesNotAddAHold(t *testing.T) {
	a := testApp(t)
	a.engine = &Engine{}
	a.status = "connecting"
	tt := ui.NewTester(a.View, 1200, 800)
	a.engine = nil
	a.status = "running"
	tt.Frame()
	if tt.HasText(a.t("connecting")) {
		t.Fatal("mount invented a connecting transition")
	}
}
