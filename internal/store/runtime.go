package store

import (
	"slices"
	"strings"
)

var privateRoutes = []string{"0.0.0.0/8", "10.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.168.0.0/16", "224.0.0.0/4", "255.255.255.255/32", "fc00::/7", "fe80::/10", "ff00::/8"}

func runtimeDefaults() map[string]any {
	return map[string]any{
		"mixed-port": 7890, "port": 0, "socks-port": 0, "redir-port": 0, "tproxy-port": 0,
		"mode": "rule", "allow-lan": false, "log-level": "error", "find-process-mode": "off",
		"keep-alive-interval": 30, "keep-alive-idle": 15, "disable-keep-alive": false, "routing-mark": 0,
		"geo-auto-update": false, "geo-update-interval": 24, "global-ua": "FlClash/MyGo",
		"geox-url": map[string]any{
			"mmdb":    "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb",
			"asn":     "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/GeoLite2-ASN.mmdb",
			"geoip":   "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.dat",
			"geosite": "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat",
		},
		"secret": "", "external-controller": "", "authentication": []any{},
		"tun": map[string]any{"enable": false, "device": "FlClash", "stack": "mips", "auto-route": true, "auto-detect-interface": true, "dns-hijack": []any{"any:53"}},
	}
}

func baselineDNS(raw map[string]any) {
	dns := Map(raw, "dns")
	if !Bool(dns["enable"]) {
		Merge(dns, map[string]any{"enable": true, "enhanced-mode": "fake-ip", "nameserver": []any{"https://doh.pub/dns-query", "https://dns.alidns.com/dns-query"}})
	}
}

func runtimeOwned(raw map[string]any, cfg Settings, profileOverride map[string]any) {
	owned := runtimeDefaults()
	Merge(owned, Clone(cfg).Overrides)
	Merge(owned, profileOverride)
	for _, key := range []string{"mixed-port", "port", "socks-port", "redir-port", "tproxy-port", "mode", "allow-lan", "log-level", "find-process-mode", "keep-alive-interval", "keep-alive-idle", "disable-keep-alive", "routing-mark", "geo-auto-update", "geo-update-interval", "geox-url", "global-ua", "secret", "authentication"} {
		raw[key] = owned[key]
	}
	raw["external-controller"] = ""
	if String(owned["secret"]) != "" {
		raw["external-controller"] = String(owned["external-controller"])
	}
	for _, key := range []string{"external-controller-tls", "external-controller-unix", "external-controller-pipe", "external-ui", "external-ui-url"} {
		raw[key] = ""
	}
	raw["skip-auth-prefixes"] = []any{}
	if iptables, ok := raw["iptables"].(map[string]any); ok && strings.Contains(String(iptables["inbound-interface"]), " ") {
		raw["iptables"] = map[string]any{"enable": false}
	}
	mode := String(owned["interface-name-mode"])
	if mode == "" && String(owned["interface-name"]) != "" {
		mode = "custom"
	}
	switch mode {
	case "follow":
	case "custom":
		raw["interface-name"] = String(owned["interface-name"])
	default:
		raw["interface-name"] = ""
	}
	delete(raw, "interface-name-mode")
	if hosts, ok := owned["hosts"].(map[string]any); ok {
		for name, value := range hosts {
			Map(raw, "hosts")[name] = splitConfigServers(value)
		}
	}
	tun, settings := Map(raw, "tun"), Map(owned, "tun")
	for _, key := range []string{"enable", "device", "stack"} {
		tun[key] = settings[key]
	}
	tun["auto-route"], tun["auto-detect-interface"] = true, true
	if cfg.BypassPrivateRoute {
		routes, _ := tun["route-exclude-address"].([]any)
		routes = slices.Clone(routes)
		for _, route := range privateRoutes {
			routes = append(routes, route)
		}
		tun["route-exclude-address"] = routes
	}
	if cfg.AppendSystemDNS {
		dns := Map(raw, "dns")
		servers, _ := dns["nameserver"].([]any)
		found := false
		for _, value := range servers {
			if server, ok := value.(string); ok && server == "system://" {
				found = true
			}
		}
		if !found {
			dns["nameserver"] = append(slices.Clone(servers), "system://")
		}
	}
	Map(raw, "profile")["store-selected"] = false
	if sniffer, ok := raw["sniffer"].(map[string]any); ok {
		if sniff, ok := sniffer["sniff"].(map[string]any); ok {
			for _, value := range sniff {
				if entry, ok := value.(map[string]any); ok {
					if ports, ok := entry["ports"].([]any); ok {
						for i, port := range ports {
							ports[i] = String(port)
						}
					}
				}
			}
		}
	}
}

func splitConfigServers(value any) any {
	if text, ok := value.(string); ok {
		parts := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
		if len(parts) > 1 {
			values := make([]any, len(parts))
			for i, part := range parts {
				values[i] = part
			}
			return values
		}
	}
	return value
}

// Rule.parse treats comma-bearing payloads differently from ordinary rules.
// SUB-RULE's last field names another rule set, not an outbound target.
func ruleTargetIndex(fields []string) int {
	if len(fields) == 0 {
		return -1
	}
	switch strings.ToUpper(strings.TrimSpace(fields[0])) {
	case "SUB-RULE":
		return -1
	case "MATCH":
		if len(fields) > 1 {
			return 1
		}
	case "AND", "OR", "NOT", "DOMAIN-REGEX", "PROCESS-NAME-REGEX", "PROCESS-PATH-REGEX":
		if len(fields) > 2 {
			return len(fields) - 1
		}
	default:
		if len(fields) > 2 {
			return 2
		}
	}
	return -1
}

func lastMatchTarget(value any) string {
	rules, _ := value.([]any)
	for i := len(rules) - 1; i >= 0; i-- {
		fields := strings.Split(String(rules[i]), ",")
		if len(fields) > 1 && strings.EqualFold(strings.TrimSpace(fields[0]), "MATCH") {
			if target := strings.TrimSpace(fields[1]); target != "" {
				return target
			}
		}
	}
	return ""
}

func resolveAddedRuleTargets(override map[string]any, target string) {
	if target == "" {
		return
	}
	rules, ok := override["rules"].([]any)
	if !ok {
		return
	}
	out := slices.Clone(rules)
	for i, value := range rules {
		fields := strings.Split(String(value), ",")
		index := ruleTargetIndex(fields)
		if index >= 0 && strings.EqualFold(strings.TrimSpace(fields[index]), "MATCH") {
			fields[index] = target
			out[i] = strings.Join(fields, ",")
		}
	}
	override["rules"] = out
}

func mergeSnifferOverride(raw, override map[string]any) {
	for key, value := range override {
		sniff, ok := value.(map[string]any)
		if key != "sniff" || !ok {
			raw[key] = value
			continue
		}
		current, _ := raw["sniff"].(map[string]any)
		if len(current) == 0 {
			current = map[string]any{}
			if legacy, ok := raw["sniffing"].([]any); ok {
				for _, name := range legacy {
					entry := map[string]any{}
					if ports, ok := raw["port-whitelist"].([]any); ok {
						entry["ports"] = slices.Clone(ports)
					}
					current[strings.ToUpper(String(name))] = entry
				}
			}
		}
		replaced := map[string]bool{}
		for name := range sniff {
			replaced[strings.ToUpper(name)] = true
		}
		merged := map[string]any{}
		for name, entry := range current {
			if !replaced[strings.ToUpper(name)] {
				merged[name] = entry
			}
		}
		for name, entry := range sniff {
			merged[name] = entry
		}
		raw[key] = merged
	}
}
