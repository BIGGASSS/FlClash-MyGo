package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/coreclient"
	"github.com/BIGGASSS/FlClash-MyGo/internal/ipquality"
	"github.com/egoist/mygo/ui"
)

// Framed IPC only: no core executable, HTTP listener, or external network.
// Replies run independently so tests can deliberately reorder late results.
func outboundTestEngine(t *testing.T, response func(string, json.RawMessage) any) *Engine {
	t.Helper()
	left, right := net.Pipe()
	client := coreclient.New(left, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	var writes sync.Mutex
	t.Cleanup(func() { cancel(); _ = left.Close(); _ = right.Close() })
	go func() {
		for {
			frame, err := coreclient.ReadFrame(right)
			if err != nil {
				return
			}
			var request struct {
				ID, Method string
				Arguments  json.RawMessage
			}
			if json.Unmarshal(frame, &request) != nil {
				return
			}
			go func() {
				result := response(request.Method, request.Arguments)
				reply := map[string]any{"id": request.ID, "result": result}
				if rpcError, ok := result.(*coreclient.RPCError); ok {
					reply = map[string]any{"id": request.ID, "error": rpcError}
				}
				data, _ := json.Marshal(reply)
				writes.Lock()
				defer writes.Unlock()
				_ = coreclient.WriteFrame(right, data)
			}()
		}
	}()
	return &Engine{client: client, ctx: ctx}
}

func outboundFixture(ip, node string) outboundResult {
	return outboundResult{URL: ipquality.OutboundSources[0], Body: "ip=" + ip + "\nloc=JP\n", Chains: []string{node, "Proxy"}, CoreEpoch: 9, RouteVersion: 3,
		Hits: []routeHit{{Host: "www.cloudflare.com:443", Exit: "parent-exit"}}, NodeHits: []routeHit{{Host: "www.cloudflare.com:443", Proxy: node, Exit: "node-exit"}}}
}

func heldOutboundRoutes(args json.RawMessage) routeCheck {
	var request struct {
		Routes [][]routeHit `json:"routes"`
	}
	_ = json.Unmarshal(args, &request)
	held := make([]bool, len(request.Routes))
	for i, hits := range request.Routes {
		held[i] = len(hits) > 0
	}
	return routeCheck{CoreEpoch: 9, RouteVersion: 3, Held: held}
}

func seedOutboundQuality(a *App, ips ...string) {
	for _, ip := range ips {
		a.qualities[ip] = qualityRecord{Checked: time.Now(), Quality: &ipquality.Quality{IP: ip, Type: "residential", Organization: "Fixture ISP", Source: ipquality.IdentMe}}
	}
}

func TestOutboundCacheSharesDashboardResolvedNodeAndVerifiesRoutes(t *testing.T) {
	a := testApp(t)
	a.safe, a.wantRunning, a.page = false, true, "tools"
	seedOutboundQuality(a, "203.0.113.20")
	var lookups, checks atomic.Int32
	var moved atomic.Bool
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		switch method {
		case "outboundIp":
			lookups.Add(1)
			var request struct {
				Proxy string `json:"proxy-name"`
				Time  int    `json:"timeout"`
			}
			_ = json.Unmarshal(args, &request)
			if request.Proxy != "" || request.Time != 6000 {
				t.Errorf("dashboard did not use rule routing/budget: %s", args)
			}
			return outboundFixture("203.0.113.20", "Node A")
		case "checkRoutes":
			checks.Add(1)
			check := heldOutboundRoutes(args)
			if moved.Load() {
				for i := range check.Held {
					check.Held[i] = false
				}
			}
			return check
		}
		return nil
	})
	a.outboundIP()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.ip != "203.0.113.20" || a.organization != "Fixture ISP" || checks.Load() != 1 {
		t.Fatal("unverified or missing dashboard result", a.ip, a.organization, checks.Load())
	}
	a.requestOutbound("Node A", false)
	if a.working != 0 || lookups.Load() != 1 || a.outbound.entries["Node A"].value.IP != a.ip {
		t.Fatal("dashboard answer was not shared with its resolved leaf")
	}
	// A different pick elsewhere does not force another network probe.
	a.markServiceRoutes()
	a.verifyOutboundRoutes()
	waitApp(t, a, func() bool { return a.working == 0 })
	if lookups.Load() != 1 || a.outbound.entries["Node A"].value == nil {
		t.Fatal("an unchanged exit was discarded")
	}
	// A moved exit is never shown again just because its node name is equal.
	moved.Store(true)
	a.markServiceRoutes()
	a.verifyOutboundRoutes()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.ip != "" || a.outbound.entries["Node A"] != nil || a.outbound.entries[""] != nil {
		t.Fatal("moved, unwatched addresses survived verification", a.ip, a.outbound.entries)
	}
}

func TestOutboundLandingIsHiddenUntilRouteCheckAndOldBackfillCannotWin(t *testing.T) {
	a := testApp(t)
	a.safe, a.wantRunning, a.page = false, true, "tools"
	seedOutboundQuality(a, "203.0.113.20", "203.0.113.21")
	dashboardGate, leafGate, checkGate := make(chan struct{}), make(chan struct{}), make(chan struct{})
	dashboardStarted, leafStarted, checkStarted := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{}, 4)
	t.Cleanup(func() {
		// Gates are closed by the test before cleanup on its successful path.
		for _, gate := range []chan struct{}{dashboardGate, leafGate, checkGate} {
			select {
			case <-gate:
			default:
				close(gate)
			}
		}
	})
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		switch method {
		case "outboundIp":
			var request struct {
				Proxy string `json:"proxy-name"`
			}
			_ = json.Unmarshal(args, &request)
			if request.Proxy == "" {
				dashboardStarted <- struct{}{}
				<-dashboardGate
				return outboundFixture("203.0.113.20", "Node A")
			}
			leafStarted <- struct{}{}
			<-leafGate
			return outboundFixture("203.0.113.21", "Node A")
		case "checkRoutes":
			checkStarted <- struct{}{}
			<-checkGate
			return heldOutboundRoutes(args)
		}
		return nil
	})
	a.outboundIP()
	<-dashboardStarted
	a.requestOutbound("Node A", true)
	<-leafStarted
	leaf := a.outbound.entries["Node A"]
	close(dashboardGate)
	waitApp(t, a, func() bool { return a.outbound.entries[""].answer != nil })
	<-checkStarted
	if a.ip != "" || a.outbound.entries["Node A"] != leaf {
		t.Fatal("landing leaked or old dashboard backfill replaced a newer node request")
	}
	close(checkGate)
	close(leafGate)
	waitApp(t, a, func() bool { return a.working == 0 })
	// A second landing waits for the next build's bounded route verification.
	if a.outbound.entries["Node A"].answer != nil {
		a.verifyOutboundRoutes()
		waitApp(t, a, func() bool { return a.working == 0 })
	}
	if entry := a.outbound.entries["Node A"]; entry != leaf || entry.value == nil || entry.value.IP != "203.0.113.21" {
		t.Fatal("late shared answer overwrote its independently requested leaf", entry)
	}
}

func TestOutboundStaleLifecycleAndEpochResultsAreRejected(t *testing.T) {
	for _, kind := range []string{"revision", "client", "proxied", "epoch", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			a := testApp(t)
			a.safe, a.page = false, "tools"
			gate, started := make(chan struct{}), make(chan struct{}, 1)
			t.Cleanup(func() {
				select {
				case <-gate:
				default:
					close(gate)
				}
			})
			var lookups atomic.Int32
			a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
				if method == "outboundIp" {
					lookups.Add(1)
					var request map[string]any
					_ = json.Unmarshal(args, &request)
					if request["proxy-name"] != "DIRECT" {
						t.Errorf("stopped probe was not pinned DIRECT: %s", args)
					}
					started <- struct{}{}
					<-gate
					return outboundFixture("203.0.113.20", "DIRECT")
				}
				check := heldOutboundRoutes(args)
				check.CoreEpoch++
				return check
			})
			a.outboundIP()
			<-started
			switch kind {
			case "revision":
				a.engine.mu.Lock()
				a.engine.revision++
				a.engine.mu.Unlock()
			case "client":
				a.engine.mu.Lock()
				a.engine.client = nil
				a.engine.mu.Unlock()
			case "proxied":
				a.wantRunning = true
			case "cancel":
				a.outbound.entries[""].cancel()
			}
			close(gate)
			waitApp(t, a, func() bool { return a.working == 0 })
			if a.ip != "" || a.organization != "" || a.qualities["203.0.113.20"].Quality != nil || lookups.Load() != 1 {
				t.Fatal("superseded probe escaped into current state", a.ip, a.organization)
			}
		})
	}
}

func TestServiceOutboundDetailsBackPrivacyAndFallbackInSafeMode(t *testing.T) {
	a := populatedApp(t)
	a.services = []Service{{Name: "google", Status: "available", Delay: 23, Region: "US", Chains: []string{"Node A"}}}
	s := a.outboundState()
	s.entries["Node A"] = &outboundEntry{value: &ipquality.Info{IP: "198.51.100.27", Country: "JP"}}
	seedOutboundQuality(a, "198.51.100.27")
	a.openServices()
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("198.51.100.27") || !tt.HasText("🇯🇵") {
		t.Fatal("service did not show its resolved outbound/country", tt.Texts())
	}
	if err := tt.Click("Google · " + a.t("outboundIp")); err != nil {
		t.Fatal(err)
	}
	if !a.qualityOpen || !a.serviceOpen || a.qualityIP != "198.51.100.27" || !tt.HasText(a.t("ipOrganization")) {
		t.Fatal("outbound tap did not open nested IP details", tt.Texts())
	}
	if err := tt.Click(a.t("ipAddress")); err != nil || tt.Clipboard() != "198.51.100.27" {
		t.Fatal("outbound IP could not be copied", err)
	}
	a.cfg.HideIP = true
	tt.Frame()
	if tt.HasText("198.51.100.27") || !tt.HasText("*****") {
		t.Fatal("nested IP privacy was not shared")
	}
	tt.SetClipboard("unchanged")
	_ = tt.Click(a.t("ipAddress"))
	if tt.Clipboard() != "unchanged" {
		t.Fatal("hidden outbound IP was copied")
	}
	if err := tt.Click(a.t("back")); err != nil || a.qualityOpen || !a.serviceOpen {
		t.Fatal("quality back did not restore the service page", err)
	}
	if !tt.HasText("Google") || !tt.HasText("*****") {
		t.Fatal("return lost service rows or leaked the hidden IP", tt.Texts())
	}
	s.entries["Node A"] = &outboundEntry{failed: true}
	tt.Frame()
	if !tt.HasText("Node A") || !tt.HasText("🇺🇸") {
		t.Fatal("failed address did not fall back to the service's node/region", tt.Texts())
	}
	if _, ok := tt.Find("Google · " + a.t("outboundIp")); ok {
		t.Fatal("failed node exposed a bogus address action")
	}
	a.outboundIP()
	a.requestOutbound("Node A", true)
	a.lookupQuality("198.51.100.27", true)
	if a.engine != nil || a.working != 0 {
		t.Fatal("safe-mode details launched work")
	}
	tt.Key(0, ui.KeyEscape)
	if a.serviceOpen {
		t.Fatal("escape did not dismiss restored services")
	}
}

func TestOutboundErrorsCoalesceAndObserveFailureCooldown(t *testing.T) {
	for _, result := range []outboundResult{{Error: "timeout"}, {Error: "failed"}, {URL: ipquality.OutboundSources[0], Body: "not an address"}} {
		a := testApp(t)
		a.safe, a.page = false, "tools"
		var lookups atomic.Int32
		a.engine = outboundTestEngine(t, func(string, json.RawMessage) any { lookups.Add(1); return result })
		a.requestOutbound("Node A", false)
		a.requestOutbound("Node A", true)
		waitApp(t, a, func() bool { return a.working == 0 })
		a.requestOutbound("Node A", false)
		if lookups.Load() != 1 || !a.outbound.entries["Node A"].failed || a.outbound.entries["Node A"].pending {
			t.Fatal("failure was not coalesced/cooled", result, lookups.Load())
		}
		a.requestOutbound("Node A", true)
		waitApp(t, a, func() bool { return a.working == 0 })
		if lookups.Load() != 2 {
			t.Fatal("manual retry did not bypass cooldown")
		}
	}
}

func TestQualityLookupsArePerIPAndRespectFreshness(t *testing.T) {
	a := testApp(t)
	a.safe, a.page = false, "tools"
	var calls atomic.Int32
	gate, started := make(chan struct{}), make(chan struct{}, 8)
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		calls.Add(1)
		var request struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal(args, &request)
		select {
		case started <- struct{}{}:
		default:
		}
		<-gate
		ip := "203.0.113.20"
		if strings.Contains(request.URL, "203.0.113.21") {
			ip = "203.0.113.21"
		}
		return map[string]any{"status-code": 200, "body": fmt.Sprintf(`{"ip":%q,"type":"isp","isp":{"asn":64500},"risk":{"is_datacenter":true}}`, ip)}
	})
	a.lookupQuality("203.0.113.20", false)
	a.lookupQuality("203.0.113.20", false)
	first := a.outbound.qualityJobs["203.0.113.20"]
	a.lookupQuality("203.0.113.21", false)
	<-started
	if len(a.outbound.qualityJobs) != 2 || a.outbound.qualityJobs["203.0.113.20"] != first {
		t.Fatal("opening a service IP canceled/replaced the dashboard quality request")
	}
	close(gate)
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.qualities["203.0.113.20"].Quality == nil || a.qualities["203.0.113.21"].Quality == nil {
		t.Fatal("per-IP quality results did not both land", a.qualities)
	}
	restored, err := New(a.store.Dir, "", true)
	if err != nil || restored.qualities["203.0.113.20"].Quality == nil || restored.qualities["203.0.113.21"].Quality == nil {
		t.Fatal("concurrent quality writes lost the newest durable snapshot", err)
	}
	before := calls.Load()
	a.lookupQuality("203.0.113.20", false)
	if calls.Load() != before || a.working != 0 {
		t.Fatal("fresh quality was fetched again")
	}
	now := time.Now()
	for _, tc := range []struct {
		record qualityRecord
		fresh  bool
	}{
		{qualityRecord{Checked: now.Add(-71 * time.Hour), Quality: &ipquality.Quality{}}, true},
		{qualityRecord{Checked: now.Add(-72 * time.Hour), Quality: &ipquality.Quality{}}, false},
		{qualityRecord{Checked: now.Add(-4 * time.Minute)}, true},
		{qualityRecord{Checked: now.Add(-5 * time.Minute)}, false},
		{qualityRecord{Checked: now.Add(time.Second), Quality: &ipquality.Quality{}}, false},
	} {
		if got := qualityFresh(tc.record, now); got != tc.fresh {
			t.Fatal("quality freshness boundary", tc.record, got)
		}
	}
}
