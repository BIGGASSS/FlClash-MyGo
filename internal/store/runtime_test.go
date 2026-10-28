package store

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestRuntimeAssemblyMatchesPinnedFlutter(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-runtime.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name               string         `json:"name"`
		Raw                map[string]any `json:"raw"`
		Patch              map[string]any `json:"patch"`
		Authentication     []any          `json:"authentication"`
		BypassPrivateRoute bool           `json:"bypassPrivateRoute"`
		AppendSystemDNS    bool           `json:"appendSystemDns"`
		Overrides          map[string]any `json:"overrides"`
		AddedRules         []any          `json:"addedRules"`
		MatchTarget        string         `json:"matchTarget"`
		Expected           map[string]any `json:"expected"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range cases {
		t.Run(fixture.Name, func(t *testing.T) {
			s, cfg, err := Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			mapFlutterSettings(&cfg, map[string]any{"patchClashConfig": fixture.Patch})
			cfg.BypassPrivateRoute, cfg.AppendSystemDNS = fixture.BypassPrivateRoute, fixture.AppendSystemDNS
			cfg.Overrides["authentication"] = fixture.Authentication
			p := Profile{ID: NewID(), Name: fixture.Name, MatchTarget: fixture.MatchTarget}
			override := selectedFlutterOverrides(fixture.Overrides, []string{"sniffer", "dns", "ntp", "tun"})
			if len(fixture.AddedRules) > 0 {
				override["rules"] = fixture.AddedRules
			}
			if len(override) > 0 {
				p.Override, err = YAML(override)
				if err != nil {
					t.Fatal(err)
				}
			}
			cfg.Profiles, cfg.Active = []Profile{p}, p.ID
			yaml, err := YAML(fixture.Raw)
			if err != nil {
				t.Fatal(err)
			}
			source := []byte("# Original subscription\n" + yaml)
			if err := AtomicWrite(s.ProfilePath(p.ID), source); err != nil {
				t.Fatal(err)
			}
			actual, err := s.Effective(cfg)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(actual)
			var normalized map[string]any
			_ = json.Unmarshal(encoded, &normalized)
			keys := map[string]bool{}
			for key := range normalized {
				keys[key] = true
			}
			for key := range fixture.Expected {
				keys[key] = true
			}
			for key := range keys {
				if !reflect.DeepEqual(normalized[key], fixture.Expected[key]) {
					t.Errorf("%s: got %#v; Flutter %#v", key, normalized[key], fixture.Expected[key])
				}
			}
			after, _ := os.ReadFile(s.ProfilePath(p.ID))
			if string(after) != string(source) {
				t.Fatal("effective configuration changed source YAML")
			}
		})
	}
}

func TestLegacyAndSelectedGeneralOverridesUseOriginalDefaults(t *testing.T) {
	for _, patch := range []map[string]any{{}, {"general-override-keys": []any{"unified-delay", "tcp-concurrent", "ipv6", "geodata-loader"}}} {
		cfg := Defaults()
		mapFlutterSettings(&cfg, map[string]any{"patchClashConfig": patch})
		if cfg.Overrides["unified-delay"] != true || cfg.Overrides["tcp-concurrent"] != true || cfg.Overrides["ipv6"] != false || cfg.Overrides["geodata-loader"] != "memconservative" {
			t.Fatal("legacy selected defaults were lost", cfg.Overrides)
		}
	}
	cfg := Defaults()
	mapFlutterSettings(&cfg, map[string]any{"patchClashConfig": map[string]any{"general-override-keys": []any{}}})
	if cfg.Overrides["unified-delay"] != nil || cfg.Overrides["ipv6"] != nil {
		t.Fatal("an explicitly empty selection forced optional settings", cfg.Overrides)
	}
}
