package app

import (
	"slices"
	"strings"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type navigationPageState struct {
	Page                                 string
	History                              []string
	RecordQuery, RecordFilter            string
	RecordSelected                       int
	Paused                               bool
	Code                                 *editor.State
	CodeProvider                         *providerEditorContext
	CodePurpose, CodeTitle, CodeOriginal string
	EditProfile                          string
	ProfileTab                           int
	ProfileDraft                         store.Profile
	Document                             *documentEditor
	FontPicker                           *fontPickerState
}

func (a *App) captureNavigationState() navigationPageState {
	return navigationPageState{Page: a.page, History: slices.Clone(a.history), RecordQuery: a.recordQuery, RecordFilter: a.recordFilter, RecordSelected: a.recordSelected, Paused: a.paused, Code: a.code, CodeProvider: a.codeProvider, CodePurpose: a.codePurpose, CodeTitle: a.codeTitle, CodeOriginal: a.codeOriginal, EditProfile: a.editProfile, ProfileTab: a.profileTab, ProfileDraft: a.profileDraft, Document: a.document, FontPicker: a.fontPicker}
}

func (a *App) restoreNavigationState(state navigationPageState) {
	a.page, a.history = state.Page, slices.Clone(state.History)
	a.recordQuery, a.recordFilter, a.recordSelected, a.paused = state.RecordQuery, state.RecordFilter, state.RecordSelected, state.Paused
	a.code, a.codePurpose, a.codeTitle, a.codeOriginal = state.Code, state.CodePurpose, state.CodeTitle, state.CodeOriginal
	a.codeProvider = state.CodeProvider
	a.fontPicker = state.FontPicker
	a.editProfile, a.profileTab, a.profileDraft, a.document = state.EditProfile, state.ProfileTab, state.ProfileDraft, state.Document
}

func navigationPrimary(page string) bool {
	return slices.Contains([]string{"dashboard", "proxies", "profiles", "connections", "logs", "tools"}, page)
}

func (a *App) smallDesktop(c *ui.Context) bool {
	w, _ := c.Size()
	return a.frameEnabled && w <= 600
}

func (a *App) hasBottomNavigation(c *ui.Context) bool {
	return a.smallDesktop(c) && slices.Contains(a.navigationPages(true), a.page)
}

func (a *App) navigationPages(mobile bool) []string {
	pages := []string{"dashboard"}
	if !a.frameEnabled || len(a.cfg.Profiles) > 0 || len(a.sample.Proxies.All) > 0 {
		pages = append(pages, "proxies")
	}
	pages = append(pages, "profiles")
	if !mobile {
		pages = append(pages, "connections")
		if a.cfg.OpenLogs {
			pages = append(pages, "logs")
		}
	}
	return append(pages, "tools")
}

func (a *App) navigationLabel(page string) string {
	if page == "connections" {
		return a.t("liveConnections")
	}
	return a.t(page)
}

func (a *App) navigationParent() string {
	primary := []string{"dashboard", "proxies", "profiles", "connections", "logs", "tools"}
	if slices.Contains(primary, a.page) {
		return a.page
	}
	if a.activeNavigation != "" {
		return a.activeNavigation
	}
	for i := len(a.history) - 1; i >= 0; i-- {
		if slices.Contains(primary, a.history[i]) {
			return a.history[i]
		}
	}
	if a.page == "profileEdit" || a.page == "editor" && strings.HasPrefix(a.codePurpose, "profile:") {
		return "profiles"
	}
	return "tools"
}

func (a *App) selectNavigation(page string) {
	if a.picker.Open && !a.picker.BackgroundBuild {
		a.requestPickerClose(func() { a.selectNavigation(page) })
		return
	}
	current := a.navigationParent()
	if current == page {
		return
	}
	a.editorLoadSerial++
	if a.frameEnabled {
		if a.navigationState == nil {
			a.navigationState = map[string]navigationPageState{}
		}
		a.navigationState[current] = a.captureNavigationState()
		if prior, ok := a.navigationState[page]; ok {
			a.restoreNavigationState(prior)
			a.activeNavigation = page
			return
		}
	}
	a.goPage(page)
	a.history = nil
	a.activeNavigation = page
}
