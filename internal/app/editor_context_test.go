package app

import (
	"os"
	"strings"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

func TestBackgroundSaveBelongsToItsOriginalTabAndBuffer(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled = true
	a.selectNavigation("profiles")
	a.editProfile = a.cfg.Active
	a.openCode("Profile", "raw", "proxies: []\n")
	profileCode := a.code
	a.saveCode()
	a.selectNavigation("tools")
	a.openCode("Script", "library-script:fixture", "original script")
	a.code.Buffer.Set("dirty script")
	scriptCode := a.code
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.page != "editor" || a.code != scriptCode || !scriptCode.Buffer.Modified() || a.codeOriginal != "original script" {
		t.Fatal("profile save completed against another tab's editor")
	}
	if profileCode.Buffer.Modified() {
		t.Fatal("saved profile buffer remained dirty")
	}
	a.selectNavigation("profiles")
	if a.page != "profiles" {
		t.Fatal("saved inactive editor did not return within its own tab", a.page)
	}
	data, err := os.ReadFile(a.store.ProfilePath(a.cfg.Active))
	if err != nil || string(data) != "proxies: []\n" {
		t.Fatal("profile save lost its target", string(data), err)
	}
}

func TestEditsMadeDuringSaveRemainDirtyAndOpen(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled = true
	a.selectNavigation("profiles")
	a.editProfile = a.cfg.Active
	a.openCode("Profile", "raw", "proxies: []\n")
	a.code.Buffer.Set("proxies: []\n# saved\n")
	a.saveCode()
	a.code.Buffer.Set("proxies: []\n# newer edit\n")
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.page != "editor" || !a.code.Buffer.Modified() || !strings.Contains(a.code.Buffer.String(), "newer edit") {
		t.Fatal("save completion discarded newer input")
	}
	data, err := os.ReadFile(a.store.ProfilePath(a.cfg.Active))
	if err != nil || !strings.Contains(string(data), "# saved") || strings.Contains(string(data), "newer edit") {
		t.Fatal("save did not keep its captured document", string(data), err)
	}
}

func TestProfileReadCannotOpenEditorAfterTabSwitch(t *testing.T) {
	a := populatedApp(t)
	a.frameEnabled = true
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte("proxies: []\n")); err != nil {
		t.Fatal(err)
	}
	a.selectNavigation("profiles")
	a.editProfile, a.profileDraft = a.cfg.Active, a.cfg.Profiles[0]
	a.goPage("profileEdit")
	a.readProfileCode(a.cfg.Active, "raw")
	a.selectNavigation("tools")
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.page != "tools" || a.code != nil {
		t.Fatal("late file read opened an editor in another tab")
	}
}

func TestProviderEditorCannotWriteThroughChangedConfig(t *testing.T) {
	for _, change := range []string{"profile", "revision", "engine"} {
		a := populatedApp(t)
		a.engine = &Engine{}
		a.openCode("Provider", "provider:fixture", "payload: [example.com]")
		switch change {
		case "profile":
			a.cfg.Active = "another-profile"
		case "revision":
			a.engine.revision++
		case "engine":
			a.engine = &Engine{}
		}
		a.saveCode()
		if a.err != a.t("editorContextChanged") || a.working != 0 {
			t.Fatal("provider save ignored changed owner", change, a.err)
		}
	}
}
