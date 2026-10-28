package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

const pickerSource = "# source comment\nproxies: []\nproxy-groups: [{name: Fixture, type: select, proxies: [DIRECT]}]\nrules: [MATCH,DIRECT]\nfuture-field: keep\n"

func pickerApp(t *testing.T) *App {
	t.Helper()
	a := populatedApp(t)
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte(pickerSource)); err != nil {
		t.Fatal(err)
	}
	return a
}

func pickerTester(a *App) *ui.Tester {
	tt := ui.NewTester(a.View, 1200, 800)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	return tt
}

func chooseProfileAction(t *testing.T, tt *ui.Tester, name string, path ...string) {
	t.Helper()
	if err := tt.RightClick(name); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(path...); err != nil {
		t.Fatal(err)
	}
}

func TestProfilePickerNestedActionsRetainDashboardAndGrid(t *testing.T) {
	a := pickerApp(t)
	for i := range 60 {
		a.cfg.Profiles = append(a.cfg.Profiles, store.Profile{ID: fmt.Sprint(i), Name: fmt.Sprintf("Local %02d", i)})
	}
	a.openProfilePicker()
	tt := pickerTester(a)
	// A real virtual-list offset must survive unmounting the picker page.
	a.picker.Profiles.List.ScrollIntoView(25)
	tt.Frame()
	before, ok := tt.Find("Local 24")
	if !ok {
		t.Fatal("selected scroll row is not visible")
	}
	chooseProfileAction(t, tt, "Local 24", a.t("edit"))
	if nav := a.profilePickerNavigation(); nav == nil || nav.Page != "profileEdit" || a.editProfile != "24" {
		t.Fatal("context edit did not push a sheet page")
	}
	if a.page != "dashboard" || len(a.history) != 0 || !tt.HasText(a.t("totalTraffic")) {
		t.Fatal("nested editor replaced the dashboard")
	}
	tt.Key(0, ui.KeyEscape)
	if !a.picker.Open || a.picker.Navigation != nil {
		t.Fatal("Escape dismissed the picker rather than popping its page")
	}
	after, ok := tt.Find("Local 24")
	if !ok || before != after {
		t.Fatal("picker scroll was lost", before, after)
	}
	chooseProfileAction(t, tt, "Local 24", a.t("extend"))
	if a.picker.Navigation.Page != "profileExtend" {
		t.Fatal("extend escaped the picker")
	}
	tt.Key(0, ui.KeyEscape)
	if !a.picker.Open || a.page != "dashboard" {
		t.Fatal("extend back dismissed the picker")
	}
}

func TestProfilePickerStagedSourceAndMetadataHaveOneOwner(t *testing.T) {
	a := pickerApp(t)
	p := a.cfg.Profiles[0]
	a.openProfilePicker()
	a.openProfileAction(p, "profileEdit")
	a.picker.Navigation.Scroll.Y = 81
	a.profileDraft.Name = "Renamed"
	a.readProfileCode(p.ID, "profile-staged")
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.picker.Navigation.Page != "editor" || a.page != "dashboard" {
		t.Fatal("file read opened the main editor")
	}
	a.code.Buffer.Set(pickerSource + "# staged\n")
	a.saveCode()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.picker.Navigation.Page != "profileEdit" || a.profileDraft.Name != "Renamed" || a.picker.Navigation.Scroll.Y != 81 {
		t.Fatal("return from the file editor lost its parent draft/scroll")
	}
	data, _ := os.ReadFile(a.store.ProfilePath(p.ID))
	if string(data) != pickerSource {
		t.Fatal("nested Save wrote the source before metadata confirmation")
	}
	a.readProfileCode(p.ID, "profile-staged")
	waitApp(t, a, func() bool { return a.working == 0 })
	if !strings.Contains(a.code.Buffer.String(), "# staged") {
		t.Fatal("reopening the file editor lost staged data")
	}
	a.backProfilePicker()
	// A concurrent refresh/selection must not be replaced by an old draft.
	a.cfg.Profiles[0].Download = 12345
	a.cfg.Profiles[0].Selected = map[string]string{"Fixture": "DIRECT"}
	a.saveProfile()
	waitApp(t, a, func() bool { return a.working == 0 })
	data, _ = os.ReadFile(a.store.ProfilePath(p.ID))
	if !strings.Contains(string(data), "# staged") || a.cfg.Profiles[0].Name != "Renamed" || a.cfg.Profiles[0].Download != 12345 || a.cfg.Profiles[0].Selected["Fixture"] != "DIRECT" {
		t.Fatal("metadata save lost the source or latest subscription/selection state")
	}
	if !a.picker.Open || a.picker.Navigation != nil || a.page != "dashboard" {
		t.Fatal("metadata confirmation did not return to the same picker")
	}
}

func TestProfilePickerDirtyBufferGuardsBackBackdropAndTabs(t *testing.T) {
	a := pickerApp(t)
	a.frameEnabled = true
	a.selectNavigation("tools")
	a.openCode("Existing script", "library-script:fixture", "old main buffer")
	a.code.Buffer.Set("dirty main buffer")
	mainCode := a.code
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
	a.readProfileCode(a.editProfile, "profile-staged")
	waitApp(t, a, func() bool { return a.working == 0 })
	sheetCode := a.code
	sheetCode.Buffer.Set(pickerSource + "# unsaved sheet\n")
	for _, action := range []func(){a.backProfilePicker, a.closeDashboardPicker, func() { a.selectNavigation("profiles") }} {
		action()
		if !a.confirmOpen || !a.picker.Open || a.code != sheetCode || !sheetCode.Buffer.Modified() || a.page != "editor" {
			t.Fatal("dirty sheet was discarded or a tab consumed its buffer")
		}
		a.confirmOpen = false // Cancel the global discard dialog.
	}
	a.closeDashboardPicker()
	a.confirmOpen = false
	a.confirm() // Explicit discard releases only the sheet's buffer.
	if a.picker.Open || a.code != mainCode || !mainCode.Buffer.Modified() || a.code.Buffer.String() != "dirty main buffer" {
		t.Fatal("closing a sheet overwrote the main editor")
	}
}

func TestProfilePickerDirtyCollectionRetainsParentAndNestedCode(t *testing.T) {
	a := pickerApp(t)
	p := a.cfg.Profiles[0]
	p.Type, p.URL = "custom", ""
	a.cfg.Profiles[0] = p
	a.openCustomProfileSheet(p)
	a.editProfileSection(p.ID, "proxy-groups")
	waitApp(t, a, func() bool { return a.working == 0 })
	d := a.document
	d.push(0)
	d.replace(map[string]any{"name": "Edited", "type": "select", "proxies": []any{"DIRECT"}})
	text, _ := store.YAML(d.Value)
	a.openCode(d.Title, "document", text)
	a.code.Buffer.Set(text + "# code edit\n")
	a.saveCode()
	if a.picker.Navigation.Page != "collection" || a.document != d || len(d.Path) != 1 {
		t.Fatal("code save lost its owned collection/path")
	}
	a.backProfilePicker() // Collection breadcrumb first, not the sheet stack.
	if len(d.Path) != 0 || a.confirmOpen {
		t.Fatal("collection breadcrumb did not pop independently")
	}
	a.backProfilePicker()
	if !a.confirmOpen || a.document != d {
		t.Fatal("dirty collection silently disappeared")
	}
	a.confirmOpen = false
	a.saveDocument()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.picker.Navigation.Page != "profileCustom" || a.page != "dashboard" {
		t.Fatal("collection save escaped its custom sheet")
	}
	data, _ := os.ReadFile(a.store.ProfilePath(p.ID))
	if !strings.Contains(string(data), "Edited") || !strings.Contains(string(data), "# source comment") || !strings.Contains(string(data), "future-field: keep") {
		t.Fatal("structured edit lost source data", string(data))
	}
}

func TestCustomDashboardProfileOpensCustomSheetAndRuntime(t *testing.T) {
	a := pickerApp(t)
	a.cfg.Profiles[0].Type, a.cfg.Profiles[0].URL = "custom", ""
	// Both wide and narrow cards already delegate to this entry point.
	a.openProfileDetails()
	tt := pickerTester(a)
	if a.profileDetail.Open || !a.picker.Open || a.picker.Navigation.Page != "profileCustom" || a.page != "dashboard" {
		t.Fatal("custom profile opened ordinary details")
	}
	if err := tt.Click(a.t("runtimeConfig")); err != nil {
		t.Fatal(err)
	}
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.code == nil || !a.code.ReadOnly || !strings.Contains(a.code.Buffer.String(), "future-field: keep") {
		t.Fatal("custom runtime preview did not use the effective-config pipeline")
	}
	a.backProfilePicker()
	if a.picker.Navigation.Page != "profileCustom" || a.page != "dashboard" {
		t.Fatal("preview back lost custom root")
	}
}

func TestProfilePickerPreviewUsesClickedProfileNotActive(t *testing.T) {
	a := pickerApp(t)
	p := store.Profile{ID: store.NewID(), Name: "Other", Script: "const main = c => { c['preview-owner'] = 'other'; return c; };"}
	a.cfg.Profiles = append(a.cfg.Profiles, p)
	if err := store.AtomicWrite(a.store.ProfilePath(p.ID), []byte("proxies: []\nrules: [MATCH,DIRECT]\n")); err != nil {
		t.Fatal(err)
	}
	active := a.cfg.Active
	a.openProfilePicker()
	tt := pickerTester(a)
	chooseProfileAction(t, tt, p.Name, a.t("more"), a.t("runtimeConfig"))
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.cfg.Active != active || !a.code.ReadOnly || !strings.Contains(a.code.Buffer.String(), "preview-owner: other") || a.page != "dashboard" {
		t.Fatal("preview activated or previewed the wrong profile")
	}
	a.backProfilePicker()
	if a.picker.Navigation != nil || !a.picker.Open {
		t.Fatal("preview back did not reveal the picker")
	}
}

func TestProfilePickerLateLoadsCannotEnterReplacementSheet(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			a := pickerApp(t)
			a.openProfilePicker()
			a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
			release := make(chan struct{})
			a.loadProfileEditor(a.editProfile, func() (any, error) {
				<-release
				if fail {
					return nil, errors.New("obsolete read error")
				}
				return "obsolete text", nil
			}, func(value any) { a.openCode("Obsolete", "raw", value.(string)) })
			a.backProfilePicker()
			a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
			owner := a.editorOwner()
			close(release)
			waitApp(t, a, func() bool { return a.working == 0 })
			if a.editorOwner() != owner || a.picker.Navigation.Page != "profileExtend" || a.err != "" || a.code != nil {
				t.Fatal("late file load/error reached a replacement route")
			}
		})
	}
}

func TestProfilePickerAsyncSaveKeepsNewInputAndFailedBuffer(t *testing.T) {
	a := pickerApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
	a.readProfileCode(a.editProfile, "profile-staged")
	waitApp(t, a, func() bool { return a.working == 0 })
	a.code.Buffer.Set(pickerSource + "# captured\n")
	a.saveCode()
	a.code.Buffer.Set(pickerSource + "# newer\n")
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.picker.Navigation.Page != "editor" || !a.code.Buffer.Modified() || !strings.Contains(a.code.Buffer.String(), "# newer") || !strings.Contains(*a.picker.Navigation.Source, "# captured") {
		t.Fatal("staging completion discarded newer input")
	}
	a.code.Buffer.Set("[invalid YAML")
	a.saveCode()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.err == "" || a.picker.Navigation.Page != "editor" || a.code.Buffer.String() != "[invalid YAML" || !a.code.Buffer.Modified() {
		t.Fatal("invalid staged edit lost its buffer")
	}
	data, _ := os.ReadFile(a.store.ProfilePath(a.editProfile))
	if string(data) != pickerSource {
		t.Fatal("failed editor save replaced source")
	}
}

func TestProfilePickerAsyncCollectionSaveCannotPopReplacement(t *testing.T) {
	a := pickerApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileCustom")
	a.editProfileSection(a.editProfile, "rules")
	waitApp(t, a, func() bool { return a.working == 0 })
	a.document.Value = []any{"MATCH,REJECT"}
	a.saveDocument()
	// Release this owner explicitly before the queued completion is committed.
	a.dismissDashboardPicker(nil)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
	owner := a.editorOwner()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.editorOwner() != owner || a.picker.Navigation.Page != "profileExtend" || a.page != "dashboard" {
		t.Fatal("old collection save popped a replacement route")
	}
}

func TestProfilePickerRenameDiscardDoesNotLeakIntoCustomParent(t *testing.T) {
	a := pickerApp(t)
	p := a.cfg.Profiles[0]
	p.Type, p.URL = "custom", ""
	a.cfg.Profiles[0] = p
	a.openCustomProfileSheet(p)
	a.goPage("profileRename")
	a.profileDraft.Name = "Discarded rename"
	a.backProfilePicker()
	if !a.confirmOpen {
		t.Fatal("dirty rename was not guarded")
	}
	a.confirmOpen = false
	a.confirm()
	if a.picker.Navigation.Page != "profileCustom" || a.profileDraft.Name != p.Name || a.cfg.Profiles[0].Name != p.Name {
		t.Fatal("discarding a rename carried its draft into the parent")
	}
}

func TestProfilePickerStagingCannotReplaceNewerRouteBuffer(t *testing.T) {
	a := pickerApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
	a.openCode("Source", "profile-staged", pickerSource)
	a.code.Buffer.Set(pickerSource + "# obsolete staged save\n")
	a.saveCode()
	a.backProfilePicker()
	if !a.confirmOpen {
		t.Fatal("dirty pending save was not guarded")
	}
	a.confirmOpen = false
	a.confirm()
	a.openCode("Source", "profile-staged", pickerSource)
	a.code.Buffer.Set(pickerSource + "# newer staged save\n")
	a.saveCode()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.picker.Navigation.Page != "profileEdit" || a.picker.Navigation.Source == nil || !strings.Contains(*a.picker.Navigation.Source, "# newer staged save") || strings.Contains(*a.picker.Navigation.Source, "obsolete") {
		t.Fatal("completion from a discarded route overwrote the newer source")
	}
}

func TestProfilePickerSaveShortcutTargetsOnlyForegroundEditor(t *testing.T) {
	a := pickerApp(t)
	a.editProfile = a.cfg.Active
	a.openCode("Main", "raw", pickerSource)
	a.code.Buffer.Set(pickerSource + "# unsaved main\n")
	mainCode := a.code
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
	a.openCode("Sheet", "profile-staged", pickerSource)
	a.code.Buffer.Set(pickerSource + "# sheet save\n")
	tt := pickerTester(a)
	tt.Key(ui.Cmd, ui.KeyS)
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.picker.Navigation.Source == nil || !strings.Contains(*a.picker.Navigation.Source, "# sheet save") || !mainCode.Buffer.Modified() {
		t.Fatal("the hidden main editor consumed the sheet's save shortcut")
	}
	data, _ := os.ReadFile(a.store.ProfilePath(a.cfg.Active))
	if string(data) != pickerSource {
		t.Fatal("sheet shortcut saved the background source")
	}
}

func TestProfilePickerBackgroundSaveRestoresOnlyItsMainEditor(t *testing.T) {
	a := pickerApp(t)
	a.frameEnabled = true
	a.selectNavigation("profiles")
	a.editProfile = a.cfg.Active
	a.openCode("Main profile", "raw", pickerSource)
	a.code.Buffer.Set(pickerSource + "# main save\n")
	a.saveCode()
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
	a.readProfileCode(a.editProfile, "profile-staged")
	waitApp(t, a, func() bool { return a.working == 0 })
	sheetCode := a.code
	sheetCode.Buffer.Set(pickerSource + "# sheet buffer\n")
	if a.picker.Navigation.Background.Page != "profiles" || a.picker.Navigation.Page != "editor" || !sheetCode.Buffer.Modified() {
		t.Fatal("main save completion navigated the sheet")
	}
	a.closeDashboardPicker()
	a.confirmOpen = false
	a.confirm()
	if a.page != "profiles" || a.code == sheetCode {
		t.Fatal("sheet dismissal did not restore the saved main route")
	}
}
