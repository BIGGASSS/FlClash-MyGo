package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/desktop"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

const onDemandPollInterval = 3 * time.Second

// All fields belong to the UI thread. Background readers publish only through
// App.update (Window.Update, or the headless callback queue).
type onDemandState struct {
	View  onDemandViewState
	State desktop.WiFiState
	Known bool
	Error string

	reader     desktop.WiFiReader
	cancel     context.CancelFunc
	ctx        context.Context
	engine     *Engine
	generation uint64
	excludes   []string
}

func (a *App) onDemandSuspended() bool {
	return !a.safe && a.onDemand.Known && a.onDemand.State.Connected &&
		a.cfg.ExcludesSSID(a.onDemand.State.SSID)
}

// listenerIntent is a projection, not a second lifecycle owner. In particular,
// parking a listener never clears wantRunning, and Wi-Fi cannot undo a user stop.
func (a *App) listenerIntent() bool {
	return !a.safe && a.wantRunning && !a.onDemandSuspended()
}

// setExcludeSSIDs is the settings UI's transactional editing entry point. Import
// and backup retain source lists unchanged; new edits enforce source validation.
func (a *App) setExcludeSSIDs(ssids []string) error {
	seen := map[string]bool{}
	for _, ssid := range ssids {
		if err := store.ValidateSSID(ssid); err != nil {
			return err
		}
		if seen[ssid] {
			return errors.New("SSID already exists")
		}
		seen[ssid] = true
	}
	next := store.Clone(a.cfg)
	next.ExcludeSSIDs = append([]string{}, ssids...)
	if err := a.store.Save(next); err != nil {
		return err
	}
	a.cfg = next
	a.syncOnDemand()
	a.reconcileOnDemand()
	return nil
}

func (a *App) stopOnDemand() {
	a.onDemand.generation++
	if a.onDemand.cancel != nil {
		a.onDemand.cancel()
		a.onDemand.cancel = nil
	}
	a.onDemand.ctx, a.onDemand.engine = nil, nil
	a.onDemand.excludes = nil
}

// syncOnDemand runs after settings/import/restore and on engine initialization.
// Safe mode does not even instantiate a platform reader, check permissions, or
// start a monitoring goroutine.
func (a *App) syncOnDemand() {
	if a.safe || a.quitting || a.engine == nil || len(a.cfg.ExcludeSSIDs) == 0 {
		a.stopOnDemand()
		a.onDemand.State, a.onDemand.Known, a.onDemand.Error = desktop.WiFiState{}, false, ""
		return
	}
	if a.engine.ctx.Err() != nil {
		a.stopOnDemand()
		return
	}
	if a.onDemand.cancel != nil && a.onDemand.engine == a.engine &&
		a.onDemand.ctx != nil && a.onDemand.ctx.Err() == nil &&
		slices.Equal(a.onDemand.excludes, a.cfg.ExcludeSSIDs) {
		return
	}
	a.stopOnDemand()
	a.onDemand.excludes = append([]string{}, a.cfg.ExcludeSSIDs...)
	if a.onDemand.reader == nil {
		a.onDemand.reader = desktop.NewWiFiReader()
	}
	reader := a.onDemand.reader
	ctx, cancel := context.WithCancel(a.engine.ctx)
	a.onDemand.cancel, a.onDemand.ctx, a.onDemand.engine = cancel, ctx, a.engine
	generation := a.onDemand.generation
	go func() {
		for {
			query, finish := context.WithTimeout(ctx, 5*time.Second)
			state, err := reader.Read(query)
			finish()
			if ctx.Err() != nil {
				return
			}
			a.update(func() { a.acceptOnDemand(generation, state, err) })
			timer := time.NewTimer(onDemandPollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func (a *App) acceptOnDemand(generation uint64, state desktop.WiFiState, err error) {
	if a.safe || a.quitting || generation != a.onDemand.generation || len(a.cfg.ExcludeSSIDs) == 0 {
		return
	}
	// Cancellation/replacement may happen after a worker queued its callback,
	// before syncOnDemand has had a chance to advance the generation.
	if a.onDemand.ctx != nil && (a.onDemand.ctx.Err() != nil || a.onDemand.engine != a.engine) {
		return
	}
	previous := a.onDemandSuspended()
	if err == nil && state.Connected && state.SSID == "" {
		err = fmt.Errorf("%w: connected Wi-Fi has no readable SSID", desktop.ErrWiFiUnavailable)
	}
	if err != nil {
		// Permission/API failures are not network transitions. Keep the last
		// known association and surface the error separately; do not silently
		// resume an excluded connection or pretend it is excluded.
		a.onDemand.Error = err.Error()
	} else {
		a.onDemand.State, a.onDemand.Known, a.onDemand.Error = state, true, ""
	}
	if previous != a.onDemandSuspended() {
		a.reconcileOnDemand()
	}
}

func (a *App) reconcileOnDemand() {
	if a.safe || a.quitting || a.engine == nil {
		return
	}
	a.markServiceRoutes()
	a.engine.Reconcile(a.cfg, a.listenerIntent())
}
