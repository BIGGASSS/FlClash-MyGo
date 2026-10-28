package app

import (
	"strings"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestNestedNativeEditorRetainsExtraFields(t *testing.T) {
	a := testApp(t)
	a.openCollection("DNS", "global:dns", map[string]any{"enable": true, "nameserver": []any{"https://example.com/dns-query"}, "future-option": "preserve"})
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("Future option") {
		t.Fatal(tt.Texts())
	}
	a.document.push("nameserver")
	tt.Frame()
	if _, ok := tt.Find("DNS 1"); !ok {
		t.Fatal(tt.Texts())
	}
	a.documentAdd()
	a.document.replace([]any{"1.1.1.1", "8.8.8.8"})
	a.saveDocument()
	dns := pathValue(a.cfg.Overrides, "dns").(map[string]any)
	if dns["future-option"] != "preserve" {
		t.Fatal(dns)
	}
	if len(dns["nameserver"].([]any)) != 2 {
		t.Fatal(dns)
	}
}
func TestProfileRuleFormSerializesNoResolve(t *testing.T) {
	a := testApp(t)
	a.openCollection("Rules", "global:rules", []any{"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve"})
	a.document.editRule(0, "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve")
	if a.document.RuleType != "IP-CIDR" || a.document.RuleTarget != "DIRECT" || a.document.RuleValue != "10.0.0.0/8" || !a.document.RuleNoResolve {
		t.Fatal(a.document)
	}
	_ = ui.NewTester(a.View, 1200, 800)
}
func TestAsyncScriptConsoleAndUnsettledPromise(t *testing.T) {
	m, e := runScript(map[string]any{"mode": "rule"}, "const main=async (config)=>{ console.info('test'); await Promise.resolve(); config.mode='direct'; return config; };")
	if e != nil || m["mode"] != "direct" {
		t.Fatalf("%v %v", m, e)
	}
	if _, e = runScript(map[string]any{}, "const main = () => new Promise(() => {});"); e == nil {
		t.Fatal("unsettled promise accepted")
	}
}
func TestHotkeyRecorder(t *testing.T) {
	a := testApp(t)
	a.recordHotkey("start")
	tt := ui.NewTester(a.View, 1200, 800)
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyP)
	if a.hotkeyDraft != "Ctrl+Shift+P" {
		t.Fatal(a.hotkeyDraft)
	}
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if a.cfg.Hotkeys["start"] != "Ctrl+Shift+P" {
		t.Fatal(a.cfg.Hotkeys)
	}
}
func TestRulesPrecedeSubscriptionAndShortIDStaysText(t *testing.T) {
	s, cfg, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	p := store.Profile{ID: store.NewID(), Name: "sample", Override: "tun: {stack: gvisor}\n"}
	cfg.Profiles = []store.Profile{p}
	cfg.Active = p.ID
	cfg.Overrides["rules"] = []any{"DOMAIN,example.com,DIRECT"}
	if e = store.AtomicWrite(s.ProfilePath(p.ID), []byte("proxies: [{name: reality, type: vless, reality-opts: {short-id: 00001234}}]\nrules: ['MATCH,DIRECT']\n")); e != nil {
		t.Fatal(e)
	}
	raw, e := s.Effective(cfg)
	if e != nil {
		t.Fatal(e)
	}
	rules := raw["rules"].([]any)
	if len(rules) != 2 || rules[0] != "DOMAIN,example.com,DIRECT" {
		t.Fatal(rules)
	}
	node := raw["proxies"].([]any)[0].(map[string]any)
	short := node["reality-opts"].(map[string]any)["short-id"]
	if short != "00001234" {
		t.Fatalf("short-id %v (%T)", short, short)
	}
	if raw["tun"].(map[string]any)["stack"] != "gvisor" {
		t.Fatal(raw["tun"])
	}
}
func TestSectionPatchPreservesSourceComments(t *testing.T) {
	b, e := store.PatchSection([]byte("# retain header\nmode: rule # retain mode\n# retain DNS\ndns: {enable: false}\nrules: ['MATCH,DIRECT']\n"), "rules", []any{"DOMAIN,example.com,DIRECT", "MATCH,DIRECT"})
	if e != nil {
		t.Fatal(e)
	}
	m, e := store.ParseYAML(b)
	if e != nil {
		t.Fatal(e)
	}
	if m["mode"] != "rule" {
		t.Fatal(m)
	}
	for _, comment := range []string{"# retain header", "# retain mode", "# retain DNS"} {
		if !strings.Contains(string(b), comment) {
			t.Fatalf("lost %s: %s", comment, b)
		}
	}
}
