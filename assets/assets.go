package assets

import (
	"embed"
	"encoding/json"
	"strings"
)

//go:embed LICENSE filters.json glyphs.json caption-glyphs.json dashboard-shapes.json icon.png tray/unix/*.png tray/macos/*.png locales/*.arb services/*.svg data/* fonts/*
var Files embed.FS

func Read(path string) []byte { b, _ := Files.ReadFile(path); return b }
func Catalog(locale string) map[string]string {
	locale = strings.ReplaceAll(locale, "-", "_")
	if strings.HasPrefix(locale, "zh") {
		locale = "zh_CN"
	}
	if i := strings.Index(locale, "_"); i >= 0 && locale != "zh_CN" {
		locale = locale[:i]
	}
	b, err := Files.ReadFile("locales/intl_" + locale + ".arb")
	if err != nil {
		b = Read("locales/intl_en.arb")
	}
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	out := map[string]string{}
	for k, v := range raw {
		if s, ok := v.(string); ok && !strings.HasPrefix(k, "@") {
			out[k] = s
		}
	}
	return out
}
