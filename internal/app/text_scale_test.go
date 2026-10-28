package app

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestProxyMetricsMatchOriginalFlutter(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-text-metrics.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scale  float32
		Header float32
		Cards  map[string]float32
		Text   map[string]float32
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		a := testApp(t)
		a.cfg.Scale = tc.Scale
		if got := a.proxyHeaderHeight(); got != tc.Header {
			t.Fatalf("scale %v header: got %v, original %v", tc.Scale, got, tc.Header)
		}
		for card, height := range tc.Cards {
			a.cfg.ProxyCard = card
			if got := a.proxyCardHeight(); got != height {
				t.Fatalf("scale %v %s card: got %v, original %v", tc.Scale, card, got, height)
			}
		}
		for name, leading := range map[string]float32{"callout": 21, "subheadline": 20, "footnote": 18, "caption1": 16, "caption2": 13} {
			if got := a.textHeight(leading); got != tc.Text[name] {
				t.Fatalf("scale %v %s: got %v, original %v", tc.Scale, name, got, tc.Text[name])
			}
		}
	}
}

func TestTextPreferenceScalesExplicitAndDefaultTextOnly(t *testing.T) {
	a := testApp(t)
	view := func(c *ui.Context) {
		a.theme(c)
		ui.Column(c).AlignItems(ui.Start).Children(func() {
			ui.Text(c, "Explicit").FontSize(fontSize(c, 20))
			ui.Text(c, "Default")
			glyph(c, "check").Label("Glyph")
			ui.Box(c).Label("Target").Size(44, 44)
		})
	}
	a.cfg.Scale = 1
	one := ui.NewTester(view, 800, 600)
	base := map[string]ui.Rect{}
	for _, label := range []string{"Explicit", "Default", "Glyph", "Target"} {
		base[label], _ = one.Find(label)
	}
	a.cfg.Scale = 1.4
	one.Frame()
	for _, label := range []string{"Explicit", "Default"} {
		r, _ := one.Find(label)
		if r.W < base[label].W*1.35 || r.H < base[label].H*1.3 {
			t.Fatal("text did not scale", label, base[label], r)
		}
	}
	for _, label := range []string{"Glyph", "Target"} {
		r, _ := one.Find(label)
		if r.W != base[label].W || r.H != base[label].H {
			t.Fatal("non-text geometry scaled", label, base[label], r)
		}
	}
}
