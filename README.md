<p align="center">
  <img src=".github/orro-logo.png" alt="Orro" width="280">
</p>

<h1 align="center">orro-cli</h1>
<p align="center">Control your standing desk from the terminal.</p>

<p align="center">
  <a href="https://github.com/yashiels/orro-cli/actions/workflows/ci.yml"><img src="https://github.com/yashiels/orro-cli/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/yashiels/orro-cli/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://www.python.org/"><img src="https://img.shields.io/badge/python-3.12%2B-blue.svg" alt="Python 3.12+"></a>
</p>

A modern Python CLI for [Orro](https://orro-brand.co.za/) and other Tuya-based standing desks. Move up, down, recall presets, target exact heights — all from the command line. Talks to the desk over your LAN first (sub-second response, no cloud roundtrip) and falls back to the Tuya Cloud API automatically.

> Orro desks use the **Tuya IoT platform** under the hood. `orro` talks to the same local protocol (TinyTuya) and cloud endpoints the SmartLife/Tuya app uses. You need a free [Tuya IoT Platform](https://iot.tuya.com/) account linked to your device.

## Features

- **LAN-first control** via [TinyTuya](https://github.com/jasonacox/tinytuya) — sub-second response, works offline
- **Automatic Cloud fallback** via the Tuya IoT Platform API when LAN isn't available
- **Preset memory slots** — `orro sit` / `orro stand` mapped to device memory positions, plus `orro goto mem1`–`mem4`
- **Exact height targeting** — `orro height 110` moves to 110 cm and stops automatically
- **Flexible credentials** — YAML config file, environment variables, or [1Password CLI](https://developer.1password.com/docs/cli/)
- **Machine-readable output** — `--output json` for scripts, `--output table` for humans
- **Config management** — `orro config init` for interactive setup, `orro config show` with secret redaction

## Quickstart

```bash
# install
git clone https://github.com/yashiels/orro-cli.git
cd orro
pip install .

# or with pipx (isolated)
pipx install .

# interactive config setup
orro config init

# check desk status
orro status

# stand up
orro stand
```

## Command Surface

| Command | Description |
|---------|-------------|
| `orro status` | Show desk status (height, connection path, protocol version) |
| `orro up` | Move desk up (hold until `orro stop`) |
| `orro down` | Move desk down (hold until `orro stop`) |
| `orro stop` | Stop desk movement |
| `orro sit` | Move to configured sit preset (default: `mem1`) |
| `orro stand` | Move to configured stand preset (default: `mem3`) |
| `orro goto <slot>` | Move to a specific memory preset (`mem1`–`mem4`) |
| `orro height <cm>` | Move to exact height in cm (e.g. `orro height 75.0`) |
| `orro presets` | Show preset configuration and device memory heights |
| `orro config show` | Print current config (secrets redacted) |
| `orro config path` | Print config file location |
| `orro config set` | Set config values (e.g. `--endpoint`, `--device-id`) |
| `orro config init` | Interactive first-time setup |

### Global Flags

| Flag | Description |
|------|-------------|
| `--output table\|json` | Output format (default: `table`) |
| `--cloud` | Force Tuya Cloud API (skip LAN) |
| `--verbose` / `-v` | Debug output (connection attempts, API calls) |
| `--quiet` / `-q` | Suppress info banners |
| `--config <path>` | Custom config file path |
| `--version` | Print version |

## Configuration

Priority: **flags > environment variables > config file > 1Password > defaults**.

### Config file (recommended)

```bash
orro config init   # interactive setup
orro config show   # verify (secrets redacted)
```

Config lives at `~/.config/orro/config.yaml` (permissions `0600`). Override with `--config` or `ORRO_CONFIG`.

```yaml
endpoint: "https://openapi.tuyaeu.com"
access_id: "your_access_id"
access_secret: "your_access_secret"
device_id: "your_device_id"
local_key: "your_local_key"
lan_ip: "192.168.10.95"        # optional, skips LAN discovery
lan_version: "3.4"             # optional, Tuya protocol version
presets:
  sit: mem1
  stand: mem3
```

### Environment variables

```bash
export ORRO_ENDPOINT="https://openapi.tuyaeu.com"
export ORRO_ACCESS_ID="your_access_id"
export ORRO_ACCESS_SECRET="your_access_secret"
export ORRO_DEVICE_ID="your_device_id"
export ORRO_LOCAL_KEY="your_local_key"       # optional, enables LAN
export ORRO_LAN_IP="192.168.10.95"           # optional, skips discovery
export ORRO_LAN_VERSION="3.4"                # optional
```

### 1Password CLI (optional)

If you have the [1Password CLI](https://developer.1password.com/docs/cli/) installed, orro reads from vault `OpenClaw`, item `Tuya IoT Platform`. Fields: `API Endpoint`, `Access ID`, `Access Secret`, `Device ID`, `Local Key`, `LAN IP`, `LAN Version`.

## LAN vs Cloud

| | LAN | Cloud |
|---|---|---|
| Speed | Sub-second | 1–3 seconds |
| Internet required | No | Yes |
| Setup | Local key + IP/discovery | API credentials only |
| Movement commands | ✅ Full control | ⚠️ Limited (some DPs not cloud-exposed) |

`orro` tries LAN first. If it fails (no local key, device offline, wrong protocol version), it falls back to Cloud automatically. Use `--cloud` to skip LAN.

For LAN control you need the device's **local key** — see [Getting Tuya Credentials](#getting-tuya-credentials).

## Supported Desks

Tested with **Orro** standing desks (Tuya device category `sjz`). Should work with any Tuya-based standing desk that uses the standard standing desk DP (data point) codes.

### Where to buy

- 🇿🇦 **Takealot**: [Orro Pro Electric Standing Desk Frame](https://www.takealot.com/orro-pro-electric-standing-desk-frame-dual-motor-memory-app-sit-/PLID97472258) — dual motor, memory, app control
- 🇿🇦 **Takealot**: [Orro Home Plus Electric Standing Desk](https://www.takealot.com/orro-home-plus-electric-standing-desk-memory-height-adjustable-s/PLID97472261) — complete desk with memory presets
- 🇿🇦 **Takealot**: [Orro Home Electric Standing Desk Frame](https://www.takealot.com/orro-home-electric-standing-desk-frame-memory-adjustable-width-s/PLID97472259) — frame only, adjustable width
- 🌐 **Official site**: [orro-brand.co.za](https://orro-brand.co.za/collections/standing-desks)

Other Tuya `sjz` desks (e.g. Ergolutions, generic SmartLife-controlled standing desks) may work — the DP codes might differ. See [Discovery](#discovery) to check your desk's DP map.

## Getting Tuya Credentials

1. Download the **Tuya Smart** or **SmartLife** app and add your desk
2. Create a free account at [iot.tuya.com](https://iot.tuya.com/)
3. Create a **Cloud Project** (Development type)
4. Subscribe to **IoT Core** and **Authorization Token Management** APIs
5. Link your SmartLife/Tuya Smart app account under **Devices > Link Tuya App Account**
6. Note your **Access ID**, **Access Secret**, and **API Endpoint** (region-dependent, e.g. `https://openapi.tuyaeu.com` for Europe/Africa)
7. Find your **Device ID** in the device list
8. The **Local Key** appears in the device details (needed for LAN control)

## Discovery

If your desk uses different DP codes, run the discovery script to map them:

```bash
cd orro
python scripts/discover_tuya.py
```

This queries the Tuya Cloud API for your device's full DP specification and writes `dp-discovery.json` (gitignored — contains raw device data).

The default DP map is:

```json
{
  "child_lock": 6,
  "move_up": 150,
  "move_down": 151,
  "height_display": 152,
  "memory_location": 155
}
```

Override with `ORRO_LAN_DP_MAP` env var or `lan_dps` in the config file if your desk differs.

## Known API Realities

- Tuya does **not** publish a stable public API for standing desks. `orro` uses the same cloud endpoints as the mobile apps.
- Not all DP codes are exposed via the Cloud API — movement commands (`move_up`, `move_down`) are typically LAN-only. Cloud status works but Cloud control may be limited.
- LAN discovery broadcasts UDP on port 6666/6667. Some networks or firewalls block this — set `lan_ip` explicitly if discovery fails.
- The local key rotates if you re-pair the device in the app. Re-run discovery or update your config.
- Protocol version auto-detection tries 3.5, 3.4, 3.3, 3.1 in order. Pin `lan_version` in config to skip probing.

## Prior Work / References

- [TinyTuya](https://github.com/jasonacox/tinytuya) — Python library for local Tuya device control
- [tuya-connector-python](https://github.com/tuya/tuya-connector-python) — Official Tuya Cloud SDK
- [Home Assistant Tuya standing desk thread](https://community.home-assistant.io/t/tuya-new-device-standing-desk/597025) — community DP mapping for `sjz` desks
- [Ergolutions local DP notes](https://www.benjamin-schieder.de/blog/2024/09/15/using-an-ergolutions-tuya-desk-controller-with-home-assistant.html) — another Tuya desk integration writeup
- [Orro official site](https://orro-brand.co.za/)

## Development

```bash
git clone https://github.com/yashiels/orro-cli.git
cd orro
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"

make lint    # ruff check
make fmt     # ruff format
make test    # pytest
```

## License

[MIT](LICENSE)
