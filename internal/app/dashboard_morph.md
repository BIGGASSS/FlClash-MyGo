# Start-control morph reference

This change ports the non-glass `dashboardStartControl` used by the narrow
start card. It does **not** change `dashboard.go`'s separate wide control,
`dashboard_narrow.go`'s card/decoration, the engine, on-demand decisions, or the
shared glyph exports. It does not establish whole-dashboard or glass parity.

Source: FlClash `7fd417fb9a9a277c6a9339a8ac87f907df7c25f7`,
`lib/views/dashboard/widgets/current_outbound.dart` (`_StartToggle`,
`_MorphPainter`), `lib/ui/icons/app_glyphs.dart` (`playPause`),
`lib/ui/icons/glyph.dart` (`GlyphPolyline`), `lib/ui/navigation_dock.dart`
(`ElasticPress`), and `lib/ui/surface.dart` (`focusRingSide`). These translated
FlClash shapes/behavior remain GPL-3.0, as does the application.

## What is compared

`testdata/flutter-dashboard-morph.json` was exported by running the **original
private widget and painter**, not a reconstructed look-alike. The harness is
appended to a new copy of the source library; the original widget is unchanged.
The successful run used `/home/tim/fvm/versions/3.47.5`, framework `6a19cca564`,
Dart 3.13.4. Upstream's workflow pins **3.47.6**, which was not available here;
that version mismatch is explicit in the fixture. No assertion about 3.47.6
rendering or platform integration follows from this run.

Native tests compare the eight original intermediate/overshooting play-pause
vertex sets and rounded path bounds, twelve samples of both curves, original
forward/reverse/interrupted widget timelines, the press/release spring samples
and painted scale, linear hover-layer colors, and focus-ring dimensions. Native
headless tests additionally check unchanged hit bounds, actual pointer swell,
keyboard focus, safe-mode activation suppression, disabled controls, quick taps,
rubber-banded dragging, and the existing filled `wifiOff` while suspended.
There are no screenshot-specific translations or endpoint icon cross-fades.

The 450 ms shape controller has easeOutBack forward and easeInCubic reverse.
Its curve direction is retained through interruption; completion is strictly
after the endpoint, so reversing at precisely 450 ms also retains that curve.
The independently retargeted 450 ms glyph tween always uses easeOutBack. Its
fold clamps to [0,1], but its 90-degree turn continues beyond both endpoints.
Suspension replaces/remounts that glyph tween, without changing run intent.

The M3 state layer is a 150 ms **linear** tween: focused 0.10, else hovered 0.08,
else zero; pressing adds no extra layer. The source focus ring is width 2,
strokeAlign 2: its unit path scales about center by `1 + 4 / side`, with a
constant two-point stroke. It is not a toolkit rectangle or an offset outline.
The existing 2x Gaussian shadow retains source offset 6, sigma 7, and alpha 0.3;
it transforms with the painted shape, glyph, and ring under the press.

ElasticPress uses mass 1, stiffness 311, damping ratio 0.593, growth
`min(0.25, 16/longestSide)`. A short tap first reaches lift 0.975 before
releasing. Dragging uses the original rubber band (7/32 side limit), 120 ms
critically damped tracking, half-pull anisotropic stretch, and press-spring
return. There is no hover swell here: the source control does not contain an
ElasticHover. All persisted state is numerical `ui.Local` data; no `ui.Element`
is retained between builds. Input remains on ButtonBase and its engine-owned
toggle, while the visual child alone transforms.

### Reduced motion is not the same as zero duration in this source

The source widget uses `motionDuration` only for the overlay. Setting
MediaQuery.disableAnimations alone leaves both morphs at 450 ms. Actual platform
accessibility disableAnimations additionally applies AnimationController's 0.05
factor: **22.5 ms** for both morphs. ElasticPress's direct ticker/springs do not
consult either flag. The native preference follows this measured platform path
(compressed morphs, immediate state layer, unchanged press springs), rather
than incorrectly applying MyGo's blanket instant-animation policy. The fixture
retains both MediaQuery-only and platform-accessibility traces.

Native cancellation resets a press if its profile-enabled state is removed;
source ElasticPress may finish a spring in that situation. The shadow still
uses MyGo's existing cached 2x raster approximation; GPU AA/blur, glass, and
cross-platform focus/gesture behavior have not been pixel-verified. These limits
are not a claim of complete parity.

## Regenerate safely

Use a disposable checkout of the pinned upstream. Disable **both** native asset
hooks in its pubspec, supply its pinned core's CA asset, and run `flutter pub get`.
Then, from this repository:

```sh
python3 tool/reference/prepare_dashboard_morph.py /path/to/disposable-upstream
cd /path/to/disposable-upstream
FLCLASH_FLUTTER_VERSION=3.47.5 \
FLCLASH_DASHBOARD_MORPH_PATH=/absolute/path/to/internal/app/testdata/flutter-dashboard-morph.json \
/path/to/flutter/bin/flutter test --no-pub --dart-define=SAFE_MODE=true \
  lib/views/dashboard/widgets/dashboard_morph_reference_test.dart
```

The helper refuses to overwrite an existing generated library. Discard that
library/check-out after use. The harness never taps the source core action,
starts a core, accesses Wi-Fi, or changes system proxy/TUN. File output is
synchronous because widget tests use a fake asynchronous clock.

## Flutter numerical behavior notice

`dashboardCurve` translates the numerical Cubic evaluation/tolerance behavior
from Flutter's `animation/curves.dart`. Flutter's notice is retained here and
in `assets/data/dashboard-morph-NOTICE.txt` (automatically embedded by the existing
asset glob, without changing the shared asset registry):

Copyright 2014 The Flutter Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.
* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.
* Neither the name of Google Inc. nor the names of its contributors may be used
  to endorse or promote products derived from this software without specific
  prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
