# Changelog

## Unreleased

(nothing yet)

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
