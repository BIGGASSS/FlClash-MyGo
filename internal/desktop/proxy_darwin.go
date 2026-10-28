//go:build darwin

package desktop

import (
	"errors"
	"strconv"
	"strings"
)

func networkServices() ([]string, error) {
	out, e := run("/usr/sbin/networksetup", "-listallnetworkservices")
	if e != nil {
		return nil, e
	}
	var services []string
	for i, s := range strings.Split(out, "\n") {
		if i > 0 && !strings.HasPrefix(s, "*") && s != "" {
			services = append(services, s)
		}
	}
	if len(services) == 0 {
		return nil, errors.New("no active network services")
	}
	return services, nil
}
func readProxy() (map[string]string, error) {
	services, e := networkServices()
	if e != nil {
		return nil, e
	}
	m := map[string]string{"services": strings.Join(services, "\n")}
	for _, s := range services {
		for _, kind := range []string{"webproxy", "securewebproxy", "socksfirewallproxy", "proxybypassdomains"} {
			out, e := run("/usr/sbin/networksetup", "-get"+kind, s)
			if e != nil {
				return nil, e
			}
			m[s+"/"+kind] = out
		}
	}
	return m, nil
}
func setProxy(port int, bypass string) error {
	services, e := networkServices()
	if e != nil {
		return e
	}
	for _, s := range services {
		for _, kind := range []string{"webproxy", "securewebproxy", "socksfirewallproxy"} {
			if _, e = run("/usr/sbin/networksetup", "-set"+kind, s, "127.0.0.1", strconv.Itoa(port)); e != nil {
				return e
			}
			if _, e = run("/usr/sbin/networksetup", "-set"+kind+"state", s, "on"); e != nil {
				return e
			}
		}
		hosts := []string{}
		for _, h := range strings.Split(bypass, ",") {
			if h != "<local>" {
				hosts = append(hosts, strings.TrimSpace(h))
			}
		}
		if len(hosts) == 0 {
			hosts = []string{"Empty"}
		}
		if _, e = run("/usr/sbin/networksetup", append([]string{"-setproxybypassdomains", s}, hosts...)...); e != nil {
			return e
		}
	}
	return nil
}
func restoreProxy(m map[string]string) error {
	for _, s := range strings.Split(m["services"], "\n") {
		for _, kind := range []string{"webproxy", "securewebproxy", "socksfirewallproxy"} {
			fields := map[string]string{}
			for _, line := range strings.Split(m[s+"/"+kind], "\n") {
				p := strings.SplitN(line, ": ", 2)
				if len(p) == 2 {
					fields[p[0]] = p[1]
				}
			}
			if fields["Server"] != "" && fields["Port"] != "0" {
				if _, e := run("/usr/sbin/networksetup", "-set"+kind, s, fields["Server"], fields["Port"]); e != nil {
					return e
				}
			}
			enabled := "off"
			if fields["Enabled"] == "Yes" {
				enabled = "on"
			}
			if _, e := run("/usr/sbin/networksetup", "-set"+kind+"state", s, enabled); e != nil {
				return e
			}
		}
		hosts := strings.Split(m[s+"/proxybypassdomains"], "\n")
		if len(hosts) == 0 || strings.HasPrefix(hosts[0], "There aren't") {
			hosts = []string{"Empty"}
		}
		if _, e := run("/usr/sbin/networksetup", append([]string{"-setproxybypassdomains", s}, hosts...)...); e != nil {
			return e
		}
	}
	return nil
}
