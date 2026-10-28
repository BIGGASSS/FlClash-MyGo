package app

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
	"github.com/egoist/mygo/ui"
)

func loadedFontPicker(families ...string) *fontPickerState {
	return &fontPickerState{Loaded: true, Cursor: -1, Families: sortFontPickerFamilies(families)}
}

func fontPickerTester(a *App, width, height int) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		a.flushUpdates()
		a.theme(c)
		a.fontPickerView(c)
	}, width, height)
}

func TestFontPickerSourceSortAndFilter(t *testing.T) {
	got := sortFontPickerFamilies([]string{" beta Serif ", ".Hidden", "", "Alpha Sans", "alpha sans", "BETA SERIF", " 日本語 ", "École", "école"})
	want := []string{"Alpha Sans", "beta Serif", "École", "日本語"}
	if !slices.Equal(got, want) {
		t.Fatalf("sort/first spelling: got %q want %q", got, want)
	}
	for _, tc := range []struct {
		query string
		want  []fontPickerRow
	}{
		{"", []fontPickerRow{{}, {Family: "Alpha Sans", Header: true}, {Family: "beta Serif"}, {Family: "École"}, {Family: "日本語"}}},
		{" SANS \t alPha ", []fontPickerRow{{Family: "Alpha Sans", Header: true}}},
		{"DEFA", []fontPickerRow{{}}},
		{"  日本 ", []fontPickerRow{{Family: "日本語", Header: true}}},
		{"missing", nil},
		{"Installed fonts", nil}, // Section labels are not searchable fields.
	} {
		if rows := fontPickerRows(got, tc.query, "Default"); !slices.Equal(rows, tc.want) {
			t.Errorf("query %q: %v, want %v", tc.query, rows, tc.want)
		}
	}
	if rows := fontPickerRows([]string{"Default"}, "default", "Default"); len(rows) != 2 || rows[0] == rows[1] {
		t.Fatalf("translated default collided with installed family identity: %v", rows)
	}
}

func TestFontPickerSelectionSavesWithoutPopping(t *testing.T) {
	a := testApp(t)
	a.page, a.history = "fontFamily", []string{"tools", "theme"}
	a.fontPicker = loadedFontPicker("Alpha Sans", "Beta Serif")
	tt := fontPickerTester(a, 480, 500)
	if !tt.HasText("Default") || !tt.HasText("Installed fonts") || tt.HasText("Search") {
		t.Fatalf("short source list: %v", tt.Texts())
	}
	for _, tc := range []struct{ label, family string }{{"Beta Serif", "Beta Serif"}, {"Default", ""}} {
		if err := tt.Click(tc.label); err != nil {
			t.Fatal(err)
		}
		if a.cfg.FontFamily != tc.family || a.page != "fontFamily" || !slices.Equal(a.history, []string{"tools", "theme"}) {
			t.Fatal("selection popped navigation or changed the wrong setting", a.cfg.FontFamily, a.page, a.history)
		}
		_, cfg, err := store.Open(a.store.Dir)
		if err != nil || cfg.FontFamily != tc.family {
			t.Fatal("font preference was not persisted", cfg.FontFamily, err)
		}
	}
}

func TestFontPickerKeyboardSearchAndVirtualization(t *testing.T) {
	a := testApp(t)
	families := make([]string, 2000)
	for i := range families {
		families[i] = fmt.Sprintf("Family %04d Sans", i)
	}
	a.fontPicker = loadedFontPicker(families...)
	tt := fontPickerTester(a, 480, 360)
	if len(tt.Texts()) > 40 {
		t.Fatal("installed fonts were not virtualized", len(tt.Texts()))
	}
	if !tt.Focused(a.t("fontFamily")) {
		t.Fatal("initial list focus was lost")
	}
	tt.Key(0, ui.KeyEnd)
	if !tt.HasText("Family 1999 Sans") || a.cfg.FontFamily != "" {
		t.Fatal("keyboard reveal changed the setting before activation", a.cfg.FontFamily, tt.Texts())
	}
	if tt.HasText(a.t("installedFonts")) {
		t.Fatal("the source's unpinned section header stuck over scrolled choices")
	}
	tt.Key(0, ui.KeyEnter)
	if a.cfg.FontFamily != "Family 1999 Sans" {
		t.Fatal("Enter did not activate the keyboard cursor", a.cfg.FontFamily)
	}
	tt.Key(ui.Cmd, ui.KeyF)
	if !tt.Focused(a.t("search")) {
		t.Fatal("find shortcut did not move focus into search")
	}
	tt.Type("sans 0007")
	if !tt.HasText("Family 0007 Sans") || tt.HasText("Family 0008 Sans") || tt.HasText("Default") {
		t.Fatal("AND search did not filter every section", tt.Texts())
	}
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeySpace)
	if a.cfg.FontFamily != "Family 0007 Sans" {
		t.Fatal("search to list keyboard activation failed", a.cfg.FontFamily)
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("not installed")
	if !tt.HasText(a.t("noSearchResults")) || tt.HasText("Installed fonts") {
		t.Fatal("search empty state did not replace the list", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if a.fontPicker.Query != "" || !tt.HasText("Default") {
		t.Fatal("Escape did not clear the search", tt.Texts())
	}
}

func TestFontPickerSelectedRevealAndTabState(t *testing.T) {
	a := testApp(t)
	a.frameEnabled, a.activeNavigation, a.page = true, "tools", "fontFamily"
	a.history = []string{"tools", "theme"}
	families := make([]string, 400)
	for i := range families {
		families[i] = fmt.Sprintf("Font %03d", i)
	}
	a.fontPicker = loadedFontPicker(families...)
	a.cfg.FontFamily = "Font 350"
	tt := ui.NewTester(a.View, 540, 700)
	if !tt.HasText("Font 350") {
		t.Fatal("saved font was not revealed on entry", tt.Texts())
	}
	a.fontPicker.Query = "350"
	tt.Advance(time.Millisecond)
	session := a.fontPicker
	a.selectNavigation("dashboard")
	tt.Advance(time.Millisecond)
	a.selectNavigation("tools")
	tt.Advance(time.Millisecond)
	if a.page != "fontFamily" || a.fontPicker != session || a.fontPicker.Query != "350" || !tt.HasText("Font 350") {
		t.Fatal("tab switching discarded picker state", a.page)
	}
	a.back()
	if a.page != "theme" {
		t.Fatal("back did not return to appearance", a.page)
	}
}

func TestFontPickerDesktopSheetAndCompactNavigation(t *testing.T) {
	for _, width := range []int{540, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := testApp(t)
			a.page, a.frameEnabled = "theme", true
			a.openFontPicker(float32(width))
			s := a.fontPicker
			s.Loaded, s.Families = true, []string{"Alpha Sans", "Beta Serif"}
			tt := ui.NewTester(a.View, width, 800)
			tt.Advance(time.Second)
			if width > 600 && (a.page != "theme" || !s.Open) || width <= 600 && (a.page != "fontFamily" || s.Open) {
				t.Fatal("wrong source presentation", a.page, s.Open)
			}
			if err := tt.Click("Beta Serif"); err != nil {
				t.Fatal(err)
			}
			if a.cfg.FontFamily != "Beta Serif" || width > 600 && !s.Open || width <= 600 && a.page != "fontFamily" {
				t.Fatal("selection dismissed the picker")
			}
			if width > 600 {
				tt.Key(0, ui.KeyEscape)
				tt.Advance(time.Second)
				if s.Open || a.page != "theme" {
					t.Fatal("Escape did not dismiss the sheet")
				}
			} else {
				a.back()
				if a.page != "theme" {
					t.Fatal("compact page back lost appearance")
				}
			}
		})
	}
}

func TestFontPickerLocaleAndEmptyInstalledList(t *testing.T) {
	for _, locale := range []string{"en", "zh_CN", "ja", "ko", "ru", "fr", "es"} {
		t.Run(locale, func(t *testing.T) {
			a := testApp(t)
			a.cfg.Locale = locale
			a.cfg.FontFamily = "Unavailable Family" // Never erase it just by opening.
			a.fontPicker = loadedFontPicker()
			tt := fontPickerTester(a, 380, 400)
			if !tt.HasText(assets.Catalog(locale)["defaultText"]) || tt.HasText(a.t("installedFonts")) || a.cfg.FontFamily != "Unavailable Family" {
				t.Fatal("localized default/failure fallback", tt.Texts())
			}
			if err := tt.Click(a.t("defaultText")); err != nil {
				t.Fatal(err)
			}
			if a.cfg.FontFamily != "" {
				t.Fatal("default stored a translated label", a.cfg.FontFamily)
			}
		})
	}
}

func TestFontPickerAsyncSessionAndBundledFonts(t *testing.T) {
	a := testApp(t)
	result := make(chan []ui.FontFamily, 1)
	done := make(chan struct{})
	old := &fontPickerState{Cursor: -1, Read: func() []ui.FontFamily { defer close(done); return <-result }}
	a.fontPicker = old
	tt := fontPickerTester(a, 480, 400)
	if tt.HasText("Default") {
		t.Fatal("loading showed an actionable default before enumeration")
	}
	tt.Advance(450 * time.Millisecond)
	if !tt.HasText(a.t("loading")) {
		t.Fatal("delayed loading feedback is missing", tt.Texts())
	}
	current := loadedFontPicker("Current Font")
	a.fontPicker = current
	result <- []ui.FontFamily{{Name: " Bundled ", Registered: true}, {Name: "Beta Serif"}, {Name: "beta serif"}, {Name: ".Private"}}
	<-done
	for deadline := time.Now().Add(time.Second); !old.Loaded && time.Now().Before(deadline); {
		tt.Advance(time.Millisecond)
		time.Sleep(time.Millisecond)
	}
	if !old.Loaded || !slices.Equal(old.Families, []string{"Beta Serif"}) || a.fontPicker != current || !tt.HasText("Current Font") {
		t.Fatal("late result overwrote a new route, or bundled/private fonts leaked into the installed section", old.Families)
	}
}

func TestFontPickerPreviewBudget(t *testing.T) {
	a := testApp(t)
	s := loadedFontPicker("Alpha Sans", "Beta Serif", "Gamma Mono")
	a.fontPicker = s
	tt := fontPickerTester(a, 480, 500)
	for i := 0; i < 5; i++ {
		before := len(s.Previews)
		tt.Advance(16 * time.Millisecond)
		if len(s.Previews)-before > 1 {
			t.Fatal("more than one font was first previewed in one frame")
		}
	}
	if len(s.Previews) != 3 {
		t.Fatal("visible names never received their family previews", s.Previews)
	}
}

func TestFontPickerOriginalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter-font-picker.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		SortCases   []struct{ Input, Output []string }
		SearchCases []struct {
			Families            []string
			Query, DefaultLabel string
			Output              []string
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, tc := range corpus.SortCases {
		if got := sortFontPickerFamilies(tc.Input); !slices.Equal(got, tc.Output) {
			t.Errorf("source sort: %q != %q", got, tc.Output)
		}
	}
	for _, tc := range corpus.SearchCases {
		var got []string
		for _, row := range fontPickerRows(tc.Families, tc.Query, tc.DefaultLabel) {
			got = append(got, row.Family)
		}
		if !slices.Equal(got, tc.Output) {
			t.Errorf("source search %q: %q != %q", tc.Query, got, tc.Output)
		}
	}
}

func TestFontPickerSettingCardAndSheetSearch(t *testing.T) {
	a := testApp(t)
	a.page = "theme"
	tt := ui.NewTester(func(c *ui.Context) {
		a.flushUpdates()
		a.theme(c)
		ui.Column(c).Fill().Children(func() { a.fontFamilySetting(c) })
		a.fontPickerSheet(c)
	}, 1200, 800)
	if !tt.HasText("Aa") || !tt.HasText(a.t("fontFamily")) || !tt.HasText(a.t("defaultText")) {
		t.Fatal("source preview card missing", tt.Texts())
	}
	if err := tt.Click("Aa"); err != nil {
		t.Fatal(err)
	}
	s := a.fontPicker
	s.Loaded, s.Families = true, []string{"Alpha Sans", "Beta Serif", "Delta Sans", "Epsilon Serif", "Eta Sans", "Gamma Mono", "Iota Mono", "JetBrains Mono", "Roboto"}
	tt.Advance(time.Second)
	if err := tt.Click(a.t("search")); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused(a.t("search")) {
		t.Fatal("sheet search did not receive focus")
	}
	tt.Type("SANS ALPHA")
	if !tt.HasText("Alpha Sans") || tt.HasText("Beta Serif") || !slices.Equal(s.Rows, []fontPickerRow{{Family: "Alpha Sans", Header: true}}) {
		t.Fatal("sheet search was not scoped to its choice list", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if s.Query != "" || !s.Open {
		t.Fatal("search Escape closed the containing sheet")
	}
	tt.Key(0, ui.KeyEscape)
	if s.Search || !s.Open {
		t.Fatal("empty search Escape should leave search before dismissing the sheet")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(time.Second)
	if s.Open {
		t.Fatal("Escape did not dismiss the sheet after leaving search")
	}
}

// Opt-in captures are safe/headless, deterministic and never enumerate the
// workstation's fonts. Source captures use the same names with fixture faces.
func TestFontPickerNativeReferences(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, state := range []string{"selected", "default", "search"} {
			name := "font-picker-" + state + map[bool]string{false: "-light", true: "-dark"}[dark]
			t.Run(name, func(t *testing.T) {
				a := testApp(t)
				a.page = "theme"
				if dark {
					a.cfg.Theme = "dark"
				}
				a.openFontPicker(1200)
				s := a.fontPicker
				s.Loaded, s.Families = true, sortFontPickerFamilies([]string{"Alpha Sans", "Beta Serif", "Gamma Mono", "Delta Sans", "Epsilon Serif", "Eta Sans", "Iota Mono", "JetBrainsMono", "Roboto", "日本語"})
				if state == "selected" {
					a.cfg.FontFamily = "Beta Serif"
				} else if state == "search" {
					s.Search, s.Query = true, "sans alpha"
				}
				tt := ui.NewTester(a.View, 1200, 800)
				for range 16 {
					tt.Advance(100 * time.Millisecond)
				}
				if !tt.HasText("Alpha Sans") || !s.Open {
					t.Fatal("picker did not render", tt.Texts())
				}
				if dir := os.Getenv("FLCLASH_FONT_NATIVE_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					file, err := os.Create(filepath.Join(dir, name+".png"))
					if err != nil {
						t.Fatal(err)
					}
					defer file.Close()
					if err := png.Encode(file, tt.Image()); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}
