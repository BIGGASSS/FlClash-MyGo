package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func (a *App) dashboard(c *ui.Context) {
	if a.dashboardNarrow(c) {
		a.narrowDashboard(c)
		return
	}
	a.dashboardEditing = false
	w, h := a.viewSize(c)
	side := a.sidebarSize()
	scale := max(a.cfg.Scale, 1)
	gap := a.dashboardGap(c)
	cols := 4
	if (w-side)/scale < 860 {
		cols = 2
	}
	statusHeight, modeHeight, infoHeight := float32(208), float32(64), float32(176)
	bodyHeight := h - 68
	compact := a.dashboardCompact(c)
	if compact {
		statusHeight, modeHeight, infoHeight = 176, 56, 152
	}
	statusHeight, modeHeight, infoHeight = statusHeight*scale, modeHeight*scale, infoHeight*scale
	summaryHeight, tableMin := 120*scale, 26+170*scale
	if compact {
		summaryHeight, tableMin = 112*scale, 28+160*scale
	}
	bottomHeight := bodyHeight - 16 - statusHeight - gap - modeHeight - gap - infoHeight - gap - 24
	tablesFit := cols == 4 && bottomHeight >= tableMin
	bottomHeight = max(bottomHeight, summaryHeight)
	if cols == 2 {
		bottomHeight = summaryHeight
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Grid(c).FillWidth().MaxWidth(float32(cols)*440*scale+float32(cols-1)*gap+40).AlignSelf(ui.Center).Columns(cols).Gap(gap).Padding(16, 24, 24, 16).Children(func() {
			a.statusCard(c).ColumnSpan(2).Height(statusHeight)
			a.trafficCard(c, true).Height(statusHeight)
			a.trafficCard(c, false).Height(statusHeight)
			a.dashboardModeBar(c).ColumnSpan(2).Height(modeHeight)
			a.takeoverTile(c, false, compact).Height(modeHeight)
			a.takeoverTile(c, true, compact).Height(modeHeight)
			a.intranetCard(c).Height(infoHeight)
			a.profileCard(c).Height(infoHeight)
			a.servicesCard(c).Height(infoHeight)
			a.memoryCard(c).Height(infoHeight)
			ui.Row(c).ColumnSpan(cols).Height(bottomHeight).Gap(gap).AlignItems(ui.Stretch).Children(func() {
				if tablesFit {
					a.dashboardCard(c).Label(a.t("liveConnections")).Padding(14).Grow(3).MinWidth(0).Gap(10).Children(func() {
						seg(c, &a.connectionTab, fmt.Sprintf("%s  %s", number(len(a.sample.Connections.Connections)), a.t("liveConnections")), fmt.Sprintf("%s  %s", number(len(a.requests)), a.t("requests")), fmt.Sprintf("%s  DNS", number(len(a.dns))))
						a.dashboardRecords(c)
					})
					a.dashboardCard(c).Grow(2).MinWidth(160).Gap(12).Children(func() { a.header(c, "layers", a.t("proxyGroup")); a.groupSummary(c, 4) })
				} else {
					a.connectionStatsCard(c).Grow(1).MinWidth(0)
					a.proxyGroupsSummaryCard(c).Grow(1).MinWidth(0)
				}
			})
		})
	})
}

func (a *App) dashboardModeBar(c *ui.Context) ui.Element {
	return a.dashboardCard(c).Padding(8, 8, 8, 20).Children(func() {
		ui.Row(c).Fill().Gap(16).Children(func() {
			a.header(c, "split", a.t("outboundMode"))
			ui.Row(c).Grow(1).MinWidth(0).FillHeight().Gap(2).Children(func() {
				for i, mode := range []string{"rule", "global", "direct"} {
					selected := store.String(a.cfg.Overrides["mode"]) == mode
					button := ui.ButtonBase(c.Key(mode)).Label(a.t(mode)).Grow(1).MinWidth(0).FillHeight().OnClick(func() { a.performHotkey(mode + "Mode") })
					a.connectedModeStyle(button, i, selected, 24, 10)
					button.Children(func() {
						ui.Row(c).Fill().Center().Gap(6).Children(func() {
							glyph(c, []string{"rules", "proxies", "target"}[i]).FontSize(18)
							ui.Text(c, a.t(mode)).FontSize(fontSize(c, 15)).FontWeight(600).SingleLine()
						})
					})
				}
			})
		})
	})
}

func (a *App) takeoverTile(c *ui.Context, tun, compact bool) ui.Element {
	label, value := a.t("systemProxy"), a.cfg.SystemProxy
	if tun {
		label, value = "TUN", store.Bool(store.Map(a.cfg.Overrides, "tun")["enable"])
	}
	change := func() {
		if tun {
			store.Map(a.cfg.Overrides, "tun")["enable"] = !value
		} else {
			a.cfg.SystemProxy = !value
		}
		a.apply()
	}
	return a.dashboardCard(c).Padding(0, 10, 0, 20).Children(func() {
		ui.Row(c).Fill().Gap(4).Children(func() {
			ui.ButtonBase(c).Label(label).Grow(1).MinWidth(0).FillHeight().OnClick(change).Children(func() {
				ui.Column(c).Fill().Justify(ui.Center).Gap(2).Children(func() {
					ui.Text(c, label).FontWeight(600).FontSize(fontSize(c, 14)).SingleLine()
					if !compact {
						status := a.t("disabled")
						if value {
							status = a.t("enabled")
							if !tun {
								status = strings.ReplaceAll(a.t("portValue"), "{port}", number(store.Int(a.cfg.Overrides["mixed-port"])))
							}
						}
						ui.Text(c, status).FontSize(fontSize(c, 12)).TextColor(a.colors.muted).SingleLine()
					}
				})
			})
			if a.switchControl(c, &value, label).Changed() {
				// The switch has already changed its local value.
				if tun {
					store.Map(a.cfg.Overrides, "tun")["enable"] = value
				} else {
					a.cfg.SystemProxy = value
				}
				a.apply()
			}
		})
	})
}

func (a *App) connectionStatsCard(c *ui.Context) ui.Element {
	return a.dashboardCard(c).Label(a.t("connectionStats")).Padding(8).Children(func() {
		ui.Row(c).Fill().Children(func() {
			for i, page := range []string{"connections", "requests", "dns"} {
				label := a.t([]string{"liveConnections", "requests", "dnsQueries"}[i])
				count := []int{len(a.sample.Connections.Connections), len(a.requests), len(a.dns)}[i]
				ui.ButtonBase(c.Key(page)).Label(label).Grow(1).MinWidth(0).FillHeight().Radius(24).OnClick(func() { a.goPage(page) }).Children(func() {
					ui.Column(c).Fill().Center().Gap(2).Children(func() {
						ui.Text(c, number(count)).FontSize(fontSize(c, 22)).FixedLineHeight(a.textHeight(28)).FontWeight(600)
						ui.Text(c, label).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(a.colors.muted).SingleLine()
					})
				})
			}
		})
	})
}

func (a *App) proxyGroupsSummaryCard(c *ui.Context) ui.Element {
	return a.dashboardCard(c).Padding(16, 20).Gap(4).Children(func() {
		ui.Row(c).Height(24).Children(func() {
			a.header(c, "layers", a.t("proxyGroup"))
			ui.Spacer(c)
			ui.ButtonBase(c).Label(a.t("proxyGroup")).Size(32, 32).Center().OnClick(func() { a.openProxyPicker("") }).Children(func() { glyph(c, "chevron").FontSize(18).TextColor(a.colors.muted) })
		})
		ui.Row(c).Grow(1).FillWidth().AlignItems(ui.Center).Children(func() {
			groups := a.currentProxyGroups()
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Text(c, number(len(groups))).FontSize(fontSize(c, 34)).FontWeight(600)
				ui.Text(c, a.t("groupCountUnit")).FontSize(fontSize(c, 13)).FontWeight(600).TextColor(a.colors.muted)
			})
			ui.Spacer(c)
			count := min(len(groups), 4)
			ui.Box(c).Width(float32(count)*34 - float32(max(count-1, 0))*8).Height(34).Children(func() {
				for i, group := range groups[:count] {
					ui.Box(c).Absolute().Left(float32(i)*26).Size(34, 34).Radius(14).BorderWidth(2).BorderColor(a.colors.card).Background(a.colors.inset).Center().Children(func() { a.targetIcon(c, group.Icon, group.Name, 22, a.colors.text) })
				}
			})
		})
	})
}
func (a *App) dashboardCompact(c *ui.Context) bool {
	if a.dashboardNarrow(c) {
		return false
	}
	w, h := a.viewSize(c)
	side := a.sidebarSize()
	scale := max(a.cfg.Scale, 1)
	width := (w - side) / scale
	return h/scale < 824 || width >= 860 && width < 1040
}

func (a *App) dashboardHeader(c *ui.Context, icon, title string, actions func()) {
	height, size, leading, color := float32(24), float32(13), float32(18), a.colors.muted
	if a.dashboardNarrow(c) {
		height, size, leading, color = 26, 15, 20, a.colors.text
	}
	ui.Row(c).FillWidth().Height(height * max(a.cfg.Scale, 1)).Gap(8).Children(func() {
		glyph(c, icon).FontSize(18).TextColor(a.colors.muted)
		ui.Text(c, title).Grow(1).MinWidth(0).FontSize(fontSize(c, size)).FixedLineHeight(a.textHeight(leading)).FontWeight(600).TextColor(color).SingleLine()
		if actions != nil {
			actions()
		}
	})
}

func (a *App) dashboardCard(c *ui.Context) ui.Element {
	card := a.card(c).Radius(32)
	if a.dashboardNarrow(c) {
		card.Padding(18, 20).Radius(34)
	} else if a.dashboardCompact(c) {
		card.Padding(16, 20)
	} else {
		card.Padding(20, 22)
	}
	if a.colors.dark {
		return card.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 128.0/255)).Shadow(0, 10, 28, -6, ui.RGBA(0, 0, 0, 140.0/255))
	}
	return card.Shadow(0, 0.5, 1, 0, ui.RGBA(0, 0, 0, 13.0/255)).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 10.0/255)).Shadow(0, 12, 32, -6, ui.RGBA(0, 0, 0, 23.0/255))
}
func (a *App) statusCard(c *ui.Context) ui.Element {
	// current_outbound.dart: _OutboundMetrics. The button slot belongs to the
	// full-height row, not to the IP row; the cells sit at the card's foot.
	compact := a.dashboardCompact(c)
	scale := max(a.cfg.Scale, 1)
	ipTop, cellGap, badge, ipSize, ipLeading, padding := float32(10), float32(28), float32(48), float32(28), float32(34), float32(22)
	button, slot, decoration, decorationTop := float32(104), float32(120), float32(290), float32(-87)
	if compact {
		ipTop, cellGap, badge, ipSize, ipLeading, padding = 8, 22, 40, 22, 28, 20
		button, slot, decoration, decorationTop = 84, 100, 250, -75
	}
	ipWidth := 2*a.dashboardColumnWidth(c) + a.dashboardGap(c) - 2*padding - 16 - slot - badge - 12
	card := a.dashboardCard(c).Label(a.t("status")).Gap(0).Clip()
	target := float32(0)
	if a.wantRunning {
		target = 1
	}
	progress := card.AnimateWith("decoration", target, 450*time.Millisecond, ui.CubicBezier(0.175, 0.885, 0.32, 1.275))
	card.Draw(func(p *ui.Painter, r ui.Rect) {
		size := decoration * (1 + 0.12*progress)
		box := ui.Rect{X: r.X + r.W + 64 - decoration - (size-decoration)/2, Y: r.Y + decorationTop - (size-decoration)/2, W: size, H: size}
		color := a.colors.block.Mix(a.colors.card.Mix(a.colors.accent, 0.1), min(max(progress, 0), 1))
		p.Clip(r, 32, func() { p.FillPath(dashboardShape("cookie12Sided", box, 0, progress*30), color) })
	}).Children(func() {
		ui.Row(c).Fill().Gap(16).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).FillHeight().Gap(0).Children(func() {
				ui.Row(c).FillWidth().Height(24 * scale).Gap(8).Children(func() {
					glyph(c, "status").FontSize(18).TextColor(a.colors.muted)
					ui.Text(c, a.t("status")).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).FontWeight(600).TextColor(a.colors.muted).SingleLine()
					status, color := a.t("stopped"), a.colors.muted
					if a.wantRunning && a.onDemandSuspended() {
						status, color = a.t("suspended"), a.colors.warning
					} else if a.status == "running" {
						status, color = a.t("running"), a.colors.success
					} else if a.status == "connecting" {
						status, color = a.t("connecting"), a.colors.warning
					} else if a.status == "error" {
						status, color = a.t("error"), a.colors.danger
					}
					a.dashboardChip(c, status, color, false, a.wantRunning)
				})
				ui.Row(c).FillWidth().Margin(ipTop, 0, 0, 0).Gap(12).Children(func() {
					label := a.country
					if label == "" {
						label = "IP"
					}
					ui.ButtonBase(c.Key("refresh-ip")).Label(a.t("refresh")).Size(badge, badge).Shrink(0).Center().OnClick(func() {
						a.outboundIP()
						if a.qualities[a.ip].Quality == nil {
							a.lookupQuality(a.ip, true)
						}
					}).Draw(func(p *ui.Painter, r ui.Rect) {
						p.FillPath(dashboardShape("clover4Leaf", r, 0, 0), a.colors.inset)
					}).Children(func() { ui.Text(c, label).FontSize(fontSize(c, 13)).FontWeight(600) })
					ip := a.ip
					if ip == "" {
						ip = "—"
					}
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
						ui.ButtonBase(c.Key("outbound-ip")).Label(a.shownIP(ip)).FillWidth().Height(ipLeading * scale).OnClick(func() { a.showQuality(a.ip) }).Children(func() {
							ui.Column(c).Fill().AlignItems(ui.Start).Justify(ui.Center).Children(func() {
								family := ""
								if a.cfg.HideIP {
									family = "JetBrains Mono"
								}
								a.dashboardFitText(c, a.shownIP(ip), ipSize, ipLeading, ipWidth, family)
							})
						})
						ui.Row(c).Height(24).FillWidth().Clip().Wrap().GapX(6).GapY(24).Children(func() {
							if quality := a.qualities[a.ip].Quality; quality != nil {
								a.dashboardChip(c, a.qualityType(quality.Type), a.colors.muted, false, false)
								color := a.colors.muted
								if quality.Level() == "good" {
									color = a.colors.success
								} else if quality.Level() == "risky" {
									color = a.colors.warning
								}
								a.dashboardChip(c, a.t("ipQualityShort")+" "+a.qualityLabel(quality.Level()), color, false, false)
							} else if a.qualityLoading(a.ip) {
								ui.Text(c, a.t("loading")).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
							}
						})
					})
				})
				ui.Spacer(c)
				ui.Row(c).FillWidth().Gap(cellGap).Children(func() {
					for i, key := range []string{"runTime", "organization"} {
						column := ui.Column(c).Gap(2)
						if i > 0 {
							column.Grow(1).Basis(0).MinWidth(0)
						}
						column.Children(func() {
							ui.Text(c, a.t(key)).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(a.colors.muted).SingleLine()
							value := a.organization
							if i == 0 {
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
			ui.Box(c).Width(slot).Shrink(0).FillHeight().Center().Children(func() { a.dashboardStartControl(c, button) })
		})
		if a.status == "running" {
			c.After(time.Second)
		}
	})
	return card
}
func (a *App) trafficCard(c *ui.Context, up bool) ui.Element {
	value, total, peak := a.sample.Traffic.Down, a.sample.Total.Down, a.peak.Down
	label, icon := a.t("download"), "arrowDown"
	history, color, ink := a.downHistory, a.colors.accent, a.colors.accent
	if up {
		value, total, peak = a.sample.Traffic.Up, a.sample.Total.Up, a.peak.Up
		label, icon = a.t("upload"), "arrowUp"
		history, color, ink = a.upHistory, a.colors.card.Mix(a.colors.accent, 0.45), a.colors.muted
	}
	// traffic.dart: SpeedCard. Chart height and bleed do not scale with text.
	compact := a.dashboardCompact(c)
	top, figure, leading, chart := float32(10), float32(40), float32(44), float32(88)
	if compact {
		top, figure, leading, chart = 6, 34, 41, 62
	}
	return a.dashboardCard(c).Label(label).Gap(0).Clip().Children(func() {
		ui.Box(c).Absolute().Left(-3).Right(-3).Bottom(-3).Height(chart).Draw(func(p *ui.Painter, r ui.Rect) { plotTraffic(p, r, history, color) })
		ui.Row(c).FillWidth().Height(24 * max(a.cfg.Scale, 1)).Gap(8).Children(func() {
			glyph(c, icon).FontSize(18).TextColor(ink)
			ui.Text(c, label).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).FontWeight(600).TextColor(a.colors.muted).SingleLine()
			ui.Text(c, a.t("peak")+" "+bytesText(peak)+"/s").Grow(1).Basis(0).MinWidth(0).TextAlign(ui.End).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(a.colors.muted).SingleLine()
		})
		parts := strings.SplitN(bytesText(value), " ", 2)
		spans := []ui.Span{
			{Text: parts[0], Size: fontSize(c, figure), Weight: 700, LetterSpacing: -0.02 * figure},
			{Text: " " + parts[1] + "/s", Size: fontSize(c, 15), Weight: 600, Color: a.colors.muted},
		}
		// Flutter allows negative font leading (display is 40/44). MyGo
		// clamps rich text to its font metrics, so center the measured line
		// in the source's allocated line box instead of growing the column.
		_, naturalHeight := c.MeasureText(0, spans...)
		lineHeight := a.textHeight(leading)
		ui.Box(c).Label(bytesText(value)+"/s").FillWidth().Height(lineHeight).Margin(top, 0, 0, 0).Children(func() {
			ui.RichText(c, spans...).Absolute().Left(0).Right(0).Top((lineHeight - naturalHeight) / 2).Height(naturalHeight).SingleLine()
		})
		ui.Text(c, a.t("totalTraffic")+" "+bytesText(total)).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(a.colors.muted).SingleLine().Margin(2, 0, 0, 0)
	})
}

// The original LineChart uses a 35-sample Gaussian window and clamped
// Catmull-Rom slopes, on a fixed thirty-sample viewport.
func plotTraffic(p *ui.Painter, r ui.Rect, values []int64, color ui.Color) {
	const count = 35
	samples := make([]float64, count)
	n := min(len(values), count)
	for i, value := range values[len(values)-n:] {
		samples[count-n+i] = float64(value)
	}
	heights := make([]float64, count)
	for i := range heights {
		weight := 0.0
		for j := max(i-4, 0); j <= min(i+4, count-1); j++ {
			distance := float64(j - i)
			w := math.Exp(-distance * distance / (2 * 1.5 * 1.5))
			heights[i] += w * samples[j]
			weight += w
		}
		heights[i] /= weight
	}
	slopes := make([]float64, count)
	for i := 1; i < count-1; i++ {
		slopes[i] = min(max((heights[i+1]-heights[i-1])/2, -3*heights[i]), 3*heights[i])
	}
	top := float64(8192)
	for i, h := range heights {
		top = max(top, h, h+slopes[i]/3, h-slopes[i]/3)
	}
	step := r.W / 29
	first := r.X + r.W - float32(count-1)*step
	scale := float64(max(r.H-2, 0)) / top
	yOf := func(h float64) float32 { return r.Y + r.H - float32(h*scale) }
	var line, area ui.Path
	line.MoveTo(first, yOf(heights[0]))
	area.MoveTo(first, yOf(heights[0]))
	x := first
	for i := 1; i < count; i++ {
		nx := x + step
		c1, c2, ny := yOf(heights[i-1]+slopes[i-1]/3), yOf(heights[i]-slopes[i]/3), yOf(heights[i])
		line.CubeTo(x+step/3, c1, nx-step/3, c2, nx, ny)
		area.CubeTo(x+step/3, c1, nx-step/3, c2, nx, ny)
		x = nx
	}
	area.LineTo(x, r.Y+r.H).LineTo(first, r.Y+r.H).Close()
	p.Clip(r, 0, func() { p.FillPath(&area, color.Alpha(0.14)); p.StrokePath(&line, 2, color) })
}
func (a *App) intranetCard(c *ui.Context) ui.Element {
	return a.dashboardCard(c).Label(a.t("intranetIP")).Gap(0).Children(func() {
		a.dashboardHeader(c, "devices", a.t("intranetIP"), func() {
			address := a.intranet
			if store.Bool(a.cfg.Overrides["allow-lan"]) && address != "" {
				address += ":" + number(store.Int(a.cfg.Overrides["mixed-port"]))
			}
			a.dashboardIconButton(c, "copy", a.t("copy"), func() { c.Services().WriteClipboard(address) }).Translate(8, 0).Disabled(address == "")
		})
		top := float32(10)
		if a.dashboardCompact(c) {
			top = 8
		}
		ip := a.intranet
		if ip == "" {
			ip = a.t("noNetwork")
		}
		// IntranetIpCard uses title2 in both desktop densities.
		ui.Box(c).FillWidth().Height(28*max(a.cfg.Scale, 1)).Margin(top, 0, 0, 0).Justify(ui.Center).Children(func() {
			a.dashboardFitText(c, ip, 22, 28, a.dashboardInfoWidth(c), "")
		})
		ui.Spacer(c)
		a.dashboardLanRow(c)
	})
}
func (a *App) profileCard(c *ui.Context) ui.Element {
	return a.dashboardCard(c).Label(a.t("profile")).Gap(0).OnClick(a.openProfileDetails).Focusable().Children(func() {
		a.desktopProfileContent(c)
	})
}
func (a *App) servicesCard(c *ui.Context) ui.Element {
	return a.dashboardCard(c).Padding(16, 20).Gap(8).Children(func() {
		shown := a.dashboardServiceTargets(c)
		columns := 1
		if len(shown) > 2 {
			columns = 2
		}
		ui.Row(c).Height(24).Children(func() {
			a.header(c, "shield", a.t("serviceStatus"))
			ui.Spacer(c)
			if columns == 2 {
				ui.ButtonBase(c.Key("refresh-services")).Label(a.t("serviceCheckAll")).Size(32, 32).Center().OnClick(func() { a.refreshServices(shown) }).Children(func() { glyph(c, "bolt").FontSize(18).TextColor(a.colors.muted) })
			}
			ui.ButtonBase(c.Key("open-services")).Label(a.t("serviceStatus")).Size(32, 32).Center().OnClick(a.openServices).Children(func() { glyph(c, "chevron").FontSize(18).TextColor(a.colors.muted) })
		})
		ui.Spacer(c)
		ui.Grid(c).FillWidth().Columns(columns).Gap(6).Children(func() {
			for _, target := range shown {
				ui.ButtonBase(c.Key(target.ID)).Label(target.Label).Tooltip(target.Label).Height(40).Radius(16).Background(a.colors.block).Padding(0, 10).Disabled(a.servicePending[target.ID] != 0).OnClick(func() { a.refreshServices([]serviceTarget{target}) }).Children(func() {
					ui.Row(c).Fill().Gap(8).Children(func() {
						serviceIcon(c, target.Icon, 20)
						label, color := a.serviceStatus(target.ID, true)
						ui.Text(c, label).FontSize(fontSize(c, 12)).FontWeight(600).TextColor(color).Grow(1).SingleLine()
					})
				})
			}
		})
	})
}
func (a *App) memoryCard(c *ui.Context) ui.Element {
	return a.dashboardCard(c).Label(a.t("memoryInfo")).Gap(0).Children(func() {
		a.dashboardHeader(c, "memory", a.t("memoryInfo"), func() {
			button := a.dashboardIconButton(c, "chevron", strings.ReplaceAll(a.t("details"), "{label}", a.t("memoryInfo")), func() { a.memoryOpen = true })
			if !a.dashboardNarrow(c) {
				button.Translate(8, 0)
			}
		})
		gap := float32(6)
		if a.dashboardNarrow(c) {
			gap = 12
		} else if a.dashboardCompact(c) {
			gap = 4
		}
		ui.Box(c).Height(gap)
		app, core := a.memoryTotals()
		if a.dashboardNarrow(c) {
			a.memoryFigure(c, app+core, 28, 13)
			ui.Box(c).Height(14)
		} else {
			a.memoryFigure(c, app+core, 22, 13)
			ui.Spacer(c)
		}
		a.memoryBar(c, app, core)
		gap = 10
		if a.dashboardCompact(c) {
			gap = 8
		}
		ui.Box(c).Height(gap)
		ui.Row(c).FillWidth().Wrap().GapX(16).GapY(4).Children(func() {
			for i, part := range []struct {
				label string
				bytes uint64
			}{{a.t("app"), app}, {a.t("core"), core}} {
				color := a.colors.accent
				if i == 1 {
					color = a.colors.card.Mix(a.colors.accent, 0.45)
				}
				ui.Row(c).Gap(6).Children(func() {
					ui.Box(c).Size(8, 8).Radius(999).Background(color)
					ui.Text(c, part.label).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(18)).TextColor(a.colors.muted)
					ui.Text(c, bytesText(int64(part.bytes))).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).FontWeight(600)
				})
			}
		})
	})
}

func (a *App) dashboardRecords(c *ui.Context) {
	if a.connectionTab == 2 {
		if len(a.dns) == 0 {
			ui.Text(c, a.t("noData")).TextColor(a.colors.muted).Padding(20)
			return
		}
		n := len(a.dns)
		ui.Table(c, nil, []ui.TableColumn{{Title: "Domain"}, {Title: "Type", Width: 60}, {Title: "Answer"}}, min(n, 8), func(row, col int) {
			d := a.dns[n-1-row]
			text := d.Domain
			if col == 1 {
				text = d.Type
			} else if col == 2 {
				text = strings.Join(d.Answers, ", ")
			}
			ui.Text(c, text).SingleLine().FontSize(fontSize(c, 12))
		}).Grow(1)
		return
	}
	connections := a.sample.Connections.Connections
	if a.connectionTab == 1 {
		connections = a.requests
	}
	if len(connections) == 0 {
		ui.Column(c).Grow(1).Center().Children(func() { ui.Text(c, a.t("noData")).TextColor(a.colors.muted) })
		return
	}
	ui.Table(c, nil, []ui.TableColumn{{Title: a.t("source"), Width: 125}, {Title: a.t("target")}, {Title: a.t("proxy"), Width: 100}, {Title: a.t("download"), Width: 75}, {Title: a.t("upload"), Width: 65}}, min(len(connections), 10), func(row, col int) {
		x := connections[row]
		text := []string{x.Source(), x.Target(), x.Exit(), bytesText(x.Download), bytesText(x.Upload)}[col]
		ui.Text(c, text).SingleLine().FontSize(fontSize(c, 12))
	}).Grow(1)
}
func (a *App) groupSummary(c *ui.Context, limit int) {
	n := 0
	for _, name := range a.sample.Proxies.All {
		g := a.sample.Proxies.Proxies[name]
		if g.Hidden {
			continue
		}
		n++
		if n > limit {
			break
		}
		ui.ButtonBase(c.Key(name)).FillWidth().Radius(18).Background(a.colors.block).Padding(10, 12).OnClick(func() { a.openProxyPicker(name) }).Children(func() {
			ui.Row(c).FillWidth().Gap(10).Children(func() {
				ui.Box(c).Size(32, 32).Radius(12).Background(a.colors.inset).Center().Children(func() { a.targetIcon(c, g.Icon, name, 24, a.colors.text) })
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, name).Bold().FontSize(fontSize(c, 14)).SingleLine()
					ui.Text(c, g.Now).TextColor(a.colors.muted).FontSize(fontSize(c, 12)).SingleLine()
				})
			})
		})
	}
	if n == 0 {
		ui.Text(c, a.t("noData")).TextColor(a.colors.muted)
	}
}
func initial(s string) string {
	for _, r := range s {
		return strings.ToUpper(string(r))
	}
	return "?"
}

var servicesIcons = map[string]*ui.SVG{}

func serviceIcon(c *ui.Context, name string, size float32) {
	svg := servicesIcons[name]
	if svg == nil {
		b := assets.Read("services/" + name + ".svg")
		if len(b) > 0 {
			svg, _ = ui.ParseSVG(b)
			servicesIcons[name] = svg
		}
	}
	if svg != nil {
		ui.Icon(c, svg).Size(size, size).TextColor(c.Theme().TextMuted)
	} else {
		ui.Text(c, initial(name)).FontSize(size).Bold()
	}
}
