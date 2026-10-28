package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"go.yaml.in/yaml/v3"
)

const DefaultTestURL = "https://www.gstatic.com/generate_204"

type Profile struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	URL          string            `json:"url,omitempty"`
	Updated      time.Time         `json:"updated"`
	Interval     int               `json:"interval"`
	IntervalMS   int64             `json:"intervalMS,omitempty"`
	AutoUpdate   *bool             `json:"autoUpdate,omitempty"`
	Upload       int64             `json:"upload,omitempty"`
	Download     int64             `json:"download,omitempty"`
	Total        int64             `json:"total,omitempty"`
	Expire       int64             `json:"expire,omitempty"`
	Website      string            `json:"website,omitempty"`
	UserAgent    string            `json:"userAgent,omitempty"`
	Override     string            `json:"override,omitempty"`
	Script       string            `json:"script,omitempty"`
	ScriptID     string            `json:"scriptID,omitempty"`
	Selected     map[string]string `json:"selected,omitempty"`
	AgeSecretKey string            `json:"ageSecretKey,omitempty"`
	Type         string            `json:"type,omitempty"`
	Group        string            `json:"group,omitempty"`
	Unfold       []string          `json:"unfold,omitempty"`
	MatchTarget  string            `json:"matchTarget,omitempty"`
	Dialers      map[string]string `json:"dialers,omitempty"`
}

// UpdateInterval keeps the original millisecond duration, while accepting the
// hour-based field written by earlier native versions.
func (p Profile) UpdateInterval() time.Duration {
	if p.AutoUpdate != nil && !*p.AutoUpdate {
		return 0
	}
	return p.UpdatePeriod()
}
func (p Profile) UpdatePeriod() time.Duration {
	const limit = int64(1<<63 - 1)
	if p.IntervalMS > 0 {
		if p.IntervalMS > limit/int64(time.Millisecond) {
			return time.Duration(limit)
		}
		return time.Duration(p.IntervalMS) * time.Millisecond
	}
	if p.Interval > 0 && int64(p.Interval) > limit/int64(time.Hour) {
		return time.Duration(limit)
	}
	return time.Duration(p.Interval) * time.Hour
}
func (p Profile) UpdateDue(now time.Time) bool {
	interval := p.UpdateInterval()
	return p.URL != "" && interval > 0 && now.Sub(p.Updated) >= interval
}

type Script struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
}

type Filter struct {
	Label string `json:"label"`
	Regex string `json:"regex"`
}

type Settings struct {
	Filters               []Filter          `json:"filters"`
	Version               int               `json:"version"`
	Disclaimer            bool              `json:"disclaimer"`
	Theme                 string            `json:"theme"`
	Accent                string            `json:"accent"`
	ColorStyle            string            `json:"colorStyle"`
	FontFamily            string            `json:"fontFamily,omitempty"`
	PureBlack             bool              `json:"pureBlack"`
	LiquidGlass           string            `json:"liquidGlass"`
	Locale                string            `json:"locale"`
	Scale                 float32           `json:"scale"`
	Sidebar               bool              `json:"sidebar"`
	FloatingNavigation    bool              `json:"floatingNavigation"`
	DashboardWidgets      []string          `json:"dashboardWidgets"`
	CloseToTray           bool              `json:"closeToTray"`
	StartHidden           bool              `json:"startHidden"`
	AutoStart             bool              `json:"autoStart"`
	ExcludeSSIDs          []string          `json:"excludeSSIDs"`
	LaunchAtLogin         bool              `json:"launchAtLogin"`
	OpenLogs              bool              `json:"openLogs"`
	ProxyColumns          int               `json:"proxyColumns"`
	ProxySort             string            `json:"proxySort"`
	ProxyStyle            string            `json:"proxyStyle"`
	ProxyLayout           string            `json:"proxyLayout"`
	ProxyCard             string            `json:"proxyCard"`
	ProxyIcon             string            `json:"proxyIcon"`
	HideTimeout           bool              `json:"hideTimeout"`
	CloseConnections      bool              `json:"closeConnections"`
	TestURL               string            `json:"testURL"`
	TestTimeout           int               `json:"testTimeout"`
	OnlyProxy             bool              `json:"onlyProxy"`
	SkipCertificateVerify bool              `json:"skipCertificateVerify,omitempty"`
	HideIP                bool              `json:"hideIP"`
	SystemProxy           bool              `json:"systemProxy"`
	BypassPrivateRoute    bool              `json:"bypassPrivateRoute"`
	AppendSystemDNS       bool              `json:"appendSystemDNS"`
	Bypass                string            `json:"bypass"`
	Profiles              []Profile         `json:"profiles"`
	Scripts               []Script          `json:"scripts,omitempty"`
	LocalProxies          []map[string]any  `json:"localProxies,omitempty"`
	ServiceOrder          []string          `json:"serviceOrder,omitempty"`
	DisabledServices      []string          `json:"disabledServices,omitempty"`
	Active                string            `json:"active"`
	Selected              map[string]string `json:"selected"`
	Overrides             map[string]any    `json:"overrides"`
	Hotkeys               map[string]string `json:"hotkeys"`
	WebDAVURL             string            `json:"webdavURL"`
	WebDAVUser            string            `json:"webdavUser"`
	WebDAVPassword        string            `json:"webdavPassword"`
	WindowWidth           int               `json:"windowWidth"`
	WindowHeight          int               `json:"windowHeight"`
	Resources             []string          `json:"resources,omitempty"`
	// ImportSnapshot retains source preferences and database fields that have no
	// native equivalent yet, including IDs and relationships for later migrations.
	ImportSnapshot json.RawMessage `json:"importSnapshot,omitempty"`
}

func Defaults() Settings {
	var filters []Filter
	_ = json.Unmarshal(assets.Read("filters.json"), &filters)
	return Settings{
		Filters: filters, DashboardWidgets: DefaultDashboardWidgets(),
		Version: 1, Theme: "system", Accent: "#007aff", ColorStyle: "vivid", LiquidGlass: "none", Scale: 1,
		Sidebar: true, FloatingNavigation: true, CloseToTray: true, CloseConnections: true, SystemProxy: true, BypassPrivateRoute: true, ProxyStyle: "list", ProxyLayout: "standard", ProxyCard: "standard", ProxyIcon: "filled", ProxySort: "default",
		TestURL: DefaultTestURL, TestTimeout: 8000, WindowWidth: 1200, WindowHeight: 800,
		Bypass:   "localhost,127.*,10.*,172.16.*,192.168.*,<local>",
		Selected: map[string]string{}, Hotkeys: map[string]string{},
		Overrides: runtimeDefaults(),
	}
}

type Store struct{ Dir string }

func Open(dir string) (*Store, Settings, error) {
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, Settings{}, err
		}
		dir = filepath.Join(base, "FlClash-MyGo")
	}
	for _, sub := range []string{"", "profiles", "resources", "core"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0700); err != nil {
			return nil, Settings{}, err
		}
	}
	s := &Store{Dir: dir}
	cfg := Defaults()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err == nil {
		err = json.Unmarshal(data, &cfg)
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, cfg, fmt.Errorf("read settings: %w", err)
	}
	if cfg.Version > 1 {
		return nil, cfg, errors.New("settings were written by a newer version")
	}
	if cfg.Selected == nil {
		cfg.Selected = map[string]string{}
	}
	if cfg.Overrides == nil {
		cfg.Overrides = Defaults().Overrides
	}
	if cfg.Hotkeys == nil {
		cfg.Hotkeys = map[string]string{}
	}
	normalizeHotkeys(&cfg)
	cfg.DashboardWidgets = NormalizeDashboardWidgets(cfg.DashboardWidgets)
	if cfg.Scale < 0.8 || cfg.Scale > 1.4 {
		cfg.Scale = 1
	}
	return s, cfg, nil
}

func (s *Store) Save(cfg Settings) error {
	cfg.DashboardWidgets = NormalizeDashboardWidgets(cfg.DashboardWidgets)
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(filepath.Join(s.Dir, "settings.json"), b)
}
func AtomicWrite(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(name, path)
}
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (s *Store) ProfilePath(id string) string {
	if !ValidID(id) {
		return ""
	}
	return filepath.Join(s.Dir, "profiles", id+".yaml")
}
func ValidID(id string) bool { b, err := hex.DecodeString(id); return err == nil && len(b) == 16 }
func (s *Store) ActiveProfile(cfg Settings) *Profile {
	for i := range cfg.Profiles {
		if cfg.Profiles[i].ID == cfg.Active {
			return &cfg.Profiles[i]
		}
	}
	return nil
}
func Clone(cfg Settings) Settings {
	b, _ := json.Marshal(cfg)
	var out Settings
	_ = json.Unmarshal(b, &out)
	return out
}
func ParseYAML(b []byte) (map[string]any, error) {
	var m map[string]any
	var document yaml.Node
	if err := yaml.Unmarshal(b, &document); err != nil {
		return nil, err
	}
	retagShortIDs(&document)
	if err := document.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("profile must be a YAML mapping")
	}
	return m, nil
}
func retagShortIDs(node *yaml.Node) {
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "short-id" {
				value := node.Content[i+1]
				values := []*yaml.Node{value}
				if value.Kind == yaml.SequenceNode {
					values = value.Content
				}
				for _, v := range values {
					if v.Kind == yaml.ScalarNode && (v.Tag == "!!int" || v.Tag == "!!float") {
						v.Tag = "!!str"
					}
				}
			}
		}
	}
	for _, child := range node.Content {
		retagShortIDs(child)
	}
}
func PatchSection(b []byte, key string, value any) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("profile must be a YAML mapping")
	}
	root := doc.Content[0]
	var replacement yaml.Node
	if err := replacement.Encode(value); err != nil {
		return nil, err
	}
	found := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			old := root.Content[i+1]
			replacement.HeadComment, replacement.LineComment, replacement.FootComment = old.HeadComment, old.LineComment, old.FootComment
			root.Content[i+1] = &replacement
			found = true
			break
		}
	}
	if !found {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &replacement)
	}
	return yaml.Marshal(&doc)
}
func YAML(v any) (string, error) { b, e := yaml.Marshal(v); return string(b), e }
func Merge(dst, src map[string]any) {
	for k, v := range src {
		sm, ok := v.(map[string]any)
		dm, exists := dst[k].(map[string]any)
		if ok && exists {
			Merge(dm, sm)
		} else {
			dst[k] = v
		}
	}
}
func (s *Store) Effective(cfg Settings) (map[string]any, error) {
	return s.EffectiveTransform(cfg, nil)
}

// EffectiveTransform runs a profile script against the source config before
// applying application settings, following FlClash's setup ordering.
func (s *Store) EffectiveTransform(cfg Settings, transform func(map[string]any) (map[string]any, error)) (map[string]any, error) {
	raw := map[string]any{"proxies": []any{}, "proxy-groups": []any{}, "rules": []any{"MATCH,DIRECT"}}
	var profileOverride map[string]any
	if p := s.ActiveProfile(cfg); p != nil {
		b, err := os.ReadFile(s.ProfilePath(p.ID))
		if err != nil {
			return nil, err
		}
		raw, err = ParseYAML(b)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.Override) != "" {
			m, e := ParseYAML([]byte(p.Override))
			if e != nil {
				return nil, fmt.Errorf("profile override: %w", e)
			}
			profileOverride = m
		}
	}
	if transform != nil {
		var err error
		raw, err = transform(raw)
		if err != nil {
			return nil, err
		}
		if raw == nil {
			return nil, errors.New("script did not return a configuration object")
		}
	}
	if err := s.confineProviders(raw, cfg.Active); err != nil {
		return nil, err
	}
	baselineDNS(raw)
	appOverride := Clone(cfg).Overrides
	delete(appOverride, "interface-name")
	delete(appOverride, "interface-name-mode")
	if p := s.ActiveProfile(cfg); p == nil || p.Type != "custom" {
		target := ""
		if p != nil {
			target = strings.TrimSpace(p.MatchTarget)
		}
		if target == "" {
			target = lastMatchTarget(raw["rules"])
		}
		resolveAddedRuleTargets(appOverride, target)
		resolveAddedRuleTargets(profileOverride, target)
	}
	MergeConfig(raw, appOverride)
	MergeConfig(raw, profileOverride)
	runtimeOwned(raw, cfg, profileOverride)
	if p := s.ActiveProfile(cfg); p != nil && p.Type == "custom" && len(cfg.LocalProxies) > 0 {
		groups, _ := raw["proxy-groups"].([]any)
		named := map[string]bool{}
		for _, value := range groups {
			group, _ := value.(map[string]any)
			for _, name := range flutterJSONList(group["proxies"]) {
				named[String(name)] = true
			}
		}
		proxies := []any{}
		for _, definition := range Clone(cfg).LocalProxies {
			name := String(definition["name"])
			if !named[name] {
				continue
			}
			delete(definition, "dialer-proxy")
			if target := p.Dialers[name]; target != "" {
				definition["dialer-proxy"] = target
			}
			proxies = append(proxies, definition)
		}
		dropDialerLoops(proxies, groups)
		MergeConfig(raw, map[string]any{"proxies": proxies})
	}
	if err := s.resolveReferences(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
func MergeConfig(dst, src map[string]any) {
	for key, value := range src {
		switch key {
		case "sniffer":
			if values, ok := value.(map[string]any); ok {
				mergeSnifferOverride(Map(dst, key), values)
			} else {
				dst[key] = value
			}
		case "dns":
			values, ok := value.(map[string]any)
			if !ok {
				dst[key] = value
				continue
			}
			dns := Map(dst, key)
			for field, entry := range values {
				if servers, ok := entry.([]any); field == "nameserver" && ok && len(servers) == 0 {
					continue
				}
				if nested, ok := entry.(map[string]any); field == "fallback-filter" && ok {
					Merge(Map(dns, field), nested)
				} else if policy, ok := entry.(map[string]any); ok && (field == "nameserver-policy" || field == "proxy-server-nameserver-policy") {
					normalized := map[string]any{}
					for pattern, servers := range policy {
						normalized[pattern] = splitConfigServers(servers)
					}
					dns[field] = normalized
				} else {
					dns[field] = entry
				}
			}
		case "rules":
			added, ok := value.([]any)
			if !ok {
				dst[key] = value
				continue
			}
			existing, _ := dst[key].([]any)
			dst[key] = append(append([]any{}, added...), existing...)
		case "proxies", "proxy-groups":
			added, ok := value.([]any)
			if !ok {
				dst[key] = value
				continue
			}
			existing, _ := dst[key].([]any)
			out := append([]any{}, existing...)
			for _, entry := range added {
				m, _ := entry.(map[string]any)
				name := String(m["name"])
				replaced := false
				for i, old := range out {
					om, _ := old.(map[string]any)
					if name != "" && String(om["name"]) == name {
						out[i] = entry
						replaced = true
						break
					}
				}
				if !replaced {
					out = append(out, entry)
				}
			}
			dst[key] = out
		default:
			sm, ok := value.(map[string]any)
			dm, exists := dst[key].(map[string]any)
			if ok && exists {
				Merge(dm, sm)
			} else {
				dst[key] = value
			}
		}
	}
}
func Int(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}
func Bool(v any) bool { b, _ := v.(bool); return b }
func String(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func Map(m map[string]any, key string) map[string]any {
	v, ok := m[key].(map[string]any)
	if !ok {
		v = map[string]any{}
		m[key] = v
	}
	return v
}

func Download(client *http.Client, rawURL, agent string) ([]byte, http.Header, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, nil, errors.New("enter an HTTP or HTTPS subscription URL")
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	if agent == "" {
		agent = "FlClash/MyGo"
	}
	req.Header.Set("User-Agent", agent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("subscription returned HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20+1))
	if err != nil {
		return nil, nil, err
	}
	if len(b) > 32<<20 {
		return nil, nil, errors.New("subscription exceeds 32 MiB")
	}
	return b, resp.Header, nil
}
func (p *Profile) Subscription(h http.Header) {
	for _, field := range strings.Split(h.Get("Subscription-Userinfo"), ";") {
		var key string
		var value int64
		parts := strings.SplitN(strings.TrimSpace(field), "=", 2)
		if len(parts) != 2 {
			continue
		}
		key = parts[0]
		_, _ = fmt.Sscan(parts[1], &value)
		switch key {
		case "upload":
			p.Upload = value
		case "download":
			p.Download = value
		case "total":
			p.Total = value
		case "expire":
			p.Expire = value
		}
	}
	p.Website = h.Get("Profile-Web-Page-Url")
}
