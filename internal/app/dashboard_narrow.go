package app

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func (a *App) dashboardNarrow(c *ui.Context) bool {
	w, _ := c.Size()
	return (w-a.sidebarSize())/max(a.cfg.Scale, 1) < 640
}

func (a *App) dashboardGap(c *ui.Context) float32 {
	w, _ := c.Size()
	gap := float32(16)
	if w <= 600 {
		gap = 8
	} else if w <= 840 {
		gap = 12
	}
	return gap * min(1+(a.cfg.Scale-1)/2, 1)
}

var dashboardLabels = map[string]string{
	"currentOutbound": "status", "networkSpeed": "networkSpeed", "outboundMode": "outboundMode",
	"networkTakeover": "networkTakeover", "intranetIp": "intranetIP", "serviceStatus": "serviceStatus",
	"profiles": "profile", "proxyGroups": "proxyGroup", "connectionStats": "connectionStats", "memoryInfo": "memoryInfo",
}
var dashboardGlyphs = map[string]string{
	"currentOutbound": "status", "networkSpeed": "speed", "outboundMode": "split", "networkTakeover": "shuffle",
	"intranetIp": "devices", "serviceStatus": "verified", "profiles": "profiles", "proxyGroups": "layers",
	"connectionStats": "connections", "memoryInfo": "memory",
}

func (a *App) dashboardWidgets() []string {
	widgets := []string{}
	for _, name := range store.NormalizeDashboardWidgets(a.cfg.DashboardWidgets) {
		if name != "vpnButton" {
			widgets = append(widgets, name)
		}
	}
	return widgets
}

func (a *App) saveDashboardWidgets(shown []string) {
	// Preserve identities belonging to other platforms, as the original editor
	// does when moving the cards around them in a shared backup.
	result := slices.Clone(shown)
	at := 0
	for _, name := range a.cfg.DashboardWidgets {
		if name == "vpnButton" {
			result = slices.Insert(result, at, name)
			at++
		} else if index := slices.Index(result, name); index >= 0 {
			at = index + 1
		}
	}
	a.cfg.DashboardWidgets = store.NormalizeDashboardWidgets(result)
	a.save()
}

func (a *App) narrowDashboard(c *ui.Context) {
	width, _ := c.Size()
	room := width - a.sidebarSize()
	side := max(float32(16), (room-560)/2)
	cardWidth := room - side*2
	widgets := a.dashboardWidgets()
	gap := a.dashboardGap(c)
	if !a.dashboardEditing {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(16, side, 16+a.bottomNavigationInset(c), side).Gap(gap).Children(func() {
				for _, name := range widgets {
					ui.Box(c.Key(name)).FillWidth().Children(func() { a.narrowDashboardCard(c, name, cardWidth) })
				}
			})
		})
		return
	}
	addable := []string{}
	for _, name := range store.DashboardWidgetNames() {
		if name != "vpnButton" && !slices.Contains(widgets, name) {
			addable = append(addable, name)
		}
	}
	a.dashboardList.Key = func(i int) any {
		if i < len(widgets) {
			return widgets[i]
		}
		if i == len(widgets) {
			return "addable-section"
		}
		return "add/" + addable[i-len(widgets)-1]
	}
	a.dashboardList.Label = func(i int) string {
		if i < len(widgets) {
			return a.t(dashboardLabels[widgets[i]])
		}
		if i == len(widgets) {
			return a.t("dashboardAddableSection")
		}
		return a.t(dashboardLabels[addable[i-len(widgets)-1]])
	}
	a.dashboardList.Header = func(i int) bool { return i >= len(widgets) }
	a.dashboardList.Reorder = func(rows []int, to int) {
		var moved, rest []string
		before := 0
		for index, name := range widgets {
			if slices.Contains(rows, index) {
				moved = append(moved, name)
				if index < to {
					before++
				}
			} else {
				rest = append(rest, name)
			}
		}
		at := min(max(to-before, 0), len(rest))
		a.saveDashboardWidgets(append(append(append([]string{}, rest[:at]...), moved...), rest[at:]...))
	}
	n := len(widgets)
	if len(addable) > 0 {
		n += 1 + len(addable)
	}
	ui.List(c, &a.dashboardList, n, func(i int) {
		if i >= len(widgets) {
			if i == len(widgets) {
				ui.Text(c, a.t("dashboardAddableSection")).Padding(18, 8, 10, 8).FontSize(fontSize(c, 13)).FontWeight(600).TextColor(a.colors.muted)
			} else {
				name := addable[i-len(widgets)-1]
				ui.Row(c).FillWidth().Height(64).Margin(0, 0, 8, 0).Padding(0, 8, 0, 12).Radius(22).Background(a.colors.card.Alpha(0.55)).Gap(12).Children(func() {
					a.button(c, "add", a.t("add")+" "+a.t(dashboardLabels[name]), func() { a.saveDashboardWidgets(append(widgets, name)) }).Size(28, 28).Padding(4).Background(a.colors.accent).TextColor(a.colors.onAccent)
					ui.Box(c).Size(40, 40).Radius(14).Background(a.colors.card).Center().Children(func() { glyph(c, dashboardGlyphs[name]).FontSize(20).TextColor(a.colors.muted) })
					ui.Text(c, a.t(dashboardLabels[name])).Grow(1).FontSize(fontSize(c, 15)).FontWeight(600)
				})
			}
			return
		}
		name := widgets[i]
		card := ui.Box(c.Key(name)).FillWidth().Margin(0, 0, gap, 0).TextColor(a.colors.text)
		if !c.Preferences().ReduceMotion {
			seed := slices.Index(store.DashboardWidgetNames(), name)
			swings := []int{6, 7, 5}[seed%3]
			turn := float64(card.Loop("sway", 3600*time.Millisecond, ui.Linear))*2*math.Pi*float64(swings) + float64(seed)*1.7
			card.Rotate(float32(math.Sin(turn)*0.7)).Translate(0, float32(math.Cos(turn)*1.25))
		}
		card.Children(func() {
			a.narrowDashboardCard(c, name, cardWidth)
			ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).OnClick(func() {})
			if name != "currentOutbound" {
				a.button(c, "remove", a.t("remove")+" "+a.t(dashboardLabels[name]), func() {
					a.saveDashboardWidgets(slices.Delete(slices.Clone(widgets), i, i+1))
				}).Absolute().Right(0).Top(0).Size(28, 28).Padding(4).Background(a.colors.dangerContainer).TextColor(a.colors.onDangerContainer)
			}
		})
	}).Fill().Padding(16, side, 16+a.bottomNavigationInset(c), side)
}

func (a *App) narrowDashboardCard(c *ui.Context, name string, width float32) {
	switch name {
	case "currentOutbound":
		a.narrowStatusCard(c)
	case "networkSpeed":
		a.narrowTrafficCard(c, width)
	case "outboundMode":
		a.narrowModeCard(c)
	case "networkTakeover":
		a.narrowTakeoverCard(c)
	case "intranetIp":
		a.narrowIntranetCard(c)
	case "memoryInfo":
		a.memoryCard(c).FillWidth()
	case "serviceStatus":
		a.narrowServicesCard(c)
	case "profiles":
		a.narrowProfileCard(c)
	case "proxyGroups":
		a.narrowGroupsCard(c)
	case "connectionStats":
		a.connectionStatsCard(c).FillWidth().Height(38 + a.textHeight(28) + a.textHeight(16))
	}
}

func (a *App) narrowStatusCard(c *ui.Context) {
	scale := max(a.cfg.Scale, 1)
	card := a.dashboardCard(c).Label(a.t("status")).FillWidth().Padding(18, 16, 18, 20).Gap(0).Clip()
	target := float32(0)
	if a.wantRunning {
		target = 1
	}
	progress := card.AnimateWith("decoration", target, 450*time.Millisecond, ui.CubicBezier(0.175, 0.885, 0.32, 1.275))
	card.Draw(func(p *ui.Painter, r ui.Rect) {
		size := 240 * (1 + 0.12*progress)
		box := ui.Rect{X: r.X + r.W + 60 - 240 - (size-240)/2, Y: r.Y - 66 - (size-240)/2, W: size, H: size}
		color := a.colors.block.Mix(a.colors.card.Mix(a.colors.accent, 0.1), min(max(progress, 0), 1))
		p.Clip(r, 34, func() { p.FillPath(dashboardShape("cookie12Sided", box, 0, progress*30), color) })
	}).Children(func() {
		ui.Row(c).FillWidth().Gap(10).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
				ui.Row(c).Height(26 * scale).Gap(8).Children(func() {
					glyph(c, "status").FontSize(18).TextColor(a.colors.muted)
					ui.Text(c, a.t("status")).FontSize(fontSize(c, 15)).FontWeight(600)
					label, color := a.t("notRunning"), a.colors.muted
					if a.wantRunning {
						label, color = a.t("running"), a.colors.success
						if a.onDemandSuspended() {
							label, color = a.t("suspended"), a.colors.warning
						}
					}
					a.dashboardChip(c, label, color, false, a.wantRunning && !a.onDemandSuspended())
				})
				ui.Row(c).FillWidth().Margin(12, 0, 0, 0).Gap(12).Children(func() {
					ui.ButtonBase(c).Label(a.t("refresh")).Size(48, 48).Center().OnClick(a.outboundIP).Draw(func(p *ui.Painter, r ui.Rect) { p.FillPath(dashboardShape("clover4Leaf", r, 0, 0), a.colors.chrome) }).Children(func() { ui.Text(c, a.country).FontSize(fontSize(c, 12)).FontWeight(600) })
					ui.ButtonBase(c).Label(a.shownIP(a.ip)).Grow(1).MinWidth(0).OnClick(func() { a.showQuality(a.ip) }).Children(func() {
						ui.Column(c).FillWidth().Gap(4).Children(func() {
							ui.Text(c, a.shownIP(a.ip)).Height(28 * scale).FontSize(fontSize(c, 22)).FontWeight(600).SingleLine()
							ui.Row(c).Height(28).FillWidth().Clip().Gap(6).Children(func() {
								if quality := a.qualities[a.ip].Quality; quality != nil {
									a.dashboardChip(c, a.qualityType(quality.Type), a.colors.muted, true, false)
									color := a.colors.muted
									if quality.Level() == "good" {
										color = a.colors.success
									} else if quality.Level() == "risky" {
										color = a.colors.warning
									}
									a.dashboardChip(c, a.t("ipQualityShort")+" "+a.qualityLabel(quality.Level()), color, true, false)
								}
							})
						})
					})
				})
				ui.Row(c).FillWidth().Margin(16, 0, 0, 0).Gap(24).Children(func() {
					for index, title := range []string{"runTime", "organization"} {
						column := ui.Column(c).Gap(2)
						if index > 0 {
							column.Grow(1).MinWidth(0)
						}
						column.Children(func() {
							ui.Text(c, a.t(title)).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(a.colors.muted).SingleLine()
							value := a.organization
							if index == 0 {
								value = elapsed(a.started)
							}
							if value == "" {
								value = "—"
							}
							ui.Text(c, value).FontSize(fontSize(c, 15)).FixedLineHeight(a.textHeight(20)).FontWeight(600).SingleLine()
						})
					}
				})
			})
			a.dashboardStartControl(c, 88)
		})
	})
}

func (a *App) narrowTrafficCard(c *ui.Context, width float32) {
	scale := max(a.cfg.Scale, 1)
	tileWidth := (width - 40 - 10) / 2
	chartHeight := max(36, tileWidth/1.06-28-68*scale)
	titleHeight, figureHeight, totalHeight := max(16, a.textHeight(18)), a.textHeight(34), a.textHeight(16)
	tileHeight := 28 + titleHeight + figureHeight + totalHeight + chartHeight
	a.dashboardCard(c).Label(a.t("networkSpeed")).FillWidth().Gap(0).Children(func() {
		a.dashboardHeader(c, "speed", a.t("networkSpeed"), nil)
		ui.Row(c).FillWidth().Margin(12, 0, 0, 0).Gap(10).Children(func() {
			for _, upload := range []bool{true, false} {
				value, total, history, color, icon, label := a.sample.Traffic.Down, a.sample.Total.Down, a.downHistory, a.colors.accent, "arrowDown", "download"
				if upload {
					value, total, history, color, icon, label = a.sample.Traffic.Up, a.sample.Total.Up, a.upHistory, a.colors.card.Mix(a.colors.accent, 0.45), "arrowUp", "upload"
				}
				glyphColor := a.colors.accent
				if upload {
					glyphColor = a.colors.muted
				}
				ui.Column(c).Grow(1).MinWidth(0).Height(tileHeight).Radius(22).Background(a.colors.block).Clip().Padding(14, 16, 0, 16).Gap(0).Children(func() {
					ui.Row(c).Height(titleHeight).Gap(4).Children(func() {
						glyph(c, icon).FontSize(16).TextColor(glyphColor)
						ui.Text(c, a.t(label)).FontSize(fontSize(c, 13)).FontWeight(600).TextColor(a.colors.muted)
					})
					parts := strings.SplitN(bytesText(value), " ", 2)
					ui.Row(c).Height(figureHeight).Margin(4, 0, 0, 0).AlignItems(ui.Baseline).Gap(3).Children(func() {
						ui.Text(c, parts[0]).FontSize(fontSize(c, 28)).FixedLineHeight(figureHeight).FontWeight(600)
						ui.Text(c, parts[1]+"/s").FontSize(fontSize(c, 13)).FontWeight(600).TextColor(a.colors.muted)
					})
					ui.Text(c, a.t("total")+" "+bytesText(total)).Height(totalHeight).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
					ui.Box(c).Width(tileWidth+4).Height(chartHeight).Margin(12, -18, -2, -18).Draw(func(p *ui.Painter, r ui.Rect) { plotTraffic(p, r, history, color) })
				})
			}
		})
	})
}

func (a *App) narrowModeCard(c *ui.Context) {
	a.dashboardCard(c).Label(a.t("outboundMode")).FillWidth().Gap(0).Children(func() {
		a.dashboardHeader(c, "split", a.t("outboundMode"), nil)
		ui.Row(c).FillWidth().Height(52).Margin(12, 0, 0, 0).Gap(2).Children(func() {
			for i, mode := range []string{"rule", "global", "direct"} {
				selected := store.String(a.cfg.Overrides["mode"]) == mode
				button := ui.ButtonBase(c.Key(mode)).Label(a.t(mode)).Grow(1).MinWidth(0).FillHeight().Center().OnClick(func() { a.performHotkey(mode + "Mode") })
				a.connectedModeStyle(button, i, selected, 26, 12)
				button.Children(func() {
					ui.Column(c).Center().Gap(2).Children(func() {
						glyph(c, []string{"rules", "proxies", "target"}[i]).FontSize(18)
						ui.Text(c, a.t(mode)).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).FontWeight(600).SingleLine()
					})
				})
			}
		})
	})
}

func (a *App) connectedModeStyle(button ui.Element, index int, selected bool, outer, inner float32) {
	target := float32(0)
	if selected {
		target = 1
	}
	amount := button.AnimateWith("selection", target, 240*time.Millisecond, sidebarLead)
	start, end := inner, inner
	if index == 0 {
		start = outer
	}
	if index == 2 {
		end = outer
	}
	start += (outer - start) * amount
	end += (outer - end) * amount
	button.Background(a.colors.block.Mix(a.colors.accent, amount)).TextColor(a.colors.text.Mix(a.colors.onAccent, amount)).Radius(start, end, end, start)
}

func (a *App) narrowSwitchRow(c *ui.Context, icon, label, subtitle string, value bool, change func(bool)) {
	ui.Row(c).FillWidth().Padding(10, 12, 10, 10).Radius(20).Background(a.colors.block).Gap(12).Children(func() {
		ui.ButtonBase(c).Label(label).Grow(1).MinWidth(0).OnClick(func() { change(!value) }).Children(func() {
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				color, ink := a.colors.card, a.colors.muted
				if value {
					color, ink = a.colors.accent, a.colors.onAccent
				}
				ui.Box(c).Size(38, 38).Radius(999).Background(color).Center().Children(func() { glyph(c, icon).FontSize(19).TextColor(ink) })
				ui.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
					ui.Text(c, label).FontSize(fontSize(c, 15)).FixedLineHeight(a.textHeight(20)).FontWeight(600).SingleLine()
					ui.Text(c, subtitle).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(a.colors.muted).SingleLine()
				})
			})
		})
		ui.Box(c).Size(52, 32).Shrink(0).Children(func() {
			if a.switchControl(c, &value, label).Absolute().Left(-4).Top(-8).Changed() {
				change(value)
			}
		})
	})
}

func (a *App) narrowTakeoverCard(c *ui.Context) {
	a.dashboardCard(c).Label(a.t("networkTakeover")).FillWidth().Gap(0).Children(func() {
		a.dashboardHeader(c, "shuffle", a.t("networkTakeover"), nil)
		ui.Column(c).FillWidth().Margin(12, 0, 0, 0).Gap(8).Children(func() {
			for _, tun := range []bool{false, true} {
				label, icon, value := a.t("systemProxy"), "shuffle", a.cfg.SystemProxy
				if tun {
					label, icon, value = "TUN", "vpn", store.Bool(store.Map(a.cfg.Overrides, "tun")["enable"])
				}
				subtitle := a.t("disabled")
				if value {
					subtitle = a.t("enabled")
					if !tun {
						subtitle = strings.ReplaceAll(a.t("portValue"), "{port}", number(store.Int(a.cfg.Overrides["mixed-port"])))
					}
				}
				a.narrowSwitchRow(c, icon, label, subtitle, value, func(next bool) {
					if tun {
						store.Map(a.cfg.Overrides, "tun")["enable"] = next
					} else {
						a.cfg.SystemProxy = next
					}
					a.apply()
				})
			}
		})
	})
}

func (a *App) narrowIntranetCard(c *ui.Context) {
	a.dashboardCard(c).Label(a.t("intranetIP")).FillWidth().Gap(0).Children(func() {
		a.dashboardHeader(c, "devices", a.t("intranetIP"), func() {
			a.dashboardIconButton(c, "copy", a.t("copy"), func() { c.Services().WriteClipboard(a.intranet) })
		})
		ip := a.intranet
		if ip == "" {
			ip = a.t("noNetwork")
		}
		value := store.Bool(a.cfg.Overrides["allow-lan"])
		if value && a.intranet != "" {
			ip += ":" + number(store.Int(a.cfg.Overrides["mixed-port"]))
		}
		ui.Text(c, ip).FillWidth().Height(34*max(a.cfg.Scale, 1)).Margin(12, 0, 14, 0).FontSize(fontSize(c, 28)).FontWeight(600).SingleLine()
		label := a.t("disabled")
		if value {
			label = a.t("enabled")
		}
		a.narrowSwitchRow(c, "devices", a.t("allowLan"), label, value, func(next bool) { a.cfg.Overrides["allow-lan"] = next; a.apply() })
	})
}
