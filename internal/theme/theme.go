// Package theme reproduces the pinned FlClash HCT theme and surface roles.
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

//go:embed palette.js
var source string

//go:embed presets.json
var presets []byte

type Colors map[string]string

var cache = struct {
	sync.Mutex
	colors map[string]Colors
}{colors: map[string]Colors{}}

func init() { _ = json.Unmarshal(presets, &cache.colors) }

func Generate(seed, style string, dark, black bool) (Colors, error) {
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
	cache.Lock()
	defer cache.Unlock()
	if colors := cache.colors[key]; colors != nil {
		return colors, nil
	}
	colors, err := evaluate(seed, style, dark, black)
	if err != nil {
		return nil, err
	}
	if len(cache.colors) > 512 {
		cache.colors = map[string]Colors{}
		_ = json.Unmarshal(presets, &cache.colors)
	}
	cache.colors[key] = colors
	return colors, nil
}
func evaluate(seed, style string, dark, black bool) (Colors, error) {
	config, _ := json.Marshal(map[string]any{"seed": seed, "style": style, "dark": dark, "black": black})
	data, err := script.Evaluate(source+"\nfunction main(input){return FlClashPalette.palette(input)}", config)
	if err != nil {
		return nil, err
	}
	var colors Colors
	if err = json.Unmarshal(data, &colors); err != nil {
		return nil, err
	}
	return colors, nil
}
