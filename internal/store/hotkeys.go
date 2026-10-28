package store

import (
	"fmt"
	"strings"
)

// CanonicalHotkey compares accelerators independently of modifier spelling/order.
// Unsupported keys and modifiers are retained so import never changes a binding.
func CanonicalHotkey(chord string) string {
	parts := strings.Split(strings.TrimSpace(chord), "+")
	if len(parts) == 1 {
		return strings.ToUpper(parts[0])
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	mods := parts[:len(parts)-1]
	if key == "" && len(mods) > 0 && mods[len(mods)-1] == "" {
		key, mods = "+", mods[:len(mods)-1]
	}
	seen := map[string]bool{}
	var unknown []string
	for _, part := range mods {
		name := strings.ToLower(strings.TrimSpace(part))
		switch name {
		case "command", "cmd", "super", "meta", "win", "windows":
			name = "Super"
		case "control", "ctrl":
			name = "Ctrl"
		case "alt", "option", "altgr":
			name = "Alt"
		case "shift":
			name = "Shift"
		case "fn":
			name = "Fn"
		case "capslock", "caps lock":
			name = "CapsLock"
		default:
			name = strings.ToUpper(name)
			if !seen[name] {
				unknown = append(unknown, name)
			}
		}
		seen[name] = true
	}
	var result []string
	for _, name := range []string{"Fn", "CapsLock", "Super", "Ctrl", "Alt", "Shift"} {
		if seen[name] {
			result = append(result, name)
		}
	}
	result = append(result, unknown...)
	return strings.Join(append(result, strings.ToUpper(key)), "+")
}

func normalizeHotkeys(cfg *Settings) {
	for old, current := range map[string]string{"toggle": "start", "show": "view", "systemProxy": "proxy"} {
		if cfg.Hotkeys[current] == "" && cfg.Hotkeys[old] != "" {
			cfg.Hotkeys[current] = cfg.Hotkeys[old]
		}
		delete(cfg.Hotkeys, old)
	}
}

func flutterHotkey(row map[string]any) string {
	usage := Int(row["key"])
	if usage == 0 {
		return ""
	}
	parts := []string{}
	for _, modifier := range flutterJSONList(row["modifiers"]) {
		parts = append(parts, String(modifier))
	}
	return CanonicalHotkey(strings.Join(append(parts, hidKey(usage)), "+"))
}

func hidKey(usage int) string {
	if usage>>16 == 7 {
		key := usage & 0xffff
		switch {
		case key >= 4 && key <= 29:
			return string(rune('A' + key - 4))
		case key >= 30 && key <= 38:
			return fmt.Sprint(key - 29)
		case key >= 0x3a && key <= 0x45:
			return fmt.Sprintf("F%d", key-0x39)
		case key >= 0x68 && key <= 0x73:
			return fmt.Sprintf("F%d", key-0x5b)
		case key >= 0x59 && key <= 0x61:
			return fmt.Sprintf("Num%d", key-0x58)
		}
		if name := map[int]string{
			0x27: "0", 0x28: "Enter", 0x29: "Escape", 0x2a: "Backspace", 0x2b: "Tab", 0x2c: "Space",
			0x2d: "-", 0x2e: "=", 0x2f: "[", 0x30: "]", 0x31: "\\", 0x33: ";", 0x34: "'", 0x35: "`", 0x36: ",", 0x37: ".", 0x38: "/",
			0x39: "CapsLock", 0x46: "PrintScreen", 0x47: "ScrollLock", 0x48: "Pause", 0x49: "Insert", 0x4a: "Home", 0x4b: "PageUp",
			0x4c: "Delete", 0x4d: "End", 0x4e: "PageDown", 0x4f: "Right", 0x50: "Left", 0x51: "Down", 0x52: "Up",
			0x53: "NumLock", 0x54: "NumDiv", 0x55: "NumMult", 0x56: "NumSub", 0x57: "NumAdd", 0x58: "NumEnter", 0x62: "Num0", 0x63: "NumDec", 0x65: "ContextMenu", 0x67: "NumEqual",
		}[key]; name != "" {
			return name
		}
	}
	if name := map[int]string{0xc00b5: "MediaNextTrack", 0xc00b6: "MediaPreviousTrack", 0xc00b7: "MediaStop", 0xc00cd: "MediaPlayPause", 0xc00e2: "VolumeMute", 0xc00e9: "VolumeUp", 0xc00ea: "VolumeDown"}[usage]; name != "" {
		return name
	}
	return fmt.Sprintf("USB:%08X", usage)
}
