package app

import (
	"fmt"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"go.yaml.in/yaml/v3"
)

type proxyPageState struct {
	proxyQuery  string
	proxySearch bool
	proxyList   ui.ListState
	proxyScroll ui.ScrollState
	proxyGrids  map[string]*ui.GridState
	proxyLocate string
}

type dashboardPickerState struct {
	Open            bool
	Kind            string
	Profiles        profileGridState
	Proxy           proxyPageState
	Navigation      *profilePickerNavigation
	BackgroundBuild bool
}

// Editor buffers belong to a sheet session, not the dashboard tab. Only native
// state is retained here; elements are rebuilt under a stable page key.
type profilePickerNavigation struct {
	Owner          string
	Background     navigationPageState
	Page           string
	Stack          []profilePickerPage
	Scroll         ui.ScrollState
	Initial        store.Profile
	Source         *string
	SourceOriginal *string
	Busy           bool
	Revision       uint64
}

func (n *profilePickerNavigation) sourceDirty() bool {
	return n.Source != nil && (n.SourceOriginal == nil || *n.Source != *n.SourceOriginal)
}

type profilePickerPage struct {
	Page   string
	State  navigationPageState
	Scroll ui.ScrollState
}

func (a *App) profilePickerNavigation() *profilePickerNavigation {
	if a.picker.Open && !a.picker.BackgroundBuild {
		return a.picker.Navigation
	}
	return nil
}

func (a *App) editorOwner() string {
	if nav := a.profilePickerNavigation(); nav != nil {
		return nav.Owner
	}
	return a.navigationParent()
}

func (a *App) withPickerBackground(build func()) {
	nav := a.profilePickerNavigation()
	if nav == nil {
		build()
		return
	}
	foreground := a.captureNavigationState()
	a.restoreNavigationState(nav.Background)
	a.picker.BackgroundBuild = true
	defer func() {
		nav.Background = a.captureNavigationState()
		foreground.Page, foreground.History = nav.Background.Page, nav.Background.History
		a.restoreNavigationState(foreground)
		a.picker.BackgroundBuild = false
	}()
	build()
}

func (a *App) beginProfilePickerNavigation() *profilePickerNavigation {
	if nav := a.profilePickerNavigation(); nav != nil {
		return nav
	}
	nav := &profilePickerNavigation{Owner: fmt.Sprintf("profile-sheet:%d", a.beginEditorLoad()), Background: a.captureNavigationState(), Page: "profiles"}
	a.picker.Navigation = nav
	a.code, a.codeProvider, a.document = nil, nil, nil
	a.codePurpose, a.codeTitle, a.codeOriginal = "", "", ""
	a.editProfile, a.profileDraft = "", store.Profile{}
	return nav
}

func (a *App) pushProfilePickerPage(page string) bool {
	nav := a.profilePickerNavigation()
	if nav == nil {
		return false
	}
	if page != "editor" && page != "collection" && page != "profileEdit" && page != "profileCustom" && page != "profileExtend" && page != "profileRename" {
		a.requestPickerClose(func() { a.goPage(page) })
		return true
	}
	if nav.Page != page {
		a.editorLoadSerial++
		nav.Revision++
		nav.Stack = append(nav.Stack, profilePickerPage{Page: nav.Page, State: a.captureNavigationState(), Scroll: nav.Scroll})
		nav.Page, nav.Scroll, nav.Busy = page, ui.ScrollState{}, false
	}
	return true
}

func (a *App) pickerPageDirty(page string, state navigationPageState) bool {
	switch page {
	case "editor":
		return state.Code != nil && !state.Code.ReadOnly && state.Code.Buffer.Modified()
	case "collection":
		if state.Document != nil {
			b, _ := yaml.Marshal(state.Document.Value)
			return string(b) != state.Document.Original
		}
	case "profileEdit", "profileExtend", "profileRename":
		if nav := a.profilePickerNavigation(); nav != nil {
			return !profileDraftEqual(state.ProfileDraft, nav.Initial) || nav.sourceDirty()
		}
	}
	return false
}

func (a *App) guardPickerDiscard(all bool, action func()) {
	nav := a.profilePickerNavigation()
	if nav == nil {
		action()
		return
	}
	dirty := a.pickerPageDirty(nav.Page, a.captureNavigationState())
	if all {
		for _, page := range nav.Stack {
			dirty = dirty || a.pickerPageDirty(page.Page, page.State)
		}
	}
	if dirty {
		revision := nav.Revision
		a.ask(a.t("tip"), a.t("discardChanges"), func() {
			// A pending save may have already returned to a different page
			// in this same sheet while the confirmation was visible.
			if a.profilePickerNavigation() == nav && nav.Revision == revision {
				action()
			}
		})
		return
	}
	action()
}

func (a *App) requestPickerClose(next func()) {
	a.guardPickerDiscard(true, func() { a.dismissDashboardPicker(next) })
}

func (a *App) dismissDashboardPicker(next func()) {
	if a.deferSheetClose("dashboard-picker", func() { a.dismissDashboardPicker(next) }) {
		return
	}
	a.editorLoadSerial++
	if nav := a.picker.Navigation; nav != nil {
		a.restoreNavigationState(nav.Background)
		a.picker.Navigation = nil
	}
	a.picker.Open = false
	if next != nil {
		next()
	}
}

func (a *App) popProfilePickerPage() { a.popProfilePickerPageWithDraft(true) }

func (a *App) popProfilePickerPageWithDraft(keepDraft bool) {
	nav := a.profilePickerNavigation()
	if nav == nil {
		return
	}
	a.editorLoadSerial++
	nav.Revision++
	if n := len(nav.Stack); n > 0 {
		parent := nav.Stack[n-1]
		nav.Stack = nav.Stack[:n-1]
		draft := a.profileDraft
		a.restoreNavigationState(parent.State)
		// Only editor fields belong to this stack. The background may have
		// completed its own save while the nested page was active.
		a.page, a.history = nav.Background.Page, append([]string(nil), nav.Background.History...)
		a.recordQuery, a.recordFilter, a.recordSelected, a.paused = nav.Background.RecordQuery, nav.Background.RecordFilter, nav.Background.RecordSelected, nav.Background.Paused
		// A script/override saved by a child belongs to the parent's draft too.
		if keepDraft && parent.State.EditProfile == draft.ID {
			a.profileDraft = draft
		}
		nav.Page, nav.Scroll, nav.Busy = parent.Page, parent.Scroll, false
		if nav.Page == "profiles" {
			a.restoreNavigationState(nav.Background)
			a.picker.Navigation = nil
		}
		return
	}
	a.requestPickerClose(nil)
}

func (a *App) backProfilePicker() {
	nav := a.profilePickerNavigation()
	if nav == nil {
		a.closeDashboardPicker()
		return
	}
	if nav.Page == "collection" && a.document != nil && len(a.document.Path) > 0 {
		a.document.Path = a.document.Path[:len(a.document.Path)-1]
		a.document.List = ui.ListState{}
		return
	}
	a.guardPickerDiscard(false, func() { a.popProfilePickerPageWithDraft(false) })
}

func (a *App) openProfileAction(p store.Profile, page string) {
	if a.picker.Open && a.picker.Kind == "profiles" {
		nav := a.beginProfilePickerNavigation()
		nav.Initial = p
	}
	a.goPage(page)
	a.profileDraft, a.editProfile, a.profileTab = p, p.ID, 0
}

func (a *App) openCustomProfileSheet(p store.Profile) {
	if a.picker.Open {
		a.requestPickerClose(func() { a.openCustomProfileSheet(p) })
		return
	}
	a.resetSheet("dashboard-picker")
	a.picker = dashboardPickerState{Open: true, Kind: "profiles"}
	nav := a.beginProfilePickerNavigation()
	nav.Page, nav.Initial = "profileCustom", p
	a.profileDraft, a.editProfile = p, p.ID
}

func (a *App) profilePickerPageView(c *ui.Context) {
	nav := a.profilePickerNavigation()
	if nav == nil {
		return
	}
	title := a.profileDraft.Name
	switch nav.Page {
	case "editor":
		title = a.codeTitle
	case "collection":
		if a.document != nil {
			title = a.document.Title
		}
	case "profileExtend":
		title = a.t("extend")
	}
	a.sheetHeader(c, title, "back", a.t("back"), a.backProfilePicker, func() {
		switch nav.Page {
		case "editor":
			a.sheetButton(c, "search", a.t("search"), func() {
				if a.code != nil {
					// Find(query) only sets a query in pinned MyGo; the Find
					// command also transfers keyboard focus to the find input.
					a.code.Run(editor.Find)
					a.code.Find("")
				}
			})
			if a.code != nil && !a.code.ReadOnly {
				a.sheetButton(c, "check", a.t("save"), a.saveCode).Disabled(nav.Busy)
			}
		case "collection":
			a.sheetButton(c, "plus", a.t("add"), a.documentAdd)
			a.sheetButton(c, "check", a.t("save"), a.saveDocument).Disabled(a.document != nil && a.document.Saving)
		case "profileEdit", "profileExtend", "profileRename":
			a.sheetButton(c, "check", a.t("save"), a.saveProfile).Disabled(nav.Busy)
		case "profileCustom":
			a.sheetButton(c, "edit", a.t("rename"), func() { a.goPage("profileRename") })
			a.sheetButton(c, "eye", a.t("runtimeConfig"), func() { a.previewPickerProfile(a.profileDraft) })
		}
	})
	ui.Column(c.Key(fmt.Sprintf("%s/%d/%s", nav.Owner, len(nav.Stack), nav.Page))).Fill().Children(func() {
		if nav.Busy {
			ui.Box(c).Fill().Center().Children(func() { loading(c, 48, a.colors.accent, a.t("loading")) })
			return
		}
		switch nav.Page {
		case "profileEdit":
			a.profileSheetEditView(c)
		case "editor":
			if a.code != nil {
				ui.Box(c).Fill().Padding(0, 16, 16, 16).Children(func() { editor.View(c, a.code).Fill().Radius(20) })
				if !a.code.ReadOnly && c.Shortcut(ui.Cmd, ui.KeyS) {
					a.saveCode()
				}
			}
		case "collection":
			a.collectionView(c)
		default:
			a.profileActionView(c, nav.Page)
		}
	})
}

func (a *App) proxyViewState() *proxyPageState {
	if a.picker.Open && a.picker.Kind == "proxies" {
		return &a.picker.Proxy
	}
	return &a.proxyPageState
}

func (a *App) openProfilePicker() {
	if a.picker.Open {
		a.requestPickerClose(a.openProfilePicker)
		return
	}
	a.resetSheet("dashboard-picker")
	a.picker = dashboardPickerState{Open: true, Kind: "profiles"}
}

func (a *App) openProxyPicker(group string) {
	if a.picker.Open {
		a.requestPickerClose(func() { a.openProxyPicker(group) })
		return
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
	a.requestPickerClose(nil)
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
	if a.picker.Kind == "profiles" && a.picker.Navigation == nil {
		natural = 68 + 16 + float32(len(a.cfg.Profiles))*(a.textHeight(22)+4+a.textHeight(18)+20+8) - 8
	}
	a.navigationSheet(c, "dashboard-picker", &a.picker.Open, a.closeDashboardPicker, a.backProfilePicker, func() {
		if a.profilePickerNavigation() != nil {
			a.profilePickerPageView(c)
			return
		}
		if a.picker.Kind == "profiles" {
			a.sheetHeader(c, a.t("profiles"), "close", a.t("close"), a.closeDashboardPicker, nil)
			a.profilesGridView(c, &a.picker.Profiles, a.sideSheetWidth(c), true)
			return
		}
		ui.Box(c).Fill().Children(func() {
			ui.Column(c).Fill().Children(func() {
				if a.picker.Proxy.proxySearch && !a.compactSheet {
					ui.Row(c.Key("proxy-search")).FillWidth().Height(72).Padding(14, 16).Gap(8).Children(func() {
						ui.SearchField(c.Key("query"), &a.picker.Proxy.proxyQuery).Grow(1).MinWidth(0).AutoFocus().Label(a.t("search"))
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
