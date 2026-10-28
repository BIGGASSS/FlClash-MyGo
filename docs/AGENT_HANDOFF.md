# Agent handoff

The user requested a desktop-only native MyGo port of FlClash with 1:1 UI/UX,
then asked to push all current changes and leave resume steps for another agent.
**Parity is not complete.** Resume implementation; do not present the current
checkpoint as a completed port. The repository is private:
<https://github.com/BIGGASSS/FlClash-MyGo>.

## Start here

1. Read `AGENTS.md`, `docs/ARCHITECTURE.md`, and `docs/PARITY.md`.
2. Inspect `git status --short` and `git log -6 --oneline`. This handoff is
   committed with the dashboard picker work. Earlier checkpoints are `df68a1e`
   (responsive navigation and editor ownership), `90703c3` (desktop caption),
   and `459af76` (profile details and nested provider editors).
3. Build/run safe mode and inspect the actual native views. Compare the matching
   original renders in `docs/reference` with `docs/screenshots`. The reference
   harness is under `tool/reference`; read its README before regenerating.
4. Work through the concrete gaps below, add focused behavior/reference checks,
   update the checklist honestly, validate, commit and push. Continue toward the
   user's original parity goal without repeatedly asking permission.

## Sources and workspace

| Project | Current workspace | Pinned revision |
| --- | --- | --- |
| Port | `/workspace/FlClash-MyGo` | this repository |
| FlClash | `/workspace/FlClash-upstream` | `7fd417fb9a9a277c6a9339a8ac87f907df7c25f7` |
| MyGo | `/workspace/mygo-upstream`, vendored under `third_party/mygo` | `41ec785c253caacb03861dfa0a1099e670144c8f` |
| mihomo | `core/Clash.Meta` submodule | `c5062921054902b25215a6a933e72ccd714c9bfa` |

The source checkout was restored after reference generation. Its pubspec must
keep both original `build_assets: true` settings. The local exact backup is
`/workspace/tooling/flclash-reference-pubspec.yaml`. Do not commit temporary
Flutter hook changes or copy modified UI widgets into the reference checkout.

Go is `/workspace/toolchains/go/bin/go` (1.27.3); the system `go` command is
unrelated. Flutter 3.47.6 is `/workspace/toolchains/flutter`. Cache directories
are `/workspace/go`, `/workspace/go-cache`, `/workspace/cache`,
`/workspace/pub-cache`, and `/workspace/npm-cache`. These absolute paths are
workspace conveniences, not repository build requirements.

## What the latest work contains

- Responsive navigation: expanded/compact sidebar above 840 points, compact
  sidebar with expanded overlay at 601–840, floating/docked bottom navigation
  at 600 or below, and a 380 × 400 minimum window. Tab histories and dirty
  editor buffers remain independent. Navigation springs and pointer velocity
  have original Flutter numerical fixtures.
- Original Linux/Windows caption geometry and controls; macOS keeps its native
  traffic lights. Modal bodies stay below the caption.
- Profile details preserve the dashboard. Runtime preview shares the engine's
  config/script assembly. Nested provider preview/edit rejects stale results
  and preserves buffers/files when the core rejects a save.
- Provider writes capture profile, engine revision and private provider path.
  The added guarded update RPC rejects a replacement provider with a reused
  name. Existing wire methods remain compatible.
- Dashboard profile/proxy picker sheets now preserve the underlying dashboard.
  Proxy picker search/scroll/grid state is separate from the main proxy page;
  actual group/node selections are shared. Profile selection leaves its picker
  open, matching the source.
- Profile cards use a virtualized source-style grid with a three-column cap,
  300-point adaptive minimum, source row heights, selection badge, compact update
  text and expiry chip. The old large icon/inline action rows were replaced.
- Navigation sheets use full-width bottom sheets in small windows, a 68-point
  handle/header, initial 9/16 detent, upper 90% limit, header drag, projected
  release, spring settling and source-derived throw limiting. The small proxy
  picker docks its search at the foot.
- There are 104 original Flutter reference images: 90 in the main harness plus
  fourteen navigation renders. The latest eighteen cover profile grids and
  profile/proxy pickers, including small windows. Fixtures are synthetic;
  original memory/file counts and some underlying dashboard values differ from
  the native fixtures. Use measured bounds and inspect the actual controls.

Key files for this checkpoint:

- `internal/app/dashboard_picker.go`: picker ownership, search and sheet content.
- `internal/app/profile_grid.go`: virtualized profile card rows and columns.
- `internal/app/navigation_sheet.go`, `snap_sheet.go`: side/bottom layout and
  current header drag implementation.
- `internal/app/profile_details.go`, `profile_providers.go`: details, preview,
  provider files and saves.
- `internal/app/navigation.go`, `editor_context.go`: tab history/editor ownership.
- `internal/app/dashboard_picker_test.go`: picker state, compact geometry,
  upper-detent/dismiss gestures and screenshots.

## Recommended next implementation steps

1. **Complete picker navigation and profile actions.** A profile card's context
   Edit currently closes the picker and opens the main editor. The original
   nests editing/custom/extend/preview navigation in the sheet. Port those paths
   while retaining the dashboard, scroll position and dirty buffers. The custom
   profile dashboard card should open the original custom-profile sheet rather
   than ordinary profile details. Compare all context actions, rename, link/QR
   sharing and group-count chips with `lib/views/profiles/profiles.dart` and
   `lib/views/dashboard/widgets/profiles.dart`.
2. **Complete sheet motion and input.** Current side/bottom presentation and
   dismissal are immediate. Side sheets need 300 ms enter / 200 ms exit with
   `Easing.standardDecelerate`; bottom sheets need the source presentation
   spring. Header dragging is implemented, but source content-scroll handoff,
   wheel latching, fit-content remeasurement, stacked sheets, footer lifting,
   cancellation/blur and nested transitions still need work. Inspect small proxy
   footer clipping, shadow, focus and search transitions. Source implementations
   are `lib/ui/side_sheet.dart`, `snap_sheet.dart`, `sheet.dart`, and
   `scaffold.dart`. Preserve reduce-motion and caption bounds.
3. **Finish dashboard card geometry and flows.** The wide outbound/profile/info
   cards still differ substantially; the newer narrow cards are closer. Reuse
   the source density/layout formulas rather than adding screenshot-specific
   offsets. Missing work includes exact play/pause morph/press/focus behavior,
   editor drag/presence transitions, source skeleton/empty states, and the
   remaining detail/custom-profile paths. Profile picker entry points and proxy
   group sheet entry points now exist.
4. **Finish specialized native forms and pickers.** Current nested forms preserve
   data, but source presets, multi-select, icon/font selectors, schema editors,
   MatchTarget controls and appearance preview choices are not all matched.
   Provider subscription dialogs are still generic text; app-managed remote
   rule-set sync and bulk failure presentation are incomplete.
5. **Finish remaining features and platform verification.** See `docs/PARITY.md`:
   optional Liquid Glass (vendored MyGo glass plugin is not yet used), Wi-Fi
   on-demand, service-node outbound details/shared route cache, Flutter-compatible
   backup export, real older Drift archives, script edge cases, and all-locale
   screen overflow/accessibility. Real macOS/Windows networking, tray, elevation,
   startup, sleep/resume, packaging/signing/notarization and update delivery have
   not been exercised in this Linux workspace. Cross-compilation is not runtime
   validation. Linux system proxy currently uses GNOME GSettings; KDE needs
   comparison with the actual original implementation.

## Validate a change

From the repository root:

```sh
export PATH=/workspace/toolchains/go/bin:$PATH
export GOPATH=/workspace/go GOCACHE=/workspace/go-cache XDG_CACHE_HOME=/workspace/cache
gofmt -w <changed-go-files>
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
GOOS=linux GOARCH=amd64 bash scripts/build.sh
CGO_ENABLED=0 FLCLASH_TEST_CORE="$PWD/build/linux-amd64/flclash-core" \
  go test ./internal/coreclient ./internal/app \
  -run 'TestRealCore|TestEngineLatestIntentAndFailedConfig' -v
```

When changing core code, also run its tests and vet from `core`, rebuild it, and
rerun the real-process IPC tests against that new binary. Maintain all six
desktop UI/core targets with `scripts/build.sh` (`linux`, `darwin`, `windows` ×
`amd64`, `arm64`). The workflow packages only desktop targets.

Loopback test servers and the Flutter test runner need network-enabled tool
execution in this sandbox. Preserve the environment's proxy and CA trust.
Read the managed cloud environment skill when starting a fresh managed session.

Run targeted race tests when changing asynchronous UI ownership. Windowless
worker results queue until `App.flushUpdates` / the next `View`; use the existing
`waitApp` helper rather than reading UI state concurrently.

For screenshots, use `FLCLASH_SNAPSHOT_DIR` with `TestPopulatedNativeScreens`,
`TestNativeNavigationReferenceScreens`, and `TestNativePickerReferenceScreens`.
The owned Linux GUI smoke command is:

```sh
python3 tool/native/smoke.py \
  --binary "$PWD/build/linux-amd64/flclash" \
  --xvfb /workspace/tooling/xvfb/usr/bin/Xvfb \
  --output "$PWD/build/native-smoke-next"
```

It uses safe mode, temporary seeded data, its own Xvfb and process groups. Its
27 checks cover captions/navigation/profile preview; the new picker gestures
also have headless tests. Do not toggle the workstation's proxy/TUN, start the
original app normally, or kill unrelated processes.

## Current verification and push

The handoff checkpoint was checked with root tests/vet, real-core IPC, focused
picker race tests, the six desktop UI/core builds, eighteen original picker
renders and the 27-check owned Linux safe-mode window smoke test. Earlier
`df68a1e`, `90703c3`, and `459af76` GitHub Desktop runs succeeded. Check the
workflow for this commit after pulling; GitHub is the source for its latest
status. Local logs under ignored `build/` are retained only in this workspace.

Push using the connected credentials; do not print tokens:

```sh
git -c 'credential.helper=!gh auth git-credential' push origin main
gh api repos/BIGGASSS/FlClash-MyGo/actions/runs \
  --jq '.workflow_runs[:4][] | {head_sha,status,conclusion,html_url}'
```

Keep GPL/MIT/BSD notices, pinned revisions, source YAML/unknown fields and the
last working config intact. Do not retain `ui.Element` objects across builds.
MyGo list/grid/editor state and scalar geometry are safe to retain. All worker
results must return through `Window.Update` / the app's queued update path.
