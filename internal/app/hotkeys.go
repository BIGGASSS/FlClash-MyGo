package app

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var hotkeyBindings = []struct{ section, action, label, icon string }{
	{"general", "view", "actionView", "eye"}, {"general", "start", "actionStart", "play"}, {"general", "exit", "exit", "close"},
	{"outboundMode", "mode", "actionMode", "split"}, {"outboundMode", "ruleMode", "actionRuleMode", "rules"}, {"outboundMode", "globalMode", "actionGlobalMode", "language"}, {"outboundMode", "directMode", "actionDirectMode", "target"},
	{"network", "proxy", "actionProxy", "shuffle"}, {"network", "tun", "actionTun", "vpn"}, {"network", "copyEnv", "copyEnvVar", "code"},
	{"proxies", "delayTest", "actionDelayTest", "bolt"}, {"proxies", "updateProfiles", "actionUpdateProfiles", "sync"},
}

func (a *App) hotkeyLabel(action string) string {
	for _, binding := range hotkeyBindings {
		if binding.action == action {
			return a.t(binding.label)
		}
	}
	return action
}

func (a *App) performHotkey(action string) {
	switch action {
	case "start":
		a.toggle()
	case "view":
		if a.window != nil {
			if a.window.IsVisible() {
				a.window.Hide()
			} else {
				a.window.Show()
				a.window.Focus()
			}
		}
	case "mode":
		mode := map[string]string{"rule": "global", "global": "direct", "direct": "rule"}[store.String(a.cfg.Overrides["mode"])]
		if mode == "" {
			mode = "rule"
		}
		a.cfg.Overrides["mode"] = mode
		a.apply()
	case "ruleMode", "globalMode", "directMode":
		a.cfg.Overrides["mode"] = strings.TrimSuffix(action, "Mode")
		a.apply()
	case "proxy":
		a.cfg.SystemProxy = !a.cfg.SystemProxy
		a.apply()
	case "tun":
		tun := store.Map(a.cfg.Overrides, "tun")
		tun["enable"] = !store.Bool(tun["enable"])
		a.apply()
	case "delayTest":
		a.testCurrentGroups()
	case "updateProfiles":
		a.updateAllProfiles()
	case "copyEnv":
		mygo.Clipboard.WriteText(proxyEnvCommand(store.Int(a.cfg.Overrides["mixed-port"]), runtime.GOOS == "windows"))
	case "exit":
		mygo.App.Quit()
	}
}

func proxyEnvCommand(port int, windows bool) string {
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	if windows {
		return "set $env:all_proxy=" + url
	}
	return "export all_proxy=" + url
}

func validHotkey(chord string) bool {
	parts := strings.Split(store.CanonicalHotkey(chord), "+")
	if len(parts) < 2 || parts[len(parts)-1] == "" {
		return false
	}
	for _, part := range parts[:len(parts)-1] {
		if part == "Ctrl" || part == "Alt" || part == "Shift" || part == "Super" {
			return true
		}
	}
	return false
}

func (a *App) hotkeyConflict() string {
	if a.hotkeyDraft == "" {
		return ""
	}
	for _, binding := range hotkeyBindings {
		if binding.action != a.hotkeyAction && store.CanonicalHotkey(a.cfg.Hotkeys[binding.action]) == store.CanonicalHotkey(a.hotkeyDraft) {
			return binding.action
		}
	}
	return ""
}

func (a *App) saveHotkey() {
	for action, shortcut := range a.cfg.Hotkeys {
		if store.CanonicalHotkey(shortcut) == store.CanonicalHotkey(a.hotkeyDraft) {
			delete(a.cfg.Hotkeys, action)
		}
	}
	if a.hotkeyDraft != "" {
		a.cfg.Hotkeys[a.hotkeyAction] = a.hotkeyDraft
	} else {
		delete(a.cfg.Hotkeys, a.hotkeyAction)
	}
	a.hotkeyOpen = false
	a.save()
	// Also register new bindings when the first shortcut was recorded.
	a.hotkeysSuspended = false
	a.installHotkeys()
}

func (a *App) hotkeySettings(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(16, 16, 20, 16).Gap(24).Children(func() {
			a.groupedSection(c, "", func() {
				ui.Row(c).Padding(14, 16).Gap(14).Children(func() {
					glyph(c, "keyboard").FontSize(24).TextColor(a.colors.accent)
					ui.Text(c, a.t("hotkeyDesc")).Grow(1).FontSize(fontSize(c, 15)).TextColor(a.colors.muted)
				})
			})
			for _, section := range []string{"general", "outboundMode", "network", "proxies"} {
				a.groupedSection(c, a.t(section), func() {
					first := true
					for _, binding := range hotkeyBindings {
						if binding.section != section {
							continue
						}
						if !first {
							a.separator(c)
						}
						first = false
						ui.Row(c.Key(binding.action)).FillWidth().Padding(0, 6, 0, 0).Children(func() {
							ui.ButtonBase(c).Label(a.t(binding.label)).Grow(1).MinWidth(0).OnClick(func() { a.recordHotkey(binding.action) }).Children(func() {
								ui.Row(c).FillWidth().MinHeight(56).Padding(10, 16).Gap(14).Children(func() {
									glyph(c, binding.icon).FontSize(22).TextColor(a.colors.muted)
									ui.Column(c).Grow(1).Gap(3).Children(func() {
										ui.Text(c, a.t(binding.label)).FontSize(fontSize(c, 17))
										if failure := a.hotkeyErrors[binding.action]; failure != "" {
											ui.Text(c, a.t("hotkeyUnavailable")).FontSize(fontSize(c, 12)).TextColor(a.colors.danger).Tooltip(failure)
										}
									})
									if chord := a.cfg.Hotkeys[binding.action]; chord != "" {
										a.hotkeyCaps(c, chord, false, a.hotkeyErrors[binding.action] != "")
									} else {
										ui.Text(c, a.t("hotkeyNotSet")).FontSize(fontSize(c, 14)).TextColor(a.colors.outline)
									}
								})
							})
							if a.cfg.Hotkeys[binding.action] != "" {
								ui.ButtonBase(c.Key("remove")).Label(a.t("remove")).Tooltip(a.t("remove")).Width(36).Height(36).Radius(999).Center().OnClick(func() { delete(a.cfg.Hotkeys, binding.action); a.save(); a.installHotkeys() }).Children(func() { glyph(c, "close").FontSize(20) })
							}
						})
					}
				})
			}
		})
	})
}

func (a *App) hotkeyCaps(c *ui.Context, chord string, large, failed bool) {
	ui.Row(c).Gap(4).Children(func() {
		parts := strings.Split(chord, "+")
		for _, part := range parts {
			label := part
			if name := map[string]string{"SPACE": "Space", "ENTER": "Enter", "NUMENTER": "Num Enter", "ESCAPE": "Esc", "BACKSPACE": "Backspace", "DELETE": "Del", "INSERT": "Ins", "TAB": "Tab", "PAGEUP": "PgUp", "PAGEDOWN": "PgDn", "HOME": "Home", "END": "End", "UP": "↑", "DOWN": "↓", "LEFT": "←", "RIGHT": "→", "MEDIAPLAYPAUSE": "Play/Pause", "MEDIANEXTTRACK": "Next Track", "MEDIAPREVIOUSTRACK": "Previous Track", "MEDIASTOP": "Stop", "VOLUMEUP": "Volume Up", "VOLUMEDOWN": "Volume Down", "VOLUMEMUTE": "Mute", "PRINTSCREEN": "PrtSc", "CONTEXTMENU": "Menu"}[strings.ToUpper(part)]; name != "" {
				label = name
			}
			if runtime.GOOS == "darwin" {
				if symbol := map[string]string{"Ctrl": "⌃", "Alt": "⌥", "Shift": "⇧", "Super": "⌘", "Command": "⌘"}[part]; symbol != "" {
					label = symbol
				}
			}
			size, font := float32(26), float32(12)
			if large {
				size, font = 44, 22
			}
			bg, fg := a.colors.block, a.colors.muted
			if failed {
				bg, fg = a.colors.dangerContainer, a.colors.onDangerContainer
			}
			ui.Box(c).MinWidth(size).Height(size).Padding(0, 7).Radius(8).Background(bg).Center().Children(func() { ui.Text(c, label).FontSize(fontSize(c, font)).FontWeight(600).TextColor(fg) })
		}
	})
}

func (a *App) recordHotkey(action string) {
	a.hotkeyAction = action
	a.hotkeyDraft = ""
	a.hotkeyOpen = true
	if !a.safe && a.window != nil {
		mygo.GlobalShortcut.UnregisterAll()
		a.hotkeysSuspended = true
	}
}
func (a *App) hotkeyDialog(c *ui.Context) {
	a.dialogBase(c.Key("hotkey-record"), &a.hotkeyOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.3))
		panel.Width(500).Padding(28).Gap(20).Radius(34).Background(a.colors.card)
		ui.Text(c, a.hotkeyLabel(a.hotkeyAction)).FontSize(fontSize(c, 22)).Bold()
		ui.ButtonBase(c.Key("recording")).Label(a.t("pressKeyboard")).FillWidth().Height(96).Radius(18).Background(a.colors.block).Center().AutoFocus().HandleInput(func(event ui.InputEvent) bool {
			if event.Kind != ui.InputKeyDown || (event.Key == ui.KeyEscape && event.Mods == 0) {
				return false
			}
			key := event.PhysicalKey
			if key == ui.KeyUnknown {
				key = event.Key
			}
			if chord := hotkeyChord(event.Mods, key); chord != "" {
				a.hotkeyDraft = chord
			}
			return true
		}).Children(func() {
			if a.hotkeyDraft == "" {
				ui.Text(c, a.t("pressKeyboard")).TextColor(a.colors.muted)
			} else {
				a.hotkeyCaps(c, a.hotkeyDraft, true, !validHotkey(a.hotkeyDraft))
			}
		})
		if a.hotkeyDraft != "" && !validHotkey(a.hotkeyDraft) {
			ui.Text(c, strings.ReplaceAll(a.t("hotkeyNeedsModifier"), "{modifiers}", "Ctrl, Alt, Shift, Super")).FontSize(fontSize(c, 13)).TextColor(a.colors.danger)
		} else if conflict := a.hotkeyConflict(); conflict != "" {
			ui.Text(c, strings.ReplaceAll(a.t("hotkeyConflictWith"), "{action}", a.hotkeyLabel(conflict))).FontSize(fontSize(c, 13)).TextColor(a.colors.muted)
		}
		ui.Row(c).Gap(10).Children(func() {
			if a.cfg.Hotkeys[a.hotkeyAction] != "" {
				ui.Button(c, a.t("remove")).Radius(999).OnClick(func() { a.hotkeyDraft = ""; a.saveHotkey() })
			}
			ui.Spacer(c)
			ui.Button(c, a.t("cancel")).Radius(999).OnClick(func() { a.hotkeyOpen = false })
			ui.PrimaryButton(c, a.t("save")).Radius(999).Disabled(!validHotkey(a.hotkeyDraft)).OnClick(a.saveHotkey)
		})
	})
	if !a.hotkeyOpen && a.hotkeysSuspended {
		a.hotkeysSuspended = false
		a.installHotkeys()
	}
}
func hotkeyChord(mods ui.Modifiers, key ui.Key) string {
	var parts []string
	if mods&ui.Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&ui.Super != 0 {
		modifier := "Super"
		if runtime.GOOS == "darwin" {
			modifier = "Command"
		}
		parts = append(parts, modifier)
	}
	if mods&ui.Alt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	name := ""
	switch {
	case key >= ui.KeyA && key <= ui.KeyZ:
		name = string(rune('A' + key - ui.KeyA))
	case key >= ui.Key0 && key <= ui.Key9:
		name = string(rune('0' + key - ui.Key0))
	case key >= ui.KeyF1 && key <= ui.KeyF12:
		name = fmt.Sprintf("F%d", key-ui.KeyF1+1)
	default:
		name = map[ui.Key]string{ui.KeyEnter: "Enter", ui.KeyTab: "Tab", ui.KeyBackspace: "Backspace", ui.KeyDelete: "Delete", ui.KeyInsert: "Insert", ui.KeyEscape: "Escape", ui.KeySpace: "Space", ui.KeyUp: "Up", ui.KeyDown: "Down", ui.KeyLeft: "Left", ui.KeyRight: "Right", ui.KeyHome: "Home", ui.KeyEnd: "End", ui.KeyPageUp: "PageUp", ui.KeyPageDown: "PageDown", ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".", ui.KeySlash: "/", ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]", ui.KeyBackslash: "\\", ui.KeySemicolon: ";", ui.KeyQuote: "'", ui.KeyBackquote: "`"}[key]
	}
	if name == "" {
		return ""
	}
	return strings.Join(append(parts, name), "+")
}
