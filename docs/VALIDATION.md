# Validation

Checks performed in the Linux amd64 cloud workspace with Go 1.27.3:

- Application unit and native UI tests pass. The UI is built headlessly in both appearances at widths 800, 1200, and 1600; populated tests cover the main screens, responsive layouts, text scale limits, and native sheets.
- The real upstream-core IPC test passes, including initialization, config validation/application, proxy data, and clean shutdown.
- The engine integration test passes: a failed config keeps the working profile, and overlapping lifecycle requests converge to the latest stop intent. It uses temporary data, no system proxy, no TUN, and disabled inbound ports.
- Application, store, and IPC race checks pass.
- Root and core `go vet` pass. The retained core's test suite passes after its preset test is adapted to the JSON source used by the native frontend.
- MyGo UI, scene, and raster tests pass with `-short`.

The full MyGo suite has one known failure in this environment: `TestDiffFrameAllocs` reports 2 allocations for a steady frame and 9 while scrolling within laid-out rows, against an expected 2. Running the same test in the untouched upstream checkout produces the same failure. Its assertion is unchanged. The `-short` setting skips the framework's allocation measurements, not the application's UI or integration tests.

The local continuous-corner opt-in was measured with both upstream sample applications, using an overlay of the modified files and the same flags before and after:

| Release-style Linux sample | Before | After |
| --- | ---: | ---: |
| `counter-native` | 9,478,407 bytes | 9,478,407 bytes |
| `hello` webview | 9,498,887 bytes | 9,498,887 bytes |

Screenshots under `screenshots/` come from `TestPopulatedNativeScreens`, using explicit fixture data rather than a live subscription. Regenerate them with:

```sh
FLCLASH_SNAPSHOT_DIR="$PWD/docs/screenshots" CGO_ENABLED=0 go test ./internal/app -run TestPopulatedNativeScreens -count=1
```

Desktop cross-compilation and packaging are checked by `.github/workflows/desktop.yml`. Compilation does not validate native OS dialogs, system takeover, GPU/driver behavior, signing, installation, or visual parity with Flutter. Those remaining checks are listed in `PARITY.md`.

Flutter backup migration tests cover synthetic Drift v21 and v0 archives, settings/metadata, rule precedence and disabled rules, shared scripts, custom proxy dialer loops, local rule sets, and cross-profile provider references across a native backup round trip. Missing files, future versions, malformed SQLite, mixed archive formats, traversal, and dangling references must leave active settings and files intact.

The embedded pinned QuickJS engine is exercised against lexical entry points, async and chained promises, rejection, pending jobs, syntax/stack messages, BigInt/cyclic JSON, native language features, a rejected 300 MiB allocation, concurrent isolated contexts, and 100 ms cancellation of script and microtask loops. Application tests verify transforms happen before final application overrides.

Proxy UI tests exercise both presentations across all layout/card combinations, group persistence, selected-node reveal among 10,000 nodes, timeout filtering with retained selections/groups/final matches, per-test-URL delays, completed-test snapshots, and multi-term search. The core side-load protocol has a base64 option for byte-exact binary providers while retaining legacy text payloads.

The pinned Flutter 3.47.6 checkout passes its proxy-list and current-outbound widget tests (19 tests), and the populated reference harness passes 42 light/dark snapshot cases, including text scaling and memory sheets. Exact color-role comparisons cover ten seeds × four styles × two appearances × two pure-black settings (160 cases). The port's virtual masonry proxy groups retain selected-node reveal among 10,000 nodes.

IP-quality tests cover all six parsers, hosting/mobile precedence, Tor/abuse levels, inferred residential fallback, delayed source waves, cancellation of losing requests, rate-limit/timeout/mismatch statuses, stable failure ordering, masked address UI, and malformed optional cache files. They use synthetic responses and never call public services.

After the geometry, font, theme, switch, and IP-quality changes, the full root tests and vet pass, including real-core IPC/engine checks. The 160-case expanded theme corpus passes (including status containers/outline), and race tests pass for app, store, core client, and IP-quality logic. Outbound address parsers cover all eight original formats; previews and tests do not request public IP services.

Imported auto-update durations retain the original milliseconds, independently of the enabled switch. The subscription scheduler test checks the restored one-hour boundary and legacy native hour fields, preventing the earlier minute/hour mismatch.

The original runtime assembler exports 36 field-by-field cases, including dynamic
MATCH targets and case-insensitive sniffer entry replacement. Proxy measurements
match original Flutter line/header/card metrics at 80%, 100%, and 140% without
window zoom. Deleting or restoring an active profile resets old selections,
probe results, and scroll positions. Memory-sheet tests include stopped/empty
cores, runtime accounting larger than RSS, safe dismissal, and the owned core's
GC acknowledgement and post-release stats. The full root tests, vet, app/store
race checks, and six CGO-free UI cross-builds pass after these changes.

A visible Linux/X11 safe-mode smoke run also passes on an owned Xvfb display:
the default renderer detects the missing GPU and draws the dashboard, sidebar
navigation opens Profiles, the restart confirmation opens and dismisses, the import dialog opens, Escape
dismisses it, and the
memory detail sheet opens. `screenshots/native-linux-*.png` are actual window
captures from that run, with fresh temporary data and no core/network takeover.
This verifies that path in this container; it does not establish real desktop
integration or Windows/macOS runtime behavior.

Reproduce with Xvfb, ImageMagick, Tesseract, dbus-run-session, libX11 and libXtst:

```sh
python3 tool/native/smoke.py --binary build/linux-amd64/flclash \
  --output build/native-smoke
```

Use `--xvfb /path/to/Xvfb` when it is not on PATH, or `--cpu` to force the CPU
renderer. The script owns a new display, waits for window/text state, drives
mouse and Escape events, and cleans up only its own process groups.

Custom-icon tests cover original SVG colors, hidden/plain/filled presentation,
Unicode/emoji monograms, 192-pixel raster sizing, JPEG orientation, bounded
encoded/decoded caches, eight concurrent requests, shared-source coalescing,
HTTP freshness and ETag revalidation, disk reuse, cancellation, malformed data,
and stale fallback on refresh errors. Original custom SVG/PNG widget renders
are included in the 72-render reference set. HTTP tests verify running/direct
routing, first proxy credentials, localhost bypass, five redirects, and explicit
certificate preferences through a native backup round trip.

The core-status control is compared separately in disconnected, connecting, and
connected states. Its 600 ms connecting hold and restart confirmations are
exercised through the native UI clock. The real-core lifecycle test coalesces
restart requests with a newer stopped intent, verifies the previous connection
closed, and checks proxy/tool/memory access on the replacement while listeners
remain stopped. Restart and config work check for superseding revisions before
listener and system-proxy transitions.

The current visible Linux smoke sequence passes twenty checks, including
sidebar expansion/collapse, memory-sheet dismissal, resizing into the narrow
dashboard, entering/exiting its editor with Escape, and returning to the wide
grid. The edit click follows the core-label geometry so native window controls
can reserve their own area. A native pixel assertion also checks that the
status decoration cannot cover the start button, alongside its 88-point hit
target. Narrow card bounds match the original at 80%, 100%, and 140%, with
custom order and memory-card coverage. A real pointer drag reorders cards;
add/remove, mandatory status, platform-identity retention, persistence, backup,
Escape, and resize behavior are exercised.

The original dashboard-setting migrator supplies eight cases. The generated
Intl catalogs supply 1,694 exact plural-message cases across all seven locales;
all match, including explicit-number branches, French zero, Russian cardinal
forms, fractions and negative counts. Accepted config counts survive failed
applies in the real-core test. Full root tests/vet, focused app/store race
checks and six CGO-free UI cross-builds pass for this checkpoint. These checks
continue to leave the remaining visual/interaction and OS gaps in PARITY.md.


Profile details retain the dashboard behind a native side sheet. Source renders
cover its normal/subscription/provider forms in both appearances. Native checks
exercise runtime assembly with scripts/overrides without starting a core or
rewriting source YAML, nested preview/search/back/Escape behavior, dirty-edit
cancel/discard, and provider saves accepted/rejected by a framed IPC fixture.
Provider file probing follows the original 512-byte zstd/NUL rules; text previews
validate UTF-8, and reads remain bounded. Ten thousand provider rows render
through a virtual list. Superseded updates cannot report errors against the
current profile. Duplicate profile downloads keep one request and clear loading
on failure. Focused race checks pass. Original compact-count fixtures match.

The visible Linux check also opens profile details, enters the runtime preview
with the keyboard, returns inside the sheet, and dismisses it. Sheet header
actions reserve the native caption-button area; a headless caption-inset check
covers the overlap that visible testing exposed. Screenshot subprocesses have
a bounded timeout. Native OS dialogs and tray integration remain unverified.


The desktop caption bar has eight original light/dark state references, plus
exported glyph geometry and source hit-target rectangles. Native checks exercise
pin/unpin, minimize, event-driven maximize/restore, leaving fullscreen and close
through a caption host. The caption stays available above modal sheets; sheet
and dialog geometry stays below the caption band. The Linux visible check closes
the owned test window through the new caption button. macOS retains native
traffic lights; real macOS/Windows event and snap behavior still need OS testing.

All 160 original palette cases pass with the added on-error role. The older
320-case description was a counting error; ten seeds × four styles × two
appearances × two pure-black states is 160. Full root tests/vet, real-core IPC,
focused caption race checks and all six CGO-free UI builds pass at this checkpoint.
