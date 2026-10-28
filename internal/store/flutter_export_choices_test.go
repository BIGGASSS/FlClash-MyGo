package store

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestFlutterExportNativeChoicePrecedenceAndDormantScript(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	profile, script := NewID(), NewID()
	cfg.Active = profile
	cfg.Selected = map[string]string{"Select": "stale legacy"}
	cfg.Profiles = []Profile{{ID: profile, Name: "Profile", ScriptID: script, Script: "// dormant inline", Selected: map[string]string{"Select": "per-profile"}}}
	cfg.Scripts = []Script{{ID: script, Name: "Shared", Source: "// chosen shared"}}
	if err := AtomicWrite(s.ProfilePath(profile), []byte("rules: ['MATCH,DIRECT']")); err != nil {
		t.Fatal(err)
	}
	files, tables, _ := exportedFlutter(t, s, cfg)
	row := tables["profiles"][0]
	if len(tables["scripts"]) != 2 || String(flutterJSONMap(row["selected_map"])["Select"]) != "per-profile" || string(files["scripts/"+String(row["script_id"])+".js"]) != "// chosen shared" {
		t.Fatal("native precedence changed", tables)
	}
	other, _, _ := Open(t.TempDir())
	restored, err := other.Restore(backupArchive(t, files))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Scripts[0].Source != "// chosen shared" || restored.Profiles[0].ScriptID != restored.Scripts[0].ID || restored.Scripts[1].Source != "// dormant inline" {
		t.Fatal("script content or selection lost")
	}
}

func TestFlutterExportRecoversEarlierImportMetadataWithoutIDMaps(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-v21.zip")
	if err != nil {
		t.Fatal(err)
	}
	s, _, _ := Open(t.TempDir())
	cfg, err := s.Restore(data)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodeFlutterSnapshot(cfg.ImportSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Profiles, snapshot.Scripts, snapshot.Resources, snapshot.Overlays = nil, nil, nil, nil
	cfg.ImportSnapshot, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	_, tables, _ := exportedFlutter(t, s, cfg)
	if String(tables["scripts"][0]["id"]) != "31" || String(tables["scripts"][0]["last_update_time"]) != "1800000000" || String(tables["clash_providers"][0]["label"]) != "Local set" || String(tables["clash_providers"][0]["id"]) != "71" {
		t.Fatal("recoverable earlier-import metadata lost", tables)
	}
}

func TestFlutterExportMissingPreservedResourceFails(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-v21.zip")
	if err != nil {
		t.Fatal(err)
	}
	s, _, _ := Open(t.TempDir())
	cfg, err := s.Restore(data)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Resources = nil
	if _, err = s.FlutterBackup(cfg); err == nil {
		t.Fatal("preserved custom profile exported a missing local rule resource")
	}
}

func TestFlutterExportGlobalRulesWithoutProfiles(t *testing.T) {
	s, cfg, _ := Open(t.TempDir())
	cfg.Overrides["rules"] = []any{"DOMAIN,global.invalid,DIRECT", "MATCH,REJECT"}
	_, tables, _ := exportedFlutter(t, s, cfg)
	rules := []string{}
	for _, row := range orderedRows(tables["rules"]) {
		if row["profile_id"] != nil {
			t.Fatal("global rule unexpectedly owned")
		}
		rules = append(rules, flutterRule(row))
	}
	if !reflect.DeepEqual(rules, []string{"DOMAIN,global.invalid,DIRECT", "MATCH,REJECT"}) {
		t.Fatal(rules)
	}
}

func TestFlutterExportOrderKeysMatchOriginalIntegerSequence(t *testing.T) {
	for n, want := range map[int]string{0: "a0", 9: "a9", 10: "aA", 35: "aZ", 36: "aa", 61: "az", 62: "b00", 63: "b01", 3905: "bzz", 3906: "c000"} {
		if got := flutterOrder(n); got != want {
			t.Fatal(n, got, want)
		}
	}
	// At the archive bounds, order keys still sort identically to entity order.
	for i := 1; i < 20000; i++ {
		if flutterOrder(i-1) >= flutterOrder(i) {
			t.Fatal("order is not monotonic", i)
		}
	}
}
