package app

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type sheetMotionHarness struct {
	app                         *App
	tester                      *ui.Tester
	open, guarded, prompt       bool
	commits, actions, underlays int
	scroll                      ui.ScrollState
	close                       func()
}

func newSheetMotionHarness(t *testing.T, width, height int) *sheetMotionHarness {
	t.Helper()
	h := &sheetMotionHarness{app: testApp(t), open: true}
	h.close = func() {
		if h.guarded {
			h.prompt = true
			return
		}
		if h.app.deferSheetClose("motion-test", h.close) {
			return
		}
		h.commits++
		h.open = false
	}
	h.tester = ui.NewTester(func(c *ui.Context) {
		h.app.theme(c)
		ui.ButtonBase(c).Fill().Label("underlay").OnClick(func() { h.underlays++ })
		h.app.navigationSheet(c, "motion-test", &h.open, h.close, h.close, func() {
			ui.Box(c).FillWidth().Height(68).Label("sheet-header").Children(func() {
				ui.ButtonBase(c).Size(44, 44).Label("dismiss-sheet").OnClick(h.close)
			})
			ui.Scroll(c).Fill().TrackScroll(&h.scroll).Label("sheet-scroll").Children(func() {
				ui.Column(c).FillWidth().Height(2000).Children(func() {
					ui.ButtonBase(c).Size(180, 44).Label("sheet-action").OnClick(func() { h.actions++ })
				})
			})
		})
		// The synthetic sheet, like a native page, supplies its own primary
		// scroller. Tests do not depend on profile/core data or UI elements.
		h.app.snapSheet("motion-test").Scroll = &h.scroll
	}, width, height)
	return h
}

func (h *sheetMotionHarness) advance(d time.Duration) {
	h.tester.Advance(d)
	h.tester.Frame() // measure exactly at d, not at the previous display refresh
}

func TestSheetSidePresentationAndDismissalFollowRouteController(t *testing.T) {
	h := newSheetMotionHarness(t, 1200, 800)
	s := h.app.snapSheet("motion-test")
	if s.Progress != 0 || s.Barrier != 0 {
		t.Fatal("side sheet must enter from outside the right edge", s.Progress)
	}
	h.advance(150 * time.Millisecond)
	want := sheetDecelerate(.5)
	r, ok := h.tester.Find("sheet-header")
	if !ok || math.Abs(float64(s.Progress-want)) > .0001 || math.Abs(float64(r.X-(1200-480*want))) > .01 || math.Abs(float64(r.W-480*want)) > .01 {
		t.Fatal("intermediate side layout differs from side_sheet.dart", r, s.Progress, want)
	}
	if math.Abs(float64(s.Barrier-sheetBarrierEase(.5))) > .0001 {
		t.Fatal("side barrier must use its route's separate ease curve", s.Barrier)
	}
	h.advance(150 * time.Millisecond)
	if s.Progress != 1 {
		t.Fatal("side enter did not finish in 300ms", s.Progress)
	}
	if err := h.tester.Click("dismiss-sheet"); err != nil {
		t.Fatal(err)
	}
	if !h.open || h.commits != 0 || !s.Dismissing {
		t.Fatal("dismissal disposed its owner before the exit", h.open, h.commits)
	}
	h.advance(100 * time.Millisecond)
	if math.Abs(float64(s.Progress-want)) > .0001 {
		t.Fatal("exit must ease the reverse controller, not the remaining painted distance", s.Progress, want)
	}
	// The fading barrier remains modal throughout exit. Repeated requests
	// must neither invoke the close twice nor reach the page below it.
	h.tester.ClickAt(50, 400)
	if h.underlays != 0 || h.commits != 0 {
		t.Fatal("exit barrier leaked input or committed early")
	}
	h.advance(100 * time.Millisecond)
	if h.open || h.commits != 1 {
		t.Fatal("side exit did not release its owner once after 200ms", h.open, h.commits)
	}
}

func TestSheetBottomPresentationAndDismissalUseSourceSpring(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	s := h.app.snapSheet("motion-test")
	for _, elapsed := range []time.Duration{100 * time.Millisecond, 200 * time.Millisecond} {
		h.advance(100 * time.Millisecond)
		seconds := elapsed.Seconds()
		// _SheetSpring's critical solution: mass 3, stiffness 1000,
		// initial velocity zero. The route is independent of detent height.
		want := 1 - (1+sheetOmega*seconds)*math.Exp(-sheetOmega*seconds)
		r, ok := h.tester.Find("sheet-header")
		if !ok || math.Abs(float64(s.Progress)-want) > 1e-6 || math.Abs(float64(r.Y)-(800-450*want)) > .02 || r.W != 560 {
			t.Fatal("bottom spring presentation differs from source", elapsed, s.Progress, want, r)
		}
		if s.Barrier != s.Progress {
			t.Fatal("bottom barrier does not follow the presentation spring")
		}
	}
	h.advance(time.Second)
	h.close()
	h.tester.Frame()
	h.advance(100 * time.Millisecond)
	want := (1 + sheetOmega*.1) * math.Exp(-sheetOmega*.1)
	if !h.open || math.Abs(float64(s.Progress)-want) > 1e-6 || h.commits != 0 {
		t.Fatal("bottom exit skipped its intermediate spring", s.Progress, want, h.commits)
	}
	h.advance(time.Second)
	if h.open || h.commits != 1 {
		t.Fatal("bottom spring exit never released its owner")
	}
}

func TestSheetReducedMotionAndDismissalGuard(t *testing.T) {
	for _, width := range []int{560, 1200} {
		t.Run(number(width), func(t *testing.T) {
			h := newSheetMotionHarness(t, width, 800)
			h.tester.SetPreferences(ui.Preferences{ReduceMotion: true})
			s := h.app.snapSheet("motion-test")
			if s.Progress != 1 || s.Barrier != 1 {
				t.Fatal("reduced motion left presentation between endpoints")
			}
			h.guarded = true
			h.tester.ClickAt(50, 100)
			if !h.prompt || !h.open || s.Dismissing || s.DismissRequested || h.commits != 0 {
				t.Fatal("guard must run before hiding or disposing the sheet")
			}
			h.guarded = false
			h.close()
			if h.open || h.commits != 1 {
				t.Fatal("reduced motion deferred owner teardown")
			}
		})
	}
}

func TestSheetLateDismissalGuardRestoresTheSameRoute(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	h.close()
	h.tester.Frame()
	// An async save may start after an accepted request but before teardown.
	h.guarded = true
	h.advance(2 * time.Second)
	if !h.prompt || !h.open || h.commits != 0 || s.Dismissing || s.Progress != 1 || s != h.app.snapSheet("motion-test") {
		t.Fatal("late guard lost its route or left it offscreen")
	}
}

func TestSheetSupersededDismissalCannotCloseReplacement(t *testing.T) {
	h := newSheetMotionHarness(t, 1200, 800)
	h.advance(time.Second)
	h.close()
	h.tester.Frame()
	h.advance(50 * time.Millisecond)
	old := h.app.snapSheet("motion-test")
	h.app.resetSheet("motion-test")
	if old.Close != nil || old.Pressed || len(old.Samples) != 0 {
		t.Fatal("reset retained the obsolete route's input/callback ownership")
	}
	h.tester.Frame()
	if old == h.app.snapSheet("motion-test") {
		t.Fatal("reopening reused the exiting route")
	}
	h.advance(time.Second)
	if !h.open || h.commits != 0 || h.app.snapSheet("motion-test").Progress != 1 {
		t.Fatal("an obsolete exit closed the replacement sheet", h.commits)
	}
}

func TestSheetInterruptedSideEnterUsesRemainingControllerDuration(t *testing.T) {
	h := newSheetMotionHarness(t, 1200, 800)
	h.advance(150 * time.Millisecond)
	s := h.app.snapSheet("motion-test")
	before := s.Progress
	h.close()
	h.tester.Frame()
	if s.Progress != before {
		t.Fatal("interrupting enter jumped the painted sheet")
	}
	h.advance(50 * time.Millisecond)
	if math.Abs(float64(s.Progress-sheetDecelerate(.25))) > .0001 || !h.open {
		t.Fatal("interrupted reverse used a whole 200ms instead of the remaining controller range", s.Progress)
	}
	h.advance(50 * time.Millisecond)
	if h.open || h.commits != 1 {
		t.Fatal("half-presented side sheet did not leave in 100ms")
	}
}

func TestSheetEnablingReducedMotionFinishesOwnedExit(t *testing.T) {
	for _, width := range []int{560, 1200} {
		h := newSheetMotionHarness(t, width, 800)
		h.advance(time.Second)
		h.close()
		h.tester.Frame()
		h.advance(50 * time.Millisecond)
		h.tester.SetPreferences(ui.Preferences{ReduceMotion: true})
		if h.open || h.commits != 1 {
			t.Fatal("enabling reduced motion stranded the exiting owner", width)
		}
	}
}

func TestSheetBreakpointReopensWithoutOldInputOwnership(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	h.tester.Press(280, 370)
	h.advance(16 * time.Millisecond)
	h.tester.Move(280, 270)
	h.tester.SetSize(1200, 800)
	if s.Pressed || len(s.Samples) != 0 || s.Compact || s.Controller != 0 {
		t.Fatal("breakpoint carried a drag into the side route")
	}
	h.tester.Release(280, 270)
	h.advance(time.Second)
	h.tester.SetSize(560, 800)
	h.advance(time.Second)
	if !h.open || s.Stop != 0 || s.Height.Value != 450 || s.Progress != 1 {
		t.Fatal("new bottom route did not reopen at its initial detent")
	}
}

func TestSheetCompactDismissalRetainsCaptionBounds(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.app.frameHeight = 32
	h.tester.Frame()
	h.advance(time.Second)
	r, _ := h.tester.Find("sheet-header")
	want := float32(32 + (800-32)*(1-9.0/16))
	if r.Y != want || h.app.snapSheet("motion-test").Height.Value != (800-32)*9.0/16 {
		t.Fatal("sheet detents included the desktop caption", r, want)
	}
}
