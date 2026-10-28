package store

// FlClash's archive format is versioned independently of this application's
// settings. Read Drift snapshots with pure-Go SQLite and no database migrations.
import (
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

type flutterTables map[string][]map[string]any

func (s *Store) importFlutter(files map[string][]byte) (Settings, map[string][]byte, error) {
	cfg := Defaults()
	var source map[string]any
	dec := json.NewDecoder(bytes.NewReader(files["config.json"]))
	dec.UseNumber()
	if err := dec.Decode(&source); err != nil || source == nil {
		return cfg, nil, errors.New("invalid FlClash config.json")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return cfg, nil, errors.New("invalid trailing FlClash config data")
	}
	if v, exists := source["version"]; exists {
		if n, ok := v.(json.Number); !ok {
			return cfg, nil, errors.New("invalid FlClash version")
		} else if _, err := n.Int64(); err != nil {
			return cfg, nil, errors.New("invalid FlClash version")
		}
	}
	version := Int(source["version"])
	if version > 1 || version < 0 {
		return cfg, nil, errors.New("unsupported FlClash backup version")
	}
	tables := flutterTables{}
	var err error
	if version == 0 {
		tables = legacyFlutter(source, files)
	} else if data, ok := files["database.sqlite"]; ok {
		tables, err = s.readFlutterSQLite(data)
		if err != nil {
			return cfg, nil, err
		}
	}
	mapFlutterSettings(&cfg, source)
	snapshot := flutterSnapshot{Config: source, Tables: tables, Profiles: map[string]string{}, Scripts: map[string]string{}, Resources: map[string]string{}, Overlays: map[string]string{}}
	out := map[string][]byte{}
	profileIDs, scriptIDs := map[string]string{}, map[string]string{}
	proxyNames := map[string]string{}
	for _, proxy := range orderedRows(tables["custom_proxies"]) {
		definition := flutterJSONMap(proxy["definition"])
		cfg.LocalProxies = append(cfg.LocalProxies, definition)
		proxyNames[String(proxy["id"])] = String(definition["name"])
	}
	for _, row := range orderedRows(tables["profiles"]) {
		old := String(row["id"])
		if old == "" || profileIDs[old] != "" {
			return cfg, nil, errors.New("invalid FlClash profile identity")
		}
		profileIDs[old] = NewID()
	}
	for _, row := range orderedRows(tables["scripts"]) {
		old := String(row["id"])
		if old == "" || scriptIDs[old] != "" {
			return cfg, nil, errors.New("invalid FlClash script identity")
		}
		id := NewID()
		scriptIDs[old] = id
		data, ok := files["scripts/"+old+".js"]
		if !ok {
			return cfg, nil, fmt.Errorf("script %s is missing", String(row["label"]))
		}
		if !utf8.Valid(data) {
			return cfg, nil, errors.New("FlClash script is not UTF-8")
		}
		cfg.Scripts = append(cfg.Scripts, Script{ID: id, Name: String(row["label"]), Source: string(data)})
		snapshot.Scripts[id] = old
	}
	providerPaths := map[string]string{}
	for _, row := range tables["clash_providers"] {
		if String(row["kind"]) != "rule" {
			continue
		}
		if String(row["url"]) != "" {
			continue
		}
		digest := md5.Sum([]byte(String(row["id"]) + "@"))
		data, ok := files["providers/rules/"+hex.EncodeToString(digest[:])]
		if !ok {
			return cfg, nil, fmt.Errorf("local rule provider %s is missing", String(row["label"]))
		}
		id := NewID()
		cfg.Resources = append(cfg.Resources, id)
		snapshot.Resources[id] = String(row["id"])
		out["resources/"+id] = data
		providerPaths[String(row["label"])] = resourceRef + id
	}
	paths := map[string]string{}
	for old, id := range profileIDs {
		paths["profiles/"+old+".yaml"] = profileRef + id
	}
	for id, old := range snapshot.Resources {
		paths[flutterProviderEntry(old)] = resourceRef + id
	}
	for _, row := range orderedRows(tables["profiles"]) {
		old := String(row["id"])
		p := Profile{ID: profileIDs[old], Name: String(row["label"]), URL: String(row["url"]), Type: String(row["type"]), Group: String(row["current_group_name"]), MatchTarget: String(row["match_target"]), AgeSecretKey: String(row["age_secret_key"]), Selected: map[string]string{}}
		if p.Name == "" {
			p.Name = old
		}
		p.IntervalMS = int64(Int(row["auto_update_duration_millis"]))
		p.Interval = int(p.IntervalMS / int64(time.Hour/time.Millisecond))
		auto := true
		if value, exists := row["auto_update"]; exists && value != nil {
			auto = flutterBool(value)
		}
		p.AutoUpdate = &auto
		if t := Int(row["last_update_date"]); t != 0 {
			p.Updated = time.Unix(int64(t), 0)
		}
		if selected := flutterJSONMap(row["selected_map"]); selected != nil {
			for k, v := range selected {
				p.Selected[k] = String(v)
			}
		}
		for _, v := range flutterJSONList(row["unfold_set"]) {
			p.Unfold = append(p.Unfold, String(v))
		}
		info := flutterJSONMap(row["subscription_info"])
		p.Upload, p.Download, p.Total, p.Expire = int64(Int(info["upload"])), int64(Int(info["download"])), int64(Int(info["total"])), int64(Int(info["expire"]))
		data, ok := files["profiles/"+old+".yaml"]
		if !ok && p.Type != "custom" {
			return cfg, nil, fmt.Errorf("profile %s is missing", p.Name)
		}
		if p.Type == "custom" {
			p.Dialers = map[string]string{}
			for _, dialer := range tables["proxy_dialers"] {
				if String(dialer["profile_id"]) == old {
					if name := proxyNames[String(dialer["proxy_id"])]; name != "" {
						p.Dialers[name] = String(dialer["target"])
					}
				}
			}
			data = []byte("proxies: []\nproxy-groups: []\nrules: []\n")
		}
		_, e := ParseYAML(data)
		if e != nil {
			return cfg, nil, fmt.Errorf("profile %s: %w", p.Name, e)
		}
		overlay := map[string]any{}
		if p.Type == "custom" {
			overlay = selectedFlutterOverrides(flutterJSONMap(row["overrides"]), []string{"dns", "ntp", "sniffer", "tun"})
			groups := []any{}
			named := map[string]bool{}
			uses := map[string]bool{}
			for _, group := range orderedRows(tables["proxy_groups"]) {
				if String(group["profile_id"]) != old {
					continue
				}
				g := flutterJSONMap(group["definition"])
				if g == nil {
					g = map[string]any{}
				}
				g["name"], g["type"] = group["name"], group["type"]
				for _, key := range []string{"proxies", "use"} {
					if group[key] != nil {
						g[key] = flutterJSONList(group[key])
					}
				}
				for _, name := range flutterJSONList(group["proxies"]) {
					named[String(name)] = true
				}
				for _, name := range flutterJSONList(group["use"]) {
					uses[String(name)] = true
				}
				groups = append(groups, g)
			}
			overlay["proxy-groups"] = groups
			proxies := []any{}
			for _, proxy := range orderedRows(tables["custom_proxies"]) {
				definition := flutterJSONMap(proxy["definition"])
				if !named[String(definition["name"])] {
					continue
				}
				delete(definition, "dialer-proxy")
				for _, dialer := range tables["proxy_dialers"] {
					if String(dialer["profile_id"]) == old && String(dialer["proxy_id"]) == String(proxy["id"]) {
						definition["dialer-proxy"] = dialer["target"]
					}
				}
				proxies = append(proxies, definition)
			}
			dropDialerLoops(proxies, groups)
			overlay["proxies"] = proxies
			proxyProviders := map[string]any{}
			options := flutterJSONMap(row["overrides"])
			for _, other := range tables["profiles"] {
				if String(other["type"]) == "custom" {
					continue
				}
				name := String(other["label"])
				if name == "" {
					name = String(other["id"])
				}
				if !uses[name] {
					continue
				}
				definition := map[string]any{"type": "file", "path": profileRef + profileIDs[String(other["id"])]}
				if opts := flutterJSONMap(options["proxy-providers"]); opts != nil {
					Merge(definition, flutterJSONMap(opts[name]))
				}
				proxyProviders[name] = definition
			}
			if len(proxyProviders) > 0 {
				overlay["proxy-providers"] = proxyProviders
			}
		} else if String(row["extend_type"]) == "script" {
			if row["script_id"] != nil {
				p.ScriptID = scriptIDs[String(row["script_id"])]
				if p.ScriptID == "" {
					return cfg, nil, fmt.Errorf("script for profile %s is missing", p.Name)
				}
			}
		}
		if p.Type == "custom" || String(row["extend_type"]) != "script" {
			rules := []any{}
			disabled := map[string]bool{}
			for _, d := range tables["disabled_rules"] {
				if String(d["profile_id"]) == old {
					disabled[String(d["rule_id"])] = true
				}
			}
			for _, global := range []bool{false, true} {
				for _, rule := range orderedRows(tables["rules"]) {
					isGlobal := rule["profile_id"] == nil
					if isGlobal != global || isGlobal && p.Type == "custom" || !isGlobal && String(rule["profile_id"]) != old || disabled[String(rule["id"])] {
						continue
					}
					rules = append(rules, flutterRule(rule))
				}
			}
			if len(rules) > 0 {
				overlay["rules"] = rules
			}
		}
		if p.Type == "custom" {
			providers := map[string]any{}
			serialized, _ := json.Marshal(overlay)
			for _, provider := range tables["clash_providers"] {
				name := String(provider["label"])
				if String(provider["kind"]) != "rule" || !strings.Contains(string(serialized), "RULE-SET,"+name+",") {
					continue
				}
				behavior, format := String(provider["behavior"]), String(provider["format"])
				if behavior == "" {
					behavior = "classical"
				}
				if format == "" {
					format = "yaml"
				}
				def := map[string]any{"type": "file", "path": providerPaths[name], "behavior": behavior, "format": format}
				if url := String(provider["url"]); url != "" {
					def["type"], def["url"], def["interval"] = "http", url, 86400
					delete(def, "path")
				}
				providers[name] = def
			}
			if len(providers) > 0 {
				overlay["rule-providers"] = providers
			}
		}
		if len(overlay) > 0 {
			p.Override, err = YAML(overlay)
			if err != nil {
				return cfg, nil, err
			}
		}
		data, err = rewriteFlutterYAMLPaths(data, func(path string) (string, error) {
			if mapped := paths[path]; mapped != "" {
				return mapped, nil
			}
			return path, nil
		})
		if err != nil {
			return cfg, nil, err
		}
		cfg.Profiles = append(cfg.Profiles, p)
		snapshot.Profiles[p.ID], snapshot.Overlays[p.ID] = old, p.Override
		out["profiles/"+p.ID+".yaml"] = data
	}
	cfg.Active = profileIDs[String(source["currentProfileId"])]
	if source["currentProfileId"] != nil && cfg.Active == "" {
		return cfg, nil, errors.New("active FlClash profile is missing")
	}
	if p := s.ActiveProfile(cfg); p != nil {
		cfg.Selected = p.Selected
	}
	cfg.ImportSnapshot, err = json.Marshal(snapshot)
	if len(cfg.ImportSnapshot) > 32<<20 {
		return cfg, nil, errors.New("FlClash import snapshot exceeds size limit")
	}
	return cfg, out, err
}

func (s *Store) readFlutterSQLite(data []byte) (tables flutterTables, err error) {
	defer func() {
		if recover() != nil {
			tables = nil
			err = errors.New("invalid FlClash SQLite snapshot")
		}
	}()
	if len(data) < 100 || string(data[:16]) != "SQLite format 3\x00" {
		return nil, errors.New("invalid SQLite header")
	}
	if binary.BigEndian.Uint32(data[60:64]) > 21 {
		return nil, errors.New("FlClash database was written by a newer version")
	}
	dir, err := os.MkdirTemp(s.Dir, ".flutter-import-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "database.sqlite")
	if err = AtomicWrite(path, data); err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := uri.Query()
	query.Set("mode", "ro")
	query.Set("immutable", "1")
	query.Add("_pragma", "query_only(1)")
	query.Add("_pragma", "trusted_schema(0)")
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	existing := map[string]bool{}
	catalog, err := db.QueryContext(ctx, "SELECT name FROM sqlite_schema WHERE type = 'table'")
	if err != nil {
		return nil, err
	}
	for catalog.Next() {
		var name string
		if err = catalog.Scan(&name); err != nil {
			catalog.Close()
			return nil, err
		}
		existing[name] = true
	}
	err = catalog.Err()
	catalog.Close()
	if err != nil {
		return nil, err
	}
	tables = flutterTables{}
	for _, name := range flutterTableNames {
		if !existing[name] {
			continue
		}
		rows, e := db.QueryContext(ctx, `SELECT * FROM "`+name+`"`)
		if e != nil {
			return nil, e
		}
		columns, e := rows.Columns()
		if e != nil {
			rows.Close()
			return nil, e
		}
		if len(columns) > 128 {
			rows.Close()
			return nil, errors.New("too many FlClash database columns")
		}
		for rows.Next() {
			if len(tables[name]) >= 100000 {
				rows.Close()
				return nil, errors.New("too many FlClash database rows")
			}
			values := make([]any, len(columns))
			dest := make([]any, len(columns))
			for i := range values {
				dest[i] = &values[i]
			}
			if e = rows.Scan(dest...); e != nil {
				rows.Close()
				return nil, e
			}
			row := map[string]any{}
			for i, key := range columns {
				value := values[i]
				if data, ok := value.([]byte); ok {
					value = string(data)
				}
				row[key] = value
			}
			tables[name] = append(tables[name], row)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}

	return tables, nil
}

func flutterJSONMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	if text, ok := v.(string); ok {
		dec := json.NewDecoder(strings.NewReader(text))
		dec.UseNumber()
		_ = dec.Decode(&m)
	}
	return m
}
func flutterJSONList(v any) []any {
	if list, ok := v.([]any); ok {
		return list
	}
	var list []any
	if text, ok := v.(string); ok {
		dec := json.NewDecoder(strings.NewReader(text))
		dec.UseNumber()
		_ = dec.Decode(&list)
	}
	return list
}
func flutterBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return Int(v) != 0
}
func orderedRows(rows []map[string]any) []map[string]any {
	out := append([]map[string]any(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i]["order"], out[j]["order"]
		if a == nil || b == nil {
			if (a == nil) != (b == nil) {
				return a != nil
			}
			return Int(out[i]["id"]) < Int(out[j]["id"])
		}
		if _, ok := a.(string); ok {
			return String(a) < String(b)
		}
		return Int(a) < Int(b)
	})
	return out
}
func flutterRule(row map[string]any) string {
	if raw := String(row["raw"]); raw != "" {
		return raw
	}
	action := strings.ReplaceAll(String(row["rule_action"]), "_", "-")
	content, target := String(row["content"]), String(row["rule_target"])
	if action == "RULE-SET" {
		content = String(row["rule_provider"])
	}
	if action == "SUB-RULE" {
		target = String(row["sub_rule"])
	}
	parts := []string{action}
	if action != "MATCH" && content != "" {
		parts = append(parts, content)
	}
	if target != "" {
		parts = append(parts, target)
	}
	if action != "MATCH" && action != "AND" && action != "OR" && action != "NOT" && action != "SUB-RULE" {
		if flutterBool(row["src"]) {
			parts = append(parts, "src")
		}
		if flutterBool(row["no_resolve"]) {
			parts = append(parts, "no-resolve")
		}
	}
	return strings.Join(parts, ",")
}
func selectedFlutterOverrides(source map[string]any, sections []string, keepEmpty ...bool) map[string]any {
	out := map[string]any{}
	for _, section := range sections {
		values := flutterJSONMap(source[section])
		selected := map[string]any{}
		for _, key := range flutterJSONList(source[section+"-override-keys"]) {
			parts := strings.Split(String(key), ".")
			var value any = values
			for _, part := range parts {
				m, ok := value.(map[string]any)
				if !ok {
					value = nil
					break
				}
				value = m[part]
			}
			if value == nil {
				continue
			}
			empty := false
			switch v := value.(type) {
			case string:
				empty = v == ""
			case []any:
				empty = len(v) == 0
			case map[string]any:
				empty = len(v) == 0
			}
			if empty && (len(keepEmpty) == 0 || !keepEmpty[0] || section != "dns" || String(key) == "nameserver") {
				continue
			}
			dst := selected
			for _, part := range parts[:len(parts)-1] {
				dst = Map(dst, part)
			}
			dst[parts[len(parts)-1]] = value
		}
		if len(selected) > 0 {
			out[section] = selected
		}
	}
	return out
}

func mapFlutterSettings(cfg *Settings, source map[string]any) {
	importFlutterOnDemand(cfg, source)
	for _, item := range flutterJSONList(source["hotKeyActions"]) {
		row := flutterJSONMap(item)
		if action, chord := String(row["action"]), flutterHotkey(row); action != "" && chord != "" {
			cfg.Hotkeys[action] = chord
		}
	}
	app := flutterJSONMap(source["appSettingProps"])
	if raw, exists := app["dashboardWidgets"]; exists {
		var names []string
		if values := flutterJSONList(raw); values != nil {
			names = []string{}
			for _, value := range values {
				names = append(names, String(value))
			}
		}
		cfg.DashboardWidgets = NormalizeDashboardWidgets(names)
	}
	for key, target := range map[string]*[]string{"serviceOrder": &cfg.ServiceOrder, "disabledServices": &cfg.DisabledServices} {
		for _, value := range flutterJSONList(app[key]) {
			*target = append(*target, String(value))
		}
	}
	for key, dst := range map[string]*bool{"onlyStatisticsProxy": &cfg.OnlyProxy, "autoLaunch": &cfg.LaunchAtLogin, "silentLaunch": &cfg.StartHidden, "autoRun": &cfg.AutoStart, "openLogs": &cfg.OpenLogs, "sidebarExpanded": &cfg.Sidebar, "floatingNavigationBar": &cfg.FloatingNavigation, "minimizeOnExit": &cfg.CloseToTray} {
		if v, ok := app[key].(bool); ok {
			*dst = v
		}
	}
	cfg.Disclaimer = Int(app["acceptedDisclaimerVersion"]) > 0
	cfg.HideIP = flutterBool(app["hideIp"])
	if check, ok := app["checkCertificate"].(bool); ok {
		cfg.SkipCertificateVerify = !check
	}
	cfg.Locale = String(app["locale"])
	if timeout := Int(app["delayTestTimeout"]); timeout > 0 {
		cfg.TestTimeout = timeout
	}
	if raw, ok := app["filters"]; ok {
		cfg.Filters = nil
		for _, v := range flutterJSONList(raw) {
			f := flutterJSONMap(v)
			cfg.Filters = append(cfg.Filters, Filter{Label: String(f["label"]), Regex: String(f["regex"])})
		}
	}
	theme := flutterJSONMap(source["themeProps"])
	switch value := theme["liquidGlass"].(type) {
	case bool:
		cfg.LiquidGlass = "none"
		if value {
			cfg.LiquidGlass = "standard"
		}
	case string:
		cfg.LiquidGlass = value
	}
	cfg.FontFamily = String(theme["systemFontFamily"])
	if mode := String(theme["themeMode"]); mode != "" {
		cfg.Theme = mode
	}
	if style := String(theme["colorStyle"]); style != "" {
		cfg.ColorStyle = style
	}
	cfg.PureBlack = flutterBool(theme["pureBlack"])
	if color := Int(theme["primaryColor"]); color != 0 {
		cfg.Accent = fmt.Sprintf("#%06x", uint32(color)&0xffffff)
	}
	if scale := flutterJSONMap(theme["textScale"]); flutterBool(scale["enable"]) {
		f, _ := strconv.ParseFloat(String(scale["scale"]), 32)
		cfg.Scale = float32(f)
	}
	window := flutterJSONMap(source["windowGeometry"])
	if n := Int(window["width"]); n > 0 {
		cfg.WindowWidth = n
	}
	if n := Int(window["height"]); n > 0 {
		cfg.WindowHeight = n
	}
	style := flutterJSONMap(source["proxiesStyleProps"])
	for key, dst := range map[string]*string{"type": &cfg.ProxyStyle, "layout": &cfg.ProxyLayout, "cardType": &cfg.ProxyCard, "iconStyle": &cfg.ProxyIcon} {
		if v := String(style[key]); v != "" {
			*dst = v
		}
	}
	cfg.HideTimeout = flutterBool(style["hideTimeoutProxies"])
	if v, ok := app["closeConnections"].(bool); ok {
		cfg.CloseConnections = v
	}
	switch String(style["sortType"]) {
	case "delay", "name":
		cfg.ProxySort = String(style["sortType"])
	}
	network := flutterJSONMap(source["networkProps"])
	if value, ok := network["bypassPrivateRoute"].(bool); ok {
		cfg.BypassPrivateRoute = value
	} else if legacy := String(network["routeMode"]); legacy != "" {
		cfg.BypassPrivateRoute = legacy == "bypassPrivate"
	}
	if value, ok := network["appendSystemDns"].(bool); ok {
		cfg.AppendSystemDNS = value
	}
	if v, ok := network["systemProxy"].(bool); ok {
		cfg.SystemProxy = v
	}
	if bypass, ok := network["bypassDomain"]; ok {
		list := []string{}
		for _, v := range flutterJSONList(bypass) {
			list = append(list, String(v))
		}
		cfg.Bypass = strings.Join(list, ",")
	}
	dav := flutterJSONMap(source["davProps"])
	cfg.WebDAVURL, cfg.WebDAVUser, cfg.WebDAVPassword = String(dav["uri"]), String(dav["user"]), decodeFlutterDAVPassword(String(dav["password"]))
	patch := flutterJSONMap(source["patchClashConfig"])
	for _, key := range []string{"ipv6", "unified-delay", "tcp-concurrent", "geodata-loader"} {
		delete(cfg.Overrides, key)
	}
	for _, key := range []string{"mixed-port", "port", "socks-port", "redir-port", "tproxy-port", "mode", "allow-lan", "log-level", "find-process-mode", "keep-alive-interval", "keep-alive-idle", "disable-keep-alive", "routing-mark", "geo-auto-update", "geo-update-interval", "geox-url", "global-ua", "hosts", "secret", "external-controller", "interface-name", "interface-name-mode"} {
		if v, ok := patch[key]; ok && v != nil {
			cfg.Overrides[key] = v
		}
	}
	generalKeys := flutterJSONList(patch["general-override-keys"])
	generalDefaults := map[string]any{"unified-delay": true, "tcp-concurrent": true, "ipv6": false, "geodata-loader": "memconservative"}
	if _, explicit := patch["general-override-keys"]; !explicit && source["patchClashConfig"] != nil {
		generalKeys = []any{"unified-delay", "tcp-concurrent", "ipv6", "geodata-loader"}
	}
	for _, key := range generalKeys {
		if String(key) == "test-url" {
			continue
		}
		v, present := patch[String(key)]
		if !present {
			v = generalDefaults[String(key)]
		}
		if v != nil {
			cfg.Overrides[String(key)] = v
		}
	}
	if url := String(patch["test-url"]); url != "" {
		cfg.TestURL = url
	}
	if tun := flutterJSONMap(patch["tun"]); len(tun) > 0 {
		for _, key := range []string{"enable", "device", "stack"} {
			if value, present := tun[key]; present && value != nil {
				Map(cfg.Overrides, "tun")[key] = value
			}
		}
	}
	Merge(cfg.Overrides, selectedFlutterOverrides(patch, []string{"dns"}, true))
	if overrides := selectedFlutterOverrides(map[string]any{"tun": patch["tun-override"], "tun-override-keys": patch["tun-override-keys"]}, []string{"tun"}); len(overrides) > 0 {
		Merge(cfg.Overrides, overrides)
	}
	if auth := flutterJSONMap(network["authentication"]); flutterBool(auth["enable"]) && String(auth["username"]) != "" {
		cfg.Overrides["authentication"] = []any{String(auth["username"]) + ":" + String(auth["password"])}
	}
}
func decodeFlutterDAVPassword(value string) string {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != "v1" {
		return value
	}
	decode := func(v string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(strings.TrimRight(v, "=")) }
	nonce, e := decode(parts[1])
	if e != nil || len(nonce) != 16 {
		return ""
	}
	data, e := decode(parts[2])
	if e != nil {
		return ""
	}
	mask := []byte{0x9d, 0x42, 0xe7, 0x1b, 0x68, 0xb4, 0x35, 0xca, 0x7f, 0x20, 0xd1, 0x56, 0x83, 0xfa, 0x0c, 0xa9}
	for i := range data {
		data[i] ^= nonce[i%len(nonce)] ^ mask[i%len(mask)]
	}
	if !utf8.Valid(data) {
		return ""
	}
	return string(data)
}

// Drop precisely the dialers that can lead back to their own proxy through a
// group or another dialer, as upstream does when assembling custom profiles.
func dropDialerLoops(proxies, groups []any) {
	edges := map[string][]string{}
	for _, v := range groups {
		g := v.(map[string]any)
		for _, n := range flutterJSONList(g["proxies"]) {
			edges[String(g["name"])] = append(edges[String(g["name"])], String(n))
		}
	}
	for _, v := range proxies {
		p := v.(map[string]any)
		if target := String(p["dialer-proxy"]); target != "" {
			edges[String(p["name"])] = append(edges[String(p["name"])], target)
		}
	}
	for _, v := range proxies {
		p := v.(map[string]any)
		name, target := String(p["name"]), String(p["dialer-proxy"])
		if target == "" {
			continue
		}
		visited := map[string]bool{}
		var reaches func(string) bool
		reaches = func(next string) bool {
			if next == name {
				return true
			}
			if visited[next] {
				return false
			}
			visited[next] = true
			for _, n := range edges[next] {
				if reaches(n) {
					return true
				}
			}
			return false
		}
		if reaches(target) {
			delete(p, "dialer-proxy")
		}
	}
}
