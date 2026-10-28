package app

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
)

func TestParameterizedMessagesMatchOriginalIntlCatalogs(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-messages.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Locale, Key, Expected string
		Values                map[string]any
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	catalogs := map[string]map[string]string{}
	for _, item := range cases {
		catalog := catalogs[item.Locale]
		if catalog == nil {
			catalog = assets.Catalog(item.Locale)
			catalogs[item.Locale] = catalog
		}
		if got := formatMessage(catalog[item.Key], item.Locale, item.Values); got != item.Expected {
			t.Errorf("%s/%s %v: %q, want %q", item.Locale, item.Key, item.Values, got, item.Expected)
		}
	}
}
