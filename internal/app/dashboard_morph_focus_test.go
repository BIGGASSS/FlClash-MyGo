package app

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDashboardStateLayerAndRingMatchOriginalWidget(t *testing.T) {
	reference := dashboardMorphFixture(t)
	if len(reference.Hover) != 3 {
		t.Fatal("missing original hover trace")
	}
	base := reference.Hover[0].Color
	fill := ui.Color{R: uint8(base >> 16), G: uint8(base >> 8), B: uint8(base), A: 255}
	for _, sample := range reference.Hover {
		if sample.Overlay != float64(dashboardOverlay(true, false, true)) && math.Abs(sample.Overlay-.08) > 1e-8 {
			t.Fatal("source hover target changed", sample.Overlay)
		}
		// The reference uses Material's default light scheme (white onPrimary).
		// Compare the actual painter's color, not just the target getter.
		native := fill.Mix(ui.Hex("#ffffff"), .08*float32(sample.MS)/150)
		want := [3]uint8{uint8(sample.Color >> 16), uint8(sample.Color >> 8), uint8(sample.Color)}
		for axis, value := range [3]uint8{native.R, native.G, native.B} {
			if math.Abs(float64(value)-float64(want[axis])) > 1 {
				t.Fatal("source linear state-layer color differs at", sample.MS, native, want)
			}
		}
		if sample.Lift != 0 || sample.Ring != nil {
			t.Fatal("source hover unexpectedly swells or rings the control")
		}
	}
	ring := reference.Focused.Ring
	if ring == nil || ring.Width != 2 || ring.Align != 2 || ring.Color != base || reference.Focused.Overlay != .1 {
		t.Fatal("source focus style changed", ring)
	}
	r := dashboardFocusPath(ui.Rect{W: 88, H: 88}, 0).Bounds()
	if len(reference.ShapeBounds) == 0 || reference.ShapeBounds[0].Progress != 0 {
		t.Fatal("missing original unit-path bounds")
	}
	unit := reference.ShapeBounds[0].Bounds
	for axis, got := range [4]float32{r.X, r.Y, r.W, r.H} {
		want := unit[axis] * 92
		if axis < 2 {
			want -= 2
		}
		if math.Abs(float64(got)-want) > 1e-5 {
			t.Fatal("focus stroke does not use source center scaling", r, unit)
		}
	}
	if reference.DisabledSuspended.Overlay != 0 || reference.DisabledSuspended.Ring != nil {
		t.Fatal("source disabled state kept a focus/hover layer")
	}
}
