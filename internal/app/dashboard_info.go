package app

import (
	"math"
	"time"

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
