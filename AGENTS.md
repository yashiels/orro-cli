# AGENTS.md — orro

CLI for Tuya-based standing desks. Go 1.22+, native Tuya LAN protocol (v3.3/v3.4) and Cloud API.

## Structure

```
cmd/orro/
  main.go            — entry point, delegates to internal/cmd
internal/cmd/
  root.go            — Cobra root command, global flags, config/output helpers
  config.go          — config subcommands: show, path, set, init
  status.go          — status command
  movement.go        — up, down, stop, sit, stand, goto commands
  height.go          — height targeting command
  presets.go         — presets command
  cli_test.go        — integration tests (builds binary, runs subcommands)
internal/config/
  config.go          — Config struct, multi-layer loading (1Password → TOML → env → flags)
  config_test.go     — unit tests for config loading, redaction, file I/O
internal/desk/
  desk.go            — high-level desk ops: movement, LAN-then-cloud dispatch, status, presets
  desk_test.go       — movement command structure and height extraction tests
  height.go          — GoToHeight polling loop with tolerance and timeout
internal/tuya/
  cloud.go           — Tuya Cloud API client with HMAC-SHA256 signing
  lan.go             — Tuya LAN client: TCP, UDP discovery, v3.3/v3.4 auto-negotiation
  protocol.go        — AES-128-ECB, PKCS7, packet build/parse, session key derivation
  protocol_test.go   — crypto and packet roundtrip tests
internal/output/
  output.go          — table/JSON output formatting with colour
  output_test.go     — output formatting tests
```

## Build / Test

```bash
go build -o orro ./cmd/orro
go vet ./...
go test -race ./...
make ci    # lint + test
```

## Key Design Decisions

- **Single static binary** — no Python/pip/venv; ships via Homebrew or pre-built tarball
- **LAN-first**: always try native Tuya LAN protocol before falling back to Cloud API
- **Config layering**: CLI flags > env vars > TOML file > 1Password > defaults
- **No async**: synchronous throughout; desk operations are inherently sequential
- **DP codes are desk-specific**: defaults work for Orro desks; override via config for other Tuya `sjz` devices
- **Height targeting polls at 500ms**: stops within 5mm tolerance, 30s timeout, always sends stop in defer
- **Error handling**: all commands return errors via Cobra RunE — no os.Exit in command handlers

## Config

Config file: `~/.config/orro/config.toml`. Credentials in 1Password vault `OpenClaw`, item `Tuya IoT Platform`.

## Constraints

- Keep 1Password as a config source
- Tuya v3.3 and v3.4 protocol support is native (no tinytuya)
- Config file format is TOML (not YAML)

## CI

- `.github/workflows/ci.yml`: lint (golangci-lint) + test (Go 1.22, 1.23) on push/PR
- `.github/workflows/release.yml`: cross-compile 4 targets + GitHub Release on `v*` tags
- Tests: `internal/config/config_test.go`, `internal/cmd/cli_test.go`, `internal/tuya/protocol_test.go`, `internal/desk/desk_test.go`, `internal/output/output_test.go`
- Run locally: `make ci`
