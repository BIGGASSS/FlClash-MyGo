package app

import (
	"encoding/json"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"github.com/egoist/mygo/ui"
)

func TestOriginalGlyphVectorsParseInNativeRenderer(t *testing.T) {
	var vectors map[string]glyphVector
	if err := json.Unmarshal(assets.Read("glyphs.json"), &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 128 {
		t.Fatal("incomplete pinned glyph export", len(vectors))
	}
	for name, vector := range vectors {
		for _, svg := range []string{vector.Outline, vector.Filled} {
			if _, err := ui.ParseSVG([]byte(svg)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
}
