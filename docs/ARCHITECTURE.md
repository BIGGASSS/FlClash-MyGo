# Architecture and provenance

The upstream reference is FlClash `7fd417fb9a9a277c6a9339a8ac87f907df7c25f7`, with its mihomo fork pinned as a submodule at `c5062921054902b25215a6a933e72ccd714c9bfa`. Mobile source in the inherited core is retained to minimize unnecessary changes; it has no build target in this application.

MyGo is pinned at `41ec785c253caacb03861dfa0a1099e670144c8f` (`v0.7.2-0.20261027143928-41ec785c253c`). Its Go source, native shaders, toolkit, and documentation are under `third_party/mygo`; the module replacement uses that snapshot. Its npm tooling, website, and example applications are not application dependencies.

The local MyGo change adds `Theme.ContinuousCorners`, which opts this app into continuous corners on Linux and Windows. MyGo's default platform behavior remains unchanged. The paint, SVG, material, clip, shadow, and embedded-surface paths use the same setting. This avoids simulating FlClash's surfaces with web content.

## Application layers

- `cmd/flclash`: desktop entry point and safe/headless rendering options.
- `internal/app`: main-thread UI state and native views. Background results are committed through `Window.Update`. Lists and proxy grids are virtualized.
- `internal/app/engine.go`: serializes and converges core lifecycle requests; revisions keep superseded results out of the UI. Settings are assembled off the UI thread. Invalid config application can retain the last working config.
- `internal/coreclient`: framed, little-endian JSON RPC over a private Unix socket or an ACL-restricted Windows named pipe. Replies can arrive out of order. Cancellation, disconnects, spilled responses, and process ownership are handled here.
- `internal/store`: private JSON preferences, atomic file replacement, YAML edits, subscription metadata, and bounded ZIP/WebDAV backup handling.
- `internal/desktop`: system proxy capture/restoration and process resident-memory measurement.
- `core`: the original Go core, built as its own executable. `Clash.Meta` remains a pinned git submodule.

## Core additions

`core/mygo_config.go` adds transactional config application, full config validation, and listener checks. A parse failure is returned before replacing the applied config. Listener checks prevent reporting a failed bind or TUN startup as a successful start.

`core/owner_mygo_unix.go` tells an elevated child which desktop user owns the data. The existing ownership checks still require the home directory to belong to that user. The host keeps ownership of the IPC listener, and the core exits when that connection closes. Shutdown never finds or kills other clients' processes by name.

The provider side-load request accepts an optional base64 body, keeping binary MRS files byte-exact through JSON IPC while preserving the existing text payload.

`core/filter_test.go` reads `assets/filters.json`, the presets ported from the Dart model, instead of requiring a Flutter source tree at test time.

## Data and settings

Subscription source YAML is kept separately from the effective runtime config. Native section edits patch a YAML document node, preserving comments and untouched fields. Reality `short-id` scalars retain their original string spelling. Added rules precede the subscription rules; custom proxies and groups merge by name; profile overrides take precedence over global overrides.

The settings format is versioned independently of Flutter's Drift database. Backups validate paths, profile identities, expansion sizes, and YAML before activation. Restore uses new profile and resource identities so an interrupted import cannot overwrite an active profile. Portable provider references resolve into the current data directory, including subscription profiles used by custom groups.

Flutter backup import reads Drift SQLite snapshots in read-only immutable mode with `modernc.org/sqlite` v1.43.0. The pure-Go SQLite implementation preserves the CGO-free desktop build matrix and accepts Drift constraints without rewriting schemas. Legacy v0 JSON entities normalize through the same importer. Selected override keys, rule ordering/disabling, scripts, proxy dialers, and local resources are mapped; original config/database rows remain in `importSnapshot` to retain unknown fields. The synthetic v21 fixture can be regenerated with `internal/store/testdata/make_flutter_fixture.py`.

Scripts execute using the same QuickJS-NG 0.15.1 sources bundled by FlClash's `rquickjs-sys` 0.12.2. A committed WASI reactor runs under Wazero v1.9.0 without cgo. Each evaluation has a fresh runtime, the original 256 MiB QuickJS heap limit, a bounded Wasm memory, and a ten-second cancellation deadline. The minimal ABI installs console methods and drains promise jobs; no filesystem, process, timer, or network bindings are installed. Application settings apply after the source transform, matching upstream setup ordering. See `third_party/quickjs/UPSTREAM.md` for source hashes and the reproducible build command.

## Visual reference and theme

The reference harnesses under `tool/reference` mount the pinned Flutter widgets in safe widget tests, with synthetic data. `docs/reference` contains their light/dark renders. These are widget references, with the memory/file-count limits documented by the harness; they are not OS integration evidence.

`internal/theme` reproduces FlClash's HCT calculations through a bundled, pure numerical Material Color Utilities 0.3.0 helper executed by the same isolated QuickJS reactor. Native MyGo draws every interface element. The helper has no UI or web bindings. The source is in `internal/theme/source`, its npm pins and build command are under `tool/theme`, and the Google Apache-2.0 notice is preserved. The Go corpus compares every theme role against 160 cases exported directly from the pinned Dart implementation. Presets keep normal theme changes out of the script runtime.

Roboto's regular/medium/bold faces align the Linux and Windows defaults with Flutter. The macOS default remains its system font; the user can choose an installed family. The original JetBrains Mono and Twemoji faces are also registered.

The IP-quality package retains the original six source parsers, wave ordering, explicit-type preference over inferred residential results, cancellation, status reporting, and TTLs. Requests go through the core's probe API, so they use its outbound routing even when the mixed listener is disabled. Successful lookups cache privately for three days; failures stay in memory for the five-minute retry cooldown.

Desktop shortcuts use the original twelve action identities. Flutter USB HID bindings convert to native accelerators; unsupported physical keys/modifiers remain visible and produce a registration error instead of silently changing their meaning. Old native action aliases migrate on load and restore. The recorder suspends global registrations, validates primary modifiers (including Shift alone), and transfers conflicting combinations when saved.

The native icon renderer uses all 128 glyph definitions and desktop control states exported from the pinned Flutter source, including outline/filled forms and detail cutouts. SVGs are parsed once as used and rendered by MyGo. `tool/reference/port_glyphs_test.dart` documents the reproducible export; the only approximation is the superellipse-box flattening tolerance of 0.001 on the original 24-unit grid.

Service preferences keep the original thirteen-target order and disabled list, including Flutter import and native backup. A core sweep receives its original ten-second per-check budget and a 65-second transport guard. Per-service serials reject superseded replies. Route checks retain answers whose recorded exits and core epoch still match; pending or newer answers are protected from an older route check. Dashboard cards observe only the services they show; opening the service sheet displays cached rows and leaves unrequested checks pending.

Runtime assembly is compared field by field against 36 configs exported from the original `makeRealProfileTask`. Application-owned listeners, authentication, controller settings, process/log defaults, TUN controls, interface clear/follow/custom behavior, DNS baseline and merge rules, private-route bypass, system DNS, host/policy lists, and sniffer port normalization follow that reference. Optional general overrides stay absent until selected; legacy selections retain the old defaults. Native explicit profile overlays remain supported. Derivation never rewrites subscription YAML.

Subscription providers receive per-profile cache paths keyed by name and URL (or section/name for file providers), matching the original namespace and legacy URL-cache migration. Inline and explicit portable library references retain their own storage. The core's home is the private application data root, so its normal safe-path checks accept local profile/resource references without disabling those checks. Core file preparation runs in the engine before launch; older `core/` GeoData and cache files move into that root without overwriting newer files. The real-core lifecycle test loads a portable local provider through this boundary.

Text scaling changes the app's text theme and explicit text sizes without applying
window zoom. Navigation widths and icon hit targets keep their logical sizes.
Sidebar row heights, proxy headers/cards, and dashboard density/heights follow
the original formulas; proxy line metrics are checked against the original
Flutter measurements at 80%, 100%, and 140%. Dashboard layout heights clamp their
scale to at least one, while the text itself can shrink to 80%.

Imported added-rule MATCH placeholders remain unresolved in storage. Each runtime
assembly uses the profile's explicit match target or the last nonempty MATCH in
the current transformed subscription. Sniffer protocol overrides replace whole
entries case-insensitively and carry deprecated sniffing/port-whitelist settings
forward when the profile has no nonempty sniff map. Both behaviors are included
in the original-assembler corpus.

The memory summary and native side sheet use the core's RSS, heap in use, idle
heap, stacks, and runtime accounting. Runtime totals above RSS never underflow
an “other” bucket. Disconnected cores contribute zero; stopping listeners retains the connected
core's memory. The app's own RSS continues
sampling without starting a core. The release action calls the existing owned
core GC method, keeps its original minimum feedback interval, and rejects a
reply from a superseded lifecycle revision. The real-core lifecycle test checks
the acknowledgement and the returned breakdown.

Subscription and WebDAV workers capture a request client on the UI thread. While
the core is running they use its mixed-port HTTP proxy and first inbound
credential; localhost remains direct. Stopped requests are direct, matching the
original HTTP override. Certificate verification remains the default and the
original user preference survives imports and backups. Subscription requests use
the selected global User-Agent (unless a native profile supplies its own), the
original twenty-second download budget, and at most five followed redirects.

Custom group icons use original-color SVG/image rendering and the source
monogram fallback (first Unicode letter/number, otherwise first grapheme).
Remote byte loads share eight fetch slots, coalesce identical requests, retain
16 MiB in memory, and use private HTTP-freshness/ETag files. Freshness follows
flutter_cache_manager 3.4.5 (seven days by default); the translated behavior's
MIT notice is retained. Disk cleanup keeps at most 200 files, 64 MiB, and 30 days
since access. Raster icons fit within 192 pixels before entering the decoded
cache, preserving JPEG orientation. Additional native decode bounds reject
encoded bodies over 16 MiB or rasters over 16 megapixels; decoded UI resources
have a 64 MiB/128-entry budget. Unsupported/malformed images keep the fallback.
Window.Update commits background results, and quit cancels icon requests.
Safe/headless previews decode inline fixtures and never fetch remote icons.

The core-status control distinguishes IPC availability from listener state. It
uses the original disconnected/connecting/connected forms, confirmation text,
and 600 ms minimum connecting feedback. A confirmed restart is an engine intent;
concurrent requests coalesce, the previous owner closes before replacement, and
the latest profile/listener selection is retained. Superseded config work is
checked before launching listeners or changing system proxy. Loading indicators
use the source one-second comet ring, including its gradient tail and round head.

Icon instances track their native element IDs across builds. Returning to an icon
after scrolling or changing pages rechecks HTTP freshness while retaining the
current image; identical bytes reuse the decoded resource. Encoded/decoded cache
state does not retain native elements, and preview decoding preserves LRU age.

The sidebar follows the original 250 ms standard width curve and clipped,
fixed-width destination labels. Its toggle uses the original interpolated glyph
strokes, and the 320 ms selection indicator stretches its leading and trailing
edges separately. Reduced motion settles both immediately. Source-widget
measurements check hit-target geometry at all three reference text scales.
Dashboard/proxy sizing follows the pane throughout expansion.

Dashboard density uses the full body height, including the area underneath the
floating toolbar, matching the original scaffold. The 1440 × 860 reference
checks the standard-height boundary that previously selected compact cards.

Manual proxy delay tests resolve selectors/computed groups to the same leaf and
innermost test URL used for display, deduplicate targets, and share a 32-request
FIFO queue. Page actions follow search; global actions cover the mode's groups.
Pending holds/runs keep overlapping jobs visible, while results flush every
300 ms and on completion. Profile reset, quit and superseded core intents cancel
owned jobs; their late RPC replies cannot repopulate current UI state. Background
core delay notifications retain upstream's URL/name identities. RPC failures and
null results preserve the previous measurement, and the original thirty-second
transport guard remains. Real-core loopback checks distinguish success, timeout
and failure. Original-widget renders cover the group and per-node progress
states; native tests also exercise pool ordering, cancellation and overlap.

Narrow desktop dashboards use the original single column with a 560-point cap,
source card order, combined traffic/takeover controls, and in-place add/remove/
reorder editing. The start card remains mandatory but movable. Shared backups
retain Android-only card identities in their relative positions without adding
mobile targets. Eight original migration cases check legacy/retired identities,
and native backup preserves the selected order. Card bounds match source widget
measurements at 80%, 100%, and 140%; spacing follows window view mode and the
source's downscaled-spacing clamp. The editor sways cards and respects reduced
motion; its presence, drag-preview and detail-sheet transitions still need work.

The narrow start control uses exported material cubic geometry, interpolates
circle/cookie-9 curves, and paints a cached 2× Gaussian alpha-mask shadow.
The country clover and cookie-12 decoration also use exported source curves.
Traffic charts use the source 35-sample Gaussian window, thirty-sample viewport,
clamped Catmull-Rom slopes, minimum scale and fill. Accepted runtime configs
publish immutable counts to the sampled UI; a failed apply retains the last
working counts. Supplied-provider proxy counts are added for display.

Parameterized dashboard labels evaluate the original generated cardinal rules,
including Dart Intl's explicit zero/one/two parameters and rounded operand.
All 1,694 original generated outputs match across the seven bundled languages.
This formatter is available for the remaining parameterized UI labels; its
addition alone does not verify every locale or screen.


Dashboard profile details open a native navigation sheet. Runtime preview uses
the same effective-config/script pipeline as the engine without launching it.
Nested provider preview/edit pages preserve the dashboard and details scroll
position. File-probe and editor results commit on the UI thread with generation
checks; windowless tests drain the same queued callbacks at their next build.
Provider updates/save requests capture their owned IPC client. A rejected save
keeps the file and edited buffer; successful saves atomically replace the file
and return to details. The provider list remains virtualized. Global dialogs
render above sheets, and sheet actions clear native caption controls.


Linux and Windows use the original 32-point application caption bar rather than
placing toolkit caption buttons over the page toolbar. Pin gets a 32-point slot;
minimize/maximize/close get 46-point slots and exported ten-point vector glyphs.
The maximize element carries MyGo's Windows snap-layout marker. Native maximize
and fullscreen events own the displayed state. Caption controls are a separate
native overlay above body modals, while body layout and modal bounds exclude the
caption height. macOS keeps native traffic lights and its narrow 28-point header.


Desktop windows retain the source 380 × 400 minimum. Above 840 points the saved
sidebar preference controls its expanded/compact width; from 601 through 840
it stays compact and opens an expanded overlay. At 600 or below, primary pages
use the floating or docked bottom navigation preference. Nested small-window
pages show a back action. Tab histories, record filters, and independent editor
buffers survive switching tabs. Native backup and Flutter import retain an
explicitly disabled floating-navigation preference.

Floating navigation uses analytic springs and a quadratic pointer-velocity fit.
Sixty spring values, seven velocity traces, and four dock rectangles are compared
with the original Flutter implementation. Fourteen navigation images extend the
then-reference set to 86 renders. The owned, safe-mode Linux window smoke test now
covers 27 interactions, including minimum sizing, laptop overlay dismissal,
bottom-tab selection, and profile preview navigation.

Async editor loads are scoped to their originating navigation intent. Provider
saves capture their editor, profile, engine revision, and provider path. A save
completion returns to its original tab and preserves text typed during the
request. The added `updateExternalProviderAtPath` RPC and optional side-load
`provider-path` reject a same-name provider whose private path changed. Legacy
name-only update and path-omitted side-load methods remain accepted. Real-core
IPC tests exercise both accepted and rejected provider identities.


Dashboard profile/proxy pickers retain the dashboard under a native navigation
sheet. Proxy pickers own their search, virtual-list/grid, locate and scroll
state; the main page keeps its existing state. Group/node selections remain
shared. Profile cards now use the source grid formulas, selected badge, compact
update text and expiry chips, with virtualized rows for large libraries.
At 600 points or below, these navigation sheets use full-width bottom sheets,
a 68-point handle/header, the 9/16 initial detent and 90% maximum, header dragging,
projected release and a source-derived throw limiter. The compact proxy picker
docks its search at the foot. Eighteen additional original images bring the
reference set to 104. Sheet presentation/dismissal animations, content-driven
dragging/wheel integration and every nested picker action remain incomplete.
