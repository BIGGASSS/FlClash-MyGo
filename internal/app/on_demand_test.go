package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/desktop"
	"github.com/egoist/mygo/ui"
)

type onDemandReadFunc func(context.Context) (desktop.WiFiState, error)

func (fn onDemandReadFunc) Read(ctx context.Context) (desktop.WiFiState, error) { return fn(ctx) }

// The engine receives real revisioned intents, but its loop is deliberately not
// run: no core, proxy or TUN backend can be reached by these tests.
func onDemandTestApp(t *testing.T) *App {
	t.Helper()
	a := testApp(t)
	a.safe = false
	a.wantRunning = true
	a.cfg.ExcludeSSIDs = []string{"Home", " padded ", "wifi*"}
	a.engine = NewEngine(a.store, "must-not-launch", io.Discard)
	a.onDemand.generation = 1
	t.Cleanup(func() { a.stopOnDemand(); a.engine.cancel() })
	return a
}

func assertOnDemandIntent(t *testing.T, a *App, want bool) {
	t.Helper()
	a.engine.mu.Lock()
	defer a.engine.mu.Unlock()
	if a.engine.running != want {
		t.Fatalf("engine listener intent %v, want %v", a.engine.running, want)
	}
}

func TestOnDemandParksRunAndNeverOverridesLatestStop(t *testing.T) {
	a := onDemandTestApp(t)
	generation := a.onDemand.generation
	a.reconcileOnDemand()
	assertOnDemandIntent(t, a, true)
	started := time.Now().Add(-time.Hour)
	a.started = started
	a.acceptOnDemand(generation, desktop.WiFiState{Connected: true, SSID: "Home"}, nil)
	assertOnDemandIntent(t, a, false)
	if !a.wantRunning || !a.onDemandSuspended() || a.started != started {
		t.Fatal("exclusion ended the run instead of parking it")
	}
	parkRevision := a.engine.Revision()
	a.acceptOnDemand(generation, desktop.WiFiState{Connected: true, SSID: "Home"}, nil)
	if a.engine.Revision() != parkRevision {
		t.Fatal("unchanged SSID issued another lifecycle intent")
	}
	// Exact matching, not trimming, globbing, or case folding.
	for _, ssid := range []string{"home", "padded", "wifi123"} {
		a.acceptOnDemand(generation, desktop.WiFiState{Connected: true, SSID: ssid}, nil)
		assertOnDemandIntent(t, a, true)
	}
	a.acceptOnDemand(generation, desktop.WiFiState{Connected: true, SSID: "Home"}, nil)
	assertOnDemandIntent(t, a, false)
	a.wantRunning = false
	a.reconcileOnDemand()
	stopRevision := a.engine.Revision()
	a.acceptOnDemand(generation, desktop.WiFiState{}, nil)
	assertOnDemandIntent(t, a, false)
	if a.wantRunning || a.engine.Revision() <= stopRevision {
		t.Fatal("Wi-Fi did not preserve the latest manual stop")
	}
	// A start requested while still excluded remains parked.
	a.acceptOnDemand(generation, desktop.WiFiState{Connected: true, SSID: "Home"}, nil)
	a.wantRunning = true
	a.reconcileOnDemand()
	assertOnDemandIntent(t, a, false)
	a.acceptOnDemand(generation, desktop.WiFiState{}, nil)
	assertOnDemandIntent(t, a, true)
}

func TestOnDemandPermissionFailuresAreNotNetworkTransitions(t *testing.T) {
	a := onDemandTestApp(t)
	generation := a.onDemand.generation
	failure := errors.Join(desktop.ErrWiFiPermission, errors.New("location denied"))
	a.reconcileOnDemand()
	a.acceptOnDemand(generation, desktop.WiFiState{}, failure)
	assertOnDemandIntent(t, a, true)
	if a.onDemand.Known || a.onDemand.Error == "" || a.onDemandSuspended() {
		t.Fatal("unknown permission state was treated as a known association", a.onDemand)
	}
	a.acceptOnDemand(generation, desktop.WiFiState{Connected: true, SSID: "Home"}, nil)
	parkRevision := a.engine.Revision()
	a.acceptOnDemand(generation, desktop.WiFiState{}, failure)
	assertOnDemandIntent(t, a, false)
	if a.engine.Revision() != parkRevision || !a.onDemandSuspended() || a.onDemand.Error == "" {
		t.Fatal("permission failure resumed an excluded connection", a.onDemand)
	}
	a.acceptOnDemand(generation, desktop.WiFiState{Connected: true}, nil)
	if a.onDemand.Error == "" || !a.onDemandSuspended() {
		t.Fatal("empty connected SSID silently cleared exclusion")
	}
	a.acceptOnDemand(generation, desktop.WiFiState{}, nil)
	assertOnDemandIntent(t, a, true)
	if a.onDemand.Error != "" || !a.onDemand.Known {
		t.Fatal("successful read did not clear the error")
	}
}

func TestOnDemandSafeModeHasNoReaderOrEngineSideEffects(t *testing.T) {
	a := onDemandTestApp(t)
	a.safe = true
	a.onDemand.reader = onDemandReadFunc(func(context.Context) (desktop.WiFiState, error) {
		t.Error("safe mode called the Wi-Fi backend")
		return desktop.WiFiState{}, nil
	})
	a.syncOnDemand()
	a.begin()
	a.apply()
	a.acceptOnDemand(a.onDemand.generation, desktop.WiFiState{Connected: true, SSID: "Home"}, nil)
	if err := a.setExcludeSSIDs([]string{"Home", "Work"}); err != nil {
		t.Fatal(err)
	}
	if a.onDemand.cancel != nil || a.onDemand.Known || a.engine.Revision() != 0 || a.listenerIntent() {
		t.Fatal("safe mode started monitoring or reconciled a core")
	}
}

func TestOnDemandReaderResultsCommitOnlyOnUIAndRejectSupersededQueries(t *testing.T) {
	a := onDemandTestApp(t)
	type request struct {
		ctx    context.Context
		result chan desktop.WiFiState
	}
	requests := make(chan request, 4)
	a.onDemand.reader = onDemandReadFunc(func(ctx context.Context) (desktop.WiFiState, error) {
		reply := make(chan desktop.WiFiState, 1)
		requests <- request{ctx, reply}
		select {
		case state := <-reply:
			return state, nil
		case <-ctx.Done():
			return desktop.WiFiState{}, ctx.Err()
		}
	})
	readRequest := func() request {
		t.Helper()
		select {
		case req := <-requests:
			return req
		case <-time.After(time.Second):
			t.Fatal("no SSID query")
			return request{}
		}
	}
	a.syncOnDemand()
	first := readRequest()
	oldGeneration := a.onDemand.generation
	first.result <- desktop.WiFiState{Connected: true, SSID: "Home"}
	deadline := time.Now().Add(time.Second)
	for {
		a.updateMu.Lock()
		queued := len(a.updates)
		a.updateMu.Unlock()
		if queued > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("read did not queue its result")
		}
		time.Sleep(time.Millisecond)
	}
	if a.onDemand.Known || a.engine.Revision() != 0 {
		t.Fatal("reader changed UI state off-thread")
	}
	tt := ui.NewTester(a.View, 1200, 800)
	if !a.onDemandSuspended() {
		t.Fatal("headless frame did not commit the queued association")
	}
	assertOnDemandIntent(t, a, false)
	if err := a.setExcludeSSIDs([]string{"Work"}); err != nil {
		t.Fatal(err)
	}
	second := readRequest()
	assertOnDemandIntent(t, a, true)
	revision := a.engine.Revision()
	a.acceptOnDemand(oldGeneration, desktop.WiFiState{Connected: true, SSID: "Work"}, nil)
	if a.engine.Revision() != revision || a.onDemandSuspended() {
		t.Fatal("superseded query was accepted")
	}
	if err := a.setExcludeSSIDs(nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-second.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("removing the final exclusion did not cancel its query")
	}
	if a.onDemand.cancel != nil || a.onDemand.Known {
		t.Fatal("empty exclusion list retained monitoring")
	}
	tt.Frame()
	assertOnDemandIntent(t, a, true)
}

func TestOnDemandRearmsAfterMonitorCancellationOrEngineReplacement(t *testing.T) {
	for _, replaceEngine := range []bool{false, true} {
		t.Run(fmt.Sprint("replace-engine=", replaceEngine), func(t *testing.T) {
			a := onDemandTestApp(t)
			requests := make(chan context.Context, 4)
			a.onDemand.reader = onDemandReadFunc(func(ctx context.Context) (desktop.WiFiState, error) {
				requests <- ctx
				<-ctx.Done()
				return desktop.WiFiState{}, ctx.Err()
			})
			await := func() context.Context {
				t.Helper()
				select {
				case ctx := <-requests:
					return ctx
				case <-time.After(time.Second):
					t.Fatal("monitor did not read")
					return nil
				}
			}
			a.syncOnDemand()
			first := await()
			generation := a.onDemand.generation
			a.acceptOnDemand(generation, desktop.WiFiState{Connected: true, SSID: "Home"}, nil)
			if replaceEngine {
				old := a.engine
				a.engine = NewEngine(a.store, "must-not-launch", io.Discard)
				t.Cleanup(old.cancel)
			} else {
				a.onDemand.cancel()
			}
			// A result already queued by the old reader is also stale before
			// syncOnDemand observes the canceled/replaced owner.
			a.acceptOnDemand(generation, desktop.WiFiState{}, nil)
			if !a.onDemandSuspended() {
				t.Fatal("accepted a queued result from the previous monitor")
			}
			a.syncOnDemand()
			second := await()
			if first.Err() == nil || second.Err() != nil || a.onDemand.generation == generation {
				t.Fatal("monitor was not replaced", first.Err(), second.Err())
			}
			current := a.onDemand.generation
			a.syncOnDemand()
			if a.onDemand.generation != current {
				t.Fatal("unchanged live monitor restarted")
			}
			a.acceptOnDemand(current, desktop.WiFiState{}, nil)
			assertOnDemandIntent(t, a, true)
		})
	}
}

func TestOnDemandDoesNotRearmOnCanceledEngine(t *testing.T) {
	a := onDemandTestApp(t)
	a.onDemand.reader = onDemandReadFunc(func(context.Context) (desktop.WiFiState, error) {
		t.Error("queried for a canceled engine")
		return desktop.WiFiState{}, nil
	})
	a.engine.cancel()
	a.syncOnDemand()
	if a.onDemand.cancel != nil {
		t.Fatal("armed a monitor on a canceled engine")
	}
}

func TestOnDemandStartupKeepsRunIntentUntilFirstUIResult(t *testing.T) {
	a := onDemandTestApp(t)
	result := make(chan desktop.WiFiState, 1)
	a.onDemand.reader = onDemandReadFunc(func(ctx context.Context) (desktop.WiFiState, error) {
		select {
		case state := <-result:
			return state, nil
		case <-ctx.Done():
			return desktop.WiFiState{}, ctx.Err()
		}
	})
	a.syncOnDemand()
	a.reconcileOnDemand()
	// Like the source's initially null currentSSID, the first pending read
	// is not a known exclusion. This is deliberately not fail-closed.
	assertOnDemandIntent(t, a, true)
	result <- desktop.WiFiState{Connected: true, SSID: "Home"}
	deadline := time.Now().Add(time.Second)
	for {
		a.updateMu.Lock()
		queued := len(a.updates)
		a.updateMu.Unlock()
		if queued > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first result was not queued")
		}
		time.Sleep(time.Millisecond)
	}
	assertOnDemandIntent(t, a, true)
	ui.NewTester(a.View, 1200, 800)
	assertOnDemandIntent(t, a, false)
	if !a.wantRunning {
		t.Fatal("startup exclusion ended the user's run")
	}
}

func TestOnDemandWithoutExclusionsDoesNotReadSSID(t *testing.T) {
	a := onDemandTestApp(t)
	a.cfg.ExcludeSSIDs = nil
	a.onDemand.reader = onDemandReadFunc(func(context.Context) (desktop.WiFiState, error) {
		t.Error("read SSID with no exclusions")
		return desktop.WiFiState{}, nil
	})
	a.apply()
	if a.onDemand.cancel != nil {
		t.Fatal("empty list started a monitor")
	}
	assertOnDemandIntent(t, a, true)
}

func TestOnDemandRealCoreParksWithoutReplacingOwner(t *testing.T) {
	binary := os.Getenv("FLCLASH_TEST_CORE")
	if binary == "" {
		t.Skip("requires the built core; SSIDs remain synthetic")
	}
	a := onDemandTestApp(t)
	a.engine.binary = binary
	a.cfg.SystemProxy = false
	a.cfg.Overrides["mixed-port"] = 0
	a.cfg.Overrides["dns"] = map[string]any{"enable": false}
	a.cfg.Overrides["tun"] = map[string]any{"enable": false}
	a.cfg.Overrides["external-controller"] = ""
	type status struct {
		name     string
		err      error
		revision uint64
	}
	events := make(chan status, 32)
	a.engine.Status = func(name string, err error, revision uint64) { events <- status{name, err, revision} }
	a.engine.Run()
	t.Cleanup(a.engine.Close)
	await := func(want string) {
		t.Helper()
		revision := a.engine.Revision()
		timer := time.NewTimer(20 * time.Second)
		defer timer.Stop()
		for {
			select {
			case got := <-events:
				if got.revision != revision || got.name == "connecting" {
					continue
				}
				if got.err != nil || got.name != want {
					t.Fatalf("got %s: %v, wanted %s", got.name, got.err, want)
				}
				return
			case <-timer.C:
				t.Fatal("engine did not converge to", want)
			}
		}
	}
	a.reconcileOnDemand()
	await("running")
	a.engine.mu.Lock()
	owner := a.engine.client
	a.engine.mu.Unlock()
	a.acceptOnDemand(a.onDemand.generation, desktop.WiFiState{Connected: true, SSID: "Home"}, nil)
	await("stopped")
	a.engine.mu.Lock()
	sameOwner := a.engine.client == owner
	a.engine.mu.Unlock()
	if !sameOwner || !a.engine.Connected() || !a.wantRunning {
		t.Fatal("parking replaced the owner, disconnected IPC, or ended the run")
	}
	a.wantRunning = false
	a.reconcileOnDemand()
	a.acceptOnDemand(a.onDemand.generation, desktop.WiFiState{}, nil)
	await("stopped")
	a.wantRunning = true
	a.reconcileOnDemand()
	await("running")
}

func TestOnDemandEditsAreTransactionalAndApplyUsesParkedIntent(t *testing.T) {
	a := onDemandTestApp(t)
	a.onDemand.reader = onDemandReadFunc(func(ctx context.Context) (desktop.WiFiState, error) {
		<-ctx.Done()
		return desktop.WiFiState{}, ctx.Err()
	})
	a.acceptOnDemand(a.onDemand.generation, desktop.WiFiState{Connected: true, SSID: "Home"}, nil)
	a.cfg.Overrides["mode"] = "direct"
	a.apply()
	assertOnDemandIntent(t, a, false)
	if a.engine.desired.Overrides["mode"] != "direct" {
		t.Fatal("on-demand discarded a newer configuration")
	}
	before := slices.Clone(a.cfg.ExcludeSSIDs)
	for _, invalid := range [][]string{{""}, {"same", "same"}} {
		if err := a.setExcludeSSIDs(invalid); err == nil || !slices.Equal(before, a.cfg.ExcludeSSIDs) {
			t.Fatal("invalid edit changed settings", err)
		}
	}
	// Force an atomic-save failure, without changing filesystem permissions.
	originalDir := a.store.Dir
	a.store.Dir = filepath.Join(a.store.Dir, "does-not-exist")
	if err := a.setExcludeSSIDs([]string{"Other"}); err == nil || !slices.Equal(before, a.cfg.ExcludeSSIDs) {
		t.Fatal("failed save changed the active list", err)
	}
	a.store.Dir = originalDir
	assertOnDemandIntent(t, a, false)
}
