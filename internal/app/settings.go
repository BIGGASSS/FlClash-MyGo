package app

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *App) tools(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(8, 16, 24+a.bottomNavigationInset(c), 16).Gap(24).Children(func() {
			a.section(c, "", func() {
				pages := []string{"requests", "dns", "resources", "logs"}
				if a.smallDesktop(c) {
					pages = []string{"requests", "connections", "dns", "resources", "logs"}
				}
				for i, page := range pages {
					if i > 0 {
						a.separator(c)
					}
					label := a.t(page)
					if page == "dns" {
						label = a.t("dnsQueries")
					}
					a.linkRow(c, page, label, page)
				}
			})
			a.section(c, a.t("settings"), func() {
				for i, entry := range [][3]string{{"proxies", "language", "language"}, {"theme", "theme", "theme"}, {"cloud", "backupAndRestore", "backup"}, {"keyboard", "hotkey", "hotkeys"}, {"settings", "advancedConfig", "advanced"}, {"layers", "customProfileResources", "customResources"}, {"settings", "general", "general"}} {
					if i > 0 {
						a.separator(c)
					}
					a.linkRow(c, entry[0], a.t(entry[1]), entry[2])
				}
			})
			a.section(c, a.t("other"), func() {
				a.actionRow(c, "shield", a.t("disclaimer"), func() { a.disclaimer = true })
				a.separator(c)
				a.linkRow(c, "info", a.t("about"), "about")
			})
		})
	})
}

type configField struct {
	key, label, kind string
	options          []string
	min, max         float64
}

func field(key, label, kind string) configField {
	return configField{key: key, label: label, kind: kind}
}
func choose(key, label string, options ...string) configField {
	return configField{key: key, label: label, kind: "select", options: options}
}
func numberField(key, label string, lo, hi float64) configField {
	return configField{key: key, label: label, kind: "number", min: lo, max: hi}
}

var schemas = map[string][]configField{
	"network":      {field("ipv6", "IPv6", "bool"), choose("tun.stack", "stackMode", "mips", "mixed", "system", "gvisor"), field("tun.enable", "TUN", "bool"), field("tun.strict-route", "strictRoute", "bool"), field("tun.device", "device", "text"), field("tun.dns-hijack", "dnsHijacking", "list"), numberField("tun.mtu", "MTU", 576, 65535), field("tun.route-address", "routeAddress", "list"), field("tun.route-exclude-address", "routeExcludeAddress", "list"), field("tun.loopback-address", "loopback", "list"), choose("interface-name-mode", "interfaceNameMode", "clear", "follow", "custom"), field("interface-name", "interfaceName", "text"), field("disable-keep-alive", "disableKeepAlive", "bool"), numberField("keep-alive-idle", "keepAliveIdle", 0, 3600), numberField("keep-alive-interval", "keepAliveInterval", 0, 3600), numberField("routing-mark", "routingMark", 0, 2147483647)},
	"dnsSettings":  {field("dns.enable", "enable", "bool"), field("dns.listen", "listen", "text"), choose("dns.enhanced-mode", "enhancedMode", "fake-ip", "redir-host"), field("dns.ipv6", "IPv6", "bool"), field("dns.fake-ip-range", "fakeIPRange", "text"), choose("dns.fake-ip-filter-mode", "fakeIPFilterMode", "blacklist", "whitelist", "rule"), field("dns.fake-ip-filter", "fakeIPFilter", "list"), field("dns.default-nameserver", "defaultNameserver", "list"), field("dns.nameserver", "nameserver", "list"), field("dns.fallback", "fallback", "list"), field("dns.proxy-server-nameserver", "proxyServerNameserver", "list"), field("dns.direct-nameserver", "directNameserver", "list"), field("dns.nameserver-policy", "nameserverPolicy", "map"), field("dns.fallback-filter", "fallbackFilter", "map"), field("dns.use-hosts", "useHosts", "bool"), field("dns.use-system-hosts", "useSystemHosts", "bool"), field("dns.respect-rules", "respectRules", "bool"), choose("dns.cache-algorithm", "cacheAlgorithm", "lru", "arc"), numberField("dns.cache-max-size", "cacheMaxSize", 0, 1000000)},
	"sniffer":      {field("sniffer.enable", "enable", "bool"), field("sniffer.override-destination", "overrideDestination", "bool"), field("sniffer.force-dns-mapping", "forceDnsMapping", "bool"), field("sniffer.parse-pure-ip", "parsePureIP", "bool"), field("sniffer.sniff", "sniff", "map"), field("sniffer.force-domain", "forceDomain", "list"), field("sniffer.skip-domain", "skipDomain", "list"), field("sniffer.skip-dst-address", "skipDstAddress", "list"), field("sniffer.skip-src-address", "skipSrcAddress", "list")},
	"ntp":          {field("ntp.enable", "enable", "bool"), field("ntp.server", "server", "text"), numberField("ntp.port", "port", 1, 65535), numberField("ntp.interval", "interval", 1, 86400), field("ntp.write-to-system", "writeToSystem", "bool")},
	"ports":        {numberField("mixed-port", "mixedPort", 0, 65535), numberField("port", "httpPort", 0, 65535), numberField("socks-port", "socksPort", 0, 65535), numberField("redir-port", "redirPort", 0, 65535), numberField("tproxy-port", "tproxyPort", 0, 65535), field("bind-address", "bindAddress", "text")},
	"coreSettings": {field("tcp-concurrent", "tcpConcurrent", "bool"), field("unified-delay", "unifiedDelay", "bool"), field("global-client-fingerprint", "globalClientFingerprint", "text"), choose("geodata-loader", "geodataLoader", "memconservative", "standard"), field("geodata-mode", "geodataMode", "bool"), field("geo-auto-update", "geoAutoUpdate", "bool"), numberField("geo-update-interval", "geoUpdateInterval", 1, 8760), field("geox-url", "geoXUrl", "map"), field("profile.store-selected", "storeSelected", "bool"), field("profile.store-fake-ip", "storeFakeIP", "bool")},
}

func pathValue(m map[string]any, path string) any {
	parts := strings.Split(path, ".")
	for i, p := range parts {
		v := m[p]
		if i == len(parts)-1 {
			return v
		}
		nested, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		m = nested
	}
	return nil
}
func setPath(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	for _, p := range parts[:len(parts)-1] {
		m = store.Map(m, p)
	}
	key := parts[len(parts)-1]
	if value == nil {
		delete(m, key)
	} else {
		m[key] = value
	}
}
func (a *App) settingsView(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(8, 16, 24+a.bottomNavigationInset(c), 16).Gap(24).Children(func() {
			switch a.page {
			case "advanced":
				a.section(c, "", func() {
					for i, entry := range [][3]string{{"shield", "network", "network"}, {"dns", "DNS", "dnsSettings"}, {"table", "addedRules", "rules"}, {"edit", "script", "scripts"}} {
						if i > 0 {
							a.separator(c)
						}
						a.linkRow(c, entry[0], a.t(entry[1]), entry[2])
					}
				})
				a.section(c, "", func() {
					a.actionRow(c, "edit", a.t("edit"), func() { text, _ := store.YAML(a.cfg.Overrides); a.openCode(a.t("advancedConfig"), "global", text) })
				})
			case "general":
				a.generalSettings(c)
			case "proxySettings":
				a.section(c, a.t("proxies"), func() {
					a.choiceSetting(c, "style", &a.cfg.ProxyStyle, [][2]string{{"list", "list"}, {"tab", "tab"}})
					a.separator(c)
					a.choiceSetting(c, "layout", &a.cfg.ProxyLayout, [][2]string{{"loose", "loose"}, {"standard", "standard"}, {"tight", "tight"}})
					a.separator(c)
					a.choiceSetting(c, "size", &a.cfg.ProxyCard, [][2]string{{"loose", "loose"}, {"standard", "standard"}, {"compact", "compact"}})
					a.separator(c)
					a.choiceSetting(c, "iconStyle", &a.cfg.ProxyIcon, [][2]string{{"filled", "iconStyleFilled"}, {"plain", "iconStylePlain"}, {"hidden", "iconStyleHidden"}})
					a.separator(c)
					a.boolRow(c, "", a.t("hideTimeoutProxies"), &a.cfg.HideTimeout, a.save)
					a.separator(c)
					a.row(c, "table", a.t("columns"), func() {
						n := float64(a.cfg.ProxyColumns)
						if ui.NumberInput(c, &n, 0, 8, 1).Width(100).Changed() {
							a.cfg.ProxyColumns = int(n)
							a.save()
						}
						if a.cfg.ProxyColumns == 0 {
							ui.Text(c, a.t("auto")).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
						}
					})
					a.separator(c)
					a.row(c, "speed", a.t("sort"), func() {
						ui.Select(c, &a.cfg.ProxySort, []string{"default", "name", "delay"}).Width(150).OnChange(a.save)
					})
				})
			case "rules", "scripts", "customResources":
				keys := []string{"rules"}
				if a.page == "customResources" {
					keys = []string{"proxy-providers", "rule-providers", "proxy-groups", "proxies", "hosts", "listeners"}
				}
				if a.page == "scripts" {
					a.section(c, "", func() {
						for _, p := range a.cfg.Profiles {
							a.actionRow(c, "profiles", p.Name, func() {
								a.profileDraft = p
								a.editProfile = p.ID
								source := p.Script
								if source == "" {
									source = "const main = (config) => {\n  return config;\n};\n"
								}
								a.openCode(p.Name, "profile-script", source)
							})
						}
					})
				} else {
					a.section(c, "", func() {
						for i, key := range keys {
							if i > 0 {
								a.separator(c)
							}
							a.actionRow(c, "layers", humanKey(key), func() {
								if a.page == "customResources" && key == "proxies" {
									a.editLocalProxies()
								} else {
									a.editGlobal(key)
								}
							})
						}
					})
				}
			default:
				fields := schemas[a.page]
				a.section(c, "", func() {
					for i, f := range fields {
						if i > 0 {
							a.separator(c)
						}
						a.configRow(c, f)
					}
				})
				if a.page == "network" {
					a.section(c, a.t("system"), func() {
						a.boolRow(c, "settings", a.t("systemProxy"), &a.cfg.SystemProxy, a.apply)
						a.separator(c)
						a.boolRow(c, "", a.t("bypassPrivateRoute"), &a.cfg.BypassPrivateRoute, a.apply)
						a.separator(c)
						a.boolRow(c, "", a.t("appendSystemDns"), &a.cfg.AppendSystemDNS, a.apply)
						a.separator(c)
						a.row(c, "link", a.t("bypassDomain"), func() { ui.TextInput(c, &a.cfg.Bypass).Width(400).OnChange(a.save) })
					})
				}
			}
		})
	})
}
func (a *App) configRow(c *ui.Context, f configField) {
	label := a.t(f.label)
	if label == f.label && strings.Contains(f.label, ".") {
		label = humanKey(f.key)
	}
	value := pathValue(a.cfg.Overrides, f.key)
	inheritable := configInheritable(f.key)
	a.row(c, "", label, func() {
		if inheritable && value == nil {
			ui.Button(c.Key(f.key), a.t("followProfile")).Radius(999).OnClick(func() {
				setPath(a.cfg.Overrides, f.key, configOverrideDefault(f))
				a.apply()
			})
			return
		}
		switch f.kind {
		case "bool":
			v := store.Bool(value)
			if a.switchControl(c.Key(f.key), &v, label).Changed() {
				setPath(a.cfg.Overrides, f.key, v)
				a.apply()
			}
		case "number":
			v := float64(store.Int(value))
			input := ui.NumberInput(c.Key(f.key), &v, f.min, f.max, 1).Width(150).Label(label)
			if input.Changed() {
				setPath(a.cfg.Overrides, f.key, int(v))
				a.scheduleApply(c)
			}
			if input.Submitted() {
				a.apply()
			}
		case "select":
			v := store.String(value)
			if f.key == "interface-name-mode" && v == "" && store.String(a.cfg.Overrides["interface-name"]) != "" {
				v = "custom"
			}
			if v == "" && len(f.options) > 0 {
				v = f.options[0]
			}
			if ui.Select(c.Key(f.key), &v, f.options).Width(210).Label(label).Changed() {
				setPath(a.cfg.Overrides, f.key, v)
				a.apply()
			}
		case "list", "map":
			label := a.t("edit")
			if value != nil {
				switch v := value.(type) {
				case []any:
					label = fmt.Sprintf("%d", len(v))
				case map[string]any:
					label = fmt.Sprintf("%d", len(v))
				}
			}
			ui.Button(c.Key(f.key), label+"  ›").Radius(999).OnClick(func() { a.editGlobal(f.key) })
		default:
			v := store.String(value)
			input := ui.TextInput(c.Key(f.key), &v).Label(label).Width(330).Placeholder(a.t("defaultText"))
			if f.kind == "secret" {
				input.Password()
			}
			if input.Changed() {
				setPath(a.cfg.Overrides, f.key, v)
				if f.key == "interface-name" {
					a.cfg.Overrides["interface-name-mode"] = "custom"
				}
				a.scheduleApply(c)
			}
			if input.Submitted() {
				a.apply()
			}
		}
		if inheritable {
			ui.ButtonBase(c.Key("inherit-"+f.key)).Label(a.t("followProfile")).Tooltip(a.t("followProfile")).Size(32, 32).Center().OnClick(func() { setPath(a.cfg.Overrides, f.key, nil); a.apply() }).Children(func() { glyph(c, "close").FontSize(18).TextColor(a.colors.muted) })
		}
	}).ContextMenu(func(m *ui.Menu) {
		if m.Item(a.t("reset")).Chosen() {
			setPath(a.cfg.Overrides, f.key, nil)
			a.apply()
		}
	})
}

func configInheritable(key string) bool {
	if strings.HasPrefix(key, "dns.") || strings.HasPrefix(key, "sniffer.") || strings.HasPrefix(key, "ntp.") {
		return true
	}
	if strings.HasPrefix(key, "tun.") && key != "tun.enable" && key != "tun.stack" && key != "tun.device" {
		return true
	}
	switch key {
	case "unified-delay", "tcp-concurrent", "ipv6", "geodata-loader":
		return true
	}
	return false
}

func configOverrideDefault(f configField) any {
	switch f.kind {
	case "bool":
		return f.key == "tcp-concurrent" || f.key == "unified-delay" || f.key == "dns.use-hosts" || f.key == "dns.use-system-hosts"
	case "number":
		return int(f.min)
	case "select":
		if len(f.options) > 0 {
			return f.options[0]
		}
	case "list":
		return []any{}
	case "map":
		return map[string]any{}
	}
	return ""
}
func (a *App) generalSettings(c *ui.Context) {
	a.section(c, a.t("startupAndBackground"), func() {
		a.boolRow(c, "", a.t("autoLaunch"), &a.cfg.LaunchAtLogin, func() {
			a.save()
			if a.safe {
				return
			}
			if err := mygo.App.SetOpenAtLogin(a.cfg.LaunchAtLogin); err != nil {
				a.err = err.Error()
			}
		})
		a.separator(c)
		a.boolRow(c, "", a.t("silentLaunch"), &a.cfg.StartHidden, a.save)
		a.separator(c)
		a.boolRow(c, "", a.t("autoRun"), &a.cfg.AutoStart, a.save)
		a.separator(c)
		a.linkRow(c, "wifi", a.t("onDemand"), "onDemand")
		a.separator(c)
		a.boolRow(c, "", a.t("minimizeOnExit"), &a.cfg.CloseToTray, a.save)
	})
	a.section(c, a.t("requestsAndUpdates"), func() {
		a.row(c, "", a.t("testUrl"), func() { ui.TextInput(c, &a.cfg.TestURL).Width(360).Label(a.t("testUrl")).OnSubmit(a.apply) })
		a.separator(c)
		a.row(c, "", a.t("delayTestTimeout"), func() {
			v := fmt.Sprint(a.cfg.TestTimeout)
			if ui.Select(c, &v, []string{"500", "1000", "2000", "5000", "8000"}).Width(150).Changed() {
				_, _ = fmt.Sscan(v, &a.cfg.TestTimeout)
				a.save()
			}
			ui.Text(c, "ms")
		})
		a.separator(c)
		a.configRow(c, field("global-ua", "userAgent", "text"))
		a.separator(c)
		checkCertificate := !a.cfg.SkipCertificateVerify
		a.boolRow(c, "", a.t("checkCertificate"), &checkCertificate, func() { a.cfg.SkipCertificateVerify = !checkCertificate; a.save() })
	}, a.t("checkCertificateDesc"))
	a.section(c, a.t("inbound"), func() {
		a.linkRow(c, "", a.t("port"), "ports")
		a.separator(c)
		a.configRow(c, field("allow-lan", "allowLan", "bool"))
		a.separator(c)
		a.configRow(c, field("external-controller", "externalController", "text"))
		a.separator(c)
		a.configRow(c, field("secret", "externalControllerSecret", "secret"))
		a.separator(c)
		a.actionRow(c, "", a.t("authentication"), func() { a.editGlobal("authentication") })
	})
	a.section(c, a.t("connection"), func() {
		a.configRow(c, choose("find-process-mode", "findProcessMode", "strict", "always", "off"))
		a.separator(c)
		a.boolRow(c, "", a.t("onlyStatisticsProxy"), &a.cfg.OnlyProxy, a.apply)
		a.separator(c)
		a.boolRow(c, "", a.t("closeConnections"), &a.cfg.CloseConnections, a.save)
		a.separator(c)
		a.actionRow(c, "", "Hosts", func() { a.editGlobal("hosts") })
		a.separator(c)
		a.linkRow(c, "", a.t("sniffer"), "sniffer")
	})
	a.section(c, a.t("core"), func() {
		a.linkRow(c, "", a.t("settings"), "coreSettings")
		a.separator(c)
		a.linkRow(c, "", "NTP", "ntp")
	})
	a.section(c, a.t("logsAndDiagnostics"), func() {
		a.configRow(c, choose("log-level", "logLevel", "silent", "error", "warning", "info", "debug"))
		a.separator(c)
		a.boolRow(c, "", a.t("logs"), &a.cfg.OpenLogs, a.save)
		a.separator(c)
		a.linkRow(c, "", a.t("tools"), "probe")
	})
}
func (a *App) boolRow(c *ui.Context, icon, label string, value *bool, save func()) {
	ui.Row(c).FillWidth().MinHeight(56).Padding(4, 16).Gap(14).Children(func() {
		if icon != "" {
			glyph(c, icon).FontSize(22).TextColor(a.colors.muted)
		}
		ui.Text(c, label).FontSize(fontSize(c, 17)).Grow(1)
		if a.switchControl(c, value, label).Changed() {
			save()
		}
	})
}
func (a *App) choiceSetting(c *ui.Context, label string, value *string, options [][2]string) {
	a.row(c, "", a.t(label), func() {
		labels := make([]string, len(options))
		selected := ""
		for i, option := range options {
			labels[i] = a.t(option[1])
			if option[0] == *value {
				selected = labels[i]
			}
		}
		if selected == "" {
			selected = labels[0]
		}
		if ui.Select(c, &selected, labels).Width(190).Label(a.t(label)).Changed() {
			for i, v := range labels {
				if v == selected {
					*value = options[i][0]
					a.save()
					break
				}
			}
		}
	})
}
func (a *App) editGlobal(key string) {
	value := pathValue(a.cfg.Overrides, key)
	if value == nil {
		if strings.Contains(key, "provider") || strings.Contains(key, "policy") || key == "hosts" || strings.Contains(key, "filter") || key == "sniffer.sniff" {
			value = map[string]any{}
		} else {
			value = []any{}
		}
	}
	a.openCollection(humanKey(key), "global:"+key, value)
}
func (a *App) themeView(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(8, 16, 24+a.bottomNavigationInset(c), 16).Gap(24).Children(func() {
			a.section(c, a.t("theme"), func() {
				a.row(c, "theme", a.t("themeMode"), func() {
					old := a.cfg.Theme
					ui.Select(c, &a.cfg.Theme, []string{"system", "light", "dark"}).Width(190)
					if old != a.cfg.Theme {
						a.save()
					}
				})
				a.separator(c)
				a.row(c, "theme", a.t("colorStyle"), func() {
					ui.Select(c, &a.cfg.ColorStyle, []string{"vivid", "tinted", "soft", "monochrome"}).Width(190).OnChange(a.save)
				})
				a.separator(c)
				a.boolRow(c, "", a.t("pureBlack"), &a.cfg.PureBlack, a.save)
				a.separator(c)
				a.choiceSetting(c, "liquidGlass", &a.cfg.LiquidGlass, [][2]string{{"none", "none"}, {"standard", "standard"}, {"ultra", "ultra"}})
				a.separator(c)
				a.row(c, "layers", a.t("navigationBarStyle"), func() {
					choice := a.t("floating")
					if !a.cfg.FloatingNavigation {
						choice = a.t("docked")
					}
					if ui.Select(c, &choice, []string{a.t("floating"), a.t("docked")}).Width(190).Label(a.t("navigationBarStyle")).Changed() {
						a.cfg.FloatingNavigation = choice == a.t("floating")
						a.save()
					}
				})
			})
			a.section(c, a.t("primaryColor"), func() {
				ui.Row(c).Padding(20).Gap(16).Wrap().Children(func() {
					for _, hex := range []string{"#007aff", "#cb30e0", "#ff2d55", "#ff383c", "#ff8d28", "#ffcc00", "#34c759", "#8e8e93"} {
						ui.ButtonBase(c.Key(hex)).Label(hex).Size(42, 42).Radius(999).Background(ui.Hex(hex)).OnClick(func() { a.cfg.Accent = hex; a.save() }).Center().Children(func() {
							if a.cfg.Accent == hex {
								glyph(c, "check").TextColor(ui.Hex("#ffffff")).FontSize(22)
							}
						})
					}
				})
				a.row(c, "", a.t("custom"), func() {
					draft := a.cfg.Accent
					if ui.TextInput(c, &draft).Width(150).Label(a.t("primaryColor")).Submitted() {
						if len(draft) == 7 && draft[0] == '#' {
							a.cfg.Accent = draft
							a.save()
						}
					}
				})
			})
			a.section(c, a.t("fontSize"), func() {
				a.fontFamilySetting(c)
				a.separator(c)
				a.row(c, "", a.t("scale"), func() {
					v := float64(a.cfg.Scale * 100)
					if ui.NumberInput(c, &v, 80, 140, 5).Width(130).Label(a.t("scale")).Changed() {
						a.cfg.Scale = float32(v / 100)
						a.save()
					}
					ui.Text(c, "%")
				})
			})
			a.card(c).Children(func() {
				a.header(c, "theme", a.t("preview"))
				ui.Text(c, "FlClash").FontSize(fontSize(c, 28)).Bold()
				ui.Text(c, a.t("networkSpeed")).TextColor(a.colors.muted)
				ui.Row(c).Gap(12).Children(func() {
					a.chip(c, a.t("running"), a.colors.success)
					ui.PrimaryButton(c, a.t("confirm")).Radius(999)
					ui.Button(c, a.t("cancel")).Radius(999)
				})
			})
		})
	})
}
func (a *App) languageView(c *ui.Context) {
	ui.Scroll(c).Fill().Padding(8, 16, 24+a.bottomNavigationInset(c), 16).Children(func() {
		a.section(c, "", func() {
			for i, item := range [][2]string{{"", "System"}, {"en", "English"}, {"zh_CN", "简体中文"}, {"ja", "日本語"}, {"ko", "한국어"}, {"ru", "Русский"}, {"fr", "Français"}, {"es", "Español"}} {
				if i > 0 {
					a.separator(c)
				}
				ui.ButtonBase(c.Key(item[0])).FillWidth().OnClick(func() { a.cfg.Locale = item[0]; a.save() }).Children(func() {
					a.row(c, "", item[1], func() {
						if a.cfg.Locale == item[0] {
							glyph(c, "check").TextColor(a.colors.accent)
						}
					})
				})
			}
		})
	})
}
func (a *App) hotkeysView(c *ui.Context) {
	a.hotkeySettings(c)
}
func (a *App) backupView(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(8, 16, 24+a.bottomNavigationInset(c), 16).Gap(24).Children(func() {
			a.section(c, a.t("local"), func() {
				a.actionRow(c, "up", a.t("backup"), a.backupLocal)
				a.separator(c)
				a.actionRow(c, "up", a.t("backup")+" · FlClash (Flutter)", func() { a.backupLocalFormat(true) })
				a.separator(c)
				a.actionRow(c, "down", a.t("restore"), a.restoreLocal)
			})
			a.section(c, "WebDAV", func() {
				a.row(c, "link", a.t("url"), func() { ui.TextInput(c, &a.cfg.WebDAVURL).Width(360).OnChange(a.save) })
				a.separator(c)
				a.row(c, "", a.t("username"), func() { ui.TextInput(c, &a.cfg.WebDAVUser).Width(280).OnChange(a.save) })
				a.separator(c)
				a.row(c, "", a.t("password"), func() { ui.TextInput(c, &a.cfg.WebDAVPassword).Password().Width(280).OnChange(a.save) })
				a.separator(c)
				a.actionRow(c, "up", a.t("backup"), func() { a.webdavBackup(false) })
				a.separator(c)
				a.actionRow(c, "down", a.t("restore"), func() { a.ask(a.t("restore"), a.t("restore"), func() { a.webdavBackup(true) }) })
			})
		})
	})
}
func (a *App) backupLocal() { a.backupLocalFormat(false) }
func (a *App) restoreLocal() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.window, Title: a.t("restore"), Filters: []mygo.FileFilter{{Name: "Backup", Extensions: []string{"zip"}}}})
		a.update(func() {
			if err != nil {
				a.err = err.Error()
				return
			}
			if len(paths) == 0 {
				return
			}
			a.ask(a.t("restore"), a.t("restore"), func() {
				a.work(func() (func(), error) {
					b, e := os.ReadFile(paths[0])
					if e != nil {
						return nil, e
					}
					cfg, e := a.store.Restore(b)
					return func() { a.restoredSettings(cfg) }, e
				})
			})
		})
	}()
}
func (a *App) webdavBackup(restore bool) {
	if a.safe {
		return
	}
	cfg := store.Clone(a.cfg)
	s := a.store
	client := a.httpClient()
	a.work(func() (func(), error) {
		defer client.CloseIdleConnections()
		if restore {
			b, e := store.WebDAV(client, cfg, http.MethodGet, nil)
			if e != nil {
				return nil, e
			}
			next, e := s.Restore(b)
			return func() { a.restoredSettings(next) }, e
		}
		b, e := s.Backup(cfg)
		if e != nil {
			return nil, e
		}
		_, e = store.WebDAV(client, cfg, http.MethodPut, b)
		return func() { a.message = a.t("success") }, e
	})
}
func (a *App) resources(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(8, 16, 24+a.bottomNavigationInset(c), 16).Gap(20).Children(func() {
			a.section(c, "GeoData", func() {
				for i, name := range []string{"GEOIP.metadb", "GEOIP.dat", "GEOSITE.dat", "ASN.mmdb"} {
					if i > 0 {
						a.separator(c)
					}
					a.row(c, "resources", name, func() {
						ui.Button(c, a.t("update")).Radius(999).OnClick(func() {
							kind := map[string]string{"GEOIP.metadb": "MMDB", "GEOIP.dat": "GEOIP", "GEOSITE.dat": "GEOSITE", "ASN.mmdb": "ASN"}[name]
							a.coreMessage("updateGeoData", kind)
						})
					})
				}
			})
			a.section(c, a.t("providers"), func() {
				if len(a.sample.Providers) == 0 {
					a.row(c, "cloud", a.t("noData"), nil)
				}
				for i, p := range a.sample.Providers {
					if i > 0 {
						a.separator(c)
					}
					a.row(c, "cloud", p.Name, func() {
						ui.Textf(c, "%s · %d", p.Type, p.Count).TextColor(a.colors.muted).FontSize(fontSize(c, 13))
						a.button(c, "refresh", a.t("update")+p.Name, func() { a.updateDetailProvider(p) })
						a.button(c, "info", a.t("details")+p.Name, func() { a.providerInfo(p) })
					})
				}
			})
		})
	})
}
func (a *App) aboutView(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(28).Gap(22).Children(func() {
			ui.Image(c, a.logo).Size(88, 88).AlignSelf(ui.Center)
			ui.Text(c, "FlClash").FontSize(fontSize(c, 30)).Bold().AlignSelf(ui.Center)
			ui.Text(c, "MyGo  "+Version).TextColor(a.colors.muted).AlignSelf(ui.Center)
			a.section(c, "", func() {
				a.actionRow(c, "link", "FlClash", func() { c.Services().OpenURL("https://github.com/chen08209/FlClash") })
				a.separator(c)
				a.actionRow(c, "link", "MyGo", func() { c.Services().OpenURL("https://github.com/egoist/mygo") })
				a.separator(c)
				a.actionRow(c, "link", a.t("sourceCode"), func() { c.Services().OpenURL("https://github.com/BIGGASSS/FlClash-MyGo") })
				a.separator(c)
				a.actionRow(c, "info", "GPL-3.0", func() { a.detail = string(assets.Read("LICENSE")); a.detailOpen = true })
			})
			ui.Text(c, "FlClash © chen08209 and contributors. MyGo © egoist and contributors.").FontSize(fontSize(c, 12)).TextColor(a.colors.muted).AlignSelf(ui.Center)
		})
	})
}
func (a *App) servicesView(c *ui.Context) {
	a.serviceRows(c)
}
func (a *App) checkServices() {
	a.refreshServices(a.enabledServiceTargets())
}
func (a *App) probeView(c *ui.Context) {
	ui.Column(c).Fill().Padding(16).Gap(20).Children(func() {
		a.section(c, "", func() {
			a.row(c, "link", a.t("url"), func() { ui.TextInput(c, &a.probeURL).Width(420).Label(a.t("url")) })
			a.row(c, "proxies", a.t("proxy"), func() {
				names := []string{""}
				for name := range a.sample.Proxies.Proxies {
					names = append(names, name)
				}
				ui.Select(c, &a.probeProxy, names).Width(250)
			})
			a.actionRow(c, "play", a.t("start"), a.probe)
			a.actionRow(c, "trash", a.t("clearDnsCache"), func() { a.coreAction("clearDnsCache", nil) })
			a.actionRow(c, "refresh", a.t("resetNetwork"), func() { a.coreAction("resetConnections", nil) })
		})
		ui.Scroll(c).Grow(1).FillWidth().Children(func() { ui.Text(c, a.probeResult).Selectable().FontSize(fontSize(c, 14)) })
	})
}
func (a *App) probe() {
	if a.engine == nil || a.safe {
		return
	}
	e := a.engine
	address, proxy := a.probeURL, a.probeProxy
	a.work(func() (func(), error) {
		var result map[string]any
		err := e.Call("probe", map[string]any{"url": address, "proxy-name": proxy, "timeout": 10000, "max-body": 65536}, &result)
		text, _ := store.YAML(result)
		return func() { a.probeResult = text }, err
	})
}

func (a *App) editLocalProxies() {
	value := []any{}
	for _, p := range store.Clone(a.cfg).LocalProxies {
		value = append(value, p)
	}
	a.openCollection(a.t("proxies"), "library:proxies", value)
}
