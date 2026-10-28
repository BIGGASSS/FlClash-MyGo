package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

const maxDelayTests = 32

type delayTarget struct{ name, url string }

func (t delayTarget) key() string { return delayKey(t.name, t.url) }

type delayPending struct{ holds, runs int }

type proxyDelayJob struct {
	id       uint64
	active   string
	engine   *Engine
	revision uint64
	group    string
	targets  []delayTarget
	started  map[string]bool
	finished map[string]bool
	cancel   context.CancelFunc
}

type delayEvent struct {
	target  delayTarget
	started bool
	value   int
	err     error
}

func (a *App) resolveDelayTarget(name, fallback string) delayTarget {
	url := ""
	seen := map[string]bool{}
	for !seen[name] {
		seen[name] = true
		node, exists := a.sample.Proxies.Proxies[name]
		if !exists || len(node.All) == 0 && !slices.Contains([]string{"Selector", "URLTest", "Fallback", "LoadBalance", "Relay"}, node.Type) {
			break
		}
		selected := node.Now
		pick := a.cfg.Selected[name]
		if computedGroup(node.Type) {
			if selected == "" {
				selected = pick
			}
		} else if pick != "" && (len(node.All) == 0 || slices.Contains(node.All, pick)) {
			selected = pick
		}
		if selected == "" {
			break
		}
		name, url = selected, node.TestURL
	}
	if url == "" {
		url = fallback
	}
	if url == "" {
		url = a.cfg.TestURL
	}
	return delayTarget{name, url}
}

func (a *App) delayTargets(names []string, url string) []delayTarget {
	seen := map[string]bool{}
	targets := []delayTarget{}
	for _, name := range names {
		target := a.resolveDelayTarget(name, url)
		if target.name != "" && !seen[target.key()] {
			targets = append(targets, target)
			seen[target.key()] = true
		}
	}
	return targets
}

func (a *App) pageProxyGroup() string {
	groups := a.visibleProxyGroups()
	for _, group := range groups {
		if group.Name == a.group {
			return group.Name
		}
	}
	if len(groups) > 0 {
		return groups[0].Name
	}
	return ""
}

func (a *App) testGroup() { a.testPageGroup(a.pageProxyGroup()) }

func (a *App) testPageGroup(name string) {
	group, ok := a.sample.Proxies.Proxies[name]
	if !ok || a.delayGroups[name] != 0 {
		return
	}
	// The original page action follows search, while global actions test all
	// current-mode groups, independent of timeout hiding, sorting or filters.
	names := []string{}
	for _, name := range group.All {
		node := a.sample.Proxies.Proxies[name]
		if searchMatches(a.proxyViewState().proxyQuery, name, node.Type, node.Provider) {
			names = append(names, name)
		}
	}
	a.startDelayTests(a.delayTargets(names, group.TestURL), group.Name)
}

func (a *App) testNames(names []string, urls ...string) {
	url := ""
	if len(urls) > 0 {
		url = urls[0]
	}
	a.startDelayTests(a.delayTargets(names, url), "")
}

func (a *App) testCurrentGroups() {
	targets := []delayTarget{}
	seen := map[string]bool{}
	for _, group := range a.currentProxyGroups() {
		for _, target := range a.delayTargets(group.All, group.TestURL) {
			if !seen[target.key()] {
				targets = append(targets, target)
				seen[target.key()] = true
			}
		}
	}
	a.startDelayTests(targets, "")
}

func (a *App) startDelayTests(targets []delayTarget, group string) {
	if a.engine == nil || a.safe || len(targets) == 0 || a.quitting {
		return
	}
	e := a.engine
	e.mu.Lock()
	client, revision := e.client, e.revision
	e.mu.Unlock()
	if client == nil {
		return
	}
	ctx, cancel := context.WithCancel(e.ctx)
	a.delaySerial++
	job := &proxyDelayJob{id: a.delaySerial, active: a.cfg.Active, engine: e, revision: revision,
		group: group, targets: targets, started: map[string]bool{}, finished: map[string]bool{}, cancel: cancel}
	if a.delayJobs == nil {
		a.delayJobs = map[uint64]*proxyDelayJob{}
		a.delayGroups = map[string]uint64{}
		a.delayPending = map[string]delayPending{}
	}
	if a.delayPool == nil {
		a.delayPool = &proxyDelayPool{}
	}
	a.delayJobs[job.id] = job
	if group != "" {
		a.delayGroups[group] = job.id
	}
	if a.delaySnapshot == nil {
		a.snapshotDelays()
	}
	for _, target := range targets {
		key := target.key()
		pending := a.delayPending[key]
		pending.holds++
		a.delayPending[key] = pending
	}
	timeout := a.cfg.TestTimeout
	events := make(chan delayEvent, maxDelayTests*2)
	go func() {
		defer cancel()
		// Like the source, commit streams in bounded 300 ms batches rather than
		// rebuilding large virtual lists once per reply. Completion flushes at once.
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		batch := []delayEvent{}
		first := true
		flush := func(done bool) {
			updates := batch
			batch = nil
			a.update(func() { a.commitDelayEvents(job, updates, done) })
		}
		for {
			select {
			case event, ok := <-events:
				if !ok {
					flush(true)
					return
				}
				batch = append(batch, event)
				if first {
					first = false
					flush(false)
				}
			case <-ticker.C:
				if len(batch) > 0 {
					flush(false)
				}
			}
		}
	}()
	done := a.delayPool.submit(ctx, targets, func(ctx context.Context, target delayTarget) (int, error) {
		var delay *struct {
			Value int `json:"value"`
		}
		if e.Revision() != revision {
			return 0, context.Canceled
		}
		request, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		err := client.Call(request, "asyncTestDelay", map[string]any{
			"proxy-name": target.name, "test-url": target.url, "timeout": timeout,
		}, &delay)
		if err == nil && delay == nil {
			err = errors.New("delay test returned no result")
		}
		if err != nil {
			return 0, err
		}
		return delay.Value, err
	}, func(event delayEvent) { events <- event }, func(started []delayTarget) {
		// TaskPool starts its idle slots synchronously in the source. Publish
		// those runs before the click's next frame, not after the stream throttle.
		for _, target := range started {
			key := target.key()
			job.started[key] = true
			pending := a.delayPending[key]
			pending.runs++
			a.delayPending[key] = pending
		}
	})
	go func() { <-done; close(events) }()
}

// One shared FIFO pool bounds all page, individual and global tests together.
type proxyDelayPool struct {
	mu      sync.Mutex
	running int
	queue   []proxyDelayTask
}
type proxyDelayTask struct {
	ctx    context.Context
	target delayTarget
	query  func(context.Context, delayTarget) (int, error)
	emit   func(delayEvent)
	done   func()
}

func (p *proxyDelayPool) submit(ctx context.Context, targets []delayTarget, query func(context.Context, delayTarget) (int, error), emit func(delayEvent), onDispatch func([]delayTarget)) <-chan struct{} {
	var pending sync.WaitGroup
	pending.Add(len(targets))
	p.mu.Lock()
	for _, target := range targets {
		p.queue = append(p.queue, proxyDelayTask{ctx, target, query, emit, pending.Done})
	}
	tasks := p.take()
	p.mu.Unlock()
	if onDispatch != nil {
		started := make([]delayTarget, 0, len(tasks))
		for _, task := range tasks {
			started = append(started, task.target)
		}
		onDispatch(started)
	}
	p.launch(tasks)
	done := make(chan struct{})
	go func() { pending.Wait(); close(done) }()
	return done
}
func (p *proxyDelayPool) take() []proxyDelayTask {
	var tasks []proxyDelayTask
	for p.running < maxDelayTests && len(p.queue) > 0 {
		tasks = append(tasks, p.queue[0])
		p.queue[0] = proxyDelayTask{}
		p.queue = p.queue[1:]
		p.running++
	}
	if len(p.queue) == 0 {
		p.queue = nil
	}
	return tasks
}
func (p *proxyDelayPool) launch(tasks []proxyDelayTask) {
	for _, task := range tasks {
		go func() {
			if task.ctx.Err() == nil {
				task.emit(delayEvent{target: task.target, started: true})
				value, err := task.query(task.ctx, task.target)
				task.emit(delayEvent{target: task.target, value: value, err: err})
			}
			task.done()
			p.mu.Lock()
			p.running--
			next := p.take()
			p.mu.Unlock()
			p.launch(next)
		}()
	}
}

func (a *App) delayJobCurrent(job *proxyDelayJob) bool {
	return !a.quitting && a.engine == job.engine && a.cfg.Active == job.active && job.revision == job.engine.Revision()
}

func (a *App) commitDelayEvents(job *proxyDelayJob, events []delayEvent, done bool) {
	if a.delayJobs[job.id] != job {
		return
	}
	current := a.delayJobCurrent(job)
	if !current {
		job.cancel()
	}
	for _, event := range events {
		key := event.target.key()
		pending := a.delayPending[key]
		if event.started {
			if job.started[key] {
				continue
			}
			job.started[key] = true
			pending.runs++
		} else {
			if current && event.err == nil {
				a.setProxyDelay(event.target.name, event.target.url, event.value)
			}
			job.finished[key] = true
			pending.runs--
			pending.holds--
		}
		if pending.holds > 0 {
			a.delayPending[key] = pending
		} else {
			delete(a.delayPending, key)
		}
	}
	if done {
		for _, target := range job.targets {
			key := target.key()
			if job.finished[key] {
				continue
			}
			pending := a.delayPending[key]
			pending.holds--
			if job.started[key] {
				pending.runs--
			}
			if pending.holds > 0 {
				a.delayPending[key] = pending
			} else {
				delete(a.delayPending, key)
			}
		}
		delete(a.delayJobs, job.id)
		if a.delayGroups[job.group] == job.id {
			delete(a.delayGroups, job.group)
		}
		if current {
			a.snapshotDelays()
		}
	}
}

func (a *App) snapshotDelays() {
	a.delaySnapshot = make(map[string]int, len(a.delays))
	for key, value := range a.delays {
		a.delaySnapshot[key] = value
	}
}

func (a *App) cancelDelayTests() {
	for _, job := range a.delayJobs {
		job.cancel()
	}
	a.delayJobs, a.delayGroups, a.delayPending = nil, nil, nil
}

func (a *App) watchDelayTests() {
	for _, job := range a.delayJobs {
		if !a.delayJobCurrent(job) {
			job.cancel()
			a.commitDelayEvents(job, nil, true)
		}
	}
}

func (a *App) proxyTestButton(c *ui.Context) {
	name := a.pageProxyGroup()
	busy := a.delayGroups[name] != 0
	if busy {
		a.delayBusyButton(c, "test-delay", a.t("testDelay"), 41)
		return
	}
	button := ui.ButtonBase(c).Label(a.t("testDelay")).Tooltip(a.t("testDelay")).Size(41, 41).
		Radius(999).Background(a.colors.block).Center().OnClick(a.testGroup)
	button.Children(func() {
		glyph(c, "bolt").FontSize(21)
	})
}

func (a *App) delayBusyButton(c *ui.Context, key, label string, side float32) {
	ui.Box(c.Key(key)).Size(side, side).Children(func() {
		ui.ButtonBase(c).Label(label).Fill().Radius(999).Background(a.colors.block).Disabled(true)
		// Flutter's custom-painted loading glyph keeps its primary color inside
		// a disabled button. A native sibling avoids MyGo fading the glyph too.
		ui.Box(c).Absolute().Left(0).Top(0).Size(side, side).Center().PassThrough().Children(func() {
			loading(c, 20, a.colors.accent, a.t("loading"))
		})
	})
}
