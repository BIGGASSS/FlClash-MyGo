package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGlassPreferenceSourceDefaultAndLegacyImport(t *testing.T) {
	data, err := os.ReadFile("../theme/testdata/flutter-glass.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct{ Default, LegacyTrue, LegacyFalse string }
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	if Defaults().LiquidGlass != source.Default {
		t.Fatal("native glass default differs from source")
	}
	for _, item := range []struct {
		value any
		want  string
	}{
		{nil, source.Default}, {true, source.LegacyTrue}, {false, source.LegacyFalse},
		{"none", "none"}, {"standard", "standard"}, {"ultra", "ultra"}, {"future-mode", "future-mode"},
	} {
		theme := map[string]any{}
		if item.value != nil {
			theme["liquidGlass"] = item.value
		}
		encoded, _ := json.Marshal(theme)
		for _, value := range []any{theme, string(encoded)} {
			cfg := Defaults()
			mapFlutterSettings(&cfg, map[string]any{"themeProps": value})
			if cfg.LiquidGlass != item.want {
				t.Fatalf("import %v: got %q, want %q", item.value, cfg.LiquidGlass, item.want)
			}
		}
	}
}

func TestGlassPreferenceSettingsNativeAndFlutterRoundTrip(t *testing.T) {
	for _, mode := range []string{"none", "standard", "ultra", "future-mode"} {
		t.Run(mode, func(t *testing.T) {
			s, cfg, err := Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			cfg.LiquidGlass = mode
			cfg.FloatingNavigation = false
			cfg.ImportSnapshot = json.RawMessage(`{"config":{"themeProps":{"liquidGlass":"ultra","unknownGlassField":7}}}`)
			if err := s.Save(cfg); err != nil {
				t.Fatal(err)
			}
			_, saved, err := Open(s.Dir)
			if err != nil || saved.LiquidGlass != mode || saved.FloatingNavigation {
				t.Fatal("settings did not preserve glass/dock choice", saved.LiquidGlass, err)
			}
			clone := Clone(saved)
			if clone.LiquidGlass != mode {
				t.Fatal("clone lost glass preference")
			}
			for _, flutter := range []bool{false, true} {
				var archive []byte
				if flutter {
					archive, err = s.FlutterBackup(saved)
				} else {
					archive, err = s.Backup(saved)
				}
				if err != nil {
					t.Fatal(err)
				}
				other, _, err := Open(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				restored, err := other.Restore(archive)
				if err != nil || restored.LiquidGlass != mode || restored.FloatingNavigation {
					t.Fatalf("round trip Flutter=%t: glass=%q floating=%t error=%v", flutter, restored.LiquidGlass, restored.FloatingNavigation, err)
				}
			}
			projected, err := exportFlutterSettings(saved, map[string]any{"themeProps": map[string]any{"liquidGlass": "ultra", "unknownGlassField": 7}})
			if err != nil {
				t.Fatal(err)
			}
			theme := flutterJSONMap(projected["themeProps"])
			if theme["liquidGlass"] != mode || theme["unknownGlassField"] != 7 {
				t.Fatal("projection overwrote native choice or unknown metadata", theme)
			}
		})
	}
}

func TestGlassOldNativeSettingsRemainOff(t *testing.T) {
	s, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(filepath.Join(s.Dir, "settings.json"), []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	_, cfg, err := Open(s.Dir)
	if err != nil || cfg.LiquidGlass != "none" {
		t.Fatal("old native settings enabled glass", cfg.LiquidGlass, err)
	}
}
