package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/coreclient"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"go.yaml.in/yaml/v3"
)

const providerStateKey = "\x00provider-state"
const providerSizeLimit = 32 << 20

type providerSyncState struct {
	slots        chan struct{}
	subscription *providerSubscription
	infoOpen     bool
	batch        *providerFailures
}

func (a *App) providerState() *providerSyncState {
	if a.providerFiles == nil {
		a.providerFiles = map[string]*providerFileProbe{}
	}
	if a.providerFiles[providerStateKey] == nil {
		a.providerFiles[providerStateKey] = &providerFileProbe{state: &providerSyncState{slots: make(chan struct{}, 4)}}
	}
	return a.providerFiles[providerStateKey].state
}

type providerUpdateOwner struct {
	engine   *Engine
	client   *coreclient.Client
	revision uint64
	active   string
	provider Provider
	key      string
	serial   uint64
	listed   bool
}

func (a *App) providerOwnerCurrent(owner providerUpdateOwner) bool {
	return a.providerPending[owner.key] == owner.serial && a.providerIdentityCurrent(owner)
}

func (a *App) providerIdentityCurrent(owner providerUpdateOwner) bool {
	if a.engine != owner.engine || a.cfg.Active != owner.active || !providerClientCurrent(owner.engine, owner.client, owner.revision) {
		return false
	}
	for _, p := range a.sample.Providers {
		if p.Name == owner.provider.Name && p.Type == owner.provider.Type {
			return p.Path == owner.provider.Path
		}
	}
	return !owner.listed
}

type providerFailures struct {
	remaining int
	messages  []providerUpdateFailure
	changes   []providerRuleChange
	settings  *store.Settings
	engine    *Engine
	revision  uint64
}

type providerUpdateFailure struct {
	owner providerUpdateOwner
	text  string
}

type providerRuleChange struct {
	owner    providerUpdateOwner
	name     string
	prepared *preparedRuleSync
}

func (a *App) updateProviderBatch(providers []Provider) {
	if a.safe || a.engine == nil {
		return
	}
	state := a.providerState()
	batch := state.batch
	if batch == nil || batch.remaining == 0 || batch.engine != a.engine || batch.revision != a.engine.Revision() || !sameResourceIntent(a.cfg, *batch.settings) {
		cfg := store.Clone(a.cfg)
		batch = &providerFailures{settings: &cfg, engine: a.engine, revision: a.engine.Revision()}
		state.batch = batch
	}
	for _, p := range providers {
		if a.startProviderUpdate(p, batch) {
			batch.remaining++
		}
	}
}

func (a *App) startProviderUpdate(p Provider, batch *providerFailures) bool {
	if a.safe || a.engine == nil || a.providerPending[a.providerKey(p)] != 0 {
		return false
	}
	e := a.engine
	e.mu.Lock()
	client, revision := e.client, e.revision
	e.mu.Unlock()
	if client == nil {
		return false
	}
	if a.providerPending == nil {
		a.providerPending = map[string]uint64{}
	}
	a.providerSerial++
	owner := providerUpdateOwner{engine: e, client: client, revision: revision, active: a.cfg.Active, provider: p, key: a.providerKey(p), serial: a.providerSerial}
	for _, listed := range a.sample.Providers {
		owner.listed = owner.listed || (listed.Name == p.Name && listed.Type == p.Type)
	}
	a.providerPending[owner.key] = owner.serial
	cfg := *batch.settings
	s, state := a.store, a.providerState()
	requestClient := a.httpClient()
	requestClient.Timeout = 20 * time.Second
	agent := store.String(a.cfg.Overrides["global-ua"])
	acquired := false
	finish := func(err error) {
		if acquired {
			<-state.slots
			acquired = false
		}
		current := a.providerOwnerCurrent(owner) && sameResourceIntent(a.cfg, cfg)
		if a.providerPending[owner.key] == owner.serial {
			delete(a.providerPending, owner.key)
		}
		if current && err != nil {
			batch.messages = append(batch.messages, providerUpdateFailure{owner: owner, text: p.Name + ": " + err.Error()})
		}
		batch.remaining--
		if batch.remaining != 0 {
			return
		}
		defer func() {
			for _, change := range batch.changes {
				change.prepared.discard()
			}
			if state.batch == batch {
				state.batch = nil
			}
		}()
		batchCurrent := a.engine == e && a.cfg.Active == owner.active && e.Revision() == revision && sameResourceIntent(a.cfg, cfg)
		if !batchCurrent {
			return
		}
		messages := []string{}
		for _, failure := range batch.messages {
			if a.providerIdentityCurrent(failure.owner) {
				messages = append(messages, failure.text)
			}
		}
		changes := []providerRuleChange{}
		for _, change := range batch.changes {
			if a.providerIdentityCurrent(change.owner) {
				changes = append(changes, change)
			}
		}
		if len(changes) > 0 {
			if err := a.applyProviderRuleChanges(e, changes); err != nil {
				for _, change := range changes {
					messages = append(messages, change.name+": "+err.Error())
				}
			}
		}
		if len(messages) > 0 {
			a.err = strings.Join(messages, "\n\n")
		}
	}
	go func() {
		defer requestClient.CloseIdleConnections()
		queueCtx, queueCancel := context.WithTimeout(e.ctx, 60*time.Second)
		defer queueCancel()
		select {
		case state.slots <- struct{}{}:
			acquired = true
		case <-queueCtx.Done():
			a.update(func() { finish(queueCtx.Err()) })
			return
		}
		if e.Revision() != revision {
			a.update(func() { finish(nil) })
			return
		}
		ctx, cancel := providerSyncContext(e, client, revision, 60*time.Second)
		defer cancel()
		var resource store.RuleResource
		remote := false
		if p.Type == "Rule" && (strings.EqualFold(p.Vehicle, "File") || strings.EqualFold(p.Vehicle, "HTTP")) {
			resource, remote = s.RemoteRuleResource(cfg, p.Path)
		}
		if !remote {
			var message string
			callCtx, callCancel := context.WithTimeout(ctx, 30*time.Second)
			defer callCancel()
			err := client.Call(callCtx, "updateExternalProviderAtPath", map[string]string{"providerName": p.Name, "provider-path": p.Path}, &message)
			if err == nil && message != "" {
				err = errors.New(message)
			}
			a.update(func() { finish(err) })
			return
		}
		prepared, err := prepareRuleSync(ctx, requestClient, agent, client, s, cfg, p, resource)
		if err != nil {
			a.update(func() { finish(err) })
			return
		}
		a.update(func() {
			if !a.providerOwnerCurrent(owner) || !sameResourceIntent(a.cfg, cfg) {
				prepared.discard()
				finish(nil)
				return
			}
			if prepared.shapeChanged {
				batch.changes = append(batch.changes, providerRuleChange{owner: owner, name: p.Name, prepared: prepared})
				finish(nil)
				return
			}
			if err := prepared.commit(); err != nil {
				prepared.discard()
				finish(err)
				return
			}
			go func() {
				ctx, cancel := providerSyncContext(e, client, revision, 30*time.Second)
				defer cancel()
				var message string
				err := client.Call(ctx, "updateExternalProviderAtPath", map[string]string{"providerName": p.Name, "provider-path": p.Path}, &message)
				if err == nil && message != "" {
					err = errors.New(message)
				}
				a.update(func() {
					defer prepared.discard()
					if !a.providerOwnerCurrent(owner) || !sameResourceIntent(a.cfg, cfg) {
						finish(nil)
						return
					}
					if err != nil {
						err = errors.Join(err, prepared.restore())
					}
					finish(err)
				})
			}()
		})
	}()
	return true
}

func providerClientCurrent(e *Engine, client *coreclient.Client, revision uint64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.client == client && e.revision == revision && e.ctx.Err() == nil
}

func providerSyncContext(e *Engine, client *coreclient.Client, revision uint64, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(e.ctx, timeout)
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !providerClientCurrent(e, client, revision) {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}

func (a *App) applyProviderRuleChanges(e *Engine, changes []providerRuleChange) error {
	next := store.Clone(a.cfg)
	for _, change := range changes {
		p := change.prepared
		var err error
		next, err = a.store.WithRuleResource(next, change.name, p.resource, p.id, p.sourceID, p.behavior, p.format)
		if err != nil {
			return err
		}
	}
	if err := a.store.Save(next); err != nil {
		return err
	}
	a.cfg = next
	for _, change := range changes {
		change.prepared.keep = true
	}
	a.markServiceRoutes()
	e.Reconcile(a.cfg, a.listenerIntent())
	return nil
}

func sameResourceIntent(latest, before store.Settings) bool {
	// These fields identify the library and its originating definition. Other
	// preferences may change without invalidating a download.
	if latest.Active != before.Active || !slices.Equal(latest.Resources, before.Resources) || !bytes.Equal(latest.ImportSnapshot, before.ImportSnapshot) {
		return false
	}
	profileIntent := func(cfg store.Settings) string {
		for _, p := range cfg.Profiles {
			if p.ID == cfg.Active {
				return p.URL + "\x00" + p.Override
			}
		}
		return ""
	}
	left, _ := json.Marshal(latest.Overrides["rule-providers"])
	right, _ := json.Marshal(before.Overrides["rule-providers"])
	return profileIntent(latest) == profileIntent(before) && bytes.Equal(left, right)
}

type preparedRuleSync struct {
	writes       []*store.PreparedResource
	backups      map[string]*store.PreparedResource
	committed    bool
	shapeChanged bool
	newPaths     []string
	keep         bool
	resource     store.RuleResource
	id           string
	sourceID     string
	behavior     string
	format       string
}

func (p *preparedRuleSync) discard() {
	for _, write := range p.writes {
		write.Discard()
	}
	for _, backup := range p.backups {
		backup.Discard()
	}
	if !p.keep {
		for _, path := range p.newPaths {
			_ = os.Remove(path)
		}
	}
}

func (p *preparedRuleSync) commit() error {
	p.committed = true
	for _, write := range p.writes {
		if err := write.Commit(); err != nil {
			return errors.Join(err, p.restore())
		}
	}
	return nil
}

func (p *preparedRuleSync) restore() error {
	if !p.committed {
		return nil
	}
	p.committed = false
	var failures []error
	for path, backup := range p.backups {
		if backup != nil {
			failures = append(failures, backup.Commit())
		} else if err := os.Remove(path); !os.IsNotExist(err) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func prepareRuleSync(ctx context.Context, httpClient *http.Client, agent string, client *coreclient.Client, s *store.Store, cfg store.Settings, p Provider, resource store.RuleResource) (_ *preparedRuleSync, err error) {
	data, err := downloadRuleResource(ctx, httpClient, resource.URL, agent)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(s.Dir, "providers", "rules")
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	name := "sync-" + store.NewID()
	stage := filepath.Join(root, name)
	defer os.Remove(stage)
	defer os.Remove(stage + ".mrs")
	if err := store.AtomicWrite(stage, data); err != nil {
		return nil, err
	}
	var info struct{ Behavior, Format string }
	if err := client.Call(ctx, "compileRuleSet", map[string]string{"name": name}, &info); err != nil {
		return nil, err
	}
	if !validRuleInfo(info.Behavior, info.Format) {
		return nil, errors.New("core returned invalid rule-set metadata")
	}
	result := &preparedRuleSync{backups: map[string]*store.PreparedResource{}}
	defer func() {
		if err != nil {
			result.discard()
		}
	}()
	coreData := data
	if info.Behavior != "classical" && info.Format != "mrs" {
		coreData, err = readProviderFile(stage + ".mrs")
		if err != nil {
			return nil, err
		}
	}
	{
		id := store.NewID()
		sourceID := id
		if info.Behavior != "classical" && info.Format != "mrs" {
			sourceID = store.NewID()
		}
		result.resource, result.id, result.sourceID, result.behavior, result.format = resource, id, sourceID, info.Behavior, info.Format
		for resourceID, body := range map[string][]byte{id: coreData, sourceID: data} {
			path := s.ResourcePath(resourceID)
			result.newPaths = append(result.newPaths, path)
			if err = store.AtomicWrite(path, body); err != nil {
				return nil, err
			}
		}
		next, err := s.WithRuleResource(cfg, p.Name, resource, id, sourceID, info.Behavior, info.Format)
		if err != nil {
			return nil, err
		}
		raw, err := effectiveConfig(s, next)
		if err != nil {
			return nil, err
		}
		validation, err := os.CreateTemp(s.Dir, "rule-validation-*.yaml")
		if err != nil {
			return nil, err
		}
		path := validation.Name()
		defer os.Remove(path)
		text, err := store.YAML(raw)
		if err != nil {
			_ = validation.Close()
			return nil, err
		}
		if _, err = validation.WriteString(text); err != nil {
			_ = validation.Close()
			return nil, err
		}
		if err = validation.Close(); err != nil {
			return nil, err
		}
		var message string
		if err = client.Call(ctx, "validateMyGoConfig", path, &message); err != nil {
			return nil, err
		}
		if message != "" {
			return nil, errors.New(message)
		}
		if resource.NeedsFileDefinition || (resource.Compiled() && resource.Path == resource.SourcePath) || info.Behavior != resource.Behavior || info.Format != resource.Format {
			result.shapeChanged = true
			return result, nil
		}
	}
	for path, body := range map[string][]byte{resource.Path: coreData, resource.SourcePath: data} {
		write, err := store.PrepareResource(path, body)
		if err != nil {
			return nil, err
		}
		result.writes = append(result.writes, write)
		previous, readErr := readProviderFile(path)
		if readErr == nil {
			backup, err := store.PrepareResource(path, previous)
			if err != nil {
				return nil, err
			}
			result.backups[path] = backup
		} else if os.IsNotExist(readErr) {
			result.backups[path] = nil
		} else {
			return nil, readErr
		}
	}
	return result, nil
}

func validRuleInfo(behavior, format string) bool {
	return (behavior == "domain" || behavior == "ipcidr" || behavior == "classical") && (format == "yaml" || format == "text" || format == "mrs")
}

func ruleResourceHasEntries(data []byte) bool {
	if bytes.HasPrefix(data, []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		return true
	}
	var schema struct {
		Payload *[]string `yaml:"payload"`
		Rules   *[]string `yaml:"rules"`
	}
	if yaml.Unmarshal(data, &schema) == nil && (schema.Payload != nil || schema.Rules != nil) {
		for _, list := range []*[]string{schema.Payload, schema.Rules} {
			if list != nil {
				for _, entry := range *list {
					if strings.TrimSpace(entry) != "" {
						return true
					}
				}
			}
		}
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") {
			return true
		}
	}
	return false
}

func downloadRuleResource(ctx context.Context, client *http.Client, remote, agent string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remote, nil)
	if err != nil {
		return nil, err
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, errors.New("rule-set URL must use HTTP or HTTPS")
	}
	if agent != "" {
		req.Header.Set("User-Agent", agent)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("rule-set download: HTTP %d", res.StatusCode)
	}
	if res.ContentLength > providerSizeLimit {
		return nil, errors.New("provider exceeds 32 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, providerSizeLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > providerSizeLimit {
		return nil, errors.New("provider exceeds 32 MiB")
	}
	if !ruleResourceHasEntries(data) {
		return nil, errors.New("rule set has no usable entries")
	}
	return data, nil
}
