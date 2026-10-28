# Agent handoff

The user asked to continue the previous agent's work until desktop 1:1 parity.
**Full parity is still not complete. Do not claim otherwise.** This continuation
implements and tests many of the previous concrete gaps; remaining work includes
specialized forms, transitions, real desktop permissions/networking, packaging,
and source shader fidelity. See `docs/PARITY.md` for the current checklist.

## Start here

1. Read `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/PARITY.md`, and this file.
2. Check `git status --short` and `git log -3 --oneline`. This continuation is
   committed as `feat: advance desktop parity and document agent handoff` on
   `main`; no push was requested. A fresh checkout should be clean. Preserve any
   subsequent agent/user edits. The imported history starts at `2fec3d8`;
   earlier hashes mentioned by the previous agent are not available here.
3. Read the feature-specific notes linked below before changing ownership or
   data conversion. Run safe/headless tests and compare actual native captures,
   not just test labels. Cross-compilation is not runtime validation.

## Workspace and provenance

- Port: `/home/tim/code/FlClash-MyGo`.
- Pinned FlClash: `7fd417fb9a9a277c6a9339a8ac87f907df7c25f7`, restored read-only
  reference checkout under ignored `build/upstream`.
- MyGo: unchanged vendored pin `41ec785c253caacb03861dfa0a1099e670144c8f`.
- mihomo: initialized `core/Clash.Meta`, unchanged pin
  `c5062921054902b25215a6a933e72ccd714c9bfa`.
- `go` on PATH is Go 1.27.3. Root/core remain CGO-free for all six desktop targets.
- Available Flutter is `/home/tim/fvm/versions/3.47.5`; the source workflow pins
  **3.47.6**. New reference fixtures explicitly record this mismatch.
- Use a disposable source copy and follow `tool/reference/README.md` for Flutter
  harnesses. Both original `build_assets: true` settings must be restored; never
  alter original UI widgets to manufacture reference agreement.
- `build/`, toolchains and local logs are ignored conveniences, not required
  checkout contents. A new workspace must initialize submodules and recreate
  the optional reference checkout before running original-source comparisons:

  ```sh
  git submodule update --init --recursive
  mkdir -p build/upstream
  git -C build/upstream init -q
  git -C build/upstream fetch --depth=1 https://github.com/chen08209/FlClash.git \
    7fd417fb9a9a277c6a9339a8ac87f907df7c25f7
  git -C build/upstream checkout --detach FETCH_HEAD
  ```

  Ordinary builds/tests use committed fixtures; original-source comparison
  tests may skip without that checkout. Do not interpret a skip as fresh
  reference validation. Install the documented Linux GUI libraries/Xvfb tools
  if GUI smoke is needed; do not depend on previous `/tmp` directories.
- Two additional pure-Go dependencies are pinned in `go.mod`: Uniseg v0.4.7
  (grapheme validation) and go-qr v0.2.6 (QR encoding). Full notices are retained
  in `THIRD_PARTY_NOTICES`. No existing dependency pins were changed.

## Implemented in this continuation

### Picker/editor ownership and sharing

- Profile picker context actions now edit/customize/extend/preview **inside the
  sheet**, retaining the dashboard, scroll position, parent drafts and independent
  dirty buffers. Custom dashboard profiles open the custom-profile sheet.
- Source edits stage until a guarded save. Rejected/conflicting saves retain
  files and buffers; newer input is not marked saved by an older completion.
  Stale discard confirmations and asynchronous loads cannot affect replacements.
- Rename, link copy, custom group-count chips, source MatchTarget choices,
  unknown-target retention and follow-at-runtime behavior are implemented.
- QR sharing preserves the verbatim URL, source medium error correction, ECI,
  capacity and mask scoring. Twenty-four original Dart matrices match exactly.
  Oversized content gets a copy-link fallback. QR dismissal preserves picker
  navigation/dirty edits. Scanning is not added by this feature.
- Key files: `dashboard_picker.go`, `profiles.go`, `editor_context.go`,
  `profile_grid.go`, `profile_match_target.go`, `profile_share.go`, and their
  regression tests under `internal/app`. See `profile_share.md`.

### Sheet motion/input

- Side enter/exit uses original 300/200 ms controller/easing; bottom presentation
  uses the source spring. Dismissal retains the owner/content until exit and
  respects dirty/save guards; obsolete exits cannot close reopened routes.
- Header dragging, safe bare-content edge handoff, wheel-turn latching, blur and
  covering-modal cancellation, editor/control input exclusion, profile-grid
  refitting, and pulled-footer clipping/preservation have tests.
- Search/Find now focus correctly when opening/nesting animated pages.
- Key files: `navigation_sheet.go`, `snap_sheet.go`, `navigation_sheet_*test.go`,
  `snap_sheet_input_test.go`, and existing picker/provider/navigation tests updated
  to advance deterministic animation clocks.
- Still missing: control-started/reversed-scroll arbitration, content fling
  inertia, general fit-content measurement, source footer-only lifting, stacked
  scaling/14-point peek, nested/page/side-drag-back transition fidelity. Older
  service/quality/memory sheets still use their immediate presentation helper.

### Dashboard and probes

- Wide status/traffic/profile/LAN layouts use source density/metrics, including
  font-leading allocation, button slots, foot-pinned statistics and usage meters.
- Narrow and wide start controls now share exact intermediate play/pause paths,
  source curves/reversal timelines, press/pull springs, state overlay and focus
  ring. Reduced motion follows measured source behavior; Gaussian shadow/AA
  remains a native approximation. See `dashboard_morph.md` and its original
  numerical fixture/export harness.
- Service nodes show resolved outbound IP/country and open nested quality
  details. Dashboard/node probes share target entries, per-IP quality jobs,
  captured IPC identities, request serials and route/epoch/version verification.
- Hidden/canceled/superseded queued results cannot land. Network/resume observers
  invalidate host-dependent answers; cache persistence is serialized privately.
- Key files: `service_outbound*.go`, `services.go`, `ip_quality.go`,
  `dashboard.go`, `dashboard_info.go`, `dashboard_morph.go` and focused tests.
- Remaining: every generic RoutedProbe scheduler behavior, dashboard skeletons,
  editor presence/drag transitions, some mode/takeover/service/table layouts,
  extreme-width action caps and LAN's active decoration.

### Remote rule resources and subscription dialogs

- Captured-client, bounded/cancellable app-managed rule downloads preserve
  original source and compiled/MRS bytes. Compilation/staging and atomic writes
  reject stale identity/revision results, preserve the last working file on
  failure and collect bulk errors.
- Provider subscription dialogs use source traffic/expiry sections globally,
  including resources/proxy entry points without an open profile sheet.
- Key files: `provider_sync.go`, `provider_subscription.go`, `providers.go`,
  `profile_providers.go`, `internal/store/{rule_resources,resource_write}.go`.
  Storage contract/limits: `internal/store/RULE_RESOURCES.md`.
- Resource-wide scheduling/editor flows and complete visual parity remain open.

### Flutter backup export

- Local backup offers an explicit “Backup · FlClash (Flutter)” format, separate
  from unchanged native ZIP/WebDAV. UI snapshots settings, reports errors and
  atomically writes private files. Unsupported native combinations fail rather
  than silently losing data; native backup remains the lossless recovery format.
- Export emits actual v1 archive paths/Drift v21 tables and preserves supported
  profiles, scripts, choices, metadata, unknown JSON and custom local resources.
  Identity/path/quota/symlink/precedence/conflict regressions are covered.
- The **original** parser and `BackupAction.restore` passed four safe tests on a
  synthetic export: parser, compatible all-data, override all-data, profiles-only.
  Original DAOs/settings/file bytes and SQLite integrity were checked; no HTTP,
  app startup, core or networking manager was used. SDK was 3.47.5, not 3.47.6.
- Key files: `internal/store/flutter_export*.go`, `flutter_snapshot.go`,
  `flutter_schema.sql`, `flutter_backup_reference_test.go`,
  `tool/reference/port_backup_test.dart`. Retained metrics:
  `internal/store/testdata/flutter-backup-restore.json`.
- Read `internal/store/FLUTTER_BACKUP.md` for unrepresentable layouts (notably
  file/URL source file providers and app-managed source metadata). Real archives
  from older Drift versions remain unavailable/unverified.

### Wi-Fi on demand

- General → On demand edits exact SSID identities (add/rename/select/delete/
  reorder) transactionally. Validation counts grapheme clusters; safe mode never
  creates a Wi-Fi reader or engine. Status/error panels preserve a usable list
  viewport at the minimum window size.
- Linux uses read-only nl80211 cached association queries; Windows uses WLAN;
  macOS uses CoreWLAN with explicit permission checks. Polling/cancellation is
  bounded; late/canceled/replaced-engine results are rejected.
- Exclusion parks listeners through the engine without clearing user run intent.
  Manual stop cannot be undone by a Wi-Fi change. Provider reconciles/probes use
  the projected listener intent too. Dashboard shows suspended/Wi-Fi-off state.
- Key files: `internal/app/on_demand*.go`, `internal/store/on_demand.go`,
  `internal/desktop/wifi*.go`. Notes: `internal/app/on_demand.md` and
  `internal/desktop/wifi.md`.
- OS permission prompts/settings flows and real API behavior remain unverified;
  synthetic-SSID real-core tests disable system proxy/TUN and listener ports.

### Appearance

- Native installed-font picker replaces the generic dropdown: search/sort,
  virtualized previews, localized default, keyboard navigation, saved checks,
  source no-dismiss-on-selection, side sheet/compact-page behavior and tab state.
  See `internal/app/font_picker.md` for toolkit enumeration limitations.
- Optional `none`/`standard`/`ultra` glass renders with MyGo's native material,
  source-exact tint/platter colors and accessibility fallback. Both enabled
  qualities share one material; original refraction/frosting/ultra-warp shaders
  are not implemented. See `docs/GLASS.md`.
- There are now 112 original images: previous 104 plus four glass fallback and
  four font-picker images. Supplemental numerical QR/morph/backup references
  are not additional full-screen parity evidence. New runs used Flutter 3.47.5.

## Validation and captures

Stable final validation passed at
`build/validation-native/20261028T145958.007929Z/summary.json`: all six UI/core
builds (12 executables), root tests/vet, core tests/vet both normally and with
`with_gvisor`, real-core IPC/engine/provider/outbound tests, and all 27 Linux GUI
smoke assertions on both default and CPU renderers. The source fingerprint did
not change during any command. The synthetic-SSID real-core parking test also
passed explicitly. Focused ownership/race tests pass (race instrumentation uses
CGO_ENABLED=1; all production/test matrix commands use CGO_ENABLED=0).

`build/parity-commit-validation.log` and `build/parity-race-final.log` retain
local logs (`build/parity-final-validation.log` is the earlier stable run). Matching affected native images in `docs/screenshots` were refreshed,
including six new font-picker captures. Native inspection caught and removed a
pane-wide focus ring without suppressing the controls' keyboard focus rings.
The application/core code and tracked runner are unchanged since that run;
only handoff verification details were updated afterward. The final run used
`tool/native/validate.py --phase all`, so reproduction no longer depends on the
ignored orchestration script or prior workspace logs.

```sh
mkdir -p build/tmp
export TMPDIR="$PWD/build/tmp" GOTMPDIR="$PWD/build/tmp"
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
(cd core && CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go vet ./...)
for os in linux windows darwin; do
  for arch in amd64 arm64; do GOOS="$os" GOARCH="$arch" bash scripts/build.sh; done
done
CGO_ENABLED=0 FLCLASH_TEST_CORE="$PWD/build/linux-amd64/flclash-core" \
  go test ./internal/coreclient ./internal/app \
  -run 'TestRealCore|TestEngineLatestIntentAndFailedConfig|TestServiceOutboundRealCoreLoopbackCache|TestOnDemandRealCore' -count=1 -v
```

Owned Xvfb/xwininfo binaries were downloaded to
`build/tooling/native-smoke/root/usr/bin`, verified against Arch repository hashes
and signing keys, and extracted without installing system packages. Never use
the workstation display or change proxy/TUN as a test.

`tool/native/smoke.py` now uses 2× OCR inputs, multiple exact-match OCR modes,
one OCR thread, an icon-free sidebar-title crop and the current two-Tab
Close→Preview focus order. The original 27 assertions/owned-process safety remain.
Run it with the extracted tools on child PATH; preserve proxy/CA environment:

```sh
PATH="$PWD/build/tooling/native-smoke/root/usr/bin:$PATH" \
python3 tool/native/smoke.py --binary "$PWD/build/linux-amd64/flclash" \
  --xvfb "$PWD/build/tooling/native-smoke/root/usr/bin/Xvfb" \
  --output "$PWD/build/native-smoke-final"
```

The reproducible runner is committed at `tool/native/validate.py`; only its
outputs/logs are ignored under `build/validation-native`. On a Linux amd64 host:

```sh
python3 tool/native/validate.py --phase all
# Or run a subset: --phase matrix, --phase checks, --phase smoke.
# Supply --xvfb /path/to/Xvfb if it is neither on PATH nor in local tooling.
```

It builds all six targets, tests/vets root/core (including `with_gvisor`), runs
real-core checks (including synthetic-SSID parking) and the tracked smoke tool
for both renderers. It sets private workspace Go temporary directories, removes
inherited display handles and fingerprints sources to detect concurrent changes.
Exit 0 means passed/stable, 1 means a failed/missing check and 2 means the checkout
changed during otherwise passing checks. `--phase checks` requires the built
Linux amd64 core (the runner's execution host; all six targets are still built).
It never downloads/installs GUI packages automatically. CPU/llvmpipe smoke is not
hardware-GPU or Windows/macOS runtime validation. Headless screenshots can be
regenerated with `FLCLASH_SNAPSHOT_DIR` and the existing native reference suites;
font/morph harness notes document their additional output variables.

## Recommended next implementation order

1. **Unify the older immediate sheets.** `internal/app/services.go:sideSheet`,
   `memory.go` and `ip_quality.go` still use `dialogBase`, unlike
   `navigation_sheet.go`. Port them onto owned animated routes while preserving
   service → IP details stacking, virtual-list positions, dialog ordering and
   guarded provider/editor closure. Compare original `lib/ui/{side_sheet,
   snap_sheet,sheet,scaffold}.dart`; extend motion/stack/focus tests rather than
   removing their deterministic clock assertions.
2. **Finish profile form navigation and discard handling.** Compare original
   `lib/views/profiles/{edit,extend/standard,extend/script}.dart` and
   `CommonPopScope` with `profiles.go`, `editor_context.go`, `collection.go`,
   `navigation.go` and `dashboard_picker.go`. Main-tab Back currently lacks the
   picker's discard guard; Standard/Script separation is incomplete. Preserve
   saved/error/newer-input ownership. Source-conflict checks are optimistic
   snapshots, not a filesystem compare-and-swap. Reuse the picker regression
   cases and add main-tab tests before changing saves.
3. **Specialized forms/presets.** Compare original `lib/features/form` and
   custom-profile editors with `collection.go`, `profile_match_target.go` and
   provider/resource editors. Group/member/icon presets, multi-select, schema
   and per-rule MatchTarget remain generic. Never rewrite untouched YAML or
   replace a rejected working config while matching these controls.
4. **Motion/rendering gaps.** General fit measurement, footer-only lift,
   content inertia and stacked scale/peek live in `snap_sheet.go`/
   `navigation_sheet.go`; remaining dashboard states live in `dashboard*.go`.
   Glass needs source shader tiers rather than merely different saved labels.
   Read each feature note before regenerating source fixtures; prefer the
   workflow-pinned Flutter 3.47.6 to close the recorded SDK mismatch.
5. **Platform/data evidence.** Add real older Drift archives and broader
   script comparison cases without modifying fixtures to hide failures.
   Implement permission prompts using the platform readers' explicit errors;
   then validate Wi-Fi and desktop integration on actual target OSes using
   isolated profiles/machines. Signing/update delivery needs credentials and
   infrastructure, not another claim based on cross-compilation.

## Remaining work and non-code blockers

- Specialized presets/multi-select/custom-icon/schema/per-rule-target forms and
  Standard/Script Extend separation, main-tab draft discard, exact transitions.
- Remaining dashboard empty/skeleton/editor transitions and static control gaps;
  source glass shader tiers, font enumeration edge cases, all-locale/accessibility.
- Wi-Fi permission prompts/event integration and target-desktop API validation.
- Real older Drift archives and broader script/native-Wasm behavior corpus.
- Real macOS/Windows/GNOME/KDE proxy/TUN takeover/restoration, tray/startup/
  sleep/resume/elevation/protocol-install tests. Cross-builds do not prove these.
- Signing/notarization/installer certificates and repository update delivery are
  not available/configured. Do not manufacture completion for these blockers.

Keep original GPL/MIT/BSD notices, source YAML, subscription metadata, per-profile
choices and last-working applied config. Retain only native state/handles/scalar
geometry, never `ui.Element` across builds. Every worker result returns through
`Window.Update`/the queued update path. No mobile targets were added.
