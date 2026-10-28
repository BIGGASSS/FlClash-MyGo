package app

import (
	"time"

	"github.com/egoist/mygo"
)

// Install only in a real, non-safe desktop window. Headless/safe views never
// register OS observers or start a core. The toolkit delivers these callbacks
// on its main thread; Update also requests the corresponding native rebuild.
func (a *App) observeProbeHost() {
	s := a.outboundState()
	if s.observing || a.window == nil || a.safe || a.quitting {
		return
	}
	s.observing = true
	var offs []func()
	offs = append(offs,
		mygo.Network.OnChanged(func(mygo.NetworkStatus) { a.update(a.probeHostChanged) }),
		mygo.Power.OnResume(func() { a.update(a.probeHostChanged) }),
		a.window.OnShow(func() { a.update(func() { a.setProbeHostVisible(true) }) }),
		a.window.OnHide(func() { a.update(func() { a.setProbeHostVisible(false) }) }),
	)
	a.window.OnClosed(func() {
		for _, off := range offs {
			off()
		}
		s.observing = false
	})
}

func (a *App) probeVisible() bool {
	return !a.quitting && (a.outbound == nil || !a.outbound.hidden) && (a.window == nil || a.window.IsVisible())
}

func (a *App) setProbeHostVisible(visible bool) {
	s := a.outboundState()
	s.hidden = !visible
	s.watched = map[string]bool{}
	if visible {
		s.routeResync = true
		a.markServiceRoutes()
		return
	}
	// Hidden windows do not build, so watchOutbound cannot release these.
	for ip, job := range s.qualityJobs {
		job.cancel()
		delete(s.qualityJobs, ip)
	}
	a.qualityBusy = false
	s.routeWanted = false
	s.routeWatchRetry = time.Time{}
	a.startProbeRouteWatch()
}

// Connectivity can change the public address without changing any proxy name
// or core route stamp. Like the source host epoch, this invalidates all old
// routes and owns pending replies. Resume also clears them conservatively:
// the native network API may coalesce same-online network changes.
func (a *App) probeHostChanged() {
	if a.quitting {
		return
	}
	s := a.outboundState()
	s.hostEpoch++
	a.syncOutboundOwner()
	a.services = nil
	a.servicePending = map[string]uint64{}
	a.serviceLast = map[string]time.Time{}
	a.markServiceRoutes()
}
