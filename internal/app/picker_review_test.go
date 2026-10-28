package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestPickerReviewSearchTakesFocusWithoutSecondClick(t *testing.T) {
	for _, style := range []string{"list", "tab"} {
		t.Run(style, func(t *testing.T) {
			a := populatedApp(t)
			a.cfg.ProxyStyle = style
			a.openProxyPicker("Streaming")
			tt := ui.NewTester(a.View, 1200, 800)
			tt.Advance(time.Second)
			if err := tt.Click(a.t("search")); err != nil {
				t.Fatal(err)
			}
			tt.Type("Japan")
			if a.picker.Proxy.proxyQuery != "Japan" {
				t.Fatal("Search did not automatically focus", a.picker.Proxy.proxyQuery)
			}
		})
	}
}

func TestPickerReviewAnimatedSheetInitialAutoFocus(t *testing.T) {
	for _, width := range []int{560, 1200} {
		t.Run(number(width), func(t *testing.T) {
			a := testApp(t)
			open, query := true, ""
			tt := ui.NewTester(func(c *ui.Context) {
				a.theme(c)
				a.navigationSheet(c, "review-focus", &open, func() { open = false }, func() { open = false }, func() {
					ui.Box(c).Height(68)
					ui.SearchField(c, &query).AutoFocus().Label("Search")
				})
			}, width, 800)
			tt.Advance(time.Second)
			tt.Type("first query")
			if query != "first query" {
				t.Fatal("animated sheet lost its initial AutoFocus", query)
			}
		})
	}
}

func TestPickerReviewFindFocusAndEscapeStayWithinAnimatedSheet(t *testing.T) {
	for _, width := range []int{560, 1200} {
		for _, preview := range []bool{false, true} {
			t.Run(number(width)+map[bool]string{false: "/edit", true: "/preview"}[preview], func(t *testing.T) {
				a := pickerApp(t)
				a.openProfilePicker()
				a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
				if preview {
					a.previewPickerProfile(a.cfg.Profiles[0])
				} else {
					a.readProfileCode(a.editProfile, "profile-staged")
				}
				waitApp(t, a, func() bool { return a.working == 0 })
				tt := ui.NewTester(a.View, width, 800)
				tt.Advance(time.Second)
				code, owner := a.code, a.editorOwner()
				if err := tt.Click(a.t("search")); err != nil {
					t.Fatal(err)
				}
				tt.Type("future-field")
				if count, _ := code.FindMatches(); count != 1 {
					t.Fatal("toolbar Find did not focus its input", count, code.Buffer.String(), tt.Texts())
				}
				escapes := 1
				if preview {
					escapes = 2
				}
				for range escapes {
					tt.Key(0, ui.KeyEscape)
				}
				if a.editorOwner() != owner || a.code != code || a.picker.Navigation.Page != "editor" || !a.picker.Open {
					t.Fatal("Escape cleared/popped the sheet before closing its find UI")
				}
				// MyGo's editor consumes Escape to collapse the selected match
				// before an unhandled Escape can navigate the sheet.
				if selection := code.Selection(); selection.Anchor != selection.Head {
					tt.Key(0, ui.KeyEscape)
					if a.picker.Navigation.Page != "editor" {
						t.Fatal("selection Escape navigated the sheet")
					}
				}
				tt.Key(0, ui.KeyEscape)
				if a.picker.Navigation == nil || a.picker.Navigation.Page != "profileEdit" || !a.picker.Open || a.confirmOpen {
					t.Fatal("Escape did not pop the editor after closing find", a.picker.Navigation.Page, a.confirmOpen, a.picker.Open, tt.Texts())
				}
			})
		}
	}
}

func TestPickerReviewChildSaveDoesNotCommitParentMetadata(t *testing.T) {
	a := pickerApp(t)
	p := a.cfg.Profiles[0]
	a.openProfilePicker()
	a.openProfileAction(p, "profileExtend")
	a.profileDraft.Name = "unsaved name"
	a.openCollection("Override", "profile-override", map[string]any{})
	a.document.Value = map[string]any{"mixed-port": 9876}
	a.saveDocument()
	if a.cfg.Profiles[0].Name != p.Name || a.cfg.Profiles[0].Override != p.Override {
		t.Fatal("child save committed the unconfirmed parent draft")
	}
	if a.picker.Navigation.Page != "profileExtend" || !strings.Contains(a.profileDraft.Override, "9876") {
		t.Fatal("child save did not return its value to the parent")
	}
	a.backProfilePicker()
	if !a.confirmOpen {
		t.Fatal("parent with staged child edits is not dirty")
	}
}

func TestPickerReviewSectionUsesStagedSource(t *testing.T) {
	a := pickerApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
	a.openCode("Source", "profile-staged", pickerSource)
	a.code.Buffer.Set(strings.ReplaceAll(pickerSource, "Fixture", "Staged group"))
	a.saveCode()
	waitApp(t, a, func() bool { return a.working == 0 })
	a.editProfileSection(a.editProfile, "proxy-groups")
	waitApp(t, a, func() bool { return a.working == 0 })
	text, _ := store.YAML(a.document.Value)
	if !strings.Contains(text, "Staged group") {
		t.Fatal("section reopened stale on-disk content", text)
	}
	a.document.Value = []any{map[string]any{"name": "Structured staged", "type": "select", "proxies": []any{"DIRECT"}}}
	a.saveDocument()
	waitApp(t, a, func() bool { return a.working == 0 })
	data, _ := os.ReadFile(a.store.ProfilePath(a.editProfile))
	if string(data) != pickerSource {
		t.Fatal("section save bypassed parent confirmation", string(data))
	}
	if a.picker.Navigation.Source == nil || !strings.Contains(*a.picker.Navigation.Source, "Structured staged") {
		t.Fatal("section save did not patch the staged source")
	}
}

func TestPickerReviewMetadataSaveKeepsNewerInput(t *testing.T) {
	a := pickerApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
	a.profileDraft.Name = "captured name"
	text := pickerSource + "# staged\n"
	a.picker.Navigation.Source = &text
	a.saveProfile()
	a.profileDraft.Name = "newer name"
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.picker.Navigation == nil || a.picker.Navigation.Page != "profileEdit" || a.profileDraft.Name != "newer name" {
		t.Fatal("metadata save discarded input typed during validation/write")
	}
	if a.cfg.Profiles[0].Name != "captured name" {
		t.Fatal("save did not use its captured draft")
	}
	a.backProfilePicker()
	if !a.confirmOpen {
		t.Fatal("newer metadata was not dirty after the captured save")
	}
}

func TestPickerReviewFailedPreferencesSavePreservesDraftAndSource(t *testing.T) {
	a := pickerApp(t)
	p := a.cfg.Profiles[0]
	a.openProfilePicker()
	a.openProfileAction(p, "profileEdit")
	a.readProfileCode(p.ID, "profile-staged")
	waitApp(t, a, func() bool { return a.working == 0 })
	a.code.Buffer.Set(pickerSource + "# pending source\n")
	a.saveCode()
	waitApp(t, a, func() bool { return a.working == 0 })
	a.profileDraft.Name = "pending name"
	settings := filepath.Join(a.store.Dir, "settings.json")
	if err := os.Mkdir(settings, 0700); err != nil {
		t.Fatal(err)
	}
	a.saveProfile()
	waitApp(t, a, func() bool { return a.working == 0 })
	data, _ := os.ReadFile(a.store.ProfilePath(p.ID))
	if a.err == "" || a.picker.Navigation == nil || a.picker.Navigation.Source == nil || a.profileDraft.Name != "pending name" || a.cfg.Profiles[0].Name != p.Name || string(data) != pickerSource {
		t.Fatal("failed preference persistence committed or discarded staged work", a.err, string(data))
	}
	if err := os.Remove(settings); err != nil {
		t.Fatal(err)
	}
	a.err = ""
	a.saveProfile()
	waitApp(t, a, func() bool { return a.working == 0 })
	data, _ = os.ReadFile(a.store.ProfilePath(p.ID))
	if a.err != "" || a.picker.Navigation != nil || a.cfg.Profiles[0].Name != "pending name" || !strings.Contains(string(data), "pending source") {
		t.Fatal("failed save could not be retried", a.err)
	}
}

func TestPickerReviewConcurrentSourceChangeRejectsStagedOverwrite(t *testing.T) {
	a := pickerApp(t)
	p := a.cfg.Profiles[0]
	a.openProfilePicker()
	a.openProfileAction(p, "profileEdit")
	a.readProfileCode(p.ID, "profile-staged")
	waitApp(t, a, func() bool { return a.working == 0 })
	a.code.Buffer.Set(pickerSource + "# staged\n")
	a.saveCode()
	waitApp(t, a, func() bool { return a.working == 0 })
	a.profileDraft.Name = "uncommitted metadata"
	latest := pickerSource + "# subscription refresh\n"
	if err := store.AtomicWrite(a.store.ProfilePath(p.ID), []byte(latest)); err != nil {
		t.Fatal(err)
	}
	a.saveProfile()
	waitApp(t, a, func() bool { return a.working == 0 })
	data, _ := os.ReadFile(a.store.ProfilePath(p.ID))
	if a.err == "" || string(data) != latest || a.cfg.Profiles[0].Name != p.Name || a.picker.Navigation.Source == nil || !a.picker.Navigation.sourceDirty() {
		t.Fatal("concurrent source was overwritten or the rejected draft was lost", a.err, string(data))
	}
}

func TestPickerReviewConcurrentSourceChangeKeepsMainRawBuffer(t *testing.T) {
	a := pickerApp(t)
	p := a.cfg.Profiles[0]
	a.openProfileAction(p, "profileEdit")
	a.readProfileCode(p.ID, "profile")
	waitApp(t, a, func() bool { return a.working == 0 })
	a.code.Buffer.Set(pickerSource + "# unsaved raw edit\n")
	code := a.code
	latest := pickerSource + "# downloaded update\n"
	if err := store.AtomicWrite(a.store.ProfilePath(p.ID), []byte(latest)); err != nil {
		t.Fatal(err)
	}
	a.saveCode()
	waitApp(t, a, func() bool { return a.working == 0 })
	data, _ := os.ReadFile(a.store.ProfilePath(p.ID))
	if a.err == "" || string(data) != latest || a.code != code || !code.Buffer.Modified() || a.page != "editor" {
		t.Fatal("raw save overwrote concurrent source or lost its dirty buffer", a.err)
	}
}

func TestPickerReviewConcurrentSectionChangeKeepsDirtyCollection(t *testing.T) {
	for _, sheet := range []bool{false, true} {
		t.Run(map[bool]string{false: "main", true: "sheet"}[sheet], func(t *testing.T) {
			a := pickerApp(t)
			p := a.cfg.Profiles[0]
			if sheet {
				a.openCustomProfileSheet(p)
			} else {
				a.openProfileAction(p, "profileCustom")
			}
			a.editProfileSection(p.ID, "rules")
			waitApp(t, a, func() bool { return a.working == 0 })
			d := a.document
			d.Value = []any{"MATCH,REJECT"}
			latest := pickerSource + "# other editor\n"
			if err := store.AtomicWrite(a.store.ProfilePath(p.ID), []byte(latest)); err != nil {
				t.Fatal(err)
			}
			a.saveDocument()
			waitApp(t, a, func() bool { return a.working == 0 })
			data, _ := os.ReadFile(a.store.ProfilePath(p.ID))
			if a.err == "" || string(data) != latest || a.document != d || d.Saving || strings.Contains(d.Original, "REJECT") {
				t.Fatal("rejected collection save lost context or overwrote concurrent source", a.err)
			}
		})
	}
}

func TestPickerReviewClearedSubscriptionURLRemainsEditableAndUnsaved(t *testing.T) {
	a := pickerApp(t)
	p := a.cfg.Profiles[0]
	a.openProfilePicker()
	a.openProfileAction(p, "profileEdit")
	tt := pickerTester(a)
	if err := tt.Click(a.t("url")); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	if a.profileDraft.URL != "" {
		t.Fatal("fixture did not clear URL")
	}
	if _, ok := tt.Find(a.t("url")); !ok {
		t.Fatal("clearing URL removed its input")
	}
	a.saveProfile()
	if a.err != a.t("profileUrlNullValidationDesc") || a.cfg.Profiles[0].URL != p.URL || a.picker.Navigation == nil {
		t.Fatal("invalid subscription metadata was saved or discarded", a.err)
	}
	tt.Type("https://example.com/replacement")
	if a.profileDraft.URL != "https://example.com/replacement" {
		t.Fatal("URL could not be corrected without reopening the editor")
	}
}

func TestPickerReviewMetadataSaveMergesOnlyChangedFields(t *testing.T) {
	a := pickerApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileExtend")
	a.profileDraft.Script = "const main = c => c;"
	a.cfg.Profiles[0].Name = "concurrent rename"
	a.cfg.Profiles[0].URL = "https://example.com/new-subscription"
	a.cfg.Profiles[0].Override = "keep-alive-interval: 30\n"
	a.saveProfile()
	p := a.cfg.Profiles[0]
	if p.Name != "concurrent rename" || p.URL != "https://example.com/new-subscription" || p.Override != "keep-alive-interval: 30\n" || p.Script != "const main = c => c;" {
		t.Fatal("saving one field replaced unrelated concurrent metadata", p)
	}
}

func TestPickerReviewUnchangedStagingIsNotDirty(t *testing.T) {
	a := pickerApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
	a.readProfileCode(a.editProfile, "profile-staged")
	waitApp(t, a, func() bool { return a.working == 0 })
	a.saveCode()
	waitApp(t, a, func() bool { return a.working == 0 })
	a.backProfilePicker()
	if a.confirmOpen || a.picker.Navigation != nil || !a.picker.Open {
		t.Fatal("no-op staging left a dirty parent")
	}
}

func TestPickerReviewStaleDiscardCannotPopSavedParent(t *testing.T) {
	a := pickerApp(t)
	a.openProfilePicker()
	a.openProfileAction(a.cfg.Profiles[0], "profileEdit")
	a.readProfileCode(a.editProfile, "profile-staged")
	waitApp(t, a, func() bool { return a.working == 0 })
	a.code.Buffer.Set(pickerSource + "# save while confirming\n")
	a.saveCode()
	a.backProfilePicker()
	if !a.confirmOpen {
		t.Fatal("pending editor save did not guard discard")
	}
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.picker.Navigation.Page != "profileEdit" {
		t.Fatal("save did not return to metadata")
	}
	a.confirmOpen = false
	a.confirm()
	if a.picker.Navigation == nil || a.picker.Navigation.Page != "profileEdit" || !a.picker.Navigation.sourceDirty() {
		t.Fatal("confirmation for the old editor discarded the saved parent")
	}
}

func TestPickerReviewCollectionCannotSaveUnderDifferentProfile(t *testing.T) {
	a := pickerApp(t)
	a.openCustomProfileSheet(a.cfg.Profiles[0])
	a.editProfileSection(a.editProfile, "rules")
	waitApp(t, a, func() bool { return a.working == 0 })
	a.document.Value = []any{"MATCH,REJECT"}
	a.editProfile = store.NewID()
	a.saveDocument()
	if a.err != a.t("editorContextChanged") || a.working != 0 {
		t.Fatal("collection save ignored its loaded profile identity", a.err)
	}
}
