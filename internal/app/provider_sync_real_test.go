package app

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

func TestRealCoreAppManagedRuleSync(t *testing.T) {
	binary := os.Getenv("FLCLASH_TEST_CORE")
	if binary == "" {
		t.Skip("requires the built core")
	}
	var body atomic.Value
	body.Store("payload:\n  - DOMAIN-KEYWORD,new.invalid\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body.Load().(string)) }))
	defer server.Close()
	a, p := remoteRuleApp(t, server.URL)
	a.wantRunning = false
	a.cfg.SystemProxy = false
	a.cfg.Overrides["mixed-port"] = 0
	a.cfg.Overrides["dns"] = map[string]any{"enable": false}
	a.cfg.Overrides["tun"] = map[string]any{"enable": false}
	a.cfg.Overrides["external-controller"] = ""
	original := []byte("payload:\n  - DOMAIN-KEYWORD,old.invalid\n")
	if err := store.AtomicWrite(p.Path, original); err != nil {
		t.Fatal(err)
	}
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte("proxies: []\nproxy-groups: []\nrules: ['RULE-SET,Library,DIRECT', 'MATCH,DIRECT']\n")); err != nil {
		t.Fatal(err)
	}
	e := NewEngine(a.store, binary, io.Discard)
	a.engine = e
	type result struct {
		revision uint64
		status   string
		err      error
	}
	events := make(chan result, 32)
	e.Status = func(status string, err error, revision uint64) { events <- result{revision, status, err} }
	e.Run()
	defer e.Close()
	await := func(revision uint64) {
		t.Helper()
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		for {
			select {
			case event := <-events:
				if event.revision != revision || event.status == "connecting" {
					continue
				}
				if event.status != "stopped" || event.err != nil {
					t.Fatal("owned core failed", event)
				}
				return
			case <-timer.C:
				t.Fatal("owned core did not converge")
			}
		}
	}
	awaitProvider := func(path string) []Provider {
		t.Helper()
		var providers []Provider
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			if err := e.Call("getExternalProviders", nil, &providers); err != nil {
				t.Fatal(err)
			}
			if len(providers) == 1 && providers[0].Path == path && providers[0].Vehicle == "File" && providers[0].Count == 1 {
				return providers
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("provider did not initialize", providers)
		return nil
	}
	e.Reconcile(a.cfg, false)
	await(e.Revision())
	awaitProvider(p.Path)
	a.updateDetailProvider(p)
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	got, _ := os.ReadFile(p.Path)
	if a.err != "" || string(got) != body.Load().(string) || e.Revision() != 1 {
		t.Fatal("unchanged shape did not side-load", string(got), a.err, e.Revision())
	}
	working := append([]byte(nil), got...)
	body.Store("payload: [new.invalid]\n")
	a.updateDetailProvider(p)
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	if a.err != "" || e.Revision() != 2 || len(a.cfg.Resources) != 3 {
		t.Fatal("shape change did not request engine apply", a.err, e.Revision(), len(a.cfg.Resources))
	}
	await(e.Revision())
	providers := awaitProvider(a.store.ResourcePath(a.cfg.Resources[1]))
	got, _ = os.ReadFile(p.Path)
	if !bytes.Equal(got, working) {
		t.Fatal("new shape changed the last working path", string(got))
	}
	body.Store("payload: [next.invalid]\n")
	a.sample.Providers = providers
	a.updateDetailProvider(providers[0])
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	resource, ok := a.store.RemoteRuleResource(a.cfg, providers[0].Path)
	source, _ := os.ReadFile(resource.SourcePath)
	compiled, _ := os.ReadFile(providers[0].Path)
	if a.err != "" || !ok || e.Revision() != 2 || string(source) != body.Load().(string) || !bytes.HasPrefix(compiled, []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		t.Fatal("compiled file/source refresh did not stay in place", resource, a.err, e.Revision())
	}
	body.Store("# empty download must not clear the working provider\n")
	a.sample.Providers = providers
	a.updateDetailProvider(providers[0])
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	if !strings.Contains(a.err, "no usable entries") || e.Revision() != 2 {
		t.Fatal("empty sync altered active shape", a.err, e.Revision())
	}
}
