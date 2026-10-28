package app

import (
	"fmt"
	"slices"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
	"github.com/rivo/uniseg"
)

type onDemandViewState struct {
	List     ui.ListState
	Selected map[string]bool
	Open     bool
	Editing  bool
	Original string
	Draft    string
	Error    string
	Serial   uint64
}

func (a *App) editSSID(ssid string, editing bool) {
	s := &a.onDemand.View
	s.Serial++
	s.Open, s.Editing, s.Original, s.Draft, s.Error = true, editing, ssid, ssid, ""
}

func (a *App) saveSSID() {
	s := &a.onDemand.View
	if err := store.ValidateSSID(s.Draft); err != nil {
		s.Error = err.Error()
		switch {
		case s.Draft == "":
			s.Error = a.messageText("emptyTip", map[string]any{"label": "SSID"})
		case uniseg.GraphemeClusterCount(s.Draft) > 32:
			s.Error = a.messageText("maxLengthTip", map[string]any{"label": "SSID", "max": 32})
		}
		return
	}
	next := slices.Clone(a.cfg.ExcludeSSIDs)
	if s.Editing && !slices.Contains(next, s.Original) {
		s.Error = a.t("editorContextChanged")
		return
	}
	if s.Draft != s.Original || !s.Editing {
		if slices.Contains(next, s.Draft) {
			s.Error = a.messageText("existsTip", map[string]any{"label": "SSID"})
			return
		}
		// The original removes the old set entry and appends the new spelling.
		if s.Editing {
			next = slices.DeleteFunc(next, func(value string) bool { return value == s.Original })
		}
		next = append(next, s.Draft)
	}
	if err := a.setExcludeSSIDs(next); err != nil {
		s.Error = err.Error()
		return
	}
	s.Open = false
}

func (a *App) removeSelectedSSIDs() {
	s := &a.onDemand.View
	next := slices.DeleteFunc(slices.Clone(a.cfg.ExcludeSSIDs), func(value string) bool { return s.Selected[value] })
	if err := a.setExcludeSSIDs(next); err != nil {
		a.err = err.Error()
		return
	}
	s.Selected = nil
}

func (a *App) reorderSSIDs(rows []int, to int) {
	selected := map[int]bool{}
	for _, i := range rows {
		selected[i] = true
	}
	var moved, rest []string
	before := 0
	for i, ssid := range a.cfg.ExcludeSSIDs {
		if selected[i] {
			moved = append(moved, ssid)
			if i < to {
				before++
			}
		} else {
			rest = append(rest, ssid)
		}
	}
	at := min(max(to-before, 0), len(rest))
	next := append(append(append([]string{}, rest[:at]...), moved...), rest[at:]...)
	if err := a.setExcludeSSIDs(next); err != nil {
		a.err = err.Error()
	}
}

func (a *App) onDemandView(c *ui.Context) {
	s := &a.onDemand.View
	if s.Selected == nil {
		s.Selected = map[string]bool{}
	}
	for ssid := range s.Selected {
		if !slices.Contains(a.cfg.ExcludeSSIDs, ssid) {
			delete(s.Selected, ssid)
		}
	}
	// Status/permission text can exceed a minimum desktop window. Keep a
	// usable list viewport and let the whole page scroll instead of shrinking
	// the list to zero beneath those panels.
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().MinHeightPercent(100).Padding(8, 16, 16+a.bottomNavigationInset(c), 16).Gap(12).Children(func() {
			if a.onDemand.Error != "" {
				a.section(c, a.t("locationPermission"), func() {
					ui.Text(c, a.onDemand.Error).Padding(16).TextColor(a.colors.danger).Selectable()
				})
			}
			if a.onDemand.Known && a.onDemand.State.Connected {
				a.section(c, "Wi-Fi", func() {
					a.row(c, "wifi", a.onDemand.State.SSID, func() {
						if a.wantRunning && a.onDemandSuspended() {
							a.chip(c, a.t("suspended"), a.colors.warning)
						}
					})
				})
			}
			a.profileSectionHeader(c, a.t("excludeSsids"), func() {
				if len(s.Selected) > 0 {
					a.sheetButton(c, "trash", a.t("delete"), a.removeSelectedSSIDs)
					a.sheetButton(c, "check", a.t("selectAll"), func() {
						if len(s.Selected) == len(a.cfg.ExcludeSSIDs) {
							s.Selected = map[string]bool{}
							return
						}
						for _, ssid := range a.cfg.ExcludeSSIDs {
							s.Selected[ssid] = true
						}
					})
				} else {
					a.sheetButton(c, "plus", a.t("add"), func() { a.editSSID("", false) })
				}
			})
			if len(a.cfg.ExcludeSSIDs) == 0 {
				ui.Box(c).FillWidth().Grow(1).MinHeight(56).Center().Children(func() { ui.Text(c, a.t("ssidsEmpty")).TextColor(a.colors.muted) })
			} else {
				s.List.Key = func(i int) any { return a.cfg.ExcludeSSIDs[i] }
				s.List.Reorder = a.reorderSSIDs
				ui.List(c, &s.List, len(a.cfg.ExcludeSSIDs), func(i int) {
					ssid := a.cfg.ExcludeSSIDs[i]
					selected := s.Selected[ssid]
					row := ui.ButtonBase(c.Key(ssid)).Label(ssid).FillWidth().MinHeight(56).Padding(10, 16).Radius(24).Background(a.colors.card).OnClick(func() {
						if len(s.Selected) > 0 {
							if selected {
								delete(s.Selected, ssid)
							} else {
								s.Selected[ssid] = true
							}
						} else {
							a.editSSID(ssid, true)
						}
					})
					row.Children(func() {
						ui.Row(c).FillWidth().Gap(16).Children(func() {
							if len(s.Selected) > 0 {
								icon := "circle"
								if selected {
									icon = "check"
								}
								glyph(c, icon).FontSize(22).TextColor(a.colors.accent)
							}
							ui.Text(c, ssid).Grow(1).MinWidth(0).FontSize(fontSize(c, 17)).MaxLines(2)
						})
					})
					row.ContextMenu(func(menu *ui.Menu) {
						if menu.Item(a.t("select")).Chosen() {
							s.Selected[ssid] = true
						}
						if menu.Item(a.t("edit")).Chosen() {
							a.editSSID(ssid, true)
						}
						if menu.Item(a.t("delete")).Chosen() {
							s.Selected[ssid] = true
							a.removeSelectedSSIDs()
						}
					})
				}).FillWidth().Grow(1).MinHeight(112)
			}
			ui.Text(c, a.t("excludeSsidsDesc")).FillWidth().FontSize(fontSize(c, 13)).TextColor(a.colors.muted)
		})
	})
}

func (a *App) onDemandDialog(c *ui.Context) {
	s := &a.onDemand.View
	a.dialogBase(c.Key(fmt.Sprintf("ssid-dialog/%d", s.Serial)), &s.Open, func(back, panel ui.Element) {
		back.Background(ui.RGBA(0, 0, 0, .32))
		panel.Width(332).MaxWidthPercent(90).Padding(24).Gap(20).Radius(34).Background(a.colors.card)
		title := a.t("addSsid")
		if s.Editing {
			title = a.t("editSsid")
		}
		ui.Text(c, title).FontSize(fontSize(c, 20)).FontWeight(600)
		ui.TextInput(c, &s.Draft).FillWidth().Label("SSID").AutoFocus().OnSubmit(a.saveSSID)
		if s.Error != "" {
			ui.Text(c, s.Error).TextColor(a.colors.danger).FontSize(fontSize(c, 13))
		}
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			ui.Button(c, a.t("cancel")).OnClick(func() { s.Open = false })
			ui.PrimaryButton(c, a.t("confirm")).OnClick(a.saveSSID)
		})
	})
}
