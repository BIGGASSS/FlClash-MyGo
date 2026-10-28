package store

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFlutterHotkeysSurviveBackupAndRestore(t *testing.T) {
	s, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := map[string]any{"appSettingProps": map[string]any{"serviceOrder": []any{"spotify", "google"}, "disabledServices": []any{"youtube"}}, "hotKeyActions": []any{
		map[string]any{"action": "start", "key": 0x70013, "modifiers": []any{"shift", "control"}},
		map[string]any{"action": "ruleMode", "key": 0x7003a, "modifiers": []any{"shift"}},
		map[string]any{"action": "view", "key": 0x7002c, "modifiers": []any{"meta", "alt"}},
		map[string]any{"action": "copyEnv", "key": 0x70058, "modifiers": []any{"control"}},
		map[string]any{"action": "exit", "key": 0x70014, "modifiers": []any{"fn", "control"}},
		map[string]any{"action": "delayTest", "key": 0xc00cd, "modifiers": []any{"control"}},
	}}
	b, _ := json.Marshal(source)
	cfg, err := s.Restore(backupArchive(t, map[string][]byte{"config.json": b}))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.ServiceOrder, []string{"spotify", "google"}) || !reflect.DeepEqual(cfg.DisabledServices, []string{"youtube"}) {
		t.Fatal("Flutter service preferences not imported", cfg.ServiceOrder, cfg.DisabledServices)
	}
	want := map[string]string{"start": "Ctrl+Shift+P", "ruleMode": "Shift+F1", "view": "Super+Alt+SPACE", "copyEnv": "Ctrl+NUMENTER", "exit": "Fn+Ctrl+Q", "delayTest": "Ctrl+MEDIAPLAYPAUSE"}
	for key, chord := range cfg.Hotkeys {
		if chord == "" {
			delete(cfg.Hotkeys, key)
		}
	}
	if !reflect.DeepEqual(cfg.Hotkeys, want) {
		t.Fatalf("got %v, want %v", cfg.Hotkeys, want)
	}
	backup, err := s.Backup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	target, _, _ := Open(t.TempDir())
	roundTrip, err := target.Restore(backup)
	if err != nil || !reflect.DeepEqual(roundTrip.Hotkeys, want) {
		t.Fatal("bindings lost on native round trip", roundTrip.Hotkeys, err)
	}
}

func TestLegacyNativeHotkeysMigrate(t *testing.T) {
	s, cfg, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Hotkeys = map[string]string{"toggle": "Ctrl+P", "show": "Alt+V", "systemProxy": "Shift+F1", "view": "Super+V"}
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := Open(s.Dir)
	if err != nil || !reflect.DeepEqual(loaded.Hotkeys, map[string]string{"start": "Ctrl+P", "view": "Super+V", "proxy": "Shift+F1"}) {
		t.Fatal(loaded.Hotkeys, err)
	}
}
