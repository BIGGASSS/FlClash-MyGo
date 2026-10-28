package app

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/ipquality"
	"github.com/egoist/mygo/ui"
)

func TestServiceNodesShareOneProbeAndRouteWatchFollowsConsumers(t *testing.T) {
	a := testApp(t)
	a.safe, a.page, a.serviceHostSet = false, "tools", true
	a.services = []Service{
		{Name: "google", Status: "available", Chains: []string{"Node A"}},
		{Name: "github", Status: "available", Chains: []string{"Node A"}},
	}
	seedOutboundQuality(a, "203.0.113.20")
	var lookups, services atomic.Int32
	watches := make(chan bool, 8)
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		switch method {
		case "outboundIp":
			lookups.Add(1)
			var request struct {
				Proxy string `json:"proxy-name"`
			}
			_ = json.Unmarshal(args, &request)
			if request.Proxy != "Node A" {
				t.Errorf("service node was not pinned: %s", args)
			}
			return outboundFixture("203.0.113.20", "Node A")
		case "watchRoute":
			var wanted bool
			_ = json.Unmarshal(args, &wanted)
			watches <- wanted
			return routeCheck{CoreEpoch: 9, RouteVersion: 3}
		case "checkRoutes":
			return heldOutboundRoutes(args)
		case "serviceCheck":
			services.Add(1)
		}
		return nil
	})
	a.openServices()
	tt := ui.NewTester(a.View, 1200, 800)
	waitApp(t, a, func() bool { return a.working == 0 })
	a.verifyOutboundRoutes()
	waitApp(t, a, func() bool { return a.working == 0 })
	tt.Frame()
	if !<-watches || lookups.Load() != 1 || services.Load() != 0 {
		t.Fatal("service sheet did not share nodes or checked unrequested services", lookups.Load(), services.Load())
	}
	if !tt.HasText("203.0.113.20") {
		t.Fatal("resolved node address was not rendered")
	}
	a.serviceManage = true
	tt.Frame()
	waitApp(t, a, func() bool { return a.working == 0 })
	if <-watches {
		t.Fatal("unwatched Tools/management kept core route polling attached")
	}
	a.serviceManage = false
	tt.Frame()
	waitApp(t, a, func() bool { return a.working == 0 })
	if !<-watches || lookups.Load() != 1 {
		t.Fatal("resuming consumers did not reattach or needlessly reprobed a fresh exit")
	}
}

func TestOutboundSnapshotCannotRollBackFreshnessAndNewEpochClearsAddresses(t *testing.T) {
	a := testApp(t)
	s := a.outboundState()
	s.routeSynced, s.routeEpoch, s.routeVersion = true, 9, 5
	a.ip, a.country = "203.0.113.20", "JP"
	cancelled := false
	entry := &outboundEntry{value: &ipquality.Info{IP: a.ip, Country: a.country}, epoch: 9, version: 5, cancel: func() { cancelled = true }}
	s.entries[""], s.watched[""] = entry, true
	s.entries["Unwatched"] = &outboundEntry{value: entry.value, epoch: 9, version: 5}
	a.applyOutboundSnapshot(routeCheck{CoreEpoch: 8, RouteVersion: 100})
	a.applyOutboundSnapshot(routeCheck{CoreEpoch: 9, RouteVersion: 4})
	if s.entries[""] != entry || cancelled || a.ip == "" {
		t.Fatal("late subscription snapshot rolled back newer cache freshness")
	}
	a.applyOutboundSnapshot(routeCheck{CoreEpoch: 10, RouteVersion: 6})
	if a.ip != "" || !cancelled || s.entries[""].value != nil || s.entries["Unwatched"] != nil {
		t.Fatal("reattaching to a new core epoch displayed stale addresses")
	}
	if elapsed := time.Since(s.entries[""].last); elapsed < outboundThrottle-time.Second || elapsed >= outboundThrottle {
		t.Fatal("epoch invalidation did not retain the short, throttle-bypassing settle", elapsed)
	}
}

func TestOutboundDoesNotGuessLeafRoutesFromParentHits(t *testing.T) {
	a := testApp(t)
	a.safe, a.wantRunning, a.page = false, true, "tools"
	seedOutboundQuality(a, "203.0.113.20")
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		if method == "outboundIp" {
			result := outboundFixture("203.0.113.20", "Balanced Leaf")
			result.NodeHits = nil
			return result
		}
		return heldOutboundRoutes(args)
	})
	a.outboundIP()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.ip == "" || a.outbound.entries["Balanced Leaf"] != nil {
		t.Fatal("parent/load-balancer hits were incorrectly used as the leaf's route stamp")
	}
}

func TestOutboundCompletionWaitsForMainThreadUpdate(t *testing.T) {
	a := testApp(t)
	a.safe, a.page = false, "tools"
	seedOutboundQuality(a, "203.0.113.20")
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		if method == "outboundIp" {
			return outboundFixture("203.0.113.20", "Node A")
		}
		return heldOutboundRoutes(args)
	})
	a.outboundIP()
	deadline := time.Now().Add(5 * time.Second)
	for {
		a.updateMu.Lock()
		queued := len(a.updates) > 0
		a.updateMu.Unlock()
		if queued {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not enqueue its completion")
		}
		time.Sleep(time.Millisecond)
	}
	if a.ip != "" || a.outbound.entries[""].answer != nil || a.outbound.entries[""].value != nil {
		t.Fatal("background worker mutated UI state outside Window.Update")
	}
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.ip != "203.0.113.20" {
		t.Fatal("draining the main-thread queue did not publish the verified result")
	}
}

func TestProbeHostChangeRejectsPendingAddressAndServiceResults(t *testing.T) {
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
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		started <- struct{}{}
		<-gate
		return outboundFixture("203.0.113.20", "DIRECT")
	})
	a.services = []Service{{Name: "google", Status: "available"}}
	a.servicePending = map[string]uint64{"github": 1}
	a.outboundIP()
	<-started
	owner := a.outbound.owner
	a.probeHostChanged()
	close(gate)
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.ownsProbe(owner) || a.ip != "" || len(a.services) != 0 || len(a.servicePending) != 0 || len(a.outbound.entries) != 0 {
		t.Fatal("network/resume host epoch accepted pre-change state")
	}
}

func TestRouteSubscriptionConvergesLatestHiddenIntentAcrossHostChange(t *testing.T) {
	a := testApp(t)
	a.safe, a.page = false, "tools"
	gate, started := make(chan struct{}), make(chan struct{}, 1)
	watches := make(chan bool, 4)
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		var wanted bool
		_ = json.Unmarshal(args, &wanted)
		watches <- wanted
		if wanted {
			started <- struct{}{}
			<-gate
		}
		return routeCheck{CoreEpoch: 9, RouteVersion: 3}
	})
	s := a.syncOutboundOwner()
	s.routeWanted = true
	a.startProbeRouteWatch()
	<-started
	s.routeWanted = false // OnHide, even if the hidden window never builds.
	a.probeHostChanged()
	close(gate)
	waitApp(t, a, func() bool { return a.working == 0 })
	if !<-watches || <-watches || s.routeWatching || s.routeWatchRun {
		t.Fatal("late attach left the hidden window's core route poll subscribed")
	}
}

func TestQualityStaleOwnerAndUnwatchedRequestsDoNotPopulateCache(t *testing.T) {
	for _, kind := range []string{"revision", "unwatch", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			a := testApp(t)
			a.safe, a.page = false, "tools"
			gate, started := make(chan struct{}), make(chan struct{}, 8)
			t.Cleanup(func() {
				select {
				case <-gate:
				default:
					close(gate)
				}
			})
			a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
				select {
				case started <- struct{}{}:
				default:
				}
				<-gate
				return map[string]any{"status-code": 200, "body": `{"ip":"203.0.113.20","type":"isp"}`}
			})
			a.lookupQuality("203.0.113.20", false)
			<-started
			switch kind {
			case "revision":
				a.engine.mu.Lock()
				a.engine.revision++
				a.engine.mu.Unlock()
			case "unwatch":
				_ = ui.NewTester(a.View, 1200, 800)
			case "cancel":
				a.outbound.qualityJobs["203.0.113.20"].cancel()
			}
			close(gate)
			waitApp(t, a, func() bool { return a.working == 0 })
			if _, found := a.qualities["203.0.113.20"]; found || a.qualityBusy {
				t.Fatal("canceled/stale quality answer entered the persistent cache", kind)
			}
		})
	}
}
