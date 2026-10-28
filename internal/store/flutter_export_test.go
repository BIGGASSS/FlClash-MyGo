package store

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func exportedFlutter(t *testing.T, s *Store, cfg Settings) (map[string][]byte, flutterTables, map[string]any) {
	t.Helper()
	data, err := s.FlutterBackup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	files, err := readBackupFiles(data)
	if err != nil {
		t.Fatal(err)
	}
	if files["settings.json"] != nil || files["resources/"] != nil || len(files["database.sqlite"]) < 100 || binary.BigEndian.Uint32(files["database.sqlite"][60:64]) != 21 {
		t.Fatal("not a Flutter v1/v21 archive")
	}
	tables, err := s.readFlutterSQLite(files["database.sqlite"])
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err = json.Unmarshal(files["config.json"], &config); err != nil {
		t.Fatal(err)
	}
	if Int(config["version"]) != 1 {
		t.Fatal(config)
	}
	// BackupEntries.ofData / readBackupArchive only name these paths.
	expected := map[string]bool{"config.json": true, "database.sqlite": true}
	for _, row := range tables["profiles"] {
		expected["profiles/"+String(row["id"])+".yaml"] = true
	}
	for _, row := range tables["scripts"] {
		expected["scripts/"+String(row["id"])+".js"] = true
	}
	for _, row := range tables["clash_providers"] {
		if String(row["url"]) == "" {
			expected[flutterProviderEntry(String(row["id"]))] = true
		}
	}
	for name := range files {
		if !expected[name] {
			t.Fatalf("not an original BackupEntries path: %s", name)
		}
	}
	for name := range expected {
		if files[name] == nil {
			t.Fatalf("missing original BackupEntries path: %s", name)
		}
	}
	assertFlutterDatabase(t, files["database.sqlite"])
	return files, tables, config
}

func assertFlutterDatabase(t *testing.T, data []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database.sqlite")
	if err := AtomicWrite(path, data); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var check string
	if err = db.QueryRow("PRAGMA integrity_check").Scan(&check); err != nil || check != "ok" {
		t.Fatal(check, err)
	}
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("dangling foreign key")
	}
}

func TestFlutterExportImportedMetadataAndNativeRoundTrip(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-v21.zip")
	if err != nil {
		t.Fatal(err)
	}
	s, _, _ := Open(t.TempDir())
	cfg, err := s.Restore(data)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise persistence/remapping, not only the import's in-memory identities.
	native, err := s.Backup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s, _, _ = Open(t.TempDir())
	cfg, err = s.Restore(native)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Profiles[0].Name = "Renamed subscription"
	cfg.Profiles[0].Upload = 1234567890123
	cfg.Profiles[0].AutoUpdate = new(bool)
	cfg.Profiles[0].IntervalMS = 1234567
	cfg.Scripts[0].Source += "\n// edited"
	cfg.Profiles[1].Selected["Select"] = "Loop" // Per-profile state wins over legacy cfg.Selected.
	files, tables, config := exportedFlutter(t, s, cfg)
	if !Bool(flutterJSONMap(config["future-setting"])["preserve"]) {
		t.Fatal("unknown config field lost")
	}
	if decodeFlutterDAVPassword(String(flutterJSONMap(config["davProps"])["password"])) != "example-password" {
		t.Fatal("DAV encoding is not source-compatible")
	}
	if len(tables["profiles"]) != 3 || len(tables["rules"]) != 5 || len(tables["disabled_rules"]) != 1 || len(tables["proxy_groups"]) != 1 || len(tables["proxy_dialers"]) != 2 {
		t.Fatalf("original relationships lost: %+v", tables)
	}
	if String(tables["scripts"][0]["last_update_time"]) != "1800000000" || String(tables["custom_proxies"][0]["empty_options"]) != "[\"sni\"]" {
		t.Fatal("unmapped metadata lost", tables)
	}
	if !bytes.Contains(files["profiles/11.yaml"], []byte("# original comment")) || !bytes.Contains(files["scripts/31.js"], []byte("// edited")) {
		t.Fatal("source content lost")
	}
	row := tables["profiles"][0]
	if String(row["id"]) != "11" || String(row["label"]) != "Renamed subscription" || flutterBool(row["auto_update"]) || Int(row["auto_update_duration_millis"]) != 1234567 || Int(flutterJSONMap(row["subscription_info"])["upload"]) != 1234567890123 {
		t.Fatal("profile metadata not updated", row)
	}
	if Int(config["currentProfileId"]) != 13 || String(flutterJSONMap(tables["profiles"][2]["selected_map"])["Select"]) != "Loop" {
		t.Fatal("profile selection lost", config, tables["profiles"])
	}
	other, _, _ := Open(t.TempDir())
	restored, err := other.Restore(backupArchive(t, files))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Profiles[0].Upload != 1234567890123 || restored.Scripts[0].Source != cfg.Scripts[0].Source || restored.Profiles[1].Selected["Select"] != "Loop" {
		t.Fatal("round trip metadata", restored)
	}
	effective, err := other.Effective(restored)
	if err != nil {
		t.Fatal(err)
	}
	if String(Map(Map(effective, "proxy-providers"), "Renamed subscription")["path"]) != other.ProfilePath(restored.Profiles[0].ID) {
		t.Fatal("renamed library reference lost", effective)
	}
}

func TestFlutterExportNativeProfilesScriptsResources(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	plain, scripted, custom, script, resource := NewID(), NewID(), NewID(), NewID(), NewID()
	cfg.Active = custom
	cfg.Selected = map[string]string{"Select": "Node"}
	cfg.Theme = "dark"
	cfg.Scale = 1.2
	cfg.FloatingNavigation = false
	cfg.WebDAVPassword = "pässword🔒"
	cfg.Hotkeys["start"] = "Ctrl+Alt+S"
	cfg.Scripts = []Script{{ID: script, Name: "Shared", Source: "function main(config) { return config; }"}}
	cfg.Profiles = []Profile{
		{ID: plain, Name: "Source", Type: "url", URL: "https://example.invalid/sub", IntervalMS: 12345, Updated: time.Unix(1700000000, 0)},
		{ID: scripted, Name: "Scripted", ScriptID: script, AgeSecretKey: "secret", Selected: map[string]string{"Other": "DIRECT"}, Unfold: []string{"Other"}},
		{ID: custom, Name: "Custom", Type: "custom", Dialers: map[string]string{"Node": "DIRECT"}, Override: "dns:\n  ipv6: true\n"},
	}
	cfg.Profiles[0].Override = "rules: [\"DOMAIN,example.invalid,DIRECT\"]\n"
	cfg.Resources = []string{resource}
	cfg.LocalProxies = []map[string]any{{"name": "Node", "type": "ss", "server": "192.0.2.1", "port": 443, "cipher": "aes-128-gcm", "password": "fixture"}}
	source := []byte("# Keep byte exact\nproxies: []\nrules: ['MATCH,DIRECT']\nunknown-setting: keep\n")
	if err := AtomicWrite(s.ProfilePath(plain), source); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(s.ProfilePath(scripted), source); err != nil {
		t.Fatal(err)
	}
	customBody := []byte("proxies: []\nproxy-groups:\n  - name: Select\n    type: select\n    proxies: [Node]\n    use: [Source]\nproxy-providers:\n  Source:\n    type: file\n    path: " + profileRef + plain + "\nrule-providers:\n  Local:\n    type: file\n    behavior: classical\n    format: yaml\n    path: " + resourceRef + resource + "\nrules: ['RULE-SET,Local,Select', 'MATCH,Select']\n")
	if err := AtomicWrite(s.ProfilePath(custom), customBody); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(s.ResourcePath(resource), []byte("payload: ['DOMAIN,local.invalid']\n")); err != nil {
		t.Fatal(err)
	}
	files, _, _ := exportedFlutter(t, s, cfg)
	if !bytes.Equal(files["profiles/1.yaml"], source) {
		t.Fatal("unmodified source bytes lost")
	}
	other, _, _ := Open(t.TempDir())
	restored, err := other.Restore(backupArchive(t, files))
	if err != nil {
		t.Fatal(err)
	}
	if restored.WebDAVPassword != cfg.WebDAVPassword || restored.FloatingNavigation || restored.Hotkeys["start"] != "Ctrl+Alt+S" || restored.Profiles[1].ScriptID != restored.Scripts[0].ID || restored.Profiles[1].AgeSecretKey != "secret" || restored.Profiles[0].IntervalMS != 12345 {
		t.Fatal("settings/scripts did not round trip", restored)
	}
	effective, err := other.Effective(restored)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(effective["rules"], []any{"RULE-SET,Local,Select", "MATCH,Select"}) || len(effective["proxy-groups"].([]any)) != 1 || len(effective["proxies"].([]any)) != 1 || !Bool(Map(effective, "dns")["ipv6"]) {
		t.Fatal("custom structure changed", effective)
	}
	if String(Map(Map(effective, "proxy-providers"), "Source")["path"]) != other.ProfilePath(restored.Profiles[0].ID) {
		t.Fatal("profile reference not portable")
	}
	if String(Map(Map(effective, "rule-providers"), "Local")["path"]) != other.ResourcePath(restored.Resources[0]) {
		t.Fatal("resource not portable")
	}
}

func TestFlutterExportBoundedPrivateIOAndNoMutation(t *testing.T) {
	for _, mode := range []string{"bad-id", "duplicate", "missing-script", "symlink", "oversize", "path", "parent-path", "missing-file", "unsupported", "unsupported-dns"} {
		t.Run(mode, func(t *testing.T) {
			s, cfg, _ := Open(t.TempDir())
			id := NewID()
			cfg.Profiles = []Profile{{ID: id, Name: "Test"}}
			cfg.Active = id
			if err := AtomicWrite(s.ProfilePath(id), []byte("proxies: []\nrules: []\n")); err != nil {
				t.Fatal(err)
			}
			if err := s.Save(cfg); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(s.Dir, "settings.json"))
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "bad-id":
				cfg.Profiles[0].ID = "../../escape"
			case "duplicate":
				cfg.Profiles = append(cfg.Profiles, cfg.Profiles[0])
			case "missing-script":
				cfg.Profiles[0].ScriptID = NewID()
			case "symlink":
				if err := os.Remove(s.ProfilePath(id)); err != nil {
					t.Fatal(err)
				}
				outside := filepath.Join(t.TempDir(), "outside")
				if err := os.WriteFile(outside, []byte("rules: []"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, s.ProfilePath(id)); err != nil {
					t.Skip(err)
				}
			case "oversize":
				f, err := os.OpenFile(s.ProfilePath(id), os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				if err = f.Truncate((32 << 20) + 1); err != nil {
					t.Fatal(err)
				}
				f.Close()
			case "path":
				if err := AtomicWrite(s.ProfilePath(id), []byte("rule-providers: {x: {type: file, path: /etc/passwd}}")); err != nil {
					t.Fatal(err)
				}
			case "parent-path":
				if err := AtomicWrite(s.ProfilePath(id), []byte("rule-providers: {x: {type: http, path: ..}}")); err != nil {
					t.Fatal(err)
				}
			case "missing-file":
				if err := AtomicWrite(s.ProfilePath(id), []byte("rule-providers: {x: {type: file, path: missing.yaml}}")); err != nil {
					t.Fatal(err)
				}
			case "unsupported-dns":
				cfg.Overrides["dns"] = map[string]any{"new-native-field": true}
			case "unsupported":
				cfg.Overrides["native-only"] = true
			}
			untouched, _ := json.Marshal(cfg)
			if _, err := s.FlutterBackup(cfg); err == nil {
				t.Fatal("unsafe/unrepresentable backup accepted")
			}
			afterExport, _ := json.Marshal(cfg)
			if !bytes.Equal(afterExport, untouched) {
				t.Fatal("export modified caller settings")
			}
			after, _ := os.ReadFile(filepath.Join(s.Dir, "settings.json"))
			if !bytes.Equal(before, after) {
				t.Fatal("export modified saved settings")
			}
			entries, _ := filepath.Glob(filepath.Join(s.Dir, ".flutter-*"))
			if len(entries) != 0 {
				t.Fatal("temporary private data leaked", entries)
			}
		})
	}
}

func TestFlutterExportSchemaAgainstPinnedGeneratedDrift(t *testing.T) {
	// This is a source/schema comparison, NOT an original Flutter restore run.
	source, err := os.ReadFile("../../build/upstream/lib/database/generated/database.g.dart")
	if os.IsNotExist(err) {
		t.Skip("pinned original checkout unavailable; committed SQL still covered by round trips and integrity checks")
	}
	if err != nil {
		t.Fatal(err)
	}
	s, _, _ := Open(t.TempDir())
	data, err := s.writeFlutterSQLite(flutterTables{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "database.sqlite")
	if err = AtomicWrite(path, data); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	classes := regexp.MustCompile(`(?s)class \$\w+Table extends .*?(?:\nclass |\z)`).FindAllString(string(source), -1)
	colPattern := regexp.MustCompile(`(?s)GeneratedColumn<[^>]+>\(\s*'([^']+)',\s*aliasedName,\s*(true|false),\s*type:\s*DriftSqlType\.(\w+)`)
	namePattern := regexp.MustCompile(`static const String \$name = '([^']+)'`)
	checked := 0
	for _, block := range classes {
		name := namePattern.FindStringSubmatch(block)
		if name == nil {
			continue
		}
		head := strings.Split(block, "List<GeneratedColumn> get $columns")[0]
		cols := colPattern.FindAllStringSubmatch(head, -1)
		fields := map[string]string{}
		for _, field := range regexp.MustCompile(`(?s)(\w+)\s*=\s*GeneratedColumn<[^>]+>\(\s*'([^']+)'`).FindAllStringSubmatch(head, -1) {
			fields[field[1]] = field[2]
		}
		pkSource := regexp.MustCompile(`get \$primaryKey => \{([^}]+)}`).FindStringSubmatch(block)
		wantPK := map[string]int{}
		for n, key := range strings.Split(pkSource[1], ",") {
			wantPK[fields[strings.TrimSpace(key)]] = n + 1
		}
		var ddl string
		if err := db.QueryRow(`SELECT sql FROM sqlite_schema WHERE name=?`, name[1]).Scan(&ddl); err != nil {
			t.Fatal(err)
		}
		for _, constraint := range regexp.MustCompile(`(?s)constraintIsAlways\(\s*'([^']+)'`).FindAllStringSubmatch(head, -1) {
			if !strings.Contains(ddl, constraint[1]) {
				t.Fatal("original Drift constraint missing", name[1], constraint[1])
			}
		}
		rows, err := db.Query(`PRAGMA table_info("` + name[1] + `")`)
		if err != nil {
			t.Fatal(err)
		}
		i := 0
		for rows.Next() {
			var cid, notnull, pk int
			var col, kind string
			var def any
			if err = rows.Scan(&cid, &col, &kind, &notnull, &def, &pk); err != nil {
				t.Fatal(err)
			}
			if i >= len(cols) {
				t.Fatalf("extra %s column %s", name[1], col)
			}
			want := cols[i]
			typ := map[string]string{"int": "INTEGER", "dateTime": "INTEGER", "string": "TEXT", "bool": "INTEGER"}[want[3]]
			if pk != wantPK[col] {
				t.Fatal("original primary key differs", name[1], col, pk, wantPK)
			}
			if col != want[1] || kind != typ || (notnull == 0) != (want[2] == "true") {
				t.Fatalf("%s column %s %s nullable=%v != original %v", name[1], col, kind, notnull == 0, want[1:])
			}
			i++
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		if i != len(cols) {
			t.Fatalf("missing %s columns: %d != %d", name[1], i, len(cols))
		}
		checked++
	}
	if checked != 11 {
		t.Fatalf("compared %d original tables, want 11", checked)
	}
	for _, index := range regexp.MustCompile(`'(CREATE INDEX [^']+)'`).FindAllStringSubmatch(string(source), -1) {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type='index' AND sql=?`, index[1]).Scan(&count); err != nil || count != 1 {
			t.Fatal("original index missing", index[1], err)
		}
	}
}

func TestFlutterExportRuleSchemaMatchesSourceParse(t *testing.T) {
	for _, rule := range []string{"DOMAIN,example.invalid,DIRECT", "IP-CIDR,192.0.2.0/24,MATCH,src,no-resolve", "DOMAIN-REGEX,^a,b$,DIRECT", "AND,((DOMAIN,a),(NETWORK,TCP)),REJECT", "SUB-RULE,(NETWORK,TCP),sub", "RULE-SET,Local,DIRECT", "MATCH,DIRECT"} {
		row, err := exportFlutterRule(rule)
		if err != nil {
			t.Fatal(err)
		}
		if actual := flutterRule(row); actual != rule {
			t.Fatalf("%s -> %+v -> %s", rule, row, actual)
		}
	}
}
