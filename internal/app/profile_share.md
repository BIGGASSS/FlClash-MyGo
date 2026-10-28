# Profile URL QR sharing

The source is FlClash `7fd417fb9a9a277c6a9339a8ac87f907df7c25f7`:
`lib/views/profiles/profiles.dart`, `lib/common/dialog.dart`,
`lib/ui/qr_code.dart`, and `lib/ui/dialog.dart`. Only the subscription profile's
**stored URL** is shared: no YAML, secret-key metadata, import wrapper,
normalization, selection change, download, or core call is involved. Native
legacy subscriptions with an empty `Type` retain their URL identity.

`github.com/piglig/go-qr` v0.2.6 is pinned at
`6ddd4948e90b3ee160ddc06f9842bb5a361e7478`. It is pure Go, has no runtime
third-party dependencies, and is used only as an encoder. Existing dependency
pins and the Go directive are unchanged. Its full MIT license and the Dart qr
4.0.0 BSD license for the translated payload/mask behavior are in
`THIRD_PARTY_NOTICES`.

The source encoder defaults to medium error correction, versions 1–40, a single
numeric/alphanumeric/UTF-8 byte segment, and ECI 26 only when a Dart code unit
exceeds 255. Its mask scorer uses eight-neighbor penalties and light
format/version placeholders, not the Go encoder's default scorer. The resulting
module matrices match the original Dart fixture, including Latin-1, Unicode,
emoji, long mixed strings, version boundaries and all four capacity limits.
At medium correction those maxima are 5596 numeric characters, 3391 alphanumeric
characters, 2331 UTF-8 bytes without ECI, or 2330 bytes with ECI.

The native modal follows the source 348-point panel, up-to-240-point square,
white background, black modules, four-module quiet zone, profile title and
localized share/too-long messages. Capacity errors stay in the modal; the
existing Copy link context action remains available and the error modal also
offers a direct Copy link fallback. Unexpected encoding errors are visible,
not silently replaced with another payload. No UI elements are retained.
Escape/backdrop/Close dismiss only the modal, retaining picker scroll, nested
navigation, dirty drafts and the main editor. The application caption remains
outside the modal's body bounds.

## Numeric reference regeneration

`tool/reference/port_profile_qr.dart` calls the unchanged `qr` 4.0.0 package
pinned in the original lockfile (archive SHA-256
`5adccca7f0274326125fcdbf97328f5d6b1b2d7fb8a52a01cef9f40d96a759d9`).
It does not import the app, start Flutter, use native plugins, or access the
network. It writes only the explicit output path. From this repository:

```sh
FLCLASH_QR_PATH="$PWD/internal/app/testdata/flutter-profile-qr.json" \
  /home/tim/fvm/versions/3.47.5/bin/dart \
  --packages=/path/to/resolved-pinned-source/.dart_tool/package_config.json \
  tool/reference/port_profile_qr.dart
CGO_ENABLED=0 go test ./internal/app -run '^TestProfileShare'
```

The available SDK is Flutter **3.47.5**, Dart **3.13.4**, rather than the source
workflow's Flutter **3.47.6**. That tooling mismatch is not concealed; this
fixture exercises the exact pinned pure-Dart encoder, not Flutter widget or OS
integration. Normal Go tests read the committed numeric data independently of
that SDK and source checkout. SHA-256 hashes cover row-major ASCII `0`/`1`
modules without a quiet zone, including the original automatically chosen mask.

## Remaining comparison limits

No original-widget QR dialog screenshots or physical-camera/real-desktop focus
runs were made. Native headless tests cover light/dark, 80/100/140% text,
minimum 380 × 400 windows, caption clearance and modal/picker dismissal. To keep
the full code visible in short native windows, the square can shrink vertically
(as well as horizontally), down to 96 points, while excess caption text scrolls;
the original uses width fitting and a scrollable dialog body. Extremely dense
version-40 codes at a minimum-size window may require enlarging the window for a
camera to read. Native titles use a two-line ellipsis/tooltip rather than an
unbounded source title. Alert/glass styling and route animations are not claimed
as exact parity. Scanner/import-from-camera and unrelated exports are out of
scope.
