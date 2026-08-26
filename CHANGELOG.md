# Changelog

## 0.3.0 — 2026-08-26

### Fixed
- **v3.4 LAN session-key handshake** (#16): LAN control now works instead of always resetting mid-negotiation and falling back to the Tuya Cloud. The `SESS_KEY_NEG` packets are now AES-ECB encrypted and HMAC-SHA256 framed under the local key (were CRC32 framed); the remote nonce is read from `payload[:16]` and the device HMAC in `payload[16:48]` is verified; the session key is derived as `AES-ECB(local_nonce XOR remote_nonce, key=local_key)`; and `SESS_KEY_NEG_FINISH` sends `HMAC-SHA256(remote_nonce, key=local_key)` framed with the local key. CONTROL commands are now prefixed with the `"3.4"` protocol header before encryption. Aligned with tinytuya's reference implementation.

### Refactored
- **Error handling**: all command handlers return errors via Cobra `RunE` instead of calling `os.Exit` — cleaner process lifecycle, testable, no panics
- **Config `SkipOP` option**: `LoadOptions.SkipOP` skips 1Password lookup entirely, preventing 60–75s `op read` timeouts in CI and tests
- **Config test speed**: tests run in ~2s (was ~75s) by using `SkipOP` instead of waiting for `op` CLI timeouts
- **Output test coverage**: added `output_test.go` covering JSON, table, booleans, nil values, nested maps, slices (0% → 78.5%)
- **Config test coverage**: added tests for env var presets, LAN DP map override, config file merge, and CLI flag overrides (59% → 64%)
- **Removed redundant export wrapper**: `MovementCommands` was wrapping an unexported `movementCommands` — collapsed to a single exported function
- **Updated AGENTS.md**: now documents Go project structure, build commands, and design decisions (was still describing the Python codebase)

## 0.2.0 — 2026-06-08

Complete rewrite from Python to Go for single-binary distribution.

- **Single static binary** — no Python runtime, virtualenv, or pip required
- Ships via Homebrew as a tarball; cross-compiled for macOS (amd64/arm64), Linux (amd64/arm64), and Windows
- All commands preserved: `status`, `up`, `down`, `stop`, `sit`, `stand`, `goto`, `height`, `presets`, `config`
- Config file format changed from YAML (`config.yaml`) to TOML (`config.toml`)
- Tuya LAN protocol v3.3 and v3.4 implemented natively (no Python dependency)
- v3.4 session key negotiation with AES-128-ECB + HMAC-SHA256
- Updated CI to Go matrix (1.22, 1.23); release workflow produces 4-platform binaries

## 0.1.0 — 2026-05-23

- Initial release
- LAN-first control via TinyTuya with Cloud API fallback
- Commands: status, up, down, stop, sit, stand, goto, height, presets, config
- Configuration: YAML config file, environment variables, 1Password CLI
- Output: table (human) and JSON (machine) formats
- Config precedence: flags > env vars > config file > 1Password > defaults
- Verbose and quiet modes
- Interactive config init
