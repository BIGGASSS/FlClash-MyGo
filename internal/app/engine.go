package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/coreclient"
	"github.com/BIGGASSS/FlClash-MyGo/internal/desktop"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

type Engine struct {
	store          *store.Store
	proxy          *desktop.Proxy
	binary         string
	stderr         io.Writer
	mu             sync.Mutex
	client         *coreclient.Client
	counts         *ConfigCounts
	desired        store.Settings
	running        bool
	revision       uint64
	restartQueued  bool
	restartPending bool
	wake           chan struct{}
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	Status         func(string, error, uint64)
	Sample         func(Sample)
	Events         func(Batch)
}

func NewEngine(s *store.Store, binary string, stderr io.Writer) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{store: s, proxy: desktop.NewProxy(s.Dir), binary: binary, stderr: stderr, wake: make(chan struct{}, 1), ctx: ctx, cancel: cancel, done: make(chan struct{})}
}
func (e *Engine) Run() { go e.loop() }
func (e *Engine) Reconcile(cfg store.Settings, running bool) {
	e.mu.Lock()
	e.desired = store.Clone(cfg)
	e.running = running
	e.revision++
	e.mu.Unlock()
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// Restart coalesces an in-progress restart and retains the latest desired
// configuration and listener intent. It never launches a second owner in parallel.
func (e *Engine) Restart() {
	e.mu.Lock()
	if !e.restartPending {
		e.restartQueued, e.restartPending = true, true
	}
	e.revision++
	e.mu.Unlock()
	select {
	case e.wake <- struct{}{}:
	default:
	}
}
func (e *Engine) Restarting() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.restartPending
}

func (e *Engine) Call(method string, args, out any) error {
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()
	return e.CallContext(ctx, method, args, out)
}
func (e *Engine) CallContext(ctx context.Context, method string, args, out any) error {
	e.mu.Lock()
	c := e.client
	e.mu.Unlock()
	if c == nil {
		return errors.New("core is not connected")
	}
	return c.Call(ctx, method, args, out)
}
func (e *Engine) Message(method string, args any) error {
	var message string
	if err := e.Call(method, args, &message); err != nil {
		return err
	}
	if message != "" {
		return errors.New(message)
	}
	return nil
}
func (e *Engine) Validate(b []byte) error {
	if _, err := store.ParseYAML(b); err != nil {
		return err
	}
	f, err := os.CreateTemp(e.store.Dir, "validate-*.yaml")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return e.Message("validateMyGoConfig", path)
}
func (e *Engine) loop() {
	defer close(e.done)
	defer func() {
		if err := e.proxy.Restore(); err != nil {
			fmt.Fprintf(e.stderr, "restore system proxy: %v\n", err)
		}
		e.mu.Lock()
		c := e.client
		e.client = nil
		e.mu.Unlock()
		if c != nil {
			_ = c.Close()
		}
	}()
	var digest [32]byte
	var initialized, running, elevated bool
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-e.wake:
		}
		for {
			if e.ctx.Err() != nil {
				return
			}
			e.mu.Lock()
			cfg, want, revision := store.Clone(e.desired), e.running, e.revision
			c := e.client
			restart := e.restartQueued
			e.mu.Unlock()
			if profile := e.store.ActiveProfile(cfg); profile != nil && profile.Selected != nil {
				cfg.Selected = profile.Selected
			}
			raw, err := effectiveConfig(e.store, cfg)
			if revision != e.Revision() {
				continue
			}
			if err == nil {
				err = e.store.PrepareCoreFiles()
			}
			configurationFailed := err != nil
			needElevation := false
			if raw != nil {
				needElevation = want && store.Bool(store.Map(raw, "tun")["enable"])
			}
			if err == nil && restart {
				e.mu.Lock()
				if e.revision != revision {
					e.mu.Unlock()
					continue
				}
				e.restartQueued = false
				e.mu.Unlock()
			}
			if err == nil {
				if e.Status != nil && (c == nil || want != running || restart) {
					e.Status("connecting", nil, revision)
				}
				dead := false
				if c != nil {
					select {
					case <-c.Done():
						dead = true
					default:
					}
				}
				if c != nil && (dead || restart || needElevation && !elevated) {
					e.mu.Lock()
					e.client = nil
					e.mu.Unlock()
					_ = e.proxy.Restore()
					_ = c.Close()
					c = nil
					initialized = false
					running = false
				}
				if c == nil {
					budget := 10 * time.Second
					if needElevation {
						budget = 2 * time.Minute
					}
					ctx, cancel := context.WithTimeout(e.ctx, budget)
					c, err = coreclient.LaunchWithPrivileges(ctx, e.binary, e.store.Dir, e.stderr, needElevation)
					cancel()
					if err == nil {
						ctx, cancel = context.WithTimeout(e.ctx, 10*time.Second)
						err = c.Call(ctx, "initClash", map[string]any{"home-dir": e.store.Dir, "version": 0}, nil)
						cancel()
					}
					if err == nil {
						elevated = needElevation
						e.mu.Lock()
						e.client = c
						e.mu.Unlock()
						go e.collect(c)
						go e.poll(c)
						_ = e.Call("startLog", nil, nil)
					} else if c != nil {
						_ = c.Close()
					}
				}
				if revision != e.Revision() {
					continue
				}
				if err == nil {
					text, marshalErr := store.YAML(raw)
					err = marshalErr
					if err == nil {
						selection, _ := json.Marshal(cfg.Selected)
						next := sha256.Sum256(append([]byte(text+cfg.TestURL), selection...))
						if !initialized || digest != next {
							err = store.AtomicWrite(filepath.Join(e.store.Dir, "config.yaml"), []byte(text))
							if err == nil {
								err = e.Message("applyMyGoConfig", map[string]any{"test-url": cfg.TestURL, "selected-map": cfg.Selected})
							}
							if err == nil {
								digest = next
								initialized = true
								count := func(value any) int {
									switch list := value.(type) {
									case []any:
										return len(list)
									case []string:
										return len(list)
									}
									return 0
								}
								e.mu.Lock()
								e.counts = &ConfigCounts{count(raw["proxy-groups"]), count(raw["proxies"]), count(raw["rules"])}
								e.mu.Unlock()
							}
						}
					}
					configurationFailed = err != nil
					if revision != e.Revision() {
						continue
					}
					if err == nil && want != running {
						method := "stopListener"
						if want {
							method = "startListener"
						}
						err = e.Call(method, nil, nil)
						if err == nil {
							running = want
						}
					}
					if err == nil && running {
						err = e.Message("checkMyGoListeners", nil)
					}
					if revision != e.Revision() {
						continue
					}
					if err == nil {
						if want && cfg.SystemProxy {
							err = e.proxy.Enable(store.Int(raw["mixed-port"]), cfg.Bypass)
						} else {
							err = e.proxy.Restore()
						}
					}
				}
			}
			e.mu.Lock()
			if revision != e.revision {
				e.mu.Unlock()
				continue
			}
			e.restartPending, e.restartQueued = false, false
			e.mu.Unlock()
			if err != nil {
				if !configurationFailed || !running || !want {
					_ = e.proxy.Restore()
					if c != nil {
						_ = e.Call("stopListener", nil, nil)
					}
					running = false
				}
				if e.Status != nil {
					status := "error"
					if running {
						status = "running"
					}
					e.Status(status, err, revision)
				}
			} else if e.Status != nil {
				status := "stopped"
				if running {
					status = "running"
				}
				e.Status(status, nil, revision)
			}
			break
		}
	}
}

// Connected reports IPC availability independently of listener/takeover state.
func (e *Engine) Connected() bool {
	e.mu.Lock()
	client := e.client
	e.mu.Unlock()
	if client == nil {
		return false
	}
	select {
	case <-client.Done():
		return false
	default:
		return true
	}
}

func (e *Engine) Revision() uint64 { e.mu.Lock(); defer e.mu.Unlock(); return e.revision }
func (e *Engine) poll(c *coreclient.Client) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-c.Done():
			if e.ctx.Err() == nil && e.current(c) && e.Status != nil {
				e.Status("error", fmt.Errorf("core disconnected: %w", c.Err()), e.Revision())
			}
			return
		case <-ticker.C:
		}
		ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
		var s Sample
		var appMemory runtime.MemStats
		runtime.ReadMemStats(&appMemory)
		s.AppHeap = appMemory.HeapInuse
		s.AppRSS = desktop.ResidentMemory()
		e.mu.Lock()
		onlyProxy := e.desired.OnlyProxy
		s.Counts = e.counts
		e.mu.Unlock()
		var wg sync.WaitGroup
		var errorsMu sync.Mutex
		var pollErr error
		for _, call := range []struct {
			method    string
			args, out any
		}{{"getTraffic", onlyProxy, &s.Traffic}, {"getTotalTraffic", onlyProxy, &s.Total}, {"getConnections", nil, &s.Connections}, {"getMemoryStats", nil, &s.Memory}, {"getProxies", nil, &s.Proxies}, {"getExternalProviders", nil, &s.Providers}} {
			wg.Add(1)
			go func(method string, args, out any) {
				defer wg.Done()
				if err := c.Call(ctx, method, args, out); err != nil {
					errorsMu.Lock()
					pollErr = err
					errorsMu.Unlock()
				}
			}(call.method, call.args, call.out)
		}
		wg.Wait()
		cancel()
		if pollErr == nil && e.current(c) && e.Sample != nil {
			e.Sample(s)
		}
	}
}
func (e *Engine) collect(c *coreclient.Client) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	batch := Batch{RequestTraffic: map[string]Traffic{}}
	flush := func() {
		if e.current(c) && e.Events != nil && (len(batch.Logs)+len(batch.DNS)+len(batch.Requests)+len(batch.Delays)+len(batch.RequestTraffic) > 0 || batch.Refresh) {
			e.Events(batch)
		}
		batch = Batch{RequestTraffic: map[string]Traffic{}}
	}
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			flush()
		case event, ok := <-c.Events():
			if !ok {
				flush()
				return
			}
			switch event.Type {
			case "log":
				l := Decode[LogEntry](event.Data)
				l.Time = time.Now()
				batch.Logs = appendLimit(batch.Logs, l, 5000)
			case "dns":
				batch.DNS = appendLimit(batch.DNS, Decode[DNSQuery](event.Data), 3000)
			case "request":
				batch.Requests = appendLimit(batch.Requests, Decode[Connection](event.Data), 2000)
			case "requestTraffic":
				var updates []struct {
					ID       string `json:"id"`
					Upload   int64  `json:"upload"`
					Download int64  `json:"download"`
				}
				if json.Unmarshal(event.Data, &updates) == nil {
					for _, u := range updates {
						batch.RequestTraffic[u.ID] = Traffic{Up: u.Upload, Down: u.Download}
					}
				}
			case "delay":
				var d NodeDelay
				if json.Unmarshal(event.Data, &d) == nil {
					batch.Delays = append(batch.Delays, d)
				}
			case "loaded", "routeChanged":
				batch.Refresh = true
			}
		}
	}
}
func appendLimit[T any](s []T, v T, limit int) []T {
	if len(s) >= limit {
		copy(s, s[len(s)-limit+1:])
		s = s[:limit-1]
	}
	return append(s, v)
}
func (e *Engine) Close() {
	e.cancel()
	select {
	case e.wake <- struct{}{}:
	default:
	}
	<-e.done
}
func (e *Engine) current(c *coreclient.Client) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.client == c
}
