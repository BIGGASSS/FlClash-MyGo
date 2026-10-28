package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

func TestEngineLatestIntentAndFailedConfig(t *testing.T) {
	path := os.Getenv("FLCLASH_TEST_CORE")
	if path == "" {
		t.Skip("requires the built core")
	}
	s, cfg, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Overrides["mixed-port"] = 0
	cfg.Overrides["dns"] = map[string]any{"enable": false}
	cfg.SystemProxy = false
	p := store.Profile{ID: store.NewID(), Name: "Keep"}
	cfg.Profiles = []store.Profile{p}
	cfg.Active = p.ID
	if err = store.AtomicWrite(s.ProfilePath(p.ID), []byte("proxies: [{name: Keep, type: direct}]\nrules: ['MATCH,Keep']\n")); err != nil {
		t.Fatal(err)
	}
	resourceID := store.NewID()
	if err := store.AtomicWrite(s.ResourcePath(resourceID), []byte("proxies: [{name: Resource, type: direct}]\n")); err != nil {
		t.Fatal(err)
	}
	cfg.Resources = []string{resourceID}
	cfg.Overrides["proxy-providers"] = map[string]any{"Portable": map[string]any{"type": "file", "path": "flclash-resource://" + resourceID, "health-check": map[string]any{"enable": false}}}
	cfg.Overrides["proxy-groups"] = []any{map[string]any{"name": "From resource", "type": "select", "use": []any{"Portable"}}}
	e := NewEngine(s, path, io.Discard)
	type event struct {
		status   string
		err      error
		revision uint64
	}
	events := make(chan event, 64)
	e.Status = func(status string, err error, revision uint64) { events <- event{status, err, revision} }
	e.Run()
	defer e.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	await := func(revision uint64, status string, wantError bool) {
		t.Helper()
		for {
			select {
			case event := <-events:
				if event.revision != revision || event.status == "connecting" {
					continue
				}
				if event.status != status || (event.err != nil) != wantError {
					t.Fatalf("status %s error %v, wanted %s error=%v", event.status, event.err, status, wantError)
				}
				return
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
	}
	e.Reconcile(cfg, true)
	await(e.Revision(), "running", false)
	var providers []Provider
	if err := e.Call("getExternalProviders", nil, &providers); err != nil || len(providers) != 1 || providers[0].Count != 1 || providers[0].Path != s.ResourcePath(resourceID) {
		t.Fatal("portable provider was not accepted by the core's path checks", providers, err)
	}
	for _, method := range []string{"sideLoadExternalProvider", "updateExternalProviderAtPath"} {
		if err := e.Message(method, map[string]string{"providerName": "Portable", "provider-path": s.ResourcePath(store.NewID()), "data": "proxies: []"}); err == nil {
			t.Fatal("core accepted stale provider identity", method)
		}
	}
	if err := e.Message("updateExternalProviderAtPath", map[string]string{"providerName": "Portable", "provider-path": s.ResourcePath(resourceID)}); err != nil {
		t.Fatal("core rejected matching provider identity", err)
	}
	var collected bool
	if err := e.Call("forceGc", nil, &collected); err != nil || !collected {
		t.Fatal("memory release was not acknowledged", collected, err)
	}
	var memory Memory
	if err := e.Call("getMemoryStats", nil, &memory); err != nil || memory.Heap == 0 {
		t.Fatal("memory breakdown missing after release", memory, err)
	}
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer endpoint.Close()
	for _, probe := range []struct {
		name, path string
		timeout    int
		want       int
	}{{"Keep", "", 1000, 1}, {"Keep", "/slow", 20, -1}, {"REJECT", "", 1000, -2}} {
		var result *NodeDelay
		err := e.Call("asyncTestDelay", map[string]any{"proxy-name": probe.name, "test-url": endpoint.URL + probe.path, "timeout": probe.timeout}, &result)
		if err != nil || result == nil || result.Name != probe.name || result.URL != endpoint.URL+probe.path {
			t.Fatal("delay RPC lost its value or target identity", result, err)
		}
		if probe.want > 0 && result.Value <= 0 || probe.want < 0 && result.Value != probe.want {
			t.Fatal("delay RPC confused success, timeout and failure", probe, result)
		}
	}
	e.mu.Lock()
	previousClient := e.client
	e.mu.Unlock()
	e.Restart()
	e.Restart()
	e.Reconcile(cfg, false)
	await(e.Revision(), "stopped", false)
	e.mu.Lock()
	restartedClient := e.client
	e.mu.Unlock()
	if restartedClient == previousClient || !e.Connected() || e.Restarting() {
		t.Fatal("restart did not replace ownership and retain the latest stopped intent")
	}
	select {
	case <-previousClient.Done():
	default:
		t.Fatal("restart left previous owner connected")
	}
	if err := e.Call("getProxies", nil, &ProxyData{}); err != nil {
		t.Fatal("restart lost tools while listeners stopped", err)
	}
	e.Reconcile(cfg, true)
	await(e.Revision(), "running", false)
	e.mu.Lock()
	countsBeforeFailure := e.counts
	e.mu.Unlock()
	if countsBeforeFailure == nil || countsBeforeFailure.Proxies != 1 || countsBeforeFailure.Groups != 1 {
		t.Fatal("applied config counts were not retained", countsBeforeFailure)
	}
	invalid := store.Clone(cfg)
	invalid.Overrides["mode"] = "invalid-mode"
	e.Reconcile(invalid, true)
	await(e.Revision(), "running", true)
	var nodes ProxyData
	if err = e.Call("getProxies", nil, &nodes); err != nil {
		t.Fatal(err)
	}
	if nodes.Proxies["Keep"].Name != "Keep" {
		t.Fatal("invalid apply replaced the working config")
	}
	e.mu.Lock()
	countsAfterFailure := e.counts
	e.mu.Unlock()
	if countsAfterFailure != countsBeforeFailure {
		t.Fatal("failed apply replaced the working config counts")
	}
	for i := range 10 {
		e.Reconcile(cfg, i%2 == 0)
	}
	await(e.Revision(), "stopped", false)
	if !e.Connected() {
		t.Fatal("stopping listeners disconnected the core")
	}
	if err := e.Call("getMemoryStats", nil, &memory); err != nil {
		t.Fatal("stopped listeners lost memory/tool access", err)
	}
	a := testApp(t)
	a.engine, a.status, a.memoryOpen = e, "stopped", true
	a.sample.Memory = memory
	if _, total := a.memoryTotals(); total != memory.RSS {
		t.Fatal("stopped listeners hid connected core memory", total, memory.RSS)
	}
	if !a.coreConnected() {
		t.Fatal("UI confused listener and IPC state")
	}
}
