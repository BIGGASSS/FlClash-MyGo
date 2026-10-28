package app

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSheetBlurCancelsVelocityAndRestoresDetent(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	h.tester.Press(280, 370)
	h.advance(8 * time.Millisecond)
	h.tester.Move(280, 395)
	h.advance(8 * time.Millisecond)
	h.tester.Move(280, 420)
	if !s.Pressed || s.Height.Value != 400 {
		t.Fatal("header did not start a drag", s.Pressed, s.Height.Value)
	}
	if projected := s.Height.Value - pointerVelocity(s.Samples, h.tester.Now())*.099; projected >= 225 {
		t.Fatal("invalid fixture: an ordinary release would not dismiss", projected)
	}
	// The same release velocity would project close to zero. Blur is a
	// synthetic pointer-up in MyGo, but must be dragEnd(0), not a fling.
	h.tester.SetFocused(false)
	if s.Pressed || len(s.Samples) != 0 || s.Dismissing || s.DismissRequested || !h.open {
		t.Fatal("blur retained capture or dismissed using stale velocity")
	}
	h.advance(time.Second)
	if !h.open || s.Height.Value != 450 || s.Progress != 1 || s.Stop != 0 {
		t.Fatal("cancelled drag did not restore the nearest detent", s.Height.Value, s.Progress)
	}
	h.tester.SetFocused(true)
	h.tester.Press(280, 370)
	if !s.Pressed {
		t.Fatal("cancellation left the next gesture unusable")
	}
	h.tester.Release(280, 370)
}

func TestSheetDragDismissalIsContinuousAndOwnsExit(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	h.tester.Press(280, 370)
	h.advance(16 * time.Millisecond)
	h.tester.Move(280, 470)
	h.advance(16 * time.Millisecond)
	h.tester.Move(280, 570)
	before, _ := h.tester.Find("sheet-header")
	beforeBarrier := s.Barrier * float32(s.Height.Value/450)
	h.tester.Release(280, 570)
	after, _ := h.tester.Find("sheet-header")
	if !s.Dismissing || !h.open || h.commits != 0 || s.Pressed || len(s.Samples) != 0 {
		t.Fatal("throw disposed the owner or retained input state", s.Dismissing, h.open, h.commits, s.Pressed, s.Height.Value, s.Stop)
	}
	if math.Abs(float64(before.Y-after.Y)) > .02 || math.Abs(float64(s.Barrier-beforeBarrier)) > .001 {
		t.Fatal("release jumped from drag coordinates to presentation coordinates", before, after, s.Barrier, beforeBarrier)
	}
	h.advance(time.Second)
	if h.open || h.commits != 1 {
		t.Fatal("drag dismissal did not finish once")
	}
}

func TestSheetDragGuardRestoresPresentationWithoutLosingContent(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	h.guarded = true
	h.tester.Press(280, 370)
	h.advance(16 * time.Millisecond)
	h.tester.Move(280, 470)
	h.advance(16 * time.Millisecond)
	h.tester.Move(280, 570)
	h.tester.Release(280, 570)
	if !h.prompt || !h.open || h.commits != 0 {
		t.Fatal("dismissal guard did not keep its owner")
	}
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	if s.Progress != 1 || s.Height.Value != 450 || s.Pressed {
		t.Fatal("guarded throw did not spring back", s.Progress, s.Height.Value)
	}
}

func TestSheetWheelLatchesOneDetentAndHandsOffAtContentEdge(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	h.tester.Scroll(280, 500, 0, 60)
	if s.Stop != 1 || s.Height.Target != 720 || h.scroll.Y != 0 {
		t.Fatal("wheel at content edge must grow the sheet before scrolling")
	}
	h.advance(100 * time.Millisecond)
	h.tester.Scroll(280, 500, 0, -60)
	if s.Stop != 1 || h.scroll.Y != 0 {
		t.Fatal("one wheel turn crossed another detent or escaped into content")
	}
	h.advance(600 * time.Millisecond)
	h.tester.Scroll(280, 500, 0, 60)
	if h.scroll.Y != 60 || s.Stop != 1 {
		t.Fatal("new wheel turn at the upper detent did not scroll content", h.scroll.Y)
	}
	h.advance(600 * time.Millisecond)
	h.tester.Scroll(280, 500, 0, -100)
	if h.scroll.Y != 0 || s.Stop != 1 {
		t.Fatal("scrolled content must take a turn that reaches its top", h.scroll.Y, s.Stop)
	}
	h.advance(100 * time.Millisecond)
	h.tester.Scroll(280, 500, 0, -100)
	if s.Stop != 1 {
		t.Fatal("a content-owned turn stole the sheet after reaching the top")
	}
	h.advance(600 * time.Millisecond)
	h.tester.Scroll(280, 500, 0, -100)
	if s.Stop != 0 || s.Height.Target != 450 || !h.open {
		t.Fatal("new backward turn at the edge did not shrink without dismissing")
	}
}

func TestSheetHeaderWheelDoesNotBorrowScrolledContentOwnership(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	h.tester.Scroll(280, 500, 0, 40)
	h.advance(time.Second)
	h.scroll.Y = 100
	h.tester.Frame()
	h.tester.Scroll(280, 100, 0, -40) // the header at the 720-point detent
	if s := h.app.snapSheet("motion-test"); s.Stop != 0 || h.scroll.Y != 100 {
		t.Fatal("a header wheel used the list's scroll offset instead of its hit bounds", s.Stop, h.scroll.Y)
	}
}

func TestSheetReducedMotionSettlesWheelDetentImmediately(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	h.tester.Scroll(280, 500, 0, 40)
	s := h.app.snapSheet("motion-test")
	if s.Stop != 1 || s.Height.Value != 720 || s.Height.Velocity != 0 || h.scroll.Y != 0 {
		t.Fatal("reduced motion left the detent spring running", s.Height.Value, s.Height.Velocity)
	}
}

func TestSheetWheelIgnoresHorizontalAndModifierGestures(t *testing.T) {
	now := time.Now()
	s := &snapSheetState{}
	s.Height.jump(now, 450)
	for _, e := range []ui.InputEvent{
		{Kind: ui.InputScroll, DX: 60, Time: now},
		{Kind: ui.InputScroll, DY: 60, Mods: ui.Ctrl, Time: now},
		{Kind: ui.InputScroll, DY: 60, Mods: ui.Cmd, Time: now},
		{Kind: ui.InputScroll, DY: 60, Mods: ui.Shift, Time: now},
	} {
		if s.input(e, 800, 720, func() { t.Fatal("wheel dismissed the sheet") }) || s.Stop != 0 || !s.LastWheel.IsZero() {
			t.Fatal("sheet consumed a horizontal/zoom/modified gesture", e)
		}
	}
}

func TestSheetNativeProfileListWheelHandoff(t *testing.T) {
	a := populatedApp(t)
	for i := range 30 {
		p := a.cfg.Profiles[0]
		p.ID, p.Name = "wheel-"+number(i), "Wheel profile "+number(i)
		a.cfg.Profiles = append(a.cfg.Profiles, p)
	}
	a.openProfilePicker()
	tt := ui.NewTester(func(c *ui.Context) { a.theme(c); a.dashboardPickerSheet(c) }, 560, 800)
	tt.Advance(time.Second)
	tt.Frame()
	s := a.snapSheet("dashboard-picker")
	if s.Scroll == nil || s.Scroll.MaxY <= 0 {
		t.Fatal("native virtual list did not expose its numeric scroll state")
	}
	tt.Scroll(280, 500, 0, 40)
	if s.Stop != 1 || s.Scroll.Y != 0 {
		t.Fatal("native content scrolled instead of growing its sheet")
	}
	tt.Advance(time.Second)
	tt.Scroll(280, 500, 0, 40)
	if s.Scroll.Y <= 0 || s.Stop != 1 {
		t.Fatal("native virtual list did not take the next wheel turn", s.Scroll.Y)
	}
}

func TestSheetPointerButtonsAndCancellationUseNearestStop(t *testing.T) {
	now := time.Now()
	s := &snapSheetState{Width: 560}
	s.Height.jump(now, 450)
	if s.input(ui.InputEvent{Kind: ui.InputPointerDown, Button: 1, X: 280, Y: 20, Time: now}, 800, 720, func() {}) {
		t.Fatal("secondary button started a drag")
	}
	s.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 280, Y: 20, Time: now}, 800, 720, func() {})
	s.Height.jump(now, 650)
	s.Samples = append(s.Samples, pointerSample{X: 1000, At: now.Add(time.Millisecond)})
	if !s.cancel(now.Add(2*time.Millisecond), 800, 720, func() { t.Fatal("cancellation used a stale fling") }) || s.Pressed || s.Stop != 1 || s.Height.Target != 720 || len(s.Samples) != 0 {
		t.Fatal("cancel did not choose the nearest stop with zero velocity")
	}
	if s.cancel(now, 800, 720, func() {}) || s.input(ui.InputEvent{Kind: ui.InputPointerUp, Time: now}, 800, 720, func() {}) {
		t.Fatal("an obsolete release retained gesture ownership")
	}
}
