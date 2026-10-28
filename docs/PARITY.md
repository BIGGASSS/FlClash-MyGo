# Desktop parity checklist

The source reference and design target are the current desktop FlClash UI. “Implemented” means there is working native code, not that every animation or platform behavior has been verified against the Flutter app.

| Area | Current implementation |
| --- | --- |
| Desktop window | Native window, source Linux/Windows caption bar with pin and event-driven maximize/restore, responsive sidebar and expanded overlay, floating/docked bottom navigation, per-tab history and editor state, menus, 380 × 400 minimum sizing, close-to-tray, window dimensions |
| Dashboard | Responsive desktop grid and editable narrow single-column layout with saved card order, short-window summaries and capped wide columns, start/stop, outbound IP/type/quality and source details, IP masking, traffic charts/totals/peaks, mode, system proxy, TUN, LAN, profile and proxy picker sheets, services, memory breakdown/release sheet, record tabs, group summaries |
| Proxies | Tabs and collapsible virtual list, density/card/icon modes, multi-term search, region filters, stable timeout filtering, selected-node reveal, per-URL delays, shared 32-request FIFO test queue, search-scoped group tests, queued/running feedback, preferred picks, provider update/import/edit/export, inline/remote SVG and raster group icons, local proxy library |
| Profiles | URL/file/clipboard/share-link import, Age decryption, selection, timed/manual update, subscription usage, dashboard detail sheet with runtime preview and nested provider preview/edit, export/delete, YAML editor |
| Custom config | Native nested list/object forms for rules, proxies, groups, providers and overrides; drag reordering; raw code editor |
| Scripts | Named shared scripts, per-profile selection, inline scripts, pinned QuickJS-NG engine, console methods, promises, 10 s timeout and 256 MiB heap limit |
| Records | Live connections, request traffic updates, DNS queries, logs; search/filter, table/list, pause/resume, details, individual/all connection close |
| Core settings | General, ports, network, DNS, TUN, sniffer, NTP, GeoData, authentication, hosts, providers, extra YAML fields; explicit follow/override controls; runtime assembly compared against 36 original configs |
| Tools | All thirteen service checks, side-sheet management, ordering/enable state, per-service refresh, route metadata/checks, probe, DNS cache clearing, connection reset, provider/GeoData refresh |
| Appearance | Light/dark/system, accent, vivid/tinted/soft/monochrome surfaces, black background, text scale, seven upstream locale catalogs, font selection, and exact HCT/surface calculations |
| Desktop integration | Tray controls and profile/proxy menus, all twelve recorded global shortcut actions, per-action registration errors and conflict transfer, login startup, single instance, packaged deep-link metadata |
| Backup | Native ZIP backup/restore and WebDAV; Flutter v1 SQLite and v0 JSON import, portable local resources and profile references |
| Builds | CGO-free desktop UI and core, macOS app archive, Windows portable archive, Linux tarball; amd64/arm64 |

## Still needed for complete 1:1 parity

- Extend the pinned Flutter widget comparisons beyond the current proxy list/tab, dashboard, appearance, shortcut, IP-details, and service-sheet references. Check the other modals, keyboard focus, reduced motion, screen readers, and locale text overflow. The native and Flutter snapshots still differ in several controls and content details.
- Finish floating-navigation hover/parallax, visibility transitions, optional glass rendering, and real desktop focus behavior. Numeric springs/velocity and static default surfaces have source comparisons.
- Finish sheet presentation/dismissal, content scrolling/dragging and wheel behavior beyond the current header snap gesture. Reproduce all other original transition, context-menu, multi-select, quick-action, preset, custom-icon, and font-selection interactions. Native nested forms cover data editing, but are not identical to every specialized Flutter form.
- Complete service-node outbound IP details and shared route-aware probe caching, Liquid Glass, remaining dashboard details and custom-profile opening paths, and Wi-Fi on-demand behavior.
- Finish nested profile-picker context actions (editing currently opens the main editor), custom profile dashboard sheets, provider subscription-dialog styling, app-managed remote rule-set sync, editor/schema interactions, and sheet transitions; the current profile/provider references cover static geometry and selected navigation paths.
- Compare migrated data against real archives from older Drift schema versions. Current tests use synthetic current-schema and legacy fixtures; unmapped source fields remain in the saved import snapshot. Native backup export does not yet produce an archive that Flutter can restore.
- Extend the cross-runtime script corpus beyond the current language, promise, exception, timeout, isolation, and memory-limit cases. The engine source now matches upstream; Wasm/native stack behavior and host-dependent APIs still need comparison.
- Validate real system proxy and TUN takeover/restoration on macOS, Windows, GNOME, KDE, and different network configurations. The macOS and Windows implementations have been cross-compiled, not run in this Linux workspace.
- Sign/notarize and exercise packaged installers, protocol registration, startup, sleep/resume, tray behavior, and privileged startup on real desktops. Current artifacts are unsigned development archives. Auto-update delivery is not configured for this new private repository.

Mobile packaging and Android-only UI/integrations are intentionally out of scope.
