# 🪑 orro — control your standing desk from the terminal

> A snappy Python CLI for Orro and other Tuya-based standing desks. Move up, down, hit a preset, or drive to an exact height — all without leaving your shell.


[![PyPI](https://img.shields.io/pypi/v/orro)](https://pypi.org/project/orro/)
[![Python 3.12+](https://img.shields.io/pypi/pyversions/orro)](https://pypi.org/project/orro/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/yashiels/orro-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/yashiels/orro-cli/actions)

---

## Features

- **LAN-first, cloud fallback** — sub-second local control via TinyTuya; automatic fallback to the Tuya Cloud API when LAN is unreachable
- **Exact height targeting** — `orro height 110` drives the desk to 110 cm and stops; no manual hold required
- **Memory presets** — `orro sit` / `orro stand` recall named slots; `orro goto mem2` hits any slot directly
- **1Password integration** — reads all credentials from your vault when the `op` CLI is present; no plain-text secrets needed
- **Machine-readable output** — every command supports `--output json` for scripting and status bars
- **Zero-friction setup** — `orro config init` walks you through credentials interactively

---

## Install

### Homebrew (recommended)

```bash
brew install yashiels/tap/orro
```

This auto-taps `yashiels/tap` and keeps `orro` updated with `brew upgrade`.

### PyPI

```bash
pipx install orro        # isolated, preferred for CLI tools
# or
pip install orro
```

### From source

```bash
git clone https://github.com/yashiels/orro-cli.git
cd orro-cli
pip install .
```

---

## Quick Start

```bash
# First-time setup — follow the prompts
orro config init

# Check connection and current height
orro status

# Go to your standing position
orro stand

# Go to your sitting position
orro sit

# Drive to an exact height
orro height 110
```

---

## Commands

### Desk control

| Command              | Description                                                     |
|----------------------|-----------------------------------------------------------------|
| `orro status`        | Show desk height, connection path, and protocol in use         |
| `orro up`            | Move desk up continuously (send `orro stop` to halt)           |
| `orro down`          | Move desk down continuously (send `orro stop` to halt)         |
| `orro stop`          | Stop any in-progress desk movement                             |
| `orro sit`           | Recall the configured sit preset (default: `mem1`)             |
| `orro stand`         | Recall the configured stand preset (default: `mem3`)           |
| `orro goto <slot>`   | Recall a specific memory slot — `mem1`, `mem2`, `mem3`, `mem4` |
| `orro height <cm>`   | Drive desk to an exact height in centimetres, then stop        |
| `orro presets`       | Show preset mapping and the heights stored in each device slot |

### Configuration

| Command               | Description                                                    |
|-----------------------|----------------------------------------------------------------|
| `orro config init`    | Interactive first-time setup wizard; prints a template if not a TTY |
| `orro config show`    | Print current resolved config (secrets redacted)              |
| `orro config path`    | Print the config file path in use                             |
| `orro config set`     | Write individual config values (see flags below)              |

`orro config set` flags:

```
--endpoint        Tuya API endpoint URL
--access-id       Tuya Access ID
--access-secret   Tuya Access Secret
--device-id       Tuya device ID
--local-key       Device local encryption key (required for LAN)
--lan-ip          Device LAN IP address
--lan-version     TinyTuya LAN protocol version (e.g. 3.4)
```

### Global flags

| Flag                    | Description                                               |
|-------------------------|-----------------------------------------------------------|
| `--output table\|json`  | Output format — `table` (human) or `json` (machine)      |
| `--json`                | Shorthand alias for `--output json`                       |
| `--cloud`               | Skip LAN and use Tuya Cloud API directly                  |
| `--verbose`, `-v`       | Print debug information (connection steps, raw responses) |
| `--quiet`, `-q`         | Suppress informational banners                            |
| `--config <path>`       | Use a custom config file path                             |
| `--version`             | Print version and exit                                    |

---

## Configuration

### Priority order

Settings are resolved in this order (highest wins):

1. CLI flags (`--cloud`, `--config`, etc.)
2. Environment variables (`ORRO_*`)
3. YAML config file (`~/.config/orro/config.yaml`)
4. 1Password vault (`OpenClaw` → `Tuya IoT Platform`)
5. Built-in defaults

### Config file

Default location: `~/.config/orro/config.yaml` (created with `0600` permissions).
Override with `ORRO_CONFIG` or `--config <path>`.

```yaml
endpoint: "https://openapi.tuyaeu.com"
access_id: "your_access_id"
access_secret: "your_access_secret"
device_id: "your_device_id"

# Optional — required for LAN-first control
local_key: "your_local_key"
lan_ip: "192.168.10.95"
lan_version: "3.4"

# Preset mapping (memory slot names)
presets:
  sit: mem1
  stand: mem3
```

### Environment variables

| Variable            | Config field     |
|---------------------|------------------|
| `ORRO_ENDPOINT`     | `endpoint`       |
| `ORRO_ACCESS_ID`    | `access_id`      |
| `ORRO_ACCESS_SECRET`| `access_secret`  |
| `ORRO_DEVICE_ID`    | `device_id`      |
| `ORRO_LOCAL_KEY`    | `local_key`      |
| `ORRO_LAN_IP`       | `lan_ip`         |
| `ORRO_LAN_VERSION`  | `lan_version`    |
| `ORRO_CONFIG`       | config file path |

Advanced JSON overrides: `ORRO_LAN_DP_MAP` (JSON object of DP codes), `ORRO_PRESETS` (JSON object of preset mappings).

### 1Password integration

When the `op` CLI is installed and signed in, `orro` reads credentials automatically from `OpenClaw` → `Tuya IoT Platform`. Expected field names:

| 1Password field  | Config field    |
|------------------|-----------------|
| `API Endpoint`   | `endpoint`      |
| `Access ID`      | `access_id`     |
| `Access Secret`  | `access_secret` |
| `Device ID`      | `device_id`     |
| `Local Key`      | `local_key`     |
| `LAN IP`         | `lan_ip`        |
| `LAN Version`    | `lan_version`   |
| `LAN DP Map`     | `lan_dps` (JSON)|
| `Presets`        | `presets` (JSON)|

No config file is needed when 1Password is configured.

---

## Getting Tuya Credentials

1. Add your desk to the **Tuya Smart** or **SmartLife** mobile app.
2. Create a free account at [iot.tuya.com](https://iot.tuya.com) and create a **Cloud Project**.
3. Subscribe to the **IoT Core** and **Authorization Token Management** APIs.
4. Under **Devices → Link Tuya App Account**, link your SmartLife account.
5. Note your **Access ID**, **Access Secret**, and **API Endpoint** (choose the region closest to you).
6. In the device list, find your desk's **Device ID**; its **Local Key** appears in the device details panel.

> **Tip:** `orro config init` walks through all of this interactively and writes the file for you.

---

## How It Works

### LAN-first, cloud fallback

`orro` tries to reach the desk directly over your local network first using [TinyTuya](https://github.com/jasonacox/tinytuya). LAN control requires `local_key` and `lan_ip` in config, but is dramatically faster (sub-100 ms vs ~800 ms for cloud).

When LAN is unreachable — wrong IP, device asleep, different network — `orro` falls back to the [Tuya Cloud REST API](https://developer.tuya.com/en/docs/iot/new-version?id=K9s9rhj8pxp7q) automatically. Use `--cloud` to skip the LAN attempt entirely.

### Tuya API

The cloud path uses Tuya's OpenAPI with HMAC-SHA256 request signing. All calls go to the regional endpoint you configure (`openapi.tuyaeu.com` for EU, `openapi.tuyaus.com` for US, etc.).

### Height targeting

`orro height <cm>` polls the desk's status DP (`height_display`, default DP 152) at ~200 ms intervals, issuing move commands until the reported height converges on the target within ±1 cm, then sends stop.

### DP codes

Default DP codes are tuned for Orro desks (Tuya device category `sjz`). Most Tuya-based standing desks use the same codes. If yours differs, override via `ORRO_LAN_DP_MAP` (JSON) or a `lan_dps:` block in the config file after running `tinytuya wizard` to discover your device's DPs.

---

## Supported Desks

Tested with **Orro** standing desks. Should work with any Tuya-based standing desk that exposes standard movement and height DPs. Not affiliated with Orro or Tuya.

---

## Development

```bash
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"

make lint    # ruff check
make fmt     # ruff format
make test    # pytest
make check   # lint + test
```

### Releases

Releases are automated via GitHub Actions. Go to **Actions → Ship**, pick `patch`, `minor`, or `major`. The workflow bumps the version, publishes to PyPI, creates a GitHub release, and updates the [Homebrew tap](https://github.com/yashiels/homebrew-tap).

---

## License

MIT — [Yashiel Sookdeo](https://github.com/yashiels)
