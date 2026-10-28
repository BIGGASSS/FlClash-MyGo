package app

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSheetContentDragGrowsThenScrollsAndReturnsToSheet(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	h.tester.Press(280, 650) // bare content, not the action at its top
	if !s.Pressed || !s.ContentDrag || !s.StartedAtEdge {
		t.Fatal("content did not capture its starting edge")
	}
	h.tester.Move(280, 649)
	if s.DragStarted || s.Height.Value != 450 {
		t.Fatal("precise-pointer slop moved the sheet")
	}
	h.tester.Move(280, 648)
	if !s.DragStarted || s.Height.Value != 450 {
		t.Fatal("accepting move was not dropped (DragStartBehavior.start)")
	}
	h.advance(16 * time.Millisecond)
	h.tester.Move(280, 348)
	if s.Height.Value != 720 || h.scroll.Y != 30 || s.SheetHeld {
		t.Fatal("growth remainder did not scroll in the same gesture", s.Height.Value, h.scroll.Y)
	}
	h.tester.Move(280, 408)
	if h.scroll.Y != 0 || s.Height.Value != 720 {
		t.Fatal("reversal must first return content to its edge", s.Height.Value, h.scroll.Y)
	}
	h.tester.Move(280, 438)
	if s.Height.Value != 690 || !s.SheetHeld {
		t.Fatal("edge-started gesture did not take back the sheet", s.Height.Value)
	}
	h.advance(50 * time.Millisecond)
	h.tester.Release(280, 438)
	h.advance(time.Second)
	if s.Pressed || s.DragScroll != nil || s.Height.Value != 720 || s.Stop != 1 || !h.open {
		t.Fatal("content drag did not release to the nearest stop")
	}
}

func TestSheetScrolledContentDragLatchesPastTop(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	h.scroll.Y = 80
	h.tester.Frame()
	h.tester.Press(280, 650)
	h.tester.Move(280, 652) // accept vertical drag
	h.tester.Move(280, 752) // reaches the top with unused downward movement
	h.tester.Move(280, 790)
	if h.scroll.Y != 0 || s.Height.Value != 450 || s.SheetTouched || s.StartedAtEdge {
		t.Fatal("a scrolled-content start stole the sheet after reaching zero", h.scroll.Y, s.Height.Value)
	}
	h.tester.Release(280, 790)
	if !h.open || s.Pressed || s.DragScroll != nil || h.actions != 0 {
		t.Fatal("content-owned release dismissed or clicked a row")
	}
}

func TestSheetContentDragBlurHasNoFlingAndCanStartAgain(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	h.tester.Press(280, 550)
	h.tester.Move(280, 552)
	h.advance(8 * time.Millisecond)
	h.tester.Move(280, 577)
	h.advance(8 * time.Millisecond)
	h.tester.Move(280, 602)
	if s.Height.Value != 400 || !s.SheetTouched {
		t.Fatal("invalid content cancellation fixture", s.Height.Value)
	}
	h.tester.SetFocused(false)
	h.advance(time.Second)
	if s.Pressed || s.DragScroll != nil || !h.open || s.Height.Value != 450 || s.Stop != 0 {
		t.Fatal("content blur retained ownership or flung the route")
	}
	h.tester.SetFocused(true)
	h.tester.Press(280, 550)
	if !s.ContentDrag {
		t.Fatal("blur stranded the next drag")
	}
	h.tester.Release(280, 550)
}

func TestSheetContentScrollerReplacementCancelsOldCapture(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	s := h.app.snapSheet("motion-test")
	h.tester.Press(280, 650)
	h.tester.Move(280, 648)
	h.tester.Move(280, 548)
	if s.Height.Value != 550 || !s.SheetTouched {
		t.Fatal("invalid page-replacement fixture")
	}
	replacement := &ui.ScrollState{Y: 200, MaxY: 1000}
	s.Scroll = replacement
	h.tester.Move(280, 448)
	if s.Pressed || s.DragScroll != nil || replacement.Y != 200 || h.scroll.Y != 0 {
		t.Fatal("late move mutated the replacement page's scroll state")
	}
	h.tester.Release(280, 448)
	h.advance(time.Second)
	if !h.open || s.Height.Value != 450 {
		t.Fatal("page replacement flung instead of cancelling")
	}
}

func TestSheetNativeProfileListGapCanHandOffPointerDrag(t *testing.T) {
	a := populatedApp(t)
	for i := range 30 {
		p := a.cfg.Profiles[0]
		p.ID, p.Name = "drag-"+number(i), "Drag profile "+number(i)
		a.cfg.Profiles = append(a.cfg.Profiles, p)
	}
	a.openProfilePicker()
	tt := ui.NewTester(func(c *ui.Context) { a.theme(c); a.dashboardPickerSheet(c) }, 560, 800)
	tt.Advance(time.Second)
	tt.Frame()
	s := a.snapSheet("dashboard-picker")
	// Header ends at 418; the first 64-point card has an eight-point gap.
	// A card itself keeps its native click rather than being replayed by us.
	tt.Press(280, 486)
	if !s.ContentDrag || s.DragScroll != s.Scroll {
		t.Fatal("virtual-list gap did not expose a safe content drag target")
	}
	tt.Move(280, 484)
	tt.Move(280, 184)
	if s.Height.Value != 720 || s.Scroll.Y != 30 {
		t.Fatal("native virtual list lost the sheet-to-scroll remainder", s.Height.Value, s.Scroll.Y)
	}
	tt.Release(280, 184)
}

func TestSheetContentControlsKeepNativeClicksEditingAndSelection(t *testing.T) {
	a := testApp(t)
	open, query, actions := true, "", 0
	var scroll ui.ScrollState
	var selection ui.Handle
	var start, end int
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		a.navigationSheet(c, "content-controls", &open, func() { open = false }, func() { open = false }, func() {
			ui.Box(c).FillWidth().Height(68).Children(func() {
				// An extra toolbar action outside the old 72-point exclusion.
				ui.ButtonBase(c).Absolute().Left(330).Top(12).Size(44, 44).Label("toolbar-action").OnClick(func() { actions++ })
			})
			ui.Scroll(c).Fill().TrackScroll(&scroll).Children(func() {
				ui.Column(c).FillWidth().Height(2000).Children(func() {
					ui.ButtonBase(c).Size(180, 44).Label("content-action").OnClick(func() { actions++ })
					ui.SearchField(c, &query).Width(300).Height(48).Label("content-search")
					text := ui.Text(c, "selectable sheet text").Selectable().Bind(&selection).Height(40).Label("content-selection")
					start, end = text.TextSelection()
				})
			})
		})
		a.snapSheet("content-controls").Scroll = &scroll
	}, 560, 800)
	tt.Advance(time.Second)
	for _, label := range []string{"toolbar-action", "content-action", "content-search"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	tt.Type("native search")
	if actions != 2 || query != "native search" || a.snapSheet("content-controls").Pressed {
		t.Fatal("sheet stole a toolbar/content click or text input", actions, query)
	}
	r, ok := tt.Find("content-selection")
	if !ok {
		t.Fatal("missing selectable text")
	}
	tt.Press(r.X+2, r.Y+r.H/2)
	tt.Move(r.X+r.W-2, r.Y+r.H/2)
	tt.Release(r.X+r.W-2, r.Y+r.H/2)
	if start == end || a.snapSheet("content-controls").Height.Value != 450 || a.snapSheet("content-controls").Pressed {
		t.Fatal("native text selection became a sheet drag", start, end)
	}
}

func TestSheetFitContentRemeasuresBothDirectionsAtUpperRest(t *testing.T) {
	a := testApp(t)
	open, natural := true, float32(560)
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		a.navigationSheet(c, "fit-test", &open, func() { open = false }, func() { open = false }, func() {
			ui.Box(c).FillWidth().Height(68).Label("fit-header")
			ui.Box(c).Fill().Label("fit-body")
		}, natural)
	}, 560, 800)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Scroll(280, 370, 0, 40)
	s := a.snapSheet("fit-test")
	if s.Height.Value != 560 || s.Stop != 1 {
		t.Fatal("fit sheet did not rest at its content", s.Height.Value)
	}
	for _, height := range []float32{490, 680, 100, 900} {
		natural = height
		tt.Frame()
		want := min(max(height, 450), 720)
		if s.Height.Value != float64(want) || s.Height.Target != float64(want) {
			t.Fatal("resting content change retained an old detent", height, s.Height.Value, s.Height.Target)
		}
		r, _ := tt.Find("fit-body")
		if r.Y+r.H != 800 || r.H != want-68 {
			t.Fatal("refit left blank/overflowing body room", height, r)
		}
	}
}

func TestSheetGuardedContentThrowDoesNotLendVelocityToALaterClose(t *testing.T) {
	h := newSheetMotionHarness(t, 560, 800)
	h.advance(time.Second)
	h.guarded = true
	h.tester.Press(280, 550)
	h.tester.Move(280, 552)
	h.advance(8 * time.Millisecond)
	h.tester.Move(280, 602)
	h.advance(8 * time.Millisecond)
	h.tester.Move(280, 652)
	h.tester.Release(280, 652)
	s := h.app.snapSheet("motion-test")
	if !h.prompt || !h.open || s.Dismissing || s.ClosingVelocity != 0 {
		t.Fatal("guarded content fling was not scoped to its own frame")
	}
	h.advance(time.Second)
	h.guarded = false
	h.close()
	h.tester.Frame()
	h.advance(100 * time.Millisecond)
	want := (1 + sheetOmega*.1) * math.Exp(-sheetOmega*.1)
	if math.Abs(float64(s.Progress)-want) > 1e-6 {
		t.Fatal("later ordinary close reused a rejected throw", s.Progress, want)
	}
}

func TestSheetFitDuringSettleRetainsPaintedHeight(t *testing.T) {
	a := testApp(t)
	open, natural := true, float32(700)
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		a.navigationSheet(c, "fit-moving", &open, func() { open = false }, func() { open = false }, func() {
			ui.Box(c).FillWidth().Height(68)
			ui.Box(c).Fill()
		}, natural)
	}, 560, 800)
	tt.Advance(time.Second)
	tt.Scroll(280, 370, 0, 40)
	tt.Advance(80 * time.Millisecond)
	tt.Frame()
	s := a.snapSheet("fit-moving")
	before := s.Height.Value
	natural = 600
	tt.Frame()
	if s.Height.Value != before || s.Height.Target != 600 || math.Abs(s.Height.Velocity) > sheetOmega*math.Abs(600-before) {
		t.Fatal("remeasure jumped or relaunched without the source limiter", before, s.Height.Value, s.Height.Velocity)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if s.Height.Value != 600 {
		t.Fatal("refit never settled", s.Height.Value)
	}
}

func TestSheetPulledFooterRemainsVisibleAndClipped(t *testing.T) {
	a := testApp(t)
	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		a.navigationSheet(c, "footer-test", &open, func() { open = false }, func() { open = false }, func() {
			ui.Box(c).Fill().Children(func() {
				ui.Column(c).Fill().Children(func() {
					ui.Box(c).FillWidth().Height(68).Shrink(0).Label("footer-header")
					ui.Box(c).Fill()
				})
				ui.ButtonBase(c).Absolute().Left(16).Right(16).Bottom(16).Height(48).Label("sheet-footer")
			})
		})
	}, 560, 800)
	tt.Advance(time.Second)
	tt.Press(280, 370)
	tt.Move(280, 470)
	r, ok := tt.Find("sheet-footer")
	if !ok || r.Y+r.H != 784 {
		t.Fatal("pulled footer fell into the off-screen minimum-layout overhang", r)
	}
	tt.Move(280, 790)
	r, _ = tt.Find("sheet-footer")
	if r.Y < 770 || r.Y+r.H > 800 {
		t.Fatal("short viewport did not clip its foot control", r)
	}
}

func TestSheetProxyDockedSearchStaysClickableDuringDragRestore(t *testing.T) {
	for _, style := range []string{"list", "tab"} {
		t.Run(style, func(t *testing.T) {
			a := populatedApp(t)
			a.cfg.ProxyStyle = style
			a.openProxyPicker("Streaming")
			tt := ui.NewTester(func(c *ui.Context) { a.theme(c); a.dashboardPickerSheet(c) }, 560, 800)
			tt.Advance(time.Second)
			tt.Press(280, 370)
			tt.Move(280, 470)
			r, ok := tt.Find(a.t("search"))
			if !ok || r.Y+r.H != 784 {
				t.Fatal("native proxy search fell below the screen", r)
			}
			tt.Advance(50 * time.Millisecond)
			tt.Release(280, 470)
			tt.Advance(50 * time.Millisecond)
			s := a.snapSheet("dashboard-picker")
			if s.Progress >= 1 || s.Dismissing {
				t.Fatal("invalid intermediate restore fixture")
			}
			if err := tt.Click(a.t("search")); err != nil {
				t.Fatal(err)
			}
			tt.Type("Japan")
			if a.picker.Proxy.proxyQuery != "Japan" || s.Pressed {
				t.Fatal("restoring sheet stole its docked search click", a.picker.Proxy.proxyQuery)
			}
		})
	}
}

func TestSheetNativeProfileFitRemeasuresRowsAfterScaleChange(t *testing.T) {
	a := populatedApp(t)
	for len(a.cfg.Profiles) < 8 {
		p := a.cfg.Profiles[0]
		p.ID, p.Name = "fit-"+number(len(a.cfg.Profiles)), "Fit profile"
		a.cfg.Profiles = append(a.cfg.Profiles, p)
	}
	a.cfg.Profiles = a.cfg.Profiles[:8]
	a.cfg.Scale = 1
	a.openProfilePicker()
	tt := ui.NewTester(func(c *ui.Context) { a.theme(c); a.dashboardPickerSheet(c) }, 600, 1000)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Scroll(280, 450, 0, 40)
	s := a.snapSheet("dashboard-picker")
	if s.Height.Value != 652 { // eight single-column rows: 68 + 16 + 8*64 + 7*8
		t.Fatal("invalid initial single-column content fit", s.Height.Value)
	}
	a.cfg.Scale = .8 // two columns now fit; four rows are shorter than the minimum
	tt.Frame()
	if s.Stop != 1 || s.Height.Value != 562.5 || s.Scroll == nil || s.Scroll.MaxY > .001 {
		t.Fatal("column/text-scale change did not remeasure fit content", s.Height.Value, s.Scroll)
	}
}

func TestSheetStackEscapeBackdropAndCaptureAreTopmost(t *testing.T) {
	for _, width := range []int{560, 1200} {
		t.Run(number(width), func(t *testing.T) {
			a := testApp(t)
			lower, upper := true, false
			lowerCloses, upperCloses, actions := 0, 0, 0
			var closeLower, closeUpper func()
			closeLower = func() {
				if !a.deferSheetClose("stack-lower", closeLower) {
					lower, lowerCloses = false, lowerCloses+1
				}
			}
			closeUpper = func() {
				if !a.deferSheetClose("stack-upper", closeUpper) {
					upper, upperCloses = false, upperCloses+1
				}
			}
			tt := ui.NewTester(func(c *ui.Context) {
				a.theme(c)
				a.navigationSheet(c, "stack-lower", &lower, closeLower, closeLower, func() {
					ui.Box(c).FillWidth().Height(68)
					ui.ButtonBase(c).Size(180, 44).Label("lower-action").OnClick(func() { actions++ })
				})
				a.navigationSheet(c, "stack-upper", &upper, closeUpper, closeUpper, func() {
					ui.Box(c).FillWidth().Height(68)
					ui.ButtonBase(c).Size(180, 44).Label("upper-action")
				})
			}, width, 800)
			tt.Advance(time.Second)
			if width == 560 {
				tt.Press(280, 370)
				tt.Move(280, 390)
			}
			upper = true
			tt.Frame()
			tt.Advance(time.Second)
			if width == 560 {
				if a.snapSheet("stack-lower").Pressed || !lower || lowerCloses != 0 {
					t.Fatal("covering sheet failed to cancel the lower capture without another pointer event")
				}
				tt.Move(280, 410)
				tt.Release(280, 410)
			}
			tt.Key(0, ui.KeyEscape)
			tt.Advance(time.Second)
			if upper || !lower || upperCloses != 1 || lowerCloses != 0 {
				t.Fatal("Escape did not dismiss only the topmost sheet", lower, upper, lowerCloses, upperCloses)
			}
			if err := tt.Click("lower-action"); err != nil || actions != 1 {
				t.Fatal("lower sheet was unusable after the upper exit", err, actions)
			}
			upper = true
			a.resetSheet("stack-upper")
			tt.Frame()
			tt.Advance(time.Second)
			tt.ClickAt(20, 100)
			tt.Advance(time.Second)
			if upper || !lower || upperCloses != 2 || lowerCloses != 0 {
				t.Fatal("upper backdrop dismissed/leaked through to the lower sheet")
			}
		})
	}
}
