package app

import "github.com/egoist/mygo/ui"

type proxyPageState struct {
	proxyQuery  string
	proxySearch bool
	proxyList   ui.ListState
	proxyScroll ui.ScrollState
	proxyGrids  map[string]*ui.GridState
	proxyLocate string
}

type dashboardPickerState struct {
	Open     bool
	Kind     string
	Profiles profileGridState
	Proxy    proxyPageState
}

func (a *App) proxyViewState() *proxyPageState {
	if a.picker.Open && a.picker.Kind == "proxies" {
		return &a.picker.Proxy
	}
	return &a.proxyPageState
}

func (a *App) openProfilePicker() {
	if a.picker.Open {
		a.closeDashboardPicker()
	}
	a.resetSheet("dashboard-picker")
	a.picker = dashboardPickerState{Open: true, Kind: "profiles"}
}

func (a *App) openProxyPicker(group string) {
	if a.picker.Open {
		a.closeDashboardPicker()
	}
	a.resetSheet("dashboard-picker")
	a.picker = dashboardPickerState{Open: true, Kind: "proxies"}
	if group != "" {
		a.selectProxyGroup(group)
	}
}

func (a *App) closeDashboardPicker() {
	if !a.picker.Open {
		return
	}
	a.picker.Open = false
}

func (a *App) sideSheetWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	if w <= 600 {
		return w
	}
	return min(max(w*.4, 360), min(w, 560))
}

func (a *App) dashboardPickerSheet(c *ui.Context) {
	if !a.picker.Open {
		return
	}
	natural := float32(1 << 30)
	if a.picker.Kind == "profiles" {
		natural = 68 + 16 + float32(len(a.cfg.Profiles))*(a.textHeight(22)+4+a.textHeight(18)+20+8) - 8
	}
	a.navigationSheet(c, "dashboard-picker", &a.picker.Open, a.closeDashboardPicker, a.closeDashboardPicker, func() {
		if a.picker.Kind == "profiles" {
			a.sheetHeader(c, a.t("profiles"), "close", a.t("close"), a.closeDashboardPicker, nil)
			a.profilesGridView(c, &a.picker.Profiles, a.sideSheetWidth(c), true)
			return
		}
		ui.Box(c).Fill().Children(func() {
			ui.Column(c).Fill().Children(func() {
				if a.picker.Proxy.proxySearch && !a.compactSheet {
					ui.Row(c).FillWidth().Height(72).Padding(14, 16).Gap(8).Children(func() {
						ui.SearchField(c, &a.picker.Proxy.proxyQuery).Grow(1).MinWidth(0).AutoFocus().Label(a.t("search"))
						a.sheetButton(c, "close", a.t("close"), func() { a.picker.Proxy.proxySearch = false; a.picker.Proxy.proxyQuery = "" })
						if a.cfg.ProxyStyle == "tab" {
							a.proxyTestButton(c)
						}
					})
				} else {
					var actions func()
					if !a.compactSheet || a.cfg.ProxyStyle == "tab" {
						actions = func() {
							if a.cfg.ProxyStyle == "tab" {
								a.proxyTestButton(c)
							}
							if !a.compactSheet {
								a.sheetButton(c, "search", a.t("search"), func() { a.picker.Proxy.proxySearch = true })
							}
						}
					}
					a.sheetHeader(c, a.t("proxies"), "close", a.t("close"), a.closeDashboardPicker, actions)
				}
				a.proxiesView(c)
			})
			if a.compactSheet {
				ui.SearchField(c, &a.picker.Proxy.proxyQuery).Absolute().Left(16).Right(16).Bottom(16).Height(48).Radius(24).Background(a.colors.card).FontSize(fontSize(c, 17)).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, .1)).Label(a.t("search"))
			}
		})
	}, natural)
}
