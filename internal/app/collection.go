package app

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
	"go.yaml.in/yaml/v3"
)

type documentEditor struct {
	Title         string
	Purpose       string
	Value         any
	Path          []any
	List          ui.ListState
	AddOpen       bool
	NewKey        string
	NewKind       string
	RuleOpen      bool
	RuleIndex     int
	RuleType      string
	RuleValue     string
	RuleTarget    string
	RuleNoResolve bool
}

func (a *App) openCollection(title, purpose string, value any) {
	b, _ := yaml.Marshal(value)
	var clone any
	_ = yaml.Unmarshal(b, &clone)
	a.document = &documentEditor{Title: title, Purpose: purpose, Value: clone, NewKind: "text"}
	a.goPage("collection")
}
func (d *documentEditor) current() any {
	v := d.Value
	for _, key := range d.Path {
		switch k := key.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v = m[k]
		case int:
			list, ok := v.([]any)
			if !ok || k < 0 || k >= len(list) {
				return nil
			}
			v = list[k]
		}
	}
	return v
}
func replaceAt(v any, path []any, value any) any {
	if len(path) == 0 {
		return value
	}
	switch key := path[0].(type) {
	case string:
		if m, ok := v.(map[string]any); ok {
			m[key] = replaceAt(m[key], path[1:], value)
		}
	case int:
		if list, ok := v.([]any); ok && key >= 0 && key < len(list) {
			list[key] = replaceAt(list[key], path[1:], value)
		}
	}
	return v
}
func (d *documentEditor) replace(value any) { d.Value = replaceAt(d.Value, d.Path, value) }
func (d *documentEditor) push(key any)      { d.Path = append(d.Path, key); d.List = ui.ListState{} }
func (a *App) collectionView(c *ui.Context) {
	d := a.document
	if d == nil {
		return
	}
	ui.Column(c).Fill().Padding(0, 16, 16, 16).Gap(14).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, a.collectionBreadcrumb()).TextColor(a.colors.muted).FontSize(fontSize(c, 13)).Grow(1).SingleLine()
			a.button(c, "edit", a.t("edit"), func() { text, _ := store.YAML(d.Value); a.openCode(d.Title, "document", text) })
		})
		switch value := d.current().(type) {
		case map[string]any:
			keys := make([]string, 0, len(value))
			for key := range value {
				keys = append(keys, key)
			}
			sort.SliceStable(keys, func(i, j int) bool {
				rank := func(s string) int {
					switch s {
					case "name":
						return 0
					case "type":
						return 1
					case "server":
						return 2
					case "port":
						return 3
					case "password":
						return 4
					default:
						return 5
					}
				}
				if rank(keys[i]) != rank(keys[j]) {
					return rank(keys[i]) < rank(keys[j])
				}
				return keys[i] < keys[j]
			})
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				a.section(c, "", func() {
					for i, key := range keys {
						if i > 0 {
							a.separator(c)
						}
						a.documentRow(c, key, value[key], func(v any) { value[key] = v }, func() { d.push(key) }, func() { delete(value, key) })
					}
				})
			})
		case []any:
			d.List.Reorder = func(rows []int, to int) {
				selected := map[int]bool{}
				var moved, rest []any
				before := 0
				for _, i := range rows {
					selected[i] = true
				}
				for i, v := range value {
					if selected[i] {
						moved = append(moved, v)
						if i < to {
							before++
						}
					} else {
						rest = append(rest, v)
					}
				}
				at := max(0, min(to-before, len(rest)))
				out := append([]any{}, rest[:at]...)
				out = append(out, moved...)
				out = append(out, rest[at:]...)
				d.replace(out)
			}
			ui.List(c, &d.List, len(value), func(i int) {
				v := value[i]
				ui.Row(c.Key(i)).FillWidth().Padding(14, 16).Radius(20).Background(a.colors.card).Margin(0, 0, 8, 0).Gap(10).Children(func() {
					glyph(c, "list").FontSize(18).TextColor(a.colors.muted)
					switch x := v.(type) {
					case map[string]any:
						label := store.String(x["name"])
						if label == "" {
							label = store.String(x["type"])
						}
						if label == "" {
							label = strconv.Itoa(i + 1)
						}
						ui.ButtonBase(c).Grow(1).OnClick(func() { d.push(i) }).Children(func() {
							ui.Row(c).Gap(10).Children(func() {
								ui.Text(c, label).Grow(1).FontWeight(600).SingleLine()
								ui.Text(c, store.String(x["type"])).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
								glyph(c, "chevron").FontSize(18)
							})
						})
					case []any:
						ui.Button(c, fmt.Sprintf("%d %s", len(x), a.t("items"))).Grow(1).OnClick(func() { d.push(i) })
					default:
						text := store.String(v)
						if ui.TextInput(c, &text).Grow(1).Label(fmt.Sprintf("%s %d", d.Title, i+1)).Changed() {
							value[i] = text
						}
						if d.Purpose == "global:rules" || strings.HasSuffix(d.Purpose, ":rules") {
							a.button(c, "edit", a.t("edit")+strconv.Itoa(i), func() { d.editRule(i, text) })
						}
					}
					a.button(c, "trash", a.t("delete")+strconv.Itoa(i), func() { d.replace(append(value[:i], value[i+1:]...)) }).Padding(6).Background(ui.Transparent)
				})
			}).Grow(1)
		default:
			ui.Text(c, a.t("noData")).TextColor(a.colors.muted)
			ui.Spacer(c)
		}
		ui.Button(c, "+ "+a.t("add")).FillWidth().Radius(999).Padding(12).OnClick(a.documentAdd)
	})
	a.documentDialogs(c)
}
func (a *App) collectionBreadcrumb() string {
	d := a.document
	parts := []string{d.Title}
	for _, p := range d.Path {
		if i, ok := p.(int); ok {
			parts = append(parts, strconv.Itoa(i+1))
		} else {
			parts = append(parts, humanKey(fmt.Sprint(p)))
		}
	}
	return strings.Join(parts, "  ›  ")
}
func (a *App) documentRow(c *ui.Context, key string, value any, set func(any), open, remove func()) {
	label := humanize(key)
	a.row(c, "", label, func() {
		switch v := value.(type) {
		case bool:
			if a.switchControl(c.Key(key), &v, label).Changed() {
				set(v)
			}
		case int:
			number := float64(v)
			if ui.NumberInput(c.Key(key), &number, -2147483648, 2147483647, 1).Width(170).Label(label).Changed() {
				set(int(number))
			}
		case float64:
			if ui.NumberInput(c.Key(key), &v, -2147483648, 2147483647, 1).Width(170).Label(label).Changed() {
				set(v)
			}
		case []any:
			ui.Button(c, fmt.Sprintf("%d  ›", len(v))).Radius(999).OnClick(open)
		case map[string]any:
			ui.Button(c, fmt.Sprintf("%d  ›", len(v))).Radius(999).OnClick(open)
		default:
			text := store.String(value)
			if options := documentOptions(key); len(options) > 0 {
				if ui.Select(c.Key(key), &text, options).Width(280).Label(label).Changed() {
					set(text)
				}
			} else {
				input := ui.TextInput(c.Key(key), &text).Label(label).Width(360)
				if key == "password" || key == "private-key" || key == "token" {
					input.Password()
				}
				if input.Changed() {
					set(text)
				}
			}
		}
		a.button(c, "close", a.t("delete")+" "+label, remove).Padding(4).Background(ui.Transparent)
	})
}
func documentOptions(key string) []string {
	switch key {
	case "type":
		return []string{"ss", "ssr", "vmess", "vless", "trojan", "hysteria", "hysteria2", "tuic", "wireguard", "socks5", "http", "anytls", "mieru", "direct", "reject", "select", "url-test", "fallback", "load-balance", "relay"}
	case "network":
		return []string{"tcp", "ws", "grpc", "h2", "http"}
	case "udp-over-tcp-version":
		return []string{"1", "2"}
	case "behavior":
		return []string{"domain", "ipcidr", "classical"}
	case "format":
		return []string{"yaml", "text", "mrs"}
	case "stack":
		return []string{"system", "gvisor", "mixed"}
	case "strategy":
		return []string{"consistent-hashing", "round-robin", "sticky-sessions"}
	}
	return nil
}
func (a *App) documentAdd() {
	d := a.document
	switch list := d.current().(type) {
	case []any:
		section := strings.TrimPrefix(strings.TrimPrefix(d.Purpose, "global:"), "section:")
		var value any = ""
		if len(d.Path) == 0 {
			switch section {
			case "proxies":
				value = map[string]any{"name": "Proxy", "type": "ss", "server": "", "port": 443, "cipher": "aes-128-gcm", "password": ""}
			case "proxy-groups":
				value = map[string]any{"name": "Proxy", "type": "select", "proxies": []any{"DIRECT"}}
			case "rules":
				value = "DOMAIN,example.com,DIRECT"
			}
		}
		if len(list) > 0 {
			if _, ok := list[0].(map[string]any); ok {
				if _, isMap := value.(map[string]any); !isMap {
					value = map[string]any{}
				}
			}
		}
		d.replace(append(list, value))
		if _, ok := value.(map[string]any); ok {
			d.push(len(list))
		}
	default:
		d.NewKey = ""
		d.NewKind = "text"
		d.AddOpen = true
	}
}
func (a *App) documentDialogs(c *ui.Context) {
	d := a.document
	a.dialogBase(c.Key("document-add"), &d.AddOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.3))
		panel.Width(450).Radius(34).Background(a.colors.card).Padding(28).Gap(18)
		ui.Text(c, a.t("add")).FontSize(fontSize(c, 22)).Bold()
		ui.TextInput(c, &d.NewKey).Label(a.t("name")).Placeholder(a.t("name")).AutoFocus()
		ui.Select(c, &d.NewKind, []string{"text", "number", "switch", "list", "object"}).Label(a.t("type"))
		ui.Row(c).Gap(12).Justify(ui.End).Children(func() {
			ui.Button(c, a.t("cancel")).Radius(999).OnClick(func() { d.AddOpen = false })
			ui.PrimaryButton(c, a.t("add")).Radius(999).Disabled(strings.TrimSpace(d.NewKey) == "").OnClick(func() {
				m, ok := d.current().(map[string]any)
				if !ok {
					m = map[string]any{}
					d.replace(m)
				}
				if _, exists := m[d.NewKey]; exists {
					a.err = a.t("alreadyExists")
					return
				}
				var value any = ""
				switch d.NewKind {
				case "number":
					value = 0
				case "switch":
					value = false
				case "list":
					value = []any{}
				case "object":
					value = map[string]any{}
				}
				m[d.NewKey] = value
				d.AddOpen = false
			})
		})
	})
	a.dialogBase(c.Key("rule-edit"), &d.RuleOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.3))
		panel.Width(560).Radius(34).Background(a.colors.card).Padding(24).Gap(14)
		ui.Text(c, a.t("rules")).FontSize(fontSize(c, 22)).Bold()
		ui.Select(c, &d.RuleType, []string{"DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD", "DOMAIN-WILDCARD", "DOMAIN-REGEX", "GEOSITE", "GEOIP", "IP-CIDR", "IP-CIDR6", "IP-ASN", "SRC-IP-CIDR", "SRC-PORT", "DST-PORT", "PROCESS-NAME", "PROCESS-PATH", "PROCESS-NAME-REGEX", "PROCESS-PATH-REGEX", "NETWORK", "IN-TYPE", "IN-NAME", "IN-PORT", "RULE-SET", "MATCH"}).Label(a.t("type"))
		if d.RuleType != "MATCH" {
			ui.TextInput(c, &d.RuleValue).Label(a.t("value")).Placeholder(a.t("value"))
		}
		ui.TextInput(c, &d.RuleTarget).Label(a.t("target")).Placeholder(a.t("target"))
		ui.Checkbox(c, &d.RuleNoResolve, "no-resolve")
		ui.Row(c).Gap(12).Justify(ui.End).Children(func() {
			ui.Button(c, a.t("cancel")).Radius(999).OnClick(func() { d.RuleOpen = false })
			ui.PrimaryButton(c, a.t("save")).Radius(999).OnClick(func() {
				parts := []string{d.RuleType}
				if d.RuleType != "MATCH" {
					parts = append(parts, d.RuleValue)
				}
				parts = append(parts, d.RuleTarget)
				if d.RuleNoResolve {
					parts = append(parts, "no-resolve")
				}
				list, ok := d.current().([]any)
				if ok && d.RuleIndex >= 0 && d.RuleIndex < len(list) {
					list[d.RuleIndex] = strings.Join(parts, ",")
				}
				d.RuleOpen = false
			})
		})
	})
}
func (d *documentEditor) editRule(index int, text string) {
	parts := strings.Split(text, ",")
	if len(parts) < 2 {
		return
	}
	d.RuleIndex = index
	d.RuleType = parts[0]
	d.RuleValue = ""
	d.RuleTarget = parts[len(parts)-1]
	d.RuleNoResolve = d.RuleTarget == "no-resolve"
	if d.RuleNoResolve && len(parts) > 2 {
		d.RuleTarget = parts[len(parts)-2]
	}
	if d.RuleType != "MATCH" && len(parts) > 2 {
		d.RuleValue = strings.Join(parts[1:len(parts)-1], ",")
		if d.RuleNoResolve {
			d.RuleValue = strings.Join(parts[1:len(parts)-2], ",")
		}
	}
	d.RuleOpen = true
}
func (a *App) saveDocument() {
	d := a.document
	if d == nil {
		return
	}
	purpose, value := d.Purpose, d.Value
	if purpose == "library:proxies" {
		list, ok := value.([]any)
		if !ok {
			a.err = "Proxies must be a list"
			return
		}
		proxies := []map[string]any{}
		names := map[string]bool{}
		for _, v := range list {
			p, ok := v.(map[string]any)
			name := store.String(p["name"])
			if !ok || name == "" || names[name] {
				a.err = "Each proxy must have a unique name"
				return
			}
			names[name] = true
			proxies = append(proxies, p)
		}
		a.cfg.LocalProxies = proxies
		a.apply()
		d.Path = nil
		a.back()
		return
	}
	if strings.HasPrefix(purpose, "global:") {
		setPath(a.cfg.Overrides, strings.TrimPrefix(purpose, "global:"), value)
		a.apply()
		d.Path = nil
		a.back()
		return
	}
	if purpose == "profile-override" {
		text, err := store.YAML(value)
		if err != nil {
			a.err = err.Error()
			return
		}
		a.profileDraft.Override = text
		a.persistProfileDraft()
		d.Path = nil
		a.back()
		return
	}
	id, s, e := a.editProfile, a.store, a.engine
	a.work(func() (func(), error) {
		b, err := os.ReadFile(s.ProfilePath(id))
		if err != nil {
			return nil, err
		}
		patched, err := store.PatchSection(b, strings.TrimPrefix(purpose, "section:"), value)
		if err != nil {
			return nil, err
		}
		if e != nil {
			if err = e.Validate(patched); err != nil {
				return nil, err
			}
		}
		if err = store.AtomicWrite(s.ProfilePath(id), patched); err != nil {
			return nil, err
		}
		return func() { a.apply(); d.Path = nil; a.back() }, nil
	})
}
func (a *App) persistProfileDraft() {
	for i := range a.cfg.Profiles {
		if a.cfg.Profiles[i].ID == a.editProfile {
			a.cfg.Profiles[i] = a.profileDraft
			a.apply()
			return
		}
	}
}
