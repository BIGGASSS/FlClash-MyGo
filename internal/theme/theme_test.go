package theme

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestThemeRolesMatchPinnedFlutterCalculations(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-palettes.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference map[string]Colors
	if err = json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	for key, want := range reference {
		parts := strings.Split(key, "/")
		got, err := evaluate(parts[0], parts[1], parts[2] == "true", parts[3] == "true")
		if err != nil {
			t.Fatal(key, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s\ngot:  %v\nwant: %v", key, got, want)
		}
	}
}
func TestThemeRejectsMalformedSeed(t *testing.T) {
	for _, seed := range []string{"", "blue", "#123", "#xxxxxx"} {
		if _, err := Generate(seed, "vivid", false, false); err == nil {
			t.Fatal(seed)
		}
	}
}
