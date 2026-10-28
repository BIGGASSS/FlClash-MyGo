# Optional native glass

The native preference is `Settings.LiquidGlass` (`liquidGlass` in JSON):
`none`, `standard`, or `ultra`. The default is `none`, as in pinned
FlClash `7fd417fb9a9a277c6a9339a8ac87f907df7c25f7`. Flutter's older boolean
values import as `false → none` and `true → standard`. Native settings,
cloning, native backups, and Flutter backup projection preserve the choice.
Unknown future strings remain saved but render without glass.

Both enabled choices currently use the vendored MyGo `plugins/glass`
material. Keeping the source quality choice does **not** mean the native
application implements its two shader tiers. No vendor changes or additional
window-system effects are used. MyGo supplies Metal, Direct3D, OpenGL, and
software pixel implementations of that material; headless `ui.Tester` exercises
the software implementation.

## Scope and accessibility

The floating navigation dock uses pill-tinted regular glass, its resting
selection uses the source secondary-hue platter, and the lifted selection uses
MyGo's clear glass. The original opaque fills return when the preference is
off. Docked navigation remains opaque, matching the source's Material
NavigationBar. Ordinary content cards and sidebar fills are not glass.

Toolbar wiring uses `a.glassButton(c, element)` (regular tint) or
`a.glassTintedButton(c, element, color)` (source 82% prominent tint), **after**
the last `Background` call. These helpers only use the current build's element.
Appearance UI persists `cfg.LiquidGlass` through the existing `a.save()`.
Both enabled quality choices currently use the same native material; separate
source quality tiers do not exist yet.

The pinned native `ui.Preferences` omits ReduceTransparency. When a real window
exists the policy reads `mygo.Theme.Preferences()`, which exposes the setting
on macOS and Windows; no running-app calls are made by headless rendering.
Reduced transparency or high contrast selects opaque fills without changing the
saved preference. Reduced motion leaves static glass enabled and settles the
existing dock springs immediately. The material adds no independent press or
continuous animation. Linux has no reduced-transparency desktop preference in
the pinned toolkit.

## Compared constants and limits

The color helper uses the same numerical Material Color Utilities 0.3.0 bundle
already used by the application. The 160-case corpus exported directly from
the pinned Dart implementation covers ten seeds, four styles, both appearances,
and pure black. That source-exported corpus also preloads the runtime glass
color cache, so selecting a preset does not run a numerical script on the UI
thread:

- Regular tint: light `#FDFDFD` at `0x8A/255`; dark surface-container HCT
  tone shifted by +7, at 62%.
- Pill tint: light `#F4F4F4` at `0x93/255`; the same dark tint at 48%.
- Resting dock platter: secondary HCT hue/chroma at tone 90 (light) or 4
  (dark), 85% alpha, fading by `1 − lift`.

The 338 × 62-point resting dock at (21, 317) in the 380 × 400 fixture retains
the existing source geometry. Native optics differ: MyGo models macOS glass,
while the source uses its iOS-style pill haze, adaptive contrast, rim,
dispersion, touch glow, and (ultra) content warp. In particular, source pill
haze is 72% with a 15.6-point blur; MyGo's small regular pane computes a
size-dependent blur and a different tone curve. Those shader effects have not
been ported or proven equivalent.

## Reference evidence

`tool/reference/port_glass_test.dart` exports the colors/preferences and four
source widget images: `docs/reference/glass-dock-{light,dark}-{rest,held}.png`.
Run it in a throwaway checkout using the reference instructions, with both
native build hooks disabled:

```sh
FLUTTER_ROOT=/path/to/flutter \
FLCLASH_REFERENCE_DIR=/path/to/output \
flutter test --no-pub --dart-define=SAFE_MODE=true test/port_glass_test.dart
```

These images were generated with the locally available Flutter 3.47.5 (the
source release workflow pins 3.47.6). That renderer reports
`ImageFilter.isShaderFilterSupported == false`, recorded in
`internal/theme/testdata/flutter-glass.json`. They show the **source fallback**,
not GPU refraction parity. Visual comparison confirms matching resting dock
geometry and platter colors but different frosting/tone strength. Native
software images can be regenerated alongside them:

```sh
FLCLASH_SNAPSHOT_DIR="$PWD/build/glass-native" CGO_ENABLED=0 \
go test ./internal/app -run TestNativeGlassReferenceScreens -count=1
```

Tests additionally check live on/off and accessibility fallback, preservation
of navigation state and hit targets, reduced-motion settlement, platter fade,
native/Flutter preference round trips, source defaults, legacy boolean import,
and unknown-field preservation. Real macOS/Windows accessibility changes and
hardware-GPU visual comparison still require target-desktop validation.
