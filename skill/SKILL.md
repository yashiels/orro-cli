---
name: orro
description: Control an Orro (Tuya-based) standing desk from the terminal — move up/down, stop, recall sit/stand memory presets, drive to an exact height, and read status. LAN-first with Tuya Cloud fallback.
---

# orro

CLI for controlling Orro and other Tuya-based standing desks. It talks the native Tuya LAN protocol (v3.3/v3.4) directly for sub-second local control, and falls back to the Tuya Cloud API when the desk is unreachable over the LAN. It can move the desk up/down, stop movement, recall memory-slot presets (sit/stand/mem1–mem4), drive to an exact height in centimetres, and report the current height, connection path, and protocol.

## Install

```sh
brew install yashiels/tap/orro
```

Single static Go binary — no Python/pip/venv runtime. After install, `orro` is on your `PATH`.

## Configuration

Config is resolved in priority order (highest wins):

1. CLI flags (`--cloud`, `--config`, …)
2. Environment variables (`ORRO_*`)
3. TOML config file (`~/.config/orro/config.toml`)
4. 1Password vault (`OpenClaw` → item `Tuya IoT Platform`)
5. Built-in defaults

Config file location: `~/.config/orro/config.toml` (XDG-aware — honours `XDG_CONFIG_HOME`; created with `0600` perms). Override with `ORRO_CONFIG` or `--config <path>`.

Interactive setup: `orro config init` walks through credentials and writes the file (prints a template instead if stdin is not a TTY). Inspect with `orro config show` (secrets redacted) and `orro config path`.

Example `config.toml`:

```toml
endpoint = "https://openapi.tuyaeu.com"
access_id = "your_access_id"
access_secret = "your_access_secret"
device_id = "your_device_id"

# Optional — required for LAN-first control
local_key = "your_local_key"
lan_ip = "192.168.10.95"
lan_version = "3.4"

[presets]
sit = "mem1"
stand = "mem3"
```

## Credentials

Tuya IoT Platform credentials are required (get them from a Cloud Project at iot.tuya.com — subscribe to IoT Core + Authorization Token Management, link your SmartLife account):

| Key             | Purpose                                                    | Required            |
|-----------------|------------------------------------------------------------|---------------------|
| `endpoint`      | Regional Tuya API endpoint (e.g. `openapi.tuyaeu.com`)     | Cloud path          |
| `access_id`     | Tuya Access ID                                             | Cloud path          |
| `access_secret` | Tuya Access Secret                                         | Cloud path          |
| `device_id`     | Desk's Tuya device ID                                      | Yes                 |
| `local_key`     | Device local encryption key                               | LAN path            |
| `lan_ip`        | Device LAN IP (else UDP discovery on 6666/6667)           | LAN path (optional) |
| `lan_version`   | Tuya LAN protocol version (e.g. `3.4`)                     | LAN path            |

Environment variables (each maps to the matching config field):

| Variable             | Field           |
|----------------------|-----------------|
| `ORRO_ENDPOINT`      | `endpoint`      |
| `ORRO_ACCESS_ID`     | `access_id`     |
| `ORRO_ACCESS_SECRET` | `access_secret` |
| `ORRO_DEVICE_ID`     | `device_id`     |
| `ORRO_LOCAL_KEY`     | `local_key`     |
| `ORRO_LAN_IP`        | `lan_ip`        |
| `ORRO_LAN_VERSION`   | `lan_version`   |
| `ORRO_CONFIG`        | config file path |

Advanced JSON overrides: `ORRO_LAN_DP_MAP` (JSON of DP codes) and `ORRO_PRESETS` (JSON of preset→slot mappings). DP-code defaults are tuned for Orro desks (Tuya category `sjz`).

**1Password**: when the `op` CLI is installed and signed in, credentials are read automatically from vault `OpenClaw`, item `Tuya IoT Platform` (fields: `API Endpoint`, `Access ID`, `Access Secret`, `Device ID`, `Local Key`, `LAN IP`, `LAN Version`, `LAN DP Map`, `Presets`). No config file needed in that case.

## Commands

### Desk control (state-changing)

| Command            | Description                                                    |
|--------------------|----------------------------------------------------------------|
| `orro up`          | Move desk up continuously (send `orro stop` to halt)          |
| `orro down`        | Move desk down continuously (send `orro stop` to halt)        |
| `orro stop`        | Stop any in-progress desk movement                            |
| `orro sit`         | Recall the configured sit preset (default `mem1`)             |
| `orro stand`       | Recall the configured stand preset (default `mem3`)           |
| `orro goto <slot>` | Recall a specific memory slot (`mem1`–`mem4`)                 |
| `orro height <cm>` | Drive desk to an exact height in cm, then stop (polls to ±5mm, 30s timeout) |

### Status (read-only)

| Command         | Description                                                |
|-----------------|------------------------------------------------------------|
| `orro status`   | Show desk height, connection path, and protocol in use     |
| `orro presets`  | Show preset mapping and heights stored in each device slot  |

### Configuration

| Command            | Description                                                  |
|--------------------|--------------------------------------------------------------|
| `orro config init` | Interactive setup wizard (prints template if not a TTY)     |
| `orro config show` | Print resolved config (secrets redacted)                    |
| `orro config path` | Print the config file path in use                           |
| `orro config set`  | Write individual config values (see flags below)            |

`orro config set` flags: `--endpoint`, `--access-id`, `--access-secret`, `--device-id`, `--local-key`, `--lan-ip`, `--lan-version`.

### Global flags

| Flag              | Description                                          |
|-------------------|------------------------------------------------------|
| `--output <fmt>`  | `table` (default) or `json`                          |
| `--json`          | Shorthand for `--output json`                        |
| `--cloud`         | Force Tuya Cloud API instead of LAN-first            |
| `--verbose`, `-v` | Print debug info (connection steps) to stderr        |
| `--quiet`, `-q`   | Suppress info banners                                |
| `--config <path>` | Use a custom config file path                        |
| `--version`       | Print version and exit                               |

## Headless / agent usage

**Safe to run unattended (read-only):** `orro status`, `orro presets`, `orro config show`, `orro config path`, `orro --version`. These only read desk/config state and change nothing. Use `--json` (or `--output json`) for machine-parseable output — ideal for status bars and scripts.

**Supplying credentials headless:** all credentials come from config sources that need no interaction — set the `ORRO_*` env vars, or point at a pre-written `~/.config/orro/config.toml` (or `--config <path>`), or rely on a signed-in `op` CLI reading vault `OpenClaw` → `Tuya IoT Platform`. Do NOT run `orro config init` unattended — it is an interactive wizard (it only prints a template when stdin is not a TTY, so it is harmless but not useful headless). Prefer `orro config set` or env vars to write config non-interactively.

**State-changing device commands — physically move the desk.** `orro up`, `orro down`, `orro stop`, `orro sit`, `orro stand`, `orro goto`, and `orro height` all drive real hardware. Run these ONLY on an explicit user request ("stand up", "go to 110", "sit"). Never issue them speculatively, in a loop, or as a side effect of a status check. Note that `orro up`/`orro down` move continuously until `orro stop` (or a preset/height command) is sent — avoid them unattended; prefer `orro goto <slot>` or `orro height <cm>`, which stop on their own.

## Typical flow

```sh
# One-time setup (interactive — user present)
orro config init

# Read-only checks — safe anytime, incl. headless
orro status --json
orro presets

# Move the desk — only on explicit user request
orro stand              # recall stand preset (default mem3)
orro height 110         # drive to 110 cm, then auto-stop
orro sit                # recall sit preset (default mem1)

# Force the cloud path if LAN is unreachable
orro status --cloud
```

## Notes

- LAN-first: `orro` tries the native Tuya LAN protocol (TCP 6668) first, then falls back to the Tuya Cloud REST API automatically. `--cloud` skips the LAN attempt entirely. LAN requires `local_key` + `lan_version` (and `lan_ip`, or UDP discovery on 6666/6667).
- Config format is TOML (not YAML).
- Commands exit non-zero on error and print a human-readable message to stderr.
- Not affiliated with Orro or Tuya; works with most Tuya `sjz`-category standing desks via DP-code overrides.
