package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func flutterReviewFixture(t *testing.T) (*Store, Settings) {
	t.Helper()
	data, err := os.ReadFile("testdata/flutter-v21.zip")
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := s.Restore(data)
	if err != nil {
		t.Fatal(err)
	}
	return s, cfg
}

func assertFlutterReviewRoundTrip(t *testing.T, s *Store, cfg Settings) {
	t.Helper()
	before, err := s.Effective(cfg)
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.FlutterBackup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	other, _, _ := Open(t.TempDir())
	restored, err := other.Restore(data)
	if err != nil {
		t.Fatal(err)
	}
	after, err := other.Effective(restored)
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{"rules", "proxies", "proxy-groups"} {
		if jsonText(before[section]) != jsonText(after[section]) {
			t.Fatalf("%s changed: %s -> %s", section, jsonText(before[section]), jsonText(after[section]))
		}
	}
}

func TestFlutterExportRebuildsEditedCustomSourceAndMissingLibraryProxies(t *testing.T) {
	for _, mode := range []string{"source-edit", "removed-library"} {
		t.Run(mode, func(t *testing.T) {
			s, cfg := flutterReviewFixture(t)
			if mode == "source-edit" {
				body := []byte("proxy-groups: [{name: Extra, type: select, proxies: [DIRECT]}]\nrules: ['DOMAIN,extra.invalid,DIRECT']\n")
				if err := AtomicWrite(s.ProfilePath(cfg.Active), body); err != nil {
					t.Fatal(err)
				}
			} else {
				// The native overlay still contains these definitions. Keeping the
				// old group rows but emitting an empty library loses working nodes.
				cfg.LocalProxies = nil
			}
			assertFlutterReviewRoundTrip(t, s, cfg)
		})
	}
}

func TestFlutterExportRejectsScriptSelectedOverPreservedAddedRules(t *testing.T) {
	s, cfg := flutterReviewFixture(t)
	cfg.Profiles[0].ScriptID = cfg.Scripts[0].ID
	if _, err := s.FlutterBackup(cfg); err == nil {
		t.Fatal("script selection silently discarded native added rules")
	}
}

func TestFlutterExportCustomProviderLabelSwaps(t *testing.T) {
	for _, rebuild := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserved", true: "rebuilt"}[rebuild], func(t *testing.T) {
			s, cfg, _ := Open(t.TempDir())
			a, b, custom := NewID(), NewID(), NewID()
			cfg.Profiles = []Profile{{ID: a, Name: "A"}, {ID: b, Name: "B"}, {ID: custom, Name: "Custom", Type: "custom"}}
			cfg.Active = custom
			for _, id := range []string{a, b} {
				if err := AtomicWrite(s.ProfilePath(id), []byte("proxies: []\nrules: []\n")); err != nil {
					t.Fatal(err)
				}
			}
			body := "proxy-groups: [{name: Select, type: select, use: [A, B]}]\nproxy-providers:\n  A: {type: file, path: " + profileRef + a + ", filter: alpha}\n  B: {type: file, path: " + profileRef + b + ", filter: beta}\nrules: ['MATCH,Select']\n"
			if err := AtomicWrite(s.ProfilePath(custom), []byte(body)); err != nil {
				t.Fatal(err)
			}
			data, err := s.FlutterBackup(cfg)
			if err != nil {
				t.Fatal(err)
			}
			s, _, _ = Open(t.TempDir())
			cfg, err = s.Restore(data)
			if err != nil {
				t.Fatal(err)
			}
			cfg.Profiles[0].Name, cfg.Profiles[1].Name = "B", "A"
			if rebuild {
				overlay, err := ParseYAML([]byte(cfg.Profiles[2].Override))
				if err != nil {
					t.Fatal(err)
				}
				overlay["dns"] = map[string]any{"ipv6": true}
				cfg.Profiles[2].Override, err = YAML(overlay)
				if err != nil {
					t.Fatal(err)
				}
			}
			_, tables, _ := exportedFlutter(t, s, cfg)
			if !reflect.DeepEqual(flutterJSONList(tables["proxy_groups"][0]["use"]), []any{"B", "A"}) {
				t.Fatal("provider use rename lost identity/order", tables["proxy_groups"])
			}
			options := flutterJSONMap(flutterJSONMap(tables["profiles"][2]["overrides"])["proxy-providers"])
			if String(flutterJSONMap(options["B"])["filter"]) != "alpha" || String(flutterJSONMap(options["A"])["filter"]) != "beta" {
				t.Fatal("provider options collided or were renamed twice", options)
			}
		})
	}
}

func TestFlutterExportBinaryCustomResource(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	id, resource := NewID(), NewID()
	cfg.Active, cfg.Resources = id, []string{resource}
	cfg.Profiles = []Profile{{ID: id, Name: "Custom", Type: "custom"}}
	body := "proxy-groups: []\nrule-providers:\n  Binary: {type: file, behavior: domain, format: mrs, path: " + resourceRef + resource + "}\nrules: ['RULE-SET,Binary,DIRECT']\n"
	if err := AtomicWrite(s.ProfilePath(id), []byte(body)); err != nil {
		t.Fatal(err)
	}
	// Deliberately non-UTF-8: the export boundary must never decode binary sets.
	binary := []byte{0x28, 0xb5, 0x2f, 0xfd, 0, 0xff, 0xfe, '\n', 0}
	if err := AtomicWrite(s.ResourcePath(resource), binary); err != nil {
		t.Fatal(err)
	}
	files, tables, _ := exportedFlutter(t, s, cfg)
	row := tables["clash_providers"][0]
	if String(row["format"]) != "mrs" || String(row["behavior"]) != "domain" || !bytes.Equal(files[flutterProviderEntry(String(row["id"]))], binary) {
		t.Fatal("binary resource or metadata changed", row)
	}
	other, _, _ := Open(t.TempDir())
	restored, err := other.Restore(backupArchive(t, files))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(other.ResourcePath(restored.Resources[0]))
	if err != nil || !bytes.Equal(content, binary) {
		t.Fatal("binary resource did not round-trip", err)
	}
}

func TestFlutterExportRejectsConflictingCustomProxyDefinitions(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	for i, server := range []string{"192.0.2.1", "192.0.2.2"} {
		id := NewID()
		cfg.Profiles = append(cfg.Profiles, Profile{ID: id, Name: server, Type: "custom"})
		cfg.Active = id
		body := "proxies: [{name: Node, type: socks5, server: " + server + ", port: 1080}]\nproxy-groups: [{name: Select, type: select, proxies: [Node]}]\nrules: ['MATCH,Select']\n"
		if err := AtomicWrite(s.ProfilePath(id), []byte(body)); err != nil {
			t.Fatal(i, err)
		}
	}
	if _, err := s.FlutterBackup(cfg); err == nil || !strings.Contains(err.Error(), "conflicting custom proxy") {
		t.Fatal("different profile-local nodes were silently conflated", err)
	}
	// The native library explicitly wins over same-name source definitions.
	cfg.LocalProxies = []map[string]any{{"name": "Node", "type": "socks5", "server": "192.0.2.3", "port": 1080}}
	assertFlutterReviewRoundTrip(t, s, cfg)
}

func TestFlutterExportPromotesSourceDialersToPerProfileTable(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	id := NewID()
	cfg.Profiles, cfg.Active = []Profile{{ID: id, Name: "Custom", Type: "custom"}}, id
	body := []byte("proxies: [{name: Node, type: socks5, server: 192.0.2.1, port: 1080, dialer-proxy: DIRECT}]\nproxy-groups: [{name: Select, type: select, proxies: [Node]}]\nrules: ['MATCH,Select']\n")
	if err := AtomicWrite(s.ProfilePath(id), body); err != nil {
		t.Fatal(err)
	}
	assertFlutterReviewRoundTrip(t, s, cfg)
	_, tables, _ := exportedFlutter(t, s, cfg)
	if len(tables["proxy_dialers"]) != 1 || String(tables["proxy_dialers"][0]["target"]) != "DIRECT" {
		t.Fatal("source dialer was not promoted", tables)
	}
}

func TestFlutterExportRejectsAmbiguousResourcesAndUnsupportedData(t *testing.T) {
	for _, mode := range []string{"resource-label", "resource-format", "resource-alias", "opaque-resource", "provider-option", "rules-shape", "null-rules", "nan", "profile-port", "profile-dns", "global-dns"} {
		t.Run(mode, func(t *testing.T) {
			s, cfg, _ := Open(t.TempDir())
			a, b := NewID(), NewID()
			cfg.Resources = []string{a, b}
			for _, id := range cfg.Resources {
				if err := AtomicWrite(s.ResourcePath(id), []byte("payload: []\n")); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				id, resource, name, format := NewID(), a, "Set", "yaml"
				if i == 1 {
					switch mode {
					case "resource-label":
						resource = b
					case "resource-format":
						format = "text"
					case "resource-alias":
						name = "Alias"
					}
				}
				body := "rule-providers:\n  " + name + ": {type: file, path: " + resourceRef + resource + ", format: " + format + "}\nrules: ['RULE-SET," + name + ",DIRECT']\n"
				cfg.Profiles = append(cfg.Profiles, Profile{ID: id, Name: "Custom", Type: "custom"})
				cfg.Active = id
				switch mode {
				case "provider-option":
					body = "rule-providers: {Set: {type: http, url: 'https://example.invalid/rules', header: {Authorization: secret}}}\nrules: ['RULE-SET,Set,DIRECT']\n"
				case "rules-shape":
					cfg.Profiles[i].Override = "rules: MATCH,REJECT\n"
				case "null-rules":
					cfg.Profiles[i].Override = "rules: null\n"
				case "nan":
					body = "proxy-groups: [{name: Select, type: select, interval: .nan}]\nrules: ['MATCH,Select']\n"
				case "profile-port":
					cfg.Profiles[i].Type, cfg.Profiles[i].Override, body = "file", "mixed-port: 17891\n", "rules: []\n"
				case "profile-dns":
					cfg.Profiles[i].Type, cfg.Profiles[i].Override, body = "file", "dns: {ipv6: true}\n", "rules: []\n"
					cfg.Overrides["dns"] = map[string]any{"ipv6": false}
				case "global-dns":
					cfg.Profiles[i].Type, body = "file", "rules: []\n"
					cfg.Overrides["dns"] = map[string]any{"respect-rules": true}
				}
				if err := AtomicWrite(s.ProfilePath(id), []byte(body)); err != nil {
					t.Fatal(err)
				}
			}
			// Avoid an unrelated unused resource error masking the targeted case.
			if mode != "resource-label" && mode != "opaque-resource" {
				cfg.Resources = []string{a}
			}
			if mode == "profile-port" || mode == "profile-dns" || mode == "global-dns" {
				cfg.Resources = nil
			}
			if err := s.Save(cfg); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(filepath.Join(s.Dir, "settings.json"))
			input, _ := json.Marshal(cfg)
			if _, err := s.FlutterBackup(cfg); err == nil {
				t.Fatal("ambiguous/unrepresentable export succeeded")
			}
			after, _ := os.ReadFile(filepath.Join(s.Dir, "settings.json"))
			current, _ := json.Marshal(cfg)
			if !bytes.Equal(before, after) || !bytes.Equal(input, current) {
				t.Fatal("failed export modified settings")
			}
			if dirs, _ := filepath.Glob(filepath.Join(s.Dir, ".flutter-*")); len(dirs) != 0 {
				t.Fatal("failed export leaked private staging", dirs)
			}
		})
	}
}

func TestFlutterExportRejectsCorruptIdentityCorrespondence(t *testing.T) {
	for _, mode := range []string{"duplicate-map", "missing-row", "numeric-alias", "null-row"} {
		t.Run(mode, func(t *testing.T) {
			s, cfg := flutterReviewFixture(t)
			snapshot, err := decodeFlutterSnapshot(cfg.ImportSnapshot)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "duplicate-map":
				snapshot.Profiles[cfg.Profiles[1].ID] = snapshot.Profiles[cfg.Profiles[0].ID]
			case "missing-row":
				snapshot.Profiles[cfg.Profiles[0].ID] = "9999"
			case "numeric-alias":
				row := copyFlutterRow(snapshot.Tables["profiles"][0])
				row["id"] = "0" + String(row["id"])
				snapshot.Tables["profiles"] = append(snapshot.Tables["profiles"], row)
			case "null-row":
				snapshot.Tables["profiles"] = append(snapshot.Tables["profiles"], nil)
			}
			cfg.ImportSnapshot, err = json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.FlutterBackup(cfg); err == nil {
				t.Fatal("corrupt identity correspondence accepted")
			}
		})
	}
}

func TestFlutterExportConfinesParentSymlinksAndBoundsEntryCount(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	id := NewID()
	cfg.Profiles = []Profile{{ID: id, Name: "Profile"}}
	outside := t.TempDir()
	body := []byte("rules: ['MATCH,DIRECT']\n")
	path := filepath.Join(outside, id+".yaml")
	if err := AtomicWrite(path, body); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(s.Dir, "profiles")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(s.Dir, "profiles")); err != nil {
		t.Skip(err)
	}
	if _, err := s.FlutterBackup(cfg); err == nil {
		t.Fatal("parent symlink escaped export root")
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(body, after) {
		t.Fatal("export modified outside file", err)
	}
	cfg.Resources = make([]string, 20000)
	if _, err := s.FlutterBackup(cfg); err == nil || !strings.Contains(err.Error(), "entries") {
		t.Fatal("entry quota not checked before IO", err)
	}
}

func TestFlutterExportTotalQuotaAndSQLiteFailureCleanup(t *testing.T) {
	t.Run("total-size", func(t *testing.T) {
		s, cfg, _ := Open(t.TempDir())
		for i := 0; i < 5; i++ {
			id := NewID()
			cfg.Resources = append(cfg.Resources, id)
			f, err := os.Create(s.ResourcePath(id))
			if err != nil {
				t.Fatal(err)
			}
			err = f.Truncate(28 << 20)
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
			}
		}
		if _, err := s.FlutterBackup(cfg); err == nil || !strings.Contains(err.Error(), "size limit") {
			t.Fatal("total quota not enforced before SQLite/ZIP creation", err)
		}
		for _, id := range cfg.Resources {
			if info, err := os.Stat(s.ResourcePath(id)); err != nil || info.Size() != 28<<20 {
				t.Fatal("quota failure changed a resource", err)
			}
		}
	})
	t.Run("sqlite-failure", func(t *testing.T) {
		s, cfg, _ := Open(t.TempDir())
		id := NewID()
		cfg.Profiles = []Profile{{ID: id, Name: "Profile"}}
		body := []byte("# unchanged\nrules: ['MATCH,DIRECT']\n")
		if err := AtomicWrite(s.ProfilePath(id), body); err != nil {
			t.Fatal(err)
		}
		// A retained row missing required SQL fields fails after staging starts.
		cfg.ImportSnapshot = json.RawMessage(`{"tables":{"icon_sets":[{"id":1}]}}`)
		if err := s.Save(cfg); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(filepath.Join(s.Dir, "settings.json"))
		if _, err := s.FlutterBackup(cfg); err == nil || !strings.Contains(err.Error(), "icon_sets") {
			t.Fatal("expected SQLite constraint failure", err)
		}
		if after, _ := os.ReadFile(filepath.Join(s.Dir, "settings.json")); !bytes.Equal(before, after) {
			t.Fatal("SQLite failure modified saved settings")
		}
		if after, _ := os.ReadFile(s.ProfilePath(id)); !bytes.Equal(body, after) {
			t.Fatal("SQLite failure modified source")
		}
		if dirs, _ := filepath.Glob(filepath.Join(s.Dir, ".flutter-*")); len(dirs) != 0 {
			t.Fatal("SQLite failure leaked private staging", dirs)
		}
	})
}

func TestFlutterExportDoesNotResurrectRemovedOwnedSettings(t *testing.T) {
	s, cfg := flutterReviewFixture(t)
	snapshot, err := decodeFlutterSnapshot(cfg.ImportSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	patch := flutterJSONMap(snapshot.Config["patchClashConfig"])
	patch["hosts"] = map[string]any{"stale.invalid": "192.0.2.99"}
	patch["interface-name"], patch["interface-name-mode"] = "old-interface", "custom"
	flutterJSONMap(snapshot.Config["networkProps"])["authentication"] = map[string]any{"enable": true, "username": "old", "password": "secret"}
	cfg.ImportSnapshot, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	delete(cfg.Overrides, "hosts")
	delete(cfg.Overrides, "interface-name")
	delete(cfg.Overrides, "interface-name-mode")
	delete(cfg.Overrides, "authentication")
	_, _, config := exportedFlutter(t, s, cfg)
	patch = flutterJSONMap(config["patchClashConfig"])
	auth := flutterJSONMap(flutterJSONMap(config["networkProps"])["authentication"])
	if len(flutterJSONMap(patch["hosts"])) != 0 || String(patch["interface-name-mode"]) != "clear" || flutterBool(auth["enable"]) {
		t.Fatal("removed runtime settings were resurrected", patch, auth)
	}
}

func TestFlutterExportRecoveryDoesNotGuessAmongNativeDuplicates(t *testing.T) {
	cfg := Defaults()
	a, b, r1, r2 := NewID(), NewID(), NewID(), NewID()
	cfg.Profiles = []Profile{
		{ID: a, Name: "Same", Override: "rule-providers: {Set: {path: " + resourceRef + r1 + "}}\n"},
		{ID: b, Name: "Same", Type: "file", Override: "rule-providers: {Set: {path: " + resourceRef + r2 + "}}\n"},
	}
	cfg.Scripts = []Script{{ID: NewID(), Name: "Same"}, {ID: NewID(), Name: "Same"}}
	snapshot := flutterSnapshot{Tables: flutterTables{
		"profiles":        {{"id": 11, "label": "Same", "type": "file", "url": ""}},
		"scripts":         {{"id": 31, "label": "Same"}},
		"clash_providers": {{"id": 71, "label": "Set", "kind": "rule", "url": ""}},
	}}
	recoverFlutterIdentities(cfg, &snapshot)
	if len(snapshot.Profiles)+len(snapshot.Scripts)+len(snapshot.Resources) != 0 {
		t.Fatal("recovery guessed the first native duplicate", snapshot)
	}
}

func TestFlutterExportKeepsExactProxyNumbers(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	cfg.LocalProxies = []map[string]any{{"name": "Node", "type": "direct", "future-integer": json.Number("9007199254740993")}}
	_, tables, _ := exportedFlutter(t, s, cfg)
	if String(flutterJSONMap(tables["custom_proxies"][0]["definition"])["future-integer"]) != "9007199254740993" {
		t.Fatal("export clone rounded an integer")
	}
	for _, rule := range []string{"MATCH,DIRECT,unexpected", "MATCH,", "DOMAIN,,DIRECT", "DOMAIN,example.invalid,"} {
		if _, err := exportFlutterRule(rule); err == nil {
			t.Fatal("lossy rule projection accepted", rule)
		}
	}
}
