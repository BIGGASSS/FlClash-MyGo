package app

import (
	"slices"
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func (a *App) View(c *ui.Context) {
	a.flushUpdates()
	a.theme(c)
	a.frameHeight = a.captionHeight(c)
	a.iconFrame++
	for id, mount := range a.iconMounts {
		if mount.frame+1 < a.iconFrame {
			delete(a.iconMounts, id)
		}
	}
	if a.page != "dashboard" {
		a.corePhase = ""
		a.coreHoldUntil = time.Time{}
	}
	a.watchServices(c)
	a.watchMemory(c)
	a.watchDelayTests()
	if !a.pendingApply.IsZero() {
		if !c.Now().Before(a.pendingApply) {
			a.pendingApply = time.Time{}
			a.apply()
		} else {
			c.After(a.pendingApply.Sub(c.Now()))
		}
	}
	if a.message != "" {
		c.Toast(a.message)
		a.message = ""
	}
	if !a.profileShare.Open && c.Shortcut(ui.Cmd, ui.KeyLeft) && (len(a.history) > 0 || a.profilePickerNavigation() != nil) {
		a.back()
	}
	root := ui.Box(c).Fill().Background(a.colors.page).Children(func() {
		a.withPickerBackground(func() {
			ui.Row(c).Fill().Padding(a.frameHeight, 0, a.navigationReservedHeight(c), 0).Gap(0).AlignItems(ui.Stretch).Children(func() {
				a.sidebar(c)
				ui.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
					a.titlebar(c)
					pageKey := a.page
					if a.frameEnabled {
						pageKey = a.navigationParent() + "/" + a.page
					}
					ui.Column(c.Key(pageKey)).Grow(1).MinHeight(0).MinWidth(0).Children(func() {
						switch a.page {
						case "dashboard":
							a.dashboard(c)
						case "proxies":
							a.proxiesView(c)
						case "profiles":
							a.profilesView(c)
						case "connections", "requests", "dns", "logs":
							a.records(c)
						case "tools":
							a.tools(c)
						case "resources":
							a.resources(c)
						case "scripts":
							a.scriptsView(c)
						case "profileEdit":
							a.profileEdit(c)
						case "profileCustom", "profileExtend", "profileRename":
							a.profileActionView(c, a.page)
						case "editor":
							a.editorView(c)
						case "collection":
							a.collectionView(c)
						case "theme":
							a.themeView(c)
						case "fontFamily":
							a.fontPickerView(c)
						case "backup":
							a.backupView(c)
						case "hotkeys":
							a.hotkeysView(c)
						case "language":
							a.languageView(c)
						case "about":
							a.aboutView(c)
						case "services":
							a.servicesView(c)
						case "probe":
							a.probeView(c)
						case "onDemand":
							a.onDemandView(c)
						default:
							a.settingsView(c)
						}
					})
				})
			})
			a.bottomNavigation(c)
		})
	})
	for _, path := range root.DroppedFiles() {
		a.importFile(path)
	}
	a.sidebarOverlayView(c)
	a.hotkeyDialog(c)
	a.serviceSheet(c)
	a.memorySheet(c)
	a.profileDetailSheet(c)
	a.dashboardPickerSheet(c)
	a.fontPickerSheet(c)
	a.qualityDialog(c)
	a.modals(c)
	if a.frameHeight > 0 {
		ui.Overlay(c.Key("window-caption"), func() { a.captionBar(c) })
	}
}
func (a *App) pageTitle() string {
	switch a.page {
	case "collection":
		if a.document != nil {
			return a.document.Title
		}
		return a.t("edit")
	case "connections":
		return a.t("liveConnections")
	case "dns":
		return a.t("dnsQueries")
	case "profileEdit", "profileCustom", "profileRename":
		return a.profileDraft.Name
	case "profileExtend":
		return a.t("extend")
	case "editor":
		return a.codeTitle
	case "backup":
		return a.t("backupAndRestore")
	case "hotkeys":
		return a.t("hotkey")
	case "general":
		return a.t("general")
	case "advanced":
		return a.t("advancedConfig")
	case "services":
		return a.t("serviceStatus")
	default:
		return a.t(a.page)
	}
}
func (a *App) titlebar(c *ui.Context) {
	if !a.dashboardNarrow(c) || a.page != "dashboard" {
		a.dashboardEditing = false
	}
	if a.dashboardEditing && c.Shortcut(0, ui.KeyEscape) {
		a.dashboardEditing = false
	}
	bar := c.TitleBar()
	ui.Row(c).Height(68).Padding(14, max(bar.Right+16, 20), 10, 16).Gap(8).DragWindow().Children(func() {
		back := a.page != "dashboard" && a.page != "proxies" && a.page != "profiles" && a.page != "connections" && a.page != "tools"
		if a.frameEnabled {
			back = !slices.Contains(a.navigationPages(a.smallDesktop(c)), a.page)
		}
		if back {
			a.button(c, "back", a.t("back"), a.back).Background(ui.Transparent)
		}
		if a.page == "proxies" && a.proxySearch {
			ui.SearchField(c, &a.proxyQuery).Label(a.t("search")).Grow(1).AutoFocus().OnSubmit(func() {})
		} else {
			title := a.pageTitle()
			if a.dashboardEditing {
				title = a.t("editDashboard")
			}
			ui.Text(c, title).FontSize(fontSize(c, 24)).FontWeight(600).Grow(1).SingleLine()
		}
		if a.working > 0 {
			ui.Text(c, "…").TextColor(a.colors.muted).Label(a.t("loading"))
		}
		switch a.page {
		case "dashboard":
			if a.dashboardEditing {
				a.sheetButton(c, "check", a.t("done"), func() { a.dashboardEditing = false }).Background(ui.Transparent).Draw(func(p *ui.Painter, r ui.Rect) {
					p.Fill(ui.Rect{X: r.X - 6, Y: r.Y - 6, W: r.W + 12, H: r.H + 12}, a.colors.card, 28)
				})
			} else {
				if a.dashboardNarrow(c) {
					a.sheetButton(c, "edit", a.t("editDashboard"), func() { a.dashboardEditing = true; a.dashboardList = ui.ListState{} }).Size(44, 44).Background(a.colors.card)
				}
				a.coreStatusControl(c)
			}
		case "profiles":
			a.button(c, "refresh", a.t("updateAll"), a.updateAllProfiles)
			a.button(c, "plus", a.t("addProfile"), func() { a.addName = ""; a.addSource = ""; a.addOpen = true })
		case "scripts":
			a.button(c, "plus", a.t("add"), a.addScript)
		case "proxies":
			if a.cfg.ProxyStyle == "tab" {
				a.proxyTestButton(c)
				a.button(c, "locate", a.t("scrollToSelected"), func() { a.locateProxy(a.group) })
			}
			icon := "search"
			if a.proxySearch {
				icon = "close"
			}
			a.button(c, icon, a.t("search"), func() {
				a.proxySearch = !a.proxySearch
				if !a.proxySearch {
					a.proxyQuery = ""
				}
			}).Size(44, 44).Background(a.colors.card)
			a.button(c, "more", a.t("more"), nil).Size(44, 44).Background(a.colors.card).Menu(a.proxyMenu)
		case "connections", "requests", "dns", "logs":
			a.button(c, func() string {
				if a.paused {
					return "play"
				}
				return "pause"
			}(), a.t("pause"), func() { a.paused = !a.paused })
			a.button(c, func() string {
				if a.recordsTable {
					return "list"
				}
				return "table"
			}(), a.t("view"), func() { a.recordsTable = !a.recordsTable })
			a.button(c, "trash", a.t("clear"), func() { a.clearRecords() })
		case "editor":
			a.button(c, "check", a.t("save"), a.saveCode)
		case "collection":
			a.button(c, "plus", a.t("add"), a.documentAdd)
			a.button(c, "check", a.t("save"), a.saveDocument)
		case "profileEdit", "profileCustom", "profileExtend", "profileRename":
			a.button(c, "check", a.t("save"), a.saveProfile)
		case "resources":
			a.button(c, "refresh", a.t("updateAll"), func() {
				for _, p := range a.sample.Providers {
					a.updateDetailProvider(p)
				}
			})
		case "services":
			a.button(c, "refresh", a.t("serviceCheckAll"), a.checkServices)
		}
	})
}
func (a *App) modals(c *ui.Context) {
	a.profileShareDialog(c)
	a.onDemandDialog(c)
	a.providerDialogs(c)
	a.dialogBase(c.Key("add-dialog"), &a.addOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.3))
		panel.Width(520).MaxWidthPercent(90).Padding(28).Gap(18).Radius(34).Background(a.colors.card)
		ui.Text(c, a.t("addProfile")).FontSize(fontSize(c, 22)).Bold()
		seg(c, &a.addKind, a.t("url"), a.t("file"), a.t("clipboard"), a.t("custom"))
		ui.TextInput(c, &a.addName).Label(a.t("name")).Placeholder(a.t("name")).Radius(14).Padding(12)
		if a.addKind != 3 {
			ui.TextInput(c, &a.addSecret).Password().Label("Age secret key").Placeholder("Age secret key (optional)").Radius(14).Padding(12)
		}
		if a.addKind == 1 {
			ui.Button(c, a.t("chooseFile")).Radius(999).OnClick(func() { a.chooseImport() })
		} else if a.addKind != 3 {
			ui.TextArea(c, &a.addSource).Label(a.t("url")).Placeholder(func() string {
				if a.addKind == 2 {
					return a.t("paste")
				}
				return "https://example.com/subscription"
			}()).Height(100).Padding(12).Radius(16)
			if a.addKind == 2 {
				ui.Button(c, a.t("paste")).OnClick(func() { a.addSource = c.Services().ReadClipboard() })
			}
		}
		ui.Row(c).Justify(ui.End).Gap(10).Children(func() {
			ui.Button(c, a.t("cancel")).Radius(999).OnClick(func() { a.addOpen = false })
			ui.PrimaryButton(c, a.t("import")).Radius(999).Padding(10, 24).Disabled(a.working > 0 || a.addKind == 1).OnClick(a.importSource)
		})
	})
	a.dialogBase(c.Key("details"), &a.detailOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.3))
		panel.Width(700).MaxWidthPercent(90).HeightPercent(80).Padding(24).Gap(16).Radius(28).Background(a.colors.card)
		ui.Row(c).Children(func() {
			ui.Text(c, a.t("details")).FontSize(fontSize(c, 22)).Bold().Grow(1)
			a.button(c, "copy", a.t("copy"), func() { c.Services().WriteClipboard(a.detail) })
			a.button(c, "close", a.t("close"), func() { a.detailOpen = false })
		})
		ui.Scroll(c).Grow(1).FillWidth().Children(func() { ui.Text(c, a.detail).Selectable().FontSize(fontSize(c, 14)) })
	})
	a.dialogBase(c.Key("confirm"), &a.confirmOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.3))
		panel.Width(440).Padding(28).Gap(20).Radius(34).Background(a.colors.card)
		ui.Text(c, a.confirmTitle).FontSize(fontSize(c, 20)).Bold()
		ui.Text(c, a.confirmText).TextColor(a.colors.muted)
		ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
			ui.Button(c, a.t("cancel")).Radius(999).OnClick(func() { a.confirmOpen = false })
			ui.PrimaryButton(c, a.t("confirm")).Radius(999).OnClick(func() {
				fn := a.confirm
				a.confirmOpen = false
				if fn != nil {
					fn()
				}
			})
		})
	})
	errorOpen := a.err != ""
	a.dialogBase(c.Key("error"), &errorOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.3))
		panel.Width(500).MaxHeightPercent(80).Padding(28).Gap(20).Radius(34).Background(a.colors.card)
		ui.Text(c, a.t("error")).FontSize(fontSize(c, 22)).Bold()
		ui.Scroll(c).MaxHeight(300).Children(func() { ui.Text(c, a.err).Selectable() })
		ui.PrimaryButton(c, a.t("confirm")).Radius(999).OnClick(func() { a.err = "" })
	})
	if !errorOpen {
		a.err = ""
	}
	a.dialogBase(c.Key("disclaimer"), &a.disclaimer, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.4))
		panel.Width(700).HeightPercent(88).Padding(28).Gap(18).Radius(34).Background(a.colors.card)
		ui.Text(c, a.t("disclaimer")).FontSize(fontSize(c, 24)).Bold()
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).Gap(14).Children(func() {
				ui.Text(c, a.t("disclaimerDesc"))
				for _, section := range []string{"Software", "Usage", "Responsibility", "ThirdParty", "Warranty", "Liability"} {
					ui.Text(c, a.t("disclaimer"+section+"Title")).Bold()
					ui.Text(c, a.t("disclaimer"+section+"Content")).FontSize(fontSize(c, 14)).TextColor(a.colors.muted)
				}
				ui.Text(c, a.t("disclaimerNoServiceStatement")).FontSize(fontSize(c, 14))
			})
		})
		ui.Row(c).Gap(12).Justify(ui.End).Children(func() {
			ui.Button(c, a.t("exit")).Radius(999).OnClick(func() { mygo.App.Quit() })
			ui.PrimaryButton(c, a.t("agree")).Radius(999).OnClick(func() { a.cfg.Disclaimer = true; a.disclaimer = false; a.save(); a.begin() })
		})
	})
	if !a.cfg.Disclaimer && !a.safe {
		a.disclaimer = true
	}
}
func (a *App) editorView(c *ui.Context) {
	if a.code == nil {
		return
	}
	ui.Column(c).Fill().Padding(0, 16, 16, 16).Gap(8).Children(func() {
		ui.Text(c, a.codeTitle).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
		editor.View(c, a.code).Fill().Radius(20)
		ui.Text(c, "YAML / JavaScript    ·    "+a.t("save")+"  ⌘/Ctrl S").FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
	})
	if a.picker.Navigation == nil && c.Shortcut(ui.Cmd, ui.KeyS) {
		a.saveCode()
	}
}
func (a *App) clearRecords() {
	switch a.page {
	case "connections":
		a.ask(a.t("closeAllConnections"), a.t("confirm"), func() { a.coreAction("closeConnections", nil) })
	case "requests":
		a.requests = nil
	case "dns":
		a.dns = nil
	case "logs":
		a.logs = nil
	}
	a.recordSelected = -1
}
func (a *App) mode(c *ui.Context) {
	mode := store.String(a.cfg.Overrides["mode"])
	index := 0
	switch mode {
	case "global":
		index = 1
	case "direct":
		index = 2
	}
	old := index
	seg(c, &index, a.t("rule"), a.t("global"), a.t("direct"))
	if old != index {
		a.cfg.Overrides["mode"] = []string{"rule", "global", "direct"}[index]
		a.apply()
	}
}
func (a *App) toggleSetting(c *ui.Context, label string, value *bool) {
	if a.switchControl(c, value, label).Changed() {
		a.apply()
	}
}
func humanKey(s string) string { return titleCase(strings.ReplaceAll(s, "-", " ")) }
