# Material Color Utilities

The theme helper bundles the Apache-2.0 JavaScript package
`@material/material-color-utilities` 0.3.0 from
https://github.com/material-foundation/material-color-utilities/tree/main/typescript.
Its source package and integrity are pinned in `tool/theme/package-lock.json`.
`internal/theme/source/palette.js` ports FlClash's HCT/surface rules, with the
compiled helper committed as `internal/theme/palette.js`. No JavaScript UI is
used. Regenerate with `npm ci && npm run build` from `tool/theme`.

The numerical behavior is checked against the original Dart package 0.13.0 as
used by the pinned FlClash revision; the full reference corpus is under
`internal/theme/testdata`.
