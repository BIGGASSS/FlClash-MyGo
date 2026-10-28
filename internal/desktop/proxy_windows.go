//go:build windows

package desktop

import (
	"errors"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const internetKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func readProxy() (map[string]string, error) {
	k, e := registry.OpenKey(registry.CURRENT_USER, internetKey, registry.QUERY_VALUE)
	if e != nil {
		return nil, e
	}
	defer k.Close()
	m := map[string]string{}
	for _, name := range []string{"ProxyServer", "ProxyOverride", "AutoConfigURL"} {
		v, _, e := k.GetStringValue(name)
		if e == nil {
			m[name] = v
		} else if !errors.Is(e, registry.ErrNotExist) {
			return nil, e
		}
	}
	v, _, e := k.GetIntegerValue("ProxyEnable")
	if e == nil {
		m["ProxyEnable"] = strconv.FormatUint(v, 10)
	} else if !errors.Is(e, registry.ErrNotExist) {
		return nil, e
	}
	return m, nil
}
func proxyChanged() {
	p := windows.NewLazySystemDLL("wininet.dll").NewProc("InternetSetOptionW")
	_, _, _ = p.Call(0, 39, 0, 0)
	_, _, _ = p.Call(0, 37, 0, 0)
}
func setProxy(port int, bypass string) error {
	k, e := registry.OpenKey(registry.CURRENT_USER, internetKey, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	if e = k.SetStringValue("ProxyServer", "127.0.0.1:"+strconv.Itoa(port)); e != nil {
		return e
	}
	if e = k.SetStringValue("ProxyOverride", strings.ReplaceAll(bypass, ",", ";")); e != nil {
		return e
	}
	if e = k.SetDWordValue("ProxyEnable", 1); e != nil {
		return e
	}
	_ = k.DeleteValue("AutoConfigURL")
	proxyChanged()
	return nil
}
func restoreProxy(m map[string]string) error {
	k, e := registry.OpenKey(registry.CURRENT_USER, internetKey, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	for _, name := range []string{"ProxyServer", "ProxyOverride", "AutoConfigURL", "ProxyEnable"} {
		v, ok := m[name]
		if !ok {
			e = k.DeleteValue(name)
			if errors.Is(e, registry.ErrNotExist) {
				e = nil
			}
		} else if name == "ProxyEnable" {
			n, _ := strconv.ParseUint(v, 10, 32)
			e = k.SetDWordValue(name, uint32(n))
		} else {
			e = k.SetStringValue(name, v)
		}
		if e != nil {
			return e
		}
	}
	proxyChanged()
	return nil
}
