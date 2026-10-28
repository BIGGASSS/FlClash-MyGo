package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func remoteRuleApp(t *testing.T, remote string) (*App, Provider) {
	t.Helper()
	a := populatedApp(t)
	a.safe, a.status = false, "stopped"
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte("# retained subscription\nproxies: []\nproxy-groups: []\nrules: [MATCH,DIRECT]\n")); err != nil {
		t.Fatal(err)
	}
	id := store.NewID()
	a.cfg.Resources = []string{id}
	path := a.store.ResourcePath(id)
	if err := store.AtomicWrite(path, []byte("payload: ['DOMAIN,old.invalid,DIRECT']\n")); err != nil {
		t.Fatal(err)
	}
	a.cfg.Overrides["rule-providers"] = map[string]any{"Library": map[string]any{"type": "file", "path": "flclash-resource://" + id, "url": remote, "behavior": "classical", "format": "yaml"}}
	p := Provider{Name: "Library", Type: "Rule", Vehicle: "File", Path: path}
	a.sample.Providers = []Provider{p}
	return a, p
}

func ruleSyncCore(t *testing.T, a *App, info map[string]string, reject string, sideLoads *atomic.Int32) {
	t.Helper()
	a.engine = providerTestClient(t, func(method string, args json.RawMessage) any {
		switch method {
		case "compileRuleSet":
			var params map[string]string
			_ = json.Unmarshal(args, &params)
			if params["url"] != "" || !strings.HasPrefix(params["name"], "sync-") {
				return nil
			}
			if info["behavior"] != "classical" && info["format"] != "mrs" {
				_ = store.AtomicWrite(filepath.Join(a.store.Dir, "providers", "rules", params["name"])+".mrs", []byte{0x28, 0xb5, 0x2f, 0xfd, 0})
			}
			return info
		case "validateMyGoConfig":
			if info["behavior"] != "classical" || info["format"] != "yaml" {
				return reject
			}
			return ""
		case "updateExternalProviderAtPath":
			sideLoads.Add(1)
			return reject
		default:
			return "unexpected method " + method
		}
	})
}

func TestRemoteRuleSyncUsesAppClientAndRetainsSourceAndMetadata(t *testing.T) {
	body := "payload: ['DOMAIN,new.invalid,DIRECT']\n"
	var agent atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent.Store(r.Header.Get("User-Agent"))
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()
	a, p := remoteRuleApp(t, server.URL)
	a.cfg.Overrides["global-ua"] = "rule-sync-fixture"
	before := store.Clone(a.cfg)
	source, err := os.ReadFile(a.store.ProfilePath(a.cfg.Active))
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	ruleSyncCore(t, a, map[string]string{"behavior": "classical", "format": "yaml"}, "", &calls)
	a.updateDetailProvider(p)
	a.updateDetailProvider(p)
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	got, err := os.ReadFile(p.Path)
	if err != nil || string(got) != body || calls.Load() != 1 || a.err != "" {
		t.Fatal("sync failed", string(got), calls.Load(), a.err, err)
	}
	if agent.Load() != "rule-sync-fixture" || a.engine.Revision() != 0 || !sameResourceIntent(a.cfg, before) {
		t.Fatal("request preferences or unchanged-shape lifecycle changed")
	}
	after, _ := os.ReadFile(a.store.ProfilePath(a.cfg.Active))
	if !bytes.Equal(source, after) {
		t.Fatal("sync rewrote subscription YAML")
	}
	matches, _ := filepath.Glob(filepath.Join(a.store.Dir, "providers", "rules", "sync-*"))
	if len(matches) != 0 {
		t.Fatal("compile staging leaked", matches)
	}
}

func TestRemoteRuleSyncRejectKeepsWorkingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "payload: ['new.invalid']\n") }))
	defer server.Close()
	for _, shape := range []bool{false, true} {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			a, p := remoteRuleApp(t, server.URL)
			original, _ := os.ReadFile(p.Path)
			info := map[string]string{"behavior": "classical", "format": "yaml"}
			if shape {
				info["behavior"] = "domain"
			}
			var calls atomic.Int32
			ruleSyncCore(t, a, info, "invalid provider fixture", &calls)
			a.updateDetailProvider(p)
			waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
			got, _ := os.ReadFile(p.Path)
			if !bytes.Equal(original, got) || !strings.Contains(a.err, "invalid provider fixture") || len(a.cfg.Resources) != 1 || a.engine.Revision() != 0 {
				t.Fatal("rejected sync changed the working config", string(got), a.err)
			}
			files, _ := os.ReadDir(filepath.Join(a.store.Dir, "resources"))
			if len(files) != 1 {
				t.Fatal("rejected sync leaked files", files)
			}
		})
	}
}

func TestRemoteRuleSyncShapeChangeKeepsOldFileAndReconciles(t *testing.T) {
	body := "payload: [new.invalid]\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) }))
	defer server.Close()
	a, p := remoteRuleApp(t, server.URL)
	original, _ := os.ReadFile(p.Path)
	a.store.ActiveProfile(a.cfg).Override = "# retained override\nunknown-fixture: true\nrules: ['MATCH,DIRECT']\n"
	var calls atomic.Int32
	ruleSyncCore(t, a, map[string]string{"behavior": "domain", "format": "yaml"}, "", &calls)
	a.updateDetailProvider(p)
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	got, _ := os.ReadFile(p.Path)
	if !bytes.Equal(original, got) || a.engine.Revision() != 1 || calls.Load() != 0 || a.err != "" || len(a.cfg.Resources) != 3 {
		t.Fatal("shape change did not retain last working file", string(got), a.engine.Revision(), calls.Load(), a.err)
	}
	path := a.store.ResourcePath(a.cfg.Resources[1])
	got, _ = os.ReadFile(a.store.ResourcePath(a.cfg.Resources[2]))
	if string(got) != body {
		t.Fatal("fresh source not saved", string(got))
	}
	resource, ok := a.store.RemoteRuleResource(a.cfg, path)
	if !ok || resource.URL != server.URL || resource.Behavior != "domain" || resource.Format != "yaml" {
		t.Fatal("new resource identity not retained", resource, ok)
	}
	if !strings.Contains(a.store.ActiveProfile(a.cfg).Override, "# retained override") || !strings.Contains(a.store.ActiveProfile(a.cfg).Override, "unknown-fixture: true") {
		t.Fatal("unrelated override metadata lost")
	}
	_, reloaded, err := store.Open(a.store.Dir)
	if err != nil || len(reloaded.Resources) != 3 || reloaded.Profiles[0].Override != a.cfg.Profiles[0].Override {
		t.Fatal("resource did not persist", err)
	}
}

func TestRemoteRuleLateResultsDoNotMutateReplacement(t *testing.T) {
	for _, replacement := range []string{"engine-revision", "path", "url", "profile", "engine"} {
		t.Run(replacement, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				close(started)
				<-release
				_, _ = io.WriteString(w, "payload: ['DOMAIN,new.invalid,DIRECT']\n")
			}))
			defer server.Close()
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			a, p := remoteRuleApp(t, server.URL)
			original, _ := os.ReadFile(p.Path)
			var calls atomic.Int32
			ruleSyncCore(t, a, map[string]string{"behavior": "classical", "format": "yaml"}, "", &calls)
			a.updateDetailProvider(p)
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("download not started")
			}
			switch replacement {
			case "engine-revision":
				a.engine.mu.Lock()
				a.engine.revision++
				a.engine.mu.Unlock()
			case "path":
				a.sample.Providers[0].Path = filepath.Join(t.TempDir(), "replacement")
			case "url":
				store.Map(a.cfg.Overrides, "rule-providers")[p.Name].(map[string]any)["url"] = server.URL + "/replacement"
			case "profile":
				a.cfg.Active = store.NewID()
			case "engine":
				a.engine = providerTestClient(t, func(string, json.RawMessage) any { return "" })
			}
			close(release)
			waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
			got, _ := os.ReadFile(p.Path)
			if !bytes.Equal(original, got) || calls.Load() != 0 || a.err != "" {
				t.Fatal("late sync touched replacement", string(got), calls.Load(), a.err)
			}
		})
	}
}

func TestProviderBulkFailuresAreCollected(t *testing.T) {
	a := populatedApp(t)
	a.safe = false
	a.sample.Providers = []Provider{{Name: "First", Type: "Proxy", Vehicle: "HTTP"}, {Name: "Second", Type: "Rule", Vehicle: "HTTP"}, {Name: "Third", Type: "Proxy", Vehicle: "HTTP"}}
	a.engine = providerTestClient(t, func(_ string, args json.RawMessage) any {
		var params map[string]string
		_ = json.Unmarshal(args, &params)
		return "failed " + params["providerName"]
	})
	a.updateDetailProviders()
	waitApp(t, a, func() bool { return len(a.providerPending) == 0 })
	for _, label := range []string{"First", "Second", "Third"} {
		if !strings.Contains(a.err, label+": failed "+label) {
			t.Fatal("bulk failure was overwritten", a.err)
		}
	}
}

func TestRuleDownloadBoundsStatusCancellationAndBinary(t *testing.T) {
	binary := []byte{0x28, 0xb5, 0x2f, 0xfd, 0, 0xff, 0xfe}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			w.WriteHeader(http.StatusForbidden)
		case "/large":
			w.Header().Set("Content-Length", fmt.Sprint(providerSizeLimit+1))
		case "/empty":
			_, _ = io.WriteString(w, " \n\t")
		case "/empty-list":
			_, _ = io.WriteString(w, "payload: []\n")
		case "/comments":
			_, _ = io.WriteString(w, "# comment\n// comment\n")
		case "/stream":
			chunk := bytes.Repeat([]byte{'x'}, 1<<20)
			for range 33 {
				if _, err := w.Write(chunk); err != nil {
					return
				}
			}
		default:
			_, _ = w.Write(binary)
		}
	}))
	defer server.Close()
	for _, path := range []string{"/status", "/large", "/empty", "/empty-list", "/comments", "/stream"} {
		if _, err := downloadRuleResource(context.Background(), server.Client(), server.URL+path, ""); err == nil {
			t.Fatal("invalid download accepted", path)
		}
	}
	got, err := downloadRuleResource(context.Background(), server.Client(), server.URL, "")
	if err != nil || !bytes.Equal(got, binary) {
		t.Fatal("binary rule set was changed", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := downloadRuleResource(ctx, server.Client(), server.URL, ""); err == nil {
		t.Fatal("canceled download succeeded")
	}
}

func TestProviderSubscriptionDialogUsesSourceSections(t *testing.T) {
	for _, size := range [][2]int{{1200, 1000}, {380, 400}} {
		a := populatedApp(t)
		a.openProfileDetails()
		info := &providerSubscription{Upload: 1 << 30, Download: 2 << 30, Total: 10 << 30}
		a.showProviderSubscription(Provider{Name: "Fixture", Subscription: info})
		info.Total = 0
		tt := ui.NewTester(a.View, size[0], size[1])
		for _, text := range []string{a.t("subscriptionInfo"), a.t("trafficUsage"), a.t("usedTraffic"), a.t("totalTraffic"), "3GB", "10GB", a.t("infiniteTime")} {
			if !tt.HasText(text) {
				t.Fatal("source subscription section absent", size, text, tt.Texts())
			}
		}
		if a.detailOpen || a.detail != "" {
			t.Fatal("subscription still uses the generic details dialog")
		}
		if err := tt.Click(a.t("confirm")); err != nil || a.providerState().infoOpen || !a.profileDetail.Open {
			t.Fatal("confirm left the originating sheet", err)
		}
	}
}

func TestProviderSubscriptionEscapeAndExpiryStayInOriginatingSheet(t *testing.T) {
	a := populatedApp(t)
	a.cfg.Theme = "dark"
	a.openProfileDetails()
	expire := time.Date(2030, time.June, 15, 0, 0, 0, 0, time.Local)
	a.showProviderSubscription(Provider{Subscription: &providerSubscription{Total: 1 << 30, Expire: expire.Unix()}})
	tt := ui.NewTester(a.View, 600, 700)
	if !tt.HasText("2030-06-15") {
		t.Fatal("expiry did not use the source local date", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if a.providerState().infoOpen || !a.profileDetail.Open {
		t.Fatal("Escape closed the underlying sheet")
	}
}

func TestProviderSaveLateReplyAndNewTyping(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			a := populatedApp(t)
			a.safe = false
			path := filepath.Join(t.TempDir(), "rules.yaml")
			_ = store.AtomicWrite(path, []byte("original"))
			p := Provider{Name: "Editor", Type: "Rule", Path: path}
			started, release := make(chan struct{}), make(chan struct{})
			a.engine = providerTestClient(t, func(method string, args json.RawMessage) any {
				var payload map[string]string
				_ = json.Unmarshal(args, &payload)
				body, _ := base64.StdEncoding.DecodeString(payload["data-base64"])
				if method != "sideLoadExternalProvider" || string(body) != "saved" {
					return "unexpected save"
				}
				close(started)
				<-release
				return ""
			})
			a.openProfileDetails()
			a.openProfileProvider(p, true, providerText)
			waitApp(t, a, func() bool { return !a.profileDetail.Busy })
			a.profileDetail.Code.Buffer.Set("saved")
			a.saveProfileProvider()
			select {
			case <-started:
			case <-time.After(time.Second):
				close(release)
				t.Fatal("save did not reach the captured core")
			}
			if replace {
				a.engine.mu.Lock()
				a.engine.revision++
				a.engine.mu.Unlock()
			} else {
				a.profileDetail.Code.Buffer.Set("newer typing")
			}
			close(release)
			waitApp(t, a, func() bool { return !a.profileDetail.Saving })
			got, _ := os.ReadFile(path)
			want := "saved"
			if replace {
				want = "original"
			}
			if string(got) != want || !a.profileDetail.Preview || a.err != "" {
				t.Fatal("save lost newer intent", string(got), a.err)
			}
			if !replace && a.profileDetail.Code.Buffer.String() != "newer typing" {
				t.Fatal("save lost text typed in flight")
			}
		})
	}
}
