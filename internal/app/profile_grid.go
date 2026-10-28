package app

import (
	"os"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

type profileGridState struct {
	List     ui.ListState
	Revealed bool
}

func profileColumns(width, spacing, scale float32) int {
	return min(3, max(1, int((width+spacing)/(300*(1+(scale-1)/2)+spacing))))
}

func (a *App) profilesGridView(c *ui.Context, state *profileGridState, width float32, picker bool) {
	if len(a.cfg.Profiles) == 0 {
		a.empty(c, "profiles", a.t("profiles"), a.t("nullProfileDesc"), func() { a.addOpen = true })
		return
	}
	w, _ := c.Size()
	spacing := float32(16)
	if w <= 600 {
		spacing = 8
	} else if w <= 840 {
		spacing = 12
	}
	spacing *= min(1, 1+(a.cfg.Scale-1)/2)
	room := max(0, width-32)
	columns := profileColumns(room, spacing, a.cfg.Scale)
	minWidth := max(1, (room-spacing*float32(columns-1))/float32(columns)-.01)
	state.List.Key = func(row int) any { return a.cfg.Profiles[row*columns].ID }
	if !picker && !state.Revealed {
		state.Revealed = true
		for i, p := range a.cfg.Profiles {
			if p.ID == a.cfg.Active {
				state.List.ScrollIntoView(i / columns)
				break
			}
		}
	}
	bottom := float32(16)
	if !picker {
		bottom += a.bottomNavigationInset(c)
	}
	top := float32(16)
	if a.sheetContentHeight > 0 {
		top = 12
	}
	if a.compactSheet {
		top = 0
	}
	ui.Box(c).Fill().Padding(top, 16, bottom, 16).Children(func() {
		rows := (len(a.cfg.Profiles) + columns - 1) / columns
		ui.List(c, &state.List, rows, func(row int) {
			gap := spacing
			if row == rows-1 {
				gap = 0
			}
			ui.Box(c).FillWidth().Padding(0, 0, gap, 0).Children(func() {
				ui.Row(c).FillWidth().Height(a.textHeight(22) + 4 + a.textHeight(18) + 20).Gap(spacing).Children(func() {
					for i := row * columns; i < (row+1)*columns; i++ {
						ui.Box(c).Grow(1).Basis(0).MinWidth(0).FillHeight().Children(func() {
							if i < len(a.cfg.Profiles) {
								a.profileGridItem(c, a.cfg.Profiles[i], minWidth)
							}
						})
					}
				})
			})
		}).Fill()
	})
}

// Group counts are shared data, not navigation state. Stale file probes cannot
// overwrite an invalidated entry after a structured/raw save.
type profileGroupCount struct {
	Updated time.Time
	Count   int
	Loaded  bool
}

func (a *App) profileGroupsCount(p store.Profile) (int, bool) {
	if a.profileGroupCounts == nil {
		a.profileGroupCounts = map[string]*profileGroupCount{}
	}
	entry := a.profileGroupCounts[p.ID]
	if entry == nil || !entry.Updated.Equal(p.Updated) {
		entry = &profileGroupCount{Updated: p.Updated}
		a.profileGroupCounts[p.ID] = entry
		s := a.store
		go func() {
			b, err := os.ReadFile(s.ProfilePath(p.ID))
			var count int
			if err == nil {
				m, parseErr := store.ParseYAML(b)
				err = parseErr
				if err == nil {
					if groups, ok := m["proxy-groups"].([]any); ok {
						count = len(groups)
					}
				}
			}
			a.update(func() {
				if a.profileGroupCounts[p.ID] == entry && err == nil {
					entry.Count, entry.Loaded = count, true
				}
			})
		}()
	}
	return entry.Count, entry.Loaded
}

func (a *App) profileGridItem(c *ui.Context, p store.Profile, width float32) {
	b := ui.ButtonBase(c.Key(p.ID)).Label(p.Name).Fill().Radius(24).Selected(p.ID == a.cfg.Active).Background(a.colors.card).TextColor(a.colors.text).Padding(10, 16).OnClick(func() { a.activateProfile(p.ID) })
	b.Children(func() {
		ui.Row(c).Fill().Gap(16).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Justify(ui.Center).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, p.Name).MinWidth(0).Shrink(1).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).FontWeight(600).SingleLine()
					if p.URL != "" && p.Expire > 0 {
						date := time.Unix(p.Expire, 0).Local()
						label := date.Format("2006-01-02")
						trailing := float32(16)
						if p.ID == a.cfg.Active {
							trailing += 24
						}
						if !p.Updated.IsZero() {
							tw, _ := c.MeasureText(0, ui.Span{Text: a.updatedAgoCompact(p.Updated), Size: fontSize(c, 13)})
							trailing += tw + 8
						}
						available := width - 32 - trailing
						titleWidth, _ := c.MeasureText(0, ui.Span{Text: p.Name, Size: fontSize(c, 17), Weight: 600})
						dateWidth, _ := c.MeasureText(0, ui.Span{Text: label, Size: fontSize(c, 12), Weight: 600})
						if available-dateWidth-24 >= min(titleWidth, available/2) {
							color := a.colors.accent
							if date.Before(time.Now()) {
								color = a.colors.danger
							}
							ui.Text(c, label).Shrink(0).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(14)).FontWeight(600).Padding(2, 8).Radius(999).Background(color.Alpha(.15)).TextColor(color).Tooltip(a.t("expireTime"))
						}
					}
				})
				label := a.t("local")
				if p.Type == "custom" {
					label = a.t("custom")
				} else if p.URL != "" {
					label = a.t("remote")
				}
				if p.URL != "" && p.Total > 0 {
					label = compactTraffic(p.Upload+p.Download) + " / " + compactTraffic(p.Total)
				}
				ui.Text(c, label).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).TextColor(a.colors.muted).SingleLine()
			})
			ui.Row(c).Gap(8).Children(func() {
				if p.Type == "custom" {
					if count, ok := a.profileGroupsCount(p); ok {
						ui.Text(c, compactNumber(count)).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(14)).FontWeight(600).Padding(2, 8).Radius(8).Background(a.colors.accent.Alpha(.15)).TextColor(a.colors.accent).Tooltip(a.messageText("proxyGroupsCount", map[string]any{"count": count, "label": compactNumber(count)}))
					}
				} else if a.profileUpdating[p.ID] {
					loading(c, 24, a.colors.accent, a.t("loading"))
				} else if p.Type != "custom" {
					label := ""
					if !p.Updated.IsZero() {
						label = a.updatedAgoCompact(p.Updated)
					}
					ui.Text(c, label).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).TextColor(a.colors.muted).SingleLine()
				}
				if p.ID == a.cfg.Active {
					ui.Box(c).Size(24, 24).Radius(999).Background(a.colors.accent).Center().Children(func() { glyphVariant(c, "check", true).FontSize(18).TextColor(a.colors.onAccent) })
				}
			})
		})
	})
	b.ContextMenu(func(m *ui.Menu) {
		if a.profileUpdating[p.ID] {
			return
		}
		if m.Item(a.t("edit")).Chosen() {
			page := "profileEdit"
			if p.Type == "custom" {
				page = "profileCustom"
			}
			a.openProfileAction(p, page)
		}
		if p.Type == "custom" {
			if m.Item(a.t("rename")).Chosen() {
				a.openProfileAction(p, "profileRename")
			}
			if m.Item(a.t("runtimeConfig")).Chosen() {
				a.previewPickerProfile(p)
			}
		} else {
			if m.Item(a.t("extend")).Chosen() {
				a.openProfileAction(p, "profileExtend")
			}
		}
		if p.URL != "" && m.Item(a.t("sync")).Chosen() {
			a.updateProfile(p)
		}
		if p.Type != "custom" {
			m.Submenu(a.t("more"), func(sub *ui.Menu) {
				if sub.Item(a.t("runtimeConfig")).Chosen() {
					a.previewPickerProfile(p)
				}
				if p.URL != "" && sub.Item(a.t("copyLink")).Chosen() {
					c.Services().WriteClipboard(p.URL)
					a.message = a.t("copySuccess")
				}
				if profileShareable(p) && sub.Item(a.t("qrcode")).Chosen() {
					a.openProfileShare(p)
				}
				if sub.Item(a.t("exportFile")).Chosen() {
					a.exportProfile(p)
				}
			})
		}
		m.Separator()
		if m.Item(a.t("delete")).Chosen() {
			a.deleteProfile(p)
		}
	})
}
