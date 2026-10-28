package app

import (
	"fmt"
	"math"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func (a *App) narrowGroupsCard(c *ui.Context) {
	groups := a.currentProxyGroups()
	a.dashboardCard(c).Label(a.t("proxyGroup")).FillWidth().Gap(0).OnClick(func() { a.openProxyPicker("") }).Focusable().Children(func() {
		a.dashboardHeader(c, "layers", a.t("proxyGroup"), func() {
			if len(groups) > 0 {
				glyph(c, "chevron").FontSize(18).TextColor(a.colors.muted)
			}
		})
		ui.Row(c).FillWidth().Height(max(40, a.textHeight(44))).Margin(12, 0, 0, 0).Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Text(c, number(len(groups))).FontSize(fontSize(c, 40)).FixedLineHeight(a.textHeight(44)).FontWeight(600)
				ui.Text(c, a.t("groupCountUnit")).FontSize(fontSize(c, 13)).FontWeight(600).TextColor(a.colors.muted)
			})
			n := min(4, len(groups))
			ui.Box(c).Width(float32(n)*40 - float32(max(n-1, 0))*8).Height(40).Children(func() {
				for i, group := range groups[:n] {
					ui.Box(c).Absolute().Left(float32(i)*32).Size(40, 40).Radius(14).BorderWidth(2).BorderColor(a.colors.card).Background(a.colors.block).Center().Children(func() { a.targetIcon(c, group.Icon, group.Name, 24, a.colors.text) })
				}
			})
		})
	})
}

func (a *App) dashboardChip(c *ui.Context, label string, color ui.Color, large, dot bool) {
	bg, fg := a.colors.block, a.colors.muted
	if color == a.colors.success {
		bg, fg = a.colors.successContainer, a.colors.onSuccessContainer
	} else if color == a.colors.warning {
		bg, fg = a.colors.warningContainer, a.colors.onWarningContainer
	} else if color == a.colors.danger {
		bg, fg = a.colors.dangerContainer, a.colors.onDanger
	}
	height, padding := float32(24), float32(10)
	if large {
		height, padding = 28, 12
	}
	ui.Row(c).Height(height).Padding(0, padding).Gap(6).Radius(999).Background(bg).Children(func() {
		if dot {
			ui.Box(c).Size(6, 6).Radius(999).Background(fg)
		}
		ui.Text(c, label).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).FontWeight(600).TextColor(fg).SingleLine()
	})
}

func (a *App) narrowServicesCard(c *ui.Context) {
	shown := a.dashboardServiceTargets(c)
	a.dashboardCard(c).Label(a.t("serviceStatus")).FillWidth().Gap(0).OnClick(a.openServices).Focusable().Children(func() {
		a.dashboardHeader(c, "verified", a.t("serviceStatus"), func() {
			all, issues := len(shown) > 0, 0
			for _, target := range shown {
				value := a.serviceValue(target.ID)
				if value == nil {
					all = false
				} else if value.Status != "available" {
					issues++
					all = false
				}
			}
			if issues > 0 {
				a.chip(c, a.messageText("serviceAbnormalCount", map[string]any{"count": issues}), a.colors.warning)
			} else if all {
				a.chip(c, a.t("serviceAllAvailable"), a.colors.success)
			}
			glyph(c, "chevron").FontSize(18).TextColor(a.colors.muted)
		})
		ui.Grid(c).FillWidth().Columns(2).Gap(8).Margin(12, 0, 0, 0).Children(func() {
			for _, target := range shown {
				label, color := a.serviceStatus(target.ID, true)
				ui.Row(c).Label(target.Label).Tooltip(target.Label+"\n"+label).Height(44*max(a.cfg.Scale, 1)).Padding(0, 12, 0, 8).Radius(16).Background(a.colors.block).Gap(8).Children(func() {
					ui.Box(c).Size(28, 28).Center().Children(func() {
						serviceIcon(c, target.Icon, 20)
						if a.serviceValue(target.ID) != nil {
							_, dot := a.serviceStatus(target.ID, false)
							ui.Box(c).Absolute().Right(-1).Bottom(-1).Size(8, 8).Radius(999).BorderWidth(1.5).BorderColor(a.colors.block).Background(dot)
						}
					})
					ui.Text(c, label).Grow(1).MinWidth(0).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(color).MaxLines(2)
				})
			}
		})
	})
}

func (a *App) narrowProfileCard(c *ui.Context) {
	profile := a.store.ActiveProfile(a.cfg)
	a.dashboardCard(c).Label(a.t("profile")).FillWidth().Gap(0).OnClick(a.openProfileDetails).Focusable().Children(func() {
		a.dashboardHeader(c, "profiles", a.t("profile"), func() {
			if profile != nil && profile.URL != "" {
				a.dashboardIconButton(c, "sync", a.t("sync"), func() { a.updateProfile(*profile) })
			}
			glyph(c, "chevron").FontSize(18).TextColor(a.colors.muted)
		})
		name := a.t("addProfile")
		if profile != nil {
			name = profile.Name
		}
		ui.Row(c).FillWidth().Height(a.textHeight(22)).Margin(12, 0, 0, 0).Gap(4).Children(func() {
			ui.Text(c, name).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).FontWeight(600).SingleLine()
			if len(a.cfg.Profiles) > 1 {
				a.dashboardIconButton(c, "swap", a.t("switchProfile"), a.openProfilePicker)
			}
		})
		if profile == nil {
			return
		}
		ui.Row(c).FillWidth().Height(a.textHeight(16)).Children(func() {
			updated := ""
			if !profile.Updated.IsZero() {
				updated = a.messageText("profileUpdatedAt", map[string]any{"time": a.updatedAgo(profile.Updated)})
			}
			ui.Text(c, updated).Grow(1).MinWidth(0).FontSize(fontSize(c, 12)).TextColor(a.colors.muted).SingleLine()
			label := a.t("local")
			if profile.URL != "" {
				label = a.t("remote")
			}
			if profile.Type == "custom" {
				label = a.t("custom")
			}
			if profile.Expire > 0 {
				days := max(0, int(time.Until(time.Unix(profile.Expire, 0)).Hours()/24))
				label = a.messageText("profileDaysLeft", map[string]any{"days": days})
			}
			ui.Text(c, label).FontSize(fontSize(c, 12)).TextColor(a.colors.muted).SingleLine()
		})
		ui.Box(c).Height(14)
		if profile.Total > 0 {
			share := min(max(float64(profile.Upload+profile.Download)/float64(profile.Total), 0), 1)
			ui.Row(c).FillWidth().AlignItems(ui.Baseline).Gap(8).Children(func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
					ui.Textf(c, "%.0f%%", share*100).FontSize(fontSize(c, 20)).FixedLineHeight(a.textHeight(25)).FontWeight(600)
					ui.Text(c, a.t("usedShort")).FontSize(fontSize(c, 13)).TextColor(a.colors.muted)
				})
				ui.Text(c, bytesText(profile.Upload+profile.Download)+" / "+bytesText(profile.Total)).Grow(1).MinWidth(0).TextAlign(ui.End).FontSize(fontSize(c, 13)).TextColor(a.colors.muted).SingleLine()
			})
			ui.Box(c).FillWidth().Height(12).Margin(8, 0, 0, 0).Draw(func(p *ui.Painter, r ui.Rect) {
				active := a.colors.accent
				if share >= 0.9 {
					active = a.colors.warning
				}
				mid, start, end := r.Y+r.H/2, r.X+2, r.X+r.W-2
				split := start + (end-start)*float32(share)
				if share > 0 {
					path := new(ui.Path).MoveTo(start, mid)
					for x := start; x <= split; x++ {
						path.LineTo(x, mid+3*float32(math.Sin(float64((x-start)/12)*2*math.Pi)))
					}
					p.StrokePath(path, 4, active)
				}
				trackStart := start
				if share > 0 {
					trackStart = split + 10
				}
				if trackStart < end-4 {
					p.Line(trackStart, mid, end-10, mid, 4, a.colors.inset)
				}
				if share < 1 {
					path := new(ui.Path).Circle(end-2, mid, 2)
					p.FillPath(path, active)
				}
			})
		} else {
			counts := []string{"—", "—", "—"}
			if value := a.sample.Counts; value != nil {
				proxies := value.Proxies
				for _, provider := range a.sample.Providers {
					if provider.Type == "Proxy" {
						proxies += provider.Count
					}
				}
				counts = []string{number(value.Groups), number(proxies), number(value.Rules)}
			}
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).Gap(16).Children(func() {
				for i, key := range []string{"proxyGroup", "proxies", "rules"} {
					ui.Column(c).Gap(0).Children(func() {
						ui.Text(c, counts[i]).FontSize(fontSize(c, 20)).FixedLineHeight(a.textHeight(25)).FontWeight(600)
						ui.Text(c, a.t(key)).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(a.colors.muted)
					})
				}
			})
		}
	})
}

// dashboardColumnWidth is the original desktop column formula (dashboard.dart).
// It is independent of a prior frame, including during sidebar animation.
func (a *App) dashboardColumnWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	scale := max(a.cfg.Scale, 1)
	columns := float32(4)
	if (w-a.sidebarSize())/scale < 860 {
		columns = 2
	}
	gap := a.dashboardGap(c)
	return max(0, min((w-a.sidebarSize()-40-gap*(columns-1))/columns, 440*scale))
}

func (a *App) dashboardInfoWidth(c *ui.Context) float32 {
	padding := float32(22)
	if a.dashboardCompact(c) {
		padding = 20
	}
	return max(0, a.dashboardColumnWidth(c)-2*padding)
}

// DashboardFitText scales down only when the whole string fits at >= 75%;
// otherwise the unscaled single line is ellipsized, as in card.dart.
func (a *App) dashboardFitText(c *ui.Context, text string, size, leading, width float32, family string) {
	measured, _ := c.MeasureText(0, ui.Span{Text: text, Font: family, Size: fontSize(c, size), Weight: 700})
	factor := float32(1)
	if measured > width && measured*0.75 <= width {
		factor = width / measured
	}
	element := ui.Text(c, text).FontSize(fontSize(c, size) * factor).FixedLineHeight(a.textHeight(leading) * factor).Bold().SingleLine()
	if family != "" {
		element.Font(family)
	}
}

func (a *App) dashboardLanRow(c *ui.Context) {
	// intranet_ip.dart: _LanRowMetrics and DashboardSwitchRow.iconFits.
	compact := a.dashboardCompact(c)
	left, right, vertical, titleStart, icon, radius := float32(10), float32(12), float32(10), float32(14), float32(38), float32(20)
	if compact {
		left, right, vertical, titleStart, icon, radius = 8, 8, 6, 12, 32, 16
	}
	title := a.t("allowLan")
	textWidth, _ := c.MeasureText(0, ui.Span{Text: title, Size: fontSize(c, 15), Weight: 600})
	showIcon := textWidth <= a.dashboardInfoWidth(c)-left-right-icon-24-52
	if !showIcon {
		left = titleStart
	}
	value := store.Bool(a.cfg.Overrides["allow-lan"])
	subtitle := a.t("disabled")
	if value {
		subtitle = a.messageText("portValue", map[string]any{"port": store.Int(a.cfg.Overrides["mixed-port"])})
	}
	height := max(float32(32), a.textHeight(20)+a.textHeight(16))
	if showIcon {
		height = max(height, icon)
	}
	ui.Row(c).Label(title).FillWidth().Height(height+2*vertical).Padding(vertical, right, vertical, left).Radius(radius).Background(a.colors.block).Gap(12).Children(func() {
		ui.ButtonBase(c).Label(title).Grow(1).Basis(0).MinWidth(0).FillHeight().OnClick(func() {
			a.cfg.Overrides["allow-lan"] = !value
			a.apply()
		}).Children(func() {
			ui.Row(c).Fill().Gap(12).Children(func() {
				if showIcon {
					fill, ink := a.colors.card, a.colors.muted
					if value {
						fill, ink = a.colors.accent, a.colors.onAccent
					}
					ui.Box(c).Size(icon, icon).Shrink(0).Radius(999).Background(fill).Center().Children(func() { glyph(c, "wifi").FontSize(icon / 2).TextColor(ink) })
				}
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(0).Children(func() {
					ui.Text(c, title).FontSize(fontSize(c, 15)).FixedLineHeight(a.textHeight(20)).FontWeight(600).SingleLine()
					ui.Text(c, subtitle).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(a.colors.muted).SingleLine()
				})
			})
		})
		ui.Box(c).Size(52, 32).Shrink(0).Children(func() {
			if a.switchControl(c, &value, title).Absolute().Left(-4).Top(-8).Changed() {
				a.cfg.Overrides["allow-lan"] = value
				a.apply()
			}
		})
	})
}

func (a *App) desktopProfileContent(c *ui.Context) {
	// profiles.dart: compact puts the expiry/source beside the name and omits
	// update time. Standard adds the metadata line. Both pin stats to the foot.
	profile := a.store.ActiveProfile(a.cfg)
	compact := a.dashboardCompact(c)
	a.dashboardHeader(c, "profiles", a.t("profile"), func() {
		ui.Row(c).Gap(0).Translate(8, 0).Children(func() {
			if profile != nil && profile.URL != "" {
				a.dashboardIconButton(c, "sync", a.t("update"), func() { a.updateProfile(*profile) })
			}
			a.dashboardIconButton(c, "chevron", a.messageText("details", map[string]any{"label": a.t("profile")}), a.openProfileDetails)
		})
	})
	size, leading := float32(17), float32(22)
	if compact {
		size, leading = 16, 21
	}
	name, end := a.t("addProfile"), ""
	endColor := a.colors.muted
	if profile != nil {
		name, end = profile.Name, a.t("local")
		if profile.URL != "" {
			end = a.t("remote")
		}
		if profile.Type == "custom" {
			end = a.t("custom")
		}
		if profile.Expire > 0 {
			days := max(0, int(time.Until(time.Unix(profile.Expire, 0)).Hours()/24))
			end = a.messageText("profileDaysLeft", map[string]any{"days": days})
			if days <= 7 {
				endColor = a.colors.warning
			}
		}
	}
	endWidth, _ := c.MeasureText(0, ui.Span{Text: end, Size: fontSize(c, 12)})
	width := a.dashboardInfoWidth(c)
	ui.Row(c).FillWidth().Height(a.textHeight(leading)).Margin(10, 0, 0, 0).Gap(12).Children(func() {
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(0).Children(func() {
			ui.Text(c, name).MinWidth(0).FontSize(fontSize(c, size)).FixedLineHeight(a.textHeight(leading)).FontWeight(600).SingleLine()
			if profile != nil && len(a.cfg.Profiles) > 1 {
				a.dashboardIconButton(c, "swap", a.t("switchProfile"), a.openProfilePicker).Height(leading)
			}
		})
		if compact && end != "" {
			// DashboardCappedRow measures the end first and scales it to at
			// most half the available row, leaving the name the remainder.
			factor := min(float32(1), max(0, (width-12)/2)/max(endWidth, 1))
			ui.Text(c, end).FontSize(fontSize(c, 12) * factor).FixedLineHeight(a.textHeight(16) * factor).TextColor(endColor).SingleLine()
		}
	})
	if profile == nil {
		return
	}
	if !compact {
		updated, ago := "", ""
		if !profile.Updated.IsZero() {
			ago = a.updatedAgo(profile.Updated)
			updated = a.messageText("profileUpdatedAt", map[string]any{"time": ago})
		}
		updatedWidth, _ := c.MeasureText(0, ui.Span{Text: updated, Size: fontSize(c, 12)})
		if updatedWidth+12+endWidth > width {
			updated = ago
		}
		ui.Row(c).FillWidth().Height(a.textHeight(16)).Gap(12).Children(func() {
			ui.Text(c, updated).Grow(1).Basis(0).MinWidth(0).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(a.colors.muted).SingleLine()
			ui.Text(c, end).FontSize(fontSize(c, 12)).FixedLineHeight(a.textHeight(16)).TextColor(endColor).SingleLine()
		})
	}
	ui.Spacer(c)
	if profile.Total > 0 {
		share := min(max(float64(profile.Upload+profile.Download)/float64(profile.Total), 0), 1)
		percent := fmt.Sprintf("%.0f%%", math.Round(share*100))
		traffic := bytesText(profile.Upload+profile.Download) + " / " + bytesText(profile.Total)
		used := " " + a.t("usedShort")
		percentWidth, _ := c.MeasureText(0, ui.Span{Text: percent, Size: fontSize(c, size), Weight: 600})
		usedWidth, _ := c.MeasureText(0, ui.Span{Text: used, Size: fontSize(c, 13), Weight: 600})
		trafficWidth, _ := c.MeasureText(0, ui.Span{Text: traffic, Size: fontSize(c, 13)})
		if percentWidth+usedWidth+12+trafficWidth > width {
			used = ""
		}
		ui.Row(c).FillWidth().Height(a.textHeight(leading)).AlignItems(ui.Baseline).Gap(12).Children(func() {
			ui.RichText(c,
				ui.Span{Text: percent, Size: fontSize(c, size), Weight: 600},
				ui.Span{Text: used, Size: fontSize(c, 13), Weight: 600, Color: a.colors.muted},
			).FixedLineHeight(a.textHeight(leading)).SingleLine()
			ui.Text(c, traffic).Grow(1).Basis(0).MinWidth(0).FontSize(fontSize(c, 13)).FixedLineHeight(a.textHeight(18)).TextAlign(ui.End).TextColor(a.colors.muted).SingleLine()
		})
		ui.Box(c).FillWidth().Height(12).Margin(6, 0, 0, 0).Draw(func(p *ui.Painter, r ui.Rect) { a.dashboardUsageWave(p, r, share) })
	} else {
		counts := []string{"—", "—", "—"}
		if value := a.sample.Counts; value != nil {
			proxies := value.Proxies
			for _, provider := range a.sample.Providers {
				if provider.Type == "Proxy" {
					proxies += provider.Count
				}
			}
			for i, count := range []int{value.Groups, proxies, value.Rules} {
				counts[i] = number(count)
				if count >= 100000 {
					counts[i] = compactNumber(count)
				}
			}
		}
		keys := []string{"proxyGroup", "proxies", "rules"}
		// _ConfigStats wraps a minimum-width row in FittedBox.scaleDown.
		// Measure the three columns before scaling the entire row, including
		// its line heights and gaps, rather than ellipsizing individual stats.
		naturalWidth := float32(32) // two 16-point inter-column gaps
		for i, key := range keys {
			countWidth, _ := c.MeasureText(0, ui.Span{Text: counts[i], Size: fontSize(c, size), Weight: 600})
			labelWidth, _ := c.MeasureText(0, ui.Span{Text: a.t(key), Size: fontSize(c, 12)})
			naturalWidth += max(countWidth, labelWidth)
		}
		factor := min(float32(1), width/naturalWidth)
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).Gap(16 * factor).Children(func() {
			for i, key := range keys {
				ui.Column(c).Gap(0).Children(func() {
					ui.Text(c, counts[i]).Height(a.textHeight(leading) * factor).FontSize(fontSize(c, size) * factor).FixedLineHeight(a.textHeight(leading) * factor).FontWeight(600).SingleLine()
					ui.Text(c, a.t(key)).Height(a.textHeight(16) * factor).FontSize(fontSize(c, 12) * factor).FixedLineHeight(a.textHeight(16) * factor).TextColor(a.colors.muted).SingleLine()
				})
			}
		})
	}
}

// profiles.dart: _WavyProgressPainter. Stroke, wavelength and gaps remain
// logical sizes; only its text grows with the user's text preference.
func (a *App) dashboardUsageWave(p *ui.Painter, r ui.Rect, share float64) {
	active := a.colors.accent
	if share >= 0.9 {
		active = a.colors.warning
	}
	mid, start, end := r.Y+r.H/2, r.X+2, r.X+r.W-2
	split := start + (end-start)*float32(share)
	if share > 0 {
		path := new(ui.Path).MoveTo(start, mid)
		for x := start; x <= split; x++ {
			path.LineTo(x, mid+3*float32(math.Sin(float64((x-start)/12)*2*math.Pi)))
		}
		p.StrokePath(path, 4, active)
	}
	trackStart := start
	if share > 0 {
		trackStart = split + 10
	}
	if trackStart < end-4 {
		p.Line(trackStart, mid, end-10, mid, 4, a.colors.inset)
	}
	if share < 1 {
		p.FillPath(new(ui.Path).Circle(end-2, mid, 2), active)
	}
}

func (a *App) updatedAgo(at time.Time) string {
	if at.Year() <= 1970 {
		return a.t("unknown")
	}
	duration := time.Since(at)
	for _, item := range []struct {
		duration time.Duration
		key      string
	}{{365 * 24 * time.Hour, "yearsAgo"}, {30 * 24 * time.Hour, "monthsAgo"}, {24 * time.Hour, "daysAgo"}, {time.Hour, "hoursAgo"}, {time.Minute, "minutesAgo"}} {
		if duration >= item.duration {
			return a.messageText(item.key, map[string]any{"count": int(duration / item.duration)})
		}
	}
	return a.t("justNow")
}
