package theme

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/BIGGASSS/FlClash-MyGo/internal/script"
)

// The exported source color corpus also supplies the shipped preset cache.
//
//go:embed testdata/flutter-glass.json
var glassPresets []byte

func presetGlassColors() map[string]Colors {
	var preset struct{ Colors map[string]Colors }
	_ = json.Unmarshal(glassPresets, &preset)
	return preset.Colors
}

var glassCache = struct {
	sync.Mutex
	colors map[string]Colors
}{colors: presetGlassColors()}

// GlassColors computes the source glass tints and the dock selection platter.
// These colors do not make MyGo's different optical material shader-identical.
func GlassColors(seed, style string, dark, black bool) (Colors, error) {
	seed = strings.ToLower(seed)
	if len(seed) != 7 || seed[0] != '#' {
		return nil, fmt.Errorf("invalid theme color %q", seed)
	}
	if _, err := strconv.ParseUint(seed[1:], 16, 24); err != nil {
		return nil, err
	}
	if style != "vivid" && style != "tinted" && style != "soft" && style != "monochrome" {
		style = "vivid"
	}
	key := fmt.Sprintf("%s/%s/%t/%t", seed, style, dark, black)
	glassCache.Lock()
	defer glassCache.Unlock()
	if colors := glassCache.colors[key]; colors != nil {
		return colors, nil
	}
	colors, err := evaluateGlass(seed, style, dark, black)
	if err != nil {
		return nil, err
	}
	if len(glassCache.colors) >= 512 {
		glassCache.colors = presetGlassColors()
	}
	glassCache.colors[key] = colors
	return colors, nil
}

func evaluateGlass(seed, style string, dark, black bool) (Colors, error) {
	config, _ := json.Marshal(map[string]any{"seed": seed, "style": style, "dark": dark, "black": black})
	data, err := script.Evaluate(source+"\nfunction main(input){return FlClashPalette.glassColors(input)}", config)
	if err != nil {
		return nil, err
	}
	var colors Colors
	if err = json.Unmarshal(data, &colors); err != nil {
		return nil, err
	}
	return colors, nil
}
