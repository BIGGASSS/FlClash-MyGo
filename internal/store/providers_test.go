package store

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionProvidersUsePerProfileAndPerNameCaches(t *testing.T) {
	s, cfg, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := Profile{ID: NewID(), Name: "Providers"}
	cfg.Active, cfg.Profiles = p.ID, []Profile{p}
	outside := filepath.Join(t.TempDir(), "unchanged")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	url := "https://example.com/provider.yaml"
	raw := map[string]any{
		"rules": []any{"MATCH,DIRECT"},
		"proxy-providers": map[string]any{
			"A":      map[string]any{"type": "http", "url": url, "path": outside, "header": map[string]any{"X-User": []any{"A"}}},
			"B":      map[string]any{"type": "http", "url": url, "path": outside, "header": map[string]any{"X-User": []any{"B"}}},
			"Inline": map[string]any{"type": "inline", "payload": []any{}},
		},
		"rule-providers": map[string]any{"File": map[string]any{"type": "file", "path": outside, "behavior": "domain"}},
	}
	yaml, _ := YAML(raw)
	if err := AtomicWrite(s.ProfilePath(p.ID), []byte(yaml)); err != nil {
		t.Fatal(err)
	}
	root := s.providerRoot(p.ID, "proxies")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, providerHash(url))
	if err := AtomicWrite(legacy, []byte("cached")); err != nil {
		t.Fatal(err)
	}
	effective, err := s.Effective(cfg)
	if err != nil {
		t.Fatal(err)
	}
	providers := Map(effective, "proxy-providers")
	a, b := String(Map(providers, "A")["path"]), String(Map(providers, "B")["path"])
	if a == b || !strings.HasPrefix(a, root+string(filepath.Separator)) || filepath.Base(a) != providerHash("A@"+url) {
		t.Fatal("providers with different names shared a cache", a, b)
	}
	for _, path := range []string{a, b} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "cached" {
			t.Fatal("legacy URL-only cache not migrated", path, err)
		}
	}
	if Map(providers, "Inline")["path"] != nil {
		t.Fatal("inline provider received a file path")
	}
	if file := String(Map(Map(effective, "rule-providers"), "File")["path"]); file != filepath.Join(s.providerRoot(p.ID, "rules"), providerHash("rule-providers/File")) {
		t.Fatal("file provider did not use the confined path", file)
	}
	if data, _ := os.ReadFile(outside); string(data) != "keep" {
		t.Fatal("subscription changed a path outside its provider cache")
	}
	if data, _ := os.ReadFile(s.ProfilePath(p.ID)); !bytes.Equal(data, []byte(yaml)) {
		t.Fatal("provider confinement rewrote subscription bytes")
	}
	if err := AtomicWrite(a, []byte("newer")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Effective(cfg); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(a); string(data) != "newer" {
		t.Fatal("migration overwrote a newer cache")
	}
}

func TestRemoveProfileDeletesOnlyItsProviderCache(t *testing.T) {
	s, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	one, two := NewID(), NewID()
	for _, id := range []string{one, two} {
		if err := os.MkdirAll(filepath.Join(s.Dir, "profiles", "providers", id, "proxies"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := AtomicWrite(s.ProfilePath(id), []byte("rules: []\n")); err != nil {
			t.Fatal(err)
		}
		if err := AtomicWrite(filepath.Join(s.Dir, "profiles", "providers", id, "proxies", "cache"), []byte("cached")); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RemoveProfile(one); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "profiles", "providers", one)); !os.IsNotExist(err) {
		t.Fatal("deleted profile cache remains", err)
	}
	if _, err := os.Stat(s.ProfilePath(one)); !os.IsNotExist(err) {
		t.Fatal("deleted profile remains", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "profiles", "providers", two, "proxies", "cache")); err != nil {
		t.Fatal("another profile cache removed", err)
	}
	if err := s.RemoveProfile("../providers"); err == nil {
		t.Fatal("invalid profile identity accepted")
	}
}
