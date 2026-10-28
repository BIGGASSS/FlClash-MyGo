# On-demand settings UI integration

The source has no enable switch: a nonempty ordered excluded-SSID list activates
monitoring. Use the existing on-demand/exclusion locale labels.

- Read/edit/reorder/delete `a.cfg.ExcludeSSIDs`. Commit a complete list through
  `a.setExcludeSSIDs(list)`; show its returned validation/save error. SSIDs must
  not be trimmed or case-folded. Imported lists remain unchanged until edited.
- Show the current `a.onDemand.State.SSID` only when `a.onDemand.Known` and
  `State.Connected`. Unknown and disconnected are distinct.
- Surface `a.onDemand.Error`, including permission/API guidance, separately from
  `a.onDemandSuspended()`. Never label a permission error as excluded.
- The parked status must not be shown as a user stop: `a.wantRunning` remains the
  user's request. Main-thread `apply` and `begin` use `a.listenerIntent()`.
- Safe mode retains editable preferences but never creates/reads a Wi-Fi backend
  or requests permissions. Do not add an automatic settings/permission launch.
- Desktop authorization prompts are not implemented; see
  `../desktop/wifi.md` for platform limitations.

The General → On demand page now supports adding, renaming, selecting, deleting
and reordering SSIDs with transactional error retention. Its status/error panels
scroll above a minimum list viewport at 380 × 400. New-name validation uses
extended grapheme clusters, matching Flutter's visible-character limit; imported
identities remain byte-for-byte unchanged. Native backup and Flutter interchange
retain the list.

`applyProviderRuleChanges` and probe ownership also use `listenerIntent()`, so a
provider refresh cannot bypass parking and parked outbound requests use DIRECT.
Dashboard controls show the source suspended status/Wi-Fi-off glyph while keeping
the user's stop intent available. Integration tests cover these cross-feature
paths without touching workstation networking.
