package store

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Mirror Rule.parse in the pinned models/clash_config.dart, including payloads
// containing commas. Reject unknown actions instead of Dart's DOMAIN fallback.
func exportFlutterRule(value string) (map[string]any, error) {
	fields := strings.Split(value, ",")
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	action := strings.ToUpper(fields[0])
	allowed := ",DOMAIN,DOMAIN-SUFFIX,DOMAIN-KEYWORD,DOMAIN-REGEX,DOMAIN-WILDCARD,GEOSITE,IP-CIDR,IP-CIDR6,IP-SUFFIX,IP-ASN,GEOIP,SRC-GEOIP,SRC-IP-ASN,SRC-IP-CIDR,SRC-IP-SUFFIX,DST-PORT,SRC-PORT,IN-PORT,IN-TYPE,IN-USER,IN-NAME,REMATCH-NAME,PROCESS-PATH,PROCESS-PATH-REGEX,PROCESS-PATH-WILDCARD,PROCESS-NAME,PROCESS-NAME-REGEX,PROCESS-NAME-WILDCARD,UID,NETWORK,DSCP,RULE-SET,AND,OR,NOT,SUB-RULE,MATCH,"
	if action == "" || !strings.Contains(allowed, ","+action+",") {
		return nil, fmt.Errorf("Flutter cannot represent rule %q", value)
	}
	row := map[string]any{"rule_action": strings.ReplaceAll(action, "-", "_"), "no_resolve": false, "src": false}
	rest := fields[1:]
	if len(rest) == 0 {
		return nil, fmt.Errorf("invalid added rule %q", value)
	}
	if action == "MATCH" {
		if len(rest) != 1 || rest[0] == "" {
			return nil, fmt.Errorf("invalid added rule %q", value)
		}
		row["rule_target"] = rest[0]
		return row, nil
	}
	if len(rest) < 2 {
		return nil, fmt.Errorf("invalid added rule %q", value)
	}
	payload, target := rest[0], rest[1]
	comma := action == "AND" || action == "OR" || action == "NOT" || action == "SUB-RULE" || action == "DOMAIN-REGEX" || action == "PROCESS-NAME-REGEX" || action == "PROCESS-PATH-REGEX"
	if comma {
		payload, target = strings.Join(rest[:len(rest)-1], ","), rest[len(rest)-1]
	} else {
		for _, param := range rest[2:] {
			switch param {
			case "src":
				row["src"] = true
			case "no-resolve":
				row["no_resolve"] = true
			default:
				return nil, fmt.Errorf("Flutter cannot represent rule parameter %q", param)
			}
		}
	}
	if payload == "" || target == "" {
		return nil, fmt.Errorf("invalid added rule %q", value)
	}
	if action == "RULE-SET" {
		row["rule_provider"] = payload
	} else {
		row["content"] = payload
	}
	if action == "SUB-RULE" {
		row["sub_rule"] = target
	} else {
		row["rule_target"] = target
	}
	return row, nil
}

func exportFlutterCustom(source, row map[string]any, tables *flutterTables, proxyIDs map[string]int64, proxyDefinitions map[string]string, newID func() int64, rewrite func(string) (string, error)) error {
	fid := row["id"]
	overrides := flutterJSONMap(row["overrides"])
	for _, section := range []string{"dns", "ntp", "sniffer", "tun"} {
		overrides[section+"-override-keys"] = []string{}
	}
	overrides["proxy-providers"] = map[string]any{}
	for key, value := range source {
		if value != nil {
			switch key {
			case "rules", "proxies", "proxy-groups":
				if _, ok := value.([]any); !ok {
					return fmt.Errorf("custom %s must be a YAML list", key)
				}
			case "dns", "ntp", "sniffer", "tun", "proxy-providers", "rule-providers":
				if _, ok := value.(map[string]any); !ok {
					return fmt.Errorf("custom %s must be a YAML mapping", key)
				}
			}
		}
		switch key {
		case "rules":
			if err := validateFlutterAddedRules(value); err != nil {
				return err
			}
			if len((*tables)["rules"])+len(flutterJSONList(value)) > 100000 {
				return errors.New("too many exported rules")
			}
			start := 0
			for _, rule := range (*tables)["rules"] {
				if String(rule["profile_id"]) == String(fid) {
					start++
				}
			}
			for i, value := range flutterJSONList(value) {
				rule, e := exportFlutterRule(String(value))
				if e != nil {
					return e
				}
				rule["id"], rule["profile_id"], rule["order"] = newID(), fid, flutterOrder(start+i)
				(*tables)["rules"] = append((*tables)["rules"], rule)
			}
		case "proxies":
			for _, value := range flutterJSONList(value) {
				definition := flutterJSONMap(value)
				name := String(definition["name"])
				if name == "" {
					return errors.New("custom proxy has no name")
				}
				if proxyIDs[name] != 0 {
					if proxyDefinitions[name] != jsonText(definition) {
						return fmt.Errorf("conflicting custom proxy definitions for %q", name)
					}
				} else {
					if len((*tables)["custom_proxies"]) >= 100000 {
						return errors.New("too many exported local proxies")
					}
					id := newID()
					proxyIDs[name] = id
					proxyDefinitions[name] = jsonText(definition)
					(*tables)["custom_proxies"] = append((*tables)["custom_proxies"], map[string]any{"id": id, "definition": jsonText(definition), "order": flutterOrder(len(proxyIDs))})
				}
			}
		case "proxy-groups":
			if len((*tables)["proxy_groups"])+len(flutterJSONList(value)) > 100000 {
				return errors.New("too many exported groups")
			}
			for i, value := range flutterJSONList(value) {
				definition := copyFlutterRow(flutterJSONMap(value))
				name, kind := String(definition["name"]), String(definition["type"])
				if name == "" || kind == "" {
					return errors.New("invalid custom group")
				}
				proxies, use := definition["proxies"], definition["use"]
				for _, key := range []string{"name", "type", "proxies", "use"} {
					delete(definition, key)
				}
				group := map[string]any{"id": newID(), "profile_id": fid, "name": name, "type": kind, "definition": jsonText(definition), "order": flutterOrder(i)}
				if proxies != nil {
					group["proxies"] = jsonText(proxies)
				}
				if use != nil {
					group["use"] = jsonText(use)
				}
				(*tables)["proxy_groups"] = append((*tables)["proxy_groups"], group)
			}
		case "dns", "ntp", "sniffer", "tun":
			values, err := flutterOverrideValues(key, flutterJSONMap(value))
			if err != nil {
				return err
			}
			saved := flutterJSONMap(overrides[key])
			Merge(saved, values)
			overrides[key] = saved
			overrides[key+"-override-keys"] = flutterOverridePaths(values, "")
		case "proxy-providers":
			options := map[string]any{}
			for name, value := range flutterJSONMap(value) {
				def := copyFlutterRow(flutterJSONMap(value))
				path, e := rewrite(String(def["path"]))
				if e != nil {
					return e
				}
				if !strings.HasPrefix(path, "profiles/") {
					return fmt.Errorf("custom proxy provider %q must reference a profile", name)
				}
				delete(def, "path")
				delete(def, "type")
				options[name] = def
			}
			overrides[key] = options
		case "rule-providers":
			for name, value := range flutterJSONMap(value) {
				def := flutterJSONMap(value)
				remote := String(def["url"])
				path, e := rewrite(String(def["path"]))
				if e != nil {
					return e
				}
				found := false
				for _, provider := range (*tables)["clash_providers"] {
					if remote != "" && String(provider["url"]) == remote && String(provider["label"]) == name || remote == "" && flutterProviderEntry(String(provider["id"])) == path {
						behavior, format := String(def["behavior"]), String(def["format"])
						if behavior == "" {
							behavior = "classical"
						}
						if format == "" {
							format = "yaml"
						}
						provider["label"], provider["behavior"], provider["format"] = name, behavior, format
						found = true
						break
					}
				}
				if !found {
					if remote == "" {
						return fmt.Errorf("rule provider %q must reference a portable resource", name)
					}
					behavior, format := String(def["behavior"]), String(def["format"])
					if behavior == "" {
						behavior = "classical"
					}
					if format == "" {
						format = "yaml"
					}
					(*tables)["clash_providers"] = append((*tables)["clash_providers"], map[string]any{"id": newID(), "kind": "rule", "label": name, "url": remote, "behavior": behavior, "format": format})
				}
			}
		default:
			return fmt.Errorf("Flutter custom profiles cannot represent YAML field %q", key)
		}
	}
	row["overrides"] = jsonText(overrides)
	return nil
}

// Only fallback-filter/sniff are nested key namespaces in the original model.
// Policy/hosts/protocol maps are whole options, not arbitrary leaf key paths.
func flutterOverridePaths(values map[string]any, prefix string) []string {
	keys := []string{}
	for key, value := range values {
		if key == "fallback-filter" || key == "sniff" {
			keys = append(keys, flutterOverridePaths(flutterJSONMap(value), prefix+key+".")...)
		} else {
			keys = append(keys, prefix+key)
		}
	}
	sort.Strings(keys)
	return keys
}

func exportFlutterSettings(cfg Settings, source map[string]any) (map[string]any, error) {
	out := copyFlutterRow(source)
	exportFlutterOnDemand(cfg, out)
	section := func(key string) map[string]any { m := copyFlutterRow(flutterJSONMap(out[key])); out[key] = m; return m }
	app := section("appSettingProps")
	for key, value := range map[string]any{"locale": cfg.Locale, "dashboardWidgets": NormalizeDashboardWidgets(cfg.DashboardWidgets), "onlyStatisticsProxy": cfg.OnlyProxy, "autoLaunch": cfg.LaunchAtLogin, "silentLaunch": cfg.StartHidden, "autoRun": cfg.AutoStart, "openLogs": cfg.OpenLogs, "closeConnections": cfg.CloseConnections, "delayTestTimeout": cfg.TestTimeout, "floatingNavigationBar": cfg.FloatingNavigation, "sidebarExpanded": cfg.Sidebar, "minimizeOnExit": cfg.CloseToTray, "checkCertificate": !cfg.SkipCertificateVerify, "filters": cfg.Filters, "hideIp": cfg.HideIP, "serviceOrder": cfg.ServiceOrder, "disabledServices": cfg.DisabledServices} {
		app[key] = value
	}
	if cfg.Disclaimer {
		if Int(app["acceptedDisclaimerVersion"]) == 0 {
			app["acceptedDisclaimerVersion"] = 1
		}
	} else {
		app["acceptedDisclaimerVersion"] = 0
	}
	if cfg.Locale == "" {
		app["locale"] = nil
	}
	// Dart requires lists, not null, when a field is present.
	if cfg.Filters == nil {
		app["filters"] = []any{}
	}
	if cfg.ServiceOrder == nil {
		app["serviceOrder"] = []string{}
	}
	if cfg.DisabledServices == nil {
		app["disabledServices"] = []string{}
	}
	theme := section("themeProps")
	theme["liquidGlass"] = cfg.LiquidGlass
	if cfg.LiquidGlass == "" {
		theme["liquidGlass"] = "none"
	}
	theme["themeMode"], theme["colorStyle"], theme["pureBlack"], theme["systemFontFamily"] = cfg.Theme, cfg.ColorStyle, cfg.PureBlack, cfg.FontFamily
	color, e := strconv.ParseUint(strings.TrimPrefix(cfg.Accent, "#"), 16, 32)
	if e != nil || len(cfg.Accent) != 7 {
		return nil, errors.New("invalid accent color")
	}
	theme["primaryColor"] = color | 0xff000000
	text := copyFlutterRow(flutterJSONMap(theme["textScale"]))
	text["enable"], text["scale"] = true, cfg.Scale
	theme["textScale"] = text
	style := section("proxiesStyleProps")
	style["type"], style["layout"], style["cardType"], style["iconStyle"], style["hideTimeoutProxies"] = cfg.ProxyStyle, cfg.ProxyLayout, cfg.ProxyCard, cfg.ProxyIcon, cfg.HideTimeout
	sortType := cfg.ProxySort
	if sortType == "default" || sortType == "" {
		sortType = "none"
	}
	style["sortType"] = sortType
	window := section("windowGeometry")
	window["width"], window["height"] = cfg.WindowWidth, cfg.WindowHeight
	network := section("networkProps")
	network["systemProxy"], network["bypassPrivateRoute"], network["appendSystemDns"] = cfg.SystemProxy, cfg.BypassPrivateRoute, cfg.AppendSystemDNS
	bypass := []string{}
	for _, v := range strings.Split(cfg.Bypass, ",") {
		if v = strings.TrimSpace(v); v != "" {
			bypass = append(bypass, v)
		}
	}
	network["bypassDomain"] = bypass
	dav := section("davProps")
	dav["uri"], dav["user"] = cfg.WebDAVURL, cfg.WebDAVUser
	password, e := encodeFlutterDAVPassword(cfg.WebDAVPassword)
	if e != nil {
		return nil, e
	}
	dav["password"] = password
	patch := section("patchClashConfig")
	supported := ",mixed-port,port,socks-port,redir-port,tproxy-port,mode,allow-lan,log-level,find-process-mode,keep-alive-interval,keep-alive-idle,disable-keep-alive,routing-mark,geo-auto-update,geo-update-interval,geox-url,global-ua,hosts,secret,external-controller,interface-name,interface-name-mode,ipv6,unified-delay,tcp-concurrent,geodata-loader,"
	general := []string{}
	owned := runtimeDefaults()
	owned["hosts"], owned["interface-name"], owned["interface-name-mode"] = map[string]any{}, "", "clear"
	Merge(owned, cfg.Overrides)
	if cfg.Overrides["interface-name-mode"] == nil && String(cfg.Overrides["interface-name"]) != "" {
		owned["interface-name-mode"] = "custom"
	}
	for key, value := range owned {
		switch key {
		case "dns":
			if _, ok := value.(map[string]any); !ok {
				return nil, errors.New("DNS overrides must be a mapping")
			}
			values, err := flutterOverrideValues("dns", flutterJSONMap(value))
			if err != nil {
				return nil, err
			}
			// state/profile.dart intersects the app DNS selection with
			// DnsOverrideKey.normalProfileKeys for every non-custom profile.
			for _, profile := range cfg.Profiles {
				if profile.Type == "custom" {
					continue
				}
				for _, path := range flutterOverridePaths(values, "") {
					if !strings.Contains(flutterNormalDNSKeys, ","+path+",") {
						return nil, fmt.Errorf("Flutter ignores global DNS override %q for non-custom profiles; use native backup", path)
					}
				}
				break
			}
			saved := flutterJSONMap(patch[key])
			Merge(saved, values)
			patch[key] = saved
			patch["dns-override-keys"] = flutterOverridePaths(values, "")
		case "tun":
			if _, ok := value.(map[string]any); !ok {
				return nil, errors.New("TUN settings must be a mapping")
			}
			tun := copyFlutterRow(flutterJSONMap(value))
			base, extra := map[string]any{}, map[string]any{}
			for k, v := range tun {
				if k == "enable" || k == "device" || k == "stack" {
					base[k] = v
				} else {
					extra[k] = v
				}
			}
			// These two are owned and always enabled by both assemblers, not
			// members of Flutter's selectable ProfileTun schema.
			delete(extra, "auto-route")
			delete(extra, "auto-detect-interface")
			values, err := flutterOverrideValues("tun", extra)
			if err != nil {
				return nil, err
			}
			saved := flutterJSONMap(patch["tun-override"])
			Merge(saved, values)
			patch["tun"], patch["tun-override"], patch["tun-override-keys"] = base, saved, flutterOverridePaths(values, "")
		case "authentication":
			if value != nil {
				if _, ok := value.([]any); !ok {
					return nil, errors.New("inbound authentication must be a list")
				}
			}
			auth := flutterJSONList(value)
			props := copyFlutterRow(flutterJSONMap(network["authentication"]))
			props["enable"] = len(auth) > 0
			if len(auth) > 1 {
				return nil, errors.New("Flutter supports only one inbound authentication credential")
			}
			if len(auth) > 0 {
				parts := strings.SplitN(String(auth[0]), ":", 2)
				if len(parts) != 2 || parts[0] == "" {
					return nil, errors.New("invalid inbound authentication")
				}
				props["username"], props["password"] = parts[0], parts[1]
			}
			network["authentication"] = props
		case "rules": // Exported as per-profile extensions.
			if value != nil {
				if _, ok := value.([]any); !ok {
					return nil, errors.New("added rules must be a list")
				}
			}
		default:
			if !strings.Contains(supported, ","+key+",") {
				return nil, fmt.Errorf("Flutter patch settings cannot represent %q; use native backup", key)
			}
			if key == "hosts" {
				if _, ok := value.(map[string]any); !ok {
					return nil, errors.New("hosts settings must be a mapping")
				}
				value = flutterStringValues(flutterJSONMap(value))
			}
			if key == "external-controller" && String(value) != "" && String(value) != "127.0.0.1:9090" {
				return nil, errors.New("Flutter supports only its loopback controller address")
			}
			patch[key] = value
			if key == "ipv6" || key == "unified-delay" || key == "tcp-concurrent" || key == "geodata-loader" {
				general = append(general, key)
			}
		}
	}
	sort.Strings(general)
	patch["general-override-keys"] = general
	patch["test-url"] = cfg.TestURL
	// Preserve unknown original option values while clearing stale selected keys.
	if cfg.Overrides["dns"] == nil {
		patch["dns-override-keys"] = []string{}
	}
	hotkeys := []any{}
	for _, action := range []string{"start", "view", "mode", "proxy", "tun", "ruleMode", "globalMode", "directMode", "delayTest", "updateProfiles", "copyEnv", "exit"} {
		chord := cfg.Hotkeys[action]
		if chord == "" {
			continue
		}
		chord = CanonicalHotkey(chord)
		parts := strings.Split(chord, "+")
		key := parts[len(parts)-1]
		if key == "" {
			key = "+"
			parts = parts[:len(parts)-1]
		}
		usage := 0
		for n := 0x70004; n <= 0x700e7; n++ {
			if strings.EqualFold(hidKey(n), key) {
				usage = n
				break
			}
		}
		for _, n := range []int{0xc00b5, 0xc00b6, 0xc00b7, 0xc00cd, 0xc00e2, 0xc00e9, 0xc00ea} {
			if strings.EqualFold(hidKey(n), key) {
				usage = n
			}
		}
		if strings.HasPrefix(key, "USB:") {
			v, err := strconv.ParseUint(strings.TrimPrefix(key, "USB:"), 16, 32)
			if err == nil {
				usage = int(v)
			}
		}
		if usage == 0 {
			return nil, fmt.Errorf("Flutter cannot represent shortcut %q", chord)
		}
		mods := []string{}
		for _, mod := range parts[:len(parts)-1] {
			m := map[string]string{"Ctrl": "control", "Super": "meta", "Alt": "alt", "Shift": "shift", "Fn": "fn", "CapsLock": "capsLock"}[mod]
			if m == "" {
				return nil, fmt.Errorf("Flutter cannot represent shortcut modifier %q", mod)
			}
			mods = append(mods, m)
		}
		hotkeys = append(hotkeys, map[string]any{"action": action, "key": usage, "modifiers": mods})
	}
	out["hotKeyActions"] = hotkeys
	return out, nil
}
