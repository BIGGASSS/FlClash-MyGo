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
	current := a.navigationParent()
	if current == page {
		return
	}
	a.editorLoadSerial++
	if a.frameEnabled {
		if a.navigationState == nil {
			a.navigationState = map[string]navigationPageState{}
		}
		a.navigationState[current] = navigationPageState{Page: a.page, History: slices.Clone(a.history), RecordQuery: a.recordQuery, RecordFilter: a.recordFilter, RecordSelected: a.recordSelected, Paused: a.paused, Code: a.code, CodeProvider: a.codeProvider, CodePurpose: a.codePurpose, CodeTitle: a.codeTitle, CodeOriginal: a.codeOriginal, EditProfile: a.editProfile, ProfileTab: a.profileTab, ProfileDraft: a.profileDraft, Document: a.document}
		if prior, ok := a.navigationState[page]; ok {
			a.page, a.history = prior.Page, slices.Clone(prior.History)
			a.recordQuery, a.recordFilter, a.recordSelected, a.paused = prior.RecordQuery, prior.RecordFilter, prior.RecordSelected, prior.Paused
			a.code, a.codePurpose, a.codeTitle, a.codeOriginal = prior.Code, prior.CodePurpose, prior.CodeTitle, prior.CodeOriginal
			a.codeProvider = prior.CodeProvider
			a.editProfile, a.profileTab, a.profileDraft, a.document = prior.EditProfile, prior.ProfileTab, prior.ProfileDraft, prior.Document
			a.activeNavigation = page
			return
		}
	}
	a.goPage(page)
	a.history = nil
	a.activeNavigation = page
}
