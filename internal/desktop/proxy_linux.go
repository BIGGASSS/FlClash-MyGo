//go:build linux

package desktop

import (
	"strconv"
	"strings"
)

var proxyKeys = [][2]string{{"org.gnome.system.proxy", "mode"}, {"org.gnome.system.proxy", "ignore-hosts"}, {"org.gnome.system.proxy.http", "host"}, {"org.gnome.system.proxy.http", "port"}, {"org.gnome.system.proxy.https", "host"}, {"org.gnome.system.proxy.https", "port"}, {"org.gnome.system.proxy.socks", "host"}, {"org.gnome.system.proxy.socks", "port"}}

func readProxy() (map[string]string, error) {
	m := map[string]string{}
	for _, k := range proxyKeys {
		v, e := run("gsettings", "get", k[0], k[1])
		if e != nil {
			return nil, e
		}
		m[k[0]+"/"+k[1]] = v
	}
	return m, nil
}
func setProxy(port int, bypass string) error {
	for _, schema := range []string{"org.gnome.system.proxy.http", "org.gnome.system.proxy.https", "org.gnome.system.proxy.socks"} {
		if _, e := run("gsettings", "set", schema, "host", "127.0.0.1"); e != nil {
			return e
		}
		if _, e := run("gsettings", "set", schema, "port", strconv.Itoa(port)); e != nil {
			return e
		}
	}
	var hosts []string
	for _, h := range strings.Split(bypass, ",") {
		if h = strings.TrimSpace(h); h != "" && h != "<local>" {
			hosts = append(hosts, "'"+strings.ReplaceAll(h, "'", "\\'")+"'")
		}
	}
	if _, e := run("gsettings", "set", "org.gnome.system.proxy", "ignore-hosts", "["+strings.Join(hosts, ",")+"]"); e != nil {
		return e
	}
	_, e := run("gsettings", "set", "org.gnome.system.proxy", "mode", "manual")
	return e
}
func restoreProxy(m map[string]string) error {
	for _, k := range proxyKeys {
		v, ok := m[k[0]+"/"+k[1]]
		if !ok {
			continue
		}
		if _, e := run("gsettings", "set", k[0], k[1], v); e != nil {
			return e
		}
	}
	return nil
}
