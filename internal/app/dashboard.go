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
	_, fullHeight := c.Size()
	compact := fullHeight/scale < 824 || (cols == 4 && (w-side)/scale < 1040)
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
	w, h := c.Size()
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
	return a.dashboardCard(c).Label(a.t("status")).Padding(16, 20).Clip().Children(func() {
		accent := a.colors.accent
		ui.Box(c).Absolute().Right(-50).Top(-30).Size(240, 250).Draw(func(p *ui.Painter, r ui.Rect) {
			shape := cookie(r.X+r.W/2, r.Y+r.H/2, r.W/2, 12)
			p.FillPath(&shape, accent.Alpha(0.09))
		})
		ui.Row(c).Gap(8).Children(func() {
			a.header(c, "status", a.t("status"))
			status := a.t("stopped")
			color := a.colors.muted
			if a.status == "running" {
				status = "• " + a.t("running")
				color = a.colors.success
			} else if a.status == "connecting" {
				status = a.t("connecting")
				color = a.colors.warning
			} else if a.status == "error" {
				status = a.t("error")
				color = a.colors.danger
			}
			a.chip(c, status, color)
		})
		ui.Row(c).Grow(1).Gap(16).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Row(c).Gap(12).Children(func() {
					label := a.country
					if label == "" {
						label = "IP"
					}
					ui.ButtonBase(c.Key("refresh-ip")).Label(a.t("refresh")).Size(40, 40).Center().OnClick(func() {
						a.outboundIP()
						if a.qualities[a.ip].Quality == nil {
							a.lookupQuality(a.ip, true)
						}
					}).Draw(func(p *ui.Painter, r ui.Rect) {
						shape := cookie(r.X+r.W/2, r.Y+r.H/2, r.W/2, 4)
						p.FillPath(&shape, a.colors.inset)
					}).Children(func() { ui.Text(c, label).FontSize(fontSize(c, 12)).Bold() })
					ip := a.ip
					if ip == "" {
						ip = "—"
					}
					ui.ButtonBase(c.Key("outbound-ip")).Label(a.shownIP(ip)).OnClick(func() { a.showQuality(a.ip) }).Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							text := ui.Text(c, a.shownIP(ip)).FontSize(fontSize(c, 22)).Bold().SingleLine()
							if a.cfg.HideIP {
								text.Font("JetBrains Mono")
							}
							a.qualityBadges(c, a.ip)
						})
					})
				})
				ui.Row(c).Gap(24).Margin(12, 0, 0, 0).Children(func() {
					ui.Column(c).Gap(3).Children(func() {
						ui.Text(c, a.t("runTime")).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
						ui.Text(c, elapsed(a.started)).FontSize(fontSize(c, 15)).FontWeight(600)
					})
					ui.Column(c).Gap(3).Grow(1).Children(func() {
						ui.Text(c, a.t("organization")).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
						org := a.organization
						if org == "" {
							org = "—"
						}
						ui.Text(c, org).SingleLine().FontWeight(600).FontSize(fontSize(c, 14))
					})
				})
			})
			label := a.t("start")
			icon := "play"
			if a.wantRunning {
				label = a.t("stop")
				icon = "pause"
			}
			button := ui.ButtonBase(c.Key("start-stop")).Label(label).Size(82, 82).Center().TextColor(a.colors.onAccent).Disabled(a.safe).OnClick(a.toggle)
			button.Draw(func(p *ui.Painter, r ui.Rect) {
				shape := cookie(r.X+r.W/2, r.Y+r.H/2, r.W/2, 12)
				p.FillPath(&shape, accent)
			}).Children(func() { glyphVariant(c, icon, true).FontSize(28) })
		})
		if a.status == "running" {
			c.After(time.Second)
		}
	})
}
func cookie(x, y, r float32, count int) ui.Path {
	var path ui.Path
	for i := 0; i <= 180; i++ {
		theta := float64(i) * 2 * math.Pi / 180
		rad := float64(r) * (0.96 + 0.04*math.Cos(float64(count)*theta))
		px, py := x+float32(rad*math.Cos(theta)), y+float32(rad*math.Sin(theta))
		if i == 0 {
			path.MoveTo(px, py)
		} else {
			path.LineTo(px, py)
		}
	}
	path.Close()
	return path
}
func (a *App) trafficCard(c *ui.Context, up bool) ui.Element {
	value, total, peak := a.sample.Traffic.Down, a.sample.Total.Down, a.peak.Down
	label, icon := a.t("download"), "down"
	history := a.downHistory
	color := a.colors.accent
	if up {
		value, total, peak = a.sample.Traffic.Up, a.sample.Total.Up, a.peak.Up
		label, icon = a.t("upload"), "up"
		history = a.upHistory
		color = color.Alpha(0.48)
	}
	return a.dashboardCard(c).Padding(20, 20, 0, 20).Gap(8).Clip().Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			a.header(c, icon, label)
			ui.Spacer(c)
			ui.Text(c, a.t("peak")+" "+bytesText(peak)+"/s").FontSize(fontSize(c, 10)).TextColor(a.colors.muted).SingleLine()
		})
		parts := strings.SplitN(bytesText(value), " ", 2)
		ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
			ui.Text(c, parts[0]).FontSize(fontSize(c, 34)).Bold()
			ui.Text(c, parts[1]+"/s").FontSize(fontSize(c, 14)).FontWeight(600).TextColor(a.colors.muted)
		})
		ui.Text(c, a.t("totalTraffic")+" "+bytesText(total)).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
		ui.Box(c).Grow(1).FillWidth().Margin(0, -20).Draw(func(p *ui.Painter, r ui.Rect) { plotTraffic(p, r, history, color) })
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
	return a.dashboardCard(c).Padding(16).Gap(8).Children(func() {
		ui.Row(c).Children(func() {
			a.header(c, "devices", a.t("intranetIP"))
			ui.Spacer(c)
			a.button(c, "copy", a.t("copy"), func() { c.Services().WriteClipboard(a.intranet) }).Padding(0).Background(ui.Transparent)
		})
		ip := a.intranet
		if ip == "" {
			ip = "—"
		}
		ui.Text(c, ip).FontSize(fontSize(c, 23)).Bold().SingleLine()
		ui.Spacer(c)
		ui.Row(c).Padding(8, 12).Radius(16).Background(a.colors.block).Children(func() {
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, a.t("allowLan")).FontSize(fontSize(c, 14)).FontWeight(600)
				enabled := store.Bool(a.cfg.Overrides["allow-lan"])
				label := a.t("disabled")
				if enabled {
					label = a.t("enabled")
				}
				ui.Text(c, label).FontSize(fontSize(c, 11)).TextColor(a.colors.muted)
			})
			value := store.Bool(a.cfg.Overrides["allow-lan"])
			if a.switchControl(c, &value, a.t("allowLan")).Changed() {
				a.cfg.Overrides["allow-lan"] = value
				a.apply()
			}
		})
	})
}
func (a *App) profileCard(c *ui.Context) ui.Element {
	return a.dashboardCard(c).Padding(16).Gap(8).OnClick(a.openProfileDetails).Focusable().Children(func() {
		ui.Row(c).Children(func() {
			a.header(c, "profiles", a.t("profile"))
			ui.Spacer(c)
			a.button(c, "chevron", a.messageText("details", map[string]any{"label": a.t("profile")}), a.openProfileDetails).Padding(0).Background(ui.Transparent)
		})
		p := a.store.ActiveProfile(a.cfg)
		if p == nil {
			ui.Text(c, a.t("nullProfileDesc")).FontSize(fontSize(c, 14)).TextColor(a.colors.muted)
			ui.Spacer(c)
			ui.Button(c, a.t("addProfile")).Radius(999).OnClick(func() { a.addOpen = true })
			return
		}
		ui.Row(c).Children(func() {
			ui.Text(c, p.Name).Bold().Grow(1).SingleLine()
			if p.Expire > 0 {
				days := int(time.Until(time.Unix(p.Expire, 0)).Hours() / 24)
				ui.Textf(c, "%d d", days).FontSize(fontSize(c, 11)).TextColor(a.colors.muted)
			}
		})
		ui.Spacer(c)
		if p.Total > 0 {
			fraction := float64(p.Upload+p.Download) / float64(p.Total)
			ui.Row(c).Children(func() {
				ui.Textf(c, "%.0f%%", 100*fraction).Bold().Grow(1)
				ui.Text(c, bytesText(p.Upload+p.Download)+" / "+bytesText(p.Total)).FontSize(fontSize(c, 11)).TextColor(a.colors.muted)
			})
			ui.Box(c).Height(5).Radius(999).Background(a.colors.inset).Children(func() {
				ui.Box(c).Height(5).WidthPercent(float32(min(fraction, 1) * 100)).Radius(999).Background(a.colors.accent)
			})
		} else {
			ui.Text(c, p.Updated.Format("2006-01-02 15:04")).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
		}
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
			a.dashboardIconButton(c, "chevron", strings.ReplaceAll(a.t("details"), "{label}", a.t("memoryInfo")), func() { a.memoryOpen = true })
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
