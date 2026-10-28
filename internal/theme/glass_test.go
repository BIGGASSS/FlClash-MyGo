package theme

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGlassColorsMatchPinnedFlutter(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-glass.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		Colors map[string]Colors
	}
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	if len(reference.Colors) != 160 {
		t.Fatal("incomplete source glass corpus", len(reference.Colors))
	}
	for key, want := range reference.Colors {
		parts := strings.Split(key, "/")
		got, err := evaluateGlass(parts[0], parts[1], parts[2] == "true", parts[3] == "true")
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %v, got %v, want %v", key, err, got, want)
		}
		cached, err := GlassColors(parts[0], parts[1], parts[2] == "true", parts[3] == "true")
		if err != nil || !reflect.DeepEqual(cached, want) {
			t.Fatalf("shipped preset differs at %s: %v", key, err)
		}
	}
}

func TestGlassColorValidationAndCache(t *testing.T) {
	for _, seed := range []string{"", "#123", "#xxxxxx"} {
		if _, err := GlassColors(seed, "vivid", false, false); err == nil {
			t.Fatal("accepted invalid seed", seed)
		}
	}
	first, err := GlassColors("#007AFF", "unsupported", false, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GlassColors("#007aff", "vivid", false, false)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("theme normalization differed", first, second, err)
	}
}
