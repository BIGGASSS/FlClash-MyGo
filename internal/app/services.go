package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type serviceTarget struct{ ID, Label, Icon string }

var serviceTargets = []serviceTarget{
	{"google", "Google", "google"}, {"github", "GitHub", "github"}, {"youtube", "YouTube", "youtube"},
	{"chatgpt", "ChatGPT", "openai"}, {"claude", "Claude", "claude"}, {"gemini", "Gemini", "gemini"},
	{"netflix", "Netflix", "netflix"}, {"disney-plus", "Disney+", "disneyplus"}, {"prime-video", "Prime Video", "primevideo"},
	{"spotify", "Spotify", "spotify"}, {"tiktok", "TikTok", "tiktok"}, {"discord", "Discord", "discord"}, {"bilibili", "Bilibili", "bilibili"},
}

func (a *App) orderedServiceTargets() []serviceTarget {
	var result []serviceTarget
	seen := map[string]bool{}
	for _, id := range append(slices.Clone(a.cfg.ServiceOrder), serviceIDs(serviceTargets)...) {
		for _, target := range serviceTargets {
			if target.ID == id && !seen[id] {
				seen[id] = true
				result = append(result, target)
			}
		}
	}
	return result
}

func serviceIDs(targets []serviceTarget) []string {
	names := make([]string, len(targets))
	for i, target := range targets {
		names[i] = target.ID
	}
	return names
}

func (a *App) enabledServiceTargets() []serviceTarget {
	all := a.orderedServiceTargets()
	var enabled []serviceTarget
	for _, target := range all {
		if !slices.Contains(a.cfg.DisabledServices, target.ID) {
			enabled = append(enabled, target)
		}
	}
	if len(enabled) == 0 {
		return all
	}
	return enabled
}

func (a *App) setServiceEnabled(id string, enabled bool) {
	current := a.enabledServiceTargets()
	if !enabled && len(current) == 1 && current[0].ID == id {
		return
	}
	a.cfg.DisabledServices = slices.DeleteFunc(a.cfg.DisabledServices, func(value string) bool { return value == id })
	if !enabled {
		a.cfg.DisabledServices = append(a.cfg.DisabledServices, id)
	}
	a.save()
}

func (a *App) serviceValue(id string) *Service {
	for i := range a.services {
		if a.services[i].Name == id {
			return &a.services[i]
		}
	}
	return nil
}

func (a *App) markServiceRoutes() {
	a.serviceRouteRev++
	a.serviceRouteDue = time.Now().Add(1500 * time.Millisecond)
}

func (a *App) refreshServices(targets []serviceTarget) {
	if a.engine == nil || a.safe || len(targets) == 0 {
		return
	}
	if a.servicePending == nil {
		a.servicePending = map[string]uint64{}
		a.serviceLast = map[string]time.Time{}
	}
	names := []string{}
	a.serviceSerial++
	serial := a.serviceSerial
	for _, target := range targets {
		if a.servicePending[target.ID] == 0 {
			names = append(names, target.ID)
			a.servicePending[target.ID] = serial
			a.serviceLast[target.ID] = time.Now()
		}
	}
	if len(names) == 0 {
		return
	}
	e, revision, proxied, routeRevision := a.engine, a.engine.Revision(), a.wantRunning, a.serviceRouteRev
	proxy := ""
	if !proxied {
		proxy = "DIRECT"
	}
	a.work(func() (func(), error) {
		ctx, cancel := context.WithTimeout(e.ctx, 65*time.Second)
		defer cancel()
		var results []Service
		err := e.CallContext(ctx, "serviceCheck", map[string]any{"proxy-name": proxy, "names": names, "timeout": 10000}, &results)
		return func() {
			if e.Revision() != revision || a.wantRunning != proxied {
				a.finishServices(names, serial, nil, true)
				a.markServiceRoutes()
				return
			}
			if err != nil {
				results = nil
			}
			a.finishServices(names, serial, results, false)
			if routeRevision != a.serviceRouteRev {
				a.markServiceRoutes()
			}
		}, nil
	})
}

func (a *App) finishServices(names []string, serial uint64, results []Service, superseded bool) {
	for _, name := range names {
		if a.servicePending[name] != serial {
			continue
		}
		delete(a.servicePending, name)
		if superseded {
			continue
		}
		value := Service{Name: name, Status: "failed", CheckedAt: time.Now().UnixMilli()}
		for _, result := range results {
			if result.Name == name {
				value = result
				break
			}
		}
		if current := a.serviceValue(name); current != nil {
			*current = value
		} else {
			a.services = append(a.services, value)
		}
	}
}

func (a *App) watchServices(c *ui.Context) {
	if a.safe || a.engine == nil || a.page != "dashboard" || (a.window != nil && !a.window.IsVisible()) {
		return
	}
	if !a.serviceHostSet || a.serviceProxied != a.wantRunning {
		a.serviceHostSet, a.serviceProxied = true, a.wantRunning
		a.services = nil
		a.servicePending = map[string]uint64{}
		a.serviceLast = map[string]time.Time{}
		a.markServiceRoutes()
	}
	if !a.serviceRouteDue.IsZero() {
		if delay := time.Until(a.serviceRouteDue); delay > 0 {
			c.After(delay)
		} else if !a.serviceRouteRun {
			a.verifyServiceRoutes()
		}
	}
	shown := a.dashboardServiceTargets(c)
	var missing []serviceTarget
	for _, target := range shown {
		if a.serviceValue(target.ID) == nil && a.servicePending[target.ID] == 0 {
			if wait := 5*time.Second - time.Since(a.serviceLast[target.ID]); wait > 0 {
				c.After(wait)
			} else {
				missing = append(missing, target)
			}
		}
	}
	a.refreshServices(missing)
}

func (a *App) verifyServiceRoutes() {
	a.serviceRouteDue = time.Time{}
	var names []string
	var routes [][]routeHit
	var epochs []uint64
	var checkedAt []int64
	for _, value := range a.services {
		if a.servicePending[value.Name] == 0 {
			names, routes, epochs = append(names, value.Name), append(routes, value.Hits), append(epochs, value.CoreEpoch)
			checkedAt = append(checkedAt, value.CheckedAt)
		}
	}
	if len(names) == 0 {
		return
	}
	e, revision := a.engine, a.serviceRouteRev
	a.serviceRouteRun = true
	a.work(func() (func(), error) {
		var checked struct {
			CoreEpoch    uint64 `json:"core-epoch"`
			RouteVersion uint64 `json:"route-version"`
			Held         []bool `json:"held"`
		}
		err := e.Call("checkRoutes", map[string]any{"routes": routes}, &checked)
		return func() {
			a.serviceRouteRun = false
			if revision != a.serviceRouteRev {
				return
			}
			if err != nil {
				a.serviceRouteDue = time.Now().Add(5 * time.Second)
				return
			}
			a.applyServiceRouteCheck(names, epochs, checkedAt, checked.Held, checked.CoreEpoch, checked.RouteVersion)
		}, nil
	})
}

func (a *App) applyServiceRouteCheck(names []string, epochs []uint64, checkedAt []int64, held []bool, epoch, version uint64) {
	for i, name := range names {
		if value := a.serviceValue(name); value == nil || a.servicePending[name] != 0 || value.CheckedAt != checkedAt[i] {
			continue
		}
		if i >= len(held) || !held[i] || epochs[i] != epoch {
			a.services = slices.DeleteFunc(a.services, func(value Service) bool { return value.Name == name })
		} else if value := a.serviceValue(name); value != nil {
			value.RouteVersion = version
		}
	}
}

func (a *App) dashboardServiceTargets(c *ui.Context) []serviceTarget {
	width, _ := c.Size()
	side := a.sidebarSize()
	count := 2
	if !a.dashboardNarrow(c) && (width-side < 860 || (width-side-40-3*a.dashboardGap(c))/4 >= 290) {
		count = 4
	}
	all := a.enabledServiceTargets()
	return all[:min(len(all), count)]
}

func (a *App) serviceStatus(id string, delay bool) (string, ui.Color) {
	if a.servicePending[id] != 0 {
		return a.t("loading"), a.colors.muted
	}
	value := a.serviceValue(id)
	if value == nil {
		return a.t("servicePending"), a.colors.muted
	}
	key := map[string]string{"available": "serviceAvailable", "unavailable": "serviceUnavailable", "restricted": "serviceRestricted", "disallowed-isp": "serviceDisallowedIsp", "blocked": "serviceBlocked", "unsupported-region": "serviceUnsupportedRegion", "originals-only": "serviceOriginalsOnly", "coming-soon": "serviceComingSoon", "timeout": "timeout", "failed": "serviceFailed"}[value.Status]
	if key == "" {
		key = "serviceFailed"
	}
	color := a.colors.warning
	if value.Status == "available" {
		if delay && value.Delay > 0 {
			return fmt.Sprintf("%d ms", value.Delay), a.colors.muted
		}
		color = a.colors.success
	} else if value.Status == "unavailable" || key == "timeout" || key == "serviceFailed" {
		color = a.colors.danger
	}
	return a.t(key), color
}

func (a *App) openServices() { a.serviceOpen, a.serviceManage = true, false }

func (a *App) sideSheet(c *ui.Context, key string, open *bool, content func()) {
	a.dialogBase(c.Key(key), open, func(backdrop, panel ui.Element) {
		width, _ := c.Size()
		backdrop.Background(ui.RGBA(0, 0, 0, 0.32))
		panel.Column().Absolute().Top(0).Bottom(0).Right(0).Width(min(max(width*0.4, 360), min(width, 560))).Background(a.colors.page).Children(content)
	})
}

func (a *App) serviceSheet(c *ui.Context) {
	a.sideSheet(c, "services", &a.serviceOpen, func() {
		if a.serviceManage {
			a.sheetHeader(c, a.t("serviceManage"), "back", a.t("back"), func() { a.serviceManage = false }, nil)
		} else {
			a.sheetHeader(c, a.t("serviceStatus"), "close", a.t("close"), func() { a.serviceOpen = false }, func() {
				ui.Row(c).Gap(0).Radius(999).Background(a.colors.card).Children(func() {
					a.sheetButton(c, "bolt", a.t("serviceCheckAll"), a.checkServices)
					a.sheetButton(c, "sliders", a.t("serviceManage"), func() { a.serviceManage = true })
				})
			})
		}
		if a.serviceManage {
			a.serviceManageRows(c)
		} else {
			a.serviceRows(c)
		}
	})
}

func (a *App) serviceRows(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(16).Children(func() {
			ui.Column(c).Radius(24).Background(a.colors.card).Clip().Children(func() {
				for i, target := range a.enabledServiceTargets() {
					ui.ButtonBase(c.Key(target.ID)).Label(target.Label).FillWidth().Background(a.colors.card).Disabled(a.servicePending[target.ID] != 0).OnClick(func() { a.refreshServices([]serviceTarget{target}) }).Children(func() {
						ui.Row(c).FillWidth().MinHeight(63).Padding(0, 16, 0, 12).Gap(16).Children(func() {
							a.serviceBadge(c, target, true)
							ui.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
								ui.Text(c, target.Label).FontSize(fontSize(c, 17)).SingleLine()
								var route []string
								if value := a.serviceValue(target.ID); value != nil {
									if value.Region != "" {
										route = append(route, value.Region)
									}
									if len(value.Chains) > 0 {
										route = append(route, value.Chains[0])
									}
								}
								label := strings.Join(route, " · ")
								if label == "" {
									label = "—"
								}
								ui.Text(c, label).FontSize(fontSize(c, 13)).Height(17).TextColor(a.colors.muted).SingleLine()
							})
							label, color := a.serviceStatus(target.ID, true)
							ui.Text(c, label).FontSize(fontSize(c, 13)).FontWeight(600).TextColor(color).MaxWidth(190).Tooltip(label)
						})
					}).Radius(0)
					if i < len(a.enabledServiceTargets())-1 {
						ui.Box(c).Height(1).Margin(0, 14, 0, 68).Background(a.colors.line.Alpha(0.6))
					}
				}
			})
		})
	})
}

func (a *App) serviceManageRows(c *ui.Context) {
	targets, enabled := a.orderedServiceTargets(), a.enabledServiceTargets()
	shown := a.dashboardServiceTargets(c)
	a.serviceList.Reorder = func(rows []int, to int) {
		selected := map[int]bool{}
		for _, row := range rows {
			selected[row] = true
		}
		var moved, rest []string
		before := 0
		for i, target := range targets {
			if selected[i] {
				moved = append(moved, target.ID)
				if i < to {
					before++
				}
			} else {
				rest = append(rest, target.ID)
			}
		}
		at := min(max(to-before, 0), len(rest))
		a.cfg.ServiceOrder = append(append(append([]string{}, rest[:at]...), moved...), rest[at:]...)
		a.save()
	}
	ui.Column(c).Fill().Padding(16, 16, 0, 16).Children(func() {
		ui.List(c, &a.serviceList, len(targets), func(i int) {
			target := targets[i]
			on := slices.Contains(serviceIDs(enabled), target.ID)
			locked := on && len(enabled) == 1
			isShown := slices.Contains(serviceIDs(shown), target.ID)
			height := float32(56)
			if isShown {
				height = 66
			}
			ui.Column(c.Key(target.ID)).FillWidth().Children(func() {
				row := ui.Row(c).FillWidth().Height(height).Padding(0, 16, 0, 12).Gap(16).Background(a.colors.card).Children(func() {
					if i < len(targets)-1 {
						ui.Box(c).Absolute().Left(68).Right(14).Bottom(0).Height(1).Background(a.colors.line.Alpha(0.6))
					}
					a.serviceBadge(c, target, false)
					ui.ButtonBase(c).Label(target.Label).Disabled(locked).Grow(1).MinWidth(0).OnClick(func() { a.setServiceEnabled(target.ID, !on) }).Children(func() {
						ui.Column(c).FillWidth().AlignItems(ui.Start).Gap(2).Children(func() {
							ui.Text(c, target.Label).FontSize(fontSize(c, 17))
							if isShown {
								ui.Text(c, a.t("serviceShownOnDashboard")).FontSize(fontSize(c, 15)).TextColor(a.colors.muted)
							}
						})
					})
					if a.switchControl(c, &on, target.Label).Disabled(locked).Changed() {
						a.setServiceEnabled(target.ID, on)
					}
				})
				if i == 0 {
					row.Radius(24, 24, 0, 0)
				}
				if i == len(targets)-1 {
					row.Radius(0, 0, 24, 24)
					ui.Box(c).Height(16)
				}
			})
		}).Fill().Clip()
	})
}

func (a *App) serviceBadge(c *ui.Context, target serviceTarget, status bool) {
	ui.Box(c).Size(40, 40).Radius(12).Background(a.colors.block).Center().Children(func() {
		serviceIcon(c, target.Icon, 24)
		if status && (a.serviceValue(target.ID) != nil || a.servicePending[target.ID] != 0) {
			_, color := a.serviceStatus(target.ID, false)
			ui.Box(c).Absolute().Right(-1).Bottom(-1).Size(10, 10).Radius(999).BorderWidth(2).BorderColor(a.colors.card).Background(color)
		}
	})
}

func (a *App) sheetButton(c *ui.Context, icon, label string, action func()) ui.Element {
	return ui.ButtonBase(c).Label(label).Tooltip(label).Size(44, 44).Radius(999).Background(a.colors.card).Center().OnClick(action).Children(func() { glyphVariant(c, icon, true).FontSize(24) })
}

func (a *App) sheetHeader(c *ui.Context, title, icon, leadingLabel string, leading func(), actions func()) {
	if a.compactSheet {
		ui.Column(c).FillWidth().Height(68).Gap(0).Children(func() {
			ui.Box(c).FillWidth().Height(10).Children(func() {
				ui.Box(c).Absolute().Top(6).LeftPercent(50).Margin(0, 0, 0, -14).Size(28, 4).Radius(999).Background(a.colors.muted)
			})
			ui.Box(c).FillWidth().Height(48).Children(func() {
				ui.Text(c, title).Absolute().Left(72).Right(72).Top(13).FontSize(fontSize(c, 17)).FixedLineHeight(a.textHeight(22)).FontWeight(600).TextAlign(ui.Center).SingleLine()
				ui.Box(c).Absolute().Left(16).Top(2).Size(44, 44).Children(func() { a.sheetButton(c, icon, leadingLabel, leading) })
				if actions != nil {
					ui.Row(c).Absolute().Right(16).Top(2).Height(44).Gap(0).Children(actions)
				}
			})
			ui.Box(c).Height(10)
		})
		return
	}
	bar := c.TitleBar()
	ui.Row(c).FillWidth().Height(72).Padding(14, max(16, bar.Right+16), 14, 16).Gap(16).Children(func() {
		a.sheetButton(c, icon, leadingLabel, leading)
		ui.Text(c, title).FontSize(fontSize(c, 24)).FontWeight(600).Grow(1).SingleLine()
		if actions != nil {
			actions()
		}
	})
}
