package store

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestFlutterExportLegacyUUIDAndRawRulePromotion(t *testing.T) {
	source := map[string]any{"currentProfileId": "old-profile", "scripts": []any{map[string]any{"id": "old-script", "label": "Old script", "content": "function main(config) { return config }"}}, "rules": []any{map[string]any{"id": "global", "value": "DOMAIN,global.invalid,DIRECT"}}, "profiles": []any{map[string]any{"id": "old-profile", "label": "Old profile", "autoUpdateDuration": 1234567000, "overwrite": map[string]any{"type": "standard", "standardOverwrite": map[string]any{"addedRules": []any{map[string]any{"id": "own", "value": "IP-CIDR,192.0.2.0/24,MATCH,no-resolve"}}, "disabledRuleIds": []any{"global"}}}}}}
	config, _ := json.Marshal(source)
	s, _, _ := Open(t.TempDir())
	cfg, err := s.Restore(backupArchive(t, map[string][]byte{"config.json": config, "profiles/old-profile.yaml": []byte("rules: ['MATCH,DIRECT']\n")}))
	if err != nil {
		t.Fatal(err)
	}
	files, tables, _ := exportedFlutter(t, s, cfg)
	if len(tables["rules"]) != 2 || len(tables["disabled_rules"]) != 1 || len(tables["scripts"]) != 1 {
		t.Fatal("legacy entities lost", tables)
	}
	for _, row := range tables["rules"] {
		if String(row["rule_action"]) == "" || row["raw"] != nil {
			t.Fatal("legacy rule was not promoted", row)
		}
	}
	target, _, _ := Open(t.TempDir())
	restored, err := target.Restore(backupArchive(t, files))
	if err != nil {
		t.Fatal(err)
	}
	effective, err := target.Effective(restored)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(effective["rules"], []any{"IP-CIDR,192.0.2.0/24,DIRECT,no-resolve", "MATCH,DIRECT"}) {
		t.Fatal(effective["rules"])
	}
}

func TestFlutterExportEditedOverridesAndUnknownPreferences(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-v21.zip")
	if err != nil {
		t.Fatal(err)
	}
	files, err := readBackupFiles(data)
	if err != nil {
		t.Fatal(err)
	}
	var original map[string]any
	if err = json.Unmarshal(files["config.json"], &original); err != nil {
		t.Fatal(err)
	}
	patch := flutterJSONMap(original["patchClashConfig"])
	Map(patch, "dns")["future-option"] = map[string]any{"keep": true}
	files["config.json"], _ = json.Marshal(original)
	s, _, _ := Open(t.TempDir())
	cfg, err := s.Restore(backupArchive(t, files))
	if err != nil {
		t.Fatal(err)
	}
	// Changing the native custom overlay should remove stale selected DNS keys,
	// not silently resurrect the source's old DNS selection.
	cfg.Profiles[1].Override = "rules: ['MATCH,REJECT']\n"
	cfg.Overrides["dns"] = map[string]any{"ipv6": true, "nameserver-policy": map[string]any{"example.invalid": []any{"1.1.1.1", "8.8.8.8"}}}
	cfg.Overrides["hosts"] = map[string]any{"example.invalid": []any{"192.0.2.1", "192.0.2.2"}}
	// Flutter only applies these app-level DNS keys to custom profiles.
	// A mixed library would silently lose nameserver-policy on its file/URL
	// profiles, so that case is rejected separately by the exporter.
	cfg.Profiles = cfg.Profiles[1:2]
	exported, tables, config := exportedFlutter(t, s, cfg)
	dns := flutterJSONMap(flutterJSONMap(config["patchClashConfig"])["dns"])
	if !Bool(flutterJSONMap(dns["future-option"])["keep"]) || !reflect.DeepEqual(flutterJSONList(dns["nameserver"]), []any{"https://example.invalid/dns-query"}) {
		t.Fatal("unknown/unselected DNS source values lost", dns)
	}
	if String(flutterJSONMap(dns["nameserver-policy"])["example.invalid"]) != "1.1.1.1,8.8.8.8" || String(flutterJSONMap(flutterJSONMap(config["patchClashConfig"])["hosts"])["example.invalid"]) != "192.0.2.1,192.0.2.2" {
		t.Fatal("not original StringMapConverter values", config)
	}
	var custom map[string]any
	for _, row := range tables["profiles"] {
		if String(row["type"]) == "custom" {
			custom = row
		}
	}
	if len(flutterJSONList(flutterJSONMap(custom["overrides"])["dns-override-keys"])) != 0 {
		t.Fatal("removed DNS selection reappeared")
	}
	other, _, _ := Open(t.TempDir())
	restored, err := other.Restore(backupArchive(t, exported))
	if err != nil {
		t.Fatal(err)
	}
	effective, err := other.Effective(restored)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(effective["rules"], []any{"MATCH,REJECT"}) {
		t.Fatal("edited rules changed", effective["rules"])
	}
}

func TestFlutterExportRejectsFileProviderInsideSubscriptionYAML(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	id, resource := NewID(), NewID()
	cfg.Active = id
	cfg.Profiles = []Profile{{ID: id, Name: "Profile"}}
	cfg.Resources = []string{resource}
	raw := []byte("# comment\nrule-providers:\n  Local:\n    type: file\n    path: " + resourceRef + resource + "\nrules: ['RULE-SET,Local,DIRECT']\n")
	if err := AtomicWrite(s.ProfilePath(id), raw); err != nil {
		t.Fatal(err)
	}
	content := []byte("payload: ['DOMAIN,example.invalid']\n")
	if err := AtomicWrite(s.ResourcePath(resource), content); err != nil {
		t.Fatal(err)
	}
	// A native re-import used to pass here, but Flutter's confineProviders
	// replaces this path and its restore does not copy files to that cache.
	if _, err := s.FlutterBackup(cfg); err == nil || !strings.Contains(err.Error(), "unbacked cache path") {
		t.Fatal("source file provider silently exported", err)
	}
	if after, err := os.ReadFile(s.ProfilePath(id)); err != nil || !bytes.Equal(after, raw) {
		t.Fatal("failed export changed source", err)
	}
	if after, err := os.ReadFile(s.ResourcePath(resource)); err != nil || !bytes.Equal(after, content) {
		t.Fatal("failed export changed resource", err)
	}
}

func TestFlutterExportOverrideKeyPathsAgainstPinnedEnums(t *testing.T) {
	source, err := os.ReadFile("../../build/upstream/lib/enum/enum.dart")
	if os.IsNotExist(err) {
		t.Skip("pinned source unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	for section, typ := range map[string]string{"dns": "Dns", "ntp": "Ntp", "sniffer": "Sniffer", "tun": "Tun"} {
		block := regexp.MustCompile(`(?s)enum ` + typ + `OverrideKey .*?\n}`).FindString(string(source))
		paths := regexp.MustCompile(`\w+\('([^']+)'\)`).FindAllStringSubmatch(block, -1)
		if len(paths) == 0 {
			t.Fatal("original enum not found", typ)
		}
		for _, path := range paths {
			if !strings.Contains(flutterOverrideKeys[section], ","+path[1]+",") {
				t.Fatal("missing original override path", section, path[1])
			}
		}
		if len(paths) != len(strings.Split(strings.Trim(flutterOverrideKeys[section], ","), ",")) {
			t.Fatal("invented override path", section)
		}
		if section == "dns" {
			normal := regexp.MustCompile(`(?s)static const normalProfileKeys = \{(.*?)\};`).FindStringSubmatch(block)
			if normal == nil {
				t.Fatal("original normal-profile DNS selection not found")
			}
			byName := map[string]string{}
			for _, entry := range regexp.MustCompile(`(\w+)\('([^']+)'\)`).FindAllStringSubmatch(block, -1) {
				byName[entry[1]] = entry[2]
			}
			names := regexp.MustCompile(`\w+`).FindAllString(normal[1], -1)
			if len(names) != len(strings.Split(strings.Trim(flutterNormalDNSKeys, ","), ",")) {
				t.Fatal("normal-profile DNS selection differs from original")
			}
			for _, name := range names {
				if !strings.Contains(flutterNormalDNSKeys, ","+byName[name]+",") {
					t.Fatal("missing original normal-profile DNS key", name)
				}
			}
		}
	}
}
