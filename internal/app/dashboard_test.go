package app

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDashboardUsesSummariesWhenTablesDoNotFit(t *testing.T) {
	for _, size := range [][2]int{{1200, 640}, {1000, 800}, {800, 600}} {
		a := populatedApp(t)
		tt := ui.NewTester(a.View, size[0], size[1])
		if tt.HasText(a.t("target")) {
			t.Fatalf("table shown at %v", size)
		}
		if _, summary := tt.Find(a.t("connectionStats")); !summary {
			t.Fatalf("missing record summaries at %v", size)
		}
		for attempt := 0; attempt < 5; attempt++ {
			rect, _ := tt.Find(a.t("requests"))
			if rect.Y >= 68 && rect.Y+rect.H < float32(size[1]) {
				break
			}
			tt.Scroll(float32(size[0]-30), float32(size[1]-30), 0, 600)
			tt.Advance(200 * time.Millisecond)
		}
		if err := tt.Click(a.t("requests")); err != nil || a.page != "requests" {
			t.Fatal("summary did not open feed", a.page, err)
		}
	}
	a := populatedApp(t)
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText(a.t("target")) {
		t.Fatal("missing table in a sufficiently tall four-column dashboard")
	}
}

func TestDashboardCapsWideColumnsAndSwitchesMode(t *testing.T) {
	a := populatedApp(t)
	tt := ui.NewTester(a.View, 2800, 1000)
	left, _ := tt.Find(a.t("status"))
	right, _ := tt.Find(a.t("memoryInfo"))
	if left.X < 500 || right.X-left.X > 1450 {
		t.Fatal("dashboard stretched past the original 440-point column cap", left, right)
	}
	if err := tt.Click(a.t("global")); err != nil || a.cfg.Overrides["mode"] != "global" {
		t.Fatal("connected mode control failed", a.cfg.Overrides["mode"], err)
	}
	// Resizing through the table threshold keeps the same cards and switches.
	tt.SetSize(1200, 640)
	if _, has := tt.Find(a.t("connectionStats")); !has {
		t.Fatal("resize did not replace the table with summaries")
	}
	if err := tt.Click("TUN"); err != nil {
		t.Fatal(err)
	}
	if a.engine != nil {
		t.Fatal("safe dashboard started a core")
	}
	if r, has := tt.Find(a.t("status")); !has || math.IsNaN(float64(r.X)) {
		t.Fatal("invalid layout after resize")
	}
}
