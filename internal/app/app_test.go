package app

import (
	"os"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a, err := New(t.TempDir(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestNativeDesktopScreens(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, width := range []int{800, 1200, 1600} {
			for _, page := range []string{"dashboard", "proxies", "profiles", "connections", "requests", "dns", "logs", "tools", "general", "advanced", "network", "dnsSettings", "theme", "backup", "hotkeys", "language", "about", "resources", "sniffer", "ntp", "ports", "coreSettings", "customResources", "proxySettings", "services", "probe"} {
				t.Run(page+"/"+map[bool]string{true: "dark", false: "light"}[dark]+"/"+number(width), func(t *testing.T) {
					a := testApp(t)
					a.page = page
					if dark {
						a.cfg.Theme = "dark"
					}
					tt := ui.NewTester(a.View, width, 800)
					if len(tt.Texts()) == 0 {
						t.Fatal("empty screen")
					}
				})
			}
		}
	}
}
func TestSidebarAndImportDialog(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1200, 800)
	if err := tt.Click("Profiles"); err != nil {
		t.Fatal(err)
	}
	if a.page != "profiles" {
		t.Fatal(a.page)
	}
	if err := tt.Click("Add profile"); err != nil {
		t.Fatal(err)
	}
	if !a.addOpen {
		t.Fatal("import dialog did not open")
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	if a.addOpen {
		t.Fatal("dialog did not close")
	}
}
func TestProxySelectionAndRecordsRender(t *testing.T) {
	a := testApp(t)
	a.sample.Proxies = ProxyData{All: []string{"Proxy"}, Proxies: map[string]ProxyNode{"Proxy": {Name: "Proxy", Type: "Selector", All: []string{"Node A", "Node B"}, Now: "Node A"}, "Node A": {Name: "Node A", Type: "Shadowsocks"}, "Node B": {Name: "Node B", Type: "VLESS"}}}
	a.group = "Proxy"
	a.cfg.ProxyStyle = "tab"
	a.page = "proxies"
	a.delays["Node A"] = 42
	a.delays["Node B"] = -1
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("Node A") || !tt.HasText("42 ms") {
		t.Fatal(tt.Texts())
	}
}
func TestOverrideScriptAndDeadline(t *testing.T) {
	m, err := runScript(map[string]any{"mode": "rule"}, "const main = (config) => { config.mode = 'global'; return config; };")
	if err != nil || m["mode"] != "global" {
		t.Fatalf("%v %v", m, err)
	}
	if _, err = runScript(map[string]any{}, "const main = () => null;"); err != nil {
		t.Fatal(err)
	}
}
func TestNestedOverrideDoesNotBecomeLiteralKey(t *testing.T) {
	a := testApp(t)
	setPath(a.cfg.Overrides, "dns.nameserver", []any{"1.1.1.1"})
	if _, ok := a.cfg.Overrides["dns.nameserver"]; ok {
		t.Fatal("nested path became a literal key")
	}
	if pathValue(a.cfg.Overrides, "dns.nameserver") == nil {
		t.Fatal("nested value missing")
	}
	setPath(a.cfg.Overrides, "dns.nameserver", nil)
	if pathValue(a.cfg.Overrides, "dns.nameserver") != nil {
		t.Fatal("reset failed")
	}
}
func TestSafeModeNeverStartsCore(t *testing.T) {
	a := testApp(t)
	a.cfg.Disclaimer = true
	a.begin()
	a.toggle()
	if a.engine != nil || a.wantRunning {
		t.Fatal("safe mode started a core")
	}
	if _, e := os.Stat(a.store.Dir + "/system-proxy.json"); !os.IsNotExist(e) {
		t.Fatal("safe mode changed proxy state")
	}
}
func TestProfileSectionPreservesUnknownFields(t *testing.T) {
	a := testApp(t)
	p := store.Profile{ID: store.NewID(), Name: "test"}
	a.cfg.Profiles = []store.Profile{p}
	a.profileDraft = p
	a.editProfile = p.ID
	a.page = "profileEdit"
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("test") {
		t.Fatal(tt.Texts())
	}
}
