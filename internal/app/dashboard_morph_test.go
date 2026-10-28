package app

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type dashboardMorphSample struct {
	MS                      int
	Controller, Shape, Lift float64
	Vertices                [][][2]float64
	PaintBounds             [4]float64
	Overlay                 float64
	Color                   uint32
	Ring                    *struct {
		Width, Align float64
		Color        uint32
	}
}

type dashboardMorphReference struct {
	Flutter, WorkflowFlutter string
	MorphMS, StateMS         int
	Glyphs                   []struct {
		Progress float64
		Vertices [][][2]float64
		Bounds   [][4]float64
	}
	ShapeBounds []struct {
		Progress float64
		Bounds   [4]float64
	}
	Curves                                                                        []struct{ T, Forward, Reverse float64 }
	Forward, Reverse, SettledReverse, Interrupted, Held, Release, PlatformReduced []dashboardMorphSample
	Hover                                                                         []dashboardMorphSample
	Focused, DisabledSuspended                                                    dashboardMorphSample
}

func dashboardMorphFixture(t *testing.T) dashboardMorphReference {
	t.Helper()
	data, err := os.ReadFile("testdata/flutter-dashboard-morph.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference dashboardMorphReference
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	return reference
}

func compareDashboardVertices(t *testing.T, got, want [][][2]float64, tolerance float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("glyph has %d paths, source %d", len(got), len(want))
	}
	for h := range got {
		if len(got[h]) != len(want[h]) {
			t.Fatalf("path %d vertices: %d, source %d", h, len(got[h]), len(want[h]))
		}
		for i := range got[h] {
			for axis := range 2 {
				if math.Abs(got[h][i][axis]-want[h][i][axis]) > tolerance {
					t.Fatalf("path %d point %d: %v, source %v", h, i, got[h][i], want[h][i])
				}
			}
		}
	}
}

func TestDashboardPlayPauseMatchesOriginalIntermediatePaths(t *testing.T) {
	reference := dashboardMorphFixture(t)
	if reference.Flutter != "3.47.5" || reference.WorkflowFlutter != "3.47.6" {
		t.Fatal("reference must disclose the SDK/workflow mismatch", reference.Flutter, reference.WorkflowFlutter)
	}
	if time.Duration(reference.MorphMS)*time.Millisecond != dashboardMorphDuration || time.Duration(reference.StateMS)*time.Millisecond != dashboardStateDuration {
		t.Fatal("source durations changed")
	}
	for _, sample := range reference.Glyphs {
		compareDashboardVertices(t, dashboardPlayPauseVertices(sample.Progress), sample.Vertices, 1e-12)
		paths := dashboardPlayPausePaths(ui.Rect{W: 24, H: 24}, float32(sample.Progress))
		for h, path := range paths {
			r := path.Bounds()
			got := [4]float64{float64(r.X), float64(r.Y), float64(r.W), float64(r.H)}
			for axis := range got {
				if math.Abs(got[axis]-sample.Bounds[h][axis]) > 1e-5 {
					t.Fatalf("progress %v path %d bounds %v, source %v", sample.Progress, h, got, sample.Bounds[h])
				}
			}
		}
	}
	for _, sample := range reference.Curves {
		if math.Abs(dashboardCurve(sample.T, false)-sample.Forward) > 1e-12 || math.Abs(dashboardCurve(sample.T, true)-sample.Reverse) > 1e-12 {
			t.Fatal("source curve mismatch at", sample.T)
		}
	}
}

func TestDashboardMorphTimelineMatchesOriginalWidget(t *testing.T) {
	reference := dashboardMorphFixture(t)
	var state dashboardMorphState
	var shape, glyph float32
	running := false
	tt := ui.NewTester(func(c *ui.Context) {
		shape, glyph = state.frame(c, running, false)
	}, 1, 1)
	set := func(value bool) { running = value; tt.Frame() }
	trace := func(name string, samples []dashboardMorphSample) {
		t.Helper()
		previous := 0
		for _, sample := range samples {
			tt.Advance(time.Duration(sample.MS-previous) * time.Millisecond)
			tt.Frame()
			previous = sample.MS
			if math.Abs(state.controller-sample.Controller) > 1e-8 || math.Abs(float64(shape)-sample.Shape) > 1e-6 {
				t.Fatalf("%s at %d ms: controller/shape %.10f/%.10f, source %.10f/%.10f", name, sample.MS, state.controller, shape, sample.Controller, sample.Shape)
			}
			compareDashboardVertices(t, dashboardPlayPauseVertices(float64(glyph)), sample.Vertices, 2e-6)
		}
	}
	set(true)
	trace("forward", reference.Forward)
	set(false)
	trace("endpoint reverse", reference.Reverse)
	tt.Advance(time.Millisecond)
	set(true)
	tt.Advance(451 * time.Millisecond)
	set(false)
	trace("settled reverse", reference.SettledReverse)
	tt.Advance(time.Millisecond)
	set(true)
	tt.Advance(180 * time.Millisecond)
	set(false)
	trace("interrupted", reference.Interrupted)
	// Source MediaQuery.disableAnimations affects the state-layer duration;
	// platform accessibility additionally compresses both morph controllers.
	tt.Advance(time.Second)
	set(true)
	tt.Advance(451 * time.Millisecond)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	set(false)
	trace("platform reduced", reference.PlatformReduced)
}

func TestDashboardPressMatchesOriginalWidget(t *testing.T) {
	reference := dashboardMorphFixture(t)
	for _, reduced := range []bool{false, true} {
		var press dashboardPressState
		var sx, sy, x, y float32
		tt := ui.NewTester(func(c *ui.Context) { sx, sy, x, y = press.frame(c, 88, true) }, 1, 1)
		tt.SetPreferences(ui.Preferences{ReduceMotion: reduced})
		press.input(ui.InputEvent{Kind: ui.InputPointerDown, Time: tt.Now(), X: 44, Y: 44}, 88)
		tt.Frame()
		trace := func(samples []dashboardMorphSample) {
			t.Helper()
			previous := 0
			for _, sample := range samples {
				tt.Advance(time.Duration(sample.MS-previous) * time.Millisecond)
				tt.Frame()
				previous = sample.MS
				if math.Abs(press.lift.Value-sample.Lift) > 1e-10 {
					t.Fatalf("reduced %v at %d: lift %.12f, source %.12f", reduced, sample.MS, press.lift.Value, sample.Lift)
				}
				// Source paintBoundsOf is the glyph's bounds under the enclosing
				// press, relative to its own (unchanged) 29.92-point layout box.
				width := 88.0 * .34 * float64(sx)
				if math.Abs(width-sample.PaintBounds[2]) > 5e-6 || sx != sy || x != 0 || y != 0 {
					t.Fatal("source press transform mismatch", sx, sy, sample.PaintBounds)
				}
			}
		}
		trace(reference.Held)
		press.input(ui.InputEvent{Kind: ui.InputPointerUp, Time: tt.Now()}, 88)
		tt.Frame()
		trace(reference.Release)
	}
}

func TestDashboardQuickTapPullAndDisabledPress(t *testing.T) {
	var press dashboardPressState
	enabled := true
	tt := ui.NewTester(func(c *ui.Context) { press.frame(c, 88, enabled) }, 1, 1)
	press.input(ui.InputEvent{Kind: ui.InputPointerDown, Time: tt.Now(), X: 44, Y: 44}, 88)
	press.input(ui.InputEvent{Kind: ui.InputPointerUp, Time: tt.Now()}, 88)
	tt.Frame()
	tt.Advance(90 * time.Millisecond)
	tt.Frame()
	if press.lift.Value < .6 || press.lift.Target != 1 {
		t.Fatal("short tap failed to rise before release", press.lift)
	}
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if press.lift.Target != 0 {
		t.Fatal("tap failed to spring back after 0.975 lift")
	}
	tt.Advance(time.Second)
	press.input(ui.InputEvent{Kind: ui.InputPointerDown, Time: tt.Now(), X: 44, Y: 44}, 88)
	press.input(ui.InputEvent{Kind: ui.InputPointerMove, Time: tt.Now(), X: 144, Y: -56}, 88)
	tt.Frame()
	tt.Advance(120 * time.Millisecond)
	tt.Frame()
	if press.x.Value <= 0 || press.y.Value >= 0 || press.x.Value >= 88*7/32.0 {
		t.Fatal("pull lost the source rubber band", press.x, press.y)
	}
	enabled = false
	tt.Frame()
	if press.pressed || press.lift.Value != 0 || press.x.Value != 0 || press.y.Value != 0 {
		t.Fatal("disabling left a held native press")
	}
}

func TestDashboardControlOverlayFocusHitTargetAndSafeMode(t *testing.T) {
	if dashboardOverlay(false, true, true) != 0 || dashboardOverlay(true, true, true) != .1 || dashboardOverlay(true, false, true) != .08 || dashboardOverlay(true, false, false) != 0 {
		t.Fatal("state-layer precedence differs from source")
	}
	a := populatedApp(t)
	a.wantRunning = false
	a.safe = true
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		ui.Box(c).Fill().Background(a.colors.card).Center().Children(func() { a.dashboardStartControl(c, 88) })
	}, 240, 240)
	button, ok := tt.Find(a.t("start"))
	if !ok || button.W != 88 || button.H != 88 {
		t.Fatal("start control lost its source hit box", button)
	}
	tt.Move(button.X+44, button.Y+44)
	tt.Advance(75 * time.Millisecond)
	tt.Frame()
	// Overlay tween is linear, not MyGo's default ease-out.
	at := tt.Image().RGBAAt(int(button.X+20), int(button.Y+44))
	want := a.colors.accent.Mix(a.colors.onAccent, .04)
	if math.Abs(float64(at.R)-float64(want.R)) > 2 || math.Abs(float64(at.G)-float64(want.G)) > 2 || math.Abs(float64(at.B)-float64(want.B)) > 2 {
		t.Fatal("hover layer does not use source 150 ms linear tween", at, want)
	}
	tt.Key(0, ui.KeyTab)
	if !tt.Focused(a.t("start")) {
		t.Fatal("start control is not keyboard-focusable")
	}
	tt.Advance(time.Second)
	// Circle ring: its centerline is expanded by 2, width 2. No rectangular
	// toolkit ring is permitted around the 88-point input box.
	image := tt.Image()
	ring := image.RGBAAt(int(button.X-2), int(button.Y+44))
	if ring.R != a.colors.accent.R || ring.G != a.colors.accent.G || ring.B != a.colors.accent.B {
		t.Fatal("missing source stand-off focus ring", ring, a.colors.accent)
	}
	corner := image.RGBAAt(int(button.X-2), int(button.Y-2))
	if corner.R == ring.R && corner.G == ring.G && corner.B == ring.B {
		t.Fatal("toolkit rectangular focus ring leaked around morph")
	}
	tt.Press(button.X+44, button.Y+44)
	tt.Advance(210 * time.Millisecond)
	grown, _ := tt.Find(a.t("start"))
	if grown != button {
		t.Fatal("press changed layout/hit target", button, grown)
	}
	painted := tt.Image().RGBAAt(int(button.X-4), int(button.Y+44))
	pressedFill := a.colors.accent.Mix(a.colors.onAccent, .1)
	if painted.R != pressedFill.R || painted.G != pressedFill.G || painted.B != pressedFill.B {
		t.Fatal("pointer press did not swell the painted shape", painted, pressedFill)
	}
	tt.Release(button.X+44, button.Y+44)
	tt.Key(0, ui.KeySpace)
	if a.wantRunning {
		t.Fatal("safe mode activated the engine")
	}
	a.cfg.Profiles = nil
	tt.Frame()
	if err := tt.Click(a.t("start")); err == nil {
		// Tester may report a located but disabled target as a successful
		// pointer dispatch; activation must still remain suppressed.
		if a.wantRunning {
			t.Fatal("disabled control activated")
		}
	}
}
