package store

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndEffectiveConfig(t *testing.T) {
	s, cfg, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	p := Profile{ID: NewID(), Name: "Example", Override: "dns:\n  ipv6: true\n"}
	cfg.Profiles = []Profile{p}
	cfg.Active = p.ID
	if e = AtomicWrite(s.ProfilePath(p.ID), []byte("mixed-port: 1234\ndns: {enable: true, nameserver: [https://example.com/dns-query]}\nunknown-setting: keep\n")); e != nil {
		t.Fatal(e)
	}
	if e = s.Save(cfg); e != nil {
		t.Fatal(e)
	}
	_, loaded, e := Open(s.Dir)
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.Effective(loaded)
	if e != nil {
		t.Fatal(e)
	}
	if Int(m["mixed-port"]) != 7890 || m["unknown-setting"] != "keep" {
		t.Fatal(m)
	}
	dns := m["dns"].(map[string]any)
	if dns["enable"] != true || dns["ipv6"] != true {
		t.Fatal(dns)
	}
	if path := s.ProfilePath("../../outside"); path != "" {
		t.Fatal(path)
	}
}
func TestBackupRestoreAndRejectTraversal(t *testing.T) {
	s, cfg, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	p := Profile{ID: NewID(), Name: "Example"}
	cfg.Profiles = []Profile{p}
	cfg.Active = p.ID
	content := []byte("rules: [MATCH,DIRECT]\n")
	if e = AtomicWrite(s.ProfilePath(p.ID), content); e != nil {
		t.Fatal(e)
	}
	b, e := s.Backup(cfg)
	if e != nil {
		t.Fatal(e)
	}
	target, _, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	restored, e := target.Restore(b)
	if e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(target.ProfilePath(restored.Active))
	if e != nil || !bytes.Equal(content, got) {
		t.Fatalf("%q %v", got, e)
	}
	if restored.Active == cfg.Active {
		t.Fatal("restore reused profile identities")
	}
	var bad bytes.Buffer
	z := zip.NewWriter(&bad)
	w, _ := z.Create("../settings.json")
	_, _ = w.Write([]byte("{}"))
	_ = z.Close()
	if _, e = target.Restore(bad.Bytes()); e == nil {
		t.Fatal("path traversal accepted")
	}
	if _, e = os.Stat(filepath.Join(filepath.Dir(target.Dir), "settings.json")); e == nil {
		t.Fatal("wrote outside data directory")
	}
}
func TestDownloadKeepsSubscriptionMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "FlClash/MyGo" {
			t.Error("missing subscription user agent")
		}
		w.Header().Set("Subscription-Userinfo", "upload=1024; download=4096; total=1048576; expire=1900000000")
		_, _ = w.Write([]byte("rules: [MATCH,DIRECT]"))
	}))
	defer srv.Close()
	b, h, e := Download(srv.Client(), srv.URL, "")
	if e != nil || len(b) == 0 {
		t.Fatal(e)
	}
	var p Profile
	p.Subscription(h)
	if p.Upload != 1024 || p.Download != 4096 || p.Total != 1048576 {
		t.Fatal(p)
	}
}
func TestMalformedSettingsAreReported(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e := Open(dir); e == nil {
		t.Fatal("malformed settings were silently reset")
	}
}
func TestFutureSettingsVersion(t *testing.T) {
	dir := t.TempDir()
	b, _ := json.Marshal(map[string]any{"version": 999})
	_ = os.WriteFile(filepath.Join(dir, "settings.json"), b, 0600)
	if _, _, e := Open(dir); e == nil {
		t.Fatal("future settings version accepted")
	}
}

func TestScriptRunsBeforeApplicationOverrides(t *testing.T) {
	s, cfg, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Profiles = []Profile{{ID: NewID(), Name: "Transform"}}
	cfg.Active = cfg.Profiles[0].ID
	if err = AtomicWrite(s.ProfilePath(cfg.Active), []byte("mixed-port: 1234\nrules: [\"MATCH,DIRECT\"]\n")); err != nil {
		t.Fatal(err)
	}
	raw, err := s.EffectiveTransform(cfg, func(source map[string]any) (map[string]any, error) {
		if Int(source["mixed-port"]) != 1234 {
			t.Fatal("script received application overrides")
		}
		source["mixed-port"] = 4321
		source["extra-field"] = "script"
		return source, nil
	})
	if err != nil || Int(raw["mixed-port"]) != 7890 || raw["extra-field"] != "script" {
		t.Fatal(raw, err)
	}
}
func TestLocalProxyLibraryOnlyUpdatesCustomProfiles(t *testing.T) {
	s, cfg, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := Profile{ID: NewID(), Name: "Custom", Type: "custom", Dialers: map[string]string{"Local": "DIRECT"}}
	cfg.Profiles = []Profile{p}
	cfg.Active = p.ID
	if err = AtomicWrite(s.ProfilePath(p.ID), []byte("proxies: []\nproxy-groups: [{name: Select, type: select, proxies: [Local]}]\nrules: [\"MATCH,Select\"]\n")); err != nil {
		t.Fatal(err)
	}
	cfg.LocalProxies = []map[string]any{{"name": "Local", "type": "ss", "server": "192.0.2.1"}, {"name": "Unused", "type": "ss", "server": "192.0.2.2"}}
	raw, err := s.Effective(cfg)
	if err != nil {
		t.Fatal(err)
	}
	proxies := raw["proxies"].([]any)
	if len(proxies) != 1 || proxies[0].(map[string]any)["dialer-proxy"] != "DIRECT" {
		t.Fatal(proxies)
	}
	if cfg.LocalProxies[0]["dialer-proxy"] != nil {
		t.Fatal("effective config mutated library")
	}
	cfg.Profiles[0].Type = "file"
	raw, err = s.Effective(cfg)
	if err != nil || len(raw["proxies"].([]any)) != 0 {
		t.Fatal("app library leaked into normal subscription", raw, err)
	}
}
