package app

import (
	"strings"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestHotkeyConflictTransfersAndUnmodifiedKeyIsRejected(t *testing.T) {
	a := testApp(t)
	a.cfg.Hotkeys["view"] = "Control+Shift+P"
	a.recordHotkey("start")
	tt := ui.NewTester(a.View, 1200, 800)
	tt.Key(0, ui.KeyP)
	if validHotkey(a.hotkeyDraft) || !tt.HasText(strings.ReplaceAll(a.t("hotkeyNeedsModifier"), "{modifiers}", "Ctrl, Alt, Shift, Super")) {
		t.Fatal("unmodified key accepted")
	}
	tt.Key(ui.Shift|ui.Ctrl, ui.KeyP)
	if a.hotkeyConflict() != "view" {
		t.Fatal("equivalent modifier aliases did not conflict", a.hotkeyDraft)
	}
	if err := tt.Click(a.t("save")); err != nil {
		t.Fatal(err)
	}
	if a.cfg.Hotkeys["start"] != "Ctrl+Shift+P" || a.cfg.Hotkeys["view"] != "" || a.hotkeyOpen {
		t.Fatal(a.cfg.Hotkeys)
	}
	a.recordHotkey("start")
	tt.Frame()
	if err := tt.Click(a.t("remove")); err != nil {
		t.Fatal(err)
	}
	if a.cfg.Hotkeys["start"] != "" || a.hotkeyOpen {
		t.Fatal("binding was not removed")
	}
	if chord := hotkeyChord(ui.Shift, ui.KeyF1); !validHotkey(chord) {
		t.Fatal("source permits shift-only combinations", chord)
	}
}

func TestHotkeyDispatchPersistsModesAndNetworkSettings(t *testing.T) {
	a := testApp(t)
	a.cfg.SystemProxy = false
	for _, mode := range []string{"global", "direct", "rule"} {
		a.performHotkey("mode")
		if a.cfg.Overrides["mode"] != mode {
			t.Fatal(a.cfg.Overrides)
		}
	}
	for _, mode := range []string{"direct", "global", "rule"} {
		a.performHotkey(mode + "Mode")
		_, saved, err := store.Open(a.store.Dir)
		if err != nil || saved.Overrides["mode"] != mode {
			t.Fatal("mode not saved", saved.Overrides, err)
		}
	}
	a.performHotkey("proxy")
	a.performHotkey("tun")
	_, saved, err := store.Open(a.store.Dir)
	if err != nil || !saved.SystemProxy || !store.Bool(store.Map(saved.Overrides, "tun")["enable"]) || a.engine != nil {
		t.Fatal("safe dispatch did not persist settings without starting a core", saved, err)
	}
}

func TestGlobalDelayGroupsIgnoreViewFilters(t *testing.T) {
	a := populatedApp(t)
	a.proxyQuery = "no matching proxy"
	a.proxyFilter = "unmatched"
	if len(a.visibleProxyGroups()) != 0 {
		t.Fatal("fixture query should hide all groups")
	}
	groups := a.currentProxyGroups()
	if len(groups) == 0 || len(groups[0].All) == 0 {
		t.Fatal("global shortcut lost the hidden proxies")
	}
	a.cfg.Overrides["mode"] = "direct"
	if len(a.currentProxyGroups()) != 0 {
		t.Fatal("direct mode should have no delay-test groups")
	}
}
