package app

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

func TestRuleSyncCapturesRunningProxyAuthenticationAndUserAgent(t *testing.T) {
	requests := make(chan *http.Request, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(context.Background())
		_, _ = io.WriteString(w, "payload: ['DOMAIN,new.invalid,DIRECT']\n")
	}))
	defer proxy.Close()
	a, p := remoteRuleApp(t, "http://rule-library.invalid/set")
	a.status = "running"
	parsed, _ := url.Parse(proxy.URL)
	port, _ := strconv.Atoi(parsed.Port())
	a.cfg.Overrides["mixed-port"] = port
	a.cfg.Overrides["authentication"] = []any{"fixture:secret", "ignored:credential"}
	a.cfg.Overrides["global-ua"] = "captured-rule-agent"
	var calls atomic.Int32
	ruleSyncCore(t, a, map[string]string{"behavior": "classical", "format": "yaml"}, "", &calls)
	a.updateDetailProvider(p)
	// The worker must not read these live request preferences.
	a.cfg.Overrides["mixed-port"] = 1
	a.cfg.Overrides["global-ua"] = "replacement-agent"
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	select {
	case req := <-requests:
		credential := "Basic " + base64.StdEncoding.EncodeToString([]byte("fixture:secret"))
		if req.URL.Host != "rule-library.invalid" || req.Header.Get("Proxy-Authorization") != credential || req.UserAgent() != "captured-rule-agent" || a.err != "" {
			t.Fatal("request did not use captured client", req.URL, req.Header, a.err)
		}
	default:
		t.Fatal("app-managed sync did not use the configured HTTP proxy", a.err)
	}
}

func TestRuleSyncDownloadPoolIsBoundedAndBulkShapesApplyOnce(t *testing.T) {
	var active, peak atomic.Int32
	started, release := make(chan struct{}, 8), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := active.Add(1)
		for previous := peak.Load(); n > previous && !peak.CompareAndSwap(previous, n); previous = peak.Load() {
		}
		started <- struct{}{}
		<-release
		_, _ = io.WriteString(w, "payload: [new.invalid]\n")
		active.Add(-1)
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	a, first := remoteRuleApp(t, server.URL)
	providers := []Provider{first}
	for i := 1; i < 8; i++ {
		id, name := store.NewID(), "Set"+strconv.Itoa(i)
		path := a.store.ResourcePath(id)
		_ = store.AtomicWrite(path, []byte("payload: ['DOMAIN,old.invalid,DIRECT']\n"))
		a.cfg.Resources = append(a.cfg.Resources, id)
		store.Map(a.cfg.Overrides, "rule-providers")[name] = map[string]any{"type": "file", "path": "flclash-resource://" + id, "url": server.URL, "behavior": "classical", "format": "yaml"}
		providers = append(providers, Provider{Name: name, Type: "Rule", Vehicle: "File", Path: path})
	}
	a.sample.Providers = providers
	var calls atomic.Int32
	ruleSyncCore(t, a, map[string]string{"behavior": "domain", "format": "yaml"}, "", &calls)
	a.updateDetailProviders()
	for range 4 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("worker pool did not fill")
		}
	}
	if active.Load() != 4 || peak.Load() != 4 {
		t.Fatal("provider downloads are unbounded", active.Load(), peak.Load())
	}
	close(release)
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	if a.err != "" || a.engine.Revision() != 1 || len(a.cfg.Resources) != 24 || calls.Load() != 0 || peak.Load() > 4 {
		t.Fatal("bulk shape results invalidated each other", a.err, a.engine.Revision(), len(a.cfg.Resources), calls.Load(), peak.Load())
	}
	raw, err := a.store.Effective(a.cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range providers {
		def := store.Map(store.Map(raw, "rule-providers"), p.Name)
		if def["behavior"] != "domain" || def["path"] == p.Path {
			t.Fatal("bulk provider shape was lost", p.Name, def)
		}
		old, _ := os.ReadFile(p.Path)
		if !strings.Contains(string(old), "old.invalid") {
			t.Fatal("bulk sync changed old working resource", p.Name)
		}
	}
}

func TestProviderResourceToolbarLoopCollectsFailures(t *testing.T) {
	a := populatedApp(t)
	a.safe = false
	a.sample.Providers = []Provider{{Name: "One", Type: "Proxy", Vehicle: "HTTP"}, {Name: "Two", Type: "Rule", Vehicle: "HTTP"}}
	a.engine = providerTestClient(t, func(string, json.RawMessage) any { return "fixture failure" })
	for _, p := range a.sample.Providers {
		a.updateDetailProvider(p)
	}
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	if !strings.Contains(a.err, "One: fixture failure") || !strings.Contains(a.err, "Two: fixture failure") || a.providerState().batch != nil {
		t.Fatal("resource toolbar lost an error or retained its completed batch", a.err)
	}
}

func TestImportedHTTPRuleLibraryMigratesToAppManagedFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "payload: ['DOMAIN-KEYWORD,new.invalid']\n")
	}))
	defer server.Close()
	a, p := remoteRuleApp(t, server.URL)
	delete(a.cfg.Overrides, "rule-providers")
	a.cfg.Resources = nil
	profile := a.store.ActiveProfile(a.cfg)
	profile.Type = "custom"
	profile.Override = "rule-providers:\n  Library: {type: http, url: '" + server.URL + "', behavior: classical, format: yaml}\n"
	snapshot := map[string]any{"tables": map[string]any{"clash_providers": []any{map[string]any{"id": "42", "kind": "rule", "label": "Library", "url": server.URL}}}}
	a.cfg.ImportSnapshot, _ = json.Marshal(snapshot)
	digest := md5.Sum([]byte(server.URL))
	p.Path = filepath.Join(a.store.Dir, "rules", hex.EncodeToString(digest[:]))
	p.Vehicle = "HTTP"
	if err := os.MkdirAll(filepath.Dir(p.Path), 0700); err != nil {
		t.Fatal(err)
	}
	_ = store.AtomicWrite(p.Path, []byte("old cache"))
	a.sample.Providers = []Provider{p}
	var calls atomic.Int32
	ruleSyncCore(t, a, map[string]string{"behavior": "classical", "format": "yaml"}, "", &calls)
	a.updateDetailProvider(p)
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	if a.err != "" || calls.Load() != 0 || len(a.cfg.Resources) != 1 || a.engine.Revision() != 1 {
		t.Fatal("library import did not migrate", a.err, calls.Load(), a.cfg.Resources)
	}
	resource, ok := a.store.RemoteRuleResource(a.cfg, a.store.ResourcePath(a.cfg.Resources[0]))
	if !ok || resource.NeedsFileDefinition || resource.URL != server.URL {
		t.Fatal("migrated resource is not app-managed", resource, ok)
	}
	old, _ := os.ReadFile(p.Path)
	if string(old) != "old cache" {
		t.Fatal("migration overwrote old working HTTP cache")
	}
}

func TestRuleShapeSaveFailureKeepsWorkingFilesAndPreferences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "payload: [new.invalid]\n") }))
	defer server.Close()
	a, p := remoteRuleApp(t, server.URL)
	before := store.Clone(a.cfg)
	if err := os.Mkdir(filepath.Join(a.store.Dir, "settings.json"), 0700); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	ruleSyncCore(t, a, map[string]string{"behavior": "domain", "format": "yaml"}, "", &calls)
	a.updateDetailProvider(p)
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	if !strings.Contains(a.err, "Library: ") || !sameResourceIntent(a.cfg, before) || a.engine.Revision() != 0 {
		t.Fatal("failed metadata save changed the current config", a.err, a.engine.Revision())
	}
	files, err := os.ReadDir(filepath.Join(a.store.Dir, "resources"))
	if err != nil || len(files) != 1 {
		t.Fatal("failed metadata save retained unreferenced new resources", files, err)
	}
	body, _ := os.ReadFile(p.Path)
	if !strings.Contains(string(body), "old.invalid") {
		t.Fatal("failed metadata save changed last working bytes")
	}
}

func TestRuleSyncCancelsSupersededDownload(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	a, p := remoteRuleApp(t, server.URL)
	var calls atomic.Int32
	ruleSyncCore(t, a, map[string]string{"behavior": "classical", "format": "yaml"}, "", &calls)
	a.updateDetailProvider(p)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("download did not start")
	}
	a.engine.mu.Lock()
	a.engine.revision++
	a.engine.mu.Unlock()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("superseded network request was not canceled")
	}
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	if calls.Load() != 0 || a.err != "" {
		t.Fatal("canceled sync mutated current state", calls.Load(), a.err)
	}
}

func TestRuleSyncRetainedCompiledLibraryKeepsBinaryExact(t *testing.T) {
	source := []byte("payload: [new.invalid]\n")
	compiled := []byte{0x28, 0xb5, 0x2f, 0xfd, 0, 0xff, 0xfe, 0}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(source) }))
	defer server.Close()
	a, p := remoteRuleApp(t, server.URL)
	a.cfg.Resources = nil
	delete(a.cfg.Overrides, "rule-providers")
	snapshot := map[string]any{"tables": map[string]any{"clash_providers": []any{map[string]any{"id": "42", "kind": "rule", "label": "Library", "url": server.URL, "behavior": "domain", "format": "yaml", "unknown": "preserve"}}}}
	a.cfg.ImportSnapshot, _ = json.Marshal(snapshot)
	digest := md5.Sum([]byte("42@" + server.URL))
	sourcePath := filepath.Join(a.store.Dir, "providers", "rules", hex.EncodeToString(digest[:]))
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0700); err != nil {
		t.Fatal(err)
	}
	_ = store.AtomicWrite(sourcePath, []byte("payload: [old.invalid]\n"))
	p.Path = sourcePath + ".mrs"
	_ = store.AtomicWrite(p.Path, []byte("old compiled"))
	a.sample.Providers = []Provider{p}
	var sideLoaded atomic.Bool
	a.engine = providerTestClient(t, func(method string, args json.RawMessage) any {
		var params map[string]string
		_ = json.Unmarshal(args, &params)
		switch method {
		case "compileRuleSet":
			_ = store.AtomicWrite(filepath.Join(a.store.Dir, "providers", "rules", params["name"])+".mrs", compiled)
			return map[string]string{"behavior": "domain", "format": "yaml"}
		case "validateMyGoConfig":
			return ""
		case "updateExternalProviderAtPath":
			body, _ := os.ReadFile(p.Path)
			sideLoaded.Store(bytes.Equal(body, compiled) && params["provider-path"] == p.Path)
			return ""
		default:
			return "unexpected RPC"
		}
	})
	a.updateDetailProvider(p)
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	got, _ := os.ReadFile(p.Path)
	raw, _ := os.ReadFile(sourcePath)
	if !sideLoaded.Load() || !bytes.Equal(got, compiled) || !bytes.Equal(raw, source) || a.err != "" || !strings.Contains(string(a.cfg.ImportSnapshot), "preserve") {
		t.Fatal("compiled/source content changed", got, string(raw), a.err)
	}
}
