package store

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFlutterSQLiteBackupRelationshipsAndPortableRoundTrip(t *testing.T) {
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
	if len(cfg.Profiles) != 3 || cfg.Profiles[0].Name != "Subscription" || cfg.Profiles[1].Name != "Custom" || cfg.Profiles[2].Name != "Scripted" {
		t.Fatalf("profiles: %+v", cfg.Profiles)
	}
	if cfg.Active != cfg.Profiles[1].ID || cfg.Theme != "dark" || cfg.Accent != "#123456" || cfg.Locale != "ja" || cfg.Sidebar || cfg.CloseToTray || cfg.SystemProxy || cfg.TestTimeout != 5000 || !cfg.Disclaimer || cfg.WebDAVPassword != "example-password" {
		t.Fatalf("settings were not mapped: %+v", cfg)
	}
	p := cfg.Profiles[0]
	if p.Interval != 1 || p.IntervalMS != 3600000 || p.Total != 1048576 || p.Upload != 1024 || p.Selected["Select"] != "Node" || len(p.Unfold) != 1 || p.Group != "Select" || p.Updated.Unix() != 1800000000 {
		t.Fatalf("profile metadata: %+v", p)
	}
	if p.UpdateDue(p.Updated.Add(time.Hour-time.Millisecond)) || !p.UpdateDue(p.Updated.Add(time.Hour)) {
		t.Fatal("restored subscription updates at the wrong time", p.UpdateInterval())
	}
	if cfg.Profiles[2].ScriptID != cfg.Scripts[0].ID || !strings.Contains(cfg.Scripts[0].Source, "config.extra") {
		t.Fatal("shared script link lost")
	}
	var snapshot map[string]any
	if err = json.Unmarshal(cfg.ImportSnapshot, &snapshot); err != nil || !Bool(Map(flutterJSONMap(snapshot["config"]), "future-setting")["preserve"]) {
		t.Fatal("unknown source data lost", err)
	}
	assertCustom := func(s *Store, cfg Settings) {
		t.Helper()
		effective, e := s.Effective(cfg)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(effective["rules"], []any{"RULE-SET,Local set,Select", "MATCH,Select"}) {
			t.Fatal("custom rules", effective["rules"])
		}
		proxies := effective["proxies"].([]any)
		if len(proxies) != 2 || proxies[0].(map[string]any)["dialer-proxy"] != "DIRECT" || proxies[1].(map[string]any)["dialer-proxy"] != nil {
			t.Fatal("dialer mapping/loop suppression", proxies)
		}
		provider := Map(Map(effective, "proxy-providers"), "Subscription")
		if provider["path"] != s.ProfilePath(cfg.Profiles[0].ID) || provider["filter"] != "Node" {
			t.Fatal("profile provider", provider)
		}
		rules := Map(Map(effective, "rule-providers"), "Local set")
		if rules["path"] != s.ResourcePath(cfg.Resources[0]) {
			t.Fatal("local resource reference", rules)
		}
		body, e := os.ReadFile(String(rules["path"]))
		if e != nil || !bytes.Contains(body, []byte("local.example")) {
			t.Fatal("local rule set", e)
		}
		if Map(effective, "dns")["ipv6"] != true {
			t.Fatal("selected profile DNS override lost")
		}
	}
	assertCustom(s, cfg)
	other := Clone(cfg)
	other.Active = p.ID
	effective, err := s.Effective(other)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(effective["rules"], []any{"IP-CIDR,192.0.2.0/24,DIRECT,no-resolve", "DOMAIN,global.example,DIRECT", "MATCH,DIRECT"}) {
		t.Fatal("global/own/disabled rule ordering", effective["rules"])
	}
	body, _ := os.ReadFile(s.ProfilePath(p.ID))
	if !bytes.Contains(body, []byte("# original comment")) {
		t.Fatal("source YAML rewritten")
	}
	backup, err := s.Backup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	target, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	restored, err := target.Restore(backup)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Active == cfg.Active || restored.Resources[0] == cfg.Resources[0] {
		t.Fatal("restore reused file identities")
	}
	assertCustom(target, restored)
}

func backupArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	for name, b := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
func TestFlutterLegacyBackup(t *testing.T) {
	config := map[string]any{"currentProfileId": "old-profile", "appSetting": map[string]any{"locale": "en"}, "scripts": []any{map[string]any{"id": "old-script", "label": "Old script", "content": "function main(config) { return config }"}}, "rules": []any{map[string]any{"id": "disabled", "value": "DOMAIN,disabled.example,DIRECT"}}, "profiles": []any{map[string]any{"id": "old-profile", "label": "Old profile", "url": "https://example.invalid/sub", "autoUpdateDuration": 3600000000, "autoUpdate": true, "overwrite": map[string]any{"type": "script", "scriptOverwrite": map[string]any{"scriptId": "old-script"}}}}}
	b, _ := json.Marshal(config)
	s, _, _ := Open(t.TempDir())
	cfg, err := s.Restore(backupArchive(t, map[string][]byte{"config.json": b, "profiles/old-profile.yaml": []byte("rules: [\"MATCH,DIRECT\"]\n")}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Interval != 1 || cfg.Profiles[0].IntervalMS != 3600000 || cfg.Profiles[0].ScriptID != cfg.Scripts[0].ID || cfg.Active != cfg.Profiles[0].ID {
		t.Fatalf("legacy mapping: %+v", cfg)
	}
}
func TestFlutterImportFailureLeavesCurrentSettingsAndFiles(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	cfg.Profiles = []Profile{{ID: NewID(), Name: "Keep"}}
	cfg.Active = cfg.Profiles[0].ID
	_ = AtomicWrite(s.ProfilePath(cfg.Active), []byte("rules: [\"MATCH,DIRECT\"]\n"))
	_ = s.Save(cfg)
	before, _ := os.ReadFile(filepath.Join(s.Dir, "settings.json"))
	fixture, _ := os.ReadFile("testdata/flutter-v21.zip")
	original, err := readBackupFiles(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"future-database", "missing-script", "missing-profile", "missing-provider", "bad-header", "broken-sqlite", "mixed-format", "bad-reference", "future-config", "traversal"} {
		t.Run(kind, func(t *testing.T) {
			files := map[string][]byte{}
			for k, v := range original {
				files[k] = bytes.Clone(v)
			}
			switch kind {
			case "future-database":
				binary.BigEndian.PutUint32(files["database.sqlite"][60:64], 999)
			case "missing-script":
				delete(files, "scripts/31.js")
			case "missing-profile":
				delete(files, "profiles/11.yaml")
			case "missing-provider":
				for k := range files {
					if strings.HasPrefix(k, "providers/") {
						delete(files, k)
					}
				}
			case "bad-header":
				files["database.sqlite"] = []byte("not SQLite")
			case "broken-sqlite":
				files["database.sqlite"] = files["database.sqlite"][:100]
			case "mixed-format":
				files["settings.json"] = []byte("{}")
			case "bad-reference":
				bad := Defaults()
				bad.Overrides["proxy-providers"] = map[string]any{"bad": map[string]any{"path": profileRef + NewID()}}
				b, _ := json.Marshal(bad)
				files = map[string][]byte{"settings.json": b}
			case "future-config":
				files["config.json"] = []byte(`{"version":999}`)
			case "traversal":
				files["../escaped.yaml"] = []byte("escape")
			}
			if _, err := s.Restore(backupArchive(t, files)); err == nil {
				t.Fatal("invalid backup accepted")
			}
			after, _ := os.ReadFile(filepath.Join(s.Dir, "settings.json"))
			if !bytes.Equal(before, after) {
				t.Fatal("current settings changed")
			}
			entries, _ := os.ReadDir(filepath.Join(s.Dir, "profiles"))
			if len(entries) != 1 {
				t.Fatal("failed import leaked profiles")
			}
			entries, _ = os.ReadDir(filepath.Join(s.Dir, "resources"))
			if len(entries) != 0 {
				t.Fatal("failed import leaked resources")
			}
		})
	}
}

func TestImportedRulePlaceholderTracksSubscriptionUpdates(t *testing.T) {
	archive, err := os.ReadFile("testdata/flutter-v21.zip")
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := s.Restore(archive)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Active = cfg.Profiles[0].ID
	cfg.Profiles[0].MatchTarget = ""
	originalOverride := cfg.Profiles[0].Override
	if !strings.Contains(originalOverride, "192.0.2.0/24,MATCH,no-resolve") {
		t.Fatal("import resolved the placeholder too early", originalOverride)
	}
	for _, target := range []string{"Before update", "After update"} {
		source := []byte("# untouched subscription\nrules: ['MATCH," + target + "']\n")
		if err := AtomicWrite(s.ProfilePath(cfg.Active), source); err != nil {
			t.Fatal(err)
		}
		effective, err := s.Effective(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if rules := effective["rules"].([]any); rules[0] != "IP-CIDR,192.0.2.0/24,"+target+",no-resolve" {
			t.Fatal("imported rule did not follow current MATCH", rules)
		}
		after, _ := os.ReadFile(s.ProfilePath(cfg.Active))
		if !bytes.Equal(source, after) || cfg.Profiles[0].Override != originalOverride {
			t.Fatal("runtime resolution changed stored input")
		}
	}
	cfg.Profiles[0].MatchTarget = "  Explicit  "
	effective, err := s.Effective(cfg)
	if err != nil || effective["rules"].([]any)[0] != "IP-CIDR,192.0.2.0/24,Explicit,no-resolve" {
		t.Fatal("explicit profile target lost", effective["rules"], err)
	}
	cfg.Profiles[0].MatchTarget = ""
	effective, err = s.EffectiveTransform(cfg, func(raw map[string]any) (map[string]any, error) {
		raw["rules"] = []any{"MATCH,Script target"}
		return raw, nil
	})
	if err != nil || effective["rules"].([]any)[0] != "IP-CIDR,192.0.2.0/24,Script target,no-resolve" {
		t.Fatal("rule target did not follow transformed source", effective["rules"], err)
	}
}

func TestCertificatePreferenceImportAndNativeRoundTrip(t *testing.T) {
	for _, check := range []bool{true, false} {
		cfg := Defaults()
		mapFlutterSettings(&cfg, map[string]any{"appSettingProps": map[string]any{"checkCertificate": check}})
		if cfg.SkipCertificateVerify == check {
			t.Fatal("certificate preference inverted on import", check)
		}
		s, _, err := Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		data, err := s.Backup(cfg)
		if err != nil {
			t.Fatal(err)
		}
		target, _, _ := Open(t.TempDir())
		restored, err := target.Restore(data)
		if err != nil || restored.SkipCertificateVerify != cfg.SkipCertificateVerify {
			t.Fatal("certificate preference lost on backup", err)
		}
	}
}
