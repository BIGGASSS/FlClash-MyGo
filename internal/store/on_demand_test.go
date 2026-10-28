package store

import (
	"reflect"
	"strings"
	"testing"
)

func TestOnDemandExactSourceMembershipAndValidation(t *testing.T) {
	cfg := Defaults()
	cfg.ExcludeSSIDs = []string{"Home", " home ", "wifi*", "東京"}
	for ssid, want := range map[string]bool{"Home": true, "home": false, "Home ": false, " home ": true, "wifi123": false, "wifi*": true, "東京": true, "": false} {
		if got := cfg.ExcludesSSID(ssid); got != want {
			t.Fatalf("%q: %v, want %v", ssid, got, want)
		}
	}
	for _, value := range []string{" ", " padded ", "東京", strings.Repeat("a", 32), strings.Repeat("e\u0301", 32), strings.Repeat("👩‍💻", 32), strings.Repeat("🇯🇵", 32)} {
		if err := ValidateSSID(value); err != nil {
			t.Fatal(value, err)
		}
	}
	for _, value := range []string{"", strings.Repeat("a", 33), strings.Repeat("e\u0301", 33), strings.Repeat("👩‍💻", 33), strings.Repeat("🇯🇵", 33), string([]byte{0xff})} {
		if err := ValidateSSID(value); err == nil {
			t.Fatal("accepted invalid edit", value)
		}
	}
}

func TestOnDemandNativeSettingsCloneAndBackupRoundTrip(t *testing.T) {
	s, cfg, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Import can contain old/duplicate values; do not rewrite source identities.
	cfg.ExcludeSSIDs = []string{"Home", "home", " padded ", "東京", "Home", ""}
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_, opened, err := Open(s.Dir)
	if err != nil || !reflect.DeepEqual(opened.ExcludeSSIDs, cfg.ExcludeSSIDs) {
		t.Fatal(opened.ExcludeSSIDs, err)
	}
	cloned := Clone(cfg)
	cloned.ExcludeSSIDs[0] = "Changed"
	if cfg.ExcludeSSIDs[0] != "Home" {
		t.Fatal("clone shares the source list")
	}
	archive, err := s.Backup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	target, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	restored, err := target.Restore(archive)
	if err != nil || !reflect.DeepEqual(restored.ExcludeSSIDs, cfg.ExcludeSSIDs) {
		t.Fatal(restored.ExcludeSSIDs, err)
	}
}

func TestOnDemandFlutterArchiveRoundTrip(t *testing.T) {
	s, cfg, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.ExcludeSSIDs = []string{"Home", "home", " padded ", "東京"}
	for pass := 0; pass < 2; pass++ {
		archive, err := s.FlutterBackup(cfg)
		if err != nil {
			t.Fatal(err)
		}
		target, _, err := Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		restored, err := target.Restore(archive)
		if err != nil || !reflect.DeepEqual(restored.ExcludeSSIDs, cfg.ExcludeSSIDs) {
			t.Fatal(restored.ExcludeSSIDs, cfg.ExcludeSSIDs, err)
		}
		s, cfg = target, restored
		// The second export must not resurrect the original snapshot's list.
		cfg.ExcludeSSIDs = nil
	}
}

func TestOnDemandFlutterImportAndExportOwnsSnapshotField(t *testing.T) {
	cfg := Defaults()
	source := map[string]any{
		"excludeSSIDs":   []any{"Home", "home", " padded ", "東京"},
		"future-setting": map[string]any{"retained": true},
	}
	mapFlutterSettings(&cfg, source)
	want := []string{"Home", "home", " padded ", "東京"}
	if !reflect.DeepEqual(cfg.ExcludeSSIDs, want) {
		t.Fatal(cfg.ExcludeSSIDs)
	}
	out, err := exportFlutterSettings(cfg, source)
	if err != nil || !reflect.DeepEqual(out["excludeSSIDs"], want) || out["future-setting"] == nil {
		t.Fatal(out, err)
	}
	cfg.ExcludeSSIDs = nil
	out, err = exportFlutterSettings(cfg, source)
	if err != nil || !reflect.DeepEqual(out["excludeSSIDs"], []string{}) {
		t.Fatal("removed exclusions resurrected from snapshot", out["excludeSSIDs"], err)
	}
	if !reflect.DeepEqual(source["excludeSSIDs"], []any{"Home", "home", " padded ", "東京"}) {
		t.Fatal("mutated source snapshot")
	}
}
