# Native font-family picker

Source behavior comes from the pinned FlClash files:

- `lib/views/font_family.dart`: default/installed choices, saved-family check,
  asynchronous enumeration, and one newly previewed family per settled frame.
- `lib/views/theme.dart::_FontFamilyItem`: the `Aa`/family preview card; a side
  sheet above 600 points and a pushed page at compact desktop-window sizes.
- `lib/features/form/selection_sheet.dart`: search at ten choices, filtering
  every section, initial selected-item reveal, and **no dismissal on selection**.
- `lib/common/system_fonts.dart` and `lib/common/search.dart`: first-spelling,
  case-insensitive deduplication/sorting (Dart UTF-16 ordering), hidden-name
  exclusion, and case-insensitive AND search over whitespace-separated terms.

`font_picker.go` owns only main-thread native state. Font enumeration is a
read-only worker; `App.update` commits its session-owned result through
`Window.Update` (or the headless update queue). No font is installed, no system
preference is written, and picking a family only saves `Settings.FontFamily`.
The default is stored as the empty string, never as translated UI text.
Compact-page search, cursor, list position and history survive tab switching.
The unpinned installed-section header shares the first installed virtual row;
it does not become a selectable option or stick over the scrolled list.

The list uses native keyboard navigation and type-ahead. Enter/Space applies
its cursor; moving the cursor alone does not change the preference. Cmd/Ctrl+F
focuses search; Down/Enter transfers focus to the list. SearchField first clears
a nonempty query on Escape; another Escape leaves desktop-sheet search, then
another dismisses the sheet. Mouse selection also leaves the route open.

## Reproducible references

`tool/reference/port_font_picker_test.dart` runs the original sort/search helpers
and the unchanged theme/picker widgets under synthetic providers. With a
prepared source checkout (native build hooks disabled as described in
`tool/reference/README.md`), copy that harness and `port_reference_test.dart`
into `test/`, then run:

```sh
FLUTTER_ROOT=/home/tim/fvm/versions/3.47.5 \
FLCLASH_FONT_REFERENCE_DIR=/path/to/output \
/path/to/flutter test --no-pub --dart-define=SAFE_MODE=true \
  test/port_font_picker_test.dart
```

The harness was run with Flutter **3.47.5**. Its four sort cases and seven search
cases are retained in `testdata/flutter-font-picker.json`; the native test reads
that corpus directly. Four light/dark selected/search source renders are in
`docs/reference/font-picker-*.png`. It also verifies per-name family styles,
resetting to default, and remaining open after either selection.

Widget tests substitute Roboto for Ahem and the synthetic family names; the
images verify layout/controls, **not installed-font glyph accuracy**. Japanese
and bundled JetBrainsMono previews use the test environment's fallback/assets.
No reference test launches a core, touches system proxy/TUN, or enumerates the
workstation's real installed fonts.

Native headless captures use deterministic families:

```sh
FLCLASH_FONT_NATIVE_DIR=/path/to/output CGO_ENABLED=0 \
  go test ./internal/app -run '^TestFontPickerNativeReferences$' -count=1
```

## Remaining differences

- MyGo caches system enumeration until font registration/restart. Newly
  installed OS fonts may require an app restart. Its public API also merges
  registered and system families by name; excluding bundled assets can omit an
  OS family with the same name as a bundled face. No vendored toolkit change
  was made to expose a separate system-only list.
- Loading uses native delayed progress feedback, not the original fonts
  illustration; search/header and compact-page container morphs are not exact
  Flutter transitions. The existing appearance page outside the font entry is
  deliberately unchanged.
- Real installed-family rendering, accessibility, platform focus behavior and
  live enumeration on Windows/macOS still require desktop validation. The
  synthetic source renders and headless tests do not establish complete parity.
