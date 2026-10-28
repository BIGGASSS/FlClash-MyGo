package app

import (
	"encoding/json"
	"runtime"
	"sync"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"github.com/egoist/mygo/ui"
)

type glyphVector struct {
	Outline string `json:"outline"`
	Filled  string `json:"filled"`
	RTL     bool   `json:"rtl"`
}

var loadGlyphs sync.Once
var glyphVectors map[string]glyphVector
var glyphCache = map[string]*ui.SVG{}

var glyphAliases = map[string]string{
	"back": "chevronBack", "chevron": "chevronForward", "plus": "add", "up": "arrowUp", "down": "arrowDown",
	"shield": "verified", "theme": "palette", "cloud": "cloudSync", "trash": "delete", "list": "appsList", "table": "iconTile",
	"triangle-up": "chevronUp", "triangle-down": "chevronDown",
}

func glyph(c *ui.Context, name string) ui.Element { return glyphVariant(c, name, false) }

func ensureGlyphs() {
	loadGlyphs.Do(func() { _ = json.Unmarshal(assets.Read("glyphs.json"), &glyphVectors) })
}

func glyphVariant(c *ui.Context, name string, filled bool) ui.Element {
	if name == "back" && runtime.GOOS != "darwin" {
		name = "arrowBack"
	}
	ensureGlyphs()
	if alias := glyphAliases[name]; alias != "" {
		name = alias
	}
	vector, ok := glyphVectors[name]
	if !ok {
		name, vector = "settings", glyphVectors["settings"]
	}
	key, data := name, vector.Outline
	if filled {
		key, data = name+"/filled", vector.Filled
	}
	svg := glyphCache[key]
	if svg == nil {
		svg = ui.MustParseSVG([]byte(data))
		glyphCache[key] = svg
	}
	return ui.Icon(c, svg).FontSize(15)
}
