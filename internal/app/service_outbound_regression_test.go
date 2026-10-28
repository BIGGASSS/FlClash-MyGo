package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/coreclient"
	"github.com/BIGGASSS/FlClash-MyGo/internal/ipquality"
	"github.com/egoist/mygo/ui"
)

func waitProbeCompletionQueued(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.updateMu.Lock()
		queued := len(a.updates) > 0
		a.updateMu.Unlock()
		if queued {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("probe completion was not queued")
}

func TestProbeCancellationWinsOverAlreadyQueuedSuccess(t *testing.T) {
	for _, kind := range []string{"outbound", "quality", "service"} {
		t.Run(kind, func(t *testing.T) {
			a := testApp(t)
			a.safe, a.page = false, "tools"
			a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
				switch method {
				case "outboundIp":
					return outboundFixture("203.0.113.20", "Node A")
				case "probe":
					return map[string]any{"status-code": 200, "body": `{"ip":"203.0.113.20","type":"isp"}`}
				case "serviceCheck":
					return []Service{{Name: "google", Status: "available"}}
				}
				return heldOutboundRoutes(args)
			})
			var cancel func()
			switch kind {
			case "outbound":
				a.requestOutbound("Node A", false)
				cancel = a.outbound.entries["Node A"].cancel
			case "quality":
				a.lookupQuality("203.0.113.20", false)
				cancel = a.outbound.qualityJobs["203.0.113.20"].cancel
			case "service":
				a.refreshServices(serviceTargets[:1])
				cancel = a.outbound.serviceJobs[a.serviceSerial]
			}
			waitProbeCompletionQueued(t, a)
			cancel()
			waitApp(t, a, func() bool { return a.working == 0 })
			if a.outbound.entries["Node A"] != nil || a.qualities["203.0.113.20"].Quality != nil || a.serviceValue("google") != nil || a.qualityBusy || a.servicePending["google"] != 0 {
				t.Fatal("cancellation after the RPC finished accepted its queued success", kind)
			}
		})
	}
}

func TestServiceOwnerChangeClearsCacheAndCancelsOwnedWork(t *testing.T) {
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
	var calls atomic.Int32
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		if calls.Add(1) == 1 {
			started <- struct{}{}
			<-gate
			return []Service{{Name: "google", Status: "blocked"}}
		}
		return []Service{{Name: "google", Status: "available"}}
	})
	a.services = []Service{{Name: "github", Status: "available", Chains: []string{"Old Node"}}}
	a.refreshServices(serviceTargets[:1])
	<-started
	a.engine.mu.Lock()
	a.engine.revision++
	a.engine.mu.Unlock()
	a.syncOutboundOwner()
	if len(a.services) != 0 || len(a.servicePending) != 0 || len(a.outbound.serviceJobs) != 0 {
		t.Fatal("service state survived a replacement lifecycle owner")
	}
	a.refreshServices(serviceTargets[:1])
	waitApp(t, a, func() bool { return a.working == 0 }) // Old IPC call must cancel without its server replying.
	if value := a.serviceValue("google"); value == nil || value.Status != "available" {
		t.Fatal("old cancellation superseded the replacement service check", value)
	}
	close(gate)
}

func TestOldServiceVerifierCannotReleaseReplacementRun(t *testing.T) {
	a := testApp(t)
	a.safe, a.page = false, "tools"
	gate := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})
	started := make(chan struct{}, 2)
	a.engine = outboundTestEngine(t, func(string, json.RawMessage) any {
		started <- struct{}{}
		<-gate
		return routeCheck{CoreEpoch: 9, RouteVersion: 3, Held: []bool{true}}
	})
	a.services = []Service{{Name: "google", Status: "available", CoreEpoch: 9}}
	a.verifyServiceRoutes()
	<-started
	a.engine.mu.Lock()
	a.engine.revision++
	a.engine.mu.Unlock()
	a.syncOutboundOwner()
	a.services = []Service{{Name: "google", Status: "available", CoreEpoch: 9}}
	a.verifyServiceRoutes()
	<-started
	waitApp(t, a, func() bool { return a.working == 1 })
	if !a.serviceRouteRun {
		t.Fatal("canceled old verifier released the newer owner's in-flight lock")
	}
	close(gate)
	waitApp(t, a, func() bool { return a.working == 0 })
	if value := a.serviceValue("google"); value == nil || value.RouteVersion != 3 || a.serviceRouteRun {
		t.Fatal("replacement verifier did not finish its own result", value)
	}
}

func TestFirstRouteSnapshotCannotDiscardNewerLanding(t *testing.T) {
	a := testApp(t)
	s := a.outboundState()
	entry := &outboundEntry{pending: true, answer: &outboundAnswer{epoch: 9, version: 3, info: ipquality.Info{IP: "203.0.113.20"}}}
	s.entries["Node A"] = entry
	a.applyOutboundSnapshot(routeCheck{CoreEpoch: 8, RouteVersion: 100})
	if s.entries["Node A"] != entry || entry.answer == nil {
		t.Fatal("the first slow subscription snapshot discarded an already newer landing")
	}
}

func TestServiceRouteCheckCannotRollBackNewerVersion(t *testing.T) {
	for _, held := range []bool{false, true} {
		a := testApp(t)
		a.services = []Service{{Name: "google", Status: "available", CoreEpoch: 9, RouteVersion: 5, CheckedAt: 10, request: 2}}
		a.applyServiceRouteCheck([]serviceRouteIdentity{{"google", 9, 10, 2}}, []bool{held}, 9, 4)
		if value := a.serviceValue("google"); value == nil || value.RouteVersion != 5 || a.serviceRouteDue.IsZero() {
			t.Fatal("an older check deleted or rolled back a newer answer", held, value)
		}
	}
}

func TestServiceVerifierRespectsNewerSharedRouteSnapshot(t *testing.T) {
	a := testApp(t)
	a.safe, a.page = false, "tools"
	a.engine = outboundTestEngine(t, func(string, json.RawMessage) any {
		return routeCheck{CoreEpoch: 9, RouteVersion: 3, Held: []bool{false}}
	})
	s := a.syncOutboundOwner()
	s.rememberRoute(routeCheck{CoreEpoch: 9, RouteVersion: 5})
	a.services = []Service{{Name: "google", Status: "available", CoreEpoch: 9, RouteVersion: 2}}
	a.verifyServiceRoutes()
	waitApp(t, a, func() bool { return a.working == 0 })
	if value := a.serviceValue("google"); value == nil || value.RouteVersion != 2 || a.serviceRouteDue.IsZero() {
		t.Fatal("an older service check ignored a newer outbound route snapshot", value)
	}
}

func TestOutboundRouteCheckCannotInvalidateNewerStamp(t *testing.T) {
	for _, knownSnapshot := range []bool{false, true} {
		a := testApp(t)
		a.safe, a.page = false, "tools"
		a.engine = outboundTestEngine(t, func(string, json.RawMessage) any {
			return routeCheck{CoreEpoch: 9, RouteVersion: 3, Held: []bool{false}}
		})
		s := a.syncOutboundOwner()
		version := uint64(5)
		if knownSnapshot {
			version = 2
			s.rememberRoute(routeCheck{CoreEpoch: 9, RouteVersion: 5})
		}
		entry := &outboundEntry{value: &ipquality.Info{IP: "203.0.113.20"}, epoch: 9, version: version, dirty: true}
		s.entries["Node A"] = entry
		a.verifyOutboundRoutes()
		waitApp(t, a, func() bool { return a.working == 0 })
		if s.entries["Node A"] != entry || entry.value == nil || entry.version != version || !entry.dirty || s.due.IsZero() {
			t.Fatal("late route check invalidated a newer route stamp", knownSnapshot)
		}
	}
}

func TestHiddenProbeLandingWaitsForVisibleVerification(t *testing.T) {
	a := testApp(t)
	a.safe, a.page = false, "tools"
	seedOutboundQuality(a, "203.0.113.20")
	gate, started := make(chan struct{}), make(chan struct{}, 1)
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		switch method {
		case "outboundIp":
			return outboundFixture("203.0.113.20", "Node A")
		case "checkRoutes":
			select {
			case started <- struct{}{}:
			default:
			}
			<-gate
			return heldOutboundRoutes(args)
		}
		return routeCheck{CoreEpoch: 9, RouteVersion: 3}
	})
	a.outboundIP()
	waitApp(t, a, func() bool { return a.outbound.entries[""].answer != nil })
	<-started
	a.setProbeHostVisible(false)
	close(gate)
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.ip != "" || a.outbound.entries[""].value != nil || a.outbound.entries[""].answer == nil {
		t.Fatal("hidden route verification exposed a newly landed address")
	}
	a.setProbeHostVisible(true)
	a.verifyOutboundRoutes()
	waitApp(t, a, func() bool { return a.working == 0 })
	if a.ip != "203.0.113.20" {
		t.Fatal("visible reattachment lost the staged answer")
	}
}

func TestHiddenHostReleasesQualityWithoutAnotherBuild(t *testing.T) {
	a := testApp(t)
	a.safe, a.page = false, "tools"
	a.engine = outboundTestEngine(t, func(string, json.RawMessage) any {
		return map[string]any{"status-code": 200, "body": `{"ip":"203.0.113.20","type":"isp"}`}
	})
	a.lookupQuality("203.0.113.20", false)
	waitProbeCompletionQueued(t, a)
	a.setProbeHostVisible(false)
	waitApp(t, a, func() bool { return a.working == 0 })
	if len(a.outbound.qualityJobs) != 0 || a.qualityBusy || a.qualities["203.0.113.20"].Quality != nil {
		t.Fatal("hidden host kept a quality request or accepted its queued result")
	}
	a.lookupQuality("203.0.113.21", false)
	a.requestOutbound("Node A", false)
	if a.working != 0 {
		t.Fatal("a hidden host launched automatic probe work")
	}
}

func TestNestedOutboundTapDoesNotRefreshParentService(t *testing.T) {
	a := populatedApp(t)
	a.safe, a.page, a.serviceHostSet, a.serviceProxied = false, "tools", true, a.wantRunning
	a.cfg.Disclaimer = true
	a.services = []Service{{Name: "google", Status: "available", Chains: []string{"Node A"}}}
	seedOutboundQuality(a, "203.0.113.20")
	var services atomic.Int32
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		if method == "serviceCheck" {
			services.Add(1)
			return []Service{{Name: "google", Status: "available", Chains: []string{"Node A"}}}
		}
		return routeCheck{CoreEpoch: 9, RouteVersion: 3}
	})
	s := a.syncOutboundOwner()
	s.entries["Node A"] = &outboundEntry{value: &ipquality.Info{IP: "203.0.113.20"}, epoch: 9, version: 3}
	a.openServices()
	tt := ui.NewTester(a.View, 1200, 800)
	waitApp(t, a, func() bool { return a.working == 0 })
	if err := tt.Click("Google · " + a.t("outboundIp")); err != nil {
		t.Fatal(err)
	}
	waitApp(t, a, func() bool { return a.working == 0 })
	if !a.qualityOpen || a.qualityIP != "203.0.113.20" || services.Load() != 0 {
		t.Fatal("nested outbound tap bubbled to the parent refresh", services.Load())
	}
	if err := tt.Click(a.t("back")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Google"); err != nil {
		t.Fatal(err, a.serviceOpen, a.qualityOpen, a.services, tt.Texts())
	}
	waitApp(t, a, func() bool { return a.working == 0 })
	if services.Load() != 1 {
		t.Fatal("parent service action stopped refreshing", services.Load())
	}
}

func TestFailedRouteAttachConvergesHiddenIntent(t *testing.T) {
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
	var releases atomic.Int32
	a.engine = outboundTestEngine(t, func(method string, args json.RawMessage) any {
		var wanted bool
		_ = json.Unmarshal(args, &wanted)
		if wanted {
			started <- struct{}{}
			<-gate
			return &coreclient.RPCError{Code: "test", Message: "attach acknowledgement lost"}
		}
		releases.Add(1)
		return routeCheck{CoreEpoch: 9, RouteVersion: 3}
	})
	s := a.syncOutboundOwner()
	s.routeWanted = true
	a.startProbeRouteWatch()
	<-started
	a.setProbeHostVisible(false)
	close(gate)
	waitApp(t, a, func() bool { return a.working == 0 })
	if releases.Load() != 1 || s.routeWatching || s.routeWatchRun || !s.routeWatchRetry.IsZero() {
		t.Fatal("failed attach stranded the hidden unsubscribe intent", releases.Load())
	}
}

func TestHiddenRouteReleaseRetriesWithoutBuild(t *testing.T) {
	a := testApp(t)
	a.safe, a.page = false, "tools"
	var releases atomic.Int32
	a.engine = outboundTestEngine(t, func(string, json.RawMessage) any {
		if releases.Add(1) == 1 {
			return &coreclient.RPCError{Code: "test", Message: "release acknowledgement lost"}
		}
		return routeCheck{CoreEpoch: 9, RouteVersion: 3}
	})
	s := a.syncOutboundOwner()
	s.routeWatching = true
	a.setProbeHostVisible(false)
	waitApp(t, a, func() bool { return releases.Load() == 2 && a.working == 0 })
	if s.routeWatching || s.routeWatchRun || !s.routeWatchRetry.IsZero() {
		t.Fatal("hidden unsubscribe did not retry without a UI build")
	}
}

func TestExpiredQualityIsNotDisplayedDuringReplacement(t *testing.T) {
	a := testApp(t)
	a.safe, a.page = false, "tools"
	seedOutboundQuality(a, "203.0.113.20")
	a.ip, a.organization = "203.0.113.20", "Expired ISP"
	record := a.qualities[a.ip]
	record.Checked = time.Now().Add(-73 * time.Hour)
	a.qualities[a.ip] = record
	a.engine = outboundTestEngine(t, func(string, json.RawMessage) any { return nil })
	a.lookupQuality(a.ip, false)
	if a.qualities[a.ip].Quality != nil || a.organization != "" || !a.qualityLoading(a.ip) {
		t.Fatal("expired quality stayed visible during refresh")
	}
	a.outbound.qualityJobs[a.ip].cancel()
	waitApp(t, a, func() bool { return a.working == 0 })
}

func TestQualityLoadDropsExpiredFutureAndFailureRecords(t *testing.T) {
	a := testApp(t)
	seedOutboundQuality(a, "203.0.113.20", "203.0.113.21", "203.0.113.22")
	old := a.qualities["203.0.113.21"]
	old.Checked = time.Now().Add(-73 * time.Hour)
	a.qualities["203.0.113.21"] = old
	future := a.qualities["203.0.113.22"]
	future.Checked = time.Now().Add(time.Hour)
	a.qualities["203.0.113.22"] = future
	a.qualities["203.0.113.23"] = qualityRecord{Checked: time.Now()}
	data, err := json.Marshal(a.qualities)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.store.Dir, "ip-quality.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	a.loadQualities()
	if len(a.qualities) != 1 || a.qualities["203.0.113.20"].Quality == nil {
		t.Fatal("durable quality cache restored stale results or memory-only failures", a.qualities)
	}
}

func TestServiceRouteSettlingIsBounded(t *testing.T) {
	a := testApp(t)
	a.markServiceRoutes()
	due := a.serviceRouteDue
	a.markServiceRoutes()
	if !a.serviceRouteDue.Equal(due) {
		t.Fatal("successive route events postponed verification indefinitely")
	}
}
