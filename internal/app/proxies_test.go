package app

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestProxyTextScaleKeepsNavigationAndFitsRows(t *testing.T) {
	for _, style := range []string{"list", "tab"} {
		for _, scale := range []float32{0.8, 1, 1.4} {
			t.Run(fmt.Sprintf("%s/%.1f", style, scale), func(t *testing.T) {
				a := populatedApp(t)
				a.page, a.cfg.ProxyStyle, a.cfg.Scale = "proxies", style, scale
				tt := ui.NewTester(a.View, 1200, 800)
				if r, ok := tt.Find(a.t("dashboard")); !ok || r.X > 20 || r.W < 180 || r.W > 220 {
					t.Fatal("text scale changed sidebar geometry", r)
				}
				if r, ok := tt.Find("United States 01"); !ok || r.X < 220 || r.H < a.proxyCardHeight()-1 {
					t.Fatal("proxy row did not fit scaled text", r, a.proxyCardHeight())
				}
				if err := tt.Click("Japan 01"); err != nil {
					t.Fatal("scaled row was not selectable", err)
				}
			})
		}
	}
}

func TestProxyPresentationModesAndExpansion(t *testing.T) {
	for _, style := range []string{"list", "tab"} {
		for _, card := range []string{"loose", "standard", "compact"} {
			for _, layout := range []string{"loose", "standard", "tight"} {
				t.Run(style+"/"+card+"/"+layout, func(t *testing.T) {
					a := populatedApp(t)
					a.page = "proxies"
					a.cfg.ProxyStyle = style
					a.cfg.ProxyCard = card
					a.cfg.ProxyLayout = layout
					tt := ui.NewTester(a.View, 1200, 800)
					if !tt.HasText("United States 01") || !tt.HasText("38 ms") {
						t.Fatal(tt.Texts())
					}
					if style == "list" {
						a.toggleProxyGroup("Proxy")
						tt.Frame()
						if a.groupUnfolded("Proxy") {
							t.Fatal("group failed to collapse")
						}
						a.locateProxy("Proxy")
						tt.Frame()
						if !a.groupUnfolded("Proxy") {
							t.Fatal("locate failed to expand group")
						}
					}
				})
			}
		}
	}
}
func TestProxyTimeoutFilteringKeepsSelectionsGroupsAndLastMatches(t *testing.T) {
	a := populatedApp(t)
	a.cfg.HideTimeout = true
	g := a.sample.Proxies.Proxies["Proxy"]
	for _, name := range g.All {
		a.setProxyDelay(name, a.cfg.TestURL, -1)
	}
	names := a.proxyNames(g)
	if !reflect.DeepEqual(names, []string{g.Now}) {
		t.Fatal("selected node was hidden", names)
	}
	a.proxyQuery = "Japan"
	names = a.proxyNames(g)
	if !reflect.DeepEqual(names, []string{"Japan 01"}) {
		t.Fatal("last search match was hidden", names)
	}
	a.proxyQuery = ""
	g.All = append(g.All, "Auto", "REJECT")
	a.sample.Proxies.Proxies["REJECT"] = ProxyNode{Name: "REJECT", Type: "Reject"}
	a.setProxyDelay("Auto", a.cfg.TestURL, -1)
	a.setProxyDelay("REJECT", a.cfg.TestURL, -1)
	if names = a.proxyNames(g); !reflect.DeepEqual(names, []string{g.Now, "Auto", "REJECT"}) {
		t.Fatal("group or unprobeable node hidden", names)
	}
}
func TestProxyDelaysRespectTestURLAndBatchSnapshot(t *testing.T) {
	a := populatedApp(t)
	g := a.sample.Proxies.Proxies["Proxy"]
	g.TestURL = "https://example.invalid/other"
	if _, ok := a.proxyDelayValue(g, g.Now, false); ok {
		t.Fatal("default URL delay leaked into custom test URL")
	}
	a.setProxyDelay(g.Now, g.TestURL, 200)
	a.delaySnapshot = map[string]int{delayKey(g.Now, g.TestURL): 200}
	a.setProxyDelay(g.Now, g.TestURL, -1)
	if value, _ := a.proxyDelayValue(g, g.Now, true); value != 200 {
		t.Fatal("in-flight test changed completed snapshot", value)
	}
	if value, _ := a.proxyDelayValue(g, g.Now, false); value != -1 {
		t.Fatal("current probe result not shown", value)
	}
}
func TestLocateSelectedInLargeVirtualProxyList(t *testing.T) {
	a := populatedApp(t)
	a.page = "proxies"
	g := a.sample.Proxies.Proxies["Proxy"]
	g.All = nil
	for i := range 10000 {
		name := fmt.Sprintf("Node %05d", i)
		g.All = append(g.All, name)
		a.sample.Proxies.Proxies[name] = ProxyNode{Name: name, Type: "Shadowsocks"}
	}
	g.Now = g.All[9900]
	a.sample.Proxies.Proxies["Proxy"] = g
	tt := ui.NewTester(a.View, 1200, 800)
	a.locateProxy("Proxy")
	tt.Frame()
	tt.Frame()
	if !tt.HasText(g.Now) {
		t.Fatal("selected proxy not revealed")
	}
}
func TestProxyGroupStateBelongsToProfile(t *testing.T) {
	a := testApp(t)
	one, two := store.NewID(), store.NewID()
	a.cfg.Profiles = []store.Profile{{ID: one, Name: "One", Group: "One group", Selected: map[string]string{"G": "A"}}, {ID: two, Name: "Two", Group: "Two group", Selected: map[string]string{"G": "B"}}}
	a.cfg.Active = one
	a.toggleProxyGroup("One group")
	a.activateProfile(two)
	if a.group != "Two group" || a.cfg.Selected["G"] != "B" || a.groupUnfolded("One group") {
		t.Fatal("profile UI state leaked")
	}
	a.activateProfile(one)
	if !a.groupUnfolded("One group") || a.cfg.Selected["G"] != "A" {
		t.Fatal("profile state was not restored")
	}
}
func TestSearchTermsMatchAcrossFields(t *testing.T) {
	if !searchMatches(" node  vless ", "Node one", "VLESS") || searchMatches("odev", "node", "vless") || searchMatches("node missing", "node", "vless") {
		t.Fatal("search differs from upstream term matching")
	}
}
func TestCreateNativeCustomProfile(t *testing.T) {
	a := testApp(t)
	a.addName = "Local setup"
	a.addKind = 3
	a.importSource()
	if len(a.cfg.Profiles) != 1 || a.cfg.Profiles[0].Type != "custom" || a.cfg.Active != a.cfg.Profiles[0].ID || a.page != "profileEdit" {
		t.Fatal(a.cfg.Profiles, a.page)
	}
	if _, err := a.store.Effective(a.cfg); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteActiveProfileRestoresNextProfileState(t *testing.T) {
	a := testApp(t)
	one, two := store.NewID(), store.NewID()
	a.cfg.Profiles = []store.Profile{{ID: one, Name: "One", Selected: map[string]string{"G": "Old"}}, {ID: two, Name: "Two", Group: "G", Selected: map[string]string{"G": "New"}}}
	a.cfg.Active = one
	a.cfg.Selected = map[string]string{"G": "Old"}
	a.group, a.proxyLocate = "Old group", "Old group"
	a.delays["Old"] = 55
	a.proxyScroll.Y = 1000
	a.deleteProfile(a.cfg.Profiles[0])
	a.confirm()
	if a.cfg.Active != two || a.group != "G" || a.cfg.Selected["G"] != "New" || len(a.delays) != 0 || a.proxyLocate != "" || a.proxyScroll.Y != 0 {
		t.Fatal("deletion retained previous profile state", a.cfg.Active, a.group, a.cfg.Selected, a.delays, a.proxyLocate, a.proxyScroll)
	}
	a.deleteProfile(a.cfg.Profiles[0])
	a.confirm()
	if a.cfg.Active != "" || a.group != "" || len(a.cfg.Selected) != 0 {
		t.Fatal("last deleted profile retained selections", a.cfg.Active, a.group, a.cfg.Selected)
	}
	if a.engine != nil {
		t.Fatal("deletion started a core in safe mode")
	}
}

func TestRestoreClearsPreviousProfileProbeAndScrollState(t *testing.T) {
	a := populatedApp(t)
	a.proxyScroll.Y, a.proxyLocate = 900, "Previous"
	cfg := store.Defaults()
	id := store.NewID()
	cfg.Active, cfg.Profiles = id, []store.Profile{{ID: id, Group: "Restored", Selected: map[string]string{"Restored": "New"}}}
	a.restoredSettings(cfg)
	if a.group != "Restored" || a.cfg.Selected["Restored"] != "New" || len(a.delays) != 0 || a.proxyLocate != "" || a.proxyScroll.Y != 0 {
		t.Fatal("restore retained previous profile state")
	}
}
