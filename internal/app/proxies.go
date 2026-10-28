package app

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/dlclark/regexp2/v2"
	"github.com/egoist/mygo/ui"
)

type proxyFilterState struct {
	pattern  string
	compiled *regexp2.Regexp
	err      error
}

func (a *App) matchesProxyFilter(name string) bool {
	pattern := ""
	for _, filter := range a.cfg.Filters {
		if filter.Label == a.proxyFilter {
			pattern = filter.Regex
			break
		}
	}
	if pattern == "" {
		return true
	}
	if a.filterState == nil || a.filterState.pattern != pattern {
		compiled, err := regexp2.Compile(pattern, regexp2.None)
		if compiled != nil {
			compiled.MatchTimeout = 100 * time.Millisecond
		}
		a.filterState = &proxyFilterState{pattern: pattern, compiled: compiled, err: err}
	}
	if a.filterState.err != nil {
		return false
	}
	matched, err := a.filterState.compiled.MatchString(name)
	return err == nil && matched
}

func (a *App) proxiesView(c *ui.Context) {
	groups := a.visibleProxyGroups()
	if len(groups) == 0 && len(a.sample.Providers) == 0 {
		a.empty(c, "proxies", a.t("proxies"), a.t("nullProfileDesc"), func() { a.addOpen = true })
		return
	}
	top, bottom := float32(16), float32(16)
	if a.sheetContentHeight > 0 {
		top = 12
	}
	if a.compactSheet {
		top = 0
	}
	if a.compactSheet && a.picker.Open && a.picker.Kind == "proxies" {
		bottom += 64
	}
	ui.Column(c).Fill().Padding(top, 16, bottom, 16).Gap(12).Children(func() {
		width, _ := a.viewSize(c)
		side := a.sidebarSize()
		room := width - side - 32
		if a.picker.Open && a.picker.Kind == "proxies" {
			room = a.sideSheetWidth(c) - 32
		}
		columns := proxyColumns(room, a.cfg.ProxyLayout, a.cfg.ProxyColumns)
		if a.cfg.ProxyStyle == "tab" {
			a.proxyTabs(c, groups, columns, room)
		} else {
			a.proxyGroupList(c, groups, room)
		}
	})
}

func proxyColumns(width float32, layout string, manual int) int {
	if manual > 0 {
		return min(manual, 8)
	}
	columns := max(int(math.Ceil(float64(width)/250)), 2)
	switch layout {
	case "tight":
		columns++
	case "loose":
		columns--
	}
	return max(columns, 1)
}
func (a *App) proxyCardHeight() float32 {
	base := 28 + a.textHeight(20)*2 + a.textHeight(18)
	switch a.cfg.ProxyCard {
	case "loose":
		return base + a.textHeight(13) + 6
	case "compact":
		return base - a.textHeight(20)
	default:
		return base
	}
}
func (a *App) proxyHeaderHeight() float32 {
	return 24 + a.textHeight(21) + a.textHeight(16)
}
func searchMatches(query string, fields ...string) bool {
	text := strings.ToLower(strings.Join(fields, "\n"))
	for _, term := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}
func (a *App) visibleProxyGroups() []ProxyNode {
	groups := a.currentProxyGroups()
	filtered := groups[:0]
	for _, group := range groups {
		group.All = a.proxyNames(group)
		if strings.TrimSpace(a.proxyViewState().proxyQuery) != "" && len(group.All) == 0 {
			continue
		}
		filtered = append(filtered, group)
	}
	return filtered
}

// Global delay tests use the current mode's complete groups, not view filters.
func (a *App) currentProxyGroups() []ProxyNode {
	groups := []ProxyNode{}
	mode := store.String(a.cfg.Overrides["mode"])
	if mode == "direct" {
		return groups
	}
	for _, name := range a.sample.Proxies.All {
		group := a.sample.Proxies.Proxies[name]
		if mode != "global" && group.Hidden {
			continue
		}
		groups = append(groups, group)
	}
	return groups
}
func (a *App) proxyNames(group ProxyNode) []string {
	names := []string{}
	for _, name := range group.All {
		node := a.sample.Proxies.Proxies[name]
		if searchMatches(a.proxyViewState().proxyQuery, name, node.Type, node.Provider) && a.matchesProxyFilter(name) {
			names = append(names, name)
		}
	}

	if a.cfg.HideTimeout {
		visible := []string{}
		for _, name := range names {
			node := a.sample.Proxies.Proxies[name]
			delay, has := a.proxyDelayValue(group, name, true)
			unprobeable := slices.Contains([]string{"Reject", "RejectDrop", "Pass", "PassRule", "Rematch", "Compatible", "Dns"}, node.Type)
			if name == group.Now || len(node.All) > 0 || unprobeable || !has || delay > 0 {
				visible = append(visible, name)
			}
		}
		// Keep the matches and their test action if everything has timed out.
		if len(visible) > 0 {
			names = visible
		}
	}
	switch a.cfg.ProxySort {
	case "name":
		sort.SliceStable(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	case "delay":
		sort.SliceStable(names, func(i, j int) bool {
			di, _ := a.proxyDelayValue(group, names[i], true)
			dj, _ := a.proxyDelayValue(group, names[j], true)
			if di <= 0 {
				di = 1 << 30
			}
			if dj <= 0 {
				dj = 1 << 30
			}
			return di < dj
		})
	}
	return names
}
func (a *App) currentProxyProfile() *store.Profile { return a.store.ActiveProfile(a.cfg) }
func (a *App) groupUnfolded(name string) bool {
	if p := a.currentProxyProfile(); p != nil {
		return slices.Contains(p.Unfold, name)
	}
	return a.proxyExpanded[name]
}
func (a *App) toggleProxyGroup(name string) {
	if p := a.currentProxyProfile(); p != nil {
		if i := slices.Index(p.Unfold, name); i >= 0 {
			p.Unfold = slices.Delete(p.Unfold, i, i+1)
		} else {
			p.Unfold = append(p.Unfold, name)
		}
	} else {
		if a.proxyExpanded == nil {
			a.proxyExpanded = map[string]bool{}
		}
		a.proxyExpanded[name] = !a.proxyExpanded[name]
	}
	a.save()
}
func (a *App) selectProxyGroup(name string) {
	a.group = name
	if p := a.currentProxyProfile(); p != nil {
		p.Group = name
	}
	a.save()
}
func (a *App) locateProxy(name string) {
	a.selectProxyGroup(name)
	if a.cfg.ProxyStyle != "tab" && !a.groupUnfolded(name) {
		a.toggleProxyGroup(name)
	}
	a.proxyViewState().proxyLocate = name
}
func (a *App) proxyTabs(c *ui.Context, groups []ProxyNode, columns int, width float32) {
	if len(groups) == 0 {
		a.empty(c, "proxies", a.t("noData"), "", nil)
		return
	}
	selected := 0
	for i, g := range groups {
		if g.Name == a.group {
			selected = i
		}
	}
	group := groups[selected]
	ui.ScrollHorizontal(c).FillWidth().Height(44).Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			for _, g := range groups {
				b := ui.ButtonBase(c.Key(g.Name)).Label(g.Name).Padding(10, 16).Radius(999).Background(a.colors.block).OnClick(func() { a.selectProxyGroup(g.Name) })
				if g.Name == group.Name {
					b.Background(a.colors.accent).TextColor(a.colors.onAccent)
				}
				b.Children(func() { ui.Text(c, g.Name).FontWeight(600).FontSize(fontSize(c, 14)).SingleLine() })
			}
		})
	})
	if a.proxyViewState().proxyGrids == nil {
		a.proxyViewState().proxyGrids = map[string]*ui.GridState{}
	}
	key := a.cfg.Active + "/" + group.Name
	state := a.proxyViewState().proxyGrids[key]
	if state == nil {
		state = &ui.GridState{}
		a.proxyViewState().proxyGrids[key] = state
	}
	state.Key = func(i int) any { return group.All[i] }
	state.Label = func(i int) string { return group.All[i] }
	if a.proxyViewState().proxyLocate == group.Name {
		if i := slices.Index(group.All, group.Now); i >= 0 {
			state.ScrollIntoView(i)
		}
		a.proxyViewState().proxyLocate = ""
	}
	ui.GridView(c.Key(key), state, len(group.All), max((width-8*float32(columns-1))/float32(columns), 1), a.proxyCardHeight(), func(i int) { a.proxyCard(c, group, group.All[i]) }).Grow(1).Gap(8)
}

type proxyGroupLayout struct {
	group      ProxyNode
	x, y, w, h float32
	columns    int
}

func (a *App) proxyGroupList(c *ui.Context, groups []ProxyNode, width float32) {
	groupColumns := max(1, min(3, int((width+16)/(360+16))))
	groupWidth := (width - 16*float32(groupColumns-1)) / float32(groupColumns)
	columns := proxyColumns(groupWidth-20, a.cfg.ProxyLayout, a.cfg.ProxyColumns)
	heights := make([]float32, groupColumns)
	layouts := make([]proxyGroupLayout, 0, len(groups))
	headerHeight := a.proxyHeaderHeight()
	rowHeight := a.proxyCardHeight() + 8
	total := float32(0)
	for i, g := range groups {
		column := i % groupColumns
		y := heights[column]
		height := headerHeight
		if a.groupUnfolded(g.Name) {
			height += float32((len(g.All)+columns-1)/columns)*rowHeight + 2
		}
		layouts = append(layouts, proxyGroupLayout{group: g, x: float32(column) * (groupWidth + 16), y: y, w: groupWidth, h: height, columns: columns})
		heights[column] = y + height + 16
		total = max(total, y+height)
		if a.proxyViewState().proxyLocate == g.Name {
			if index := slices.Index(g.All, g.Now); index >= 0 {
				a.proxyViewState().proxyScroll.Y = y + float32(index/columns)*rowHeight
			}
		}
	}
	a.proxyViewState().proxyLocate = ""
	_, height := a.viewSize(c)
	viewport := max(height-100, 200)
	if a.sheetContentHeight > 0 {
		viewport = max(0, a.sheetContentHeight-72)
	}
	total += float32(len(a.sample.Providers)) * 94
	ui.Scroll(c).TrackScroll(&a.proxyViewState().proxyScroll).Fill().Children(func() {
		ui.Box(c).FillWidth().Height(max(total, viewport)).Children(func() {
			for _, layout := range layouts {
				if layout.y+layout.h < a.proxyViewState().proxyScroll.Y-150 || layout.y > a.proxyViewState().proxyScroll.Y+viewport+150 {
					continue
				}
				g := layout.group
				ui.Box(c.Key(g.Name)).Absolute().Left(layout.x).Top(layout.y).Width(layout.w).Height(layout.h).Radius(24).Background(a.colors.card).Clip().Children(func() {
					if a.groupUnfolded(g.Name) {
						rowCount := (len(g.All) + columns - 1) / columns
						first := max(int((a.proxyViewState().proxyScroll.Y-layout.y-headerHeight)/rowHeight)-1, 0)
						last := min(rowCount, int((a.proxyViewState().proxyScroll.Y+viewport-layout.y-headerHeight)/rowHeight)+3)
						for row := first; row < last; row++ {
							ui.Row(c.Key(row)).Absolute().Left(10).Right(10).Top(headerHeight + float32(row)*rowHeight).Height(a.proxyCardHeight()).Gap(8).Children(func() {
								for i := row * columns; i < min((row+1)*columns, len(g.All)); i++ {
									name := g.All[i]
									ui.Box(c.Key(name)).Grow(1).MinWidth(0).FillHeight().Children(func() { a.proxyCard(c, g, name) })
								}
								for i := min((row+1)*columns, len(g.All)); i < (row+1)*columns; i++ {
									ui.Spacer(c).Grow(1)
								}
							})
						}
					}
					pinned := min(max(a.proxyViewState().proxyScroll.Y-layout.y, 0), max(layout.h-headerHeight, 0))
					ui.Box(c.Key("header")).Absolute().Left(0).Right(0).Top(pinned).Height(headerHeight).Background(a.colors.card).Radius(24).Children(func() { a.proxyGroupHeader(c, g) })
				})
			}
			y := float32(0)
			for _, h := range heights {
				y = max(y, h)
			}
			for i, p := range a.sample.Providers {
				ui.Box(c.Key(p.Name)).Absolute().Top(y + float32(i)*94).Left(0).Right(0).Height(86).Children(func() { a.proxyProviderRow(c, p) })
			}
		})
	})
}
func (a *App) proxyGroupHeader(c *ui.Context, g ProxyNode) {
	ui.Row(c).FillWidth().Height(a.proxyHeaderHeight()).Padding(10).Gap(8).Children(func() {
		ui.ButtonBase(c.Key(g.Name)).Label(g.Name).Grow(1).MinWidth(0).OnClick(func() { a.toggleProxyGroup(g.Name) }).Children(func() {
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				if a.cfg.ProxyIcon != "hidden" {
					side := a.proxyHeaderHeight() - 20
					icon := ui.Box(c).Size(side, side).Radius(24*(1+(a.cfg.Scale-1)/2) - 10).Center()
					if a.cfg.ProxyIcon != "plain" {
						icon.Background(a.colors.block)
					}
					icon.Children(func() {
						inset, color := 12*(1+(a.cfg.Scale-1)/2), a.colors.muted
						if a.cfg.ProxyIcon == "plain" {
							inset, color = 0, a.colors.accent
						}
						a.targetIcon(c, g.Icon, g.Name, side-inset, color)
					})
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, g.Name).FixedLineHeight(a.textHeight(21)).FontSize(fontSize(c, 16)).FontWeight(600).SingleLine()
					ui.Text(c, g.Now).FixedLineHeight(a.textHeight(16)).FontWeight(600).FontSize(fontSize(c, 12)).TextColor(a.colors.muted).SingleLine()
				})
			})
		})
		ui.Row(c).Gap(6).Children(func() {
			if a.groupUnfolded(g.Name) {
				ui.Row(c).Gap(0).Radius(999).Background(a.colors.block).Children(func() {
					a.proxyGroupAction(c, "locate", a.t("scrollToSelected")+" "+g.Name, func() { a.locateProxy(g.Name) })
					a.proxyGroupAction(c, "bolt", a.t("delayTest")+" "+g.Name, func() { a.testPageGroup(g.Name) }, a.delayGroups[g.Name] != 0)
				})
			} else {
				ui.Text(c, g.Type).FontSize(fontSize(c, 12)).FontWeight(600).TextColor(a.colors.muted)
			}
			icon := "triangle-down"
			if a.groupUnfolded(g.Name) {
				icon = "triangle-up"
			}
			label := a.t("showMore")
			if a.groupUnfolded(g.Name) {
				label = a.t("showLess")
			}
			a.proxyGroupAction(c, icon, label+" "+g.Name, func() { a.toggleProxyGroup(g.Name) })
		})
	})
}
func (a *App) proxyCard(c *ui.Context, group ProxyNode, name string) {
	node := a.sample.Proxies.Proxies[name]
	selected := group.Now == name
	background := a.colors.card
	if a.cfg.ProxyStyle != "tab" {
		background = a.colors.block
	}
	b := ui.ButtonBase(c.Key(name)).Label(name).Fill().Radius(20).Padding(0, 12).Background(background).OnClick(func() { a.selectProxy(group.Name, name) })

	b.Children(func() {
		ui.Column(c).Fill().Gap(8).Justify(ui.Center).Children(func() {
			ui.Row(c).Height(func() float32 {
				if a.cfg.ProxyCard == "compact" {
					return a.textHeight(20)
				}
				return a.textHeight(20) * 2
			}()).AlignItems(ui.Start).Gap(4).Children(func() {
				title := ui.Text(c, name).Grow(1).MinWidth(0).FixedLineHeight(a.textHeight(20)).FontSize(fontSize(c, 15)).FontWeight(400)
				if a.cfg.ProxyCard == "compact" {
					title.SingleLine()
				} else {
					title.MaxLines(2)
				}
				if group.Fixed == name || computedGroup(group.Type) && a.cfg.Selected[group.Name] == name || selected {
					ui.Box(c).Size(22, 22).Radius(999).Background(a.colors.accent).Center().Children(func() {
						icon := "check"
						if computedGroup(group.Type) {
							icon = "pin"
						}
						glyph(c, icon).FontSize(14).TextColor(a.colors.onAccent)
					})
				}
			})
			detail := func() {
				text := node.Type
				if node.Now != "" {
					text += "(" + node.Now + ")"
				}
				ui.Text(c, text).FixedLineHeight(a.textHeight(18)).FontSize(fontSize(c, 13)).TextColor(a.colors.muted).Grow(1).SingleLine()
			}
			if a.cfg.ProxyCard == "loose" {
				detail()
				a.proxyDelay(c, group, name)
			} else {
				ui.Row(c).Gap(8).Children(func() { detail(); a.proxyDelay(c, group, name) })
			}
		})
	})
	b.ContextMenu(func(m *ui.Menu) {
		label := a.t("select")
		if computedGroup(group.Type) {
			label = a.t("preferred")
		}
		if m.Item(label).Chosen() {
			a.selectProxy(group.Name, name)
		}
		if m.Item(a.t("testDelay")).Chosen() {
			a.testNames([]string{name}, group.TestURL)
		}
		if m.Item(a.t("copy")).Chosen() {
			c.Services().WriteClipboard(name)
		}
		if m.Item(a.t("details")).Chosen() {
			a.proxyDefinition(name)
		}
		if len(node.All) == 0 && !slices.Contains([]string{"DIRECT", "REJECT", "PASS", "COMPATIBLE"}, name) {
			if m.Item(a.t("addToLocalProxies")).Chosen() {
				a.addProxyToLocal(name)
			}
		}
	})
}
func (a *App) proxyDelay(c *ui.Context, group ProxyNode, name string) {
	value, ok := a.proxyDelayValue(group, name, false)
	target := a.resolveDelayTarget(name, group.TestURL)
	pending := a.delayPending[target.key()]
	side := a.textHeight(13)
	if pending.holds > 0 {
		ui.Box(c.Key("delay")).Label(a.t("testDelay")+" "+name).Size(side, side).Center().Children(func() {
			if pending.runs > 0 {
				loading(c, side, a.colors.accent, a.t("loading"))
			} else {
				glyph(c, "clock").FontSize(side).TextColor(a.colors.muted.Alpha(0.38))
			}
		})
		return
	}
	button := ui.ButtonBase(c.Key("delay")).Label(a.t("testDelay") + " " + name).Tooltip(a.t("testDelay")).Height(side).MinWidth(side).Center().OnClick(func() { a.testNames([]string{name}, group.TestURL) })
	if !ok {
		button.Width(side).Children(func() { glyph(c, "bolt").FontSize(side).TextColor(a.colors.muted) })
		return
	}
	color, label := a.colors.success, fmt.Sprintf("%d ms", value)
	if value < 0 {
		color, label = a.colors.danger, a.t("delayFailed")
		if value == -1 {
			label = a.t("timeout")
		}
	} else if value == 0 {
		color, label = a.colors.muted, "…"
	} else if value >= 600 {
		color = a.colors.warning
	}
	button.Children(func() {
		ui.Text(c, label).FixedLineHeight(side).FontSize(fontSize(c, 11)).FontWeight(600).TextColor(color).SingleLine()
	})
}
func computedGroup(kind string) bool { return kind == "URLTest" || kind == "Fallback" }
func (a *App) addProxyToLocal(name string) {
	if a.engine == nil || a.safe {
		return
	}
	e := a.engine
	a.work(func() (func(), error) {
		var definition map[string]any
		if err := e.Call("getProxyDefinition", name, &definition); err != nil {
			return nil, err
		}
		return func() {
			replaced := false
			for i, p := range a.cfg.LocalProxies {
				if store.String(p["name"]) == name {
					a.cfg.LocalProxies[i] = definition
					replaced = true
					break
				}
			}
			if !replaced {
				a.cfg.LocalProxies = append(a.cfg.LocalProxies, definition)
			}
			a.save()
			a.message = a.t("success")
		}, nil
	})
}
func (a *App) proxyProviderRow(c *ui.Context, p Provider) {
	a.card(c).FillWidth().Padding(12, 16).Children(func() {
		ui.Row(c).Gap(12).Children(func() {
			glyph(c, "cloud").FontSize(22).TextColor(a.colors.muted)
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, p.Name).FontSize(fontSize(c, 15)).Bold().SingleLine()
				ui.Textf(c, "%s · %s · %d", p.Type, p.Vehicle, p.Count).FontSize(fontSize(c, 12)).TextColor(a.colors.muted)
			})
			if strings.EqualFold(p.Vehicle, "HTTP") {
				a.button(c, "refresh", a.t("update")+" "+p.Name, func() { a.updateDetailProvider(p) })
			}
			a.button(c, "edit", a.t("edit")+" "+p.Name, func() { a.editProvider(p) }).Disabled(a.safe)
			a.button(c, "info", a.t("details")+" "+p.Name, func() { a.providerInfo(p) })
			a.button(c, "down", a.t("import")+" "+p.Name, func() { a.importProvider(p) }).Disabled(a.safe).ContextMenu(func(m *ui.Menu) {
				if m.Item(a.t("export")).Chosen() {
					a.exportProvider(p)
				}
			})
		})
	})
}
func (a *App) selectProxy(group, name string) {
	if a.engine == nil || a.safe {
		return
	}
	kind := a.sample.Proxies.Proxies[group].Type
	if kind != "Selector" && !computedGroup(kind) {
		a.message = a.t("notSelectedTip")
		return
	}
	if computedGroup(kind) && a.cfg.Selected[group] == name {
		name = ""
	}
	active := a.cfg.Active
	e := a.engine
	a.work(func() (func(), error) {
		var result struct {
			Message string `json:"message"`
			Changed bool   `json:"changed"`
		}
		if err := e.Call("changeProxy", map[string]string{"group-name": group, "proxy-name": name}, &result); err != nil {
			return nil, err
		}
		if result.Message != "" {
			return nil, errors.New(result.Message)
		}
		return func() {
			if a.cfg.Active == active {
				a.cfg.Selected[group] = name
			}
			for i := range a.cfg.Profiles {
				if a.cfg.Profiles[i].ID == active {
					if a.cfg.Profiles[i].Selected == nil {
						a.cfg.Profiles[i].Selected = map[string]string{}
					}
					a.cfg.Profiles[i].Selected[group] = name
				}
			}
			if a.cfg.Active != active {
				a.save()
				return
			}
			g := a.sample.Proxies.Proxies[group]
			if computedGroup(g.Type) {
				g.Fixed = name
			} else {
				g.Now = name
			}
			a.sample.Proxies.Proxies[group] = g
			a.save()
			if result.Changed && a.cfg.CloseConnections {
				a.coreAction("closeConnections", nil)
			}
			a.outboundIP()
		}, nil
	})
}
func (a *App) proxyDefinition(name string) {
	if a.engine == nil {
		return
	}
	e := a.engine
	a.work(func() (func(), error) {
		var value map[string]any
		err := e.Call("getProxyDefinition", name, &value)
		return func() { a.showJSON(name, value) }, err
	})
}

func delayKey(name, url string) string { return url + "\x00" + name }
func (a *App) setProxyDelay(name, url string, value int) {
	if url == "" {
		url = a.cfg.TestURL
	}
	a.delays[delayKey(name, url)] = value
	if url == a.cfg.TestURL {
		a.delays[name] = value
	}
}
func (a *App) proxyDelayValue(group ProxyNode, name string, completed bool) (int, bool) {
	target := a.resolveDelayTarget(name, group.TestURL)
	name, url := target.name, target.url
	delays := a.delays
	if completed && a.delaySnapshot != nil {
		delays = a.delaySnapshot
	}
	if value, ok := delays[delayKey(name, url)]; ok {
		return value, true
	}
	if url == a.cfg.TestURL {
		value, ok := delays[name]
		return value, ok
	}
	return 0, false
}

func (a *App) proxyMenu(m *ui.Menu) {
	for _, entry := range []struct {
		label   string
		value   *string
		options [][2]string
	}{
		{"style", &a.cfg.ProxyStyle, [][2]string{{"list", "list"}, {"tab", "tab"}}},
		{"sort", &a.cfg.ProxySort, [][2]string{{"default", "defaultText"}, {"delay", "delay"}, {"name", "name"}}},
		{"layout", &a.cfg.ProxyLayout, [][2]string{{"loose", "loose"}, {"standard", "standard"}, {"tight", "tight"}}},
		{"size", &a.cfg.ProxyCard, [][2]string{{"loose", "loose"}, {"standard", "standard"}, {"compact", "compact"}}},
		{"iconStyle", &a.cfg.ProxyIcon, [][2]string{{"filled", "iconStyleFilled"}, {"plain", "iconStylePlain"}, {"hidden", "iconStyleHidden"}}},
	} {
		m.Submenu(a.t(entry.label), func(sub *ui.Menu) {
			for _, option := range entry.options {
				if sub.Item(a.t(option[1])).Checked(*entry.value == option[0]).Chosen() {
					*entry.value = option[0]
					a.save()
				}
			}
		})
	}
	m.Separator()
	if m.Item(a.t("hideTimeoutProxies")).Checked(a.cfg.HideTimeout).Chosen() {
		a.cfg.HideTimeout = !a.cfg.HideTimeout
		a.save()
	}
	m.Submenu(a.t("filter"), func(sub *ui.Menu) {
		if sub.Item(a.t("all")).Checked(a.proxyFilter == "").Chosen() {
			a.proxyFilter = ""
		}
		for _, filter := range a.cfg.Filters {
			if sub.Item(filter.Label).Checked(a.proxyFilter == filter.Label).Chosen() {
				a.proxyFilter = filter.Label
			}
		}
	})
}

func (a *App) proxyGroupAction(c *ui.Context, icon, label string, action func(), busy ...bool) {
	key := icon
	if icon == "triangle-up" || icon == "triangle-down" {
		key = "expand"
	}
	loadingNow := len(busy) > 0 && busy[0]
	if loadingNow {
		a.delayBusyButton(c, key, label, 32)
		return
	}
	button := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Size(32, 32).Radius(999).Background(a.colors.block).Center().OnClick(action)
	button.Children(func() {
		if icon == "triangle-up" || icon == "triangle-down" {
			target := float32(0)
			if icon == "triangle-up" {
				target = 180
			}
			glyphVariant(c, "caretDown", true).FontSize(20).TextColor(a.colors.text).Rotate(button.AnimateWith("expand", target, 200*time.Millisecond, ui.CubicBezier(0.4, 0, 0.2, 1)))
		} else {
			size := float32(20)
			if icon == "locate" {
				size = 19
			}
			glyphVariant(c, icon, true).FontSize(size).TextColor(a.colors.text)
		}
	})
}
