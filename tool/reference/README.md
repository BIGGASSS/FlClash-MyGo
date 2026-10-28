# Pinned Flutter reference renders

These test harnesses mount unchanged widgets from FlClash
`7fd417fb9a9a277c6a9339a8ac87f907df7c25f7` under synthetic providers. They use
Flutter 3.47.6, the version pinned in that source's workflow. They do not start
its core, privileged helper, system-proxy manager, or network takeover.

`port_reference_test.dart` renders proxy lists, proxy tabs, the dashboard,
custom SVG/raster group icons, queued/running delay controls, appearance settings, desktop shortcuts, IP details, service checks, and service
management, connected/disconnected core memory details, and all three core connection
controls, collapsed navigation, and narrow desktop dashboard editing/custom order, profile details, subscription usage, and provider rows, desktop caption states, profile grids and profile/proxy pickers in light/dark mode (90 renders). Its repaint boundary includes the
original modal routes, with the Flutter debug banner suppressed. Most references
use 1200 × 800; additional dashboards use 1200 × 640, 1000 × 800, 1440 × 860, and 1440 × 1000
to cover summaries, two columns, and standard density. Additional proxy/dashboard
renders use the supported 80% and 140% text limits with window geometry fixed.
Each render also exports original measured line/header/card metrics; the three
scale cases are committed in `internal/app/testdata/flutter-text-metrics.json`. Normal shadow blur is
explicitly enabled, overriding Flutter widget tests' default. The sidebar uses the
original `NavigationSidebar`. Sidebar hit-target rectangles and dashboard card
rectangles are also exported; selected cases are retained in
`internal/app/testdata/flutter-layout-metrics.json`. Roboto is loaded from Flutter's material-font
artifacts, so test text does not use Flutter's Ahem test font.

`port_palette_test.dart` exports the original theme/surface calculations for ten
accent seeds, all four styles, both appearances, and pure-black settings. The Go
theme test compares its numerical implementation against every exported role.

To regenerate in a separate checkout of the pinned FlClash source:

1. Initialize its core submodule and use Flutter 3.47.6.
2. Copy the desired harness files into its `test/` directory.
3. Temporarily set **both** `hooks.user_defines.setup.build_assets` and
   `hooks.user_defines.rust_api.build_assets` to `false` in its `pubspec.yaml`, as
   documented in its `.agents/commands.md`. Keep that edit out of commits.
4. Run `flutter pub get`, then the tests below. Restore the original pubspec and
   remove the copied harness files afterward.

```sh
FLUTTER_ROOT=/path/to/flutter \
FLCLASH_REFERENCE_DIR=/path/to/FlClash-MyGo/build/reference \
FLCLASH_ICON_FIXTURE=/path/to/FlClash-MyGo/internal/app/testdata/proxy-icons.json \
flutter test --no-pub --dart-define=SAFE_MODE=true test/port_reference_test.dart

FLCLASH_PALETTE_PATH=/path/to/FlClash-MyGo/build/reference/palettes.json \
flutter test --no-pub --dart-define=SAFE_MODE=true test/port_palette_test.dart

FLCLASH_CAPTION_PATH=/path/to/FlClash-MyGo/assets/caption-glyphs.json \
FLCLASH_GLYPHS_PATH=/path/to/FlClash-MyGo/assets/glyphs.json \
flutter test --no-pub --dart-define=SAFE_MODE=true test/port_glyphs_test.dart

FLCLASH_RUNTIME_PATH=/path/to/FlClash-MyGo/internal/store/testdata/flutter-runtime.json \
flutter test --no-pub --dart-define=SAFE_MODE=true test/port_runtime_test.dart
```

The image harness is a widget reference, not an installed-app screenshot. The
memory card reads the reference test process's RSS; other values are synthetic,
and profile file counts may be absent. Window-manager chrome, full application
managers, platform dialogs, and OS integration are outside this harness. Use the
images for matching geometry and controls, with those limits in mind.

The numerical palette fixture is copied to `internal/theme/testdata` and its
preset cache to `internal/theme/presets.json`. Regenerate the internal numerical
helper with `npm ci && npm run build` in `tool/theme`; ordinary app builds need
only Go. The helper bundles Google's Apache-2.0 Material Color Utilities 0.3.0
and FlClash's surface rules; all UI elements are still drawn by native MyGo.

`port_glyphs_test.dart` exports all 128 static glyphs and desktop control states
from the pinned `AppGlyphs`, in outline and filled forms. Paths, rounded
polylines, circles, links, and arcs retain their original geometry. Flutter's
superellipse boxes are flattened and simplified with a maximum deviation of
0.001 on the 24-unit glyph grid; coordinates retain five decimal places. SVG
luminance masks retain the filled forms' detail cutouts, with the body outlines
painted afterward, matching the original painter. The generated vectors remain
under FlClash's GPL-3.0 license. Ordinary Go builds use the committed export.

`port_runtime_test.dart` calls the original configuration assembler without
starting a core or listeners. Its 36 cases cover inherited and selected general
settings, enabled/disabled/empty DNS, DNS map replacement, controller and inbound
ownership, authentication, interface modes, TUN settings, private routes, system
DNS, hosts, and sniffer normalization. The Go store tests compare every resulting
field, retain unknown fields, and verify the source YAML remains byte-exact.

The narrow dashboard renders use 820 × 800 at 80%, 100%, and 140%, plus editing
and a custom card order. The harness supplies the actual sidebar width to its
state provider so the original edit action is available. Card rectangles retain
the original natural heights and text-scale spacing. Profile-count skeletons
and process RSS remain the fixture limitations described above.

`port_shapes_test.dart` exports the exact unit-square cubic curves for the
country clover, cookie-12 decoration, and matched circle/cookie-9 morph. Set
`FLCLASH_SHAPES_PATH` to `assets/dashboard-shapes.json`. The original material
shape package's MIT notice is retained. `port_messages_test.dart` exports all
22 generated cardinal-plural messages across the seven locales and eleven
counts (1,694 cases), including fractional and negative values. Set
`FLCLASH_MESSAGES_PATH` to `internal/app/testdata/flutter-messages.json`.
Dart Intl 0.20.3's BSD notice is retained for the translated behavior. The
reference test also exports the original dashboard-widget migration cases to
`dashboard-widgets.json` in `FLCLASH_REFERENCE_DIR`.

Provider references include source row sizing, count/expiry chips and usage meters.
`compact-counts.json` exports the original English-pinned compact number labels;
`internal/app/testdata/flutter-compact-counts.json` retains the native comparison corpus.

The four caption vectors follow the original `caption_icon.dart`, including
sampled continuous corners and the restore glyph's front-rectangle exclusion.
The caption references export the original 32/46-point hit targets, state glyphs,
pin and close-hover colors. The color corpus includes the on-error role.

`port_navigation_test.dart` adds fourteen light/dark renders of the floating
and docked bottom bars, 380/600-point windows, 140% text, the three-tab empty
state, laptop sidebar, and expanded overlay. It imports the main reference
harness and must be copied alongside it. Set `FLCLASH_REFERENCE_DIR` and run
`flutter test --no-pub --dart-define=SAFE_MODE=true test/port_navigation_test.dart`.
It also exports sixty spring samples and seven pointer-velocity fixtures from
Flutter's original simulations and tracker. Native tests compare those values
and the dock bounds directly. Default non-glass surfaces are covered; optional
glass rendering and every intermediate pointer animation are not.

The profile/proxy picker cases add eighteen original light/dark renders,
including 560-point bottom sheets. The main harness has 90 renders and the
navigation harness has fourteen (104 total). `profileCards` in each image's
JSON sidecar contains measured original card bounds. Small-sheet images are
captured 500 ms into presentation; their residual subpixel spring offset is
visible in those bounds. Native static captures settle at the exact detent.

## Supplemental continuation references

Eight more source images (four font-picker and four glass-fallback states) bring
`docs/reference` to 112 images. Additional numerical/behavior harnesses are:

- `port_font_picker_test.dart`: original sort/search and picker behavior; see
  `internal/app/font_picker.md` for synthetic font aliases and limitations.
- `port_glass_test.dart`: 160 exact tint/platter colors and shader-unsupported
  dock fallback images. It does not establish GPU/refraction parity.
- `port_dashboard_morph_test.dart` plus `prepare_dashboard_morph.py`: unchanged
  private source widget/painter timelines, rounded glyph paths, press springs,
  focus and accessibility traces; see `internal/app/dashboard_morph.md`.
- `port_profile_qr.dart`: 24 original `qr` encoder matrices, including ECI,
  capacity and mask selection; see `internal/app/profile_share.md`.
- `port_backup_test.dart`: original parser/generated models and actual
  `BackupAction.restore` in compatible/override/profiles-only modes. Generate
  synthetic input using `TestFlutterBackupReferenceFixture` with
  `FLCLASH_FLUTTER_BACKUP_FIXTURE`; use the same variable for the Flutter test.
  Retained metrics are `internal/store/testdata/flutter-backup-restore.json`.

These continuation runs used the locally available **Flutter 3.47.5**, not the
source workflow's **3.47.6**. Their fixtures/docs record the mismatch. All tests
remain safe/isolated: no original app startup, system takeover or workstation
preferences. Restore tests use owned temporary paths/in-memory databases and
forbid HTTP; binary provider fixture bytes are synthetic, not MRS decode proof.
Always restore both native asset-hook settings and remove copied/generated
harness files from a disposable checkout after reference generation.
