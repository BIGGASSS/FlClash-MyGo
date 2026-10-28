# Read-only desktop Wi-Fi SSIDs

The native on-demand reader has no connect, scan, location-prompt, proxy or TUN
operations. It is created only when a non-safe app has an initialized engine and
at least one excluded SSID. The app polls every three seconds, with five-second
query contexts, and cancels on exclusion changes, engine replacement, final
removal and quit. A canceled monitor is rearmed only on a live engine; queued
results from its previous owner are discarded even before the next sync.
Results commit through Window.Update (the queued headless equivalent in tests).
This is polling rather than upstream's connectivity-event subscription.

- **Linux:** generic netlink nl80211 GET_INTERFACE and GET_SCAN. GET_SCAN reads
  the existing BSS cache; it does not trigger a scan. Only associated/joined
  entries count. This works independently of NetworkManager/iwd/ConnMan, but
  requires kernel/driver nl80211 support. No legacy wireless-extension fallback.
  Kernel permission failures and unavailable APIs are reported, not guessed.
- **macOS:** dynamically loaded CoreWLAN, with an explicit CoreLocation
  authorization check on macOS 14+. No authorization request/prompt is
  implemented. A not-yet-authorized app must report that limitation; the settings
  UI must not display an inert “Authorize” action as if it requested permission.
  Existing authorization may be used. Authorization and global Location Services
  are checked again after the blocking query, so observed revocation cannot turn
  a redacted result into a disconnection. The reader uses CoreWLAN's `ssid` text,
  like the source, rather than replacing invalid UTF-8 in `ssidData`; this retains
  CoreWLAN's legacy WinLatin1 decoding. CoreWLAN still cannot distinguish all
  native errors from disconnection when it returns nil with permission granted.
  Package permission-prompt integration and real desktop authorization remain
  follow-up work.
- **Windows:** lazily loaded system wlanapi.dll, WlanEnumInterfaces and
  WlanQueryInterface(current_connection). Access-denied errors include guidance
  for Windows location settings (notably Windows 11 24H2). Missing WLAN support
  is explicit. Fixed DWORD/WCHAR layouts are the same on amd64 and arm64;
  synthetic wire tests and compile-time interface-layout assertions cover them.
  Connection parsing rechecks the returned state instead of using an SSID left
  in a disconnected/transitioning result. No WinRT permission prompt or settings
  launch is automatic.

macOS/Windows native calls are synchronous and not interruptible. Cancellation
returns promptly, discards late results, and permits at most one unfinished call
per reader; it does not claim to abort an OS XPC/RPC in flight. Linux polls its
nonblocking netlink socket in 100 ms cancellation intervals.

SSID identities use exact, case-sensitive list membership: no trimming, globbing
or normalization. Linux replaces each invalid UTF-8 byte and embedded NUL with
U+FFFD, matching the source's GLib conversion; consecutive invalid bytes must not
collapse to a single replacement. Import, native settings/backup and Flutter
export retain order and source spellings. Failed saves never change active
settings.

The source edit field uses `InputDialog(maxLength: 32)` and Flutter's
`LengthLimitingTextInputFormatter`, which counts grapheme clusters, not runes or
UTF-16 units. The separate store validator currently counts runes; that validation
mismatch is outside the desktop reader and needs a store-side correction.

Exclusion parks listeners through an engine intent, never by direct IPC or proxy
calls, and retains the user's run intent. A manual stop therefore cannot be
undone by leaving an excluded Wi-Fi. A read/permission failure is visible and
does not invent a network transition: the last known association remains in
force. This intentionally differs from the source connectivity manager, which
publishes null on read errors. Until a first successful read there is no known
exclusion (the existing run intent remains in force, like the source's initially
null current SSID); this is not a fail-closed network policy.

Unit validation uses synthetic netlink payloads, blocked/cancellable fake SSID
calls, permission changes during a fake read, Windows wire/layout fixtures,
temporary settings/backup directories and an engine whose loop is not run. An opt-in rebuilt-core IPC test also confirms that parking retains the
owned IPC process and a later manual stop is not undone; it uses synthetic SSIDs,
zero listener ports, and explicitly disabled system proxy/TUN.

Focused reader/lifecycle tests and root vet pass with CGO_ENABLED=0. Desktop
tests cross-compile and the app builds for Linux, Windows and macOS on
amd64/arm64. Cross-compilation does not establish real SSID API or permission-flow
correctness on any platform. No real SSID, permission dialog, system proxy or TUN
was exercised for this feature.
