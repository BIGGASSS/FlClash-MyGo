package app

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestResponsiveSidebarKeepsPreferenceAndOpensOverlay(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled = true
	tt := ui.NewTester(a.View, 820, 800)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if a.sidebarSize() != 48 || !a.cfg.Sidebar {
		t.Fatal("laptop sidebar did not preserve expansion preference", a.sidebarSize())
	}
	if err := tt.Click(a.t("showMore")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.sidebarOverlay.Open || a.sidebarSize() != 48 || !a.cfg.Sidebar {
		t.Fatal("laptop toggle changed the body width or preference")
	}
	tt.ClickAt(600, 400)
	tt.Frame()
	if a.sidebarOverlay.Mounted {
		t.Fatal("sidebar overlay did not dismiss")
	}
	tt.SetSize(1200, 800)
	if a.sidebarSize() != 220 {
		t.Fatal("widening lost expanded sidebar", a.sidebarSize())
	}
	tt.SetSize(380, 600)
	if a.sidebarSize() != 0 {
		t.Fatal("small desktop kept a sidebar", a.sidebarSize())
	}
	if _, ok := tt.Find(a.t("navigationBarStyle")); !ok {
		t.Fatal("small desktop has no navigation")
	}
}

func TestSmallDesktopDockNavigatesAndRetainsTabRoutes(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled = true
	tt := ui.NewTester(a.View, 380, 600)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	r, ok := tt.Find(a.t("tools"))
	if !ok || r.W < 50 || r.Y < 500 {
		t.Fatal("tools destination is inaccessible", r)
	}
	if err := tt.Click(a.t("tools")); err != nil || a.page != "tools" {
		t.Fatal("dock tap did not navigate", err, a.page)
	}
	a.goPage("general")
	tt.Frame()
	if _, ok := tt.Find(a.t("navigationBarStyle")); ok {
		t.Fatal("small desktop settings retained home navigation")
	}
	if err := tt.Click(a.t("back")); err != nil || a.page != "tools" {
		t.Fatal("small settings route has no back action", err)
	}
	tt.SetSize(1200, 600)
	a.goPage("general")
	tt.Frame()
	if err := tt.Click(a.t("profiles")); err != nil || a.page != "profiles" {
		t.Fatal("profiles unavailable from settings", err, a.page)
	}
	if err := tt.Click(a.t("tools")); err != nil || a.page != "general" {
		t.Fatal("tab switch lost its nested route", err, a.page)
	}
	tt.SetSize(600, 600)
	if a.sidebarSize() != 0 {
		t.Fatal("600-point window did not use bottom navigation")
	}
	tt.SetSize(601, 600)
	if a.sidebarSize() != 48 {
		t.Fatal("laptop boundary did not restore compact sidebar")
	}
}

func TestDockDragCommitsOnlyAtReleaseAndCapsFling(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled = true
	tt := ui.NewTester(a.View, 380, 600)
	pages := a.navigationPages(true)
	start := time.Unix(1700000000, 0)
	a.navigationDockInput(ui.InputEvent{Kind: ui.InputPointerDown, Button: 0, X: 40, Time: start}, 338, pages)
	for i, x := range []float32{70, 100, 130} {
		a.navigationDockInput(ui.InputEvent{Kind: ui.InputPointerMove, Button: 0, X: x, Time: start.Add(time.Duration(i+1) * 33 * time.Millisecond)}, 338, pages)
	}
	if a.page != "dashboard" || !a.navigationDock.Dragging {
		t.Fatal("dock navigated before release", a.page)
	}
	a.navigationDockInput(ui.InputEvent{Kind: ui.InputPointerUp, Button: 0, X: 130, Time: start.Add(110 * time.Millisecond)}, 338, pages)
	if a.page != "profiles" {
		t.Fatal("fling did not advance at most one destination", a.page)
	}
	tt.Frame()
}

func TestPointerVelocityMatchesFlutter(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-navigation-velocities.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Times     []int
		Positions []float64
		Velocity  float64
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	start := time.Unix(1700000000, 0)
	for _, row := range rows {
		var samples []pointerSample
		for i, ms := range row.Times {
			samples = append(samples, pointerSample{X: row.Positions[i], At: start.Add(time.Duration(ms) * time.Millisecond)})
		}
		now := samples[len(samples)-1].At
		if got := pointerVelocity(samples, now); math.Abs(got-row.Velocity) > 1e-7 {
			t.Fatal("velocity differs from original", got, row.Velocity)
		}
		if pointerVelocity(samples, now.Add(41*time.Millisecond)) != 0 {
			t.Fatal("stopped pointer retained a fling")
		}
	}
}

func TestSmallDesktopCaptionAndSheetBounds(t *testing.T) {
	for _, platform := range []string{"linux", "windows", "darwin"} {
		a := populatedApp(t)
		a.frameEnabled, a.framePlatform = true, platform
		a.openProfileDetails()
		tt := ui.NewTester(a.View, 380, 600)
		tt.Advance(time.Second)
		want := float32(32)
		if platform == "darwin" {
			want = 28
		}
		if a.frameHeight != want || a.sidebarSize() != 0 {
			t.Fatal("small window chrome differs", platform, a.frameHeight, a.sidebarSize())
		}
		r, ok := tt.Find(a.t("runtimeConfig"))
		if !ok || r.Y != want+(600-want)*7/16+12 || r.X+r.W > 380 {
			t.Fatal("sheet escaped small desktop", platform, r)
		}
	}
}

func TestCompactNestedCollectionWheelTracksDocumentList(t *testing.T) {
	a := populatedApp(t)
	a.openProfilePicker()
	nav := a.beginProfilePickerNavigation()
	rows := make([]any, 100)
	for i := range rows {
		rows[i] = "Rule " + number(i)
	}
	a.openCollection("Rules", "global:rules", rows)
	tt := ui.NewTester(a.View, 560, 800)
	tt.Advance(time.Second)
	s := a.snapSheet("dashboard-picker")
	if s.Scroll == nil || s.Scroll == &nav.Scroll || s.Scroll != s.Scrolls[a.document.List.Handle] || s.Scroll.MaxY <= 0 {
		t.Fatal("nested collection did not bind its actual document list", s.Scroll, nav.Page, a.document.List.Handle, tt.Texts())
	}
	s.Scroll.Y = 120
	tt.Frame()
	tt.Scroll(280, 650, 0, 80)
	tt.Advance(time.Second)
	if s.Stop != 0 || s.Height.Value != 450 || s.Scroll.Y <= 120 {
		t.Fatal("wheel in scrolled collection resized the sheet", s.Stop, s.Scroll.Y)
	}
	s.Scroll.Y = 0
	tt.Frame()
	tt.Scroll(280, 650, 0, 80)
	tt.Advance(time.Second)
	if s.Stop != 1 || s.Height.Value != 720 || s.Scroll.Y != 0 {
		t.Fatal("wheel at collection top did not hand off to the upper detent", s.Stop, s.Height.Value, s.Scroll.Y)
	}
	tt.Scroll(280, 650, 0, 80)
	tt.Advance(time.Second)
	if s.Stop != 1 || s.Scroll.Y <= 0 || a.document == nil || a.profilePickerNavigation() != nav {
		t.Fatal("upper-detent wheel did not return to the owned collection")
	}
}

func TestCompactSheetDoesNotInterceptEditorWheel(t *testing.T) {
	for _, kind := range []string{"details", "picker"} {
		t.Run(kind, func(t *testing.T) {
			a := populatedApp(t)
			key := "profile-details"
			if kind == "details" {
				a.openProfileDetails()
				a.profileDetail.Preview = true
				a.profileDetail.Code = &editor.State{Buffer: editor.NewBuffer("short document"), ReadOnly: true}
			} else {
				key = "dashboard-picker"
				a.openProfilePicker()
				a.beginProfilePickerNavigation()
				a.openCode("Preview", "raw", "short document")
			}
			tt := ui.NewTester(a.View, 560, 800)
			tt.Advance(time.Second)
			s := a.snapSheet(key)
			if s.Scroll != nil {
				t.Fatal("editor inherited another page's sheet scroller")
			}
			// The editor's 16-point padding has no inner input handler.
			// Bubbling wheel input there must still belong to the editor page.
			tt.Scroll(8, 600, 0, 80)
			tt.Advance(time.Second)
			if s.Stop != 0 || s.Height.Value != 450 || !s.LastWheel.IsZero() {
				t.Fatal("wheel over editor content was intercepted by the sheet")
			}
			tt.Scroll(280, 370, 0, 80) // The 68-point header still controls detents.
			tt.Advance(time.Second)
			if s.Stop != 1 || s.Height.Value != 720 {
				t.Fatal("editor page lost header wheel control")
			}
		})
	}
}

func TestDesktopTabSwitchRetainsConnectionViewState(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled = true
	a.selectNavigation("connections")
	a.recordQuery, a.recordFilter, a.recordSelected, a.paused = "github.com", "tcp", 2, true
	a.selectNavigation("tools")
	if a.recordQuery != "" || a.paused {
		t.Fatal("record state leaked into another tab")
	}
	a.selectNavigation("connections")
	if a.recordQuery != "github.com" || a.recordFilter != "tcp" || a.recordSelected != 2 || !a.paused {
		t.Fatal("tab switch reset its connection view")
	}
}

func TestDesktopTabsRetainIndependentDirtyEditors(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled = true
	a.selectNavigation("profiles")
	a.openCode("Profile", "profile:"+a.cfg.Active, "original profile")
	a.code.Buffer.Set("dirty profile")
	profileCode := a.code
	a.selectNavigation("tools")
	a.openCode("Script", "script:fixture", "original script")
	a.code.Buffer.Set("dirty script")
	scriptCode := a.code
	a.selectNavigation("profiles")
	if a.code != profileCode || a.code.Buffer.String() != "dirty profile" || a.codeOriginal != "original profile" || a.codePurpose != "profile:"+a.cfg.Active {
		t.Fatal("another tab replaced the profile editor")
	}
	a.selectNavigation("tools")
	if a.code != scriptCode || a.code.Buffer.String() != "dirty script" || a.codeOriginal != "original script" || a.codePurpose != "script:fixture" {
		t.Fatal("another tab replaced the script editor")
	}
}
