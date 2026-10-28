package app

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/coreclient"
	"github.com/BIGGASSS/FlClash-MyGo/internal/ipquality"
	"github.com/egoist/mygo/ui"
)

// Target identities, stamps, node backfill and timings follow the pinned
// lib/providers/{outbound_ip,routed_probe}.dart. Successful addresses have no
// wall-clock TTL: they stand only while the core can verify their recorded exits.
const outboundThrottle = 5 * time.Second

// Capture the IPC owner, not just Engine: a queued worker must never call a
// replacement core through Engine.CallContext after a restart.
type probeOwner struct {
	engine   *Engine
	client   *coreclient.Client
	revision uint64
	proxied  bool
	host     uint64
}

func (a *App) probeOwner() probeOwner {
	o := probeOwner{engine: a.engine, proxied: a.listenerIntent()}
	if a.outbound != nil {
		o.host = a.outbound.hostEpoch
	}
	if e := a.engine; e != nil {
		e.mu.Lock()
		o.client, o.revision = e.client, e.revision
		e.mu.Unlock()
	}
	return o
}

func (a *App) ownsProbe(o probeOwner) bool {
	return !a.safe && !a.quitting && o.engine != nil && o.engine.ctx != nil && o.engine.ctx.Err() == nil && a.probeOwner() == o
}

func (o probeOwner) call(ctx context.Context, method string, args, out any) error {
	if o.client == nil {
		return errors.New("core is not connected")
	}
	return o.client.Call(ctx, method, args, out)
}

type routeCheck struct {
	CoreEpoch    uint64 `json:"core-epoch"`
	RouteVersion uint64 `json:"route-version"`
	Held         []bool `json:"held"`
}

type outboundResult struct {
	URL          string     `json:"url"`
	Body         string     `json:"body"`
	Error        string     `json:"error"`
	Chains       []string   `json:"chains"`
	CoreEpoch    uint64     `json:"core-epoch"`
	RouteVersion uint64     `json:"route-version"`
	Hits         []routeHit `json:"hits"`
	NodeHits     []routeHit `json:"node-hits"`
}

type outboundAnswer struct {
	info    ipquality.Info
	epoch   uint64
	version uint64
	hits    []routeHit
}

type outboundEntry struct {
	serial  uint64
	value   *ipquality.Info
	answer  *outboundAnswer // not exposed until checkRoutes accepts it
	epoch   uint64
	version uint64
	hits    []routeHit
	pending bool
	failed  bool
	dirty   bool
	last    time.Time
	cancel  context.CancelFunc
}

type outboundProbeState struct {
	owner              probeOwner
	hostEpoch          uint64
	observing          bool
	hidden             bool
	serviceJobs        map[uint64]context.CancelFunc
	serviceVerify      uint64
	serviceVerifyStop  context.CancelFunc
	entries            map[string]*outboundEntry
	watched            map[string]bool
	serial             uint64
	due                time.Time
	verifying          bool
	qualityFromService bool
	qualityJobs        map[string]*qualityJob
	qualitySaves       qualityPersistence
	routeWatching      bool
	routeWanted        bool
	routeResync        bool
	routeWatchRun      bool
	routeWatchRetry    time.Time
	routeRetryTimer    *time.Timer
	routeSynced        bool
	routeEpoch         uint64
	routeVersion       uint64
}

func (a *App) outboundState() *outboundProbeState {
	if a.outbound == nil {
		a.outbound = &outboundProbeState{entries: map[string]*outboundEntry{}, watched: map[string]bool{}, qualityJobs: map[string]*qualityJob{}, serviceJobs: map[uint64]context.CancelFunc{}}
	}
	return a.outbound
}

func (a *App) syncOutboundOwner() *outboundProbeState {
	s := a.outboundState()
	o := a.probeOwner()
	if s.owner != o {
		sameClient := s.owner.client != nil && s.owner.client == o.client && s.owner.engine == o.engine
		for _, entry := range s.entries {
			if entry.cancel != nil {
				entry.cancel()
			}
		}
		for serial, cancel := range s.serviceJobs {
			cancel()
			delete(s.serviceJobs, serial)
		}
		if s.serviceVerifyStop != nil {
			s.serviceVerifyStop()
			s.serviceVerifyStop = nil
		}
		s.serviceVerify++
		a.serviceRouteRun = false
		if s.owner.engine != nil {
			a.ip, a.country, a.organization = "", "", ""
			a.services = nil
			a.servicePending = map[string]uint64{}
			a.serviceLast = map[string]time.Time{}
			a.markServiceRoutes()
		}
		for ip, job := range s.qualityJobs {
			if job.owner != o {
				job.cancel()
				delete(s.qualityJobs, ip)
			}
		}
		a.qualityBusy = len(s.qualityJobs) > 0
		a.checkIP, a.ipPending = false, false
		s.owner, s.entries, s.watched = o, map[string]*outboundEntry{}, map[string]bool{}
		s.due, s.verifying = time.Time{}, false
		if sameClient {
			// Keep the serialized subscription in flight across host intents.
			// Its completion converges to routeWanted before a new watch call.
			s.routeResync = true
		} else {
			if s.routeRetryTimer != nil {
				s.routeRetryTimer.Stop()
				s.routeRetryTimer = nil
			}
			s.routeWatching, s.routeWatchRun, s.routeWatchRetry = false, false, time.Time{}
			s.routeResync = false
		}
		s.routeSynced, s.routeEpoch, s.routeVersion = false, 0, 0
	}
	return s
}

func (a *App) markOutboundRoutes() {
	if a.outbound == nil {
		return
	}
	s := a.outbound
	for _, entry := range s.entries {
		entry.dirty = true
	}
	// Bounded settling: successive pick polls must not postpone checks forever.
	if s.due.IsZero() {
		s.due = time.Now().Add(1500 * time.Millisecond)
	}
}

func (a *App) requestDashboardOutbound() { a.requestOutbound("", true) }

func (a *App) requestOutbound(target string, force bool) {
	if a.safe || !a.probeVisible() || a.engine == nil {
		return
	}
	s := a.syncOutboundOwner()
	if !a.ownsProbe(s.owner) {
		return
	}
	previous := s.entries[target]
	if previous != nil {
		if previous.pending || !force && (previous.value != nil || time.Since(previous.last) < outboundThrottle) {
			return
		}
	}
	s.serial++
	entry := &outboundEntry{serial: s.serial, pending: true, last: time.Now()}
	if previous != nil && force {
		entry.value, entry.epoch, entry.version, entry.hits = previous.value, previous.epoch, previous.version, previous.hits
	}
	s.entries[target] = entry
	o, routeRevision := s.owner, a.serviceRouteRev
	proxy := target
	if target == "" && !o.proxied {
		proxy = "DIRECT"
	}
	ctx, cancel := context.WithTimeout(o.engine.ctx, 17*time.Second)
	entry.cancel = cancel
	if target == "" {
		a.checkIP = true
	}
	a.work(func() (func(), error) {
		var result outboundResult
		err := o.call(ctx, "outboundIp", map[string]any{"proxy-name": proxy, "urls": ipquality.OutboundSources, "timeout": 6000}, &result)
		info, parseErr := ipquality.ParseOutbound(result.URL, result.Body)
		return func() {
			defer cancel()
			if !a.ownsProbe(o) {
				a.syncOutboundOwner()
				return
			}
			if s.entries[target] != entry {
				return
			}
			entry.cancel = nil
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				if previous != nil {
					s.entries[target] = previous
				} else {
					delete(s.entries, target)
				}
				if target == "" {
					a.checkIP = false
				}
				return
			}
			if err != nil || result.Error != "" || parseErr != nil {
				entry.pending, entry.failed, entry.value = false, true, nil
				if target == "" {
					a.checkIP = false
					a.ip, a.country, a.organization = "", "", ""
				}
				return
			}
			entry.answer = &outboundAnswer{info, result.CoreEpoch, result.RouteVersion, slices.Clone(result.Hits)}
			entry.dirty = true
			// The leaf's route is not the parent target's route (notably for
			// load balancers). Use explicit node-hits unless the core request
			// was already pinned to that exact leaf, e.g. stopped -> DIRECT.
			if len(result.Chains) > 0 && result.Chains[0] != target {
				node := result.Chains[0]
				nodeHits := result.NodeHits
				if node == proxy {
					nodeHits = result.Hits
				}
				old := s.entries[node]
				if len(nodeHits) > 0 && (old == nil || old.serial <= entry.serial) {
					if old != nil && old.cancel != nil {
						old.cancel()
					}
					s.entries[node] = &outboundEntry{serial: entry.serial, pending: true, dirty: true, last: entry.last,
						answer: &outboundAnswer{info, result.CoreEpoch, result.RouteVersion, slices.Clone(nodeHits)}}
				}
			}
			if routeRevision != a.serviceRouteRev {
				a.markOutboundRoutes()
			}
			// A landing is verified immediately, rather than briefly exposing
			// an address returned under a route that moved while it was out.
			a.verifyOutboundRoutes()
		}, nil
	})
}

// Called from watchServices on every build, even away from the dashboard.
func (a *App) watchOutbound(c *ui.Context) {
	if a.safe || a.engine == nil {
		return
	}
	s := a.syncOutboundOwner()
	a.observeProbeHost()
	visible := a.probeVisible()
	wanted := map[string]bool{}
	if visible && a.page == "dashboard" {
		wanted[""] = true
	}
	if visible && a.serviceOpen && !a.serviceManage {
		for _, target := range a.enabledServiceTargets() {
			if value := a.serviceValue(target.ID); value != nil && len(value.Chains) > 0 {
				wanted[value.Chains[0]] = true
			}
		}
	}
	a.watchProbeRoutes(c, visible && (len(wanted) > 0 || a.page == "dashboard"))
	previous := s.watched
	s.watched = wanted
	qualityWanted := map[string]bool{}
	for target := range wanted {
		entry := s.entries[target]
		if entry == nil || entry.value == nil && !entry.pending && (!entry.failed || !previous[target]) {
			if entry != nil && time.Since(entry.last) < outboundThrottle {
				c.After(outboundThrottle - time.Since(entry.last))
				// Preserve the reattachment intent until its cooldown expires.
				delete(s.watched, target)
			} else {
				a.requestOutbound(target, false)
			}
		}
		if entry != nil && entry.value != nil {
			qualityWanted[entry.value.IP] = true
			a.lookupQuality(entry.value.IP, false)
		}
	}
	if visible && a.qualityOpen {
		qualityWanted[a.qualityIP] = true
		a.lookupQuality(a.qualityIP, false)
	}
	for ip := range qualityWanted {
		if record, ok := a.qualities[ip]; ok && qualityFresh(record, time.Now()) {
			c.After(time.Until(record.Checked.Add(qualityTTL(record))))
		}
	}
	for ip, job := range s.qualityJobs {
		if !qualityWanted[ip] {
			job.cancel()
			delete(s.qualityJobs, ip)
		}
	}
	a.qualityBusy = len(s.qualityJobs) > 0
	if !visible {
		return
	}
	if !s.due.IsZero() {
		if wait := time.Until(s.due); wait > 0 {
			c.After(wait)
		} else {
			a.verifyOutboundRoutes()
		}
	}
}

func (a *App) verifyOutboundRoutes() {
	s := a.outboundState()
	if s.verifying || !a.probeVisible() || !a.ownsProbe(s.owner) {
		return
	}
	var targets []string
	var entries []*outboundEntry
	var routes [][]routeHit
	for target, entry := range s.entries {
		if !entry.dirty || entry.pending && entry.answer == nil {
			continue
		}
		hits := entry.hits
		if entry.answer != nil {
			hits = entry.answer.hits
		}
		targets, entries, routes = append(targets, target), append(entries, entry), append(routes, slices.Clone(hits))
	}
	s.due = time.Time{}
	if len(targets) == 0 {
		return
	}
	s.verifying = true
	o, revision := s.owner, a.serviceRouteRev
	a.work(func() (func(), error) {
		ctx, cancel := context.WithTimeout(o.engine.ctx, 10*time.Second)
		defer cancel()
		var checked routeCheck
		err := o.call(ctx, "checkRoutes", map[string]any{"routes": routes}, &checked)
		return func() {
			if !a.ownsProbe(o) || s.owner != o {
				return
			}
			s.verifying = false
			if !a.probeVisible() {
				// A hidden window cannot consume or verify a landing. Resume
				// rechecks all dirty entries before accepting their addresses.
				s.due = time.Now()
				return
			}
			if err == nil {
				s.rememberRoute(checked)
			}
			if revision != a.serviceRouteRev {
				s.due = time.Now().Add(300 * time.Millisecond)
				return
			}
			for i, target := range targets {
				entry := entries[i]
				if s.entries[target] != entry || entry.pending && entry.answer == nil {
					continue
				}
				epoch, version := entry.epoch, entry.version
				if entry.answer != nil {
					epoch, version = entry.answer.epoch, entry.answer.version
				}
				if err == nil && (checked.olderThan(epoch, version) || s.routeSynced && checked.olderThan(s.routeEpoch, s.routeVersion)) {
					// A late check is not evidence against a newer answer/stamp.
					s.due = time.Now().Add(300 * time.Millisecond)
					continue
				}
				held := err == nil && epoch == checked.CoreEpoch && i < len(checked.Held) && checked.Held[i]
				if !held {
					entry.value, entry.answer, entry.pending, entry.dirty = nil, nil, false, false
					entry.failed = false
					if !s.watched[target] {
						delete(s.entries, target)
					} else {
						if err != nil {
							entry.last = time.Now()
						}
						s.due = time.Now().Add(outboundThrottle)
					}
				} else {
					if answer := entry.answer; answer != nil {
						entry.value, entry.hits, entry.epoch = &answer.info, answer.hits, answer.epoch
					}
					entry.version = checked.RouteVersion
					entry.answer, entry.pending, entry.failed, entry.dirty = nil, false, false, false
				}
				if target == "" {
					a.checkIP = entry.pending
					a.ip, a.country, a.organization = "", "", ""
					if entry.value != nil {
						a.ip, a.country = entry.value.IP, entry.value.Country
						a.lookupQuality(a.ip, false)
					}
				}
			}
			// A different lookup may have landed while this check was out.
			for _, entry := range s.entries {
				if entry.dirty {
					s.due = time.Now().Add(300 * time.Millisecond)
					break
				}
			}
		}, nil
	})
}

func (a *App) serviceOutboundEntry(value *Service) *outboundEntry {
	if value == nil || len(value.Chains) == 0 || a.outbound == nil {
		return nil
	}
	return a.outbound.entries[value.Chains[0]]
}

func countryFlag(code string) string {
	if len(code) != 2 {
		return ""
	}
	runes := []rune(code)
	for i, r := range runes {
		if r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		if r < 'A' || r > 'Z' {
			return ""
		}
		runes[i] = 0x1f1e6 + r - 'A'
	}
	return string(runes)
}

func (a *App) serviceOutboundLine(c *ui.Context, target serviceTarget) {
	if a.servicePending[target.ID] != 0 {
		ui.Row(c).Height(17).Children(func() {
			ui.Box(c).Width(96).Height(12).Radius(6).Background(a.colors.inset)
		})
		return
	}
	value := a.serviceValue(target.ID)
	entry := a.serviceOutboundEntry(value)
	pending := false
	region, fallback := "", "—"
	if value != nil {
		region = value.Region
		if len(value.Chains) > 0 {
			fallback = value.Chains[0]
			pending = pending || entry == nil || entry.value == nil && !entry.failed
		}
	}
	if entry != nil && entry.value != nil {
		region = entry.value.Country
	}
	line := func() {
		ui.Row(c).Height(17).Gap(4).Children(func() {
			if flag := countryFlag(region); flag != "" {
				ui.Text(c, flag).Font("Twemoji").FontSize(fontSize(c, 13))
			}
			label := fallback
			if entry != nil && entry.value != nil {
				label = a.shownIP(entry.value.IP)
			} else if pending {
				ui.Box(c).Width(88).Height(12).Radius(6).Background(a.colors.inset)
				return
			}
			color := a.colors.muted
			if entry != nil && entry.value != nil {
				if quality := a.qualities[entry.value.IP].Quality; quality != nil {
					color = a.qualityColor(quality.Level())
				}
			}
			text := ui.Text(c, label).FontSize(fontSize(c, 13)).TextColor(color).SingleLine().Tooltip(label)
			if entry != nil && entry.value != nil && a.cfg.HideIP {
				text.Font("JetBrains Mono")
			}
		})
	}
	if entry != nil && entry.value != nil {
		ip := entry.value.IP
		ui.ButtonBase(c.Key("service-outbound")).Label(target.Label + " · " + a.t("outboundIp")).Tooltip(a.t("outboundIp")).OnClick(func() {
			a.showQuality(ip)
			a.outboundState().qualityFromService = true
		}).Children(line)
	} else {
		line()
	}
}
