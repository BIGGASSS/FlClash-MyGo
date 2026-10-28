# App-managed rule resource files

Manual and bulk provider updates recognize library files by their private path,
not by a provider label alone. Native file definitions retain their remote `url`
and use an owned `flclash-resource://<id>` reference. Retained upstream library
rows also identify the original `providers/rules/md5(id@url)[.mrs]` cache.
Older native imports' HTTP overrides migrate only when their retained library
label, URL, active custom-profile override and private cache path all agree.
Ordinary subscription HTTP providers remain core-managed.

Downloads use the captured application request client and global User-Agent,
a twenty-second HTTP budget, the existing five-redirect limit, and a 32 MiB
uncompressed body limit. Four provider operations can run concurrently.
Engine revision changes cancel active requests. UI commits also check the
captured core client, profile, resource definition and provider path.

The existing `compileRuleSet` IPC method operates on an unreferenced private
staging file. Domain/IP sources use its byte-exact MRS output. A generated native
file definition retains the original source with this application-only field:

```yaml
Library:
  type: file
  url: https://example.invalid/rules.yaml
  path: flclash-resource://<compiled-content-id>
  behavior: domain
  format: mrs
  flclash-source:
    path: flclash-resource://<original-source-id>
    format: yaml
```

Both identities are listed in `Settings.Resources`. Nested `path` references
use the existing runtime resolution and native backup/remapping machinery;
source YAML and retained import rows are not rewritten. MRS sources and
classical sets need only one identity.

A shape change uses fresh resource identities, validates the generated config,
and requests an engine reconcile. Previous files remain available to the last
working config. Unchanged shapes prepare and fsync replacements and rollback
copies off-thread; the UI atomically swaps them after checking ownership, then
requests the path-guarded file-provider reload. A rejected reload restores the
previous files only if the operation still owns the current provider. This
avoids the core side-load method's non-atomic file write for managed sync.

This does not add the entire upstream resource library editor, resource-wide
auto-update scheduling, alert/sheet animations, or complete visual parity.
