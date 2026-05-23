# Changelog

## 0.1.0 — 2026-05-23

- Initial release
- LAN-first control via TinyTuya with Cloud API fallback
- Commands: status, up, down, stop, sit, stand, goto, height, presets, config
- Configuration: YAML config file, environment variables, 1Password CLI
- Output: table (human) and JSON (machine) formats
- Config precedence: flags > env vars > config file > 1Password > defaults
- Verbose and quiet modes
- Interactive config init
