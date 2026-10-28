# QuickJS-NG

This is the unchanged engine source bundled in `rquickjs-sys` 0.12.2, matching
FlClash's pinned Rust dependency. Its reported QuickJS-NG version is 0.15.1.

Source archive: https://static.crates.io/crates/rquickjs-sys/rquickjs-sys-0.12.2.crate

SHA-256: `a13ac243b86a74120814ef7e9e30ad5a2c1199b7b9963b1cf7c84e4cdc1cad99`

Only the engine sources, required headers, and MIT license are included. There
are no local engine changes. The application ABI lives in
`internal/script/source/evaluate.c`. It creates a fresh runtime/context per
transform, installs console methods, applies the original 256 MiB heap limit,
resolves promises by draining jobs, and leaves OS/module bindings uninstalled.

`scripts/build-quickjs.sh` produces the embedded WASI reactor using wasi-sdk 34.0
(`wasi-sdk-34.0-x86_64-linux.tar.gz`, SHA-256
`b761e3a0721dbae9c09a0059e5fdb2bf917d1b4a8a7b430fb3b5aafb0984b2c4`). The committed reactor keeps ordinary Go builds independent
of a C toolchain. Wazero runs it without cgo or external shared libraries.
