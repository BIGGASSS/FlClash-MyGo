package store

import (
	"fmt"
	"strings"
)

// The paths are the pinned enum.dart OverrideKey.path values. Flutter discards
// unknown selected keys; treating that as a successful export would lose data.
const flutterNormalDNSKeys = ",listen,ipv6,enhanced-mode,fake-ip-filter,fake-ip-filter-mode,default-nameserver,nameserver,fallback,proxy-server-nameserver,direct-nameserver,"

var flutterOverrideKeys = map[string]string{
	"dns":     ",enable,listen,listen-routing-mark,use-hosts,use-system-hosts,ipv6,ipv6-timeout,respect-rules,prefer-h3,cache-algorithm,cache-max-size,enhanced-mode,fake-ip-range,fake-ip-range6,fake-ip-filter,fake-ip-filter-mode,fake-ip-ttl,default-nameserver,nameserver-policy,nameserver,fallback,fallback-lazy-query,proxy-server-nameserver,proxy-server-nameserver-policy,direct-nameserver,direct-nameserver-follow-policy,fallback-filter.geoip,fallback-filter.geoip-code,fallback-filter.geosite,fallback-filter.ipcidr,fallback-filter.domain,",
	"ntp":     ",enable,server,port,interval,dialer-proxy,write-to-system,",
	"sniffer": ",enable,override-destination,force-dns-mapping,parse-pure-ip,force-domain,skip-domain,skip-src-address,skip-dst-address,sniff.HTTP,sniff.TLS,sniff.QUIC,",
	"tun":     ",mtu,congestion-controller,strict-route,route-address,route-exclude-address,dns-hijack,udp-timeout,disable-icmp-forwarding,gso,include-interface,exclude-interface,include-uid,exclude-uid,exclude-src-port,exclude-dst-port,",
}

func containsParentSegment(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func validateFlutterCustomProviders(source map[string]any, profileLabels, resourceLabels, aliases map[string]string, rewrite func(string) (string, error)) error {
	for name, value := range flutterJSONMap(source["proxy-providers"]) {
		path, err := rewrite(String(flutterJSONMap(value)["path"]))
		if err != nil {
			return err
		}
		label := profileLabels[path]
		if label == "" || label != name && aliases[name] != label {
			return fmt.Errorf("custom proxy provider %q must name the profile its path references", name)
		}
	}
	for name, value := range flutterJSONMap(source["rule-providers"]) {
		def := flutterJSONMap(value)
		if String(def["url"]) != "" {
			continue
		}
		path, err := rewrite(String(def["path"]))
		if err != nil {
			return err
		}
		if old := resourceLabels[path]; old != "" && old != name {
			return fmt.Errorf("Flutter cannot share one custom rule resource under both %q and %q", old, name)
		}
		resourceLabels[path] = name
	}
	return nil
}

func validateFlutterFileProviders(body []byte) error {
	config, err := ParseYAML(body)
	if err != nil {
		return err
	}
	for _, section := range []string{"proxy-providers", "rule-providers"} {
		for name, value := range flutterJSONMap(config[section]) {
			def := flutterJSONMap(value)
			if String(def["type"]) == "file" {
				// common/task.dart confines all source providers *before* custom
				// library injection. Restore copies no files to that cache path.
				return fmt.Errorf("Flutter rewrites source file provider %q to an unbacked cache path; use native backup", name)
			}
		}
	}
	return nil
}

func flutterOverrideValues(section string, values map[string]any) (map[string]any, error) {
	for _, key := range flutterOverridePaths(values, "") {
		if !strings.Contains(flutterOverrideKeys[section], ","+key+",") {
			return nil, fmt.Errorf("Flutter cannot represent %s override %q; use native backup", section, key)
		}
	}
	if section == "dns" {
		for _, key := range []string{"nameserver-policy", "proxy-server-nameserver-policy"} {
			if values[key] != nil {
				values[key] = flutterStringValues(flutterJSONMap(values[key]))
			}
		}
	}
	return values, nil
}

func flutterStringValues(values map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		if list, ok := value.([]any); ok {
			parts := []string{}
			for _, v := range list {
				parts = append(parts, String(v))
			}
			out[key] = strings.Join(parts, ",")
		} else {
			out[key] = value
		}
	}
	return out
}
