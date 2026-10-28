package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFlutterBackupReferenceFixture generates synthetic input for the ORIGINAL
// Flutter restore harness, not evidence that a native reimport works. To retain:
//
// FLCLASH_FLUTTER_BACKUP_FIXTURE=$PWD/build/backup-reference \
// CGO_ENABLED=0 go test ./internal/store -run '^TestFlutterBackupReferenceFixture$' -count=1
//
// Copy tool/reference/port_backup_test.dart to the pinned checkout's test/ and
// run flutter test --no-pub --dart-define=SAFE_MODE=true test/port_backup_test.dart
// with FLCLASH_FLUTTER_BACKUP_FIXTURE pointing at the same directory.
func TestFlutterBackupReferenceFixture(t *testing.T) {
	s, cfg, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const source = "# Synthetic reference: preserve comments and unknown YAML.\nproxies: []\nrules: ['MATCH,DIRECT']\nunknown-setting: keep\n"
	const shared = "function main(config) { config['synthetic-shared'] = true; return config; }\n"
	const inline = "function main(config) { config['synthetic-inline'] = true; return config; }\n"
	const dormant = "// synthetic dormant inline buffer; shared script takes precedence\n"
	const resource = "payload: ['DOMAIN,local.invalid']\n"
	binary := []byte{0x28, 0xb5, 0x2f, 0xfd, 0, 0xff, 0xfe, '\n', 0}
	ids := []string{
		"00000000000000000000000000000001",
		"00000000000000000000000000000002",
		"00000000000000000000000000000003",
		"00000000000000000000000000000004",
		"00000000000000000000000000000005",
		"00000000000000000000000000000006",
		"00000000000000000000000000000007",
	}
	plain, scripted, inlined, custom, script, local, mrs := ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6]
	cfg.Active = custom
	cfg.Selected = map[string]string{"Select": "stale legacy selection"}
	cfg.Theme, cfg.Scale, cfg.FloatingNavigation = "dark", 1.2, false
	cfg.SystemProxy, cfg.AutoStart, cfg.LaunchAtLogin = false, false, false
	cfg.Overrides["tun"] = map[string]any{"enable": false}
	cfg.ExcludeSSIDs = []string{"synthetic-reference-wifi"}
	cfg.Hotkeys["start"] = "Ctrl+Alt+S"
	cfg.WebDAVURL, cfg.WebDAVUser, cfg.WebDAVPassword = "https://example.invalid/dav", "synthetic", "synthetic-not-a-secret"
	cfg.Scripts = []Script{{ID: script, Name: "Shared", Source: shared}}
	no := false
	cfg.Profiles = []Profile{
		{ID: plain, Name: "Source", Type: "url", URL: "https://example.invalid/sub", AutoUpdate: &no, IntervalMS: 1234567, Updated: time.Unix(1700000000, 0), Upload: 1234567890123, Download: 2345, Total: 9000000000000, Expire: 1900000000, Group: "Select", Selected: map[string]string{"Select": "DIRECT"}, Unfold: []string{"Select"}, Override: "rules: ['DOMAIN,added.invalid,DIRECT', 'IP-CIDR,192.0.2.0/24,DIRECT,src,no-resolve']\n"},
		{ID: scripted, Name: "Scripted", ScriptID: script, Script: dormant, Selected: map[string]string{"Select": "REJECT"}, Group: "Select"},
		{ID: inlined, Name: "Inline", Script: inline, Selected: map[string]string{"Select": "DIRECT"}},
		{ID: custom, Name: "Custom", Type: "custom", Dialers: map[string]string{"Node": "DIRECT"}, Override: "dns:\n  ipv6: true\n", Selected: map[string]string{"Select": "Node"}, Group: "Select", Unfold: []string{"Select"}},
	}
	cfg.Resources = []string{local, mrs}
	cfg.LocalProxies = []map[string]any{{"name": "Node", "type": "socks5", "server": "192.0.2.1", "port": 1080}}
	for _, id := range []string{plain, scripted, inlined} {
		if err := AtomicWrite(s.ProfilePath(id), []byte(source)); err != nil {
			t.Fatal(err)
		}
	}
	customBody := "proxies: []\nproxy-groups:\n  - name: Select\n    type: select\n    proxies: [Node, DIRECT]\n    use: [Source]\n    default-selected: Node\nproxy-providers:\n  Source:\n    type: file\n    path: " + profileRef + plain + "\n    filter: synthetic\nrule-providers:\n  Local:\n    type: file\n    behavior: classical\n    format: yaml\n    path: " + resourceRef + local + "\n  Binary:\n    type: file\n    behavior: domain\n    format: mrs\n    path: " + resourceRef + mrs + "\nrules: ['RULE-SET,Local,Select', 'RULE-SET,Binary,Select', 'DOMAIN-REGEX,^a,b$,DIRECT', 'MATCH,Select']\n"
	for path, body := range map[string][]byte{s.ProfilePath(custom): []byte(customBody), s.ResourcePath(local): []byte(resource), s.ResourcePath(mrs): binary} {
		if err := AtomicWrite(path, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(s.Dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	archive, err := s.FlutterBackup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	files, err := readBackupFiles(archive)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := s.readFlutterSQLite(files["database.sqlite"])
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{"profiles": 4, "scripts": 3, "rules": 6, "disabled_rules": 0, "proxy_groups": 1, "clash_providers": 2, "custom_proxies": 1, "proxy_dialers": 1}
	for table, count := range counts {
		if len(tables[table]) != count {
			t.Fatalf("%s: got %d rows, want %d", table, len(tables[table]), count)
		}
	}
	if len(files) != 11 || !bytes.Equal(files["profiles/1.yaml"], []byte(source)) {
		t.Fatal("missing files or modified source", len(files))
	}
	after, err := os.ReadFile(filepath.Join(s.Dir, "settings.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("export mutated settings", err)
	}
	for _, id := range []string{plain, scripted, inlined} {
		body, err := os.ReadFile(s.ProfilePath(id))
		if err != nil || string(body) != source {
			t.Fatal("export mutated source", err)
		}
	}
	// Expected content is defined above independently of the exported ZIP. The
	// Dart harness also checks provider paths using original BackupEntries.
	expectedFiles := map[string][]byte{}
	for name, body := range files {
		if name != "config.json" && name != "database.sqlite" {
			expectedFiles[name] = body
		}
	}
	hash := sha256.Sum256(archive)
	manifest := map[string]any{
		"synthetic": true, "archive": "native-export.zip", "sha256": hex.EncodeToString(hash[:]),
		"rowCounts": counts, "files": expectedFiles,
		"source": source, "sharedScript": shared, "inlineScript": inline, "dormantScript": dormant,
		"localResource": resource, "binaryResource": binary,
	}
	out := os.Getenv("FLCLASH_FLUTTER_BACKUP_FIXTURE")
	if out == "" {
		return
	}
	if err := os.MkdirAll(out, 0700); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(filepath.Join(out, "native-export.zip"), archive); err != nil {
		t.Fatal(err)
	}
	metadata, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(filepath.Join(out, "expected.json"), append(metadata, '\n')); err != nil {
		t.Fatal(err)
	}
	t.Logf("synthetic Flutter restore fixture: %s (%d bytes, sha256 %x)", out, len(archive), hash)
}
