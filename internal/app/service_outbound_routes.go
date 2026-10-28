package app

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"
)

func (snapshot routeCheck) olderThan(epoch, version uint64) bool {
	return snapshot.CoreEpoch < epoch || snapshot.CoreEpoch == epoch && snapshot.RouteVersion < version
}

func (s *outboundProbeState) rememberRoute(snapshot routeCheck) {
	if !s.routeSynced || !snapshot.olderThan(s.routeEpoch, s.routeVersion) {
		s.routeSynced, s.routeEpoch, s.routeVersion = true, snapshot.CoreEpoch, snapshot.RouteVersion
	}
}

// A reattachment snapshot can be older than a newer lookup's check response.
// Never let that late snapshot invalidate a result from the newer core epoch.
func (a *App) applyOutboundSnapshot(snapshot routeCheck) {
	s := a.outboundState()
	if s.routeSynced && snapshot.olderThan(s.routeEpoch, s.routeVersion) {
		return
	}
	moved := s.routeSynced && snapshot.CoreEpoch != s.routeEpoch
	s.rememberRoute(snapshot)
	for target, entry := range s.entries {
		epoch := entry.epoch
		if entry.answer != nil {
			epoch = entry.answer.epoch
		} else if entry.pending && entry.value == nil && !moved {
			// The first snapshot need not supersede a still-unstamped lookup.
			continue
		}
		if epoch >= snapshot.CoreEpoch {
			// A lookup can already carry a newer stamp while the first
			// subscription snapshot is still in flight.
			continue
		}
		if entry.cancel != nil {
			entry.cancel()
		}
		if s.watched[target] {
			// A new epoch bypasses the ordinary throttle, after the source's
			// short start/config settling interval.
			s.entries[target] = &outboundEntry{last: time.Now().Add(-outboundThrottle + 300*time.Millisecond)}
		} else {
			delete(s.entries, target)
		}
		if target == "" {
			a.ip, a.country, a.organization, a.checkIP = "", "", "", false
		}
	}
}

// URL-test/fallback moves are only published while the core route poll is
// watched. The owned subscription follows visible consumers, including a
// service sheet opened from Tools rather than the dashboard.
func (a *App) watchProbeRoutes(c *ui.Context, wanted bool) {
	s := a.outboundState()
	s.routeWanted = wanted
	if wait := time.Until(s.routeWatchRetry); wait > 0 {
		c.After(wait)
		return
	}
	a.startProbeRouteWatch()
}

func (a *App) startProbeRouteWatch() {
	s := a.outboundState()
	wanted := s.routeWanted
	if s.routeWatchRun || s.routeWatching == wanted && !s.routeResync || time.Now().Before(s.routeWatchRetry) {
		return
	}
	s.routeResync = false
	o := s.owner
	if !a.ownsProbe(o) {
		return
	}
	s.routeWatchRun = true
	a.work(func() (func(), error) {
		ctx, cancel := context.WithTimeout(o.engine.ctx, 10*time.Second)
		defer cancel()
		var snapshot routeCheck
		err := o.call(ctx, "watchRoute", wanted, &snapshot)
		return func() {
			if s.owner.client != o.client || s.owner.engine != o.engine {
				return
			}
			s.routeWatchRun = false
			if s.routeRetryTimer != nil {
				s.routeRetryTimer.Stop()
				s.routeRetryTimer = nil
			}
			if err != nil {
				// The RPC may have applied before its reply was lost. Retry
				// the latest intent even when local routeWatching looks equal.
				s.routeResync = true
				if s.routeWanted != wanted {
					s.routeWatchRetry = time.Time{}
					a.startProbeRouteWatch()
					return
				}
				s.routeWatchRetry = time.Now().Add(5 * time.Second)
				// Hidden windows do not build or run Context.After callbacks.
				s.routeRetryTimer = time.AfterFunc(5*time.Second, func() {
					a.update(func() {
						if s.owner.client == o.client && s.owner.engine == o.engine {
							a.startProbeRouteWatch()
						}
					})
				})
				return
			}
			s.routeWatching = wanted
			s.routeWatchRetry = time.Time{}
			if wanted && a.ownsProbe(o) {
				a.applyOutboundSnapshot(snapshot)
				// A hidden window may have missed route notifications. All
				// cached answers must be reverified, even if names look equal.
				a.markServiceRoutes()
			}
			// Visibility can change without another build (hidden windows do
			// not draw). Converge the latest subscription intent here too.
			if s.routeWanted != s.routeWatching || s.routeResync {
				a.startProbeRouteWatch()
			}
		}, nil
	})
}
