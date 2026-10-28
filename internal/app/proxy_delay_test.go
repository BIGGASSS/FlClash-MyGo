package app

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDelayTargetsFollowPicksURLsAndDeduplicate(t *testing.T) {
	a := populatedApp(t)
	a.sample.Proxies.Proxies["Outer"] = ProxyNode{Name: "Outer", Type: "Selector", All: []string{"Auto", "Japan 01"}, Now: "Auto", TestURL: "https://outer.invalid"}
	a.sample.Proxies.Proxies["Auto"] = ProxyNode{Name: "Auto", Type: "URLTest", All: []string{"Japan 01", "Hong Kong 01"}, Now: "Japan 01", TestURL: "https://inner.invalid"}
	a.cfg.Selected = map[string]string{"Auto": "Hong Kong 01"}
	target := a.resolveDelayTarget("Outer", "https://parent.invalid")
	if target.name != "Japan 01" || target.url != "https://inner.invalid" {
		t.Fatal("computed group did not use its real pick and innermost URL", target)
	}
	a.cfg.Selected["Outer"] = "Japan 01"
	if target = a.resolveDelayTarget("Outer", ""); target.name != "Japan 01" || target.url != "https://outer.invalid" {
		t.Fatal("selector did not use its valid saved choice", target)
	}
	a.cfg.Selected["Outer"] = "Removed"
	if targets := a.delayTargets([]string{"Outer", "Auto", "Outer"}, ""); len(targets) != 1 || targets[0].name != "Japan 01" {
		t.Fatal("nested aliases were tested more than once", targets)
	}
	a.sample.Proxies.Proxies["Empty"] = ProxyNode{Name: "Empty", Type: "Selector", TestURL: "https://empty.invalid"}
	a.cfg.Selected["Empty"] = "Japan 01"
	if target := a.resolveDelayTarget("Empty", ""); target.name != "Japan 01" || target.url != "https://empty.invalid" {
		t.Fatal("empty provider group lost its saved pick", target)
	}
	// A malformed cyclic group must not hang the UI.
	a.sample.Proxies.Proxies["Cycle"] = ProxyNode{Name: "Cycle", Type: "Selector", All: []string{"Cycle"}, Now: "Cycle"}
	if target := a.resolveDelayTarget("Cycle", ""); target.name != "Cycle" {
		t.Fatal(target)
	}
}

func TestDelayPoolSharesLimitKeepsQueueOrderAndCancelsQueuedJobs(t *testing.T) {
	pool := &proxyDelayPool{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firstRelease := make(chan struct{}, 32)
	laterRelease := make(chan struct{})
	defer close(laterRelease)
	defer close(firstRelease)
	started := make(chan string, 100)
	var active, peak, cancelledCalls atomic.Int32
	query := func(ctx context.Context, target delayTarget) (int, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		started <- target.name
		gate := laterRelease
		if target.url == "first" {
			gate = firstRelease
		}
		select {
		case <-gate:
			return 40, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	await := func(want string) {
		t.Helper()
		select {
		case name := <-started:
			if want != "" && name != want {
				t.Fatalf("queue started %q before %q", name, want)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	first := []delayTarget{}
	for i := range 32 {
		first = append(first, delayTarget{fmt.Sprint(i), "first"})
	}
	done := pool.submit(ctx, first, query, func(delayEvent) {}, nil)
	for range 32 {
		await("")
	}
	queuedCtx, stopQueued := context.WithCancel(ctx)
	queued := pool.submit(queuedCtx, first, func(context.Context, delayTarget) (int, error) {
		cancelledCalls.Add(1)
		return 0, nil
	}, func(delayEvent) {}, nil)
	b := pool.submit(ctx, []delayTarget{{"B", "later"}}, query, func(delayEvent) {}, nil)
	c := pool.submit(ctx, []delayTarget{{"C", "later"}}, query, func(delayEvent) {}, nil)
	stopQueued()
	firstRelease <- struct{}{}
	await("B")
	firstRelease <- struct{}{}
	await("C")
	if peak.Load() != 32 || cancelledCalls.Load() != 0 {
		t.Fatal("pool exceeded its shared limit or ran abandoned queries", peak.Load(), cancelledCalls.Load())
	}
	cancel()
	for _, finished := range []<-chan struct{}{done, queued, b, c} {
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Fatal("cancelled pool did not drain")
		}
	}
}

func TestDelayRepliesCannotCrossProfileOrCoreRevision(t *testing.T) {
	for _, change := range []string{"profile", "revision", "reset"} {
		t.Run(change, func(t *testing.T) {
			a := populatedApp(t)
			a.engine = &Engine{}
			target := delayTarget{"Japan 01", a.cfg.TestURL}
			cancelled := false
			job := &proxyDelayJob{id: 1, active: a.cfg.Active, engine: a.engine, group: "Proxy", targets: []delayTarget{target}, started: map[string]bool{}, finished: map[string]bool{}, cancel: func() { cancelled = true }}
			a.delayJobs = map[uint64]*proxyDelayJob{1: job}
			a.delayGroups = map[string]uint64{"Proxy": 1}
			a.delayPending = map[string]delayPending{target.key(): {holds: 1}}
			a.commitDelayEvents(job, []delayEvent{{target: target, started: true}}, false)
			if pending := a.delayPending[target.key()]; pending.holds != 1 || pending.runs != 1 {
				t.Fatal("pending indicator lost its active query", pending)
			}
			switch change {
			case "profile":
				a.cfg.Active = "another-profile"
			case "revision":
				a.engine.revision++
			case "reset":
				a.resetProxyProfile()
			}
			a.watchDelayTests()
			a.commitDelayEvents(job, []delayEvent{{target: target, value: 9876}}, true)
			if !cancelled || len(a.delayJobs) != 0 || len(a.delayGroups) != 0 || len(a.delayPending) != 0 || a.delays[target.key()] == 9876 {
				t.Fatal("abandoned delay reply modified current profile", a.delayPending, a.delays)
			}
		})
	}
}

func TestOverlappingDelayJobsRetainRunningIndicatorAndOldValueOnIPCError(t *testing.T) {
	a := populatedApp(t)
	a.engine = &Engine{}
	target := delayTarget{"Japan 01", a.cfg.TestURL}
	a.setProxyDelay(target.name, target.url, 45)
	a.delayJobs = map[uint64]*proxyDelayJob{}
	a.delayGroups = map[string]uint64{}
	a.delayPending = map[string]delayPending{target.key(): {holds: 2}}
	jobs := []*proxyDelayJob{}
	for id := uint64(1); id <= 2; id++ {
		job := &proxyDelayJob{id: id, active: a.cfg.Active, engine: a.engine, targets: []delayTarget{target}, started: map[string]bool{}, finished: map[string]bool{}, cancel: func() {}}
		jobs = append(jobs, job)
		a.delayJobs[id] = job
		a.commitDelayEvents(job, []delayEvent{{target: target, started: true}}, false)
	}
	a.commitDelayEvents(jobs[0], []delayEvent{{target: target, value: 60}}, true)
	if pending := a.delayPending[target.key()]; pending.holds != 1 || pending.runs != 1 {
		t.Fatal("one completion removed another job's progress", pending)
	}
	a.commitDelayEvents(jobs[1], []delayEvent{{target: target, err: context.Canceled}}, true)
	if len(a.delayPending) != 0 || a.delays[target.key()] != 60 || a.delaySnapshot[target.key()] != 60 {
		t.Fatal("IPC failure replaced the last measured delay", a.delayPending, a.delays)
	}
}

func TestProxyDelayControlsExposeQueuedAndRunningStates(t *testing.T) {
	a := populatedApp(t)
	a.page = "proxies"
	target := a.resolveDelayTarget("Japan 01", "")
	a.delayPending = map[string]delayPending{target.key(): {holds: 1}}
	tt := ui.NewTester(a.View, 1200, 800)
	if _, ok := tt.Find(a.t("testDelay") + " Japan 01"); !ok {
		t.Fatal("delay control lost its accessible label")
	}
	a.delayPending[target.key()] = delayPending{holds: 1, runs: 1}
	tt.Frame()
	if _, ok := tt.Find(a.t("loading")); !ok {
		t.Fatal("running delay has no progress indicator")
	}
	delete(a.delayPending, target.key())
	a.setProxyDelay(target.name, target.url, -2)
	tt.Frame()
	if !tt.HasText(a.t("delayFailed")) {
		t.Fatal("failed request was misreported as a timeout")
	}
}
