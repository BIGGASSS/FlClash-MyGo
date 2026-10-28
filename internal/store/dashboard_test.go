package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDashboardMigrationMatchesFlutterAndImports(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-dashboard-widgets.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Input, Output []string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, item := range cases {
		if got := NormalizeDashboardWidgets(item.Input); !reflect.DeepEqual(got, item.Output) {
			t.Fatalf("%v -> %v, want %v", item.Input, got, item.Output)
		}
		encoded, _ := json.Marshal(map[string]any{"appSettingProps": map[string]any{"dashboardWidgets": item.Input}})
		var source map[string]any
		if err := json.Unmarshal(encoded, &source); err != nil {
			t.Fatal(err)
		}
		cfg := Defaults()
		mapFlutterSettings(&cfg, source)
		if !reflect.DeepEqual(cfg.DashboardWidgets, item.Output) {
			t.Fatal("Flutter import changed card migration", cfg.DashboardWidgets, item.Output)
		}
	}
}

func TestDashboardOrderSurvivesSettingsAndBackup(t *testing.T) {
	s, cfg, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.DashboardWidgets = []string{"memoryInfo", "currentOutbound", "vpnButton", "profiles"}
	cfg.FloatingNavigation = false
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := Open(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.DashboardWidgets, loaded.DashboardWidgets) {
		t.Fatal("settings lost dashboard order", loaded.DashboardWidgets)
	}
	if loaded.FloatingNavigation {
		t.Fatal("settings lost docked navigation preference")
	}
	data, err := s.Backup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	restored, err := other.Restore(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.DashboardWidgets, restored.DashboardWidgets) {
		t.Fatal("backup lost dashboard order", restored.DashboardWidgets)
	}
	if restored.FloatingNavigation {
		t.Fatal("backup lost docked navigation preference")
	}
}

func TestFlutterNavigationStyleImportAndOldNativeDefault(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		app := map[string]any{"floatingNavigationBar": enabled}
		encoded, _ := json.Marshal(app)
		for _, value := range []any{app, string(encoded)} {
			cfg := Defaults()
			mapFlutterSettings(&cfg, map[string]any{"appSettingProps": value})
			if cfg.FloatingNavigation != enabled {
				t.Fatal("Flutter navigation style was lost", value)
			}
		}
	}
	s, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(filepath.Join(s.Dir, "settings.json"), []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	_, cfg, err := Open(s.Dir)
	if err != nil || !cfg.FloatingNavigation {
		t.Fatal("old native settings lost source default", err)
	}
}
