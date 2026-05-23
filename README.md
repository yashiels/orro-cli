# 🪑 orro — Control your standing desk from the terminal

CLI for Tuya-based standing desks. Move up, down, go to presets, target exact heights — all from the command line. Connects over your LAN first (fast, no cloud roundtrip) and falls back to the Tuya Cloud API automatically.

## Features

- **LAN-first control** via TinyTuya with automatic Cloud API fallback
- **Preset memory slots** — sit/stand aliases mapped to device memory positions (mem1–mem4)
- **Exact height targeting** — move to a specific height in cm with automatic stop
- **Multiple config sources** — YAML file, environment variables, 1Password CLI
- **Machine-readable output** — `--output json` for scripting, `--output table` for humans
- **Zero cloud dependency for control** — LAN mode works without internet

## Quickstart

```bash
# Clone and install
git clone https://github.com/yashiels/orro.git
cd orro
python -m venv .venv && source .venv/bin/activate
pip install -e .

# Set up config (interactive)
orro config init

# Or set values directly
orro config set --endpoint https://openapi.tuyaeu.com \
    --access-id YOUR_ID --access-secret YOUR_SECRET \
    --device-id YOUR_DEVICE --local-key YOUR_KEY \
    --lan-ip 192.168.10.95

# Check desk status
orro status

# Move to standing position
orro stand
```

## Command Surface

| Command | Description |
|---------|-------------|
| `orro status` | Show desk status (height, connection path, protocol version) |
| `orro up` | Move desk up (hold until stop) |
| `orro down` | Move desk down (hold until stop) |
| `orro stop` | Stop desk movement |
| `orro sit` | Move to configured sit preset |
| `orro stand` | Move to configured stand preset |
| `orro goto mem1` | Move to a specific memory preset (mem1–mem4) |
| `orro height 75.0` | Move to a specific height in cm |
| `orro presets` | Show preset configuration and device memory slots |
| `orro config show` | Print current config (secrets redacted) |
| `orro config path` | Print config file location |
| `orro config set` | Set config values |
| `orro config init` | Interactive config setup |

### Global Flags

| Flag | Description |
|------|-------------|
| `--output table\|json` | Output format (default: `table`) |
| `--cloud` | Force Cloud API (skip LAN) |
| `--verbose` / `-v` | Print debug info (connection attempts, API calls) |
| `--quiet` / `-q` | Suppress info banners, output data only |
| `--config PATH` | Path to config file |
| `--version` | Print version and exit |

## Configuration

orro reads configuration from multiple sources with this precedence (highest first):

1. **CLI flags** — `--cloud`, `--config`, etc.
2. **Environment variables** — `ORRO_ENDPOINT`, `ORRO_ACCESS_ID`, etc.
3. **Config file** — `~/.config/orro/config.yaml` (or `ORRO_CONFIG` / `--config`)
4. **1Password CLI** — vault `OpenClaw`, item `Tuya IoT Platform`
5. **Built-in defaults** — preset mappings, LAN DP IDs

### Config File

Location: `~/.config/orro/config.yaml` (created with `0600` permissions)

```yaml
endpoint: https://openapi.tuyaeu.com
access_id: your_access_id
access_secret: your_access_secret
device_id: your_device_id
local_key: your_local_key
lan_ip: 192.168.10.95
lan_version: "3.4"

presets:
  sit: mem1
  stand: mem3
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ORRO_ENDPOINT` | Tuya API endpoint URL |
| `ORRO_ACCESS_ID` | Tuya IoT Platform Access ID |
| `ORRO_ACCESS_SECRET` | Tuya IoT Platform Access Secret |
| `ORRO_DEVICE_ID` | Tuya device ID |
| `ORRO_LOCAL_KEY` | Device local encryption key |
| `ORRO_LAN_IP` | Device LAN IP (skips discovery) |
| `ORRO_LAN_VERSION` | Tuya LAN protocol version (e.g. `3.4`) |
| `ORRO_LAN_DP_MAP` | JSON override for LAN DP IDs |
| `ORRO_PRESETS` | JSON override for preset mappings |
| `ORRO_CONFIG` | Path to config file |

### 1Password CLI

If the `op` CLI is available, orro reads credentials from vault `OpenClaw`, item `Tuya IoT Platform`:

| Field | Maps to |
|-------|---------|
| `API Endpoint` | endpoint |
| `Access ID` | access_id |
| `Access Secret` | access_secret |
| `Device ID` | device_id |
| `Local Key` | local_key |
| `LAN IP` | lan_ip |
| `LAN Version` | lan_version |
| `LAN DP Map` | lan_dps (JSON) |
| `Presets` | presets (JSON) |

## LAN vs Cloud

orro tries **LAN first** via TinyTuya, then falls back to the **Tuya Cloud API** if the device is unreachable locally. LAN control is faster (~50ms vs ~500ms) and works without internet.

LAN mode requires:
- `local_key` — the device encryption key (from Tuya IoT Platform)
- `lan_ip` (optional) — set explicitly or let orro discover it via UDP broadcast
- `lan_version` (optional) — protocol version, auto-detected during connection

Use `--cloud` to skip LAN and go straight to the Cloud API.

## Supported Desks

orro works with Tuya-based standing desks in the `sjz` (adjustable table) category. Tested with:

- **Orro standing desk** (South Africa)

Other Tuya `sjz` desks should work — the protocol and DP codes are standard. If your desk uses different DP IDs, override them via `ORRO_LAN_DP_MAP` or the config file `lan_dps` key.

## Getting Tuya Credentials

1. Download the **Tuya Smart** or **Smart Life** app and add your desk
2. Create an account on the [Tuya IoT Platform](https://platform.tuya.com/)
3. Create a **Cloud Development** project (choose your data centre region)
4. Link your Smart Life app account under **Devices → Link Tuya App Account**
5. Note your **Access ID** and **Access Secret** from the project overview
6. Find your **Device ID** in the device list
7. The **API Endpoint** depends on your region (e.g. `https://openapi.tuyaeu.com` for Europe)
8. The **Local Key** appears in the device details

## Discovery

The included `scripts/discover_tuya.py` queries multiple Tuya Cloud API endpoints for your device and writes a full DP map to `dp-discovery.json`:

```bash
python scripts/discover_tuya.py
```

This is useful for identifying DP codes if your desk model uses different IDs than the defaults.

## Known API Realities

- **LAN protocol version varies** — orro tries 3.5, 3.4, 3.3, 3.1 in order. Set `lan_version` to skip the scan.
- **LAN discovery uses UDP broadcast** — may not work across VLANs. Set `lan_ip` explicitly if discovery fails.
- **Cloud API rate limits** — Tuya enforces rate limits on the IoT Platform API. LAN mode avoids this entirely.
- **Height polling** — the `height` command polls status every 500ms for up to 30 seconds. The desk stops within ~5mm of the target.
- **Memory preset DPs** — `memory_location` sends a preset name (e.g. `"mem1"`), not a height value. The desk handles the movement internally.
- **Status DPs are read-only on LAN** — movement and preset commands use different DPs than what `status` returns.

## Prior Work / References

- [TinyTuya](https://github.com/jasonacox/tinytuya) — Python library for local Tuya device control
- [Tuya IoT Platform](https://platform.tuya.com/) — official Cloud API
- [tuyaopen](https://github.com/tuya/tuyaopen) — Tuya open SDK
- [steipete/eightctl](https://github.com/steipete/eightctl) — CLI patterns inspiration

## License

[MIT](LICENSE)
