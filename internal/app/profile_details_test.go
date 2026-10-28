package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func waitApp(t *testing.T, a *App, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ready() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		a.flushUpdates()
	}
	if !ready() {
		t.Fatal("background operation did not finish")
	}
}

func TestProfileDetailsStayOnDashboardAndDismiss(t *testing.T) {
	for _, width := range []int{820, 1200} {
		a := populatedApp(t)
		tt := ui.NewTester(a.View, width, 2400)
		label := a.messageText("details", map[string]any{"label": a.t("profile")})
		if width == 820 {
			label = a.t("profile")
		}
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Advance(time.Second)
		if !a.profileDetail.Open || a.page != "dashboard" || len(a.history) != 0 {
			t.Fatal("profile details navigated away from dashboard", a.page, a.history)
		}
		for _, key := range []string{"runtimeConfig", "lastUpdated", "extendMode", "subscriptionInfo"} {
			if !tt.HasText(a.t(key)) {
				t.Fatal("missing profile detail", key)
			}
		}
		if !tt.HasText("42GB / 100GB") {
			t.Fatal("subscription usage differs", tt.Texts())
		}
		tt.Key(0, ui.KeyEscape)
		if !a.profileDetail.Open {
			t.Fatal("Escape disposed profile details before its exit")
		}
		tt.Advance(time.Second)
		if a.profileDetail.Open {
			t.Fatal("Escape did not dismiss profile sheet")
		}
		a.openProfileDetails()
		tt.Frame()
		tt.Advance(time.Second)
		tt.ClickAt(100, 400)
		if !a.profileDetail.Open {
			t.Fatal("backdrop disposed profile details before its exit")
		}
		tt.Advance(time.Second)
		if a.profileDetail.Open || a.page != "dashboard" {
			t.Fatal("backdrop did not dismiss without changing the page")
		}
	}
}

func TestProfileRuntimePreviewUsesAssemblyAndNestedNavigation(t *testing.T) {
	a := populatedApp(t)
	source := []byte("# keep subscription\nproxies: []\nproxy-groups: []\nrules: [MATCH,DIRECT]\ncustom-field: retained\n")
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), source); err != nil {
		t.Fatal(err)
	}
	a.cfg.Profiles[0].Script = "function main(config) { config['script-marker'] = 'preview'; return config; }"
	a.cfg.Overrides["mixed-port"] = 17890
	a.openProfileDetails()
	tt := ui.NewTester(a.View, 1200, 800)
	tt.Advance(time.Second)
	if err := tt.Click(a.t("runtimeConfig")); err != nil || !a.profileDetail.Preview {
		t.Fatal("preview did not start", err)
	}
	waitApp(t, a, func() bool { return !a.profileDetail.Busy })
	tt.Frame()
	code := a.profileDetail.Code
	if code == nil || !code.ReadOnly || !strings.Contains(code.Buffer.String(), "script-marker: preview") || !strings.Contains(code.Buffer.String(), "mixed-port: 17890") {
		t.Fatal("preview bypassed effective assembly", code, a.err)
	}
	if a.engine != nil || a.page != "dashboard" {
		t.Fatal("preview started a core or changed background page")
	}
	data, err := os.ReadFile(a.store.ProfilePath(a.cfg.Active))
	if err != nil || !bytes.Equal(source, data) {
		t.Fatal("preview rewrote source YAML", err)
	}
	if err := tt.Click(a.t("search")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if !a.profileDetail.Open || !a.profileDetail.Preview {
		t.Fatal("Escape closed the preview before its find bar")
	}
	tt.Key(0, ui.KeyEscape)
	if !a.profileDetail.Open || a.profileDetail.Preview {
		t.Fatal("Escape did not pop preview within the sheet")
	}
	tt.Key(0, ui.KeyEscape)
	if !a.profileDetail.Open {
		t.Fatal("Escape disposed root sheet before its exit")
	}
	tt.Advance(time.Second)
	if a.profileDetail.Open {
		t.Fatal("Escape did not close root sheet")
	}
}

func TestProfilePreviewDropsCompletionAfterCloseAndReopen(t *testing.T) {
	a := populatedApp(t)
	if err := store.AtomicWrite(a.store.ProfilePath(a.cfg.Active), []byte("proxies: []\n")); err != nil {
		t.Fatal(err)
	}
	a.openProfileDetails()
	a.previewProfileRuntime()
	a.closeProfileDetails()
	a.openProfileDetails()
	// Put a sentinel behind the worker callback to observe its arrival without
	// exposing editor state to the worker or reading the queue without its lock.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		a.updateMu.Lock()
		pending := len(a.updates) > 0
		a.updateMu.Unlock()
		if pending {
			break
		}
		time.Sleep(time.Millisecond)
	}
	a.flushUpdates()
	if !a.profileDetail.Open || a.profileDetail.Preview || a.profileDetail.Code != nil {
		t.Fatal("old preview populated a reopened sheet")
	}
}

func TestProfileSheetActionsClearNativeCaptionButtons(t *testing.T) {
	a := populatedApp(t)
	a.openProfileDetails()
	tt := ui.NewTester(a.View, 1200, 800)
	tt.SetTitleBar(ui.TitleBar{Height: 44, Right: 216})
	tt.Advance(time.Second)
	r, ok := tt.Find(a.t("runtimeConfig"))
	if !ok || r.X+r.W > 1200-216 {
		t.Fatal("runtime preview overlaps native caption buttons", r)
	}
}

func TestProfileUpdateDeduplicatesAndClearsBusyAfterFailure(t *testing.T) {
	var count atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		<-release
		http.Error(w, "fixture failed", http.StatusBadGateway)
	}))
	defer server.Close()
	a := populatedApp(t)
	a.safe = false
	a.status, a.wantRunning = "stopped", false
	p := a.cfg.Profiles[0]
	p.URL = server.URL
	a.updateProfile(p)
	a.updateProfile(p)
	if !a.profileUpdating[p.ID] || a.working != 1 {
		t.Fatal("duplicate profile update started")
	}
	close(release)
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.profileUpdating[p.ID] || count.Load() != 1 || a.err == "" {
		t.Fatal("failed update retained loading state", count.Load(), a.err)
	}
}
