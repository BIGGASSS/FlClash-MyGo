package app

import (
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"
)

// Sources: views/font_family.dart, views/theme.dart::_FontFamilyItem,
// features/form/selection_sheet.dart, common/system_fonts.dart and search.dart.
// An empty Family is the default choice, never a translated label in storage.
type fontPickerRow struct {
	Family string
	// Header precedes the first installed choice inside its virtual row, so
	// it scrolls away like the source's unpinned section header.
	Header bool
}

type fontPickerState struct {
	Open, Loading, Loaded  bool
	Families               []string
	Query                  string
	Search                 bool
	List                   ui.ListState
	Cursor                 int
	Rows                   []fontPickerRow
	Revealed               bool
	FocusSearch, FocusList bool
	LoadStarted            time.Time
	Previews               map[string]bool
	NextPreview            string
	PreviewAt              time.Time
	// A per-session reader makes slow/failing enumeration testable without
	// replacing a process-wide hook or touching any desktop preferences.
	Read func() []ui.FontFamily
}

// sortFontPickerFamilies follows sortFontFamilies: keep the first spelling,
// trim, hide private names, deduplicate and sort without regard to case.
func sortFontPickerFamilies(families []string) []string {
	seen := map[string]bool{}
	var names []string
	for _, family := range families {
		name := strings.TrimSpace(family)
		key := strings.ToLower(name)
		if name == "" || strings.HasPrefix(name, ".") || seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		// Dart String.compareTo orders UTF-16 units, including supplementary
		// characters before BMP private-use characters, not UTF-8 bytes.
		return slices.Compare(utf16.Encode([]rune(strings.ToLower(a))), utf16.Encode([]rune(strings.ToLower(b))))
	})
	return names
}

func fontPickerMatches(label, query string) bool {
	label = strings.ToLower(label)
	for _, term := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(label, term) {
			return false
		}
	}
	return true
}

func fontPickerRows(families []string, query, defaultLabel string) []fontPickerRow {
	var rows []fontPickerRow
	if fontPickerMatches(defaultLabel, query) {
		rows = append(rows, fontPickerRow{})
	}
	headerAdded := false
	for _, family := range families {
		if !fontPickerMatches(family, query) {
			continue
		}
		rows = append(rows, fontPickerRow{Family: family, Header: !headerAdded})
		headerAdded = true
	}
	return rows
}

func (a *App) openFontPicker(width float32) {
	var previews map[string]bool
	if a.fontPicker != nil {
		previews = a.fontPicker.Previews
	}
	a.fontPicker = &fontPickerState{Open: width > 600, Cursor: -1, Previews: previews}
	if width <= 600 {
		a.goPage("fontFamily")
	} else {
		a.resetSheet("font-family")
	}
}

func (a *App) closeFontPicker() {
	if a.deferSheetClose("font-family", a.closeFontPicker) {
		return
	}
	if a.fontPicker != nil {
		a.fontPicker.Open = false
	}
}

func (a *App) loadFontPicker(s *fontPickerState, now time.Time) {
	if s.Loaded || s.Loading {
		return
	}
	s.Loading, s.LoadStarted = true, now
	read := s.Read
	if read == nil {
		read = ui.FontFamilies // Thread-safe; enumeration does not install fonts.
	}
	go func() {
		var names []string
		for _, family := range read() {
			// Source lists host-installed fonts, not bundled app assets.
			if !family.Registered {
				names = append(names, family.Name)
			}
		}
		names = sortFontPickerFamilies(names)
		a.update(func() {
			// This session owns its result even while its tab is hidden. A
			// reopened picker has another state pointer and cannot be replaced.
			s.Families, s.Loading, s.Loaded = names, false, true
		})
	}()
}

func (a *App) fontFamilySetting(c *ui.Context) {
	width, _ := c.Size()
	family := a.cfg.FontFamily
	label := family
	if label == "" {
		label = a.t("defaultText")
	}
	ui.Text(c, a.t("fontFamily")).FontSize(fontSize(c, 13)).TextColor(a.colors.muted).Padding(8, 16, 0, 16)
	ui.ButtonBase(c.Key("font-family-setting")).Label(a.t("fontFamily")).FillWidth().Padding(0).OnClick(func() { a.openFontPicker(width) }).Children(func() {
		ui.Row(c).FillWidth().Padding(14, 12, 14, 16).Gap(12).Children(func() {
			sample := ui.Text(c, "Aa").FontSize(fontSize(c, 17)).FontWeight(600)
			name := ui.Text(c, label).FontSize(fontSize(c, 15)).TextColor(a.colors.muted).SingleLine().Grow(1).MinWidth(0)
			if family != "" {
				sample.Font(family)
				name.Font(family)
			}
			glyph(c, "chevron").FontSize(20).TextColor(a.colors.muted)
		})
	})
}

func (a *App) fontPickerSheet(c *ui.Context) {
	s := a.fontPicker
	if s == nil || !s.Open {
		return
	}
	a.navigationSheet(c, "font-family", &s.Open, a.closeFontPicker, func() {
		if s.Search {
			s.Search, s.Query, s.FocusList = false, "", true
		} else {
			a.closeFontPicker()
		}
	}, func() {
		a.fontPickerHeader(c, s)
		a.fontPickerView(c)
	})
}

func (s *fontPickerState) searchable() bool { return len(s.Families)+1 >= 10 || s.Query != "" }

func (a *App) fontPickerHeader(c *ui.Context, s *fontPickerState) {
	if s.Search {
		ui.Row(c).FillWidth().Height(72).Padding(14, 16).Gap(12).Children(func() {
			a.sheetButton(c, "back", a.t("back"), func() { s.Search, s.Query, s.FocusList = false, "", true })
			a.fontPickerSearch(c, s)
		})
		return
	}
	a.sheetHeader(c, a.t("fontFamily"), "close", a.t("close"), a.closeFontPicker, func() {
		if s.searchable() {
			a.sheetButton(c, "search", a.t("search"), func() { s.Search, s.FocusSearch = true, true })
		}
	})
}

func (a *App) fontPickerSearch(c *ui.Context, s *fontPickerState) {
	field := ui.SearchField(c.Key("font-search"), &s.Query).Label(a.t("search")).Placeholder(a.t("search")).Grow(1).MinWidth(0)
	if s.FocusSearch {
		field.Focus()
		s.FocusSearch = false
	}
	field.OnShortcut(0, ui.KeyDown, func() { s.FocusList = true }).OnSubmit(func() { s.FocusList = true })
	field.OnShortcut(0, ui.KeyEscape, func() {
		s.Query, s.Search, s.FocusList = "", false, true
	})
}

func (a *App) fontPickerView(c *ui.Context) {
	if a.fontPicker == nil {
		a.fontPicker = &fontPickerState{Cursor: -1}
	}
	s := a.fontPicker
	a.loadFontPicker(s, c.Now())
	if s.Previews == nil {
		s.Previews = map[string]bool{}
	}
	moving := false
	if sheet := a.snapSheets["font-family"]; s.Open && sheet != nil {
		moving = sheet.Controller != 1
	}
	// _FontLabel promotes at most one visible name on each settled frame.
	if !moving && s.NextPreview != "" && !s.PreviewAt.Equal(c.Now()) {
		s.Previews[s.NextPreview] = true
		s.NextPreview, s.PreviewAt = "", c.Now()
	}
	rows := fontPickerRows(s.Families, s.Query, a.t("defaultText"))
	if !slices.Equal(rows, s.Rows) {
		s.Rows, s.List, s.Cursor = rows, ui.ListState{}, -1
	}
	if s.Loaded && !s.Revealed {
		for i, row := range rows {
			if row.Family == a.cfg.FontFamily {
				s.Cursor = i
				s.List.ScrollTo(i, ui.Center)
				break
			}
		}
		s.Revealed = true
		if !s.Search {
			s.FocusList = true
		}
	}
	root := ui.Column(c).Fill().MinHeight(0)
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		if s.searchable() {
			s.Search, s.FocusSearch = true, true
		}
	})
	root.Children(func() {
		if !s.Loaded {
			// NullStatusSwitcher holds its first-load illustration for 400 ms.
			elapsed := c.Now().Sub(s.LoadStarted)
			if elapsed < 400*time.Millisecond {
				c.After(400*time.Millisecond - elapsed)
				ui.Box(c).Grow(1)
			} else {
				ui.Box(c).Grow(1).FillWidth().Center().Children(func() { loading(c, 48, a.colors.muted, a.t("loading")) })
			}
		} else if len(rows) == 0 {
			ui.Box(c).Grow(1).FillWidth().Center().Children(func() { ui.Text(c, a.t("noSearchResults")).TextColor(a.colors.muted) })
		} else {
			a.fontPickerList(c, s, rows, moving)
		}
		// A pushed compact-window page docks search at the foot. Desktop
		// sheets replace their title with search instead of opening a dropdown.
		if !s.Open && s.searchable() {
			ui.Row(c).FillWidth().Padding(8, 16, 16, 16).Children(func() { a.fontPickerSearch(c, s) })
		}
	})
}

func (a *App) fontPickerList(c *ui.Context, s *fontPickerState, rows []fontPickerRow, moving bool) {
	// SelectionSheet marks the saved family with a check, not the toolkit's
	// filled-accent list choice. Scope that native list styling to this list;
	// keep the app accent for checks and keyboard focus rings.
	base := c.Theme()
	listTheme := *base
	listTheme.Accent, listTheme.AccentText = a.colors.card, a.colors.text
	listTheme.AccentHover, listTheme.AccentPressed = a.colors.inset, a.colors.inset
	c.SetTheme(&listTheme)
	defer c.SetTheme(base)
	labelOf := func(row fontPickerRow) string {
		if row.Family == "" {
			return a.t("defaultText")
		}
		return row.Family
	}
	s.List.Key = func(i int) any { return rows[i].Family }
	s.List.Label = func(i int) string { return labelOf(rows[i]) }
	s.List.Selected = &s.Cursor
	list := ui.List(c.Key("font-list"), &s.List, len(rows), func(i int) {
		row := rows[i]
		label := labelOf(row)
		ui.Column(c).FillWidth().Children(func() {
			if row.Header {
				ui.Text(c, a.t("installedFonts")).FillWidth().Padding(24, 16, 11, 16).FontSize(fontSize(c, 17)).FontWeight(600).TextColor(a.colors.muted).Background(a.colors.page)
			}
			top, bottom := float32(0), float32(0)
			if i == 0 || row.Header {
				top = 24
			}
			if i == len(rows)-1 || rows[i+1].Header {
				bottom = 24
			}
			ui.Row(c).FillWidth().MinHeight(max(56, a.textHeight(22)+20)).Padding(10, 16).Gap(12).Background(a.colors.card).OnClick(func() { a.selectFontFamily(row.Family) }).DrawOver(func(p *ui.Painter, r ui.Rect) {
				if i == s.Cursor && c.Resolve(s.List.Handle).FocusVisible() {
					p.FocusRing(r, [4]float32{top, top, bottom, bottom})
				}
			}).Children(func() {
				if bottom == 0 {
					ui.Box(c).Absolute().Left(16).Right(14).Bottom(0).Height(1).Background(a.colors.line.Alpha(.6))
				}
				name := ui.Text(c, label).FontSize(fontSize(c, 17)).SingleLine().Grow(1).MinWidth(0)
				if row.Family != "" {
					if s.Previews[row.Family] {
						name.Font(row.Family)
					} else {
						if s.NextPreview == "" {
							s.NextPreview = row.Family
						}
						if !moving {
							c.AnimationFrame()
						}
					}
				}
				if row.Family == a.cfg.FontFamily {
					glyph(c, "check").FontSize(20).TextColor(a.colors.accent)
				}
			}).Selected(row.Family == a.cfg.FontFamily).Radius(top, top, bottom, bottom)
		})
	}).Grow(1).MinHeight(0).FillWidth().Padding(12, 16, 20, 16).Label(a.t("fontFamily"))
	if s.FocusList {
		list.Focus()
		s.FocusList = false
	}
	if list.Submitted() || list.Shortcut(0, ui.KeySpace) {
		if s.Cursor >= 0 && s.Cursor < len(rows) {
			a.selectFontFamily(rows[s.Cursor].Family)
		}
	}
}

func (a *App) selectFontFamily(family string) {
	if a.cfg.FontFamily != family {
		a.cfg.FontFamily = family
		a.save()
	}
	// SelectionSheet.removeOnSelect is false: selecting never pops the page
	// or closes the sheet, including when the default is selected again.
}
