# Flutter-compatible export boundary

`(*Store).FlutterBackup(Settings) ([]byte, error)` is separate from `Backup`.
Native ZIP/WebDAV behavior and its filename remain unchanged. The local backup
page offers an explicit “Backup · FlClash (Flutter)” action. It captures settings
before background work, surfaces export errors, and atomically saves the credential-
bearing ZIP privately. It never silently substitutes this format for native backup.

## Source contract

The target is FlClash **7fd417fb9a9a277c6a9339a8ac87f907df7c25f7**, not an
invented interchange schema:

- `lib/common/backup.dart`: v1 `config.json`, `database.sqlite`,
  `profiles/<integer>.yaml`, `scripts/<integer>.js`, and
  `providers/rules/<md5(integer + "@")>` for local rule providers. Remote caches
  are deliberately absent. No native `settings.json`/`resources/` entries.
- `lib/database/database.dart`, its table parts, and
  `lib/database/generated/database.g.dart`: Drift **user_version 21**, all eleven
  tables, column nullability, primary/foreign keys, defaults, and indexes.
- `lib/models/generated/config.g.dart`, `clash_config.g.dart`, `profile.g.dart`,
  `common.g.dart`, and `lib/enum/enum.dart`: config names, string/list/map
  converters, subscription counters, milliseconds versus SQLite epoch seconds,
  USB-HID shortcuts, and selected override-key paths.
- `lib/models/common.dart`: WebDAV's randomized `v1` obfuscation, **not encryption**.
- `lib/common/indexing.dart`: editable fractional-order integer keys.
- `lib/providers/actions/backup.dart`: restore copies only the files named by
  restored entities. Export never relies on arbitrary extra ZIP files.

Regenerate the committed schema from that checkout with
`python3 internal/store/testdata/make_flutter_schema.py`. No Flutter SDK, cgo,
application launch, system proxy, or TUN is needed for export or these tests.

## Preservation and limits

New imports retain original/native identity correspondence and overlay baselines
inside the existing private `importSnapshot`; native restore remaps them with
its fresh file identities. Unchanged imported overlays retain the original
own/global/disabled rules, group definitions, profile overrides, script URL/time,
subscription JSON, proxy empty-options, and local provider metadata. Custom
source YAML edits or removed library proxies force a rebuild rather than being
hidden by an unchanged overlay baseline. Renamed profile library references
follow identity to the new label, including simultaneous label swaps. Earlier
snapshots without correspondence recover only unambiguous matching labels/URLs;
edited overlays
are rebuilt and need not retain the original disabled-rule IDs/order keys.

Unknown source preferences and unselected JSON option values are retained where
possible. Corrupt/missing identity correspondence and duplicate logical library
identities are errors, not a reason to select a row by position. Earlier-snapshot
recovery requires uniqueness on both the source and native sides. JSON cloning
preserves exact integers. Only actual v21 SQL columns are emitted. Unknown extra
SQL columns, unreferenced arbitrary archive files, and native-only UI/profile metadata with
no Flutter field are not a lossless interchange contract; retain a native backup.
The original Flutter restore itself does not import icon/IP cache tables, even
though they are retained in the emitted database snapshot.

Portable resource/profile paths are translated structurally, not by replacing
text in secrets/comments. Source YAML stays byte-exact unless paths or native
profile overlays require a patch. Native per-profile choices win over legacy
global choices. A shared script wins over a dormant inline buffer; the latter
is retained as an unselected script-library item.

The exporter returns errors for unrepresentable native global/custom fields,
script-plus-native-overlay combinations, missing/ambiguous references,
unsupported override keys, nonportable paths, missing local provider files,
conflicting global-library proxy/resource definitions, unsupported provider
options, resources without known metadata, and multiple authentication
credentials. Malformed/null added-rule lists and rules whose projection would
discard fields are rejected. Binary custom rule resources are copied byte-for-
byte, with explicit format/behavior metadata; they are never decoded as text.
Ordinary non-script file/URL profile YAML overlays can be materialized in the
exported source, but Flutter cannot
retain that native overlay as a separate editable overlay across a subscription
refresh. Profile settings that would lose their precedence to Flutter's later
app settings are rejected. Global DNS keys outside the original
`DnsOverrideKey.normalProfileKeys` are rejected when file/URL profiles are present;
Flutter applies those keys only to custom profiles. Arbitrary native
custom-provider layouts are not universally supported.

**File/URL profile source file providers are not exportable**, even when their
paths are native portable resource/profile references. In the pinned
`lib/common/task.dart`, `confineProviders` replaces every non-inline source
provider path before custom-library injection. Flutter restore only copies
`BackupEntries.ofData` paths, not these per-profile cache paths. Merely emitting
`providers/rules/<hash>` and rewriting YAML appeared to pass a native re-import,
but would leave Flutter using a missing file. Export now fails explicitly;
custom-profile library injection remains supported. Keep a native backup for
these layouts and for app-managed `flclash-source` resource definitions, which
Flutter cannot faithfully retain.

Reads are confined through `os.Root`, reject nonregular/symlink leaf files, and
are limited to 32 MiB per entry, 128 MiB total, and 20,000 entries. SQLite export
uses a private temporary directory, a 32 MiB page cap and ten-second context;
cleanup runs on success and failure. Settings input is capped at 128 MiB,
snapshots/extensions at 32 MiB, and individual tables at 100,000 rows. Export never
saves settings or rewrites source files. Returned bytes include secrets and must be treated as private.

## Evidence, not a runtime parity claim

Tests cover native and imported Flutter export/re-import, native-restore identity
remapping, legacy UUID/raw-rule promotion, source bytes, scripts and choices,
portable resources, unknown JSON data, edited overlays, schema integrity/foreign
keys, and unsafe input. Review regressions cover edited custom source, removed
library nodes, preserved versus rebuilt provider-label swaps, conflicting definitions,
MRS byte preservation, script-selection changes, profile/global precedence,
malformed identity maps, exact integers, parent-directory symlink escape and
entry quotas. When `build/upstream` exists, tests compare the emitted
SQL columns, nullability, primary keys and constraints and override-key enums
against the actual pinned generated Dart. Those source comparisons skip without
the checkout; committed-schema round trips still run.

The original pinned parser and `BackupAction.restore` now pass four safe tests
against a synthetic native export: parser/generated-model decoding, compatible
all-data restore, override restore, and profiles-only restore. Original DAOs,
installed file bytes, settings, ordering and SQLite integrity are checked without
starting the app/core or making HTTP requests. See
`../../tool/reference/port_backup_test.dart` and the retained metrics in
`testdata/flutter-backup-restore.json`. The available SDK was Flutter **3.47.5**,
not the workflow's **3.47.6**; that mismatch is explicit. This current-schema
synthetic evidence does not establish real older-archive or all-layout/platform
parity. Regenerate input with `TestFlutterBackupReferenceFixture` and follow the
reference README to rerun the original harness.
