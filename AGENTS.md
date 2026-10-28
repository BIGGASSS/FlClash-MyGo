# FlClash MyGo

This is a desktop-only native MyGo application, with FlClash's Go core in a separate module and mihomo pinned as a submodule. Read `docs/ARCHITECTURE.md` and `docs/PARITY.md` before extending it. Do not describe incomplete parity as complete.

- Keep the application and core buildable with `CGO_ENABLED=0` on Linux, Windows, and macOS, for amd64 and arm64.
- Keep UI state on MyGo's main thread. Use `Window.Update` for background results; never retain `ui.Element` values between builds.
- Preserve the engine's latest-intent lifecycle ownership. UI views request transitions through the engine.
- Test through `--safe-mode`, headless `ui.Tester`, and temporary core/data directories. Never change the workstation's system proxy or TUN as a test, or terminate other processes by name.
- Preserve source YAML, unknown fields, subscription metadata, per-profile choices, and the last working applied config on failed edits.
- Keep original GPL notices and vendored MIT notices. Document changes to pinned dependencies.
- Run `gofmt`, `CGO_ENABLED=0 go test ./...`, and `CGO_ENABLED=0 go vet ./...` from the root. Run the core's tests and vet from `core` when touching its boundary.
- After building the core, run the real-core IPC test with `FLCLASH_TEST_CORE` pointing to that executable.
- Maintain the desktop build matrix. Do not add mobile build targets.
