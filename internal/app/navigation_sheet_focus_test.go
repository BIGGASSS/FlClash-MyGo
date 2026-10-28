package app

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSheetContainerFocusDoesNotPaintAWholePaneRing(t *testing.T) {
	a := testApp(t)
	open, focus, focused := true, false, false
	tt := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		ui.Box(c).Fill().Background(a.colors.page)
		a.navigationSheet(c, "focus-pane", &open, func() { open = false }, func() { open = false }, func() {})
		if focus {
			pane := c.Resolve(a.snapSheet("focus-pane").Focus)
			pane.Focus()
			focused = pane.Focused()
		}
	}, 1200, 800)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	before := make([]color.RGBA, 8)
	for i := range before {
		before[i] = tt.Image().RGBAAt(716+i, 400)
	}
	focus = true
	tt.Frame()
	tt.Frame()
	after := tt.Image()
	if !focused {
		t.Fatal("test did not focus the gesture-ownership container")
	}
	// The container is a focus target for gesture ownership, not an editable
	// control. Keyboard use must not draw an accent line down its entire edge.
	for x := 716; x <= 723; x++ {
		if before[x-716] != after.RGBAAt(x, 400) {
			t.Fatal("focusing the dialog container painted a pane-wide ring", x)
		}
	}
}
