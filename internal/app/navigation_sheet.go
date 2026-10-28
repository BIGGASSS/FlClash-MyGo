package app

import "github.com/egoist/mygo/ui"

// A sheet can host a second page. Escape pops that page while clicking the
// backdrop dismisses the sheet, matching BaseNavigator inside the source sheet.
func (a *App) navigationSheet(c *ui.Context, key string, open *bool, close, back func(), content func(), naturalHeight ...float32) {
	if !*open {
		return
	}
	ui.Overlay(c.Key(key), func() {
		backdrop := ui.Box(c).Absolute().Left(0).Top(a.frameHeight).Right(0).Bottom(0).Modal().Background(ui.RGBA(0, 0, 0, 0.32)).OnClick(close)
		backdrop.Children(func() {
			width, height := c.Size()
			available := max(0, height-a.frameHeight)
			compact := width <= 600
			pane := ui.Column(c).Role(ui.RoleDialog).Absolute().Right(0).Background(a.colors.page).OnClick(func() {})
			shown := available
			if compact {
				s := a.snapSheet(key)
				s.Width = width
				maximum := available * .9
				if len(naturalHeight) > 0 {
					maximum = min(maximum, max(available*9/16, naturalHeight[0]))
				}
				shown = s.height(c, available, maximum)
				pane.Width(width).Height(max(available*9/16, shown)).Bottom(min(0, shown-available*9/16)).Radius(28, 28, 0, 0).Clip().HandleInput(func(e ui.InputEvent) bool { return s.input(e, available, maximum, close) })
			} else {
				pane.Top(0).Bottom(0).Width(a.sideSheetWidth(c))
			}
			pane.Children(func() {
				previousCompact, previousHeight := a.compactSheet, a.sheetContentHeight
				a.compactSheet, a.sheetContentHeight = compact, max(available*9/16, shown)
				defer func() { a.compactSheet, a.sheetContentHeight = previousCompact, previousHeight }()
				content()
			})
		})
		// Let the editor's find bar and other focused children handle Escape
		// before falling back to popping the sheet's navigation stack.
		if backdrop.OverlayShortcut(0, ui.KeyEscape) {
			back()
			c.AnimationFrame()
		}
	})
}
